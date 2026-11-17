//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"math/rand"
	"runtime"
	"testing"
)

// MEM-04: the per-call working buffers of the hot paths live in the Arena, so
// trials and rejected candidates allocate nothing on the heap. (A local make
// of non-constant size above 32 bytes is heap-allocated even when escape
// analysis reports that it does not escape.)

// UT-ARN-22: decoding a trial in scratch allocates nothing.
func TestDecodeTrialAllocatesNothing(t *testing.T) {
	const n = 60
	rnd := rand.New(rand.NewSource(51))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0.3, 16, 2)
	postfix := randomPostfix(rnd, n)
	if allocs := testing.AllocsPerRun(50, func() { _, _ = p.DecodeForestForTest(postfix) }); allocs != 0 {
		t.Fatalf("decode allocates %.0f times per call, want 0", allocs)
	}
}

// UT-ARN-23: PMX allocates only the two child individuals.
func TestPMXAllocatesOnlyChildren(t *testing.T) {
	const n = 60
	rnd := rand.New(rand.NewSource(52))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	a := csp2d.NewIndividualForTest(randomPostfix(rnd, n))
	b := csp2d.NewIndividualForTest(randomPostfix(rnd, n))
	if allocs := testing.AllocsPerRun(50, func() { _, _, _ = p.PMXWithPointsForTest(a, b, 5, 40) }); allocs > 2 {
		t.Fatalf("PMX allocates %.0f times per call, want at most 2 (the children)", allocs)
	}
}

// UT-ARN-24: ranking the critical operators allocates nothing.
func TestRankCriticalOperatorsAllocatesNothing(t *testing.T) {
	const n = 60
	rnd := rand.New(rand.NewSource(53))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	postfix := randomPostfix(rnd, n)
	if allocs := testing.AllocsPerRun(50, func() { _ = p.RankCriticalOperatorsForTest(postfix) }); allocs != 0 {
		t.Fatalf("ranking allocates %.0f times per call, want 0", allocs)
	}
}

// UT-ARN-25: an eliminate-and-repair attempt that does not improve allocates
// nothing.
func TestRepairWithoutImprovementAllocatesNothing(t *testing.T) {
	// Ten full sheets: a removed part fits nowhere else, so no order improves.
	parts := make([]part, 10)
	for i := range parts {
		parts[i] = part{"1", 100, 100}
	}
	p := newProblem(t, parts, sheet100, 0, 16, 2)
	parent := evaluated(t, p, chainPostfix([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, S))
	if sheets := len(parent.ForestForTest()); sheets != 10 {
		t.Fatalf("setup: %d sheets, want 10", sheets)
	}
	rnd := rand.New(rand.NewSource(54))
	allocs := testing.AllocsPerRun(50, func() {
		if _, improved, err := p.MutateEliminateAndRepairForTest(parent, rnd); err != nil || improved {
			t.Fatalf("improved %v err %v", improved, err)
		}
	})
	if allocs != 0 {
		t.Fatalf("a non-improving repair allocates %.0f times per call, want 0", allocs)
	}
}

// UT-ARN-26: a Mutation call at the default rate (60 parts: about 3 accepted
// mutations per individual, mostly repairs) allocates at most 2 postfix lengths
// of heap per individual: the kept children's structs and forests, not
// per-attempt working buffers.
func TestMutationHeapPerIndividual(t *testing.T) {
	const size = 200
	p, n := fixtureProblem(t, "D14-medium-60.json", 2000, 10)
	rnd := rand.New(rand.NewSource(3))
	set := make([]*problem.Individual, size)
	for i := range set {
		ind := evaluated(t, p, randomPostfix(rnd, n))
		var pi problem.Individual = ind
		set[i] = &pi
	}
	_, _, _, _ = p.Mutation(0.0001, set[:1], rand.New(rand.NewSource(1))) // warm up the scratch buffers
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	count, _, _, err := p.Mutation(0.07, set, rnd)
	runtime.ReadMemStats(&m1)
	if err != nil {
		t.Fatal(err)
	}
	perIndividual := float64(m1.TotalAlloc-m0.TotalAlloc) / 8 / float64(size*(2*n-1))
	t.Logf("%d accepted mutations; heap %.2f postfix lengths per individual", count, perIndividual)
	if perIndividual > 2 {
		t.Fatalf("Mutation allocated %.2f postfix lengths of heap per individual, want at most 2", perIndividual)
	}
}

// UT-ARN-27: rejected PMX attempts allocate nothing: with identical protected
// parents all 128 attempts per pair are rejected, and Crossover's allocations
// do not grow with them.
func TestCrossoverRejectedAttemptsAllocateNothing(t *testing.T) {
	const n = 10
	rnd := rand.New(rand.NewSource(55))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	elite := randomPostfix(rnd, n)
	p.Protect([]problem.Individual{csp2d.NewIndividualForTest(elite)})
	allocs := testing.AllocsPerRun(20, func() {
		if _, failures, err := p.Crossover(individuals(elite, copyInts(elite)), rnd); err != nil || failures != 2 {
			t.Fatalf("failures %d err %v", failures, err)
		}
	})
	// What remains is the two fallback random individuals and the result slice.
	if allocs >= 128 {
		t.Fatalf("crossover allocates %.0f times for 128 rejected attempts", allocs)
	}
	t.Logf("%.0f allocations per crossover with 128 rejected attempts", allocs)
}
