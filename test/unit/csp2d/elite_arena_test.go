//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"csp2d/test/support/testutil"
	"testing"
)

// MEM-03: the elite arena is double-buffered instead of reallocated every epoch.

// UT-ARN-19: across many epochs the elites alternate between two buffers.
func TestEliteArenaIsDoubleBuffered(t *testing.T) {
	rnd := testutil.Rand(t, 41)
	p := newProblem(t, distinctParts(15, rnd), sheet100, 0, 30, 4)
	d := &epochDriver{p: p, popSize: 30, eliteCount: 4, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	buffers := map[*int]bool{}
	for e := 0; e < 12; e++ {
		if err := d.epoch(); err != nil {
			t.Fatalf("epoch %d: %v", e, err)
		}
		buffers[p.ArenaForTest().EliteArenaForTest().BaseForTest()] = true
	}
	if len(buffers) > 2 {
		t.Fatalf("12 epochs used %d elite buffers, want 2", len(buffers))
	}
}

// UT-ARN-20: an elite needs at most 2 postfix lengths (its postfix of 2n-1
// ints plus sheets totalling 2n - sheets ints), so the elite arena is sized to
// that and never overflows.
func TestEliteArenaSizedToTwoPostfixes(t *testing.T) {
	const n, elite = 40, 6
	rnd := testutil.Rand(t, 42)
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 40, elite)
	d := &epochDriver{p: p, popSize: 40, eliteCount: elite, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	for e := 0; e < 10; e++ {
		if err := d.epoch(); err != nil {
			t.Fatalf("epoch %d: %v", e, err)
		}
		ea := p.ArenaForTest().EliteArenaForTest()
		if want := elite * 2 * (2*n - 1); ea.Capacity() > want {
			t.Fatalf("elite arena %d ints, want at most %d (2 postfixes per elite)", ea.Capacity(), want)
		}
		if ea.OverflowCount() != 0 {
			t.Fatalf("epoch %d: elite arena overflowed %d times", e, ea.OverflowCount())
		}
	}
}

// UT-ARN-21: an individual that still references an old elite buffer is never
// overwritten when the buffer is reused. Normally every demoted elite is
// replaced within an epoch, but a population that is not fully replaced (a
// shrinking reconfigure did that before BE-01 was fixed) keeps one; reusing its
// buffer would corrupt it.
func TestEliteBufferStillReferencedIsNotReused(t *testing.T) {
	const n, pop, elite = 12, 20, 2
	rnd := testutil.Rand(t, 43)
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, pop, elite)
	d := &epochDriver{p: p, popSize: pop, eliteCount: elite, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	if err := d.epoch(); err != nil { // the elites now live in an elite buffer
		t.Fatal(err)
	}
	species := *p.GetPopulation().Species
	x := csp2d.AsIndividualForTest(species[0])
	postfix := copyInts(x.PostfixForTest())
	var trees [][]int
	for _, tree := range x.ForestForTest() {
		trees = append(trees, copyInts(tree.Postfix))
	}
	// Demote x to the last place and stop replacing that place (as a shrunk
	// population size does): it keeps pointing into its elite buffer.
	x.SetFitnessForTest([3]float64{1e9, 1e18, 1}, 1e27)
	species[0], species[pop-1] = species[pop-1], species[0]
	d.popSize = pop - 1
	for e := 0; e < 4; e++ {
		if err := d.epoch(); err != nil {
			t.Fatalf("epoch %d: %v", e, err)
		}
		if got := csp2d.AsIndividualForTest((*p.GetPopulation().Species)[pop-1]); got != x {
			t.Fatalf("epoch %d: the demoted individual moved", e)
		}
		if !equalInts(x.PostfixForTest(), postfix) {
			t.Fatalf("epoch %d: demoted elite overwritten: %v -> %v", e, postfix, x.PostfixForTest())
		}
		for i, tree := range x.ForestForTest() {
			if !equalInts(tree.Postfix, trees[i]) {
				t.Fatalf("epoch %d: demoted elite's sheet %d overwritten: %v -> %v", e, i, trees[i], tree.Postfix)
			}
		}
	}
}

// UT-ARN-22: the elite count can change between epochs (a reconfigure, BE-03):
// the elite buffers grow to the larger count, never overflow, and every
// individual's genes stay intact, up and down.
func TestEliteCountChangesBetweenEpochs(t *testing.T) {
	const n, pop = 20, 30
	rnd := testutil.Rand(t, 44)
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, pop, 2)
	d := &epochDriver{p: p, popSize: pop, eliteCount: 2, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	for _, elite := range []int{2, 9, 9, 1, 5} {
		d.eliteCount = elite
		for e := 0; e < 3; e++ {
			if err := d.epoch(); err != nil {
				t.Fatalf("elite %d, epoch %d: %v", elite, e, err)
			}
			ea := p.ArenaForTest().EliteArenaForTest()
			if ea.OverflowCount() != 0 {
				t.Fatalf("elite %d, epoch %d: elite arena overflowed %d times", elite, e, ea.OverflowCount())
			}
			if want := elite * 2 * (2*n - 1); ea.Capacity() < want {
				t.Fatalf("elite %d: elite arena %d ints, want at least %d", elite, ea.Capacity(), want)
			}
			for i, ind := range *p.GetPopulation().Species {
				if err := validatePostfix(csp2d.AsIndividualForTest(ind).PostfixForTest(), n); err != nil {
					t.Fatalf("elite %d, epoch %d, individual %d: %v", elite, e, i, err)
				}
			}
		}
	}
}
