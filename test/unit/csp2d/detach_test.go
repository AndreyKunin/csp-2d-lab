//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"math/rand"
	"slices"
	"testing"
)

// UT-DET-01: DetachIndividual (cycles and championship, ga.go) returns a copy
// whose postfix is not arena memory: it is unchanged when the original's
// buffer is overwritten and when the generation arenas are reused, and
// CopyIndividual brings it back as an independent copy.
func TestDetachIndividualSurvivesArenaReuse(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	const n = 8
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 10, 2)

	ind, err := p.NewRandomIndividual(rnd)
	if err != nil || ind == nil {
		t.Fatalf("random individual: %v", err)
	}
	orig := csp2d.AsIndividualForTest(ind).PostfixForTest()
	want := slices.Clone(orig)

	detached := csp2d.AsIndividualForTest(p.DetachIndividual(ind))
	if !slices.Equal(detached.PostfixForTest(), want) {
		t.Fatalf("detached postfix %v, want %v", detached.PostfixForTest(), want)
	}

	// The original's buffer is overwritten: the detached copy does not change.
	for i := range orig {
		orig[i] = -99
	}
	if !slices.Equal(detached.PostfixForTest(), want) {
		t.Fatal("detached copy shares the original's memory")
	}

	// The generation arenas are reused (two rotations, then fresh allocations).
	for round := 0; round < 3; round++ {
		p.RotateGenerations()
		for i := 0; i < 20; i++ {
			if _, err := p.NewRandomIndividual(rnd); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !slices.Equal(detached.PostfixForTest(), want) {
		t.Fatalf("detached postfix changed after arena reuse: %v", detached.PostfixForTest())
	}
	if err := validatePostfix(detached.PostfixForTest(), n); err != nil {
		t.Fatal(err)
	}

	// CopyIndividual brings it back: same postfix, its own memory, fitness not
	// cached (the population computes it).
	back := csp2d.AsIndividualForTest(p.CopyIndividual(detached))
	if !slices.Equal(back.PostfixForTest(), want) {
		t.Fatalf("copied-back postfix %v, want %v", back.PostfixForTest(), want)
	}
	if back.IsFitnessComputed() {
		t.Fatal("copied-back individual claims a computed fitness")
	}
	back.PostfixForTest()[0] = -99
	if !slices.Equal(detached.PostfixForTest(), want) {
		t.Fatal("copied-back individual shares the detached memory")
	}
}

// UT-DET-02: detaching keeps nothing of the fitness cache, so a detached
// individual brought back is evaluated afresh and gets the same fitness.
func TestDetachedIndividualReevaluates(t *testing.T) {
	rnd := rand.New(rand.NewSource(2))
	p := newProblem(t, distinctParts(6, rnd), sheet100, 0, 10, 2)
	ind, _ := p.NewRandomIndividual(rnd)
	orig := csp2d.AsIndividualForTest(ind)
	if err := p.EvaluateFitnessForTest(orig); err != nil {
		t.Fatal(err)
	}
	back := csp2d.AsIndividualForTest(p.CopyIndividual(p.DetachIndividual(ind)))
	if err := p.EvaluateFitnessForTest(back); err != nil {
		t.Fatal(err)
	}
	if back.FitnessVectorForTest() != orig.FitnessVectorForTest() {
		t.Fatalf("fitness %v after detach and copy, want %v", back.FitnessVectorForTest(), orig.FitnessVectorForTest())
	}
}
