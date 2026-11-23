package steps

import (
	"csp2d/test/support/api"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v2"
)

func registerServerSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^the server is running with config "([^"]*)"$`, w.serverRunningWithConfig)
	ctx.Step(`^a server started with config "([^"]*)"$`, w.serverStartedWithConfig)
	ctx.Step(`^the server is healthy$`, w.serverIsHealthy)

	ctx.Step(`^I send a "(GET|POST|PUT|DELETE)" request to "([^"]*)"$`, w.sendRequest)
	ctx.Step(`^I request the parameter definitions$`, w.requestParameterDefinitions)

	ctx.Step(`^the response status is (\d+)$`, w.responseStatusIs)
	ctx.Step(`^the response status is successful$`, w.responseIsSuccessful)
	ctx.Step(`^the response status is not successful$`, w.responseIsNotSuccessful)
	ctx.Step(`^the response arrives within (\d+(?:\.\d+)?) seconds?$`, w.responseArrivesWithin)
	ctx.Step(`^the error message contains "([^"]*)"$`, w.errorMessageContains)
	ctx.Step(`^the response header "([^"]*)" is "([^"]*)"$`, w.responseHeaderIs)
	ctx.Step(`^the response header "([^"]*)" contains "([^"]*)"$`, w.responseHeaderContains)
	ctx.Step(`^the response body contains "([^"]*)"$`, w.responseBodyContains)
	ctx.Step(`^the response body is "([^"]*)"$`, w.responseBodyIs)
	ctx.Step(`^the parameter rule of "([^"]*)" is "([^"]*)"$`, w.parameterRuleIs)
	ctx.Step(`^the response has (\d+) parameter definitions$`, w.parameterDefinitionCount)
	ctx.Step(`^the parameter definitions match the server config$`, w.parameterDefinitionsMatchConfig)
}

func (w *World) serverRunningWithConfig(profile string) error {
	srv, err := w.suite.sharedServer(profile)
	if err != nil {
		return err
	}
	w.useServer(srv, false)
	return nil
}

func (w *World) serverStartedWithConfig(profile string) error {
	srv, err := w.suite.startServer(profile)
	if err != nil {
		return err
	}
	w.useServer(srv, true)
	return nil
}

func (w *World) serverIsHealthy() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	if !w.srv.Alive() {
		return fmt.Errorf("server process is not running: %v", w.srv.ExitError())
	}
	resp, err := w.client.WithTimeout(2 * time.Second).Heartbeat()
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat: %s", resp)
	}
	return nil
}

func (w *World) sendRequest(method, path string) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.Do(method, path, nil, ""))
	return nil
}

func (w *World) requestParameterDefinitions() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.ParameterDefinitions())
	return nil
}

func (w *World) responseStatusIs(code int) error {
	_, err := w.requireStatus(code)
	return err
}

func (w *World) responseIsSuccessful() error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if !resp.OK() {
		return fmt.Errorf("expected a 2xx response, got %s", resp)
	}
	return nil
}

func (w *World) responseIsNotSuccessful() error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if resp.OK() {
		return fmt.Errorf("expected a non-2xx response, got %s", resp)
	}
	return nil
}

func (w *World) responseArrivesWithin(seconds float64) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	limit := time.Duration(seconds * float64(time.Second))
	if resp.Duration > limit {
		return fmt.Errorf("response took %s, limit %s", resp.Duration.Round(time.Millisecond), limit)
	}
	return nil
}

func (w *World) errorMessageContains(text string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if msg := resp.ErrorMessage(); !strings.Contains(strings.ToLower(msg), strings.ToLower(text)) {
		return fmt.Errorf("error message %q does not contain %q", msg, text)
	}
	return nil
}

func (w *World) responseHeaderIs(name, value string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if got := resp.Header.Get(name); got != value {
		return fmt.Errorf("header %s = %q, want %q", name, got, value)
	}
	return nil
}

