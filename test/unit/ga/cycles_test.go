//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"csp2d/test/support/fakes"
	"csp2d/test/support/testutil"
	"errors"
	"math/rand"
	"testing"
	"time"
)

// Cycles and championship (ga.go). With a constant fitness the best never
// improves after epoch 1, so with window W a cycle ends W epochs after it
// starts: epoch 1+W ends cycle 1, the next cycle starts at the following epoch,
// and so on.

func constantFitness(string, *rand.Rand) float64 { return 100 }

func runCycles(t *testing.T, p *ga.Parameters, fake *fakes.Problem) (*ga.Simulation, error) {
	t.Helper()
	sim := ga.NewSimulation(p)
	job := ga.NewSimulationJob("test", sim, fake)
	var err error
	testutil.Within(t, 10*time.Second, "Simulate", func() { err = ga.Simulate(job) })
	return sim, err
}

func countCalls(fake *fakes.Problem, name string) int {
	n := 0
	for _, c := range fake.Calls() {
		if c == name {
			n++
		}
	}
	return n
}

// UT-GA-CY-01: cycles end on stagnation, the championship follows the last
// one, and the run ends when the championship stagnates.
func TestCyclesAndChampionship(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 5)
	p.CycleCount = 2
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	st := sim.State
	// Cycle 1: epochs 1..6 (ends at 6), cycle 2: 7..12, championship: 13..18.
	if st.Cycles != 2 || st.ChampionshipSince != 13 || st.CurrentEpoch.EpochNumber != 18 {
		t.Fatalf("cycles %d, championship since %d, last epoch %d; want 2, 13, 18",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber)
	}
	if st.StopReason != ga.StopChampionshipStagnated {
		t.Fatalf("stop reason %q, want %q", st.StopReason, ga.StopChampionshipStagnated)
	}
	if p.MaxEpochsCount != 1000 {
		t.Fatalf("maxEpochsCount changed to %d", p.MaxEpochsCount)
	}
	// No offspring on the two start-over epochs (6, 12) and on the last epoch
	// (the run ends before reproduction): 18 - 3 epochs with crossover.
	if n := countCalls(fake, "Crossover"); n != 15 {
		t.Fatalf("%d crossover calls, want 15", n)
	}
}

// UT-GA-CY-02: the championship starts from the saved elites. With a constant
// fitness and a stable sort, cycle 1's elites are initial individuals and
// stay at the top of the championship (elitism), while cycle 2's population
// is all random.
func TestChampionshipSeedsSavedElites(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 5)
	p.CycleCount = 2
	if _, err := runCycles(t, p, fake); err != nil {
		t.Fatal(err)
	}
	species := *fake.GetPopulation().Species
	for i := 0; i < 2; i++ {
		if o := species[i].(*fakes.Individual).Origin; o != "initial" {
			t.Fatalf("championship elite %d has origin %q, want cycle 1's initial elites", i, o)
		}
	}
}

// UT-GA-CY-03: a start-over replaces every slot, elite slots included, with
// random individuals: in cycle 2 no initial individual is left before the
// championship brings cycle 1's elites back.
func TestStartOverReplacesElites(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 11, 5) // stops in cycle 2 (epoch 11), before its end at 12
	p.CycleCount = 2
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	if sim.State.Cycles != 1 || sim.State.StopReason != "" {
		t.Fatalf("cycles %d, stop reason %q; want 1 and none (the epoch cap)", sim.State.Cycles, sim.State.StopReason)
	}
	for i, ind := range *fake.GetPopulation().Species {
		if ind.(*fakes.Individual).Origin == "initial" {
			t.Fatalf("slot %d still holds an initial individual after the start-over", i)
		}
	}
}

