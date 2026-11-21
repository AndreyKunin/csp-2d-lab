//go:build testhooks

// Unit tests for csp2d/ga with a fake problem. Build with -tags testhooks.
package ga_test

import (
	"csp2d/ga"
	"csp2d/problem"
	"csp2d/test/support/fakes"
	"csp2d/test/support/testutil"
	"errors"
	"math"
	"math/rand"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func params(pop, elite, epochs, window int) *ga.Parameters {
	return &ga.Parameters{
		EliteCount: elite, MutationRate: 0.05,
		PopulationSize: pop, MaxEpochsCount: epochs, WindowSize: window,
	}
}

func simulate(t *testing.T, p *ga.Parameters, fake *fakes.Problem) error {
	t.Helper()
	job := ga.NewSimulationJob("test", ga.NewSimulation(p), fake)
	var err error
	testutil.Within(t, 10*time.Second, "Simulate", func() { err = ga.Simulate(job) })
	return err
}

// UT-GA-SIM-01: per-epoch call order of the GA loop.
func TestSimulateCallOrder(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5}
	if err := simulate(t, params(10, 2, 3, 1000), fake); err != nil {
		t.Fatal(err)
	}
	epoch := []string{"GetPopulation", "ComputeFitness", "SortSpecies", "PromoteElites", "Protect", "Crossover", "Mutation", "RotateGenerations"}
	want := []string{"NewPopulation"}
	for i := 0; i < 3; i++ {
		want = append(want, epoch...)
	}
	if got := fake.Calls(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("call order\n got: %v\nwant: %v", got, want)
	}
	if fake.LastPromoteCount != 2 {
		t.Errorf("PromoteElites(%d), want 2", fake.LastPromoteCount)
	}
}

// UT-GA-SIM-03 / UT-GA-SIM-04: offspring fill the non-elite slots; with an odd
// number of slots the last one is a random individual.
func TestSimulateOffspringPlacement(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5}
	if err := simulate(t, params(9, 2, 1, 1000), fake); err != nil {
		t.Fatal(err)
	}
	if fake.LastCrossoverInput != 7 {
		t.Errorf("crossover input %d, want 7", fake.LastCrossoverInput)
	}
	species := *fake.GetPopulation().Species
	for i := 2; i < 8; i++ {
		if o := species[i].(*fakes.Individual).Origin; o != "crossover" {
			t.Errorf("slot %d origin %q, want crossover", i, o)
		}
	}
	if o := species[8].(*fakes.Individual).Origin; o != "random" {
		t.Errorf("last slot origin %q, want random", o)
	}
}

// UT-GA-SIM-02: elites are carried over, so the best individual survives
// when all offspring are worse.
func TestSimulateElitism(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5, FitnessFn: func(origin string, rnd *rand.Rand) float64 {
		if origin == "initial" {
			return rnd.Float64() * 100
		}
		return 1000 + rnd.Float64()
	}}
	p := params(12, 3, 5, 1000)
	job := ga.NewSimulationJob("test", ga.NewSimulation(p), fake)
	if err := ga.Simulate(job); err != nil {
		t.Fatal(err)
	}
	selected := fake.GetPopulation().Selected
	if len(selected) != 3 {
		t.Fatalf("%d elites selected, want 3", len(selected))
	}
	for _, e := range selected {
		if e.(*fakes.Individual).Origin != "initial" {
			t.Fatalf("an elite was replaced by a worse offspring: %+v", e)
		}
	}
}

// UT-GA-SIM-05: the loop stops exactly at MaxEpochsCount.
func TestSimulateTermination(t *testing.T) {
	fake := &fakes.Problem{InputsSize: 5}
	p := params(10, 2, 7, 1000)
	job := ga.NewSimulationJob("test", ga.NewSimulation(p), fake)
	if err := ga.Simulate(job); err != nil {
		t.Fatal(err)
	}
	if n := job.SimulationForTest().State.CurrentEpoch.EpochNumber; n != 7 {
		t.Fatalf("stopped at epoch %d, want 7", n)
	}
}

// UT-GA-SIM-06: errors from the problem end the simulation.
func TestSimulateErrorPropagation(t *testing.T) {
	for _, method := range []string{"NewPopulation", "ComputeFitness", "Crossover", "Mutation"} {
		t.Run(method, func(t *testing.T) {
			boom := errors.New("boom in " + method)
			fake := &fakes.Problem{InputsSize: 5, ErrorIn: map[string]error{method: boom}}
			if err := simulate(t, params(10, 2, 3, 1000), fake); !errors.Is(err, boom) {
				t.Fatalf("got %v, want %v", err, boom)
			}
		})
	}
}

// UT-GA-SIM-07: an exhausted gene pool (nil random individual) must not panic.
func TestSimulateNilRandomIndividual(t *testing.T) {
	testutil.Pending(t, "a nil individual from the random fill is stored in the population and panics in the next epoch")
	fake := &fakes.Problem{InputsSize: 5, NilRandomIndividual: true}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Simulate panicked: %v", r)
		}
	}()
	_ = ga.Simulate(ga.NewSimulationJob("test", ga.NewSimulation(params(9, 2, 3, 1000)), fake))
}

