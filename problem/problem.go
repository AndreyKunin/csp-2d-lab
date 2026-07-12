package problem

import "math/rand"

type Individual interface {
	IsFitnessComputed() bool
	GetFitnessValue() float64
	GetFitnessVector() []float64
}

type Population struct {
	Species  *[]Individual
	Selected []Individual
}

type Problem interface {
	GetInputsSize() int
	GetPopulation() *Population
	SetPopulation(population *Population)
	NewPopulation(size int, eliteCount int, rnd *rand.Rand) error
	NewRandomIndividual(rnd *rand.Rand) (Individual, error)
	ComputeFitness() error
	SortSpecies()
	Protect(individuals []Individual)
	Crossover(crossoverSet []*Individual, rnd *rand.Rand) ([]*Individual, int, error)
	Mutation(mutationRate float64, mutationSet []*Individual, rnd *rand.Rand) (int, int, int, error)
}
