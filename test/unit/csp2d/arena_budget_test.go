//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"testing"
)

// withArenaBudget sets the process-wide arena budget to what is reserved now
// plus extra bytes, and restores "unlimited" after the test. Arenas created by
// earlier tests are never released, so tests reason about deltas.
func withArenaBudget(t *testing.T, extra int64) (before int64) {
	t.Helper()
	before = csp2d.ArenaBytesReserved()
	csp2d.SetArenaBudget(before + extra)
	t.Cleanup(func() { csp2d.SetArenaBudget(0) })
	return before
}

// UT-ARN-17: the initial generation arenas are sized modestly (at most 4
// postfixes per individual) and grow from observed demand, instead of
// reserving 16 postfixes per individual up front.
func TestArenaInitialSizeIsModest(t *testing.T) {
	const postfixLen, pop = 299, 1000
	ar := csp2d.NewArena(postfixLen, pop, 10)
	defer ar.Release()
	cur, next := ar.GenCapacitiesForTest()
	if limit := 4 * postfixLen * pop; cur > limit || next > limit {
		t.Fatalf("initial generation arenas %d/%d ints, want at most %d (4 postfixes per individual)", cur, next, limit)
	}
}

// UT-ARN-09: after an epoch that overflowed, the arena that receives the next
// epoch's writes holds the whole observed demand (plus margin), however large
// the overflow was: growth follows demand rather than doubling.
func TestArenaGrowsToObservedDemand(t *testing.T) {
	ar := csp2d.NewArena(10, 10, 1)
	defer ar.Release()
	_, next := ar.GenCapacitiesForTest()
	demand := 6 * next
	for used := 0; used < demand; used += 100 {
		_ = ar.GenAllocN(100)
	}
	ar.RotateGenerations()
	if _, next := ar.GenCapacitiesForTest(); next < demand {
		t.Fatalf("next generation arena holds %d ints after an epoch that needed %d", next, demand)
	}
}

// UT-ARN-10: however much demand there is, the arenas never hold more than the
// process-wide budget; the excess is served from the ordinary heap.
func TestArenaGrowthRespectsBudget(t *testing.T) {
	const budget = 256 << 10 // 256 KiB
	before := withArenaBudget(t, budget)
	ar := csp2d.NewArena(10, 10, 1)
	defer ar.Release()
	for epoch := 0; epoch < 10; epoch++ {
		for i := 0; i < 1000; i++ {
			s := ar.GenAllocN(100) // 800 KB per epoch: always over budget
			s = append(s, make([]int, 100)...)
			if len(s) != 100 {
				t.Fatalf("allocation over budget returned len %d", len(s))
			}
		}
		ar.RotateGenerations()
		if got := csp2d.ArenaBytesReserved() - before; got > budget {
			t.Fatalf("epoch %d: arenas hold %d bytes, budget %d", epoch, got, budget)
		}
	}
}

// UT-ARN-11: Release returns every byte the arena held to the budget.
func TestArenaReleaseReturnsBudget(t *testing.T) {
	before := csp2d.ArenaBytesReserved()
	ar := csp2d.NewArena(50, 100, 5)
	_ = ar.GenAllocN(100000) // overflow, so the arena grows
	ar.RotateGenerations()
	ar.RebuildEliteArena(5) // replaces the elite arena, as every epoch does
	if csp2d.ArenaBytesReserved() == before {
		t.Fatal("creating an arena reserved nothing")
	}
	ar.Release()
	if got := csp2d.ArenaBytesReserved(); got != before {
		t.Fatalf("after Release %d bytes remain reserved (before: %d)", got-before, before)
	}
}
