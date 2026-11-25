//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// GA-01: crossover must recombine structure. A building block of a slicing
// tree is a subtree (its parts, shape and operators); in a postfix it is a
// contiguous segment.

// subtreeKeys returns the subtrees with at least 2 leaves of a postfix, as
// segment keys. It is independent of the production code.
func subtreeKeys(p []int) map[string]int {
	keys := map[string]int{}
	var starts []int // start index of each pending subtree
	for i, v := range p {
		if v >= 0 {
			starts = append(starts, i)
			continue
		}
		start := starts[len(starts)-2]
		starts = append(starts[:len(starts)-2], start)
		keys[fmt.Sprint(p[start:i+1])] = (i - start + 2) / 2 // leaves
	}
	return keys
}

// structureShare describes how much of child is inherited verbatim.
type structureShare struct {
	fromA, fromB int // subtrees found in only that parent
	inherited    int // subtrees found in either parent
	total        int // subtrees of the child (internal nodes)
}

func shareOf(child, a, b []int) structureShare {
	ka, kb := subtreeKeys(a), subtreeKeys(b)
	var s structureShare
	for key := range subtreeKeys(child) {
		_, inA := ka[key]
		_, inB := kb[key]
		s.total++
		if inA || inB {
			s.inherited++
		}
		if inA && !inB {
			s.fromA++
		}
		if inB && !inA {
			s.fromB++
		}
	}
	return s
}

// UT-CX-05: each child of two unrelated parents carries a building block
// (subtree of >= 2 parts) from each parent, and most of its subtrees are
// inherited verbatim. Measured through Crossover with one pair per call, so both
// children come from the known parents.
func TestCrossoverRecombinesStructure(t *testing.T) {
	const n, pairs = 20, 300
	rnd := rand.New(rand.NewSource(20261117))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 16, 2)
	both, inherited, total := 0, 0, 0
	randomInherited, randomTotal := 0, 0 // baseline: an unrelated random genotype
	for i := 0; i < pairs; i++ {
		a, b := randomPostfix(rnd, n), randomPostfix(rnd, n)
		offspring, failures, err := p.Crossover(individuals(a, b), rnd)
		if err != nil || failures != 0 || len(offspring) != 2 {
			t.Fatalf("pair %d: offspring %d failures %d err %v", i, len(offspring), failures, err)
		}
		for _, o := range offspring {
			c := csp2d.AsIndividualForTest(*o).PostfixForTest()
			if err := validatePostfix(c, n); err != nil {
				t.Fatalf("pair %d: %v", i, err)
			}
			s := shareOf(c, a, b)
			if s.fromA > 0 && s.fromB > 0 {
				both++
			}
			inherited += s.inherited
			total += s.total
		}
		r := shareOf(randomPostfix(rnd, n), a, b)
		randomInherited += r.inherited
		randomTotal += r.total
		if i%20 == 19 { // keep the generation arena from overflowing
			p.RotateGenerations()
			p.RotateGenerations()
		}
	}
	children := 2 * pairs
	bothRate := float64(both) / float64(children)
	inheritedRate := float64(inherited) / float64(total)
	t.Logf("children with a building block from each parent: %d/%d (%.1f%%); subtrees inherited verbatim: %.1f%% (random genotype: %.1f%%)",
		both, children, 100*bothRate, 100*inheritedRate, 100*float64(randomInherited)/float64(randomTotal))
	if bothRate < 0.9 {
		t.Errorf("only %.1f%% of children carry a building block from each parent, want >= 90%%", 100*bothRate)
	}
	if inheritedRate < 0.4 {
		t.Errorf("only %.1f%% of the children's subtrees are inherited verbatim, want >= 40%%", 100*inheritedRate)
	}
}

