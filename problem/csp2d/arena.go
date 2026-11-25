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
//   - eliteArena: holds postfixes of elite individuals (re-promoted each
//     epoch from the survivors). It alternates with eliteStandby, which holds
//     the previous promotion's copies.
//
// At the end of an epoch (RotateGenerations):
//  1. elites' postfix data is copied into the reset standby elite buffer,
//     which becomes eliteArena (PromoteElites);
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
	trialPostfix       []int    // a critical-flip trial being decoded
	candidatePostfix   []int    // the candidate child of a mutation step
	crossoverOffspring [2][]int // the two children of one crossover attempt
	// trialInts holds the sheets re-packed by eliminate-and-repair (bump
	// allocated, rewound for every reinsertion order). It grows when an order
	// needs more; slices into a replaced buffer stay valid.
	trialInts   []int
	trialCursor int
	// reinsertionFresh[i] reports whether reinsertion forest node i was built
	// in trialInts (true) or is shared with the parent's forest (false).
	reinsertionFresh []bool

	// Working buffers of the hot paths (MEM-04). Each is used by one function
	// and is valid until that function is called again.
	decodeStack      []MeteredNode       // decodeForest: operand stack
	decodeSheets     []MeteredNode       // decodeForest: the returned sheets
	baseForest       []MeteredNode       // mutateEliminateAndRepair: sheets kept
	leafItems        []int               // mutateEliminateAndRepair: the removed sheet's parts
	orders           [][]int             // buildReinsertionOrders: the 4 orders
	crossoverLeaves  []int               // pickDonorSubtree: leaf count per pending subtree
	crossoverEnds    []int               // pickDonorSubtree: candidate subtree ends
	crossoverRemoved []bool              // removeLeaves: part id -> removed (cleared before return)
	crossoverPresent []bool              // removeLeaves: pending subtree is non-empty
	crossoverRest    []int               // removeLeaves: the result
	rankStack        []Metrics           // rankCriticalOperators: operand stack
	rankCandidate    []OperatorCandidate // rankCriticalOperators: the result

	// Tier-2: double-buffered generational + elite arenas.
	currentGen *PostfixArena
	nextGen    *PostfixArena
	eliteArena *PostfixArena
	// eliteStandby holds the elites of the previous promotion. Demoted elites
	// are replaced within the epoch that demotes them, so by the next
	// promotion nothing references it and it is reset and reused (MEM-03),
	// unless the caller reports that it is still referenced.
	eliteStandby *PostfixArena
	// eliteGuardLogged: the warning about a still-referenced standby buffer
	// was logged for this job.
	eliteGuardLogged bool

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

// eliteMultiplier: an elite needs its postfix (2n-1 ints) plus its forest's
// postfixes (2n - sheets ints), at most 2 postfix lengths, so the elite arena
// never overflows.
const eliteMultiplier = 2

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
		crossoverOffspring: [2][]int{make([]int, 0, postfixLen), make([]int, 0, postfixLen)},
		trialInts:          make([]int, 2*postfixLen),
		reinsertionFresh:   make([]bool, 0, postfixLen),
		decodeStack:        make([]MeteredNode, 0, postfixLen/2+1),
		decodeSheets:       make([]MeteredNode, 0, postfixLen/2+1),
		baseForest:         make([]MeteredNode, 0, postfixLen/2+1),
		leafItems:          make([]int, 0, postfixLen/2+1),
		orders:             [][]int{make([]int, 0, postfixLen/2+1), make([]int, 0, postfixLen/2+1), make([]int, 0, postfixLen/2+1), make([]int, 0, postfixLen/2+1)},
		crossoverLeaves:    make([]int, 0, postfixLen/2+1),
		crossoverEnds:      make([]int, 0, postfixLen/2),
		crossoverRemoved:   make([]bool, postfixLen/2+1),
		crossoverPresent:   make([]bool, 0, postfixLen/2+1),
		crossoverRest:      make([]int, 0, postfixLen),
		rankStack:          make([]Metrics, 0, postfixLen/2+1),
		rankCandidate:      make([]OperatorCandidate, 0, postfixLen/2+1),
		numLabelScratch:    make([]int32, 0, postfixLen),
		postfixLen:         postfixLen,
		currentGen:         NewPostfixArena("currentGen", genCapacity),
		nextGen:            NewPostfixArena("nextGen", genCapacity),
		eliteArena:         NewPostfixArena("elite", eliteCapacity),
		eliteStandby:       NewPostfixArena("elite", eliteCapacity),
		genTarget:          genCapacity,
	}
}

func (a *Arena) GetForestBuffer() []MeteredNode      { return a.forestBuffer[:a.postfixLen] }
func (a *Arena) GetForestSliceBuffer() []int         { return a.forestSliceBuffer[:0] }
func (a *Arena) GetReinsertionForest() []MeteredNode { return a.reinsertionForest[:0] }
func (a *Arena) GetReinsertionPostfix() []int        { return a.reinsertionPostfix[:0] }
func (a *Arena) GetTrialPostfix() []int              { return a.trialPostfix[:0] }
func (a *Arena) GetCandidatePostfix() []int          { return a.candidatePostfix[:0] }

// GetCrossoverOffspring returns the two empty crossover child buffers.
func (a *Arena) GetCrossoverOffspring() ([]int, []int) {
	return a.crossoverOffspring[0][:0], a.crossoverOffspring[1][:0]
}

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

// NextEliteArena prepares the elite arena for a promotion of eliteCount
// elites: the standby buffer, which holds the copies of two promotions ago, is
// reset and becomes the elite arena; the current one becomes the standby. The
// current elites' data therefore stays intact while it is copied.
//
// inUse reports whether anything still references the standby buffer. If it
// does (a population that was not fully replaced, BE-01), the buffer is left
// to the GC with its referents and a fresh one is used. A buffer smaller than
// eliteCount needs is replaced too, as far as the arena budget allows.
func (a *Arena) NextEliteArena(eliteCount int, inUse func(*PostfixArena) bool) {
	want := max(eliteCount, 1) * perIndividual(a.postfixLen, eliteMultiplier)
	standby := a.eliteStandby
	referenced := inUse(standby)
	if referenced && !a.eliteGuardLogged {
		a.eliteGuardLogged = true
		util.Warning.Printf("PostfixArena[elite]: the previous elite buffer is still referenced by the population; using a new one (population not fully replaced)")
	}
	grow := standby.Capacity() < want && availableArenaInts() > 0
	if referenced || grow {
		overflows := standby.OverflowCount()
		standby.Release()
		standby = NewPostfixArena("elite", want)
		standby.overflow = overflows // keep the warning throttle across buffers
	} else {
		standby.Reset()
	}
	a.eliteStandby, a.eliteArena = a.eliteArena, standby
}

// Release returns all arena buffers to the process budget. Call it when the
// job no longer needs the arena; later allocations fall back to the heap.
func (a *Arena) Release() {
	a.currentGen.Release()
	a.nextGen.Release()
	a.eliteArena.Release()
	a.eliteStandby.Release()
}

// GetNumLabelScratch returns an empty []int32 with cap >= n. The buffer is
// shared and is overwritten on every call.
func (a *Arena) GetNumLabelScratch(n int) []int32 {
	if cap(a.numLabelScratch) < n {
		a.numLabelScratch = make([]int32, 0, n)
	}
	return a.numLabelScratch[:0]
}
