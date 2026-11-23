package mvc

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Parameter validation (BE-02). Each parameter in defaults.simulationParameters
// of application.yaml has a default value and a validation rule. The server
// validates requests with the rules; the UI takes the defaults and the same
// rules from GET /api/v1/service/parameter-definitions
// (static/src/utils/parameterDefinitions.ts parses the rules with the same
// DSL). The rule DSL:
//
//	<type><open><min>-<max><close>
//	type      int (whole number) or float
//	open      [ = min inclusive, ( = min exclusive
//	close     ] = max inclusive, ) = max exclusive
//	min, max  non-negative numbers; an empty bound means unbounded
//
// Examples: float[0-1] = 0 <= v <= 1; int(0-100) = 0 < v < 100, whole;
// int[2-) = whole, at least 2.

// ParameterRule is a parsed rule.
type ParameterRule struct {
	Type         string  // "int" or "float"
	Min          float64 // -Inf when unbounded
	MinInclusive bool
	Max          float64 // +Inf when unbounded
	MaxInclusive bool
}

var rulePattern = regexp.MustCompile(`^(int|float)([[(])(\d+(?:\.\d+)?)?-(\d+(?:\.\d+)?)?([\])])$`)

// ParseRule parses a rule of the DSL above.
func ParseRule(text string) (ParameterRule, error) {
	m := rulePattern.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return ParameterRule{}, fmt.Errorf("invalid rule %q", text)
	}
	rule := ParameterRule{Type: m[1], Min: math.Inf(-1), Max: math.Inf(1), MinInclusive: m[2] == "[", MaxInclusive: m[5] == "]"}
	if m[3] != "" {
		rule.Min, _ = strconv.ParseFloat(m[3], 64) // the pattern admits only valid numbers
	}
	if m[4] != "" {
		rule.Max, _ = strconv.ParseFloat(m[4], 64)
	}
	if rule.Min > rule.Max || rule.Min == rule.Max && !(rule.MinInclusive && rule.MaxInclusive) {
		return ParameterRule{}, fmt.Errorf("invalid rule %q: no value satisfies it", text)
	}
	return rule, nil
}

// Allows reports whether v satisfies the rule.
func (r ParameterRule) Allows(v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) || r.Type == "int" && v != math.Trunc(v) {
		return false
	}
	if r.MinInclusive && v < r.Min || !r.MinInclusive && v <= r.Min {
		return false
	}
	if r.MaxInclusive && v > r.Max || !r.MaxInclusive && v >= r.Max {
		return false
	}
	return true
}

// Describe words the rule like the UI does (describeRule): "a whole number
// from 1 to 250", "a number, greater than 0 and at most 1", "a whole number, at
// least 2".
func (r ParameterRule) Describe() string {
	kind := "a number"
	if r.Type == "int" {
		kind = "a whole number"
	}
	format := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	hasMin, hasMax := !math.IsInf(r.Min, -1), !math.IsInf(r.Max, 1)
	if hasMin && hasMax && r.MinInclusive && r.MaxInclusive {
		return fmt.Sprintf("%s from %s to %s", kind, format(r.Min), format(r.Max))
	}
	var parts []string
	if hasMin {
		if r.MinInclusive {
			parts = append(parts, "at least "+format(r.Min))
		} else {
			parts = append(parts, "greater than "+format(r.Min))
		}
	}
	if hasMax {
		if r.MaxInclusive {
			parts = append(parts, "at most "+format(r.Max))
		} else {
			parts = append(parts, "less than "+format(r.Max))
		}
	}
	if len(parts) == 0 {
		return kind
	}
	return kind + ", " + strings.Join(parts, " and ")
}

// ParameterSpec is one parameter of defaults.simulationParameters. Key is the
// request field; a dotted key is a nested field (populationSize.min).
type ParameterSpec struct {
	Key      string
	Default  float64
	Rule     ParameterRule
	RuleText string // the rule as written, served to the UI
}

// ParameterSpecs is defaults.simulationParameters: every parameter of
// SimulationParameters, each a mapping with a default and a rule:
//
//	eliteCount:
//	  default: 3
//	  rule: "int[1-)"
type ParameterSpecs []ParameterSpec

// UnmarshalYAML reads the section and checks it: every SimulationParameters
// field has a spec, every spec names a field, the rule parses and matches the
// field type, and the default satisfies the rule.
func (s *ParameterSpecs) UnmarshalYAML(unmarshal func(any) error) error {
	var tree map[any]any
	if err := unmarshal(&tree); err != nil {
		return err
	}
	var specs ParameterSpecs
	var problems []string
	var walk func(node map[any]any, prefix string)
	walk = func(node map[any]any, prefix string) {
		for name, value := range node {
			key := prefix + fmt.Sprint(name)
			child, ok := value.(map[any]any)
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: expected default and rule, got %v", key, value))
				continue
			}
			if _, isSpec := child["rule"]; !isSpec {
				if _, isSpec = child["default"]; !isSpec {
					walk(child, key+".")
					continue
				}
			}
			spec, err := parseSpec(key, child)
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			specs = append(specs, spec)
		}
	}
	walk(tree, "")
	seen := map[string]bool{}
	for _, spec := range specs {
		seen[spec.Key] = true
	}
	for _, key := range parameterKeys() {
		if !seen[key] {
			problems = append(problems, fmt.Sprintf("%s: missing", key))
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems) // map order is random
		return fmt.Errorf("defaults.simulationParameters: %s", strings.Join(problems, "; "))
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Key < specs[j].Key })
	*s = specs
	return nil
}

