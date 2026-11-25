//go:build testhooks

// Unit tests for the server-side validation (BE-02): the rule DSL, the
// parameter specs in application.yaml (default + rule) and the request checks.
package mvc_test

import (
	"csp2d/ga"
	"csp2d/mvc"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(append([]string{filepath.Dir(file), "..", "..", ".."}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func productionConfig(t *testing.T) *mvc.Configuration {
	t.Helper()
	cfg, err := mvc.ParseConfiguration(repoFile(t, "application.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// UT-CFG-02: the rule DSL parses as the UI's (parseRule in
// static/src/utils/parameterDefinitions.ts); malformed or empty rules are
// rejected.
func TestParseRule(t *testing.T) {
	inf := math.Inf(1)
	valid := []struct {
		text string
		want mvc.ParameterRule
	}{
		{"int[1-)", mvc.ParameterRule{Type: "int", Min: 1, MinInclusive: true, Max: inf}},
		{"int[1-250]", mvc.ParameterRule{Type: "int", Min: 1, MinInclusive: true, Max: 250, MaxInclusive: true}},
		{"float[0-1]", mvc.ParameterRule{Type: "float", Min: 0, MinInclusive: true, Max: 1, MaxInclusive: true}},
		{"float(0-1]", mvc.ParameterRule{Type: "float", Min: 0, Max: 1, MaxInclusive: true}},
		{"int(0-100)", mvc.ParameterRule{Type: "int", Min: 0, Max: 100}},
		{"float(-2.5)", mvc.ParameterRule{Type: "float", Min: math.Inf(-1), Max: 2.5}},
		{"int[-]", mvc.ParameterRule{Type: "int", Min: math.Inf(-1), MinInclusive: true, Max: inf, MaxInclusive: true}},
		{" int[5-5] ", mvc.ParameterRule{Type: "int", Min: 5, MinInclusive: true, Max: 5, MaxInclusive: true}},
	}
	for _, c := range valid {
		got, err := mvc.ParseRule(c.text)
		if err != nil || got != c.want {
			t.Errorf("%q: %+v, %v; want %+v", c.text, got, err, c.want)
		}
	}
	for _, text := range []string{"", "int", "integer[1-)", "int[1-", "int1-2]", "int[-1-3]", "int[a-3]",
		"float[0-1]x", "int[5-3]", "int(5-5]", "int[5-5)", "float(1-1)"} {
		if _, err := mvc.ParseRule(text); err == nil {
			t.Errorf("%q accepted", text)
		}
	}
}

// UT-CFG-03: a rule checks the bounds, their inclusiveness and whole numbers,
// and words itself like the UI (describeRule).
func TestRuleAllowsAndDescribe(t *testing.T) {
	cases := []struct {
		rule     string
		allowed  []float64
		rejected []float64
		describe string
	}{
		{"int[1-250]", []float64{1, 2, 250}, []float64{0, 251, 1.5, -1, math.NaN(), math.Inf(1)}, "a whole number from 1 to 250"},
		{"float(0-1]", []float64{0.001, 0.5, 1}, []float64{0, 1.0001, -0.5}, "a number, greater than 0 and at most 1"},
		{"float[0-1]", []float64{0, 1}, []float64{-0.01, 1.01}, "a number from 0 to 1"},
		{"int[2-)", []float64{2, 1e9}, []float64{1, 2.5}, "a whole number, at least 2"},
		{"int(0-100)", []float64{1, 99}, []float64{0, 100}, "a whole number, greater than 0 and less than 100"},
		{"float(-1]", []float64{-5, 1}, []float64{1.1}, "a number, at most 1"},
		{"float[-]", []float64{-1e300, 0, 1e300}, []float64{math.Inf(-1)}, "a number"},
	}
	for _, c := range cases {
		rule, err := mvc.ParseRule(c.rule)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range c.allowed {
			if !rule.Allows(v) {
				t.Errorf("%s rejects %v", c.rule, v)
			}
		}
		for _, v := range c.rejected {
			if rule.Allows(v) {
				t.Errorf("%s allows %v", c.rule, v)
			}
		}
		if got := rule.Describe(); got != c.describe {
			t.Errorf("%s: %q, want %q", c.rule, got, c.describe)
		}
	}
}

// UT-CFG-07: the production application.yaml has a default and a rule for
// every parameter; the defaults are unchanged, the rules are the former UI
// rules, and Definitions() serves both to the UI.
func TestProductionParameterSpecs(t *testing.T) {
	cfg := productionConfig(t)
	want := map[string]struct {
		def  float64
		rule string
	}{
		"populationSize.min": {100, "int[3-)"}, "populationSize.max": {60000, "int[3-)"},
		"eliteCount": {3, "int[1-)"}, "mutationRate": {0.07, "float[0-1]"}, "crossoverRate": {0.75, "float(0-1]"},
		"maxEpochsCount": {20000, "int[1-)"}, "windowSize": {50, "int[1-250]"}, "cycleCount": {10, "int[1-)"},
		"tournamentSize": {5, "int[2-)"}, "repairWeight": {3, "int[1-)"}, "flipWeight": {1, "int[1-)"},
		"donorFraction": {0.5, "float(0-1]"},
	}
	if len(cfg.Defaults.Parameters) != len(want) {
		t.Fatalf("%d specs, want %d", len(cfg.Defaults.Parameters), len(want))
	}
	for _, spec := range cfg.Defaults.Parameters {
		w, ok := want[spec.Key]
		if !ok {
			t.Errorf("unexpected spec %s", spec.Key)
			continue
		}
		rule, _ := mvc.ParseRule(w.rule)
		if spec.Default != w.def || spec.Rule != rule || spec.RuleText != w.rule {
			t.Errorf("%s: default %v rule %q %+v; want %v %s", spec.Key, spec.Default, spec.RuleText, spec.Rule, w.def, w.rule)
		}
	}
	// The UI takes the defaults and the rules as written (GET parameter-definitions).
	definitions := cfg.Defaults.Parameters.Definitions()
	if len(definitions) != len(want) {
		t.Errorf("%d definitions, want %d", len(definitions), len(want))
	}
	for key, w := range want {
		if d := definitions[key]; d.Default != w.def || d.Rule != w.rule {
			t.Errorf("Definitions()[%s] = %+v, want %v %q", key, d, w.def, w.rule)
		}
	}
	if d := cfg.Defaults.SimulationParameters; *d.EliteCount != 3 || *d.PopulationSize.Max != 60000 || *d.DonorFraction != 0.5 {
		t.Errorf("default values: %+v", d)
	}
	if cfg.Inputs.MaxPartsCount != 100 {
		t.Errorf("maxPartsCount %d, want 100 (the UI's MAX_TOTAL_COUNT)", cfg.Inputs.MaxPartsCount)
	}
}

// UT-CFG-04: a malformed parameter section or input limit is rejected with a
// message naming the problem.
func TestParameterSpecsRejectMalformed(t *testing.T) {
	prod := strings.ReplaceAll(string(repoFile(t, "application.yaml")), "\r\n", "\n")
	const elite = "    eliteCount:\n      default: 3\n      rule: \"int[1-)\"\n"
	if !strings.Contains(prod, elite) {
		t.Fatal("application.yaml layout changed; update the test")
	}
	replace := func(old, new string) string { return strings.Replace(prod, old, new, 1) }
	cases := []struct {
		name, yaml, want string
	}{
		{"old scalar format", replace(elite, "    eliteCount: 3\n"), "eliteCount: expected default and rule"},
		{"missing rule", replace(elite, "    eliteCount:\n      default: 3\n"), "eliteCount: rule must be a string"},
		{"missing default", replace(elite, "    eliteCount:\n      rule: \"int[1-)\"\n"), "eliteCount: default must be a number"},
		{"unknown field", replace(elite, elite+"      max: 5\n"), "eliteCount: unknown field max"},
		{"unknown parameter", replace(elite, strings.Replace(elite, "eliteCount", "eliteSize", 1)), "eliteSize: not a parameter"},
		{"missing parameter", replace(elite, ""), "eliteCount: missing"},
		{"malformed rule", replace(elite, strings.Replace(elite, "int[1-)", "int[1-", 1)), `eliteCount: invalid rule "int[1-"`},
		{"float rule on an int parameter", replace(elite, strings.Replace(elite, "int[1-)", "float[1-)", 1)), "eliteCount: rule type float, the parameter is int"},
		{"int rule on a float parameter", replace(`rule: "float[0-1]"`, `rule: "int[0-1]"`), "mutationRate: rule type int, the parameter is float"},
		{"default outside the rule", replace("default: 50\n", "default: 251\n"), "windowSize: default must be a whole number from 1 to 250, got 251"},
		{"fractional default of an int", replace(elite, strings.Replace(elite, "default: 3", "default: 2.5", 1)), "eliteCount: default must be a whole number"},
		{"maxPartsCount below the minimum", replace("maxPartsCount: 100", "maxPartsCount: 3"), "inputs.maxPartsCount must be at least 4, got 3"},
		{"no maxPartsCount", replace("  maxPartsCount: 100\n", ""), "inputs.maxPartsCount must be at least 4, got 0"},
		{"no parameter section", replace("  simulationParameters:\n", "  simulationParameterz:\n"), "defaults.simulationParameters: missing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.yaml == prod {
				t.Fatal("the replacement did not apply")
			}
			_, err := mvc.ParseConfiguration([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v, want it to contain %q", err, c.want)
			}
		})
	}
}

// UT-CFG-05: every parameter present is checked against its rule; an absent
// one is valid; all problems are reported.
func TestParameterRules(t *testing.T) {
	specs := productionConfig(t).Defaults.Parameters
	cases := []struct {
		body string
		want string // "" = valid
	}{
		{`{}`, ""},
		{`{"eliteCount":1,"mutationRate":0,"crossoverRate":1,"windowSize":250,"donorFraction":1,"tournamentSize":2}`, ""},
		{`{"mutationRate":1,"windowSize":1,"populationSize":{"min":3,"max":3},"maxEpochsCount":100000000,"cycleCount":1000000}`, ""},
		{`{"eliteCount":0}`, "eliteCount must be a whole number, at least 1, got 0"},
		{`{"windowSize":0}`, "windowSize must be a whole number from 1 to 250, got 0"},
		{`{"windowSize":251}`, "windowSize must be a whole number from 1 to 250, got 251"},
		{`{"mutationRate":1.01}`, "mutationRate must be a number from 0 to 1, got 1.01"},
		{`{"crossoverRate":0}`, "crossoverRate must be a number, greater than 0 and at most 1, got 0"},
		{`{"donorFraction":1.5}`, "donorFraction must be a number, greater than 0 and at most 1, got 1.5"},
		{`{"tournamentSize":1}`, "tournamentSize must be a whole number, at least 2, got 1"},
		{`{"populationSize":{"min":2}}`, "populationSize.min must be a whole number, at least 3, got 2"},
		{`{"populationSize":{"max":-1}}`, "populationSize.max must be a whole number, at least 3, got -1"},
		{`{"maxEpochsCount":0,"cycleCount":0}`, "cycleCount must be a whole number, at least 1, got 0; maxEpochsCount must be a whole number, at least 1, got 0"},
		{`{"repairWeight":0,"flipWeight":0}`, "flipWeight must be a whole number, at least 1, got 0; repairWeight must be a whole number, at least 1, got 0"},
	}
	for _, c := range cases {
		var p mvc.SimulationParameters
		if err := json.Unmarshal([]byte(c.body), &p); err != nil {
			t.Fatal(err)
		}
		err := specs.Validate(&p)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v, want valid", c.body, err)
		case c.want != "" && (err == nil || err.Error() != c.want):
			t.Errorf("%s: error %v, want %q", c.body, err, c.want)
		}
	}
	if err := specs.Validate(nil); err != nil {
		t.Errorf("no parameters: %v", err)
	}
}

// UT-REQ-09: a start request is validated before anything is built (BE-02):
// the parameters, the parts, the total part count, the sheet and the saw
// width. Every problem is reported.
func TestValidateRequest(t *testing.T) {
	specs := productionConfig(t).Defaults.Parameters
	const parts4 = `"parts":[{"label":"1","width":10,"length":20,"count":4}]`
	cases := []struct {
		name string
		body string
		want []string // nil = valid
	}{
		{"valid", `{` + parts4 + `,"sheets":{"width":100,"length":200},"sawWidth":0}`, nil},
		{"valid without sheet and saw width", `{` + parts4 + `}`, nil},
		{"count 0 rows are allowed", `{"parts":[{"label":"1","width":10,"length":20,"count":4},{"label":"2","width":1,"length":1,"count":0}]}`, nil},
		{"100 parts", `{"parts":[{"label":"1","width":10,"length":20,"count":60},{"label":"2","width":5,"length":5,"count":40}]}`, nil},
		{"3 parts", `{"parts":[{"label":"1","width":10,"length":20,"count":3}]}`, []string{"at least 4 parts are required, got 3"}},
		{"no parts", `{"parts":[]}`, []string{"at least 4 parts are required, got 0"}},
		{"101 parts", `{"parts":[{"label":"1","width":10,"length":20,"count":60},{"label":"2","width":5,"length":5,"count":41}]}`,
			[]string{"at most 100 parts are allowed, got 101"}},
		{"10^7 parts", `{"parts":[{"label":"1","width":10,"length":20,"count":10000000}]}`, []string{"part 1: count must be from 0 to 100, got 10000000"}},
		{"counts that would overflow", `{"parts":[{"label":"1","width":1,"length":1,"count":9223372036854775807},{"label":"2","width":1,"length":1,"count":9223372036854775807}]}`,
			[]string{"part 1: count must be from 0 to 100", "part 2: count must be from 0 to 100"}},
		{"negative count", `{"parts":[{"label":"1","width":10,"length":20,"count":5},{"label":"2","width":1,"length":1,"count":-1}]}`, []string{"part 2: count must be from 0 to 100, got -1"}},
		{"non-numeric label", `{"parts":[{"label":"A","width":10,"length":20,"count":4}]}`, []string{`part 1: label must be a whole number, got "A"`}},
		{"zero width", `{"parts":[{"label":"1","width":0,"length":20,"count":4}]}`, []string{"part 1: width and length must be greater than 0, got 0 x 20"}},
		{"negative length", `{"parts":[{"label":"1","width":10,"length":-2,"count":4}]}`, []string{"part 1: width and length must be greater than 0"}},
		{"zero sheet", `{` + parts4 + `,"sheets":{"width":0,"length":200}}`, []string{"sheet width and length must be greater than 0"}},
		{"negative saw width", `{` + parts4 + `,"sawWidth":-0.5}`, []string{"sawWidth must be at least 0, got -0.5"}},
		{"invalid parameter", `{` + parts4 + `,"parameters":{"eliteCount":0}}`, []string{"eliteCount must be a whole number, at least 1, got 0"}},
		{"a label repeated with the same sizes", `{"parts":[{"label":"1","width":10,"length":20,"count":2},{"label":"1","width":20,"length":10,"count":2}]}`, nil},
		{"a label repeated with different sizes", `{"parts":[{"label":"1","width":10,"length":20,"count":2},{"label":"1","width":10,"length":30,"count":2}]}`,
			[]string{`part 2: label "1" is used by part 1 with another size (10 x 20), got 10 x 30`}},
		{"a label repeated in a count 0 row", `{"parts":[{"label":"1","width":10,"length":20,"count":4},{"label":"1","width":10,"length":30,"count":0}]}`,
			[]string{`part 2: label "1" is used by part 1 with another size (10 x 20), got 10 x 30`}},
		{"several problems", `{"parts":[{"label":"x","width":0,"length":1,"count":2}],"sawWidth":-1,"parameters":{"windowSize":0}}`,
			[]string{"windowSize must be", `label must be a whole number, got "x"`, "width and length must be greater than 0", "at least 4 parts are required, got 2", "sawWidth must be at least 0"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := decodeRequest(t, c.body).Validate(specs, 100)
			if c.want == nil {
				if err != nil {
					t.Fatalf("%v, want valid", err)
				}
				return
			}
			if err == nil {
				t.Fatal("accepted")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q, want it to contain %q", err, w)
				}
			}
		})
	}
	if err := decodeRequest(t, `{"parts":[{"label":"1","width":1,"length":1,"count":150}]}`).Validate(specs, 150); err != nil {
		t.Errorf("150 parts with maxPartsCount 150: %v", err)
	}
}

// UT-REQ-10: the elite count must be below the population size, which only
// the built parameters know.
func TestValidateBuiltParameters(t *testing.T) {
	cases := []struct {
		elite, population int
		valid             bool
	}{
		{1, 2, true}, {3, 100, true}, {99, 100, true}, {100, 100, false}, {101, 100, false},
	}
	for _, c := range cases {
		err := mvc.ValidateBuiltParameters(ga.Parameters{EliteCount: c.elite, PopulationSize: c.population})
		if (err == nil) != c.valid {
			t.Errorf("elite %d, population %d: %v, want valid %v", c.elite, c.population, err, c.valid)
		}
	}
	err := mvc.ValidateBuiltParameters(ga.Parameters{EliteCount: 30, PopulationSize: 16})
	if err == nil || err.Error() != "eliteCount must be less than the population size 16, got 30" {
		t.Errorf("message: %v", err)
	}
}

// UT-CFG-06: every functional-test profile (test/functional/fixtures/config)
// is a valid configuration.
func TestTestProfilesParse(t *testing.T) {
	for _, name := range []string{"defaults", "fast", "quality", "limited-1g"} {
		if _, err := mvc.ParseConfiguration(repoFile(t, "test", "functional", "fixtures", "config", name+".yaml")); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
