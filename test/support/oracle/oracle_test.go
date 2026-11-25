package oracle_test

import (
	"csp2d/test/support/api"
	"csp2d/test/support/oracle"
	"strings"
	"testing"
)

func TestTreeVectors(t *testing.T) {
	vectors, err := oracle.LoadTreeVectors()
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) == 0 {
		t.Fatal("no tree vectors found")
	}
	for _, tv := range vectors {
		t.Run(tv.Name, func(t *testing.T) {
			root, err := tv.Instance().ParseTree(tv.Tree)
			if err != nil {
				t.Fatal(err)
			}
			if root.Dims.Long != tv.Long || root.Dims.Short != tv.Short {
				t.Fatalf("got %gx%g, want %gx%g", root.Dims.Long, root.Dims.Short, tv.Long, tv.Short)
			}
		})
	}
}

func TestReduceIsSymmetric(t *testing.T) {
	cases := [][2]oracle.Dims{
		{oracle.NewDims(40, 30), oracle.NewDims(50, 20)},
		{oracle.NewDims(100, 60), oracle.NewDims(50, 40)},
		{oracle.NewDims(100, 50), oracle.NewDims(50, 30)},
		{oracle.NewDims(10, 10), oracle.NewDims(10, 10)},
	}
	for _, op := range []string{"S", "L"} {
		for _, c := range cases {
			ab, _ := oracle.Reduce(op, c[0], c[1], 1.5)
			ba, _ := oracle.Reduce(op, c[1], c[0], 1.5)
			if ab != ba {
				t.Errorf("%s(%v,%v)=%v but reversed=%v", op, c[0], c[1], ab, ba)
			}
		}
	}
}

func TestParseTreeErrors(t *testing.T) {
	inst := mustInstance(t, &api.StartRequest{Parts: []api.Part{{Label: "1", Width: 10, Length: 20, Count: 2}}})
	for _, tree := range []string{"", "S", "1 S", "1 1", "1 2 S", "1 1 X"} {
		if _, err := inst.ParseTree(tree); err == nil {
			t.Errorf("tree %q: expected an error", tree)
		}
	}
}

func TestNewInstanceRejectsAmbiguousLabels(t *testing.T) {
	_, err := oracle.NewInstance(&api.StartRequest{Parts: []api.Part{
		{Label: "1", Width: 10, Length: 20, Count: 1},
		{Label: "1", Width: 10, Length: 30, Count: 1},
	}})
	if err == nil {
		t.Fatal("expected an error for a label reused with different dimensions")
	}
}

func TestLowerBound(t *testing.T) {
	cases := []struct {
		req  *api.StartRequest
		want int
	}{
		{&api.StartRequest{Sheets: api.Rect(100, 100), Parts: []api.Part{{Label: "1", Width: 50, Length: 50, Count: 4}}}, 1},
		{&api.StartRequest{Sheets: api.Rect(100, 50), Parts: []api.Part{{Label: "1", Width: 50, Length: 50, Count: 5}}}, 3},
		{&api.StartRequest{Parts: []api.Part{{Label: "1", Width: 50, Length: 50, Count: 9}}}, 1},
	}
	for i, c := range cases {
		if got := mustInstance(t, c.req).LowerBound(); got != c.want {
			t.Errorf("case %d: lower bound %d, want %d", i, got, c.want)
		}
	}
}

// quad is four 50x50 parts on a 100x100 sheet.
func quad(t *testing.T) *oracle.Instance {
	return mustInstance(t, &api.StartRequest{
		Sheets: api.Rect(100, 100),
		Parts:  []api.Part{{Label: "1", Width: 50, Length: 50, Count: 4}},
	})
}

func TestValidSolutionPasses(t *testing.T) {
	inst := quad(t)
	sol := api.Solution{
		Sheets:        []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 1 L L"}},
		Fitness:       10000 + 10000,
		FitnessVector: []float64{1, 10000, 1},
	}
	if v := inst.ValidateSolution(sol); len(v) > 0 {
		t.Fatalf("unexpected violations: %v", v)
	}
	if w := inst.Waste(sol); w != 0 {
		t.Fatalf("waste %g, want 0", w)
	}
}

func TestMultiSheetSolutionPasses(t *testing.T) {
	inst := quad(t)
	// Two sheets: 1 1 L = 100x50 each.
	sol := api.Solution{
		Sheets:        []api.SheetLayout{{Number: 0, Tree: "1 1 L"}, {Number: 1, Tree: "1 1 S"}},
		Fitness:       2*10000 + 5000 + 5000,
		FitnessVector: []float64{2, 10000},
	}
	if v := inst.ValidateSolution(sol); len(v) > 0 {
		t.Fatalf("unexpected violations: %v", v)
	}
}

func TestInvalidSolutionsAreDetected(t *testing.T) {
	inst := quad(t)
	cases := map[string]struct {
		sol  api.Solution
		want string
	}{
		"missing part": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 S"}}, Fitness: 17500, FitnessVector: []float64{1, 7500}},
			"appears 3 times",
		},
		"overflow": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 S 1 S 1 S"}}, Fitness: 20000, FitnessVector: []float64{1, 10000}},
			"does not fit",
		},
		"wrong sheet count in vector": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 1 L L"}}, Fitness: 20000, FitnessVector: []float64{2, 10000}},
			"fitnessVector[0]",
		},
		"wrong area": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 1 L L"}}, Fitness: 20000, FitnessVector: []float64{1, 9000}},
			"fitnessVector[1]",
		},
		"wrong scalar fitness": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 1 L L"}}, Fitness: 10000, FitnessVector: []float64{1, 10000}},
			"fitness =",
		},
		"bad numbering": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L"}, {Number: 2, Tree: "1 1 L"}}, Fitness: 30000, FitnessVector: []float64{2, 10000}},
			"sheet numbers",
		},
		"unknown label": {
			api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 7 L L"}}, Fitness: 20000, FitnessVector: []float64{1, 10000}},
			"unknown label",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v := inst.ValidateSolution(c.sol)
			if !containsAny(v, c.want) {
				t.Fatalf("violations %v do not mention %q", v, c.want)
			}
		})
	}
}

func TestValidateResultChecksOrdering(t *testing.T) {
	inst := quad(t)
	good := api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L 1 1 L L"}}, Fitness: 20000, FitnessVector: []float64{1, 10000}}
	worse := api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L"}, {Number: 1, Tree: "1 1 L"}}, Fitness: 30000, FitnessVector: []float64{2, 10000}}
	if v := inst.ValidateResult(&api.Result{Solutions: []api.Solution{good, worse}}); len(v) > 0 {
		t.Fatalf("sorted result rejected: %v", v)
	}
	if v := inst.ValidateResult(&api.Result{Solutions: []api.Solution{worse, good}}); !containsAny(v, "not sorted") {
		t.Fatalf("unsorted result accepted: %v", v)
	}
}

func TestUnconstrained(t *testing.T) {
	inst := mustInstance(t, &api.StartRequest{Parts: []api.Part{{Label: "1", Width: 30, Length: 20, Count: 2}}})
	sol := api.Solution{Sheets: []api.SheetLayout{{Number: 0, Tree: "1 1 L"}}, Fitness: 1200, FitnessVector: []float64{1, 1200}}
	if v := inst.ValidateSolution(sol); len(v) > 0 {
		t.Fatalf("unexpected violations: %v", v)
	}
}

func mustInstance(t *testing.T, req *api.StartRequest) *oracle.Instance {
	t.Helper()
	inst, err := oracle.NewInstance(req)
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

func containsAny(list []string, sub string) bool {
	for _, s := range list {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
