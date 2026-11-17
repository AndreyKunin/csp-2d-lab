//go:build testhooks

// Unit tests for csp2d/problem/csp2d. Build with -tags testhooks.
package csp2d_test

import (
	"csp2d/problem/csp2d"
	"fmt"
	"math/rand"
	"strconv"
	"testing"

	"github.com/leanovate/gopter"
)

// part is a test-side part definition: one entry per individual part (ids are
// assigned in order). Parts sharing a label must share dimensions.
type part struct {
	label string
	w, l  float64
}

// newProblem builds an initialised problem (label ranks + arena). sheet nil
// means unconstrained.
func newProblem(t testing.TB, parts []part, sheet *[2]float64, kerf float64, popSize, eliteCount int) *csp2d.Csp2dProblemImpl {
	t.Helper()
	inputs := make([]csp2d.InputDefinition, len(parts))
	for i, p := range parts {
		intLabel, _ := strconv.Atoi(p.label)
		long, short := p.w, p.l
		if short > long {
			long, short = short, long
		}
		inputs[i] = csp2d.InputDefinition{Id: i, Label: p.label, IntLabel: intLabel, LongSide: long, ShortSide: short}
	}
	maxL, maxS := -1.0, -1.0
	if sheet != nil {
		maxL, maxS = sheet[0], sheet[1]
		if maxS > maxL {
			maxL, maxS = maxS, maxL
		}
	}
	p := csp2d.NewProblemForTest(inputs, csp2d.NewConstraints(maxL, maxS, kerf))
	p.InitForTest(popSize, eliteCount)
	return p
}

// distinctParts returns n parts with distinct labels and varied sizes that all
// fit a 100x100 sheet.
func distinctParts(n int, rnd *rand.Rand) []part {
	parts := make([]part, n)
	for i := range parts {
		parts[i] = part{label: strconv.Itoa(i + 1), w: float64(5 + rnd.Intn(60)), l: float64(5 + rnd.Intn(60))}
	}
	return parts
}

var sheet100 = &[2]float64{100, 100}

// validatePostfix checks that p is a valid postfix over the ids 0..n-1, each
// used exactly once, with n-1 operators (S or L), satisfying the ballot
// property (stack depth >= 1 after each operand, ends at exactly 1).
func validatePostfix(p []int, n int) error {
	if len(p) != 2*n-1 {
		return fmt.Errorf("length %d, want %d", len(p), 2*n-1)
	}
	seen := make([]bool, n)
	depth := 0
	for i, v := range p {
		switch {
		case v >= 0:
			if v >= n {
				return fmt.Errorf("position %d: id %d out of range", i, v)
			}
			if seen[v] {
				return fmt.Errorf("position %d: id %d used twice", i, v)
			}
			seen[v] = true
			depth++
		case v == csp2d.NumS || v == csp2d.NumL:
			if depth < 2 {
				return fmt.Errorf("position %d: operator with stack depth %d", i, depth)
			}
			depth--
		default:
			return fmt.Errorf("position %d: unknown token %d", i, v)
		}
	}
	if depth != 1 {
		return fmt.Errorf("final stack depth %d", depth)
	}
	return nil
}

// operatorSkeleton returns the postfix with every leaf replaced by -1.
func operatorSkeleton(p []int) []int {
	s := make([]int, len(p))
	for i, v := range p {
		if v >= 0 {
			s[i] = -1
		} else {
			s[i] = v
		}
	}
	return s
}

// leafOrder returns the leaves of a postfix in order.
func leafOrder(p []int) []int {
	var out []int
	for _, v := range p {
		if v >= 0 {
			out = append(out, v)
		}
	}
	return out
}

// randomPostfix generates a uniformly shaped random postfix over ids 0..n-1,
// independently of the production generator: a random binary tree built by
// recursive splitting, random operators, and a random leaf permutation.
func randomPostfix(rnd *rand.Rand, n int) []int {
	ids := rnd.Perm(n)
	var build func(leaves []int) []int
	build = func(leaves []int) []int {
		if len(leaves) == 1 {
			return []int{leaves[0]}
		}
		k := 1 + rnd.Intn(len(leaves)-1)
		out := append(build(leaves[:k]), build(leaves[k:])...)
		op := csp2d.NumS
		if rnd.Intn(2) == 1 {
			op = csp2d.NumL
		}
		return append(out, op)
	}
	return build(ids)
}

// chainPostfix builds "o0 o1 op o2 op ..." for the given leaf order.
func chainPostfix(order []int, op int) []int {
	p := []int{order[0]}
	for _, id := range order[1:] {
		p = append(p, id, op)
	}
	return p
}

// gopterParams returns deterministic gopter parameters.
func gopterParams(t *testing.T, seed int64, minSuccessful int) *gopter.TestParameters {
	params := gopter.DefaultTestParametersWithSeed(seed)
	params.MinSuccessfulTests = minSuccessful
	t.Logf("gopter seed %d", seed)
	return params
}

func copyInts(p []int) []int { return append([]int(nil), p...) }

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
