//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"math/rand"
	"strconv"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func toIDs(labels []int) []int {
	ids := make([]int, len(labels))
	for i, l := range labels {
		ids[i] = l - 1
	}
	return ids
}

// UT-PMX-01: the textbook example (Goldberg & Lingle), segment = leaf
// positions 3..6.
//
//	p1 = 1 2 3 | 4 5 6 7 | 8 9      c1 = 4 2 3 | 1 8 7 6 | 5 9
//	p2 = 4 5 2 | 1 8 7 6 | 9 3      c2 = 1 8 2 | 4 5 6 7 | 9 3
func TestPMXTextbookExample(t *testing.T) {
	p := newProblem(t, distinctParts(9, rand.New(rand.NewSource(1))), sheet100, 0, 4, 1)
	p1 := chainPostfix(toIDs([]int{1, 2, 3, 4, 5, 6, 7, 8, 9}), csp2d.NumS)
	p2 := chainPostfix(toIDs([]int{4, 5, 2, 1, 8, 7, 6, 9, 3}), csp2d.NumL)

	c1, c2, err := p.PMXWithPointsForTest(csp2d.NewIndividualForTest(p1), csp2d.NewIndividualForTest(p2), 3, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := leafOrder(c1.PostfixForTest()), toIDs([]int{4, 2, 3, 1, 8, 7, 6, 5, 9}); !equalInts(got, want) {
		t.Errorf("child 1 leaves %v, want %v", got, want)
	}
	if got, want := leafOrder(c2.PostfixForTest()), toIDs([]int{1, 8, 2, 4, 5, 6, 7, 9, 3}); !equalInts(got, want) {
		t.Errorf("child 2 leaves %v, want %v", got, want)
	}
	// Each child keeps its own parent's operator skeleton.
	if !equalInts(operatorSkeleton(c1.PostfixForTest()), operatorSkeleton(p1)) {
		t.Error("child 1 does not keep parent 1's skeleton")
	}
	if !equalInts(operatorSkeleton(c2.PostfixForTest()), operatorSkeleton(p2)) {
		t.Error("child 2 does not keep parent 2's skeleton")
	}
}

// UT-PMX-03: identical parents produce clones.
func TestPMXIdenticalParents(t *testing.T) {
	rnd := rand.New(rand.NewSource(3))
	p := newProblem(t, distinctParts(12, rnd), sheet100, 0, 4, 1)
	parent := randomPostfix(rnd, 12)
	c1, c2, _, _, err := p.PMXForTest(csp2d.NewIndividualForTest(parent), csp2d.NewIndividualForTest(parent), rnd)
	if err != nil {
		t.Fatal(err)
	}
	if !equalInts(c1.PostfixForTest(), parent) || !equalInts(c2.PostfixForTest(), parent) {
		t.Fatal("PMX of identical parents must return clones")
	}
}

// UT-PMX-P1..P4 (property): children are valid permutations, inherit their
// parent's skeleton, carry the other parent's segment, and parents are not
// modified.
func TestPMXProperties(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 20261117, 300))
	properties.Property("PMX children are valid and structured", prop.ForAll(
		func(n int, seed int64) string {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 4, 1)
			a, b := randomPostfix(rnd, n), randomPostfix(rnd, n)
			aCopy, bCopy := copyInts(a), copyInts(b)
			left := rnd.Intn(n)
			right := left + rnd.Intn(n-left)
			c1, c2, err := p.PMXWithPointsForTest(csp2d.NewIndividualForTest(a), csp2d.NewIndividualForTest(b), left, right)
			if err != nil {
				return err.Error()
			}
			for i, c := range []*csp2d.Csp2dIndividualImpl{c1, c2} {
				if err := validatePostfix(c.PostfixForTest(), n); err != nil {
					return "child " + strconv.Itoa(i+1) + ": " + err.Error()
				}
			}
			if !equalInts(operatorSkeleton(c1.PostfixForTest()), operatorSkeleton(a)) ||
				!equalInts(operatorSkeleton(c2.PostfixForTest()), operatorSkeleton(b)) {
				return "skeleton not inherited"
			}
			o1, ob := leafOrder(c1.PostfixForTest()), leafOrder(b)
			if !equalInts(o1[left:right+1], ob[left:right+1]) {
				return "child 1 does not carry parent 2's segment"
			}
			if !equalInts(a, aCopy) || !equalInts(b, bCopy) {
				return "parents were modified"
			}
			return ""
		},
		gen.IntRange(2, 40).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}
