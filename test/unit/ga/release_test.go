//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"csp2d/test/support/fakes"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// releasingProblem is a fake problem that counts Release calls.
type releasingProblem struct {
	*fakes.Problem
	released atomic.Int32
}

func (p *releasingProblem) Release() { p.released.Add(1) }

// UT-GA-MGR-15: once a job's goroutine ends (completed, failed, panicked or
// cancelled), the manager releases the problem's memory exactly once.
func TestManagerReleasesProblemWhenJobEnds(t *testing.T) {
	cases := map[string]struct {
		problem *fakes.Problem
		epochs  int
		end     func(m ga.SimulationManager, id string)
	}{
		"completed": {&fakes.Problem{InputsSize: 3}, 3, nil},
		"error":     {&fakes.Problem{InputsSize: 3, ErrorIn: map[string]error{"Crossover": errors.New("boom")}}, 1000, nil},
		"panic":     {&fakes.Problem{InputsSize: 3, PanicIn: map[string]bool{"Mutation": true}}, 1000, nil},
		"cancelled": {&fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond}, 100000000,
			func(m ga.SimulationManager, id string) { _, _ = m.DeleteSimulation(id) }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := &releasingProblem{Problem: c.problem}
			m := ga.NewSimulationManager()
			st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, c.epochs, 10)), p)
			if err != nil {
				t.Fatal(err)
			}
			if c.end != nil {
				time.Sleep(20 * time.Millisecond)
				c.end(m, st.ID)
			}
			deadline := time.Now().Add(5 * time.Second)
			for p.released.Load() == 0 {
				if time.Now().After(deadline) {
					t.Fatal("the problem was not released after the job ended")
				}
				time.Sleep(5 * time.Millisecond)
			}
			time.Sleep(20 * time.Millisecond)
			if n := p.released.Load(); n != 1 {
				t.Fatalf("released %d times, want 1", n)
			}
		})
	}
}
