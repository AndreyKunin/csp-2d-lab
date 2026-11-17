//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"math/rand"
	"testing"
)

// UT-ARN-01: allocations do not overlap; cap limits appends.
func TestPostfixArenaAllocDoesNotOverlap(t *testing.T) {
	a := csp2d.NewPostfixArena("t", 10)
	s1 := a.Alloc(3)
	s2 := a.Alloc(3)
	if len(s1) != 0 || cap(s1) != 3 || cap(s2) != 3 {
		t.Fatalf("len/cap: s1 %d/%d s2 %d/%d", len(s1), cap(s1), len(s2), cap(s2))
	}
	s1 = append(s1, 1, 2, 3)
	s2 = append(s2, 7, 8, 9)
	if !equalInts(s1, []int{1, 2, 3}) || !equalInts(s2, []int{7, 8, 9}) {
		t.Fatalf("overlap: s1 %v s2 %v", s1, s2)
	}
	// Appending past cap must reallocate, not overwrite s2.
	s1 = append(s1, 4)
	if !equalInts(s2, []int{7, 8, 9}) {
		t.Fatalf("append past cap overwrote the neighbour: %v", s2)
	}
}

// UT-ARN-02: overflow falls back to the heap and is counted.
func TestPostfixArenaOverflow(t *testing.T) {
	a := csp2d.NewPostfixArena("t", 4)
	_ = a.Alloc(3)
	s := a.Alloc(3)
	if cap(s) != 3 || a.OverflowCount() != 1 {
		t.Fatalf("cap %d overflow %d", cap(s), a.OverflowCount())
	}
	if used, capacity, _ := a.Stats(); used != 3 || capacity != 4 {
		t.Fatalf("stats used=%d capacity=%d", used, capacity)
	}
}

// UT-ARN-03 / UT-ARN-04: AllocFilled copies; Reset rewinds, so the next
// allocation reuses the same memory.
func TestPostfixArenaFilledAndReset(t *testing.T) {
	a := csp2d.NewPostfixArena("t", 8)
	src := []int{5, 6, 7}
	cp := a.AllocFilled(src)
	src[0] = 99
	if !equalInts(cp, []int{5, 6, 7}) {
		t.Fatalf("AllocFilled did not copy: %v", cp)
	}
	if a.AllocFilled(nil) != nil {
		t.Fatal("empty AllocFilled must return nil")
	}
	a.Reset()
	if used, _, _ := a.Stats(); used != 0 {
		t.Fatalf("used %d after Reset", used)
	}
	_ = a.AllocFilled([]int{8, 9, 10})
	if !equalInts(cp, []int{8, 9, 10}) {
		t.Fatalf("Reset did not rewind: the next allocation took other memory (%v)", cp)
	}
}

// UT-ARN-28: Reset only rewinds the cursor: it does not write the used region,
// so recycling an arena costs O(1) whatever its size.
func TestArenaResetDoesNotWriteBuffer(t *testing.T) {
	a := csp2d.NewPostfixArena("t", 64)
	defer a.Release()
	stale := a.AllocFilled([]int{5, 6, csp2d.NumS})
	a.Reset()
	if !equalInts(stale, []int{5, 6, csp2d.NumS}) {
		t.Fatalf("Reset wrote the used region: %v", stale)
	}
	if used, _, _ := a.Stats(); used != 0 {
		t.Fatalf("cursor %d after Reset", used)
	}
}

// UT-ARN-05: generation lifetime contract — a slice allocated in epoch e is
// intact after one rotation and its memory is handed out again after two.
func TestArenaGenerationLifetime(t *testing.T) {
	ar := csp2d.NewArena(5, 4, 1)
	s := ar.GenAlloc([]int{1, 2, 3})
	ar.RotateGenerations()
	if !equalInts(s, []int{1, 2, 3}) {
		t.Fatalf("slice changed after one rotation: %v", s)
	}
	ar.RotateGenerations()
	_ = ar.GenAlloc([]int{7, 8, 9})
	if !equalInts(s, []int{7, 8, 9}) {
		t.Fatalf("memory not reused after two rotations: %v", s)
	}
}

