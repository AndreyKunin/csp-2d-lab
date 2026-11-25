//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"csp2d/problem"
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
	if sim.State.Cycles != 1 || sim.State.CurrentEpoch.EpochNumber != 11 {
		t.Fatalf("cycles %d, last epoch %d; want 1 and 11 (the epoch cap)", sim.State.Cycles, sim.State.CurrentEpoch.EpochNumber)
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
	if st.Cycles != 0 || st.ChampionshipSince != 0 || st.CurrentEpoch.EpochNumber != 6 {
		t.Fatalf("cycles %d, championship %d, last epoch %d; want 0, 0, 6",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber)
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
	if st.Cycles != 1 || st.ChampionshipSince != 7 || st.CurrentEpoch.EpochNumber != 12 {
		t.Fatalf("cycles %d, championship %d, last epoch %d; want 1, 7, 12",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber)
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
	if sim.State.Cycles != 0 || sim.State.CurrentEpoch.EpochNumber != 40 {
		t.Fatalf("cycles %d, last epoch %d; want 0, 40",
			sim.State.Cycles, sim.State.CurrentEpoch.EpochNumber)
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
	if st.Cycles != 1 || st.ChampionshipSince != 3 || st.CurrentEpoch.EpochNumber != 4 {
		t.Fatalf("cycles %d, championship %d, last epoch %d; want 1, 3, 4",
			st.Cycles, st.ChampionshipSince, st.CurrentEpoch.EpochNumber)
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

// UT-GA-CY-09: the cycle counters and the final epoch of a run that ends by
// stagnation reach the readers (the published snapshot).
func TestCycleStatePublished(t *testing.T) {
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
	if s.State.Cycles != 1 || s.State.ChampionshipSince != 7 || s.State.CurrentEpoch.EpochNumber != 12 {
		t.Fatalf("published cycles %d, championship %d, epoch %d; want 1, 7, 12",
			s.State.Cycles, s.State.ChampionshipSince, s.State.CurrentEpoch.EpochNumber)
	}
}

// searchProblem is a fake problem that records the search parameters the engine
// passes it every epoch.
type searchProblem struct {
	*fakes.Problem
	calls  int
	repair int
	flip   int
	donor  float64
}

func (s *searchProblem) SetSearchParameters(repairWeight, flipWeight int, donorFraction float64) {
	s.calls++
	s.repair, s.flip, s.donor = repairWeight, flipWeight, donorFraction
}

// UT-GA-SP-01: the engine passes the repair weight, flip weight and donor
// fraction to the problem every epoch, unchanged.
func TestSearchParametersPassedEveryEpoch(t *testing.T) {
	sp := &searchProblem{Problem: &fakes.Problem{InputsSize: 5}}
	p := params(10, 2, 7, 1000)
	p.RepairWeight, p.FlipWeight, p.DonorFraction = 6, 2, 0.75
	job := ga.NewSimulationJob("test", ga.NewSimulation(p), sp)
	var err error
	testutil.Within(t, 10*time.Second, "Simulate", func() { err = ga.Simulate(job) })
	if err != nil {
		t.Fatal(err)
	}
	if sp.calls != 7 || sp.repair != 6 || sp.flip != 2 || sp.donor != 0.75 {
		t.Fatalf("%d calls with repair %d, flip %d, donor %v; want 7 calls with 6, 2, 0.75", sp.calls, sp.repair, sp.flip, sp.donor)
	}
}

// UT-GA-SP-02: a reconfigured search parameter reaches the problem from the
// next epoch on.
func TestSearchParametersFollowConfigure(t *testing.T) {
	sp := &searchProblem{Problem: &fakes.Problem{InputsSize: 5, EpochDelay: 2 * time.Millisecond}}
	m := ga.NewSimulationManager()
	p := params(10, 2, 1000000, 1000000)
	p.RepairWeight, p.FlipWeight, p.DonorFraction = 3, 1, 0.5
	st, err := m.StartSimulation(ga.NewSimulation(p), sp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = m.DeleteSimulation(st.ID) }()
	if err := m.PauseSimulation(st.ID); err != nil {
		t.Fatal(err)
	}
	q := params(10, 2, 1000000, 1000000)
	q.RepairWeight, q.FlipWeight, q.DonorFraction = 9, 4, 0.25
	if err := m.ConfigureSimulation(st.ID, q); err != nil {
		t.Fatal(err)
	}
	if err := m.ResumeSimulation(st.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := m.PauseSimulation(st.ID); err != nil {
			t.Fatal(err)
		}
		if sp.repair == 9 && sp.flip == 4 && sp.donor == 0.25 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("problem still has repair %d, flip %d, donor %v; want 9, 4, 0.25", sp.repair, sp.flip, sp.donor)
		}
		if err := m.ResumeSimulation(st.ID); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// tournamentCounter is a fake problem that counts how many individuals reach
// Crossover; with tournament size k, the share of the best individual among
// the selected parents grows with k.
type tournamentCounter struct {
	*fakes.Problem
	best, total int
}

func (c *tournamentCounter) Crossover(set []*problem.Individual, rnd *rand.Rand) ([]*problem.Individual, int, error) {
	species := *c.GetPopulation().Species
	for _, ind := range set {
		if *ind == species[0] {
			c.best++
		}
		c.total++
	}
	return c.Problem.Crossover(set, rnd)
}

func bestShare(t *testing.T, k int) float64 {
	t.Helper()
	c := &tournamentCounter{Problem: &fakes.Problem{InputsSize: 5}}
	p := params(50, 2, 40, 1000)
	p.TournamentSize = k
	job := ga.NewSimulationJob("test", ga.NewSimulation(p), c)
	job.SetSeedForTest(7)
	var err error
	testutil.Within(t, 10*time.Second, "Simulate", func() { err = ga.Simulate(job) })
	if err != nil {
		t.Fatal(err)
	}
	return float64(c.best) / float64(c.total)
}

// UT-GA-SP-03: the tournament size comes from the parameters: a larger size
// selects the best individual more often (for 50 individuals, with
// replacement: 1-(49/50)^k of each draw: k=1 2 %, k=5 9.6 %, k=20 33 %).
// 0 or less takes the default 5.
func TestTournamentSizeFromParameters(t *testing.T) {
	one, five, twenty := bestShare(t, 1), bestShare(t, 5), bestShare(t, 20)
	if !(one < five && five < twenty) {
		t.Fatalf("best-individual share k=1 %.3f, k=5 %.3f, k=20 %.3f; want increasing", one, five, twenty)
	}
	if def := bestShare(t, 0); def != five {
		t.Fatalf("tournament size 0 gives share %.4f, want the default 5's %.4f", def, five)
	}
}