// UT-GA-CY-04: cycleCount 0: no cycles, no championship; the run ends at the
// first stagnation.
func TestCycleCountZeroStopsAtStagnation(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 5)
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	st := sim.State
	if st.Cycles != 0 || st.ChampionshipSince != 0 || st.CurrentEpoch.EpochNumber != 6 || st.StopReason != ga.StopStagnated {
		t.Fatalf("cycles %d, championship %d, last epoch %d, reason %q; want 0, 0, 6, %q",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber, st.StopReason, ga.StopStagnated)
	}
}

// UT-GA-CY-05: cycleCount 1: one cycle, then the championship.
func TestCycleCountOne(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 5)
	p.CycleCount = 1
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	st := sim.State
	if st.Cycles != 1 || st.ChampionshipSince != 7 || st.CurrentEpoch.EpochNumber != 12 || st.StopReason != ga.StopChampionshipStagnated {
		t.Fatalf("cycles %d, championship %d, last epoch %d, reason %q; want 1, 7, 12, championship",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber, st.StopReason)
	}
}

// UT-GA-CY-06: an improving best never ends a cycle; the run reaches the
// epoch cap.
func TestImprovementKeepsTheCycle(t *testing.T) {
	next := 1000.0
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: func(string, *rand.Rand) float64 {
		next-- // every new individual is better than all before it
		return next
	}}
	p := params(10, 2, 40, 5)
	p.CycleCount = 2
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	if sim.State.Cycles != 0 || sim.State.StopReason != "" || sim.State.CurrentEpoch.EpochNumber != 40 {
		t.Fatalf("cycles %d, reason %q, last epoch %d; want 0, none, 40",
			sim.State.Cycles, sim.State.StopReason, sim.State.CurrentEpoch.EpochNumber)
	}
}

// UT-GA-CY-07: the smallest window, 1: a cycle ends the epoch after the best
// stops improving. (Window 0 is rejected before this point: it breaks the
// window metrics, MET-03.)
func TestCycleWindowOne(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 1)
	p.CycleCount = 1
	sim, err := runCycles(t, p, fake)
	if err != nil {
		t.Fatal(err)
	}
	// Cycle 1: epochs 1..2, championship: 3..4.
	st := sim.State
	if st.Cycles != 1 || st.ChampionshipSince != 3 || st.CurrentEpoch.EpochNumber != 4 || st.StopReason != ga.StopChampionshipStagnated {
		t.Fatalf("cycles %d, championship %d, last epoch %d, reason %q; want 1, 3, 4, championship",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber, st.StopReason)
	}
}

// UT-GA-CY-08: an error while refilling the population at a start-over ends
// the run with that error.
func TestStartOverErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness}
	p := params(10, 2, 1000, 5)
	p.CycleCount = 2
	// NewRandomIndividual is first called at the start-over (epoch 6): the
	// regular epochs fill every slot with offspring (10 - 2 elites, even).
	fake.ErrorIn = map[string]error{"NewRandomIndividual": boom}
	_, err := runCycles(t, p, fake)
	if !errors.Is(err, boom) {
		t.Fatalf("error %v, want %v", err, boom)
	}
}

// UT-GA-CY-09: the stop reason and the cycle counters reach the readers
// (the published snapshot).
func TestStopReasonPublished(t *testing.T) {
	m := ga.NewSimulationManager()
	p := params(10, 2, 1000, 5)
	p.CycleCount = 1
	st, err := m.StartSimulation(ga.NewSimulation(p), &fakes.Problem{InputsSize: 5, FitnessFn: constantFitness})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, m, st.ID, ga.StatusCompleted)
	final, err := m.GetSimulationStatistics(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	s := final.Statistics
	if s == nil || s.State == nil {
		t.Fatal("no published state")
	}
	if s.State.StopReason != ga.StopChampionshipStagnated || s.State.Cycles != 1 || s.State.ChampionshipSince != 7 {
		t.Fatalf("published reason %q, cycles %d, championship %d", s.State.StopReason, s.State.Cycles, s.State.ChampionshipSince)
	}
}
