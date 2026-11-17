package steps

import (
	"csp2d/test/support/api"
	"csp2d/test/support/httpclient"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

func registerResultSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^I fetch the result$`, w.fetchResultStep)
	ctx.Step(`^every solution is valid$`, w.everySolutionIsValid)
	ctx.Step(`^the solutions are sorted by fitness$`, w.solutionsSorted)
	ctx.Step(`^the result has between (\d+) and (\d+) solutions$`, w.resultHasBetween)
	ctx.Step(`^the best solution uses (\d+) sheets?$`, w.bestUsesSheets)
	ctx.Step(`^the best solution uses at most (\d+) sheets?$`, w.bestUsesAtMostSheets)
	ctx.Step(`^the best solution waste is (\S+)$`, w.bestWasteIs)
	ctx.Step(`^at least (\d+) of (\d+) runs reach (\d+) sheets? within (\d+) epochs$`, w.statisticalRuns)
}

// fetchResult requests the result; it must answer 200 with a decodable body.
func (w *World) fetchResult() (*api.Result, error) {
	if err := w.requireJob(); err != nil {
		return nil, err
	}
	resp, err := w.client.Result(w.jobID)
	w.record(resp, err)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("result request failed: %s", resp)
	}
	var res api.Result
	if err := resp.Decode(&res); err != nil {
		return nil, err
	}
	w.result = &res
	return &res, nil
}

func (w *World) fetchResultStep() error {
	_, err := w.fetchResult()
	return err
}

func (w *World) currentResult() (*api.Result, error) {
	if w.result != nil {
		return w.result, nil
	}
	return w.fetchResult()
}

func (w *World) everySolutionIsValid() error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	res, err := w.currentResult()
	if err != nil {
		return err
	}
	if len(res.Solutions) == 0 {
		return fmt.Errorf("result has no solutions")
	}
	if v := w.inst.ValidateResult(res); len(v) > 0 {
		return fmt.Errorf("invalid solutions:\n  %s", strings.Join(v, "\n  "))
	}
	return nil
}

func (w *World) solutionsSorted() error {
	res, err := w.currentResult()
	if err != nil {
		return err
	}
	for i := 1; i < len(res.Solutions); i++ {
		if res.Solutions[i].Fitness < res.Solutions[i-1].Fitness {
			return fmt.Errorf("solutions not sorted at index %d", i)
		}
	}
	return nil
}

func (w *World) resultHasBetween(min, max int) error {
	res, err := w.currentResult()
	if err != nil {
		return err
	}
	if n := len(res.Solutions); n < min || n > max {
		return fmt.Errorf("result has %d solutions, want %d..%d", n, min, max)
	}
	return nil
}

func (w *World) best() (*api.Solution, error) {
	res, err := w.currentResult()
	if err != nil {
		return nil, err
	}
	if len(res.Solutions) == 0 {
		return nil, fmt.Errorf("result has no solutions")
	}
	return &res.Solutions[0], nil
}

func (w *World) bestUsesSheets(n int) error {
	b, err := w.best()
	if err != nil {
		return err
	}
	if len(b.Sheets) != n {
		return fmt.Errorf("best solution uses %d sheets, want %d", len(b.Sheets), n)
	}
	return nil
}

func (w *World) bestUsesAtMostSheets(n int) error {
	b, err := w.best()
	if err != nil {
		return err
	}
	if len(b.Sheets) > n {
		return fmt.Errorf("best solution uses %d sheets, want at most %d", len(b.Sheets), n)
	}
	return nil
}

func (w *World) bestWasteIs(value string) error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	want, err := parseNumber(value)
	if err != nil {
		return err
	}
	b, err := w.best()
	if err != nil {
		return err
	}
	if got := w.inst.Waste(*b); math.Abs(got-want) > 1e-9*math.Max(1, want) {
		return fmt.Errorf("best solution waste is %g, want %g", got, want)
	}
	return nil
}

// statisticalRuns starts R independent runs of the current request (with
// maxEpochsCount = epochs), waits for them, validates every result and
// requires at least k runs whose best solution uses <= sheets sheets.
func (w *World) statisticalRuns(k, r, sheets, epochs int) error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	if w.req == nil {
		return fmt.Errorf("no start request")
	}
	type outcome struct {
		sheets int
		err    error
	}
	outcomes := make([]outcome, r)
	timeout := time.Duration(w.suite.cfg.RunTimeout) * time.Second
	var wg sync.WaitGroup
	var idsMu sync.Mutex
	for i := 0; i < r; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outcomes[i].sheets, outcomes[i].err = w.oneRun(epochs, timeout, func(id string) {
				idsMu.Lock()
				w.jobs = append(w.jobs, id)
				idsMu.Unlock()
			})
		}(i)
	}
	wg.Wait()
	reached := 0
	var report []string
	for i, o := range outcomes {
		if o.err != nil {
			return fmt.Errorf("run %d: %w", i, o.err)
		}
		report = append(report, fmt.Sprintf("%d", o.sheets))
		if o.sheets <= sheets {
			reached++
		}
	}
	fmt.Printf("    statistical runs: best sheets per run = [%s]\n", strings.Join(report, ", "))
	if reached < k {
		return fmt.Errorf("only %d of %d runs reached %d sheets (best per run: %s)", reached, r, sheets, strings.Join(report, ", "))
	}
	return nil
}

func (w *World) oneRun(epochs int, timeout time.Duration, track func(string)) (int, error) {
	data, err := json.Marshal(w.req)
	if err != nil {
		return 0, err
	}
	var req api.StartRequest
	if err := json.Unmarshal(data, &req); err != nil { // deep copy
		return 0, err
	}
	if req.Parameters == nil {
		req.Parameters = &api.Parameters{}
	}
	req.Parameters.MaxEpochsCount = api.Int(epochs)

	var started api.JobStatus
	resp, err := w.client.Start(&req)
	if _, err := httpclient.Expect(resp, err, http.StatusCreated, &started); err != nil {
		return 0, err
	}
	track(started.ID)

	deadline := time.Now().Add(timeout)
	for {
		var st api.JobStatus
		resp, err := w.client.Status(started.ID)
		if _, err := httpclient.Expect(resp, err, http.StatusOK, &st); err != nil {
			return 0, err
		}
		if st.Status == api.StatusError {
			return 0, fmt.Errorf("job %s failed: %v", started.ID, deref(st.Error))
		}
		if st.Status == api.StatusCompleted {
			break
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("job %s not finished within %s (epoch %d)", started.ID, timeout, epochOf(&st))
		}
		time.Sleep(pollInterval)
	}

	var res api.Result
	resp, err = w.client.Result(started.ID)
	if _, err := httpclient.Expect(resp, err, http.StatusOK, &res); err != nil {
		return 0, err
	}
	if len(res.Solutions) == 0 {
		return 0, fmt.Errorf("job %s returned no solutions", started.ID)
	}
	if v := w.inst.ValidateResult(&res); len(v) > 0 {
		return 0, fmt.Errorf("job %s returned invalid solutions:\n  %s", started.ID, strings.Join(v, "\n  "))
	}
	return len(res.Solutions[0].Sheets), nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}