func (w *World) responseHeaderContains(name, value string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if got := resp.Header.Get(name); !strings.Contains(got, value) {
		return fmt.Errorf("header %s = %q does not contain %q", name, got, value)
	}
	return nil
}

func (w *World) responseBodyContains(text string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if !strings.Contains(string(resp.Body), text) {
		return fmt.Errorf("body does not contain %q: %s", text, resp)
	}
	return nil
}

func (w *World) responseBodyIs(text string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if got := strings.TrimSpace(string(resp.Body)); got != text {
		return fmt.Errorf("body = %q, want %q", got, text)
	}
	return nil
}

// parameterDefinitions decodes the last response of GET parameter-definitions
// (key -> default and rule).
func (w *World) parameterDefinitions() (map[string]api.ParameterDefinition, error) {
	resp, err := w.last()
	if err != nil {
		return nil, err
	}
	var definitions map[string]api.ParameterDefinition
	if err := json.Unmarshal(resp.Body, &definitions); err != nil {
		return nil, fmt.Errorf("parameter definitions: %w: %s", err, resp)
	}
	return definitions, nil
}

func (w *World) parameterRuleIs(key, rule string) error {
	definitions, err := w.parameterDefinitions()
	if err != nil {
		return err
	}
	if got, ok := definitions[key]; !ok || got.Rule != rule {
		return fmt.Errorf("rule of %s = %q (present %v), want %q", key, got.Rule, ok, rule)
	}
	return nil
}

func (w *World) parameterDefinitionCount(n int) error {
	definitions, err := w.parameterDefinitions()
	if err != nil {
		return err
	}
	if len(definitions) != n {
		return fmt.Errorf("%d parameter definitions, want %d: %v", len(definitions), n, definitions)
	}
	return nil
}

// parameterDefinitionsMatchConfig compares the definitions the server returns
// with defaults.simulationParameters of the profile's application.yaml (each
// parameter: default and rule; a group such as populationSize nests its
// parameters, so populationSize.min is a dotted key).
func (w *World) parameterDefinitionsMatchConfig() error {
	if _, err := w.requireStatus(http.StatusOK); err != nil {
		return err
	}
	got, err := w.parameterDefinitions()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(w.srv.WorkDir, "application.yaml"))
	if err != nil {
		return err
	}
	var cfg struct {
		Defaults struct {
			SimulationParameters map[string]any `yaml:"simulationParameters"`
		} `yaml:"defaults"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	want := map[string]api.ParameterDefinition{}
	var walk func(node map[any]any, prefix string)
	walk = func(node map[any]any, prefix string) {
		for name, value := range node {
			child, _ := value.(map[any]any)
			key := prefix + fmt.Sprint(name)
			if rule, ok := child["rule"].(string); ok {
				def, _ := strconv.ParseFloat(fmt.Sprint(child["default"]), 64)
				want[key] = api.ParameterDefinition{Default: def, Rule: rule}
				continue
			}
			walk(child, key+".")
		}
	}
	for name, value := range cfg.Defaults.SimulationParameters {
		walk(map[any]any{name: value}, "")
	}
	if len(want) == 0 {
		return fmt.Errorf("no parameters in %s", w.srv.WorkDir)
	}
	var diffs []string
	for key, d := range want {
		g, ok := got[key]
		switch {
		case !ok:
			diffs = append(diffs, key+": missing")
		case g.Rule != d.Rule || math.Abs(g.Default-d.Default) > 1e-12:
			diffs = append(diffs, fmt.Sprintf("%s: got %v %q, want %v %q", key, g.Default, g.Rule, d.Default, d.Rule))
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			diffs = append(diffs, key+": not in the config")
		}
	}
	if len(diffs) > 0 {
		return fmt.Errorf("parameter definitions differ from config:\n  %s", strings.Join(diffs, "\n  "))
	}
	return nil
}

// parseNumber parses a step argument as float64.
func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
