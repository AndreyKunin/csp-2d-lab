//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"csp2d/test/support/fakes"
	"csp2d/test/support/testutil"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func receive(t *testing.T, ch chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("no %s within 5s", what)
	}
}

// UT-GA-SIM-08: pausing through the job channels stops all problem calls
// until the job is resumed.
func TestSimulatePauseResumeChannels(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, EpochDelay: time.Millisecond}
	job := ga.NewSimulationJob("test", ga.NewSimulation(params(10, 2, 300, 1000)), fake)
	done := make(chan error, 1)
	go func() { done <- ga.Simulate(job) }()

	time.Sleep(20 * time.Millisecond)
	job.PauseChan <- struct{}{}
	receive(t, job.PauseAckChan, "pause acknowledgement")
	if s := job.GetStatus().Status; s != ga.StatusPaused {
		t.Fatalf("status %s after pause", s)
	}
	n := len(fake.Calls())
	time.Sleep(50 * time.Millisecond)
	if m := len(fake.Calls()); m != n {
		t.Fatalf("%d problem calls while paused", m-n)
	}

	job.ResumeChan <- struct{}{}
	receive(t, job.ResumeAckChan, "resume acknowledgement")
	if s := job.GetStatus().Status; s != ga.StatusRunning {
		t.Fatalf("status %s after resume", s)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("simulation did not finish after resume")
	}
	if len(fake.Calls()) <= n {
		t.Fatal("no progress after resume")
	}
}

// UT-GA-MGR-05: deleting a running or paused job ends its goroutine.
func TestManagerDeleteStopsJob(t *testing.T) {
	m := ga.NewSimulationManager()
	baseline := runtime.NumGoroutine()
	for _, pauseFirst := range []bool{false, true} {
		st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 100000000, 100000000)),
			&fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		if pauseFirst {
			if err := m.PauseSimulation(st.ID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := m.DeleteSimulation(st.ID); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines remain after deleting the jobs (baseline %d)", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// UT-GA-MGR-08: many clients controlling several jobs at once: no deadlock,
// every call returns. Run with -race (make test-unit-race) to also catch data
// races. (A narrow window exists where a second concurrent resume waits for a
// later pause; it did not reproduce in 30 consecutive runs.)
func TestManagerConcurrentControl(t *testing.T) {
	if testing.Short() {
		t.Skip("takes a few seconds")
	}
	m := ga.NewSimulationManager()
	var ids []string
	for i := 0; i < 5; i++ {
		st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 100000000, 100000000)),
			&fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, st.ID)
	}
	testutil.Within(t, 30*time.Second, "concurrent control calls", func() {
		var wg sync.WaitGroup
		for c := 0; c < 50; c++ {
			wg.Add(1)
			go func(c int) {
				defer wg.Done()
				rnd := rand.New(rand.NewSource(int64(c)))
				for k := 0; k < 40; k++ {
					id := ids[rnd.Intn(len(ids))]
					switch rnd.Intn(5) {
					case 0:
						_ = m.PauseSimulation(id)
					case 1:
						_ = m.ResumeSimulation(id)
					case 2:
						_, _ = m.GetSimulationStatus(id)
					case 3:
						_, _ = m.GetSimulationStatistics(id)
					case 4:
						p := params(6, 2, 100000000, 100000000)
						p.MutationRate = float64(k%10) / 100
						_ = m.ConfigureSimulation(id, p)
					}
				}
			}(c)
		}
		wg.Wait()
	})
	for _, id := range ids {
		st, err := m.GetSimulationStatus(id)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status != ga.StatusRunning && st.Status != ga.StatusPaused {
			t.Errorf("job %s ended in status %s", id, fmt.Sprint(st.Status))
		}
	}
}