// UT-CX-06: removing parts collapses each operator left with one operand.
// ((a S b) L c) S d without b and d is a L c.
func TestRemoveLeavesExample(t *testing.T) {
	p := newProblem(t, distinctParts(4, rand.New(rand.NewSource(1))), sheet100, 0, 4, 1)
	a, b, c, d := 0, 1, 2, 3
	if got, want := p.RemoveLeavesForTest([]int{a, b, S, c, L, d, S}, []int{b, d, L}), []int{a, c, L}; !equalInts(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Both operands of (c L d) removed: the operator above it collapses too.
	if got, want := p.RemoveLeavesForTest([]int{a, b, S, c, d, L, S}, []int{c, d, S}), []int{a, b, S}; !equalInts(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// UT-CX-07: grafting joins the donor to the subtree that ends at k.
func TestGraftAtExample(t *testing.T) {
	a, b, c, d := 0, 1, 2, 3
	rest := []int{a, c, L}
	for _, tc := range []struct {
		k    int
		want []int
	}{
		{0, []int{a, b, d, S, L, c, L}}, // (a L bd) L c
		{1, []int{a, c, b, d, S, L, L}}, // a L (c L bd)
		{2, []int{a, c, L, b, d, S, L}}, // (a L c) L bd: at the root
	} {
		if got := csp2d.GraftAtForTest(rest, []int{b, d, S}, tc.k, L); !equalInts(got, tc.want) {
			t.Errorf("k=%d: got %v, want %v", tc.k, got, tc.want)
		}
	}
}

// validSubPostfix checks that p is a valid postfix over exactly the ids in
// want (an empty p for an empty set).
func validSubPostfix(p []int, want map[int]bool) error {
	depth, seen := 0, 0
	for i, v := range p {
		if v >= 0 {
			if !want[v] {
				return fmt.Errorf("position %d: unexpected id %d", i, v)
			}
			seen++
			depth++
			continue
		}
		if v != S && v != L {
			return fmt.Errorf("position %d: unknown token %d", i, v)
		}
		if depth < 2 {
			return fmt.Errorf("position %d: operator with stack depth %d", i, depth)
		}
		depth--
	}
	if seen != len(want) {
		return fmt.Errorf("%d of %d ids", seen, len(want))
	}
	if seen > 0 && depth != 1 {
		return fmt.Errorf("final stack depth %d", depth)
	}
	return nil
}

// subtreesWithout returns the subtrees (>= 2 leaves) of p that hold none of ids.
func subtreesWithout(p []int, ids map[int]bool) [][]int {
	type pending struct {
		start int
		clean bool
	}
	var out [][]int
	var stack []pending
	for i, v := range p {
		if v >= 0 {
			stack = append(stack, pending{i, !ids[v]})
			continue
		}
		l, r := stack[len(stack)-2], stack[len(stack)-1]
		top := pending{l.start, l.clean && r.clean}
		stack = append(stack[:len(stack)-2], top)
		if top.clean {
			out = append(out, p[top.start:i+1])
		}
	}
	return out
}

// UT-CX-P1 (property): the picked donor is a subtree with 2 .. max(2, n/2)
// parts; removing its parts from the recipient leaves a valid postfix over the
// other parts that keeps every recipient subtree without them verbatim;
// grafting the donor at any position gives a valid postfix that carries it
// verbatim.
func TestSubtreeCrossoverStepsProperties(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 20261117, 300))
	properties.Property("donor, removal and graft are valid", prop.ForAll(
		func(n int, seed int64) string {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 4, 1)
			recipient, donor := randomPostfix(rnd, n), randomPostfix(rnd, n)
			start, end := p.PickDonorSubtreeForTest(donor, rnd)
			segment := donor[start : end+1]
			inSegment := map[int]bool{}
			for _, id := range segment {
				if id >= 0 {
					inSegment[id] = true
				}
			}
			if err := validSubPostfix(segment, inSegment); err != nil {
				return fmt.Sprintf("donor %v is not a subtree: %v", segment, err)
			}
			if parts := len(inSegment); parts < 2 || parts > max(2, n/2) {
				return fmt.Sprintf("donor %v has %d parts, want 2..%d", segment, parts, max(2, n/2))
			}
			others := map[int]bool{}
			for id := 0; id < n; id++ {
				if !inSegment[id] {
					others[id] = true
				}
			}
			rest := p.RemoveLeavesForTest(recipient, segment)
			if err := validSubPostfix(rest, others); err != nil {
				return fmt.Sprintf("rest %v: %v", rest, err)
			}
			restKeys := subtreeKeys(rest)
			for _, sub := range subtreesWithout(recipient, inSegment) {
				if _, ok := restKeys[fmt.Sprint(sub)]; !ok {
					return fmt.Sprintf("recipient subtree %v is not kept verbatim in %v", sub, rest)
				}
			}
			for k := 0; k < len(rest); k++ {
				for _, op := range []int{S, L} {
					child := csp2d.GraftAtForTest(rest, segment, k, op)
					if err := validatePostfix(child, n); err != nil {
						return "graft at " + strconv.Itoa(k) + ": " + err.Error()
					}
					if _, ok := subtreeKeys(child)[fmt.Sprint(segment)]; !ok {
						return "graft at " + strconv.Itoa(k) + ": donor not carried verbatim"
					}
				}
			}
			return ""
		},
		gen.IntRange(3, 40).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}

// UT-CX-P2 (property): subtree crossover gives two valid children (each part
// once, n-1 operators, ballot property), with and without a sheet constraint,
// for unrelated and identical parents, and does not modify the parents.
func TestSubtreeCrossoverProperties(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 20261118, 500))
	properties.Property("children are valid", prop.ForAll(
		func(n int, seed int64, constrained, identical bool) string {
			rnd := rand.New(rand.NewSource(seed))
			sheet := sheet100
			if !constrained {
				sheet = nil
			}
			p := newProblem(t, distinctParts(n, rnd), sheet, 0, 4, 1)
			a, b := randomPostfix(rnd, n), randomPostfix(rnd, n)
			if identical {
				b = copyInts(a)
			}
			aCopy, bCopy := copyInts(a), copyInts(b)
			c1, c2, err := p.SubtreeCrossoverForTest(csp2d.NewIndividualForTest(a), csp2d.NewIndividualForTest(b), rnd)
			if err != nil {
				return err.Error()
			}
			for i, c := range []*csp2d.Csp2dIndividualImpl{c1, c2} {
				if err := validatePostfix(c.PostfixForTest(), n); err != nil {
					return "child " + strconv.Itoa(i+1) + ": " + err.Error()
				}
			}
			if !equalInts(a, aCopy) || !equalInts(b, bCopy) {
				return "parents were modified"
			}
			return ""
		},
		gen.IntRange(1, 40).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
		gen.Bool().WithLabel("constrained"),
		gen.Bool().WithLabel("identical"),
	))
	properties.TestingRun(t)
}

