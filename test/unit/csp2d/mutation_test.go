//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"csp2d/test/support/oracle"
	"csp2d/test/support/testutil"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

const S, L = csp2d.NumS, csp2d.NumL

// UT-SPAN-01: subtree start positions on hand-written postfixes.
func TestSubtreeSpan(t *testing.T) {
	p := newProblem(t, []part{{"1", 1, 1}}, nil, 0, 1, 1)
	cases := []struct {
		postfix []int
		want    []int // span start for every end position
	}{
		{[]int{0, 1, S, 2, L}, []int{0, 1, 0, 3, 0}},
		{[]int{0, 1, 2, S, L}, []int{0, 1, 2, 1, 0}},
		{[]int{0, 1, S, 2, 3, L, S}, []int{0, 1, 0, 3, 4, 3, 0}},
	}
	for _, c := range cases {
		for end, want := range c.want {
			if got := p.SubtreeSpanForTest(c.postfix, end); got != want {
				t.Errorf("%v end %d: span %d, want %d", c.postfix, end, got, want)
			}
		}
	}
}

// UT-SPAN-P1 (property): every span is a valid sub-expression.
func TestSubtreeSpanProperty(t *testing.T) {
	p := newProblem(t, []part{{"1", 1, 1}}, nil, 0, 1, 1)
	properties := gopter.NewProperties(gopterParams(t, 808, 300))
	properties.Property("span is a valid sub-expression", prop.ForAll(
		func(n int, seed int64) string {
			postfix := randomPostfix(rand.New(rand.NewSource(seed)), n)
			for end := range postfix {
				start := p.SubtreeSpanForTest(postfix, end)
				if err := validateSubPostfix(postfix[start:end+1], n); err != nil {
					return fmt.Sprintf("end %d start %d: %v", end, start, err)
				}
			}
			return ""
		},
		gen.IntRange(1, 40).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}

// UT-MUT-01: changeOperator flips exactly one operator in a fresh copy.
func TestChangeOperator(t *testing.T) {
	rnd := rand.New(rand.NewSource(13))
	p := newProblem(t, distinctParts(10, rnd), sheet100, 0, 8, 2)
	for i := 0; i < 50; i++ {
		original := randomPostfix(rnd, 10)
		snapshot := copyInts(original)
		ind := csp2d.NewIndividualForTest(original)
		if err := p.EvaluateFitnessForTest(ind); err != nil {
			t.Fatal(err)
		}
		if w, err := p.ChangeOperatorForTest(ind, rnd); err != nil || w != 1 {
			t.Fatalf("weight %d, err %v", w, err)
		}
		if !equalInts(original, snapshot) {
			t.Fatal("the original postfix slice was modified")
		}
		if ind.ForestForTest() != nil {
			t.Fatal("cache not invalidated")
		}
		diff := 0
		for pos, v := range ind.PostfixForTest() {
			if v != snapshot[pos] {
				diff++
				if snapshot[pos] >= 0 || v != csp2d.FlipOperator(snapshot[pos]) {
					t.Fatalf("position %d changed %d -> %d", pos, snapshot[pos], v)
				}
			}
		}
		if diff != 1 {
			t.Fatalf("%d positions changed, want 1", diff)
		}
	}
}

// swapSubtrees returns postfix with the disjoint subtrees ending at e1 < e2
// exchanged (nil when they overlap).
func swapSubtrees(p *csp2d.Csp2dProblemImpl, postfix []int, e1, e2 int) []int {
	s1, s2 := p.SubtreeSpanForTest(postfix, e1), p.SubtreeSpanForTest(postfix, e2)
	if e1 >= s2 {
		return nil
	}
	out := append([]int(nil), postfix[:s1]...)
	out = append(out, postfix[s2:e2+1]...)
	out = append(out, postfix[e1+1:s2]...)
	out = append(out, postfix[s1:e1+1]...)
	return append(out, postfix[e2+1:]...)
}

func countOps(p []int) (s, l int) {
	for _, v := range p {
		switch v {
		case S:
			s++
		case L:
			l++
		}
	}
	return
}

// UT-MUT-02 / UT-MUT-03 / UT-MUT-P1: replaceElements swaps two disjoint
// subtrees (weight 2) or, for nested picks, flips the root operator (weight 1).
func TestReplaceElements(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 909, 300))
	properties.Property("subtree swap or root flip", prop.ForAll(
		func(n int, seed int64) string {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 8, 2)
			original := randomPostfix(rnd, n)
			ind := csp2d.NewIndividualForTest(copyInts(original))
			w, err := p.ReplaceElementsForTest(ind, rnd)
			if err != nil {
				return err.Error()
			}
			got := ind.PostfixForTest()
			if err := validatePostfix(got, n); err != nil {
				return err.Error()
			}
			switch w {
			case 2:
				s0, l0 := countOps(original)
				if s1, l1 := countOps(got); s0 != s1 || l0 != l1 {
					return "operator multiset changed by a swap"
				}
				for e1 := 1; e1 < len(original); e1++ {
					for e2 := e1 + 1; e2 < len(original); e2++ {
						if sw := swapSubtrees(p, original, e1, e2); sw != nil && equalInts(sw, got) {
							return ""
						}
					}
				}
				return "result is not a swap of two disjoint subtrees"
			case 1:
				last := len(original) - 1
				for pos := range original {
					want := original[pos]
					if pos == last {
						want = csp2d.FlipOperator(want)
					}
					if got[pos] != want {
						return fmt.Sprintf("nested case changed position %d", pos)
					}
				}
				return ""
			default:
				return fmt.Sprintf("unexpected weight %d", w)
			}
		},
		gen.IntRange(3, 30).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}

// UT-MUT-04: critical operators are ranked by the waste of their join.
func TestRankCriticalOperators(t *testing.T) {
	parts := []part{{"1", 40, 30}, {"2", 50, 20}, {"3", 10, 10}}
	p := newProblem(t, parts, sheet100, 1, 4, 1)
	postfix := []int{0, 1, S, 2, L}
	got := p.RankCriticalOperatorsForTest(postfix)
	if len(got) != 2 {
		t.Fatalf("%d candidates, want 2", len(got))
	}
	// Independent expectation from the oracle geometry.
	d := func(i int) oracle.Dims { return oracle.NewDims(parts[i].w, parts[i].l) }
	j1, _ := oracle.Reduce("S", d(0), d(1), 1)
	j2, _ := oracle.Reduce("L", j1, d(2), 1)
	want := map[int]float64{
		2: j1.Area() - d(0).Area() - d(1).Area(),
		4: j2.Area() - j1.Area() - d(2).Area(),
	}
	for i, c := range got {
		if postfix[c.Index] >= 0 {
			t.Fatalf("candidate %d points at a leaf", c.Index)
		}
		if math.Abs(c.Score-want[c.Index]) > 1e-9 {
			t.Errorf("index %d score %g, want %g", c.Index, c.Score, want[c.Index])
		}
		if i > 0 && got[i-1].Score < c.Score {
			t.Error("candidates not sorted by descending score")
		}
	}
}

// UT-MUT-05: the worst tree is the one with the lowest fill ratio.
func TestSelectWorstTree(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}, {"2", 10, 10}, {"3", 10, 10}}, sheet100, 0, 4, 1)
	forest := []csp2d.NodeView{
		{Postfix: []int{0}, Long: 10, Short: 10}, // fill 1.0
		{Postfix: []int{1}, Long: 20, Short: 10}, // fill 0.5
		{Postfix: []int{2}, Long: 20, Short: 10}, // fill 0.5 (tie: first wins)
	}
	if got := p.SelectWorstTreeForTest(forest); got != 1 {
		t.Fatalf("worst tree %d, want 1", got)
	}
}

