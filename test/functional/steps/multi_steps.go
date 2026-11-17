package steps

import (
	"bytes"
	"crypto/rand"
	"csp2d/test/support/api"
	"csp2d/test/support/httpclient"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// Steps for several jobs, concurrent calls, server resources and extra PDF
// checks.
func registerMultiSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^I start these simulations:$`, w.startTheseSimulations)
	ctx.Step(`^all started simulations complete within (\d+) seconds$`, w.allMultiComplete)
	ctx.Step(`^every started simulation has a valid result for its own input$`, w.everyMultiResultValid)
	ctx.Step(`^I run concurrently:$`, w.runConcurrently)
	ctx.Step(`^every concurrent call answered within (\d+) seconds$`, w.everyConcurrentAnswered)
	ctx.Step(`^I start and delete (\d+) simulations for fixture "([^"]*)" with a (\d+) ms pause$`, w.churn)
	ctx.Step(`^I start (\d+) long-running simulations for fixture "([^"]*)"$`, w.startManyLongRunning)
	ctx.Step(`^at least one start was answered with status (\d+)$`, w.someStartAnswered)

	ctx.Step(`^I record the server memory baseline$`, w.recordMemoryBaseline)
	ctx.Step(`^the server memory returns to within (\d+)% of the baseline within (\d+) seconds$`, w.memoryReturnsToBaseline)
	ctx.Step(`^the server memory stays below (\d+) MB while I poll for (\d+) seconds$`, w.memoryStaysBelow)
	ctx.Step(`^the server memory grew by less than (\d+) MB$`, w.memoryGrewLessThan)
	ctx.Step(`^I start and stop a simulation for fixture "([^"]*)" (\d+) times$`, w.startStopTimes)
	ctx.Step(`^the job reaches at least (\d+) epochs per second$`, w.epochsPerSecond)

	ctx.Step(`^I request a PDF for fixture "([^"]*)" with a (\d+) MB image$`, w.requestPdfBigImage)
	ctx.Step(`^I request a PDF for fixture "([^"]*)" with an invalid image$`, w.requestPdfInvalidImage)
	ctx.Step(`^the response is an error with a message$`, w.responseIsErrorWithMessage)
	ctx.Step(`^the PDF contains the text "([^"]*)"$`, w.pdfContainsText)
}

// ---- several jobs ----------------------------------------------------------

func (w *World) startOne(fixture string, params map[string]string) (*multiJob, error) {
	if err := w.loadFixture(fixture); err != nil {
		return nil, err
	}
	if w.req == nil {
		return nil, fmt.Errorf("fixture %s is not a JSON request", fixture)
	}
	for k, v := range params {
		if w.req.Parameters == nil {
			w.req.Parameters = &api.Parameters{}
		}
		if err := setParameter(w.req.Parameters, k, v); err != nil {
			return nil, err
		}
	}
	var st api.JobStatus
	resp, err := w.client.Start(w.req)
	if _, err := httpclient.Expect(resp, err, http.StatusCreated, &st); err != nil {
		return nil, err
	}
	w.jobs = append(w.jobs, st.ID)
	return &multiJob{id: st.ID, fixture: fixture, inst: w.inst}, nil
}

// startTheseSimulations: table rows "fixture | maxEpochsCount" (no header).
func (w *World) startTheseSimulations(table *godog.Table) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.multi = nil
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("rows must be: fixture | maxEpochsCount")
		}
		job, err := w.startOne(row.Cells[0].Value, map[string]string{"maxEpochsCount": row.Cells[1].Value})
		if err != nil {
			return err
		}
		w.multi = append(w.multi, *job)
	}
	return nil
}

