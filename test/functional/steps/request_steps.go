package steps

import (
	"csp2d/test/support/api"
	"csp2d/test/support/oracle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cucumber/godog"
)

func registerRequestSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^a start request from fixture "([^"]*)"$`, w.startRequestFromFixture)
	ctx.Step(`^the request parameter "([^"]*)" is (\S+)$`, w.requestParameterIs)
	ctx.Step(`^the request has no parameters$`, w.requestHasNoParameters)
	ctx.Step(`^I start the simulation$`, w.startSimulation)
	ctx.Step(`^a running simulation for fixture "([^"]*)"$`, w.runningSimulationForFixture)
	ctx.Step(`^a running simulation for fixture "([^"]*)" with (\S+) (\S+)$`, w.runningSimulationForFixtureWith)
	ctx.Step(`^a running simulation for fixture "([^"]*)" with parameters:$`, w.runningSimulationForFixtureWithTable)
	ctx.Step(`^a long-running simulation for fixture "([^"]*)"$`, w.longRunningSimulation)
}

// A long-running job: the early stop is only evaluated once
// epochNumber >= windowSize, so a large window keeps the job running until the
// scenario pauses or deletes it. The window must stay moderate: the server
// pre-allocates 8 bytes per window slot per job (10^6 -> 8 MB, ~11 min of epochs).
const (
	longRunningEpochs = "100000000"
	longRunningWindow = "1000000"
)

func (w *World) longRunningSimulation(id string) error {
	if err := w.startRequestFromFixture(id); err != nil {
		return err
	}
	if err := w.requestParameterIs("maxEpochsCount", longRunningEpochs); err != nil {
		return err
	}
	if err := w.requestParameterIs("windowSize", longRunningWindow); err != nil {
		return err
	}
	return w.startRequired()
}

func (w *World) fixturePath(id string) string {
	return filepath.Join(w.suite.cfg.FixturesDir, "inputs", id+".json")
}

// loadFixture reads a request body. Bodies that are not valid JSON (malformed
// request fixtures) are kept raw and sent verbatim.
func (w *World) loadFixture(id string) error {
	data, err := os.ReadFile(w.fixturePath(id))
	if err != nil {
		return fmt.Errorf("fixture %q: %w", id, err)
	}
	w.fixture, w.raw, w.req, w.inst = id, data, nil, nil
	var req api.StartRequest
	if json.Unmarshal(data, &req) == nil {
		w.req = &req
		// Ambiguous inputs (same label, different sizes) have no oracle instance.
		if inst, err := oracle.NewInstance(&req); err == nil {
			w.inst = inst
		}
	}
	return nil
}

func (w *World) startRequestFromFixture(id string) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	return w.loadFixture(id)
}

func (w *World) requestParameterIs(key, value string) error {
	if w.req == nil {
		return fmt.Errorf("fixture %q is not a JSON request; parameters cannot be set", w.fixture)
	}
	if w.req.Parameters == nil {
		w.req.Parameters = &api.Parameters{}
	}
	return setParameter(w.req.Parameters, key, value)
}

func (w *World) requestHasNoParameters() error {
	if w.req == nil {
		return fmt.Errorf("fixture %q is not a JSON request", w.fixture)
	}
	w.req.Parameters = nil
	return nil
}

// setParameter sets a dotted parameter key. populationSize.min/max should be
// set together: the server applies both bounds.
func setParameter(p *api.Parameters, key, value string) error {
	asInt := func() (*int, error) {
		v, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %q is not an integer", key, value)
		}
		return &v, nil
	}
	asFloat := func() (*float64, error) {
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %s: %q is not a number", key, value)
		}
		return &v, nil
	}
	var err error
	switch key {
	case "populationSize.min", "populationSize.max":
		var v *int
		if v, err = asInt(); err != nil {
			return err
		}
		if p.PopulationSize == nil {
			p.PopulationSize = &api.PopulationSize{}
		}
		if key == "populationSize.min" {
			p.PopulationSize.Min = *v
		} else {
			p.PopulationSize.Max = *v
		}
	case "eliteCount":
		p.EliteCount, err = asInt()
	case "maxEpochsCount":
		p.MaxEpochsCount, err = asInt()
	case "windowSize":
		p.WindowSize, err = asInt()
	case "mutationRate":
		p.MutationRate, err = asFloat()
	case "crossoverRate":
		p.CrossoverRate, err = asFloat()
	case "minRate":
		p.MinRate, err = asFloat()
	case "maxRate":
		p.MaxRate, err = asFloat()
	default:
		return fmt.Errorf("unknown parameter %q", key)
	}
	return err
}

// requestBody returns the JSON body for the current request.
func (w *World) requestBody() ([]byte, error) {
	if w.req != nil {
		return json.Marshal(w.req)
	}
	if w.raw != nil {
		return w.raw, nil
	}
	return nil, fmt.Errorf("no start request in this scenario")
}

func (w *World) startSimulation() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	body, err := w.requestBody()
	if err != nil {
		return err
	}
	resp, err := w.client.StartRaw(body)
	w.record(resp, err)
	if err == nil && resp.StatusCode == http.StatusCreated {
		var st api.JobStatus
		if err := resp.Decode(&st); err != nil {
			return err
		}
		w.jobID = st.ID
		w.jobs = append(w.jobs, st.ID)
		w.result = nil
		w.history = nil
	}
	return nil
}

func (w *World) startRequired() error {
	if err := w.startSimulation(); err != nil {
		return err
	}
	if _, err := w.requireStatus(http.StatusCreated); err != nil {
		return fmt.Errorf("could not start simulation: %w", err)
	}
	return nil
}

func (w *World) runningSimulationForFixture(id string) error {
	if err := w.startRequestFromFixture(id); err != nil {
		return err
	}
	return w.startRequired()
}

func (w *World) runningSimulationForFixtureWith(id, key, value string) error {
	if err := w.startRequestFromFixture(id); err != nil {
		return err
	}
	if err := w.requestParameterIs(key, value); err != nil {
		return err
	}
	return w.startRequired()
}

func (w *World) runningSimulationForFixtureWithTable(id string, table *godog.Table) error {
	if err := w.startRequestFromFixture(id); err != nil {
		return err
	}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("parameter table rows must have 2 cells (key | value)")
		}
		if err := w.requestParameterIs(row.Cells[0].Value, row.Cells[1].Value); err != nil {
			return err
		}
	}
	return w.startRequired()
}