// UT-CX-08: one part: the children are copies, and crossover returns.
func TestSubtreeCrossoverSinglePart(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 4, 1)
	one := csp2d.NewIndividualForTest([]int{0})
	c1, c2, err := p.SubtreeCrossoverForTest(one, one, rand.New(rand.NewSource(1)))
	if err != nil || !equalInts(c1.PostfixForTest(), []int{0}) || !equalInts(c2.PostfixForTest(), []int{0}) {
		t.Fatalf("got %v, %v, %v; want [0], [0], nil", c1, c2, err)
	}
}

// UT-CX-09: identical parents (a converged population) still give new
// genotypes: the donor is regrafted elsewhere, and a clone is re-picked.
func TestSubtreeCrossoverIdenticalParents(t *testing.T) {
	const n, runs = 20, 500
	rnd := rand.New(rand.NewSource(13))
	p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 4, 1)
	clones := 0
	for i := 0; i < runs; i++ {
		parent := csp2d.NewIndividualForTest(randomPostfix(rnd, n))
		c1, c2, err := p.SubtreeCrossoverForTest(parent, parent, rnd)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []*csp2d.Csp2dIndividualImpl{c1, c2} {
			if equalInts(c.PostfixForTest(), parent.PostfixForTest()) {
				clones++
			}
		}
	}
	t.Logf("%d clones in %d children of identical parents", clones, 2*runs)
	if clones > 2*runs/100 {
		t.Fatalf("%d clones in %d children, want at most 1%%", clones, 2*runs)
	}
}
