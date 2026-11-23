//go:build testhooks

// Unit tests for csp2d/mvc request mapping (exported API only).
package mvc_test

import (
	"csp2d/mvc"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func decodeRequest(t *testing.T, body string) *mvc.Request {
	t.Helper()
	var r mvc.Request
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	return &r
}

func defaults(t *testing.T) mvc.SimulationParameters {
	t.Helper()
	var p mvc.SimulationParameters
	body := `{"populationSize":{"min":100,"max":60000},"eliteCount":10,"mutationRate":0.07,
	          "maxEpochsCount":20000,"windowSize":50,"crossoverRate":0.75,"cycleCount":10,
	          "tournamentSize":5,"repairWeight":3,"flipWeight":1,"donorFraction":0.5}`
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// UT-REQ-01: count expansion, sequential ids, long/short normalisation.
func TestBuildInputs(t *testing.T) {
	r := decodeRequest(t, `{"parts":[{"label":"1","width":30,"length":20,"count":2},
	                                  {"label":"2","width":10,"length":40,"count":1},
	                                  {"label":"3","width":5,"length":5,"count":0}]}`)
	inputs, err := r.BuildInputs()
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 3 {
		t.Fatalf("%d inputs, want 3", len(inputs))
	}
	for i, in := range inputs {
		if in.Id != i {
			t.Errorf("input %d has id %d", i, in.Id)
		}
		if in.LongSide < in.ShortSide {
			t.Errorf("input %d not normalised: %g x %g", i, in.LongSide, in.ShortSide)
		}
	}
	if inputs[0].Label != "1" || inputs[1].Label != "1" || inputs[2].Label != "2" || inputs[2].IntLabel != 2 {
		t.Errorf("labels: %+v", inputs)
	}
	if inputs[2].LongSide != 40 || inputs[2].ShortSide != 10 {
		t.Errorf("part 2 dims %g x %g", inputs[2].LongSide, inputs[2].ShortSide)
	}
}

// UT-REQ-02: non-numeric labels are rejected.
func TestBuildInputsRejectsNonNumericLabel(t *testing.T) {
	r := decodeRequest(t, `{"parts":[{"label":"A","width":30,"length":20,"count":1}]}`)
	if _, err := r.BuildInputs(); err == nil {
		t.Fatal("expected an error")
	}
}

// UT-REQ-03 / UT-REQ-06: constraints and the PDF mapping (ToPdfInput).
func TestBuildConstraints(t *testing.T) {
	cases := []struct {
		name                  string
		body                  string
		long, short, sawWidth float64
	}{
		{"unconstrained", `{"parts":[]}`, -1, -1, 0},
		{"orientation normalised", `{"sheets":{"width":80,"length":120},"sawWidth":2.5,"parts":[]}`, 120, 80, 2.5},
		{"already long first", `{"sheets":{"width":120,"length":80},"parts":[]}`, 120, 80, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			long, short, saw := decodeRequest(t, c.body).BuildConstraints().ValuesForTest()
			if long != c.long || short != c.short || saw != c.sawWidth {
				t.Fatalf("got (%g, %g, %g), want (%g, %g, %g)", long, short, saw, c.long, c.short, c.sawWidth)
			}
		})
	}

	pdf := decodeRequest(t, `{"sheets":{"width":120,"length":80},"sawWidth":2,"parts":[{"label":"1","width":1,"length":2,"count":3}]}`).ToPdfInput()
	if pdf.Sheets == nil || pdf.Sheets.Width != 120 || pdf.Sheets.Length != 80 || pdf.SawWidth != 2 || len(pdf.Parts) != 1 || pdf.Parts[0].Count != 3 {
		t.Fatalf("pdf input %+v", pdf)
	}
	if p := decodeRequest(t, `{"parts":[]}`).ToPdfInput(); p.Sheets != nil || p.SawWidth != 0 {
		t.Fatalf("unconstrained pdf input %+v", p)
	}
}

// UT-REQ-04 / UT-REQ-05: defaults merge and the population formula.
func TestBuildParameters(t *testing.T) {
	d := defaults(t)
	cases := []struct {
		name    string
		body    string
		n       int
		wantPop int
	}{
		{"n squared below min", `{"parameters":{"populationSize":{"min":200,"max":60000}}}`, 12, 200},
		{"n squared between", `{}`, 50, 2500},
		{"n squared above max", `{}`, 300, 60000},
		{"request bounds", `{"parameters":{"populationSize":{"min":30,"max":50}}}`, 20, 50},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := decodeRequest(t, c.body).BuildParameters(d, c.n)
			if p.PopulationSize != c.wantPop {
				t.Fatalf("population %d, want %d", p.PopulationSize, c.wantPop)
			}
		})
	}
	p := decodeRequest(t, `{"parameters":{"mutationRate":0.2,"windowSize":7}}`).BuildParameters(d, 100)
	if p.MutationRate != 0.2 || p.WindowSize != 7 || p.EliteCount != 10 || p.MaxEpochsCount != 20000 {
		t.Fatalf("partial override: %+v", p)
	}
	// GA-02: the crossover rate comes from the defaults and a request can override it.
	if p.CrossoverRate != 0.75 {
		t.Fatalf("default crossoverRate %v, want 0.75", p.CrossoverRate)
	}
	if p := decodeRequest(t, `{"parameters":{"crossoverRate":0.5}}`).BuildParameters(d, 10); p.CrossoverRate != 0.5 {
		t.Fatalf("crossoverRate override %v, want 0.5", p.CrossoverRate)
	}
	// The cycle count (cycles before the championship) comes from the defaults
	// and a request can override it.
	if p.CycleCount != 10 {
		t.Fatalf("default cycleCount %d, want 10", p.CycleCount)
	}
	if p := decodeRequest(t, `{"parameters":{"cycleCount":4}}`).BuildParameters(d, 10); p.CycleCount != 4 {
		t.Fatalf("cycleCount override %d, want 4", p.CycleCount)
	}
	// Tournament size, repair and flip weights and donor fraction come from the
	// defaults and a request can override each of them.
	if p.TournamentSize != 5 || p.RepairWeight != 3 || p.FlipWeight != 1 || p.DonorFraction != 0.5 {
		t.Fatalf("defaults: tournament %d, repair %d, flip %d, donor %v; want 5, 3, 1, 0.5",
			p.TournamentSize, p.RepairWeight, p.FlipWeight, p.DonorFraction)
	}
	o := decodeRequest(t, `{"parameters":{"tournamentSize":3,"repairWeight":6,"flipWeight":2,"donorFraction":0.75}}`).BuildParameters(d, 10)
	if o.TournamentSize != 3 || o.RepairWeight != 6 || o.FlipWeight != 2 || o.DonorFraction != 0.75 {
		t.Fatalf("overrides: tournament %d, repair %d, flip %d, donor %v; want 3, 6, 2, 0.75",
			o.TournamentSize, o.RepairWeight, o.FlipWeight, o.DonorFraction)
	}
}

