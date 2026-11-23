//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"math"
	"testing"
)

// UT-GA-EP-02: one elite, or all-equal fitness, gives zero (finite) deviations.
func TestComputeEpochStateDegenerate(t *testing.T) {
	cases := []struct {
		name    string
		elite   int
		fitness []float64
	}{
		{"one elite", 1, []float64{1, 2, 3, 4}},
		{"all equal", 3, []float64{7, 7, 7, 7, 7}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sim := ga.NewSimulation(params(len(c.fitness), c.elite, 10, 5))
			sim.State = &ga.State{}
			st := ga.ComputeEpochStateForTest(sim, population(c.fitness...))
			if st.EliteFitnessDeviation != 0 || math.IsNaN(st.FitnessDeviation) {
				t.Fatalf("elite deviation %g, deviation %g", st.EliteFitnessDeviation, st.FitnessDeviation)
			}
		})
	}
}

func windowOf(epochs ...ga.EpochState) *ga.State {
	st := &ga.State{}
	for i := range epochs {
		e := epochs[i]
		st.CurrentWindow.Epochs = append(st.CurrentWindow.Epochs, &e)
	}
	st.CurrentEpoch = epochs[len(epochs)-1]
	return st
}

// UT-GA-WS-04: window averages and the saturation formula.
func TestComputeWindowState(t *testing.T) {
	st := windowOf(
		ga.EpochState{EpochNumber: 1, AverageFitness: 10, FitnessDeviation: 2, AverageEliteFitness: 4, EliteFitnessDeviation: 1, Failures: 3},
		ga.EpochState{EpochNumber: 2, AverageFitness: 20, FitnessDeviation: 4, AverageEliteFitness: 6, EliteFitnessDeviation: 3, Failures: 1},
	)
	st.ComputeWindowStateForTest(4)
	w := st.CurrentWindow
	want := map[string][2]float64{
		"averageFitness":          {w.AverageFitness, 15},
		"averageFitnessDeviation": {w.AverageFitnessDeviation, 3},
		"averageEliteFitness":     {w.AverageEliteFitness, 5},
		"eliteFitnessDeviation":   {w.EliteFitnessDeviation, 2},
		"saturationRate":          {w.SaturationRate, 0.5}, // failures per individual: (3+1) / (2 epochs × 4)
	}
	for name, gw := range want {
		if gw[0] != gw[1] {
			t.Errorf("%s = %g, want %g", name, gw[0], gw[1])
		}
	}
}

// UT-GA-WS-07: the saturation rate is the share of the population replaced by
// migrants, averaged over the window: 0 without failures, 1 when every
// individual of every epoch is a migrant, finite for an empty population.
func TestSaturationRateIsShareOfPopulation(t *testing.T) {
	cases := []struct {
		name     string
		failures []int
		pop      int
		want     float64
	}{
		{"no failures", []int{0, 0, 0}, 10, 0},
		{"every individual a migrant", []int{10, 10}, 10, 1},
		{"one epoch in the window", []int{3}, 100, 0.03},
		{"large population", []int{12, 12}, 22500, 12.0 / 22500},
		{"population 0 stays finite", []int{5}, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var epochs []ga.EpochState
			for i, f := range c.failures {
				epochs = append(epochs, ga.EpochState{EpochNumber: i + 1, Failures: f})
			}
			st := windowOf(epochs...)
			st.ComputeWindowStateForTest(c.pop)
			if got := st.CurrentWindow.SaturationRate; math.Abs(got-c.want) > 1e-12 || math.IsNaN(got) {
				t.Fatalf("saturation rate %g, want %g", got, c.want)
			}
		})
	}
}