// UT-GA-TS-01..03: tournament selection probabilities.
func TestTournamentSelect(t *testing.T) {
	rnd := testutil.Rand(t, 11)
	const n, samples = 10, 200000
	for _, k := range []int{1, 3} {
		counts := make([]int, n)
		for i := 0; i < samples; i++ {
			counts[ga.TournamentSelectForTest(n, k, rnd)]++
		}
		for i := 0; i < n; i++ {
			// P(min of k uniform draws = i) = ((n-i)^k - (n-i-1)^k) / n^k
			want := (math.Pow(float64(n-i), float64(k)) - math.Pow(float64(n-i-1), float64(k))) / math.Pow(n, float64(k))
			got := float64(counts[i]) / samples
			if math.Abs(got-want) > 0.01 {
				t.Errorf("k=%d rank %d: frequency %.4f, want %.4f", k, i, got, want)
			}
		}
	}
	if got := ga.TournamentSelectForTest(1, 5, rnd); got != 0 {
		t.Errorf("population of 1 must select 0, got %d", got)
	}
}

func population(fitness ...float64) *problem.Population {
	species := make([]problem.Individual, len(fitness))
	for i, f := range fitness {
		species[i] = &fakes.Individual{ID: i, Fitness: f, Computed: true}
	}
	return &problem.Population{Species: &species}
}

// UT-GA-EP-01: epoch statistics.
func TestComputeEpochState(t *testing.T) {
	sim := ga.NewSimulation(params(6, 2, 10, 5))
	sim.State = &ga.State{CurrentEpoch: ga.EpochState{EpochNumber: 4}}
	st := ga.ComputeEpochStateForTest(sim, population(1, 2, 3, 4, 5, 6))
	approx := func(name string, got, want float64) {
		if math.Abs(got-want) > 1e-12 {
			t.Errorf("%s = %g, want %g", name, got, want)
		}
	}
	approx("best", st.BestFitness, 1)
	approx("average", st.AverageFitness, 3.5)
	approx("deviation", st.FitnessDeviation, math.Sqrt(17.5/6))
	approx("elite average", st.AverageEliteFitness, 1.5)
	approx("elite deviation", st.EliteFitnessDeviation, 0.5)
	if st.EpochNumber != 5 {
		t.Errorf("epoch %d, want 5", st.EpochNumber)
	}
}

// UT-GA-EP-03: elite count 0 must not produce NaN (NaN breaks JSON encoding).
func TestComputeEpochStateZeroElites(t *testing.T) {
	testutil.Pending(t, "eliteCount 0 divides by zero and yields NaN")
	sim := ga.NewSimulation(params(4, 0, 10, 5))
	sim.State = &ga.State{}
	st := ga.ComputeEpochStateForTest(sim, population(1, 2, 3, 4))
	if math.IsNaN(st.AverageEliteFitness) || math.IsNaN(st.EliteFitnessDeviation) {
		t.Fatal("elite statistics are NaN")
	}
}

// UT-GA-WS-01/02: the sliding window fills up and then slides.
func TestWindowSlides(t *testing.T) {
	var w ga.WindowState
	for e := 1; e <= 5; e++ {
		w.AddEpochForTest(&ga.EpochState{EpochNumber: e, AverageFitness: float64(e)}, 3)
	}
	if len(w.Epochs) != 3 {
		t.Fatalf("window length %d, want 3", len(w.Epochs))
	}
	for i, want := range []int{3, 4, 5} {
		if w.Epochs[i].EpochNumber != want {
			t.Fatalf("window holds epochs %d,%d,%d", w.Epochs[0].EpochNumber, w.Epochs[1].EpochNumber, w.Epochs[2].EpochNumber)
		}
	}
}

// UT-GA-WS-03: shrinking the window at runtime keeps only the newest epochs.
func TestWindowShrink(t *testing.T) {
	testutil.Pending(t, "shrinking windowSize leaves stale epochs in the window")
	var w ga.WindowState
	for e := 1; e <= 10; e++ {
		w.AddEpochForTest(&ga.EpochState{EpochNumber: e}, 10)
	}
	w.AddEpochForTest(&ga.EpochState{EpochNumber: 11}, 4)
	if len(w.Epochs) != 4 {
		t.Fatalf("window length %d after shrinking to 4", len(w.Epochs))
	}
}