// UT-CFG-01: the production configuration parses.
func TestProductionConfigurationParses(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := mvc.GetConfigurationFromBytes(data)
	sp := cfg.Defaults.SimulationParameters
	if sp.PopulationSize == nil || sp.EliteCount == nil || sp.MutationRate == nil || sp.CrossoverRate == nil || sp.CycleCount == nil || sp.TournamentSize == nil || sp.RepairWeight == nil || sp.FlipWeight == nil || sp.DonorFraction == nil || sp.WindowSize == nil || sp.MaxEpochsCount == nil {
		t.Fatalf("missing defaults: %+v", sp)
	}
	if cfg.Server.StaticDir == "" {
		t.Error("staticDir is empty")
	}
}

// UT-REQ-06: small inputs (GA-05). For n parts in [4, 10] the population
// minimum is n²/2 (so the population is n²); the maximum stays as configured.
// The elite count is 1 for n < 5, 2 for n in [5, 10], else as configured. The
// overrides win over request values.
func TestSmallInputOverrides(t *testing.T) {
	d := defaults(t) // min 100, max 60000, elite 10
	cases := []struct {
		name      string
		body      string
		n         int
		wantPop   int
		wantElite int
	}{
		{"n=3: below the minimum part count, elite only", `{}`, 3, 100, 1},
		{"n=4", `{}`, 4, 16, 1},
		{"n=4 ignores a request min and elite", `{"parameters":{"populationSize":{"min":500,"max":60000},"eliteCount":7}}`, 4, 16, 1},
		{"n=4 keeps a request max", `{"parameters":{"populationSize":{"min":500,"max":10}}}`, 4, 10, 1},
		{"n=5", `{}`, 5, 25, 2},
		{"n=10", `{}`, 10, 100, 2},
		{"n=10 ignores a request elite", `{"parameters":{"eliteCount":7}}`, 10, 100, 2},
		{"n=11: min and elite as configured", `{}`, 11, 121, 10},
		{"n=11 keeps a request min and elite", `{"parameters":{"populationSize":{"min":300,"max":60000},"eliteCount":7}}`, 11, 300, 7},
		{"n=100: elite as requested", `{"parameters":{"eliteCount":7}}`, 100, 10000, 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := decodeRequest(t, c.body).BuildParameters(d, c.n)
			if p.PopulationSize != c.wantPop || p.EliteCount != c.wantElite {
				t.Fatalf("population %d, elite %d; want %d, %d", p.PopulationSize, p.EliteCount, c.wantPop, c.wantElite)
			}
		})
	}
}

