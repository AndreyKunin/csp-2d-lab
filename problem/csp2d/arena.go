package csp2d

import "csp2d/util"

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
//  1. elites' postfix data is copied into a fresh eliteArena (PromoteElites);
//  2. currentGen, whose contents are now dead, is reset — or replaced by a
//     larger buffer when the epoch's demand outgrew it;
//  3. currentGen and nextGen are swapped.
//
// This guarantees that readers (parents) and writers (children) never
// alias the same memory within an epoch.
//
// Sizing: the generation arenas start at initialPerIndividualMultiplier
// postfixes per individual and follow the observed demand (ints requested from
// nextGen during an epoch, including the ones that overflowed to the heap) plus
// demandMargin. All buffers come from the process-wide arena budget
// (arena_budget.go); when it is exhausted an arena stays at the size it has and
// the excess is allocated on the heap, where the GC can collect it.
type Arena struct {
	forestBuffer       []MeteredNode
	forestSliceBuffer  []int
	reinsertionForest  []MeteredNode
	reinsertionPostfix []int
	postfixLen         int

	// Tier-1: reusable scratch for trial children that are usually discarded
	// (MEM-02). A candidate is built here and copied into nextGen only when it
	// is kept. These are fields, not locals: a local buffer whose size is not a
	// compile-time constant is heap-allocated on every call even when escape
	// analysis reports that it does not escape.
	trialPostfix     []int    // a critical-flip trial being decoded
	candidatePostfix []int    // the candidate child of a mutation step
	pmxOffspring     [2][]int // the two PMX children of one attempt
	// trialInts holds the sheets re-packed by eliminate-and-repair (bump
	// allocated, rewound for every reinsertion order). It grows when an order
	// needs more; slices into a replaced buffer stay valid.
	trialInts   []int
	trialCursor int
	// reinsertionFresh[i] reports whether reinsertion forest node i was built
	// in trialInts (true) or is shared with the parent's forest (false).
	reinsertionFresh []bool

	// Tier-2: double-buffered generational + elite arenas.
	currentGen *PostfixArena
	nextGen    *PostfixArena
	eliteArena *PostfixArena

	// genTarget is the desired generation-arena capacity in ints: the largest
	// observed per-epoch demand plus margin (the initial estimate before that).
	genTarget int

	// Reusable scratch for the numeric-label projection used by the
	// elite-protection lookup (toNumLabels). One buffer is enough because
	// every call site consumes the result synchronously (build → use as
	// map key → discard) before the next call.
	numLabelScratch []int32
}

// Measured arena demand per individual per epoch is about 3 postfix lengths
// (20 to 150 parts): only kept individuals and their forests are allocated here,
// trials are built in scratch. The initial size is that plus a margin. An
// over-sized buffer counts against the memory limit for the whole job, while an
// under-sized one costs a single epoch of heap allocations before the arena
// grows to the observed demand.
const initialPerIndividualMultiplier = 4

// eliteMultiplier: an elite needs its postfix plus its forest's postfixes,
// about 2 postfix lengths.
const eliteMultiplier = 4

// demandMargin is added to the observed demand when an arena grows (1/4 = 25%).
const demandMarginDivisor = 4

// minPerIndividual is the minimum budget per individual, in ints.
const minPerIndividual = 64

func perIndividual(postfixLen, multiplier int) int {
	return max(postfixLen*multiplier, minPerIndividual)
}

func NewArena(postfixLen int, populationSize int, eliteCount int) *Arena {
	genCapacity := populationSize * perIndividual(postfixLen, initialPerIndividualMultiplier)
	eliteCapacity := max(eliteCount, 1) * perIndividual(postfixLen, eliteMultiplier)

	return &Arena{
		forestBuffer:       make([]MeteredNode, postfixLen),
		forestSliceBuffer:  make([]int, 0, postfixLen*(postfixLen/2+1)),
		reinsertionForest:  make([]MeteredNode, postfixLen),
		reinsertionPostfix: make([]int, 0, postfixLen),
		trialPostfix:       make([]int, 0, postfixLen),
		candidatePostfix:   make([]int, 0, postfixLen),
		pmxOffspring:       [2][]int{make([]int, 0, postfixLen), make([]int, 0, postfixLen)},
		trialInts:          make([]int, 2*postfixLen),
		reinsertionFresh:   make([]bool, 0, postfixLen),
		numLabelScratch:    make([]int32, 0, postfixLen),
		postfixLen:         postfixLen,
		currentGen:         NewPostfixArena("currentGen", genCapacity),
		nextGen:            NewPostfixArena("nextGen", genCapacity),
		eliteArena:         NewPostfixArena("elite", eliteCapacity),
		genTarget:          genCapacity,
	}
}

