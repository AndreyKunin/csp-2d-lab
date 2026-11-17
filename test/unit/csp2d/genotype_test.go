//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// newRandom draws one random individual and fails on error or nil.
func newRandom(t testing.TB, p *csp2d.Csp2dProblemImpl, rnd *rand.Rand) *csp2d.Csp2dIndividualImpl {
	t.Helper()
	ind, err := p.NewRandomIndividual(rnd)
	if err != nil || ind == nil {
		t.Fatalf("NewRandomIndividual: %v, %v", ind, err)
	}
	return csp2d.AsIndividualForTest(ind)
}

// UT-RND-01 / UT-RND-04 (property): random genotypes are valid, n = 1..60.
func TestRandomIndividualIsValid(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 505, 300))
	properties.Property("valid postfix", prop.ForAll(
		func(n int, seed int64) string {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 4, 1)
			ind, err := p.NewRandomIndividual(rnd)
			if err != nil || ind == nil {
				return fmt.Sprintf("no individual: %v", err)
			}
			if err := validatePostfix(csp2d.AsIndividualForTest(ind).PostfixForTest(), n); err != nil {
				return err.Error()
			}
			return ""
		},
		gen.IntRange(1, 60).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}

// shapeKey is the leaf/operator pattern of a postfix ("LLOLO...").
func shapeKey(p []int) string {
	var b strings.Builder
	for _, v := range p {
		if v >= 0 {
			b.WriteByte('L')
		} else {
			b.WriteByte('O')
		}
	}
	return b.String()
}

// UT-RND-02: for n = 4 all 5 tree shapes appear, and every operator slot is
// seen with both operators.
func TestRandomIndividualShapeCoverage(t *testing.T) {
	rnd := rand.New(rand.NewSource(606))
	p := newProblem(t, distinctParts(4, rnd), sheet100, 0, 64, 1)
	shapes := map[string]bool{}
	ops := map[string]map[int]bool{} // shape|position -> operators seen
	for i := 0; i < 5000; i++ {
		postfix := newRandom(t, p, rnd).PostfixForTest()
		key := shapeKey(postfix)
		shapes[key] = true
		for pos, v := range postfix {
			if v < 0 {
				k := fmt.Sprintf("%s|%d", key, pos)
				if ops[k] == nil {
					ops[k] = map[int]bool{}
				}
				ops[k][v] = true
			}
		}
		if i%500 == 499 {
			p.RotateGenerations() // keep the arena within budget
			p.RotateGenerations()
		}
	}
	if len(shapes) != 5 {
		t.Errorf("saw %d shapes, want 5 (Catalan(3)): %v", len(shapes), shapes)
	}
	for k, seen := range ops {
		if !seen[csp2d.NumS] || !seen[csp2d.NumL] {
			t.Errorf("slot %s never had both operators: %v", k, seen)
		}
	}
}

// UT-RND-03: when every genotype is protected, NewRandomIndividual returns
// nil (no error) after its retries.
func TestRandomIndividualExhausted(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 4, 1)
	p.Protect([]problem.Individual{csp2d.NewIndividualForTest([]int{0})})
	ind, err := p.NewRandomIndividual(rand.New(rand.NewSource(1)))
	if err != nil {
		t.Fatal(err)
	}
	if ind != nil {
		t.Fatalf("expected nil, got %v", csp2d.AsIndividualForTest(ind).PostfixForTest())
	}
}