// UT-MUT-06: greedy insertion picks the (tree, operator, side) with the best
// resulting fitness and never modifies the original postfixes.
func TestInsertItemGreedy(t *testing.T) {
	parts := []part{{"1", 60, 40}, {"2", 90, 30}, {"3", 30, 30}}
	p := newProblem(t, parts, sheet100, 0, 4, 1)
	forest := []csp2d.NodeView{{Postfix: []int{0}, Long: 60, Short: 40}, {Postfix: []int{1}, Long: 90, Short: 30}}
	snapshot := [][]int{copyInts(forest[0].Postfix), copyInts(forest[1].Postfix)}

	// Brute force the best fitness over every placement.
	best := [3]float64{math.Inf(1)}
	for i, tree := range forest {
		for _, op := range []int{S, L} {
			for _, itemLeft := range []bool{false, true} {
				var long, short float64
				if itemLeft {
					long, short = p.ReduceForTest(op, 30, 30, tree.Long, tree.Short, 0)
				} else {
					long, short = p.ReduceForTest(op, tree.Long, tree.Short, 30, 30, 0)
				}
				if !p.IsValidMetricsForTest(long, short) {
					continue
				}
				if f := p.FitnessVectorWithReplacementForTest(forest, i, long, short); lexLess(f, best) {
					best = f
				}
			}
		}
	}
	got, ok := p.InsertItemGreedyForTest(forest, 2)
	if !ok {
		t.Fatal("item should fit")
	}
	if f := p.FitnessVectorForTest(got); f != best {
		t.Fatalf("fitness after insertion %v, best possible %v", f, best)
	}
	for i := range forest {
		if !equalInts(forest[i].Postfix, snapshot[i]) {
			t.Fatal("an original postfix was modified")
		}
	}
	// An item that fits nowhere.
	q := newProblem(t, []part{{"1", 100, 100}, {"2", 50, 50}}, sheet100, 0, 4, 1)
	if _, ok := q.InsertItemGreedyForTest([]csp2d.NodeView{{Postfix: []int{0}, Long: 100, Short: 100}}, 1); ok {
		t.Fatal("insertion into a full sheet must fail")
	}
}