// UT-ARN-06: an overflowing epoch grows the generation arenas to the observed
// demand (served + overflowed ints) plus margin: the next-generation arena at
// the next rotation, the other one at the rotation after. Slices allocated
// before the growth stay readable for their generation lifetime.
func TestArenaGrowsAfterOverflow(t *testing.T) {
	ar := csp2d.NewArena(1, 1, 1) // budget: 64 ints per individual minimum
	defer ar.Release()
	before, _ := ar.GenCapacitiesForTest()
	kept := ar.GenAlloc([]int{4, 5, 6})
	_ = ar.GenAllocN(before + 1) // overflow
	demand := 3 + before + 1
	ar.RotateGenerations()
	if _, next := ar.GenCapacitiesForTest(); next < demand {
		t.Fatalf("next generation arena %d ints after an epoch that needed %d", next, demand)
	}
	if !equalInts(kept, []int{4, 5, 6}) {
		t.Fatalf("slice allocated before growth changed: %v", kept)
	}
	ar.RotateGenerations()
	if cur, next := ar.GenCapacitiesForTest(); cur < demand || next < demand {
		t.Fatalf("capacities after two rotations %d/%d, want both >= %d", cur, next, demand)
	}
}

// UT-ARN-07: PromoteElites copies elite postfixes and forest postfixes into
// fresh storage with identical content.
func TestPromoteElitesCopies(t *testing.T) {
	rnd := rand.New(rand.NewSource(31))
	p := newProblem(t, distinctParts(12, rnd), sheet100, 0, 8, 3)
	species := make([]problem.Individual, 8)
	for i := range species {
		species[i] = evaluated(t, p, randomPostfix(rnd, 12))
	}
	p.SetPopulation(&problem.Population{Species: &species})
	type snap struct {
		postfix, firstTree []int
		pPtr, tPtr         *int
	}
	before := make([]snap, 3)
	for i := 0; i < 3; i++ {
		ind := csp2d.AsIndividualForTest(species[i])
		forest := ind.ForestForTest()
		before[i] = snap{copyInts(ind.PostfixForTest()), copyInts(forest[0].Postfix), &ind.PostfixForTest()[0], &forest[0].Postfix[0]}
	}
	p.PromoteElites(3)
	for i := 0; i < 3; i++ {
		ind := csp2d.AsIndividualForTest(species[i])
		forest := ind.ForestForTest()
		if !equalInts(ind.PostfixForTest(), before[i].postfix) || !equalInts(forest[0].Postfix, before[i].firstTree) {
			t.Fatalf("elite %d content changed by promotion", i)
		}
		if &ind.PostfixForTest()[0] == before[i].pPtr || &forest[0].Postfix[0] == before[i].tPtr {
			t.Fatalf("elite %d was not copied into the elite arena", i)
		}
	}
}

// UT-ARN-08: scratch getters return empty slices with the documented
// capacities; the label scratch grows on demand.
func TestArenaScratch(t *testing.T) {
	ar := csp2d.NewArena(9, 4, 1)
	if got := ar.GetForestBuffer(); len(got) != 9 {
		t.Errorf("forest buffer len %d, want 9", len(got))
	}
	checks := map[string][2]int{
		"forest slice": {len(ar.GetForestSliceBuffer()), cap(ar.GetForestSliceBuffer())},
		"reinsertion":  {len(ar.GetReinsertionForest()), cap(ar.GetReinsertionForest())},
		"reinsert pf":  {len(ar.GetReinsertionPostfix()), cap(ar.GetReinsertionPostfix())},
		"trial":        {len(ar.GetTrialPostfix()), cap(ar.GetTrialPostfix())},
	}
	for name, lc := range checks {
		if lc[0] != 0 || lc[1] < 9 {
			t.Errorf("%s: len %d cap %d", name, lc[0], lc[1])
		}
	}
	if s := ar.GetNumLabelScratch(100); len(s) != 0 || cap(s) < 100 {
		t.Errorf("label scratch len %d cap %d", len(s), cap(s))
	}
}