// UT-POP-01: the initial population has the requested size, pairwise
// distinct genotypes, and leaves no protection behind.
func TestNewPopulation(t *testing.T) {
	rnd := rand.New(rand.NewSource(707))
	p := newProblem(t, distinctParts(10, rnd), sheet100, 0, 50, 5)
	if err := p.NewPopulation(50, 5, rnd); err != nil {
		t.Fatal(err)
	}
	species := *p.GetPopulation().Species
	if len(species) != 50 {
		t.Fatalf("size %d, want 50", len(species))
	}
	keys := map[string]bool{}
	for i, ind := range species {
		postfix := csp2d.AsIndividualForTest(ind).PostfixForTest()
		if err := validatePostfix(postfix, 10); err != nil {
			t.Fatalf("individual %d: %v", i, err)
		}
		key := p.NumLabelKeyForTest(postfix)
		if keys[key] {
			t.Fatalf("individual %d duplicates an earlier genotype", i)
		}
		keys[key] = true
		if p.IsProtectedForTest(postfix) {
			t.Fatalf("individual %d is still protected after NewPopulation", i)
		}
	}
}

// UT-POP-02: a gene space smaller than the population. 3 distinct parts have
// 3!·Catalan(2)·2² = 48 genotypes; the current behaviour is an error (the
// required behaviour is a capped population, see FT-EDGE-03).
func TestNewPopulationSmallGeneSpace(t *testing.T) {
	p := newProblem(t, []part{{"1", 50, 50}, {"2", 50, 40}, {"3", 30, 30}}, sheet100, 0, 100, 5)
	err := p.NewPopulation(100, 5, rand.New(rand.NewSource(1)))
	if err == nil {
		species := *p.GetPopulation().Species
		for i, ind := range species {
			if err := validatePostfix(csp2d.AsIndividualForTest(ind).PostfixForTest(), 3); err != nil {
				t.Fatalf("individual %d: %v", i, err)
			}
		}
		return
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// UT-KEY-01: keys identify genotypes modulo identical labels.
func TestNumLabelKey(t *testing.T) {
	// ids 0 and 1 share label "1"; id 2 is label "2".
	p := newProblem(t, []part{{"1", 10, 20}, {"1", 10, 20}, {"2", 30, 30}}, sheet100, 0, 4, 1)
	key := p.NumLabelKeyForTest
	a := []int{0, 1, csp2d.NumS, 2, csp2d.NumL}
	if key(a) != key([]int{1, 0, csp2d.NumS, 2, csp2d.NumL}) {
		t.Error("swapping parts with the same label must give the same key")
	}
	if key(a) == key([]int{0, 1, csp2d.NumL, 2, csp2d.NumL}) {
		t.Error("a different operator must give a different key")
	}
	if key(a) == key([]int{0, 2, csp2d.NumS, 1, csp2d.NumL}) {
		t.Error("moving a different label must give a different key")
	}
	if key([]int{0}) == key([]int{0, 1, csp2d.NumS}) {
		t.Error("different lengths must give different keys")
	}
}

// UT-KEY-02: Protect replaces the protected set.
func TestProtect(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 20}, {"2", 30, 30}}, sheet100, 0, 4, 1)
	a := csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumS})
	b := csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumL})
	p.Protect([]problem.Individual{a})
	if !p.IsProtectedForTest(a.PostfixForTest()) || p.IsProtectedForTest(b.PostfixForTest()) {
		t.Fatal("protected set should be exactly {a}")
	}
	p.Protect([]problem.Individual{b})
	if p.IsProtectedForTest(a.PostfixForTest()) || !p.IsProtectedForTest(b.PostfixForTest()) {
		t.Fatal("Protect must replace the previous set")
	}
}

// UT-KEY-03: a returned key is not affected by later scratch reuse.
func TestNumLabelKeyIsIndependent(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 20}, {"2", 30, 30}, {"3", 5, 5}}, sheet100, 0, 4, 1)
	first := []int{0, 1, csp2d.NumS, 2, csp2d.NumL}
	k1 := p.NumLabelKeyForTest(first)
	saved := strings.Clone(k1)
	_ = p.NumLabelKeyForTest([]int{2, 1, csp2d.NumL, 0, csp2d.NumS})
	if k1 != saved || k1 != p.NumLabelKeyForTest(first) {
		t.Fatal("key changed after the scratch buffer was reused")
	}
}
