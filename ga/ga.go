package ga

import (
	"csp2d/problem"
	"csp2d/util"
	"math"
	"math/rand"
	"time"
)

func Simulate(job *SimulationJob) error {
	simulation := job.getSimulation()
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	simulation.State = &State{
		CurrentEpoch: EpochState{
			BestFitness:           0,
			AverageFitness:        0,
			AverageEliteFitness:   0,
			FitnessDeviation:      0,
			EliteFitnessDeviation: 0,
			EpochNumber:           0,
		},
		CurrentWindow: WindowState{
			Epochs:                  make([]*EpochState, 0, simulation.Parameters.WindowSize),
			AverageFitness:          0,
			AverageEliteFitness:     0,
			AverageFitnessDeviation: 0,
			EliteFitnessDeviation:   0,
			SaturationRate:          0,
		},
	}

	var eliteCount = simulation.Parameters.EliteCount
	err := job.problem.NewPopulation(simulation.Parameters.PopulationSize, eliteCount, rnd)
	if err != nil {
		return err
	}

	//debug code
	start := time.Now()
	epochs := 0
	defer func() {
		util.Info.Printf("[job=%s] %.2f epochs/s", job.id, float64(epochs)/time.Since(start).Seconds())
	}()

	for !terminationCondition(simulation) {
		select {
		case <-job.PauseChan:
			job.UpdateJobStatus(StatusPaused, nil)
			util.Info.Printf("Simulation %s paused at epoch %d", job.id, simulation.State.CurrentEpoch.EpochNumber)
			job.PauseAckChan <- struct{}{}

			select {
			case <-job.ResumeChan:
				job.UpdateJobStatus(StatusRunning, nil)
				util.Info.Printf("Simulation %s resumed", job.id)
				job.ResumeAckChan <- struct{}{}
			}
		default:
		}

		population := job.problem.GetPopulation()

		err = job.problem.ComputeFitness()
		if err != nil {
			return err
		}

		job.problem.SortSpecies()
		computeState(simulation, population)

		var species = *population.Species
		var size = simulation.Parameters.PopulationSize

		var crossoverCount = size - eliteCount

		var eliteSet = make([]problem.Individual, 0, eliteCount)
		var crossoverSet = make([]*problem.Individual, 0, crossoverCount)

		// Elitism: preserve top individuals
		for i := 0; i < eliteCount; i++ {
			eliteSet = append(eliteSet, species[i])
		}

		// Promote elites into the elite arena BEFORE any new allocations this
		// epoch. After this, elite postfixes are safe even when the
		// generational arenas rotate at the end of the epoch.
		if promoter, ok := job.problem.(interface {
			PromoteElites(eliteCount int)
		}); ok {
			promoter.PromoteElites(eliteCount)
		}

		// Tournament selection
		tournamentSize := 3 // todo: make configurable from UI; Typical values: 2–5
		for len(crossoverSet) < crossoverCount {
			bestIdx := tournamentSelect(size, tournamentSize, rnd)
			crossoverSet = append(crossoverSet, &species[bestIdx])
		}

		population.Selected = eliteSet
		job.problem.Protect(eliteSet)
		epochNumber := simulation.State.CurrentEpoch.EpochNumber

		// each 2 parents produce 2 offsprings but remain immutable
		newGeneration, failuresCrossover, err1 := job.problem.Crossover(crossoverSet, rnd)
		if err1 != nil {
			return err1
		}
		simulation.State.CurrentEpoch.Failures += failuresCrossover

		_, _, failuresMutation, err2 := job.problem.Mutation(simulation.Parameters.MutationRate, newGeneration, rnd)
		if err2 != nil {
			return err2
		}
		simulation.State.CurrentEpoch.Failures += failuresMutation

		// repopulating the generation except the elite
		newGenerationCount := len(newGeneration)
		var failuresGeneration = 0
		for i := eliteCount; i < size; i++ {
			if i-eliteCount < newGenerationCount {
				species[i] = *newGeneration[i-eliteCount]
			} else { // can be different by 1 if crossoverCount is odd
				ind, err3 := job.problem.NewRandomIndividual(rnd)
				if err3 != nil {
					return err3
				}
				if ind == nil {
					failuresGeneration = failuresGeneration + 1
				}
				species[i] = ind
			}
		}
		if failuresGeneration > 0 {
			util.Warning.Printf("Unable to generate %d species. All possible combinations exhausted. Epoch %d.\n", failuresGeneration, epochNumber)
		}

		// End of epoch: rotate generational arenas. The just-produced offspring
		// become the next epoch's parents (currentGen); the old parents' memory
		// is released in O(1). Elites are unaffected (they live in eliteArena).
		if rotator, ok := job.problem.(interface{ RotateGenerations() }); ok {
			rotator.RotateGenerations()
		}

		adaptParameters(simulation.State, simulation.Parameters)

		//debug code
		epochs++
	}
	return nil
}