func (w *World) multiStatus(id string) (*api.JobStatus, error) {
	var st api.JobStatus
	resp, err := w.client.Status(id)
	if _, err := httpclient.Expect(resp, err, http.StatusOK, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (w *World) allMultiComplete(seconds int) error {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for _, job := range w.multi {
		for {
			st, err := w.multiStatus(job.id)
			if err != nil {
				return err
			}
			if st.Status == api.StatusError {
				return fmt.Errorf("job %s (%s) failed: %s", job.id, job.fixture, deref(st.Error))
			}
			if st.Status == api.StatusCompleted {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("job %s (%s) not completed within %ds", job.id, job.fixture, seconds)
			}
			time.Sleep(pollInterval)
		}
	}
	return nil
}

func (w *World) everyMultiResultValid() error {
	for _, job := range w.multi {
		var res api.Result
		resp, err := w.client.Result(job.id)
		if _, err := httpclient.Expect(resp, err, http.StatusOK, &res); err != nil {
			return err
		}
		if len(res.Solutions) == 0 {
			return fmt.Errorf("job %s (%s) has no solutions", job.id, job.fixture)
		}
		if v := job.inst.ValidateResult(&res); len(v) > 0 {
			return fmt.Errorf("job %s (%s):\n  %s", job.id, job.fixture, strings.Join(v, "\n  "))
		}
	}
	return nil
}

// runConcurrently fires the listed actions (one per row) on the current job
// at the same moment.
func (w *World) runConcurrently(table *godog.Table) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	actions := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		actions = append(actions, row.Cells[0].Value)
	}
	w.concurrent = make([]callOutcome, len(actions))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, action := range actions {
		wg.Add(1)
		go func(i int, action string) {
			defer wg.Done()
			<-start
			var resp *httpclient.Response
			var err error
			switch action {
			case "pause":
				resp, err = w.client.Pause(w.jobID)
			case "resume":
				resp, err = w.client.Resume(w.jobID)
			case "delete":
				resp, err = w.client.Delete(w.jobID)
			case "status":
				resp, err = w.client.Status(w.jobID)
			case "result":
				resp, err = w.client.Result(w.jobID)
			case "configure":
				resp, err = w.client.Configure(w.jobID, &api.Parameters{
					MutationRate: api.Float(0.05), MaxEpochsCount: api.Int(100000000), WindowSize: api.Int(1000000)})
			default:
				err = fmt.Errorf("unknown action %q", action)
			}
			w.concurrent[i] = callOutcome{action: action, resp: resp, err: err}
		}(i, action)
	}
	close(start)
	wg.Wait()
	return nil
}

