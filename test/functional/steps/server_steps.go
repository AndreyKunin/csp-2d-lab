package steps

import (
	"csp2d/test/support/api"
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
	ctx.Step(`^I request the default parameters$`, w.requestDefaultParameters)

	ctx.Step(`^the response status is (\d+)$`, w.responseStatusIs)
	ctx.Step(`^the response status is successful$`, w.responseIsSuccessful)
	ctx.Step(`^the response status is not successful$`, w.responseIsNotSuccessful)
	ctx.Step(`^the response arrives within (\d+(?:\.\d+)?) seconds?$`, w.responseArrivesWithin)
	ctx.Step(`^the error message contains "([^"]*)"$`, w.errorMessageContains)
	ctx.Step(`^the response header "([^"]*)" is "([^"]*)"$`, w.responseHeaderIs)
	ctx.Step(`^the response header "([^"]*)" contains "([^"]*)"$`, w.responseHeaderContains)
	ctx.Step(`^the response body contains "([^"]*)"$`, w.responseBodyContains)
	ctx.Step(`^the response body is "([^"]*)"$`, w.responseBodyIs)
	ctx.Step(`^the default parameters match the server config$`, w.defaultParametersMatchConfig)
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

func (w *World) requestDefaultParameters() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.DefaultParameters())
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

// configDefaults mirrors the defaults block of application.yaml.
type configDefaults struct {
	Defaults struct {
		SimulationParameters struct {
			PopulationSize struct {
				Min int `yaml:"min"`
				Max int `yaml:"max"`
			} `yaml:"populationSize"`
			EliteCount     int     `yaml:"eliteCount"`
			MutationRate   float64 `yaml:"mutationRate"`
			MinRate        float64 `yaml:"minRate"`
			MaxRate        float64 `yaml:"maxRate"`
			MaxEpochsCount int     `yaml:"maxEpochsCount"`
			WindowSize     int     `yaml:"windowSize"`
		} `yaml:"simulationParameters"`
	} `yaml:"defaults"`
}

func (w *World) defaultParametersMatchConfig() error {
	resp, err := w.requireStatus(http.StatusOK)
	if err != nil {
		return err
	}
	var got api.Parameters
	if err := resp.Decode(&got); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(w.srv.WorkDir, "application.yaml"))
	if err != nil {
		return err
	}
	var cfg configDefaults
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}
	want := cfg.Defaults.SimulationParameters
	var diffs []string
	check := func(name string, ok bool, got, want any) {
		if !ok {
			diffs = append(diffs, fmt.Sprintf("%s: got %v, want %v", name, got, want))
		}
	}
	if got.PopulationSize == nil {
		diffs = append(diffs, "populationSize missing")
	} else {
		check("populationSize.min", got.PopulationSize.Min == want.PopulationSize.Min, got.PopulationSize.Min, want.PopulationSize.Min)
		check("populationSize.max", got.PopulationSize.Max == want.PopulationSize.Max, got.PopulationSize.Max, want.PopulationSize.Max)
	}
	checkInt := func(name string, g *int, want int) {
		if g == nil {
			diffs = append(diffs, name+" missing")
			return
		}
		check(name, *g == want, *g, want)
	}
	checkFloat := func(name string, g *float64, want float64) {
		if g == nil {
			diffs = append(diffs, name+" missing")
			return
		}
		check(name, math.Abs(*g-want) < 1e-12, *g, want)
	}
	checkInt("eliteCount", got.EliteCount, want.EliteCount)
	checkFloat("mutationRate", got.MutationRate, want.MutationRate)
	checkFloat("minRate", got.MinRate, want.MinRate)
	checkFloat("maxRate", got.MaxRate, want.MaxRate)
	checkInt("maxEpochsCount", got.MaxEpochsCount, want.MaxEpochsCount)
	checkInt("windowSize", got.WindowSize, want.WindowSize)
	if len(diffs) > 0 {
		return fmt.Errorf("default parameters differ from config:\n  %s", strings.Join(diffs, "\n  "))
	}
	return nil
}

// parseNumber parses a step argument as float64.
func parseNumber(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
