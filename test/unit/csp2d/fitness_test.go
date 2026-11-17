//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// UT-FIT-01: fitness vector of a hand-built forest.
func TestFitnessVector(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 1, 1)
	forest := []csp2d.NodeView{{Postfix: []int{0}, Long: 50, Short: 40}, {Postfix: []int{1}, Long: 30, Short: 10}}
	got := p.FitnessVectorForTest(forest)
	if want := [3]float64{2, 2000 + 300, 1}; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// UT-FIT-02: scalar fitness, constrained and unconstrained.
func TestFitnessValue(t *testing.T) {
	ind := csp2d.NewIndividualForTest([]int{0})
	ind.SetFitnessForTest([3]float64{3, 1234, 1}, 0)

	constrained := newProblem(t, []part{{"1", 10, 10}}, &[2]float64{200, 100}, 0, 1, 1)
	if got := constrained.FitnessValueForTest(ind); got != 3*200*100+1234 {
		t.Errorf("constrained fitness %g", got)
	}
	unconstrained := newProblem(t, []part{{"1", 10, 10}}, nil, 0, 1, 1)
	if got := unconstrained.FitnessValueForTest(ind); got != 1234 {
		t.Errorf("unconstrained fitness %g", got)
	}
}

// UT-FIT-03 (property): replacement fitness equals a full recompute.
func TestFitnessWithReplacement(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 1, 1)
	properties := gopter.NewProperties(gopterParams(t, 303, 500))
	properties.Property("replacement equals recompute", prop.ForAll(
		func(seed int64, size int) bool {
			rnd := rand.New(rand.NewSource(seed))
			forest := make([]csp2d.NodeView, size)
			for i := range forest {
				forest[i] = csp2d.NodeView{Postfix: []int{i}, Long: 1 + rnd.Float64()*99, Short: 1 + rnd.Float64()*99}
			}
			idx := rnd.Intn(size)
			long, short := 1+rnd.Float64()*99, 1+rnd.Float64()*99
			got := p.FitnessVectorWithReplacementForTest(forest, idx, long, short)
			replaced := append([]csp2d.NodeView(nil), forest...)
			replaced[idx] = csp2d.NodeView{Postfix: []int{idx}, Long: long, Short: short}
			want := p.FitnessVectorForTest(replaced)
			return got[0] == want[0] && math.Abs(got[1]-want[1]) <= 1e-9*want[1] && got[2] == want[2]
		},
		gen.Int64().WithLabel("seed"),
		gen.IntRange(1, 20).WithLabel("trees"),
	))
	properties.TestingRun(t)
}

// UT-FIT-04: evaluation is cached; invalidation forces a recompute.
func TestFitnessCaching(t *testing.T) {
	p := newProblem(t, []part{{"1", 60, 40}, {"2", 60, 40}, {"3", 30, 30}}, sheet100, 0, 4, 1)
	ind := csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumS, 2, csp2d.NumL})
	if err := p.EvaluateFitnessForTest(ind); err != nil {
		t.Fatal(err)
	}
	first := ind.ForestForTest()
	if err := p.EvaluateFitnessForTest(ind); err != nil {
		t.Fatal(err)
	}
	if second := ind.ForestForTest(); &second[0].Postfix[0] != &first[0].Postfix[0] {
		t.Fatal("second evaluation recomputed the forest")
	}
	ind.InvalidateCacheForTest()
	if ind.ForestForTest() != nil || ind.FitnessVectorForTest() != [3]float64{} {
		t.Fatal("invalidateCache did not clear the cache")
	}
	if err := p.EvaluateFitnessForTest(ind); err != nil {
		t.Fatal(err)
	}
	if third := ind.ForestForTest(); &third[0].Postfix[0] == &first[0].Postfix[0] {
		t.Fatal("evaluation after invalidation reused the old forest")
	}
}

// UT-FIT-05: the public fitness vector exposes [sheets, area].
func TestPublicFitnessVector(t *testing.T) {
	ind := csp2d.NewIndividualForTest([]int{0})
	ind.SetFitnessForTest([3]float64{2, 99, 1}, 0)
	if v := ind.GetFitnessVector(); len(v) != 2 || v[0] != 2 || v[1] != 99 {
		t.Fatalf("GetFitnessVector = %v", v)
	}
}

func withFitness(v [3]float64) *csp2d.Csp2dIndividualImpl {
	ind := csp2d.NewIndividualForTest([]int{0})
	ind.SetFitnessForTest(v, 0)
	return ind
}

// UT-SORT-01: lexicographic order.
func TestLessFitness(t *testing.T) {
	cases := []struct {
		a, b [3]float64
		want bool
	}{
		{[3]float64{1, 900, 1}, [3]float64{2, 100, 1}, true}, // fewer sheets wins
		{[3]float64{2, 100, 1}, [3]float64{1, 900, 1}, false},
		{[3]float64{2, 100, 1}, [3]float64{2, 200, 1}, true},  // tie -> area
		{[3]float64{2, 100, 0}, [3]float64{2, 100, 1}, true},  // tie -> third
		{[3]float64{2, 100, 1}, [3]float64{2, 100, 1}, false}, // irreflexive
	}
	for i, c := range cases {
		if got := csp2d.LessFitnessRaw(c.a, c.b); got != c.want {
			t.Errorf("case %d raw: got %v", i, got)
		}
		if got := csp2d.LessFitness(withFitness(c.a), withFitness(c.b)); got != c.want {
			t.Errorf("case %d: got %v", i, got)
		}
	}
}

// UT-SORT-02 (property): SortSpecies orders a permutation of the input.
func TestSortSpecies(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 1, 1)
	properties := gopter.NewProperties(gopterParams(t, 404, 300))
	properties.Property("sorted permutation", prop.ForAll(
		func(seed int64, size int) string {
			rnd := rand.New(rand.NewSource(seed))
			species := make([]problem.Individual, size)
			for i := range species {
				species[i] = withFitness([3]float64{float64(1 + rnd.Intn(3)), float64(rnd.Intn(5)), 1})
			}
			before := append([]problem.Individual(nil), species...)
			p.SetPopulation(&problem.Population{Species: &species})
			p.SortSpecies()
			if !sort.SliceIsSorted(species, func(i, j int) bool {
				return csp2d.LessFitness(csp2d.AsIndividualForTest(species[i]), csp2d.AsIndividualForTest(species[j]))
			}) {
				return "not sorted"
			}
			set := map[problem.Individual]int{}
			for _, ind := range before {
				set[ind]++
			}
			for _, ind := range species {
				set[ind]--
			}
			for _, c := range set {
				if c != 0 {
					return "not a permutation of the input"
				}
			}
			return ""
		},
		gen.Int64().WithLabel("seed"),
		gen.IntRange(0, 50).WithLabel("size"),
	))
	properties.TestingRun(t)
}
