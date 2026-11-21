package ga

import (
	"csp2d/problem"
	"csp2d/util"
	"errors"
	"math"
	"math/rand"
	"time"
)

// ErrSimulationCancelled is returned by Simulate when the job was deleted.
var ErrSimulationCancelled = errors.New("simulation cancelled")

func Simulate(job *SimulationJob) error {
	simulation := job.getSimulation()
	rnd := rand.New(rand.NewSource(job.randomSeed()))

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
			// Epochs starts empty and grows by append (addEpoch) up to windowSize:
			// its memory follows the epochs run, never the client-supplied size.
			Epochs:                  nil,
			AverageFitness:          0,
			AverageEliteFitness:     0,
			AverageFitnessDeviation: 0,
			EliteFitnessDeviation:   0,
			SaturationRate:          0,
		},
	}

	job.publish(simulation, nil)

	var eliteCount = simulation.Parameters.EliteCount
	err := job.problem.NewPopulation(simulation.Parameters.PopulationSize, eliteCount, rnd)
	if err != nil {
		return err
	}

	// Cycles and championship (see the loop below).
	var bestValue float64
	lastImprove, seenBest := 0, false
	var savedElites []problem.Individual // the elites of the finished cycles, outside the arenas

	//debug code
	start := time.Now()
	epochs := 0
	defer func() {
		util.Info.Printf("[job=%s] %.2f epochs/s", job.id, float64(epochs)/time.Since(start).Seconds())
	}()

	for !terminationCondition(simulation) {
		select {
		case <-job.done:
			return ErrSimulationCancelled
		case <-job.PauseChan:
			job.UpdateJobStatus(StatusPaused, nil)
			util.Info.Printf("Simulation %s paused at epoch %d", job.id, simulation.State.CurrentEpoch.EpochNumber)
			job.PauseAckChan <- struct{}{}

			select {
			case <-job.ResumeChan:
				job.UpdateJobStatus(StatusRunning, nil)
				util.Info.Printf("Simulation %s resumed", job.id)
				job.ResumeAckChan <- struct{}{}
			case <-job.done:
				return ErrSimulationCancelled
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

		// Cycles and championship: a cycle ends when the best fitness has not
		// improved for WindowSize epochs. Its elites are saved and the whole
		// population (elite slots too) starts over from random individuals; the
		// random source continues. After CycleCount cycles the population is all
		// saved elites plus random individuals (the championship); when that
		// stagnates too, the run ends. CycleCount 0: the run ends at the first
		// stagnation.
		epochNow := simulation.State.CurrentEpoch.EpochNumber
		if v := species[0].GetFitnessValue(); !seenBest || v < bestValue {
			bestValue, seenBest = v, true
			lastImprove = epochNow
		}
		startOver := false // refill every slot this epoch, no offspring
		if window := max(simulation.Parameters.WindowSize, 1); epochNow-lastImprove >= window {
			state := simulation.State
			if state.ChampionshipSince > 0 || simulation.Parameters.CycleCount <= 0 {
				job.publish(simulation, population) // readers see the final epoch
				break                               // the championship (or, with no cycles, the run) stagnated: the run ends
			}
			for i := 0; i < eliteCount; i++ {
				savedElites = append(savedElites, detachIndividual(job.problem, species[i]))
			}
			state.Cycles++
			if state.Cycles >= simulation.Parameters.CycleCount {
				state.ChampionshipSince = epochNow + 1 // the championship population is evaluated next epoch
			}
			startOver = true
			seenBest = false
			lastImprove = epochNow
		}

		var crossoverCount = crossoverSlots(simulation.Parameters.CrossoverRate, size-eliteCount)
		if startOver {
			crossoverCount = 0
		}

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
		tournamentSize := simulation.Parameters.TournamentSize
		if tournamentSize <= 0 {
			tournamentSize = defaultTournamentSize
		}

		// The problem's search parameters may change between epochs (Configure).
		if sp, ok := job.problem.(interface {
			SetSearchParameters(repairWeight, flipWeight int, donorFraction float64)
		}); ok {
			sp.SetSearchParameters(simulation.Parameters.RepairWeight, simulation.Parameters.FlipWeight, simulation.Parameters.DonorFraction)
		}
		for len(crossoverSet) < crossoverCount {
			bestIdx := tournamentSelect(size, tournamentSize, rnd)
			crossoverSet = append(crossoverSet, &species[bestIdx])
		}

		population.Selected = eliteSet
		job.problem.Protect(eliteSet)
		epochNumber := simulation.State.CurrentEpoch.EpochNumber

		var newGeneration []*problem.Individual
		if !startOver { // a start-over epoch leaves newGeneration empty: the loop below refills every slot
			// each 2 parents produce 2 offsprings but remain immutable
			var failuresCrossover int
			var err1 error
			newGeneration, failuresCrossover, err1 = job.reproduce(species, crossoverSet, size-eliteCount-crossoverCount, tournamentSize, rnd)
			if err1 != nil {
				return err1
			}
			simulation.State.CurrentEpoch.Failures += failuresCrossover

			_, _, failuresMutation, err2 := job.problem.Mutation(simulation.Parameters.MutationRate, newGeneration, rnd)
			if err2 != nil {
				return err2
			}
			simulation.State.CurrentEpoch.Failures += failuresMutation
		}

		// repopulating the generation except the elite (start-over: every slot,
		// the saved elites first when the championship begins)
		newGenerationCount := len(newGeneration)
		var failuresGeneration = 0
		first := eliteCount
		var seeds []problem.Individual
		if startOver {
			first = 0
			if simulation.State.ChampionshipSince > 0 {
				seeds = savedElites
			}
		}
		for i := first; i < size; i++ {
			if i < len(seeds) {
				species[i] = copyIndividual(job.problem, seeds[i])
			} else if i >= eliteCount && i-eliteCount < newGenerationCount {
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

		// The only point where HTTP readers get new data (see SimulationJob.publish).
		job.publish(simulation, population)

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

// detachIndividual returns a copy of the individual that survives any number
// of epochs: the problem's optional DetachIndividual (outside its memory
// arenas), else the individual itself.
func detachIndividual(p problem.Problem, individual problem.Individual) problem.Individual {
	if d, ok := p.(interface {
		DetachIndividual(problem.Individual) problem.Individual
	}); ok {
		return d.DetachIndividual(individual)
	}
	return individual
}

// copyIndividual brings a detached individual back into the population: the
// problem's optional CopyIndividual (into its generation memory), else the
// individual itself.
func copyIndividual(p problem.Problem, individual problem.Individual) problem.Individual {
	if c, ok := p.(interface {
		CopyIndividual(problem.Individual) problem.Individual
	}); ok {
		return c.CopyIndividual(individual)
	}
	return individual
}

// defaultTournamentSize is the tournament size when Parameters.TournamentSize
// is not set: 5, measured on G060 against 2 and 3 (design.md §16.12).
const defaultTournamentSize = 5

// crossoverSlots returns how many of the n non-elite slots are filled by
// crossover: all of them unless 0 < rate < 1.
func crossoverSlots(rate float64, n int) int {
	if rate <= 0 || rate >= 1 {
		return n
	}
	return int(math.Round(rate * float64(n)))
}

// reproduce returns the offspring of the crossover set followed by copies of
// `copies` tournament-selected individuals: the slots that crossover skips
// (CrossoverRate < 1) are left to mutation alone. A copy takes its own memory
// (CopyIndividual), because the parents' memory is released at the end of the
// epoch.
func (job *SimulationJob) reproduce(species []problem.Individual, crossoverSet []*problem.Individual, copies, k int, rnd *rand.Rand) ([]*problem.Individual, int, error) {
	offspring, failures, err := job.problem.Crossover(crossoverSet, rnd)
	if err != nil || copies <= 0 {
		return offspring, failures, err
	}
	copier, ok := job.problem.(interface {
		CopyIndividual(problem.Individual) problem.Individual
	})
	if !ok {
		return nil, failures, errors.New("crossoverRate < 1 needs a problem that can copy individuals")
	}
	for i := 0; i < copies; i++ {
		c := copier.CopyIndividual(species[tournamentSelect(len(species), k, rnd)])
		offspring = append(offspring, &c)
	}
	return offspring, failures, nil
}