// UT-REQ-07: a request that sends only the population maximum (the UI leaves an
// overridden minimum out) keeps the configured minimum for n > 10.
func TestPopulationMaxOnly(t *testing.T) {
	d := defaults(t)
	if p := decodeRequest(t, `{"parameters":{"populationSize":{"max":150}}}`).BuildParameters(d, 12); p.PopulationSize != 144 {
		t.Fatalf("population %d, want 144 (n², between the configured min 100 and max 150)", p.PopulationSize)
	}
	if p := decodeRequest(t, `{"parameters":{"populationSize":{"max":120}}}`).BuildParameters(d, 12); p.PopulationSize != 120 {
		t.Fatalf("population %d, want 120 (capped by max)", p.PopulationSize)
	}
	if p := decodeRequest(t, `{"parameters":{"populationSize":{"max":60000}}}`).BuildParameters(d, 6); p.PopulationSize != 36 {
		t.Fatalf("population %d, want 36 (n = 6: min n²/2)", p.PopulationSize)
	}
}

// UT-REQ-08: the total part count is the sum of the counts (the start request
// needs at least mvc.MinPartsCount).
func TestPartsCount(t *testing.T) {
	cases := []struct {
		body string
		want int
	}{
		{`{"parts":[]}`, 0},
		{`{"parts":[{"label":"1","width":10,"length":10,"count":3}]}`, 3},
		{`{"parts":[{"label":"1","width":10,"length":10,"count":2},{"label":"2","width":5,"length":5,"count":2}]}`, 4},
		{`{"parts":[{"label":"1","width":10,"length":10,"count":-3}]}`, -3},
	}
	for _, c := range cases {
		if got := decodeRequest(t, c.body).PartsCount(); got != c.want {
			t.Errorf("%s: parts count %d, want %d", c.body, got, c.want)
		}
	}
	if mvc.MinPartsCount != 4 {
		t.Fatalf("MinPartsCount %d, want 4", mvc.MinPartsCount)
	}
}

// UT-REQ-09: a window size below 1 is rejected (MET-03); an omitted or
// positive one is accepted.
func TestValidateWindowSize(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"no parameters", `{}`, false},
		{"window size omitted", `{"parameters":{"eliteCount":3}}`, false},
		{"window size 1", `{"parameters":{"windowSize":1}}`, false},
		{"window size 250", `{"parameters":{"windowSize":250}}`, false},
		{"large window size", `{"parameters":{"windowSize":100000000}}`, false},
		{"window size 0", `{"parameters":{"windowSize":0}}`, true},
		{"negative window size", `{"parameters":{"windowSize":-5}}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := decodeRequest(t, c.body).ValidateWindowSize()
			if (err != nil) != c.wantErr {
				t.Fatalf("error %v, want error %v", err, c.wantErr)
			}
		})
	}
}