// UT-GA-WS-06: the window takes memory for the epochs a job has run, not for
// the client-supplied windowSize: a 3-epoch job allocates about the same with
// a window of 10^7 as with a window of 50.
func TestWindowNotPreallocated(t *testing.T) {
	heap := func(window int) uint64 {
		runtime.GC()
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		if err := simulate(t, params(10, 2, 3, window), &fakes.Problem{InputsSize: 5}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&m1)
		return m1.TotalAlloc - m0.TotalAlloc
	}
	small, huge := heap(50), heap(10_000_000)
	t.Logf("heap per job: window 50 %d KB, window 10^7 %d KB", small/1024, huge/1024)
	if huge > small+1<<20 {
		t.Fatalf("a window of 10^7 costs %d KB more than a window of 50", (huge-small)/1024)
	}
}

var hexID = regexp.MustCompile(`^[0-9a-f]{32}$`)

// UT-GA-MGR-01: ids are unique 32-character hex strings.
func TestManagerIDs(t *testing.T) {
	m := ga.NewSimulationManager()
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 1, 1000)), &fakes.Problem{InputsSize: 3})
		if err != nil {
			t.Fatal(err)
		}
		if !hexID.MatchString(st.ID) || seen[st.ID] {
			t.Fatalf("bad or duplicate id %q", st.ID)
		}
		seen[st.ID] = true
	}
}

// UT-GA-MGR-02: unknown ids are reported.
func TestManagerUnknownID(t *testing.T) {
	m := ga.NewSimulationManager()
	id := "nope"
	checks := map[string]error{
		"pause":  m.PauseSimulation(id),
		"resume": m.ResumeSimulation(id),
	}
	_, checks["status"] = m.GetSimulationStatus(id)
	_, checks["statistics"] = m.GetSimulationStatistics(id)
	_, checks["result"] = m.GetSimulationResult(id)
	_, checks["delete"] = m.DeleteSimulation(id)
	checks["configure"] = m.ConfigureSimulation(id, params(6, 2, 1, 1000))
	for name, err := range checks {
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("%s: got %v, want a 'not found' error", name, err)
		}
	}
}

func waitStatus(t *testing.T, m ga.SimulationManager, id string, want ga.SimulationStatus) ga.SimulationJobStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := m.GetSimulationStatus(id)
		if err != nil {
			t.Fatal(err)
		}
		if st.Status == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("status %s, want %s", st.Status, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// UT-GA-MGR-03 / UT-GA-MGR-07: pause, configure while paused, resume.
func TestManagerPauseConfigureResume(t *testing.T) {
	m := ga.NewSimulationManager()
	fake := &fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond}
	st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 1000000, 1000000)), fake)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ConfigureSimulation(st.ID, params(6, 2, 10, 10)); err == nil {
		t.Error("configure must be rejected while running")
	}
	testutil.Within(t, 5*time.Second, "pause", func() {
		if err := m.PauseSimulation(st.ID); err != nil {
			t.Error(err)
		}
	})
	waitStatus(t, m, st.ID, ga.StatusPaused)
	if err := m.PauseSimulation(st.ID); err == nil || !strings.Contains(err.Error(), "already paused") {
		t.Errorf("second pause: %v", err)
	}
	newParams := params(6, 2, 1000000, 1000000)
	newParams.MutationRate = 0.33
	if err := m.ConfigureSimulation(st.ID, newParams); err != nil {
		t.Fatal(err)
	}
	stats, _ := m.GetSimulationStatistics(st.ID)
	if stats.Statistics.Parameters.MutationRate != 0.33 {
		t.Errorf("configured rate not applied: %g", stats.Statistics.Parameters.MutationRate)
	}
	testutil.Within(t, 5*time.Second, "resume", func() {
		if err := m.ResumeSimulation(st.ID); err != nil {
			t.Error(err)
		}
	})
	waitStatus(t, m, st.ID, ga.StatusRunning)
	if err := m.ResumeSimulation(st.ID); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("second resume: %v", err)
	}
	_ = m.PauseSimulation(st.ID)
}

// UT-GA-MGR-04: control calls on a completed job return promptly.
func TestManagerCompletedJobDoesNotHang(t *testing.T) {
	m := ga.NewSimulationManager()
	st, _ := m.StartSimulation(ga.NewSimulation(params(6, 2, 2, 1000)), &fakes.Problem{InputsSize: 3})
	waitStatus(t, m, st.ID, ga.StatusCompleted)
	testutil.Within(t, time.Second, "pause on completed job", func() { _ = m.PauseSimulation(st.ID) })
	testutil.Within(t, time.Second, "resume on completed job", func() { _ = m.ResumeSimulation(st.ID) })
}

// UT-GA-MGR-06: a panic in the problem becomes an error status.
func TestManagerRecoversPanic(t *testing.T) {
	m := ga.NewSimulationManager()
	fake := &fakes.Problem{InputsSize: 3, PanicIn: map[string]bool{"Crossover": true}}
	st, _ := m.StartSimulation(ga.NewSimulation(params(6, 2, 5, 1000)), fake)
	got := waitStatus(t, m, st.ID, ga.StatusError)
	if got.Error == nil || !strings.Contains(*got.Error, "panicked") {
		t.Fatalf("error %v, want a panic message", got.Error)
	}
}
