package steps

import (
	"csp2d/test/support/api"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
)

func registerStatisticsSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^I poll the status for (\d+(?:\.\d+)?) seconds?$`, w.pollFor)
	ctx.Step(`^all statistics are finite$`, w.allStatisticsFinite)
	ctx.Step(`^the statistics are internally consistent$`, w.statisticsConsistent)
	ctx.Step(`^the epoch numbers never decrease$`, w.epochsNeverDecrease)
	ctx.Step(`^the best fitness never increases$`, w.bestFitnessNeverIncreases)
	ctx.Step(`^the status payload has the full statistics schema$`, w.fullSchema)
	ctx.Step(`^(\d+) clients poll the status at (\d+) Hz for (\d+(?:\.\d+)?) seconds?$`, w.concurrentPolling)
	ctx.Step(`^every poll succeeded$`, w.everyPollSucceeded)
}

func (w *World) pollFor(seconds float64) error {
	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	for time.Now().Before(deadline) {
		if _, err := w.fetchStatus(); err != nil {
			return err
		}
		time.Sleep(pollInterval)
	}
	return nil
}

func (w *World) requireHistory() error {
	if len(w.history) == 0 {
		if _, err := w.fetchStatus(); err != nil {
			return err
		}
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (w *World) allStatisticsFinite() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	for i, st := range w.history {
		if st.Statistics == nil || st.Statistics.State == nil {
			continue
		}
		s := st.Statistics.State
		values := map[string]float64{
			"epoch.bestFitness": s.EpochState.BestFitness, "epoch.averageFitness": s.EpochState.AverageFitness,
			"epoch.fitnessDeviation": s.EpochState.FitnessDeviation, "epoch.averageEliteFitness": s.EpochState.AverageEliteFitness,
			"epoch.eliteFitnessDeviation": s.EpochState.EliteFitnessDeviation, "window.averageFitness": s.WindowState.AverageFitness,
			"window.averageFitnessDeviation": s.WindowState.AverageFitnessDeviation, "window.averageEliteFitness": s.WindowState.AverageEliteFitness,
			"window.eliteFitnessDeviation": s.WindowState.EliteFitnessDeviation, "window.saturationRate": s.WindowState.SaturationRate,
		}
		for _, k := range sortedKeys(values) {
			if !finite(values[k]) {
				return fmt.Errorf("poll %d: %s is not finite (%v)", i, k, values[k])
			}
		}
	}
	return nil
}

func (w *World) statisticsConsistent() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	const eps = 1e-9
	for i, st := range w.history {
		if st.Statistics == nil || st.Statistics.State == nil || st.Statistics.State.EpochState.EpochNumber == 0 {
			continue
		}
		e := st.Statistics.State.EpochState
		tol := eps * math.Max(1, math.Abs(e.AverageFitness))
		if e.BestFitness > e.AverageEliteFitness+tol {
			return fmt.Errorf("poll %d: bestFitness %g > averageEliteFitness %g", i, e.BestFitness, e.AverageEliteFitness)
		}
		if e.AverageEliteFitness > e.AverageFitness+tol {
			return fmt.Errorf("poll %d: averageEliteFitness %g > averageFitness %g", i, e.AverageEliteFitness, e.AverageFitness)
		}
		if e.FitnessDeviation < 0 || e.EliteFitnessDeviation < 0 {
			return fmt.Errorf("poll %d: negative deviation", i)
		}
		if p := st.Statistics.Parameters; p != nil && e.EpochNumber > p.MaxEpochsCount && p.MaxEpochsCount > 0 {
			return fmt.Errorf("poll %d: epoch %d exceeds maxEpochsCount %d", i, e.EpochNumber, p.MaxEpochsCount)
		}
	}
	return nil
}

func (w *World) epochsNeverDecrease() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	prev := -1
	for i, st := range w.history {
		e := epochOf(&st)
		if e < prev {
			return fmt.Errorf("epoch decreased from %d to %d at poll %d", prev, e, i)
		}
		prev = e
	}
	return nil
}

func (w *World) bestFitnessNeverIncreases() error {
	if err := w.requireHistory(); err != nil {
		return err
	}
	prev := math.Inf(1)
	for i, st := range w.history {
		if epochOf(&st) == 0 {
			continue
		}
		b := st.Statistics.State.EpochState.BestFitness
		if b > prev*(1+1e-12) {
			return fmt.Errorf("best fitness increased from %g to %g at poll %d", prev, b, i)
		}
		prev = b
	}
	return nil
}

func (w *World) fullSchema() error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	var missing []string
	if st.Statistics == nil {
		missing = append(missing, "statistics")
	} else {
		if st.Statistics.Parameters == nil {
			missing = append(missing, "statistics.parameters")
		}
		if st.Statistics.State == nil {
			missing = append(missing, "statistics.state")
		}
	}
	if st.Problem == nil {
		missing = append(missing, "problem")
	} else {
		if len(st.Problem.BestFitnessVector) != 2 {
			missing = append(missing, fmt.Sprintf("problem.bestFitnessVector (len %d, want 2)", len(st.Problem.BestFitnessVector)))
		}
		if len(st.Problem.SelectedSpecies) == 0 {
			missing = append(missing, "problem.selectedSpecies")
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("status payload lacks: %s", strings.Join(missing, ", "))
	}
	return nil
}

// concurrentPolling runs n clients, each polling the current job's status at
// hz requests per second; failures are collected for "every poll succeeded".
func (w *World) concurrentPolling(n, hz int, seconds float64) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	interval := time.Second / time.Duration(hz)
	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	var mu sync.Mutex
	var wg sync.WaitGroup
	w.pollFailures = nil
	for c := 0; c < n; c++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			for time.Now().Before(deadline) {
				resp, err := w.client.Status(w.jobID)
				var failure string
				switch {
				case err != nil:
					failure = err.Error()
				case resp.StatusCode != http.StatusOK:
					failure = resp.String()
				default:
					var st api.JobStatus
					if err := resp.Decode(&st); err != nil {
						failure = err.Error()
					}
				}
				if failure != "" {
					mu.Lock()
					w.pollFailures = append(w.pollFailures, fmt.Sprintf("client %d: %s", c, failure))
					mu.Unlock()
				}
				time.Sleep(interval)
			}
		}(c)
	}
	wg.Wait()
	return nil
}

func (w *World) everyPollSucceeded() error {
	if len(w.pollFailures) > 0 {
		shown := w.pollFailures
		if len(shown) > 10 {
			shown = shown[:10]
		}
		return fmt.Errorf("%d polls failed, first: \n  %s", len(w.pollFailures), strings.Join(shown, "\n  "))
	}
	return nil
}
