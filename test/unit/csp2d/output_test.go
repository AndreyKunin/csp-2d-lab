//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"csp2d/test/support/api"
	"csp2d/test/support/oracle"
	"math/rand"
	"strings"
	"testing"
)

// outputFixture: 3 part types (with repeated labels) and a set of evaluated
// elites in Selected.
func outputFixture(t *testing.T, seed int64, elites int) (*csp2d.Csp2dProblemImpl, []part) {
	t.Helper()
	rnd := rand.New(rand.NewSource(seed))
	parts := []part{
		{"1", 60, 40}, {"1", 60, 40}, {"1", 60, 40},
		{"2", 30, 70}, {"2", 30, 70},
		{"3", 45, 45}, {"3", 45, 45}, {"3", 45, 45},
	}
	p := newProblem(t, parts, sheet100, 1, 16, elites)
	selected := make([]problem.Individual, elites)
	for i := range selected {
		selected[i] = evaluated(t, p, randomPostfix(rnd, len(parts)))
	}
	p.SetPopulation(&problem.Population{Selected: selected})
	return p, parts
}

// UT-OUT-01 / UT-OUT-02: one solution per elite, numbered sheets, label/op
// tokens, sorted by fitness.
func TestNewResponse(t *testing.T) {
	p, _ := outputFixture(t, 21, 6)
	resp, err := csp2d.NewResponse(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Solutions) != 6 {
		t.Fatalf("%d solutions, want 6", len(resp.Solutions))
	}
	for i, sol := range resp.Solutions {
		if i > 0 && sol.Fitness < resp.Solutions[i-1].Fitness {
			t.Fatalf("solutions not sorted at %d", i)
		}
		for k, sheet := range sol.Sheets {
			if sheet.Number != k {
				t.Fatalf("solution %d sheet %d has number %d", i, k, sheet.Number)
			}
			for _, tok := range strings.Fields(sheet.Tree) {
				switch tok {
				case "S", "L", "1", "2", "3":
				default:
					t.Fatalf("unexpected token %q in %q", tok, sheet.Tree)
				}
			}
		}
	}
}

// UT-OUT-03: no selected individuals -> no solutions, no panic.
func TestNewResponseEmpty(t *testing.T) {
	p := newProblem(t, []part{{"1", 10, 10}}, sheet100, 0, 4, 1)
	p.SetPopulation(&problem.Population{})
	resp, err := csp2d.NewResponse(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Solutions) != 0 {
		t.Fatalf("%d solutions, want 0", len(resp.Solutions))
	}
}

// UT-OUT-04: round trip — the independent oracle accepts every solution the
// production serializer produces.
func TestNewResponseRoundTrip(t *testing.T) {
	for seed := int64(30); seed < 40; seed++ {
		p, parts := outputFixture(t, seed, 5)
		resp, err := csp2d.NewResponse(p)
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]*api.Part{}
		req := &api.StartRequest{Sheets: api.Rect(100, 100), SawWidth: api.Float(1)}
		for _, pt := range parts {
			if c := counts[pt.label]; c != nil {
				c.Count++
				continue
			}
			counts[pt.label] = &api.Part{Label: pt.label, Width: pt.w, Length: pt.l, Count: 1}
		}
		for _, label := range []string{"1", "2", "3"} {
			req.Parts = append(req.Parts, *counts[label])
		}
		inst, err := oracle.NewInstance(req)
		if err != nil {
			t.Fatal(err)
		}
		res := &api.Result{}
		for _, sol := range resp.Solutions {
			out := api.Solution{Fitness: sol.Fitness, FitnessVector: sol.FitnessVector[:]}
			for _, sh := range sol.Sheets {
				out.Sheets = append(out.Sheets, api.SheetLayout{Number: sh.Number, Tree: sh.Tree})
			}
			res.Solutions = append(res.Solutions, out)
		}
		if v := inst.ValidateResult(res); len(v) > 0 {
			t.Fatalf("seed %d: oracle rejects the serialized result:\n  %s", seed, strings.Join(v, "\n  "))
		}
	}
}
