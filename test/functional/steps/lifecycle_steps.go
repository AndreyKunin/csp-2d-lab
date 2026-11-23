package steps

import (
	"csp2d/test/support/api"
	"csp2d/test/support/httpclient"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

const pollInterval = 100 * time.Millisecond

func registerLifecycleSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^I (pause|resume|delete) the simulation$`, w.controlSimulation)
	ctx.Step(`^I fetch the status$`, w.fetchStatusStep)
	ctx.Step(`^I call "(pause|resume|status|result|delete|configure)" for an unknown simulation$`, w.callUnknown)
	ctx.Step(`^I configure the simulation with:$`, w.configureWithTable)
	ctx.Step(`^I send the raw action body '([^']*)'$`, w.sendRawActionBody)
	ctx.Step(`^I fetch the simulation with format "([^"]*)"$`, w.fetchWithFormat)
	ctx.Step(`^I wait for job completion within (\d+) seconds$`, w.waitForCompletion)

	ctx.Step(`^the job status is "([^"]*)"$`, w.jobStatusIs)
	ctx.Step(`^the response job status is "([^"]*)"$`, w.responseJobStatusIs)
	ctx.Step(`^the response message is "([^"]*)"$`, w.responseMessageIs)
	ctx.Step(`^the response message contains "([^"]*)"$`, w.responseMessageContains)
	ctx.Step(`^the job id is a 32-character hex string$`, w.jobIDIsHex)
	ctx.Step(`^the response has RFC 3339 timestamps$`, w.responseHasTimestamps)
	ctx.Step(`^the epoch number does not change for (\d+(?:\.\d+)?) seconds?$`, w.epochDoesNotChange)
	ctx.Step(`^the epoch number increases within (\d+(?:\.\d+)?) seconds?$`, w.epochIncreasesWithin)
	ctx.Step(`^the epoch number is at most (\d+)$`, w.epochAtMost)
	ctx.Step(`^the reported parameter "([^"]*)" is (\S+)$`, w.reportedParameterIs)
	ctx.Step(`^the simulation no longer exists$`, w.simulationNoLongerExists)
}

func (w *World) controlSimulation(action string) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	switch action {
	case "pause":
		w.record(w.client.Pause(w.jobID))
	case "resume":
		w.record(w.client.Resume(w.jobID))
	case "delete":
		w.record(w.client.Delete(w.jobID))
	}
	return nil
}

func (w *World) callUnknown(action string) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	switch action {
	case "pause":
		w.record(w.client.Pause(unknownID))
	case "resume":
		w.record(w.client.Resume(unknownID))
	case "status":
		w.record(w.client.Status(unknownID))
	case "result":
		w.record(w.client.Result(unknownID))
	case "delete":
		w.record(w.client.Delete(unknownID))
	case "configure":
		w.record(w.client.Configure(unknownID, &api.Parameters{MutationRate: api.Float(0.05)}))
	}
	return nil
}

func (w *World) configureWithTable(table *godog.Table) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	params := &api.Parameters{}
	for _, row := range table.Rows {
		if len(row.Cells) != 2 {
			return fmt.Errorf("parameter table rows must have 2 cells (key | value)")
		}
		if err := setParameter(params, row.Cells[0].Value, row.Cells[1].Value); err != nil {
			return err
		}
	}
	w.record(w.client.Configure(w.jobID, params))
	return nil
}

// fetchWithFormat gets the simulation (GET /api/v1/simulation/<id>) with the
// format query parameter; an empty format sends none.
func (w *World) fetchWithFormat(format string) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	path := httpclient.SimulationPath + "/" + w.jobID
	if format != "" {
		path += "?format=" + format
	}
	w.record(w.client.Do(http.MethodGet, path, nil, ""))
	return nil
}

// sendRawActionBody posts body verbatim to the simulation (POST
// /api/v1/simulation/<id>), for malformed or ill-typed actions.
func (w *World) sendRawActionBody(body string) error {
	if err := w.requireJob(); err != nil {
		return err
	}
	w.record(w.client.ActionRaw(w.jobID, []byte(body)))
	return nil
}

// fetchStatus polls the status once (must succeed) and records it in history.
func (w *World) fetchStatus() (*api.JobStatus, error) {
	if err := w.requireJob(); err != nil {
		return nil, err
	}
	resp, err := w.client.Status(w.jobID)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request failed: %s", resp)
	}
	var st api.JobStatus
	if err := resp.Decode(&st); err != nil {
		return nil, err
	}
	w.history = append(w.history, st)
	return &st, nil
}

func (w *World) fetchStatusStep() error {
	if err := w.requireJob(); err != nil {
		return err
	}
	resp, err := w.client.Status(w.jobID)
	w.record(resp, err)
	if err == nil && resp.StatusCode == http.StatusOK {
		var st api.JobStatus
		if err := resp.Decode(&st); err != nil {
			return err
		}
		w.history = append(w.history, st)
	}
	return nil
}

func epochOf(st *api.JobStatus) int {
	if st.Statistics == nil || st.Statistics.State == nil {
		return 0
	}
	return st.Statistics.State.EpochState.EpochNumber
}

func (w *World) waitForCompletion(seconds int) error {
	deadline := time.Now().Add(time.Duration(seconds) * time.Second)
	var last *api.JobStatus
	for time.Now().Before(deadline) {
		st, err := w.fetchStatus()
		if err != nil {
			return err
		}
		last = st
		if st.Status == api.StatusCompleted || st.Status == api.StatusError {
			return nil
		}
		time.Sleep(pollInterval)
	}
	if last != nil {
		return fmt.Errorf("job not finished within %ds: status %s at epoch %d", seconds, last.Status, epochOf(last))
	}
	return fmt.Errorf("job not finished within %ds", seconds)
}