func (a *Arena) GetForestBuffer() []MeteredNode      { return a.forestBuffer[:a.postfixLen] }
func (a *Arena) GetForestSliceBuffer() []int         { return a.forestSliceBuffer[:0] }
func (a *Arena) GetReinsertionForest() []MeteredNode { return a.reinsertionForest[:0] }
func (a *Arena) GetReinsertionPostfix() []int        { return a.reinsertionPostfix[:0] }
func (a *Arena) GetTrialPostfix() []int              { return a.trialPostfix[:0] }
func (a *Arena) GetCandidatePostfix() []int          { return a.candidatePostfix[:0] }

// GetPMXOffspring returns the two empty PMX child buffers.
func (a *Arena) GetPMXOffspring() ([]int, []int) { return a.pmxOffspring[0][:0], a.pmxOffspring[1][:0] }

// GetReinsertionFresh returns the empty freshness flags of the reinsertion forest.
func (a *Arena) GetReinsertionFresh() []bool { return a.reinsertionFresh[:0] }

// ResetTrialInts rewinds trialInts: every slice it handed out is dead.
func (a *Arena) ResetTrialInts() { a.trialCursor = 0 }

// TrialAllocN returns an empty slice with capacity n from trialInts. When the
// buffer is full it is replaced by a larger one; earlier slices keep the old
// buffer alive until they are dropped.
func (a *Arena) TrialAllocN(n int) []int {
	if a.trialCursor+n > len(a.trialInts) {
		a.trialInts = make([]int, max(2*len(a.trialInts), n))
		a.trialCursor = 0
	}
	s := a.trialInts[a.trialCursor : a.trialCursor : a.trialCursor+n]
	a.trialCursor += n
	return s
}

// GenAlloc allocates a slice in the NEXT-generation arena and copies src into it.
// Use this for NEW offspring/children produced this epoch.
func (a *Arena) GenAlloc(src []int) []int { return a.nextGen.AllocFilled(src) }

// GenAllocN allocates an empty slice with capacity n from the next-generation arena.
func (a *Arena) GenAllocN(n int) []int { return a.nextGen.Alloc(n) }

// EliteAlloc copies src into the elite arena (used during promotion only).
func (a *Arena) EliteAlloc(src []int) []int { return a.eliteArena.AllocFilled(src) }

// RotateGenerations swaps currentGen with nextGen and resets the new nextGen.
// Call this AFTER an epoch completes (after crossover/mutation/fitness, before
// the next epoch starts reading parents).
//
// The buffer that is reset held the parents of the epoch that just ended, so
// nothing references it any more. If the epoch's demand exceeded the arena,
// that buffer is replaced by one sized to the demand plus margin, as far as the
// arena budget allows. The other buffer (now holding the parents) is resized
// the same way at the next rotation, when it is dead in turn.
func (a *Arena) RotateGenerations() {
	if demand := a.nextGen.Demand(); demand > a.genTarget {
		a.genTarget = demand + demand/demandMarginDivisor
	}

	dead := a.currentGen
	if dead.Capacity() < a.genTarget {
		// Only reallocate when the budget allows a larger buffer than the current
		// one; otherwise keep it (no reallocation churn at the budget ceiling).
		// (availableArenaInts is MaxInt when unlimited: never add to it.)
		possible := dead.Capacity() + min(a.genTarget-dead.Capacity(), availableArenaInts())
		if possible > dead.Capacity() {
			name, overflows := dead.name, dead.OverflowCount()
			dead.Release()
			dead = NewPostfixArena(name, possible)
			dead.overflow = overflows // keep the warning throttle across buffers
			if dead.Capacity() < a.genTarget {
				util.Info.Printf("PostfixArena[%s]: %d of %d ints granted (arena budget)", name, dead.Capacity(), a.genTarget)
			}
		} else {
			dead.Reset()
		}
	} else {
		dead.Reset()
	}

	a.currentGen, a.nextGen = a.nextGen, dead
}

// RebuildEliteArena replaces the elite arena with a fresh one sized for
// `eliteCount`. The previous elite arena (and slices into it) become
// unreachable — callers must have copied any survivors out beforehand.
func (a *Arena) RebuildEliteArena(eliteCount int) {
	overflows := a.eliteArena.OverflowCount()
	a.eliteArena.Release()
	a.eliteArena = NewPostfixArena("elite", max(eliteCount, 1)*perIndividual(a.postfixLen, eliteMultiplier))
	a.eliteArena.overflow = overflows // keep the warning throttle across epochs
}

// Release returns all arena buffers to the process budget. Call it when the
// job no longer needs the arena; later allocations fall back to the heap.
func (a *Arena) Release() {
	a.currentGen.Release()
	a.nextGen.Release()
	a.eliteArena.Release()
}

// GetNumLabelScratch returns an empty []int32 with cap >= n. The buffer is
// shared and is overwritten on every call.
func (a *Arena) GetNumLabelScratch(n int) []int32 {
	if cap(a.numLabelScratch) < n {
		a.numLabelScratch = make([]int32, 0, n)
	}
	return a.numLabelScratch[:0]
}
