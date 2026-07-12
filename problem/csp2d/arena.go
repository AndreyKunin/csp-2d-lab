package csp2d

// Arena holds reusable scratch buffers and generational allocators
// for postfix arrays. It is owned by a single Csp2dProblemImpl and
// is NOT goroutine-safe.
//
// Generational lifetime model (double-buffered):
//
//   - currentGen: holds postfixes from the previous epoch — these are the
//     PARENTS being read this epoch by crossover/mutation.
//   - nextGen:    holds postfixes being written this epoch (new offspring,
//     fitness-evaluation forests, mutation children).
//   - eliteArena: holds postfixes of elite individuals (rebuilt each epoch
//     from the survivors).
//
// At the end of an epoch (RotateGenerations):
//  1. elites' postfix data is copied into a fresh eliteArena;
//  2. currentGen is reset to zero;
//  3. currentGen and nextGen are swapped.
//
// This guarantees that readers (parents) and writers (children) never
// alias the same memory within an epoch.
type Arena struct {
	forestBuffer       []MeteredNode
	forestSliceBuffer  []int
	reinsertionForest  []MeteredNode
	reinsertionPostfix []int
	postfixLen         int

	// Tier-1: reusable scratch for trial children that are usually discarded.
	trialPostfix []int

	// Tier-2: double-buffered generational + elite arenas.
	currentGen *PostfixArena
	nextGen    *PostfixArena
	eliteArena *PostfixArena

	// Reusable scratch for the numeric-label projection used by the
	// elite-protection lookup (toNumLabels). One buffer is enough because
	// every call site consumes the result synchronously (build → use as
	// map key → discard) before the next call.
	numLabelScratch []int32
}

// perIndividualBudget estimates how many ints one individual may consume
// in a single epoch across all allocation sites:
//   - 1× postfixLen for offspring postfix (PMX or random)
//   - 1× postfixLen for cached meteredForest postfixes (sum of leaves+ops)
//   - up to 4× postfixLen for mutation trials (mutateCriticalOperatorFlip)
//   - up to 4× postfixLen for reinsertion trials (mutateEliminateAndRepair)
//   - safety multiplier
const perIndividualMultiplier = 16

func computePerIndividualBudget(postfixLen int) int {
	budget := postfixLen * perIndividualMultiplier
	if budget < 64 {
		budget = 64
	}
	return budget
}

func NewArena(postfixLen int, populationSize int, eliteCount int) *Arena {
	per := computePerIndividualBudget(postfixLen)
	genCapacity := populationSize * per
	eliteCapacity := eliteCount * per
	if eliteCapacity < per {
		eliteCapacity = per
	}

	return &Arena{
		forestBuffer:       make([]MeteredNode, postfixLen),
		forestSliceBuffer:  make([]int, 0, postfixLen*(postfixLen/2+1)),
		reinsertionForest:  make([]MeteredNode, postfixLen),
		reinsertionPostfix: make([]int, 0, postfixLen),
		trialPostfix:       make([]int, 0, postfixLen),
		numLabelScratch:    make([]int32, 0, postfixLen),
		postfixLen:         postfixLen,
		currentGen:         NewPostfixArena("currentGen", genCapacity),
		nextGen:            NewPostfixArena("nextGen", genCapacity),
		eliteArena:         NewPostfixArena("elite", eliteCapacity),
	}
}

func (a *Arena) GetForestBuffer() []MeteredNode      { return a.forestBuffer[:a.postfixLen] }
func (a *Arena) GetForestSliceBuffer() []int         { return a.forestSliceBuffer[:0] }
func (a *Arena) GetReinsertionForest() []MeteredNode { return a.reinsertionForest[:0] }
func (a *Arena) GetReinsertionPostfix() []int        { return a.reinsertionPostfix[:0] }
func (a *Arena) GetTrialPostfix() []int              { return a.trialPostfix[:0] }

// GenAlloc allocates a slice in the NEXT-generation arena and copies src into it.
// Use this for NEW offspring/children produced this epoch.
func (a *Arena) GenAlloc(src []int) []int { return a.nextGen.AllocFilled(src) }

// GenAllocN allocates an empty slice with capacity n from the next-generation arena.
func (a *Arena) GenAllocN(n int) []int { return a.nextGen.Alloc(n) }

// EliteAlloc copies src into the elite arena (used during promotion only).
func (a *Arena) EliteAlloc(src []int) []int { return a.eliteArena.AllocFilled(src) }

// RotateGenerations swaps currentGen with nextGen and resets the new currentGen.
// Call this AFTER an epoch completes (after crossover/mutation/fitness, before
// the next epoch starts reading parents).
//
// Auto-grows the arena if the previous epoch overflowed.
func (a *Arena) RotateGenerations() {
	// If the just-completed epoch overflowed nextGen, grow the buffers so
	// future epochs have headroom. Both gen buffers must stay the same size
	// because they swap roles.
	overflow := a.nextGen.OverflowCount()
	if overflow > 0 {
		// Grow proportionally: at least double, plus a safety margin.
		newCap := a.nextGen.Capacity() * 2
		// Reallocate both — they swap roles, so they must share capacity.
		a.currentGen = NewPostfixArena("currentGen", newCap)
		a.nextGen = NewPostfixArena("nextGen", newCap)
		return
	}

	// Reset what is about to become the next "next" buffer (i.e. the one
	// that held this epoch's parents — they're no longer needed because
	// elites have been promoted and non-elites are being replaced).
	a.currentGen.Reset()
	a.currentGen, a.nextGen = a.nextGen, a.currentGen
}

// RebuildEliteArena replaces the elite arena with a fresh one sized for
// `eliteCount`. The previous elite arena (and slices into it) become
// unreachable — callers must have copied any survivors out beforehand.
func (a *Arena) RebuildEliteArena(eliteCount int) {
	per := computePerIndividualBudget(a.postfixLen)
	cap_ := eliteCount * per
	if cap_ < per {
		cap_ = per
	}
	a.eliteArena = NewPostfixArena("elite", cap_)
}

// GetNumLabelScratch returns an empty []int32 with cap >= n. The buffer is
// shared and is overwritten on every call.
func (a *Arena) GetNumLabelScratch(n int) []int32 {
	if cap(a.numLabelScratch) < n {
		a.numLabelScratch = make([]int32, 0, n)
	}
	return a.numLabelScratch[:0]
}