func parseSpec(key string, node map[any]any) (ParameterSpec, error) {
	for name := range node {
		if name != "default" && name != "rule" {
			return ParameterSpec{}, fmt.Errorf("%s: unknown field %v (default, rule)", key, name)
		}
	}
	var probe SimulationParameters
	field, err := parameterField(&probe, key, true)
	if err != nil {
		return ParameterSpec{}, err
	}
	text, ok := node["rule"].(string)
	if !ok {
		return ParameterSpec{}, fmt.Errorf("%s: rule must be a string, got %v", key, node["rule"])
	}
	rule, err := ParseRule(text)
	if err != nil {
		return ParameterSpec{}, fmt.Errorf("%s: %w", key, err)
	}
	if want := map[reflect.Kind]string{reflect.Int: "int", reflect.Float64: "float"}[field.Kind()]; rule.Type != want {
		return ParameterSpec{}, fmt.Errorf("%s: rule type %s, the parameter is %s", key, rule.Type, want)
	}
	var value float64
	switch v := node["default"].(type) {
	case int:
		value = float64(v)
	case float64:
		value = v
	default:
		return ParameterSpec{}, fmt.Errorf("%s: default must be a number, got %v", key, node["default"])
	}
	if !rule.Allows(value) {
		return ParameterSpec{}, fmt.Errorf("%s: default must be %s, got %v", key, rule.Describe(), node["default"])
	}
	return ParameterSpec{Key: key, Default: value, Rule: rule, RuleText: strings.TrimSpace(text)}, nil
}

// Defaults returns the default value of every parameter.
func (s ParameterSpecs) Defaults() SimulationParameters {
	var p SimulationParameters
	for _, spec := range s {
		field, err := parameterField(&p, spec.Key, true)
		if err != nil {
			continue // UnmarshalYAML rejected it already
		}
		if field.Kind() == reflect.Int {
			field.SetInt(int64(spec.Default))
		} else {
			field.SetFloat(spec.Default)
		}
	}
	return p
}

// ParameterDefinition is what the UI gets of a parameter: its default value
// and its rule as written.
type ParameterDefinition struct {
	Default float64 `json:"default"`
	Rule    string  `json:"rule"`
}

// Definitions returns the definition of every parameter by key, for the UI.
func (s ParameterSpecs) Definitions() map[string]ParameterDefinition {
	definitions := make(map[string]ParameterDefinition, len(s))
	for _, spec := range s {
		definitions[spec.Key] = ParameterDefinition{Default: spec.Default, Rule: spec.RuleText}
	}
	return definitions
}

// Validate checks every parameter present in p against its rule. Absent (nil)
// parameters are valid: the server fills them from the defaults.
func (s ParameterSpecs) Validate(p *SimulationParameters) error {
	if p == nil {
		return nil
	}
	var problems []string
	for _, spec := range s {
		field, err := parameterField(p, spec.Key, false)
		if err != nil || !field.IsValid() {
			continue
		}
		var value float64
		if field.Kind() == reflect.Int {
			value = float64(field.Int())
		} else {
			value = field.Float()
		}
		if !spec.Rule.Allows(value) {
			problems = append(problems, fmt.Sprintf("%s must be %s, got %s", spec.Key, spec.Rule.Describe(), strconv.FormatFloat(value, 'g', -1, 64)))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// parameterField returns the leaf field of p addressed by a dotted key of JSON
// field names (populationSize.min). With create, nil pointers on the way are
// allocated; without it, a nil pointer gives an invalid Value (the parameter
// is absent). An unknown key, or one that does not end at a number, is an
// error.
func parameterField(p *SimulationParameters, key string, create bool) (reflect.Value, error) {
	v := reflect.ValueOf(p).Elem()
	parts := strings.Split(key, ".")
	for i, part := range parts {
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("%s: not a parameter", key)
		}
		field, ok := fieldByJSONName(v, part)
		if !ok {
			return reflect.Value{}, fmt.Errorf("%s: not a parameter", key)
		}
		if field.Kind() == reflect.Pointer {
			if field.IsNil() {
				if !create {
					return reflect.Value{}, nil
				}
				field.Set(reflect.New(field.Type().Elem()))
			}
			field = field.Elem()
		}
		if i == len(parts)-1 && field.Kind() != reflect.Int && field.Kind() != reflect.Float64 {
			return reflect.Value{}, fmt.Errorf("%s: not a parameter", key)
		}
		v = field
	}
	return v, nil
}

func fieldByJSONName(v reflect.Value, name string) (reflect.Value, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if strings.Split(t.Field(i).Tag.Get("json"), ",")[0] == name {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// parameterKeys lists the dotted keys of every numeric leaf of
// SimulationParameters.
func parameterKeys() []string {
	var keys []string
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			ft := f.Type
			if ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				walk(ft, prefix+name+".")
			} else {
				keys = append(keys, prefix+name)
			}
		}
	}
	walk(reflect.TypeOf(SimulationParameters{}), "")
	return keys
}