// UT-MUT-07: reinsertion opens a new sheet when needed and rejects parts
// larger than the sheet.
func TestReinsertItems(t *testing.T) {
	p := newProblem(t, []part{{"1", 100, 100}, {"2", 50, 50}, {"3", 20, 20}}, sheet100, 0, 4, 1)
	base := []csp2d.NodeView{{Postfix: []int{0}, Long: 100, Short: 100}}
	got, err := p.ReinsertItemsForTest(base, []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	var leaves []int
	for _, tree := range got {
		leaves = append(leaves, leafOrder(tree.Postfix)...)
	}
	sort.Ints(leaves)
	if !equalInts(leaves, []int{0, 1, 2}) {
		t.Fatalf("leaves %v, want [0 1 2]", leaves)
	}
	if len(got) != 2 || !equalInts(got[0].Postfix, []int{0}) {
		t.Fatalf("expected the full sheet untouched plus one new sheet, got %+v", got)
	}
	big := newProblem(t, []part{{"1", 10, 10}, {"2", 101, 10}}, sheet100, 0, 4, 1)
	if _, err := big.ReinsertItemsForTest([]csp2d.NodeView{{Postfix: []int{0}, Long: 10, Short: 10}}, []int{1}); err == nil {
		t.Fatal("a part larger than the sheet must be rejected")
	}
}

func evaluated(t *testing.T, p *csp2d.Csp2dProblemImpl, postfix []int) *csp2d.Csp2dIndividualImpl {
	t.Helper()
	ind := csp2d.NewIndividualForTest(postfix)
	if err := p.EvaluateFitnessForTest(ind); err != nil {
		t.Fatal(err)
	}
	return ind
}

// UT-MUT-08: eliminate-and-repair.
func TestEliminateAndRepair(t *testing.T) {
	rnd := rand.New(rand.NewSource(14))

	// One sheet only: nothing to eliminate.
	one := newProblem(t, []part{{"1", 30, 30}, {"2", 30, 30}}, sheet100, 0, 4, 1)
	if child, improved, err := one.MutateEliminateAndRepairForTest(evaluated(t, one, []int{0, 1, S}), rnd); err != nil || improved || child != nil {
		t.Fatalf("single sheet: child %v improved %v err %v", child, improved, err)
	}

	// 100x60 S 30x30 overflows -> 2 sheets; repairing with L packs both on one.
	p := newProblem(t, []part{{"1", 100, 60}, {"2", 30, 30}}, sheet100, 0, 4, 1)
	parent := evaluated(t, p, []int{0, 1, S})
	if parent.FitnessVectorForTest()[0] != 2 {
		t.Fatalf("setup: parent uses %g sheets, want 2", parent.FitnessVectorForTest()[0])
	}
	child, improved, err := p.MutateEliminateAndRepairForTest(parent, rnd)
	if err != nil || !improved {
		t.Fatalf("improved %v err %v", improved, err)
	}
	if !lexLess(child.FitnessVectorForTest(), parent.FitnessVectorForTest()) {
		t.Fatalf("child %v not better than parent %v", child.FitnessVectorForTest(), parent.FitnessVectorForTest())
	}
	if err := validatePostfix(child.PostfixForTest(), 2); err != nil {
		t.Fatal(err)
	}

	// Two full sheets cannot be improved.
	full := newProblem(t, []part{{"1", 100, 100}, {"2", 100, 100}}, sheet100, 0, 4, 1)
	if _, improved, err := full.MutateEliminateAndRepairForTest(evaluated(t, full, []int{0, 1, L}), rnd); err != nil || improved {
		t.Fatalf("full sheets: improved %v err %v", improved, err)
	}
}

// UT-MUT-09: an eliminate-and-repair child's cached fitness against the
// decode of its own genotype: never better (always), equal (pending).
func TestEliminateAndRepairConsistency(t *testing.T) {
	run := func(t *testing.T, strict bool) {
		improvedCount := 0
		for seed := int64(1); seed <= 200; seed++ {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(15, rnd), sheet100, 0, 64, 2)
			parent := evaluated(t, p, randomPostfix(rnd, 15))
			child, improved, err := p.MutateEliminateAndRepairForTest(parent, rnd)
			if err != nil {
				t.Fatal(err)
			}
			if !improved {
				continue
			}
			improvedCount++
			if err := checkIndividual(p, child, 15, strict); err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
		}
		if improvedCount < 10 {
			t.Fatalf("only %d of 200 seeds produced an improved child; the test exercises too little", improvedCount)
		}
		t.Logf("%d improved children checked", improvedCount)
	}
	t.Run("not better than decode", func(t *testing.T) { run(t, false) })
	t.Run("equal to decode", func(t *testing.T) {
		testutil.Pending(t, "the child's genotype joins sheets with random operators that may merge on decode")
		run(t, true)
	})
}

