package steps

import (
	"context"
	"csp2d/test/support/api"
	"csp2d/test/support/httpclient"
	"csp2d/test/support/oracle"
	"csp2d/test/support/server"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// unknownID is a well-formed id that never exists.
const unknownID = "00000000000000000000000000000000"

// World is the per-scenario state shared by all steps.
type World struct {
	suite *Suite

	srv       *server.Server
	ownServer bool
	client    *httpclient.Client
	logOffset int64

	expectPanic bool

	// Current request.
	req     *api.StartRequest // nil when the fixture is not valid JSON
	raw     []byte            // raw fixture body (used when req is nil)
	fixture string
	inst    *oracle.Instance

	// Last HTTP exchange (non-fatal: assertions inspect it).
	resp    *httpclient.Response
	respErr error

	// Jobs.
	jobID   string
	jobs    []string
	history []api.JobStatus
	result  *api.Result

	pollFailures []string

	// Remembered values for before/after comparisons.
	rememberedEpoch   int
	rememberedFitness float64

	// Several jobs started by one step (concurrency / resource scenarios).
	multi []multiJob
	// Outcomes of concurrently issued calls.
	concurrent []callOutcome
	// Failures collected by loops (churn, repeated calls).
	loopFailures []string
	// HTTP status codes of a batch of start requests.
	startCodes []int
	// Server memory baseline (bytes).
	memBaseline int64
}

// multiJob is one of several jobs started in a scenario.
type multiJob struct {
	id      string
	fixture string
	inst    *oracle.Instance
}

// callOutcome is the result of one concurrently issued call.
type callOutcome struct {
	action string
	resp   *httpclient.Response
	err    error
}

func (w *World) before(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	for _, tag := range sc.Tags {
		if tag.Name == "@expects-panic" {
			w.expectPanic = true
		}
	}
	return ctx, nil
}

func (w *World) after(ctx context.Context, sc *godog.Scenario, scenarioErr error) (context.Context, error) {
	if w.srv == nil {
		return ctx, nil
	}
	// Best-effort cleanup of jobs created by this scenario.
	if w.srv.Alive() {
		quick := w.client.WithTimeout(3 * time.Second)
		for _, id := range w.jobs {
			_, _ = quick.Delete(id)
		}
	}
	scenarioLog := w.srv.LogSince(w.logOffset)
	if scenarioErr != nil {
		fmt.Printf("\n--- server log for scenario %q (%s) ---\n%s\n--- end of server log ---\n",
			sc.Name, w.srv.LogPath, tail(scenarioLog, 40))
	}
	var hookErr error
	if !w.expectPanic {
		if fatal := server.FindFatal(scenarioLog); len(fatal) > 0 {
			hookErr = fmt.Errorf("server log contains fatal markers:\n  %s", strings.Join(fatal, "\n  "))
		}
	}
	if !w.srv.Alive() && hookErr == nil {
		hookErr = fmt.Errorf("server process died: %v", w.srv.ExitError())
	}
	if w.ownServer {
		w.suite.stopServer(w.srv)
	}
	return ctx, hookErr
}

// useServer attaches the scenario to a server.
func (w *World) useServer(srv *server.Server, own bool) {
	w.srv = srv
	w.ownServer = own
	w.client = httpclient.New(srv.URL)
	w.logOffset = srv.LogSize()
}

// ensureServer attaches the default shared server when no server step ran.
func (w *World) ensureServer() error {
	if w.srv != nil {
		return nil
	}
	srv, err := w.suite.sharedServer(w.suite.cfg.DefaultProfile)
	if err != nil {
		return err
	}
	w.useServer(srv, false)
	return nil
}

// record stores the outcome of a request; it never fails the step itself, so
// later steps can assert on status codes, errors and timing.
func (w *World) record(resp *httpclient.Response, err error) {
	w.resp, w.respErr = resp, err
}

// last returns the last response or the transport error that replaced it.
func (w *World) last() (*httpclient.Response, error) {
	if w.respErr != nil {
		if errors.Is(w.respErr, httpclient.ErrTimeout) {
			return nil, fmt.Errorf("request hung: %w", w.respErr)
		}
		return nil, w.respErr
	}
	if w.resp == nil {
		return nil, errors.New("no request has been sent yet")
	}
	return w.resp, nil
}

// requireStatus asserts the last response had the given code.
func (w *World) requireStatus(code int) (*httpclient.Response, error) {
	resp, err := w.last()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != code {
		return nil, fmt.Errorf("expected HTTP %d, got %s", code, resp)
	}
	return resp, nil
}

func (w *World) requireJob() error {
	if w.jobID == "" {
		return errors.New("no simulation has been started in this scenario")
	}
	return nil
}

func (w *World) requireInstance() error {
	if w.inst == nil {
		return errors.New("no valid start request (fixture) in this scenario")
	}
	return nil
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
