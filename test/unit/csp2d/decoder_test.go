//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"csp2d/test/support/api"
	"csp2d/test/support/oracle"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func norm(a, b float64) (float64, float64) {
	if a >= b {
		return a, b
	}
	return b, a
}

// UT-RED-P1 / UT-RED-P2: reduce is symmetric and never shrinks.
func TestReduceProperties(t *testing.T) {
	p := newProblem(t, []part{{"1", 1, 1}}, nil, 0, 1, 1)
	properties := gopter.NewProperties(gopterParams(t, 101, 1000))
	dim := gen.Float64Range(1, 200)
	properties.Property("reduce is symmetric, monotone and normalised", prop.ForAll(
		func(a1, a2, b1, b2, gap float64, useL bool) string {
			op := csp2d.NumS
			if useL {
				op = csp2d.NumL
			}
			l1, s1 := norm(a1, a2)
			l2, s2 := norm(b1, b2)
			long, short := p.ReduceForTest(op, l1, s1, l2, s2, gap)
			rl, rs := p.ReduceForTest(op, l2, s2, l1, s1, gap)
			if long != rl || short != rs {
				return fmt.Sprintf("not symmetric: %gx%g vs %gx%g", long, short, rl, rs)
			}
			if long < short {
				return "result not normalised"
			}
			if long*short < l1*s1+l2*s2-1e-9 {
				return fmt.Sprintf("area %g below children %g", long*short, l1*s1+l2*s2)
			}
			if long < math.Max(l1, l2)-1e-9 && short < math.Max(s1, s2)-1e-9 {
				return "result smaller than a child in both dimensions"
			}
			return ""
		},
		dim, dim, dim, dim, gen.Float64Range(0, 5), gen.Bool(),
	))
	properties.TestingRun(t)
}

// UT-FOR-01: a single leaf decodes to itself.
func TestForestSingleLeaf(t *testing.T) {
	p := newProblem(t, []part{{"1", 30, 40}}, sheet100, 0, 4, 1)
	forest, err := p.GetForestForTest(csp2d.NewIndividualForTest([]int{0}))
	if err != nil {
		t.Fatal(err)
	}
	if len(forest) != 1 || forest[0].Long != 40 || forest[0].Short != 30 || !equalInts(forest[0].Postfix, []int{0}) {
		t.Fatalf("forest %+v", forest)
	}
}

// UT-FOR-02: when everything fits one sheet, the forest is the whole postfix.
func TestForestAllFit(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}, {"2", 10, 10}, {"3", 10, 10}, {"4", 10, 10}}, sheet100, 0, 4, 1)
	postfix := []int{0, 1, csp2d.NumS, 2, 3, csp2d.NumL, csp2d.NumS}
	forest, err := p.GetForestForTest(csp2d.NewIndividualForTest(postfix))
	if err != nil {
		t.Fatal(err)
	}
	if len(forest) != 1 || !equalInts(forest[0].Postfix, postfix) {
		t.Fatalf("forest %+v", forest)
	}
}

// UT-FOR-06: the returned forest does not alias the decoder scratch buffers.
func TestForestDoesNotAliasScratch(t *testing.T) {
	rnd := rand.New(rand.NewSource(5))
	p := newProblem(t, distinctParts(30, rnd), sheet100, 0, 8, 2)
	first, err := p.GetForestForTest(csp2d.NewIndividualForTest(randomPostfix(rnd, 30)))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := make([][]int, len(first))
	for i, tree := range first {
		snapshot[i] = copyInts(tree.Postfix)
	}
	for k := 0; k < 5; k++ { // reuse the scratch buffers
		if _, err := p.GetForestForTest(csp2d.NewIndividualForTest(randomPostfix(rnd, 30))); err != nil {
			t.Fatal(err)
		}
	}
	for i, tree := range first {
		if !equalInts(tree.Postfix, snapshot[i]) {
			t.Fatalf("tree %d changed after later decodes: %v -> %v", i, snapshot[i], tree.Postfix)
		}
	}
}

// validateSubPostfix checks a sheet's postfix: a valid expression whose leaves
// are distinct ids below n.
func validateSubPostfix(p []int, n int) error {
	depth := 0
	seen := map[int]bool{}
	for i, v := range p {
		if v >= 0 {
			if v >= n || seen[v] {
				return fmt.Errorf("position %d: bad or repeated id %d", i, v)
			}
			seen[v] = true
			depth++
			continue
		}
		if depth < 2 {
			return fmt.Errorf("position %d: operator with depth %d", i, depth)
		}
		depth--
	}
	if depth != 1 {
		return fmt.Errorf("final depth %d", depth)
	}
	return nil
}

// UT-FOR-P1..P3 (property): the forest covers every part once, every sheet is
// a valid fitting sub-expression, and the independent oracle agrees on every
// sheet's dimensions.
func TestForestProperties(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 202, 300))
	properties.Property("forest covers, fits and matches the oracle", prop.ForAll(
		func(n int, seed int64, kerfTenths int) string {
			rnd := rand.New(rand.NewSource(seed))
			parts := distinctParts(n, rnd)
			kerf := float64(kerfTenths) / 10
			p := newProblem(t, parts, sheet100, kerf, 8, 2)
			forest, err := p.GetForestForTest(csp2d.NewIndividualForTest(randomPostfix(rnd, n)))
			if err != nil {
				return err.Error()
			}
			req := &api.StartRequest{Sheets: api.Rect(100, 100), SawWidth: api.Float(kerf)}
			for _, pt := range parts {
				req.Parts = append(req.Parts, api.Part{Label: pt.label, Width: pt.w, Length: pt.l, Count: 1})
			}
			inst, err := oracle.NewInstance(req)
			if err != nil {
				return err.Error()
			}
			count := make([]int, n)
			for i, tree := range forest {
				if err := validateSubPostfix(tree.Postfix, n); err != nil {
					return fmt.Sprintf("sheet %d: %v", i, err)
				}
				for _, v := range tree.Postfix {
					if v >= 0 {
						count[v]++
					}
				}
				if !p.IsValidMetricsForTest(tree.Long, tree.Short) {
					return fmt.Sprintf("sheet %d (%gx%g) does not fit", i, tree.Long, tree.Short)
				}
				root, err := inst.ParseTree(p.ToStringForTest(tree.Postfix))
				if err != nil {
					return err.Error()
				}
				if math.Abs(root.Dims.Long-tree.Long) > 1e-9 || math.Abs(root.Dims.Short-tree.Short) > 1e-9 {
					return fmt.Sprintf("sheet %d: decoder %gx%g, oracle %gx%g", i, tree.Long, tree.Short, root.Dims.Long, root.Dims.Short)
				}
			}
			for id, c := range count {
				if c != 1 {
					return fmt.Sprintf("part %d appears %d times", id, c)
				}
			}
			return ""
		},
		gen.IntRange(1, 40).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
		gen.IntRange(0, 30).WithLabel("kerf/10"),
	))
	properties.TestingRun(t)
}