// UT-MUT-10: critical operator flip.
func TestCriticalOperatorFlip(t *testing.T) {
	rnd := rand.New(rand.NewSource(15))
	p := newProblem(t, []part{{"1", 60, 40}, {"2", 60, 40}}, sheet100, 0, 4, 1)

	// 60x40 S 60x40 = 120x40 overflows (2 sheets); flipping to L gives 60x80.
	parent := evaluated(t, p, []int{0, 1, S})
	child, improved, err := p.MutateCriticalOperatorFlipForTest(parent, rnd)
	if err != nil || !improved {
		t.Fatalf("improved %v err %v", improved, err)
	}
	if !csp2d.LessFitness(child, parent) {
		t.Fatal("child is not better")
	}
	diff := 0
	for i, v := range child.PostfixForTest() {
		if v != parent.PostfixForTest()[i] {
			diff++
		}
	}
	if diff != 1 {
		t.Fatalf("%d positions differ, want 1", diff)
	}

	// Already optimal: the parent comes back unchanged.
	best := evaluated(t, p, []int{0, 1, L})
	got, improved, err := p.MutateCriticalOperatorFlipForTest(best, rnd)
	if err != nil || improved || got != best {
		t.Fatalf("optimal parent: got %p (parent %p) improved %v err %v", got, best, improved, err)
	}
}

