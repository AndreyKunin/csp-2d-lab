//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"csp2d/test/support/testutil"
	"math/rand"
	"testing"
	"time"
)

// UT-PMX-02: repairs that need multi-step mapping chains.
//
//	p1 = 1 | 2 3 4 | 5    mapping1: 4→2, 5→3, 1→4   c1 = 2 4 5 1 3 (1→4→2)
//	p2 = 3 | 4 5 1 | 2    mapping2: 2→4, 3→5, 4→1   c2 = 5 2 3 4 1 (2→4→1)
func TestPMXMappingChains(t *testing.T) {
	p := newProblem(t, distinctParts(5, rand.New(rand.NewSource(1))), sheet100, 0, 4, 1)
	p1 := chainPostfix(toIDs([]int{1, 2, 3, 4, 5}), csp2d.NumS)
	p2 := chainPostfix(toIDs([]int{3, 4, 5, 1, 2}), csp2d.NumS)
	c1, c2, err := p.PMXWithPointsForTest(csp2d.NewIndividualForTest(p1), csp2d.NewIndividualForTest(p2), 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := leafOrder(c1.PostfixForTest()), toIDs([]int{2, 4, 5, 1, 3}); !equalInts(got, want) {
		t.Errorf("child 1 %v, want %v", got, want)
	}
	if got, want := leafOrder(c2.PostfixForTest()), toIDs([]int{5, 2, 3, 4, 1}); !equalInts(got, want) {
		t.Errorf("child 2 %v, want %v", got, want)
	}
}

// UT-PMX-04: a segment spanning all leaves swaps the leaf orders.
func TestPMXWholeRange(t *testing.T) {
	rnd := rand.New(rand.NewSource(9))
	p := newProblem(t, distinctParts(8, rnd), sheet100, 0, 4, 1)
	a, b := randomPostfix(rnd, 8), randomPostfix(rnd, 8)
	c1, c2, err := p.PMXWithPointsForTest(csp2d.NewIndividualForTest(a), csp2d.NewIndividualForTest(b), 0, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !equalInts(leafOrder(c1.PostfixForTest()), leafOrder(b)) || !equalInts(leafOrder(c2.PostfixForTest()), leafOrder(a)) {
		t.Fatal("whole-range PMX must swap the leaf orders")
	}
}

// UT-PMX-05: two leaves.
func TestPMXTwoLeaves(t *testing.T) {
	rnd := rand.New(rand.NewSource(10))
	p := newProblem(t, distinctParts(2, rnd), sheet100, 0, 4, 1)
	for i := 0; i < 50; i++ {
		c1, c2, _, _, err := p.PMXForTest(
			csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumS}), csp2d.NewIndividualForTest([]int{1, 0, csp2d.NumL}), rnd)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []*csp2d.Csp2dIndividualImpl{c1, c2} {
			if err := validatePostfix(c.PostfixForTest(), 2); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// UT-PMX-06: a single leaf must not hang (point selection needs two
// distinct points).
func TestPMXSingleLeafTerminates(t *testing.T) {
	testutil.Pending(t, "pmx loops forever choosing a second distinct point when there is one leaf")
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 4, 1)
	one := csp2d.NewIndividualForTest([]int{0})
	testutil.Within(t, 2*time.Second, "pmx with one leaf", func() {
		_, _, _, _, _ = p.PMXForTest(one, one, rand.New(rand.NewSource(1)))
	})
}

func individuals(postfixes ...[]int) []*problem.Individual {
	out := make([]*problem.Individual, len(postfixes))
	for i, pf := range postfixes {
		var ind problem.Individual = csp2d.NewIndividualForTest(pf)
		out[i] = &ind
	}
	return out
}

// UT-CX-01 / UT-CX-02: offspring count, validity and protection.
func TestCrossoverCountAndProtection(t *testing.T) {
	rnd := rand.New(rand.NewSource(11))
	p := newProblem(t, distinctParts(12, rnd), sheet100, 0, 16, 2)
	var elites []problem.Individual
	for i := 0; i < 3; i++ {
		elites = append(elites, csp2d.NewIndividualForTest(randomPostfix(rnd, 12)))
	}
	p.Protect(elites)
	parents := make([][]int, 0, 7)
	for i := 0; i < 7; i++ {
		parents = append(parents, randomPostfix(rnd, 12))
	}
	parents[0] = copyInts(csp2d.AsIndividualForTest(elites[0]).PostfixForTest()) // an elite may be a parent
	offspring, _, err := p.Crossover(individuals(parents...), rnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(offspring) != 6 {
		t.Fatalf("%d offspring from 7 parents, want 6", len(offspring))
	}
	for i, o := range offspring {
		postfix := csp2d.AsIndividualForTest(*o).PostfixForTest()
		if err := validatePostfix(postfix, 12); err != nil {
			t.Fatalf("offspring %d: %v", i, err)
		}
		if p.IsProtectedForTest(postfix) {
			t.Fatalf("offspring %d is a protected genotype", i)
		}
	}
}

// UT-CX-03: two identical protected parents always give protected children,
// so Crossover falls back to two random individuals and counts 2 failures.
func TestCrossoverFallback(t *testing.T) {
	rnd := rand.New(rand.NewSource(12))
	p := newProblem(t, distinctParts(10, rnd), sheet100, 0, 16, 2)
	elite := randomPostfix(rnd, 10)
	p.Protect([]problem.Individual{csp2d.NewIndividualForTest(elite)})
	offspring, failures, err := p.Crossover(individuals(elite, copyInts(elite)), rnd)
	if err != nil {
		t.Fatal(err)
	}
	if failures != 2 || len(offspring) != 2 {
		t.Fatalf("failures %d, offspring %d; want 2 and 2", failures, len(offspring))
	}
	for _, o := range offspring {
		if p.IsProtectedForTest(csp2d.AsIndividualForTest(*o).PostfixForTest()) {
			t.Fatal("fallback produced a protected genotype")
		}
	}
}

// UT-CX-04: when even random individuals are exhausted, Crossover returns an
// error instead of panicking.
func TestCrossoverExhausted(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}, {"2", 20, 20}}, sheet100, 0, 16, 4)
	all := [][]int{{0, 1, csp2d.NumS}, {0, 1, csp2d.NumL}, {1, 0, csp2d.NumS}, {1, 0, csp2d.NumL}}
	var protected []problem.Individual
	for _, pf := range all {
		protected = append(protected, csp2d.NewIndividualForTest(pf))
	}
	p.Protect(protected)
	_, _, err := p.Crossover(individuals(all[0], copyInts(all[0])), rand.New(rand.NewSource(1)))
	if err == nil {
		t.Fatal("expected an error when the gene pool is exhausted")
	}
}
