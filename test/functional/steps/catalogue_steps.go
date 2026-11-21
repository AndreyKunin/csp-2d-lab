package steps

import (
	"csp2d/test/support/api"
	"csp2d/test/support/httpclient"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

// Steps used across the functional catalogue: response/status variants, remembered
// values, repeated calls, result and statistics checks.
func registerCatalogueSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^the response status is one of ([\d, ]+)$`, w.responseStatusOneOf)
	ctx.Step(`^the two started jobs have different ids$`, w.twoJobsDifferentIDs)
	ctx.Step(`^the job status is not "([^"]*)"$`, w.jobStatusIsNot)
	ctx.Step(`^the epoch number is exactly (\d+)$`, w.epochExactly)
	ctx.Step(`^the run finished (\d+) cycles?$`, w.finishedCycles)
	ctx.Step(`^the championship has started$`, w.championshipStarted)
	ctx.Step(`^the championship has not started$`, w.championshipNotStarted)
	ctx.Step(`^the epoch number exceeds (\d+) within (\d+) seconds$`, w.epochExceedsWithin)
	ctx.Step(`^I remember the current epoch$`, w.rememberEpoch)
	ctx.Step(`^I configure maxEpochsCount to the remembered epoch$`, w.configureMaxEpochsToRemembered)
	ctx.Step(`^the epoch number is at most the remembered epoch plus (\d+)$`, w.epochAtMostRememberedPlus)
	ctx.Step(`^I pause and resume the simulation (\d+) times$`, w.pauseResumeTimes)
	ctx.Step(`^every repeated call succeeded$`, w.everyLoopCallSucceeded)
	ctx.Step(`^the start is rejected or the job completes without error within (\d+) seconds$`, w.rejectedOrCompletes)

	ctx.Step(`^I fetch the result without waiting$`, w.fetchResultNoCheck)
	ctx.Step(`^I fetch and validate the result (\d+) times$`, w.fetchAndValidateTimes)
	ctx.Step(`^I remember the best result fitness$`, w.rememberBestFitness)
	ctx.Step(`^the best result fitness is not worse than remembered$`, w.bestFitnessNotWorse)
	ctx.Step(`^the solutions only use labels from the request$`, w.onlyRequestLabels)
	ctx.Step(`^the sheet numbers are contiguous from 0$`, w.sheetNumbersContiguous)
	ctx.Step(`^every sheet of the best solution holds a single part$`, w.everySheetSinglePart)
	ctx.Step(`^the result is empty or valid$`, w.resultEmptyOrValid)

	ctx.Step(`^the best fitness vector is consistent with the sheet area$`, w.bestVectorConsistent)
	ctx.Step(`^the selected species are sorted and number (\d+)$`, w.selectedSpeciesSorted)
	ctx.Step(`^the selected species count becomes (\d+) within (\d+) seconds$`, w.selectedSpeciesBecomes)
	ctx.Step(`^the saturation rate stays within 0 and 1$`, w.saturationWithinUnit)
	ctx.Step(`^the window averages are not below the best fitness$`, w.windowAveragesNotBelowBest)
	ctx.Step(`^I wait (\d+) seconds$`, w.waitSeconds)
}

func (w *World) responseStatusOneOf(list string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	for _, s := range strings.Split(list, ",") {
		code, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return err
		}
		if resp.StatusCode == code {
			return nil
		}
	}
	return fmt.Errorf("status %d is not one of %s: %s", resp.StatusCode, list, resp)
}

func (w *World) twoJobsDifferentIDs() error {
	if len(w.jobs) < 2 {
		return fmt.Errorf("%d jobs started, need 2", len(w.jobs))
	}
	a, b := w.jobs[len(w.jobs)-2], w.jobs[len(w.jobs)-1]
	if a == b {
		return fmt.Errorf("both jobs have id %s", a)
	}
	return nil
}

func (w *World) jobStatusIsNot(unwanted string) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if st.Status == unwanted {
		return fmt.Errorf("job status is %q (error: %s)", st.Status, deref(st.Error))
	}
	return nil
}

func (w *World) epochExactly(n int) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if epochOf(st) != n {
		return fmt.Errorf("epoch %d, want exactly %d", epochOf(st), n)
	}
	return nil
}

func (w *World) epochExceedsWithin(n, seconds int) error {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for {
		st, err := w.fetchStatus()
		if err != nil {
			return err
		}
		if epochOf(st) > n {
			return nil
		}
		if st.Status != api.StatusRunning {
			return fmt.Errorf("job is %s at epoch %d (limit %d)", st.Status, epochOf(st), n)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("epoch %d did not exceed %d within %ds", epochOf(st), n, seconds)
		}
		time.Sleep(pollInterval)
	}
}

func (w *World) rememberEpoch() error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	w.rememberedEpoch = epochOf(st)
	return nil
}

func (w *World) configureMaxEpochsToRemembered() error {
	if err := w.requireJob(); err != nil {
		return err
	}
	w.record(w.client.Configure(w.jobID, &api.Parameters{MaxEpochsCount: api.Int(w.rememberedEpoch)}))
	return nil
}