func (w *World) jobStatusIs(want string) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if st.Status != want {
		msg := ""
		if st.Error != nil {
			msg = " (error: " + *st.Error + ")"
		}
		return fmt.Errorf("job status is %q, want %q%s", st.Status, want, msg)
	}
	return nil
}

func (w *World) lastJobStatus() (*api.JobStatus, error) {
	resp, err := w.last()
	if err != nil {
		return nil, err
	}
	var st api.JobStatus
	if err := resp.Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (w *World) responseJobStatusIs(want string) error {
	st, err := w.lastJobStatus()
	if err != nil {
		return err
	}
	if st.Status != want {
		return fmt.Errorf("response status field is %q, want %q", st.Status, want)
	}
	return nil
}

func (w *World) responseMessageIs(want string) error {
	st, err := w.lastJobStatus()
	if err != nil {
		return err
	}
	if st.Message != want {
		return fmt.Errorf("response message is %q, want %q", st.Message, want)
	}
	return nil
}

// responseMessageContains checks the "message" field of the last response
// (a job status or an error body).
func (w *World) responseMessageContains(want string) error {
	resp, err := w.last()
	if err != nil {
		return err
	}
	var body api.ErrorBody
	if err := resp.Decode(&body); err != nil {
		return err
	}
	if !strings.Contains(body.Message, want) {
		return fmt.Errorf("response message %q does not contain %q", body.Message, want)
	}
	return nil
}

var hexID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (w *World) jobIDIsHex() error {
	st, err := w.lastJobStatus()
	if err != nil {
		return err
	}
	if !hexID.MatchString(st.ID) {
		return fmt.Errorf("job id %q is not 32 lowercase hex characters", st.ID)
	}
	return nil
}

func (w *World) responseHasTimestamps() error {
	st, err := w.lastJobStatus()
	if err != nil {
		return err
	}
	for name, v := range map[string]string{"created_at": st.CreatedAt, "updated_at": st.UpdatedAt} {
		if _, err := time.Parse(time.RFC3339, v); err != nil {
			return fmt.Errorf("%s %q is not RFC 3339: %w", name, v, err)
		}
	}
	return nil
}

func (w *World) epochDoesNotChange(seconds float64) error {
	first, err := w.fetchStatus()
	if err != nil {
		return err
	}
	step := time.Duration(seconds * float64(time.Second) / 3)
	for i := 0; i < 3; i++ {
		time.Sleep(step)
		st, err := w.fetchStatus()
		if err != nil {
			return err
		}
		if epochOf(st) != epochOf(first) {
			return fmt.Errorf("epoch changed from %d to %d while it should be stable", epochOf(first), epochOf(st))
		}
	}
	return nil
}

func (w *World) epochIncreasesWithin(seconds float64) error {
	first, err := w.fetchStatus()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(time.Duration(seconds * float64(time.Second)))
	last := first
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		st, err := w.fetchStatus()
		if err != nil {
			return err
		}
		if epochOf(st) > epochOf(first) {
			return nil
		}
		last = st
	}
	return fmt.Errorf("epoch stayed at %d for %gs (status %s %s)", epochOf(first), seconds, last.Status, deref(last.Error))
}

func (w *World) epochAtMost(max int) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if epochOf(st) > max {
		return fmt.Errorf("epoch %d exceeds %d", epochOf(st), max)
	}
	return nil
}

func (w *World) reportedParameterIs(key, value string) error {
	st, err := w.fetchStatus()
	if err != nil {
		return err
	}
	if st.Statistics == nil || st.Statistics.Parameters == nil {
		return fmt.Errorf("status has no statistics.parameters")
	}
	p := st.Statistics.Parameters
	want, err := parseNumber(value)
	if err != nil {
		return err
	}
	var got float64
	switch key {
	case "populationSize":
		got = float64(p.PopulationSize)
	case "eliteCount":
		got = float64(p.EliteCount)
	case "mutationRate":
		got = p.MutationRate
	case "crossoverRate":
		got = p.CrossoverRate
	case "cycleCount":
		got = float64(p.CycleCount)
	case "tournamentSize":
		got = float64(p.TournamentSize)
	case "repairWeight":
		got = float64(p.RepairWeight)
	case "flipWeight":
		got = float64(p.FlipWeight)
	case "donorFraction":
		got = p.DonorFraction
	case "maxEpochsCount":
		got = float64(p.MaxEpochsCount)
	case "windowSize":
		got = float64(p.WindowSize)
	default:
		return fmt.Errorf("unknown parameter %q", key)
	}
	if got != want {
		return fmt.Errorf("reported %s = %g, want %g", key, got, want)
	}
	return nil
}

func (w *World) simulationNoLongerExists() error {
	if err := w.requireJob(); err != nil {
		return err
	}
	resp, err := w.client.Status(w.jobID)
	if err != nil {
		return err
	}
	if resp.OK() {
		return fmt.Errorf("simulation %s still exists: %s", w.jobID, resp)
	}
	if msg := resp.ErrorMessage(); !containsFold(msg, "not found") {
		return fmt.Errorf("expected a 'not found' error, got %s", resp)
	}
	return nil
}
