//go:build testhooks

// Test hooks: thin exported wrappers around unexported ga functions, so that
// tests living under test/ (outside this package) can reach them. Compiled ONLY
// with `-tags testhooks`; production builds never contain it.

package ga

import (
	"csp2d/problem"
	"math/rand"
)

func TournamentSelectForTest(populationSize int, k int, rnd *rand.Rand) int {
	return tournamentSelect(populationSize, k, rnd)
}

func TerminationConditionForTest(simulation *Simulation) bool {
	return terminationCondition(simulation)
}

func ComputeStateForTest(simulation *Simulation, population *problem.Population) {
	computeState(simulation, population)
}

func ComputeEpochStateForTest(simulation *Simulation, population *problem.Population) *EpochState {
	return computeEpochState(simulation, population)
}

func AdaptParametersForTest(state *State, parameters *Parameters) {
	adaptParameters(state, parameters)
}

func (window *WindowState) AddEpochForTest(newState *EpochState, windowSize int) {
	window.addEpoch(newState, windowSize)
}

func (state *State) ComputeWindowStateForTest() {
	state.computeWindowState()
}

// JobForTest gives tests access to a job's simulation (for driving Simulate).
func (job *SimulationJob) SimulationForTest() *Simulation { return job.getSimulation() }