func (w *World) epochAtMostRememberedPlus(k int) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if epochOf(st) > w.rememberedEpoch+k {
		return fmt.Errorf("epoch %d exceeds remembered %d + %d", epochOf(st), w.rememberedEpoch, k)
	}
	return nil
}

func (w *World) pauseResumeTimes(n int) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	w.loopFailures = nil
	for i := 0; i < n; i++ {
		for _, call := range []struct {
			name string
			fn   func(string) (*httpclient.Response, error)
		}{{"pause", w.client.Pause}, {"resume", w.client.Resume}} {
			resp, err := call.fn(w.jobID)
			switch {
			case err != nil:
				w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d %s: %v", i, call.name, err))
			case resp.StatusCode != http.StatusOK:
				// The job may legitimately have completed; anything else is a failure.
				st, serr := w.fetchStatus()
				if serr != nil || st.Status != api.StatusCompleted {
					w.loopFailures = append(w.loopFailures, fmt.Sprintf("#%d %s: %s", i, call.name, resp))
				}
			}
		}
	}
	return nil
}

func (w *World) everyLoopCallSucceeded() error {
	if len(w.loopFailures) > 0 {
		shown := w.loopFailures
		if len(shown) > 10 {
			shown = shown[:10]
		}
		return fmt.Errorf("%d calls failed:\n  %s", len(w.loopFailures), strings.Join(shown, "\n  "))
	}
	return nil
}

// rejectedOrCompletes accepts either a 4xx answer to the start request or a
// job that completes (not "error") within the time limit.
func (w *World) rejectedOrCompletes(seconds int) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return nil
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("start answered %s", resp)
	}
	if err := w.waitForCompletion(seconds); err != nil {
		return err
	}
	return w.jobStatusIs(api.StatusCompleted)
}

func (w *World) fetchResultNoCheck() error {
	if err := w.requireJob(); err != nil {
		return err
	}
	w.record(w.client.Result(w.jobID))
	return nil
}

func (w *World) fetchAndValidateTimes(n int) error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		res, err := w.fetchResult()
		if err != nil {
			return fmt.Errorf("fetch %d: %w", i, err)
		}
		if len(res.Solutions) == 0 {
			continue // before the first epoch
		}
		if v := w.inst.ValidateResult(res); len(v) > 0 {
			return fmt.Errorf("fetch %d: invalid result:\n  %s", i, strings.Join(v, "\n  "))
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

func (w *World) rememberBestFitness() error {
	res, err := w.fetchResult()
	if err != nil {
		return err
	}
	if len(res.Solutions) == 0 {
		return fmt.Errorf("no solutions yet")
	}
	w.rememberedFitness = res.Solutions[0].Fitness
	return nil
}

func (w *World) bestFitnessNotWorse() error {
	res, err := w.fetchResult()
	if err != nil {
		return err
	}
	if len(res.Solutions) == 0 {
		return fmt.Errorf("no solutions")
	}
	if got := res.Solutions[0].Fitness; got > w.rememberedFitness*(1+1e-12) {
		return fmt.Errorf("best fitness got worse: %g -> %g", w.rememberedFitness, got)
	}
	return nil
}

func (w *World) onlyRequestLabels() error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	res, err := w.currentResult()
	if err != nil {
		return err
	}
	for i, sol := range res.Solutions {
		for _, sh := range sol.Sheets {
			for _, tok := range strings.Fields(sh.Tree) {
				if tok == "S" || tok == "L" {
					continue
				}
				if _, ok := w.inst.Parts[tok]; !ok {
					return fmt.Errorf("solution %d uses unknown label %q", i, tok)
				}
			}
		}
	}
	return nil
}

func (w *World) sheetNumbersContiguous() error {
	res, err := w.currentResult()
	if err != nil {
		return err
	}
	for i, sol := range res.Solutions {
		for k, sh := range sol.Sheets {
			if sh.Number != k {
				return fmt.Errorf("solution %d: sheet at position %d has number %d", i, k, sh.Number)
			}
		}
	}
	return nil
}

func (w *World) everySheetSinglePart() error {
	b, err := w.best()
	if err != nil {
		return err
	}
	for _, sh := range b.Sheets {
		if len(strings.Fields(sh.Tree)) != 1 {
			return fmt.Errorf("sheet %d holds more than one part: %q", sh.Number, sh.Tree)
		}
	}
	return nil
}

func (w *World) resultEmptyOrValid() error {
	res, err := w.fetchResult()
	if err != nil {
		return err
	}
	if len(res.Solutions) == 0 {
		return nil
	}
	if err := w.requireInstance(); err != nil {
		return err
	}
	if v := w.inst.ValidateResult(res); len(v) > 0 {
		return fmt.Errorf("invalid result:\n  %s", strings.Join(v, "\n  "))
	}
	return nil
}

