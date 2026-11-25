//go:build testhooks

package mvc_test

import (
	"context"
	"csp2d/ga"
	"csp2d/mvc"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// The Lab's Stop button (PlayerPanel.onStop, FE-04) as the server sees it.
// The UI pauses, then runs finishJob (GET the result, DELETE the job) twice
// concurrently: once from the status subscriber that finishMonitoring
// notifies, once explicitly from onStop. When Stop is pressed before the first
// status poll answers, finishMonitoring does not notify (the client has no job
// status yet), so the explicit call is the only one. These tests emulate each
// request sequence against a real job and a manager limited to one unfinished
// job, so a job that is not removed shows up as a refused start (429).

func newLimitedAPI(t *testing.T) *api {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cfg := productionConfig(t)
	app := mvc.WebApp{Configuration: *cfg,
		SimulationManager: ga.NewSimulationManagerWithLimits(ctx, ga.Limits{MaxActiveJobs: 1, IdleTimeout: -1})}
	e := echo.New()
	mvc.ConfigureRoutes(e, app)
	return &api{t, e}
}

// slotIsFree starts and deletes a job: it succeeds only when no other job is
// unfinished (MaxActiveJobs 1).
func (a *api) slotIsFree() bool {
	a.t.Helper()
	code, body := a.call(http.MethodPut, "/api/v1/simulation", runningJob)
	if code != http.StatusCreated {
		return false
	}
	a.expect(http.MethodDelete, "/api/v1/simulation/"+body["id"].(string), "", http.StatusOK)
	return true
}

// waitEpochs waits until the job has published more than n epochs.
func (a *api) waitEpochs(path string, n float64) {
	a.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		_, st := a.call(http.MethodGet, path, "")
		if stats, ok := st["statistics"].(map[string]any); ok {
			if state, ok := stats["state"].(map[string]any); ok {
				if epoch, ok := state["epochState"].(map[string]any); ok && epoch["epochNumber"].(float64) > n {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			a.t.Fatalf("%s: no epoch beyond %v", path, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// UT-WEB-04: the UI's current Stop (pause, then two concurrent finishJob)
// is harmless to the server: the job is removed once, the second DELETE and
// any result requested after the first DELETE answer 404, nothing panics, and
// the job's slot is free afterwards. Repeated to shake out interleavings.
func TestStopTwiceConcurrently(t *testing.T) {
	a := newLimitedAPI(t)
	for round := 0; round < 25; round++ {
		id := a.start(runningJob)
		path := "/api/v1/simulation/" + id
		a.expect(http.MethodPost, path, `{"action":"pause"}`, http.StatusOK)

		var wg sync.WaitGroup
		codes := make([][2]int, 2) // [result, delete] of each finishJob
		for i := range codes {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				codes[i][0], _ = a.call(http.MethodGet, path+"?format=result", "")
				codes[i][1], _ = a.call(http.MethodDelete, path, "")
			}(i)
		}
		wg.Wait()

		deleted := 0
		for i, c := range codes {
			if c[0] != http.StatusOK && c[0] != http.StatusNotFound {
				t.Fatalf("round %d, finishJob %d: result %d, want 200 or 404", round, i, c[0])
			}
			switch c[1] {
			case http.StatusOK:
				deleted++
			case http.StatusNotFound:
			default:
				t.Fatalf("round %d, finishJob %d: delete %d, want 200 or 404", round, i, c[1])
			}
		}
		if deleted != 1 {
			t.Fatalf("round %d: %d deletes succeeded, want exactly 1 (codes %v)", round, deleted, codes)
		}
		if codes[0][0] != http.StatusOK && codes[1][0] != http.StatusOK {
			t.Fatalf("round %d: no finishJob got the result (codes %v)", round, codes)
		}
		a.expect(http.MethodGet, path, "", http.StatusNotFound)
		if !a.slotIsFree() {
			t.Fatalf("round %d: the stopped job still holds its slot", round)
		}
	}
}

// UT-WEB-05: the fixed Stop (pause, one finishJob) gets the result and removes
// the job; the slot is free afterwards. (A job stopped before its first epoch
// has no result yet: the result is empty.)
func TestStopOnce(t *testing.T) {
	a := newLimitedAPI(t)
	id := a.start(runningJob)
	path := "/api/v1/simulation/" + id
	a.waitEpochs(path, 2)
	a.expect(http.MethodPost, path, `{"action":"pause"}`, http.StatusOK)
	if r := a.expect(http.MethodGet, path+"?format=result", "", http.StatusOK); r["solutions"] == nil {
		t.Errorf("no solutions in the result: %v", r)
	}
	a.expect(http.MethodDelete, path, "", http.StatusOK)
	if !a.slotIsFree() {
		t.Fatal("the stopped job still holds its slot")
	}
}

// UT-WEB-06: a Stop that only pauses (what the UI would send if onStop relied
// on the status subscriber alone and Stop were pressed before the first status
// poll) leaves the job registered and paused: it keeps its slot and its memory
// until the idle sweep, and further starts are refused. This is why the fix of
// FE-04 must keep the explicit finishJob and only guard against running it
// twice.
func TestStopWithoutFinishLeavesTheJob(t *testing.T) {
	a := newLimitedAPI(t)
	id := a.start(runningJob)
	path := "/api/v1/simulation/" + id
	a.expect(http.MethodPost, path, `{"action":"pause"}`, http.StatusOK)

	if st := a.expect(http.MethodGet, path, "", http.StatusOK); st["status"] != "paused" {
		t.Fatalf("status %v, want paused", st["status"])
	}
	if a.slotIsFree() {
		t.Fatal("a new job started while the paused one holds the only slot")
	}
	a.expect(http.MethodPut, "/api/v1/simulation", runningJob, http.StatusTooManyRequests)
	a.expect(http.MethodDelete, path, "", http.StatusOK)
	if !a.slotIsFree() {
		t.Fatal("the slot is not free after the delete")
	}
}
