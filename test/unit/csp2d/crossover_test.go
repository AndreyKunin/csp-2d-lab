//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"math/rand"
	"testing"
)

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

// UT-CX-03: two identical protected parents of 2 parts always give protected
// children (the only donor subtree is the whole tree, so each child equals the
// other parent), so Crossover falls back to two random individuals and counts
// 2 failures.
func TestCrossoverFallback(t *testing.T) {
	rnd := rand.New(rand.NewSource(12))
	p := newProblem(t, distinctParts(2, rnd), sheet100, 0, 16, 2)
	elite := randomPostfix(rnd, 2)
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