// bestVectorConsistent: STAT-05 — best sheets >= lower bound and
// bestFitness = sheets*sheetArea + area.
func (w *World) bestVectorConsistent() error {
	if err := w.requireInstance(); err != nil {
		return err
	}
	if w.inst.Sheet == nil {
		return fmt.Errorf("the fixture has no sheet")
	}
	if err := w.requireHistory(); err != nil {
		return err
	}
	for i, st := range w.history {
		if st.Problem == nil || len(st.Problem.BestFitnessVector) < 2 || st.Statistics == nil || st.Statistics.State == nil {
			continue
		}
		v := st.Problem.BestFitnessVector
		if int(v[0]) < w.inst.LowerBound() {
			return fmt.Errorf("poll %d: best uses %g sheets, below the lower bound %d", i, v[0], w.inst.LowerBound())
		}
		// The status snapshot is not atomic (the vector comes from the elites of
		// the last epoch, bestFitness from the epoch state); only compare polls
		// taken while the job is not running.
		if st.Status == api.StatusRunning {
			continue
		}
		want := v[0]*w.inst.Sheet.Area() + v[1]
		if got := st.Statistics.State.EpochState.BestFitness; math.Abs(got-want) > 1e-9*math.Max(1, want) {
			return fmt.Errorf("poll %d: bestFitness %g, vector gives %g", i, got, want)
		}
	}
	return nil
}

func vecLess(a, b []float64) bool {
	for i := range a {
		if i >= len(b) {
			return false
		}
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func (w *World) selectedSpeciesSorted(n int) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if st.Problem == nil {
		return fmt.Errorf("status has no problem insights")
	}
	sel := st.Problem.SelectedSpecies
	if len(sel) != n {
		return fmt.Errorf("%d selected species, want %d", len(sel), n)
	}
	for i := 1; i < len(sel); i++ {
		if vecLess(sel[i], sel[i-1]) {
			return fmt.Errorf("selected species not sorted at %d: %v before %v", i, sel[i-1], sel[i])
		}
	}
	return nil
}

func (w *World) selectedSpeciesBecomes(n, seconds int) error {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	for {
		st, err := w.fetchStatus()
		if err != nil {
			return err
		}
		if st.Problem != nil && len(st.Problem.SelectedSpecies) == n {
			return nil
		}
		if time.Now().After(deadline) {
			got := -1
			if st.Problem != nil {
				got = len(st.Problem.SelectedSpecies)
			}
			return fmt.Errorf("selected species count is %d after %ds, want %d", got, seconds, n)
		}
		time.Sleep(pollInterval)
	}
}

func (w *World) saturationWithinUnit() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	for i, st := range w.history {
		if st.Statistics == nil || st.Statistics.State == nil {
			continue
		}
		if r := st.Statistics.State.WindowState.SaturationRate; r < 0 || r > 1 {
			return fmt.Errorf("poll %d: saturationRate %g outside [0, 1]", i, r)
		}
	}
	return nil
}

// windowAveragesNotBelowBest: STAT-08. Every per-epoch average is >= that
// epoch's best, which is >= the current best (elitism), so window averages
// can never be below the current best fitness.
func (w *World) windowAveragesNotBelowBest() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	for i, st := range w.history {
		if epochOf(&st) == 0 {
			continue
		}
		s := st.Statistics.State
		tol := 1e-9 * math.Max(1, math.Abs(s.EpochState.BestFitness))
		if s.WindowState.AverageFitness < s.EpochState.BestFitness-tol {
			return fmt.Errorf("poll %d: window averageFitness %g below bestFitness %g", i, s.WindowState.AverageFitness, s.EpochState.BestFitness)
		}
		if s.WindowState.AverageEliteFitness < s.EpochState.BestFitness-tol {
			return fmt.Errorf("poll %d: window averageEliteFitness %g below bestFitness %g", i, s.WindowState.AverageEliteFitness, s.EpochState.BestFitness)
		}
	}
	return nil
}

func (w *World) waitSeconds(n int) error {
	time.Sleep(time.Duration(n) * time.Second)
	return nil
}

// Cycles and championship (ga.go): the published state's cycle counters.

func (w *World) publishedState() (*api.State, error) {
	st, err := w.fetchStatus()
	if err != nil {
		return nil, err
	}
	if st.Statistics == nil || st.Statistics.State == nil {
		return nil, fmt.Errorf("no published state")
	}
	return st.Statistics.State, nil
}

func (w *World) finishedCycles(n int) error {
	s, err := w.publishedState()
	if err != nil {
		return err
	}
	if s.Cycles != n {
		return fmt.Errorf("%d finished cycles, want %d", s.Cycles, n)
	}
	return nil
}

func (w *World) championshipStarted() error {
	s, err := w.publishedState()
	if err != nil {
		return err
	}
	if s.ChampionshipSince <= 0 {
		return fmt.Errorf("championship not started (championshipSince %d)", s.ChampionshipSince)
	}
	return nil
}

func (w *World) championshipNotStarted() error {
	s, err := w.publishedState()
	if err != nil {
		return err
	}
	if s.ChampionshipSince != 0 {
		return fmt.Errorf("championship started at epoch %d", s.ChampionshipSince)
	}
	return nil
}