// mutationSet builds a set of fresh individuals.
func mutationSet(rnd *rand.Rand, n, size int) ([]*problem.Individual, [][]int) {
	var postfixes [][]int
	for i := 0; i < size; i++ {
		postfixes = append(postfixes, randomPostfix(rnd, n))
	}
	return individuals(postfixes...), postfixes
}

// UT-MUT-11: rate 0 leaves the set untouched.
func TestMutationRateZero(t *testing.T) {
	rnd := rand.New(rand.NewSource(16))
	p := newProblem(t, distinctParts(8, rnd), sheet100, 0, 16, 2)
	set, _ := mutationSet(rnd, 8, 6)
	before := append([]*problem.Individual(nil), set...)
	count, weight, failures, err := p.Mutation(0, set, rnd)
	if err != nil || count != 0 || weight != 0 || failures != 0 {
		t.Fatalf("count %d weight %d failures %d err %v", count, weight, failures, err)
	}
	for i := range set {
		if set[i] != before[i] {
			t.Fatalf("entry %d replaced at rate 0", i)
		}
	}
}

// UT-MUT-12: the mutation budget is ceil(L*N*rate) weight units.
func TestMutationBudget(t *testing.T) {
	for _, rate := range []float64{0.01, 0.07, 0.2} {
		rnd := rand.New(rand.NewSource(17))
		n, size := 12, 10
		p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 64, 2)
		set, _ := mutationSet(rnd, n, size)
		_, weight, _, err := p.Mutation(rate, set, rnd)
		if err != nil {
			t.Fatal(err)
		}
		budget := int(math.Ceil(float64((2*n-1)*size) * rate))
		if weight < budget || weight >= budget+3 {
			t.Errorf("rate %g: weight %d, budget %d", rate, weight, budget)
		}
	}
}

// UT-MUT-P2 (property): after Mutation every entry is valid and unprotected,
// and the original individuals' genotypes are untouched.
func TestMutationProperty(t *testing.T) {
	properties := gopter.NewProperties(gopterParams(t, 1010, 150))
	properties.Property("mutation output is valid", prop.ForAll(
		func(n int, seed int64) string {
			rnd := rand.New(rand.NewSource(seed))
			p := newProblem(t, distinctParts(n, rnd), sheet100, 0, 64, 2)
			elite := randomPostfix(rnd, n)
			p.Protect([]problem.Individual{csp2d.NewIndividualForTest(elite)})
			set, postfixes := mutationSet(rnd, n, 8)
			originals := make([]*csp2d.Csp2dIndividualImpl, len(set))
			snapshots := make([][]int, len(set))
			for i, ind := range set {
				originals[i] = csp2d.AsIndividualForTest(*ind)
				snapshots[i] = copyInts(postfixes[i])
			}
			if _, _, _, err := p.Mutation(0.1, set, rnd); err != nil {
				return err.Error()
			}
			for i, ind := range set {
				postfix := csp2d.AsIndividualForTest(*ind).PostfixForTest()
				if err := validatePostfix(postfix, n); err != nil {
					return fmt.Sprintf("entry %d: %v", i, err)
				}
				if p.IsProtectedForTest(postfix) {
					return fmt.Sprintf("entry %d is protected", i)
				}
			}
			for i, o := range originals {
				if !equalInts(o.PostfixForTest(), snapshots[i]) {
					return fmt.Sprintf("original individual %d was modified in place", i)
				}
			}
			return ""
		},
		gen.IntRange(3, 25).WithLabel("n"),
		gen.Int64().WithLabel("seed"),
	))
	properties.TestingRun(t)
}