func (w *World) everyConcurrentAnswered(seconds int) error {
	limit := time.Duration(seconds) * time.Second
	var problems []string
	for _, o := range w.concurrent {
		switch {
		case o.err != nil:
			problems = append(problems, fmt.Sprintf("%s: %v", o.action, o.err))
		case o.resp.Duration > limit:
			problems = append(problems, fmt.Sprintf("%s took %s", o.action, o.resp.Duration))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("concurrent calls failed:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func (w *World) churn(n int, fixture string, pauseMs int) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.loopFailures = nil
	for i := 0; i < n; i++ {
		job, err := w.startOne(fixture, map[string]string{"maxEpochsCount": "100000000", "windowSize": "1000000"})
		if err != nil {
			w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d start: %v", i, err))
			continue
		}
		time.Sleep(time.Duration(pauseMs) * time.Millisecond)
		resp, err := w.client.Delete(job.id)
		if err != nil || resp.StatusCode != http.StatusOK {
			w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d delete: %v %v", i, err, resp))
		}
	}
	return nil
}

func (w *World) startManyLongRunning(n int, fixture string) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	if err := w.loadFixture(fixture); err != nil {
		return err
	}
	w.req.Parameters = &api.Parameters{MaxEpochsCount: api.Int(100000000), WindowSize: api.Int(1000000)}
	w.startCodes = nil
	for i := 0; i < n; i++ {
		resp, err := w.client.Start(w.req)
		if err != nil {
			return err
		}
		w.startCodes = append(w.startCodes, resp.StatusCode)
		var st api.JobStatus
		if resp.StatusCode == http.StatusCreated && resp.Decode(&st) == nil {
			w.jobs = append(w.jobs, st.ID)
		}
	}
	return nil
}

func (w *World) someStartAnswered(code int) error {
	for _, c := range w.startCodes {
		if c == code {
			return nil
		}
	}
	return fmt.Errorf("no start request was answered with %d (codes: %v)", code, w.startCodes)
}

// ---- resources ---------------------------------------------------------------

const mb = 1024 * 1024

func (w *World) recordMemoryBaseline() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	rss, err := w.srv.RSS()
	if err != nil {
		return err
	}
	w.memBaseline = rss
	fmt.Printf("    server memory baseline: %d MB\n", rss/mb)
	return nil
}

func (w *World) memoryReturnsToBaseline(percent, seconds int) error {
	limit := w.memBaseline + w.memBaseline*int64(percent)/100
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for {
		rss, err := w.srv.RSS()
		if err != nil {
			return err
		}
		if rss <= limit {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server memory %d MB after %ds, baseline %d MB (+%d%% = %d MB)", rss/mb, seconds, w.memBaseline/mb, percent, limit/mb)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (w *World) memoryStaysBelow(limitMB, seconds int) error {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	peak := int64(0)
	for time.Now().Before(deadline) {
		rss, err := w.srv.RSS()
		if err != nil {
			return err
		}
		if rss > peak {
			peak = rss
		}
		if rss > int64(limitMB)*mb {
			return fmt.Errorf("server memory %d MB exceeds %d MB", rss/mb, limitMB)
		}
		if w.jobID != "" {
			if _, err := w.fetchStatus(); err != nil {
				return err
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Printf("    server memory peak: %d MB\n", peak/mb)
	return nil
}

func (w *World) memoryGrewLessThan(limitMB int) error {
	rss, err := w.srv.RSS()
	if err != nil {
		return err
	}
	if growth := rss - w.memBaseline; growth >= int64(limitMB)*mb {
		return fmt.Errorf("server memory grew by %d MB (baseline %d MB, now %d MB)", growth/mb, w.memBaseline/mb, rss/mb)
	}
	return nil
}

func (w *World) startStopTimes(fixture string, n int) error {
	w.loopFailures = nil
	for i := 0; i < n; i++ {
		job, err := w.startOne(fixture, map[string]string{"maxEpochsCount": "100000000", "windowSize": "1000000"})
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
		// Stop as the UI does: pause, then delete.
		if resp, err := w.client.Pause(job.id); err != nil || resp.StatusCode != http.StatusOK {
			w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d pause: %v %v", i, err, resp))
		}
		if resp, err := w.client.Delete(job.id); err != nil || resp.StatusCode != http.StatusOK {
			w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d delete: %v %v", i, err, resp))
		}
		// Safety guard: leaked jobs can take hundreds of MB each; stop before the
		// test host runs out of memory.
		if rss, err := w.srv.RSS(); err == nil && rss > memoryGuard {
			return fmt.Errorf("aborted after %d cycles: server memory %d MB exceeds the %d MB safety guard",
				i+1, rss/mb, memoryGuard/mb)
		}
	}
	return nil
}

// memoryGuard stops memory-hungry loops before they endanger the test host.
const memoryGuard = 2048 * mb

func (w *World) epochsPerSecond(minRate int) error {
	first, err := w.fetchStatus()
	if err != nil {
		return err
	}
	t0 := time.Now()
	time.Sleep(5 * time.Second)
	last, err := w.fetchStatus()
	if err != nil {
		return err
	}
	rate := float64(epochOf(last)-epochOf(first)) / time.Since(t0).Seconds()
	fmt.Printf("    throughput: %.1f epochs/s\n", rate)
	if rate < float64(minRate) {
		return fmt.Errorf("throughput %.1f epochs/s below %d", rate, minRate)
	}
	return nil
}

// ---- PDF ---------------------------------------------------------------------

func (w *World) requestPdfWithImages(fixture string, images []httpclient.PdfImage) error {
	if err := w.startRequestFromFixture(fixture); err != nil {
		return err
	}
	body, err := json.Marshal(w.req)
	if err != nil {
		return err
	}
	w.record(w.client.WithTimeout(30*time.Second).GeneratePdf(body, images))
	return nil
}

func (w *World) requestPdfBigImage(fixture string, sizeMB int) error {
	data := make([]byte, sizeMB*mb)
	_, _ = rand.Read(data)
	return w.requestPdfWithImages(fixture, []httpclient.PdfImage{{Name: "big.png", Data: data}})
}

func (w *World) requestPdfInvalidImage(fixture string) error {
	return w.requestPdfWithImages(fixture, []httpclient.PdfImage{{Name: "bad.png", Data: []byte("definitely not a png")}})
}

func (w *World) responseIsErrorWithMessage() error {
	resp, err := w.last()
	if err != nil {
		return fmt.Errorf("no HTTP response (dropped connection?): %w", err)
	}
	if resp.OK() {
		return fmt.Errorf("expected an error status, got %s", resp)
	}
	if strings.TrimSpace(resp.ErrorMessage()) == "" {
		return fmt.Errorf("error response has no message: %s", resp)
	}
	return nil
}

func (w *World) pdfContainsText(text string) error {
	resp, err := w.requireStatus(http.StatusOK)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "csp2d-pdf-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := pdfapi.ExtractContent(bytes.NewReader(resp.Body), dir, "report", nil, model.NewDefaultConfiguration()); err != nil {
		return fmt.Errorf("cannot read the PDF: %w", err)
	}
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err == nil && bytes.Contains(data, []byte(text)) {
			return nil
		}
	}
	return fmt.Errorf("the PDF does not contain %q", text)
}
