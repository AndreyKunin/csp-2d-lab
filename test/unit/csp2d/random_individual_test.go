//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

// referenceRandomPostfix is the fragment-merging construction NewRandomIndividual
// used before PERF-01, kept as an oracle. It draws from rnd exactly as the
// production code does (operators, then one shuffle), so on the same seed both
// must produce the same postfix.
func referenceRandomPostfix(rnd *rand.Rand, n int) []int {
	postfixLen := 2*n - 1
	shuffled := make([]int, postfixLen)
	for i := 0; i < n; i++ {
		shuffled[i] = i
	}
	for i := n; i < postfixLen; i++ {
		if rnd.Intn(csp2d.OpsCount) == int(csp2d.S) {
			shuffled[i] = csp2d.NumS
		} else {
			shuffled[i] = csp2d.NumL
		}
	}
	rnd.Shuffle(postfixLen, func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	var complete [][]int
	var pending []int
	for _, e := range shuffled {
		if e < 0 {
			pending = append(pending, e)
		} else {
			complete = append(complete, []int{e})
		}
		for len(complete) > 1 && len(pending) > 0 {
			op := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			right, left := complete[len(complete)-1], complete[len(complete)-2]
			complete = complete[:len(complete)-2]
			merged := append(append(append([]int{}, left...), right...), op)
			complete = append(complete, merged)
		}
	}
	return complete[0]
}

// UT-RND-05: NewRandomIndividual produces exactly the postfix of the
// fragment-merging construction on the same random stream (PERF-01): the
// distribution of random genotypes is unchanged. Many draws per problem, so the
// reuse of scratch buffers between calls is covered too.
func TestRandomIndividualMatchesReference(t *testing.T) {
	for _, n := range []int{1, 2, 3, 4, 7, 20, 100, 250} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			parts := distinctParts(n, rand.New(rand.NewSource(int64(n))))
			p := newProblem(t, parts, sheet100, 0, 64, 4)
			for seed := int64(0); seed < 50; seed++ {
				prod := rand.New(rand.NewSource(seed))
				ref := rand.New(rand.NewSource(seed))
				for draw := 0; draw < 5; draw++ {
					got := slices.Clone(newRandom(t, p, prod).PostfixForTest())
					want := referenceRandomPostfix(ref, n)
					if !slices.Equal(got, want) {
						t.Fatalf("seed %d draw %d:\n got  %v\n want %v", seed, draw, got, want)
					}
					if err := validatePostfix(got, n); err != nil {
						t.Fatalf("seed %d draw %d: %v", seed, draw, err)
					}
				}
				p.RotateGenerations() // keep the arena within budget
				p.RotateGenerations()
			}
		})
	}
}

// UT-RND-06: consecutive random individuals are distinct and each keeps its own
// genes: a later call does not overwrite an earlier individual's postfix
// (the construction must not hand out its scratch buffer).
func TestRandomIndividualsAreDistinctAndStable(t *testing.T) {
	const n, count = 30, 500
	rnd := rand.New(rand.NewSource(808))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, count, 4)
	var inds []*csp2d.Csp2dIndividualImpl
	var copies [][]int
	seen := map[string]bool{}
	for i := 0; i < count; i++ {
		ind := newRandom(t, p, rnd)
		inds = append(inds, ind)
		copies = append(copies, slices.Clone(ind.PostfixForTest()))
		key := fmt.Sprint(ind.PostfixForTest())
		if seen[key] {
			t.Fatalf("draw %d repeats an earlier genotype: %v", i, key)
		}
		seen[key] = true
	}
	for i, ind := range inds {
		if !slices.Equal(ind.PostfixForTest(), copies[i]) {
			t.Fatalf("individual %d changed after later draws:\n now  %v\n was  %v", i, ind.PostfixForTest(), copies[i])
		}
		if err := validatePostfix(ind.PostfixForTest(), n); err != nil {
			t.Fatalf("individual %d: %v", i, err)
		}
	}
}
