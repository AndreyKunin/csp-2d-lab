//go:build testhooks

package csp2d_test

import (
	"csp2d/mvc"
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"encoding/json"
	"math/rand"
	"os"
	"testing"
	"unsafe"
)

// MEM-02: candidates that are rejected (protected genotype, no improvement) or
// superseded within the same call must not take generation-arena space. The
// arena demand of a call is what the individuals it keeps need, nothing more.

func forestLen(forest []csp2d.NodeView) int {
	n := 0
	for _, tree := range forest {
		n += len(tree.Postfix)
	}
	return n
}

// sameBacking reports whether two non-empty slices start at the same element.
func sameBacking(a, b []int) bool {
	return len(a) > 0 && len(b) > 0 && unsafe.SliceData(a) == unsafe.SliceData(b)
}

// UT-ARN-12: NewRandomIndividual takes arena space only for the individual it
// returns; protected attempts take none.
func TestRandomIndividualRejectedTakesNoArena(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}, {"2", 20, 20}}, sheet100, 0, 16, 4)
	var all []problem.Individual
	for _, pf := range [][]int{{0, 1, S}, {0, 1, L}, {1, 0, S}, {1, 0, L}} {
		all = append(all, csp2d.NewIndividualForTest(pf))
	}
	p.Protect(all) // every genotype is protected: all attempts are rejected
	before := p.ArenaForTest().GenDemandForTest()
	ind, err := p.NewRandomIndividual(rand.New(rand.NewSource(1)))
	if err != nil || ind != nil {
		t.Fatalf("got %v, %v; want nil, nil (gene pool exhausted)", ind, err)
	}
	if d := p.ArenaForTest().GenDemandForTest() - before; d != 0 {
		t.Fatalf("rejected attempts took %d arena ints, want 0", d)
	}
}

// UT-ARN-13: Crossover takes arena space only for the offspring it returns;
// PMX children rejected as protected take none.
func TestCrossoverRejectedTakesNoArena(t *testing.T) {
	const n = 10
	rnd := rand.New(rand.NewSource(12))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	elite := randomPostfix(rnd, n)
	p.Protect([]problem.Individual{csp2d.NewIndividualForTest(elite)})
	before := p.ArenaForTest().GenDemandForTest()
	// Identical protected parents: every PMX child is the protected genotype,
	// so all retries are rejected and two random individuals are returned.
	offspring, failures, err := p.Crossover(individuals(elite, copyInts(elite)), rnd)
	if err != nil || failures != 2 || len(offspring) != 2 {
		t.Fatalf("offspring %d failures %d err %v; want 2, 2, nil", len(offspring), failures, err)
	}
	if d, want := p.ArenaForTest().GenDemandForTest()-before, 2*(2*n-1); d != want {
		t.Fatalf("crossover took %d arena ints, want %d (two postfixes)", d, want)
	}
}

// UT-ARN-14: the critical operator flip takes arena space only for the child it
// returns (its postfix and forest); rejected trials take none.
func TestCriticalFlipTrialsTakeNoArena(t *testing.T) {
	const n = 15
	improvedCount := 0
	for seed := int64(1); seed <= 100; seed++ {
		rnd := rand.New(rand.NewSource(seed))
		p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 64, 2)
		parent := evaluated(t, p, randomPostfix(rnd, n))
		before := p.ArenaForTest().GenDemandForTest()
		child, improved, err := p.MutateCriticalOperatorFlipForTest(parent, rnd)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if improved {
			improvedCount++
			want = len(child.PostfixForTest()) + forestLen(child.ForestForTest())
		}
		if d := p.ArenaForTest().GenDemandForTest() - before; d != want {
			t.Fatalf("seed %d (improved %v): flip took %d arena ints, want %d", seed, improved, d, want)
		}
	}
	if improvedCount == 0 || improvedCount == 100 {
		t.Fatalf("%d of 100 seeds improved; the test must exercise both outcomes", improvedCount)
	}
}

// UT-ARN-15: eliminate-and-repair takes arena space only for the child it
// returns: its postfix and the sheets it re-packed. Failed reinsertion orders
// and intermediate insertions take none.
func TestEliminateAndRepairTrialsTakeNoArena(t *testing.T) {
	const n = 15
	improvedCount := 0
	for seed := int64(1); seed <= 200; seed++ {
		rnd := rand.New(rand.NewSource(seed))
		p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 64, 2)
		parent := evaluated(t, p, randomPostfix(rnd, n))
		before := p.ArenaForTest().GenDemandForTest()
		child, improved, err := p.MutateEliminateAndRepairForTest(parent, rnd)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if improved {
			improvedCount++
			want = len(child.PostfixForTest())
			parentForest := parent.ForestForTest()
			for _, tree := range child.ForestForTest() {
				shared := false
				for _, pt := range parentForest {
					shared = shared || sameBacking(tree.Postfix, pt.Postfix)
				}
				if !shared {
					want += len(tree.Postfix)
				}
			}
		}
		if d := p.ArenaForTest().GenDemandForTest() - before; d != want {
			t.Fatalf("seed %d (improved %v): repair took %d arena ints, want %d", seed, improved, d, want)
		}
	}
	if improvedCount < 10 {
		t.Fatalf("only %d of 200 seeds improved; the test exercises too little", improvedCount)
	}
}

