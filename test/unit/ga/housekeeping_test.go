//go:build testhooks

package ga_test

import (
	"context"
	"csp2d/ga"
	"csp2d/test/support/fakes"
	"errors"
	"runtime"
	"testing"
	"time"
)

func startLong(t *testing.T, m ga.SimulationManager) (ga.SimulationJobStatus, error) {
	t.Helper()
	return m.StartSimulation(ga.NewSimulation(params(6, 2, 100000000, 1000)),
		&fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond})
}

// UT-GA-MGR-10: StartSimulation refuses with ErrTooManyJobs once MaxActiveJobs
// unfinished jobs exist; deleting one, or a job finishing, frees a slot.
func TestManagerLimitsActiveJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := ga.NewSimulationManagerWithLimits(ctx, ga.Limits{MaxActiveJobs: 2, IdleTimeout: -1})

	a, err := startLong(t, m)
	if err != nil {
		t.Fatal(err)
	}
	b, err := startLong(t, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.PauseSimulation(b.ID); err != nil { // paused jobs count too
		t.Fatal(err)
	}
	if _, err := startLong(t, m); !errors.Is(err, ga.ErrTooManyJobs) {
		t.Fatalf("third start: got %v, want ErrTooManyJobs", err)
	}

	if _, err := m.DeleteSimulation(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := startLong(t, m); err != nil {
		t.Fatalf("start after a delete: %v", err)
	}

	// A completed job keeps its registry entry but not its slot.
	if _, err := m.DeleteSimulation(b.ID); err != nil {
		t.Fatal(err)
	}
	short, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 3, 10)), &fakes.Problem{InputsSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, short.ID, ga.StatusCompleted)
	if _, err := startLong(t, m); err != nil {
		t.Fatalf("start next to a completed job: %v", err)
	}
}

// UT-GA-MGR-11: Sweep removes and stops every job not looked up within
// IdleTimeout (running, paused or completed) and keeps recently polled ones.
func TestManagerSweepRemovesIdleJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const idle = time.Hour // the sweeper's own ticker never fires during the test
	m := ga.NewSimulationManagerWithLimits(ctx, ga.Limits{MaxActiveJobs: -1, IdleTimeout: idle})
	baseline := runtime.NumGoroutine()

	running, err := startLong(t, m)
	if err != nil {
		t.Fatal(err)
	}
	paused, err := startLong(t, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.PauseSimulation(paused.ID); err != nil {
		t.Fatal(err)
	}
	completed, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 3, 10)), &fakes.Problem{InputsSize: 3})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, completed.ID, ga.StatusCompleted)

	if n := m.Sweep(time.Now().Add(idle - time.Minute)); n != 0 {
		t.Fatalf("swept %d jobs before the idle timeout", n)
	}

	later := time.Now().Add(idle + time.Minute)
	if n := m.Sweep(later); n != 3 {
		t.Fatalf("swept %d jobs after the idle timeout, want 3", n)
	}
	for _, id := range []string{running.ID, paused.ID, completed.ID} {
		if _, err := m.GetSimulationStatus(id); err == nil {
			t.Errorf("job %s still registered after the sweep", id)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines remain after the sweep (baseline %d)", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// UT-GA-MGR-12: any API lookup of a job (status, statistics, control) resets
// its idle clock.
func TestManagerLookupKeepsJobAlive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const idle = time.Hour
	m := ga.NewSimulationManagerWithLimits(ctx, ga.Limits{MaxActiveJobs: -1, IdleTimeout: idle})
	st, err := startLong(t, m)
	if err != nil {
		t.Fatal(err)
	}
	defer m.DeleteSimulation(st.ID)

	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	if _, err := m.GetSimulationStatistics(st.ID); err != nil {
		t.Fatal(err)
	}
	// Idle for exactly the timeout measured from creation, but not from the lookup.
	if n := m.Sweep(start.Add(idle - time.Millisecond)); n != 0 {
		t.Fatalf("a job polled at %v was swept", start)
	}
	if n := m.Sweep(start.Add(idle + time.Second)); n != 1 {
		t.Fatalf("swept %d jobs, want 1", n)
	}
}
