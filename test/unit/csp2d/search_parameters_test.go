//go:build testhooks

package csp2d_test

import (
	"math"
	"math/rand"
	"testing"
)

// UT-SP-01: a new problem has the default search parameters; SetSearchParameters
// sets valid values and replaces out-of-range ones with the defaults (a weight
// below 1 would never use up the mutation budget).
func TestSearchParametersDefaultsAndRange(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	p := newProblem(t, distinctParts(6, rnd), sheet100, 0, 16, 2)
	check := func(name string, repair, flip int, donor float64) {
		t.Helper()
		r, f, d := p.SearchParametersForTest()
		if r != repair || f != flip || d != donor {
			t.Fatalf("%s: repair %d, flip %d, donor %v; want %d, %d, %v", name, r, f, d, repair, flip, donor)
		}
	}
	check("new problem", 3, 1, 0.5)
	p.SetSearchParameters(6, 2, 0.75)
	check("valid values", 6, 2, 0.75)
	p.SetSearchParameters(1, 1, 1)
	check("smallest weights, largest fraction", 1, 1, 1)
	for _, c := range []struct {
		repair, flip int
		donor        float64
	}{{0, 0, 0}, {-4, -1, -0.5}, {0, 0, 1.5}, {0, 0, math.NaN()}} {
		p.SetSearchParameters(c.repair, c.flip, c.donor)
		check("out of range", 3, 1, 0.5)
	}
}

// UT-SP-02: the repair and flip weights are the budget cost of an accepted
// mutation: with heavier weights the same budget buys fewer mutations, and the
// spent weight still covers the budget.
func TestSearchParametersWeightsSpendTheBudget(t *testing.T) {
	const n, size, rate = 12, 10, 0.2
	budget := int(math.Ceil(float64((2*n-1)*size) * rate))
	run := func(repair, flip int) (count, weight int) {
		rnd := rand.New(rand.NewSource(17))
		p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 64, 2)
		p.SetSearchParameters(repair, flip, 0.5)
		set, _ := mutationSet(rnd, n, size)
		c, w, _, err := p.Mutation(rate, set, rnd)
		if err != nil {
			t.Fatal(err)
		}
		return c, w
	}
	lightCount, lightWeight := run(3, 1)
	heavyCount, heavyWeight := run(9, 3)
	if heavyCount >= lightCount {
		t.Fatalf("weights 9/3 made %d mutations, weights 3/1 made %d; want fewer", heavyCount, lightCount)
	}
	if lightWeight < budget || lightWeight >= budget+3 || heavyWeight < budget || heavyWeight >= budget+9 {
		t.Fatalf("spent %d (weights 3/1) and %d (weights 9/3) of budget %d", lightWeight, heavyWeight, budget)
	}
}

// UT-SP-03: the donor fraction caps the donor subtree of the crossover at
// max(2, fraction·leaves) leaves.
func TestSearchParametersDonorFractionCapsTheDonor(t *testing.T) {
	const n = 20
	rnd := rand.New(rand.NewSource(3))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	largest := func(fraction float64) int {
		p.SetSearchParameters(3, 1, fraction)
		most := 0
		for i := 0; i < 400; i++ {
			postfix := randomPostfix(rnd, n)
			start, end := p.PickDonorSubtreeForTest(postfix, rnd)
			leaves := 0
			for _, id := range postfix[start : end+1] {
				if id >= 0 {
					leaves++
				}
			}
			most = max(most, leaves)
		}
		return most
	}
	if got := largest(0.25); got > 5 {
		t.Fatalf("fraction 0.25: a donor with %d leaves, cap is 5", got)
	}
	if got := largest(0.1); got > 2 {
		t.Fatalf("fraction 0.1: a donor with %d leaves, cap is max(2, 2) = 2", got)
	}
	if got := largest(1); got <= 5 {
		t.Fatalf("fraction 1: largest donor %d leaves in 400 picks; want above the 0.25 cap of 5", got)
	}
}