func fixtureProblem(t testing.TB, name string, popSize, eliteCount int) (*csp2d.Csp2dProblemImpl, int) {
	t.Helper()
	b, err := os.ReadFile("../../functional/fixtures/inputs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var r mvc.Request
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	inputs, err := r.BuildInputs()
	if err != nil {
		t.Fatal(err)
	}
	p := csp2d.NewProblemForTest(inputs, r.BuildConstraints())
	p.InitForTest(popSize, eliteCount)
	return p, len(inputs)
}

// UT-ARN-16: a slot mutated several times in one Mutation call keeps one
// postfix buffer: the children it supersedes take no further arena space.
// With the default rate a 60-part individual is mutated about 3 times per
// epoch; the arena demand must stay near one postfix per individual instead of
// one per accepted mutation.
func TestMutationChainReusesSlotBuffer(t *testing.T) {
	const size = 200
	p, n := fixtureProblem(t, "D14-medium-60.json", 2000, 10)
	rnd := rand.New(rand.NewSource(3))
	set := make([]*problem.Individual, size)
	originals := make([]*csp2d.Csp2dIndividualImpl, size)
	snapshots := make([][]csp2d.NodeView, size)
	for i := range set {
		ind := evaluated(t, p, randomPostfix(rnd, n)) // parents' forests are not part of the measurement
		var pi problem.Individual = ind
		set[i] = &pi
		originals[i] = ind
		for _, tree := range ind.ForestForTest() {
			snapshots[i] = append(snapshots[i], csp2d.NodeView{Postfix: copyInts(tree.Postfix), Long: tree.Long, Short: tree.Short})
		}
	}
	before := p.ArenaForTest().GenDemandForTest()
	count, _, _, err := p.Mutation(0.07, set, rnd)
	if err != nil {
		t.Fatal(err)
	}
	postfixLen := 2*n - 1
	perIndividual := float64(p.ArenaForTest().GenDemandForTest()-before) / float64(size*postfixLen)
	t.Logf("%d accepted mutations (%.2f per individual); arena demand %.2f postfix lengths per individual",
		count, float64(count)/size, perIndividual)
	if float64(count)/size < 2 {
		t.Fatalf("only %.2f mutations per individual; the test needs chains", float64(count)/size)
	}
	if perIndividual > 2 {
		t.Fatalf("arena demand %.2f postfix lengths per individual, want at most 2", perIndividual)
	}
	// No kept individual may have been overwritten by a reused buffer. (The
	// cached fitness is not compared with a decode: GA-10.)
	for i, ind := range set {
		if err := checkStructure(p, csp2d.AsIndividualForTest(*ind), n); err != nil {
			t.Fatalf("individual %d: %v", i, err)
		}
	}
	// Only children created by the call are reused: the individuals passed in
	// keep their cached forests (MEM-04 reuses forest buffers too).
	for i, o := range originals {
		forest := o.ForestForTest()
		if len(forest) != len(snapshots[i]) {
			t.Fatalf("input %d: forest has %d sheets, had %d", i, len(forest), len(snapshots[i]))
		}
		for k, tree := range forest {
			if want := snapshots[i][k]; !equalInts(tree.Postfix, want.Postfix) || tree.Long != want.Long || tree.Short != want.Short {
				t.Fatalf("input %d: cached sheet %d changed: %v -> %v", i, k, want, tree)
			}
		}
	}
}

// UT-ARN-18 (also MEM-05): when only the fallback step applies, Mutation takes
// one postfix of arena space for the new child: no copy of the parent before
// the operator, and no space for the critical-flip trial that did not improve.
func TestMutationFallbackTakesOnePostfix(t *testing.T) {
	// Two equal squares on one sheet: there is nothing to eliminate, and both
	// operators give the same area, so neither local search step improves.
	p := newProblem(t, []part{{"1", 10, 10}, {"2", 10, 10}}, sheet100, 0, 16, 2)
	parent := evaluated(t, p, []int{0, 1, S})
	var pi problem.Individual = parent
	set := []*problem.Individual{&pi}
	before := p.ArenaForTest().GenDemandForTest()
	// Budget ceil(3 × 1 × 0.1) = 1 weight unit: exactly one fallback mutation.
	count, _, failures, err := p.Mutation(0.1, set, rand.New(rand.NewSource(1)))
	if err != nil || count != 1 || failures != 0 {
		t.Fatalf("count %d failures %d err %v; want 1 mutation", count, failures, err)
	}
	if d := p.ArenaForTest().GenDemandForTest() - before; d != 3 {
		t.Fatalf("mutation took %d arena ints, want 3 (one postfix)", d)
	}
	if child := csp2d.AsIndividualForTest(*set[0]); child == parent {
		t.Fatal("the slot still holds the parent")
	}
}