func tournamentSelect(populationSize int, k int, rnd *rand.Rand) int {
	if k > populationSize {
		k = populationSize
	}
	bestIdx := rnd.Intn(populationSize)
	for i := 1; i < k; i++ {
		candidate := rnd.Intn(populationSize)
		if candidate < bestIdx {
			bestIdx = candidate
		}
	}
	return bestIdx
}

func terminationCondition(simulation *Simulation) bool {
	return simulation.State.CurrentEpoch.EpochNumber >= simulation.Parameters.MaxEpochsCount
}

func computeState(simulation *Simulation, population *problem.Population) {
	failures := simulation.State.CurrentEpoch.Failures
	simulation.State.CurrentEpoch = *computeEpochState(simulation, population)
	epochClone := simulation.State.CurrentEpoch
	epochClone.Failures = failures
	simulation.State.CurrentWindow.addEpoch(&epochClone, simulation.Parameters.WindowSize)
	simulation.State.computeWindowState()
}

func computeEpochState(simulation *Simulation, population *problem.Population) *EpochState {
	var species = *population.Species
	var size = simulation.Parameters.PopulationSize
	var eliteCount = simulation.Parameters.EliteCount
	var avgFitness = 0.0
	var avgEliteFitness = 0.0
	var fitnessVariance = 0.0
	var eliteFitnessVariance = 0.0
	for i := 0; i < size; i++ {
		if i < eliteCount {
			avgEliteFitness += species[i].GetFitnessValue()
		}
		avgFitness += species[i].GetFitnessValue()
	}
	avgFitness = avgFitness / float64(size)
	avgEliteFitness = avgEliteFitness / float64(eliteCount)
	for i := 0; i < size; i++ {
		if i < eliteCount {
			eliteFitnessVariance += math.Pow(species[i].GetFitnessValue()-avgEliteFitness, 2)
		}
		fitnessVariance += math.Pow(species[i].GetFitnessValue()-avgFitness, 2)
	}
	fitnessDev := math.Sqrt(fitnessVariance / float64(size))
	eliteFitnessDev := math.Sqrt(eliteFitnessVariance / float64(eliteCount))
	return &EpochState{
		Failures:              0,
		BestFitness:           species[0].GetFitnessValue(),
		AverageFitness:        avgFitness,
		FitnessDeviation:      fitnessDev,
		AverageEliteFitness:   avgEliteFitness,
		EliteFitnessDeviation: eliteFitnessDev,
		EpochNumber:           simulation.State.CurrentEpoch.EpochNumber + 1,
	}
}

func adaptParameters(state *State, parameters *Parameters) {
	if state.CurrentEpoch.EpochNumber < parameters.WindowSize {
		return
	}
	if state.ConvergedSince > 0 { // Converged to the minimum
		parameters.MaxEpochsCount = state.CurrentEpoch.EpochNumber
	}
}
