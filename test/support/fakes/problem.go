// Package fakes provides scripted implementations of problem.Problem and
// problem.Individual for unit-testing the generic GA engine (ga package)
// without the CSP-2D problem.
package fakes

import (
	"csp2d/problem"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Individual is a fake GA individual with a scalar fitness.
type Individual struct {
	ID       int
	Fitness  float64
	Computed bool
	Origin   string // "initial", "random", "crossover", "mutation"
}

func (i *Individual) IsFitnessComputed() bool     { return i.Computed }
func (i *Individual) GetFitnessValue() float64    { return i.Fitness }
func (i *Individual) GetFitnessVector() []float64 { return []float64{i.Fitness, 0} }

// Problem is a fake problem.Problem that records every call in order.
// Zero values give a working default; set the function fields to script
// behaviour. All exported methods are safe for concurrent use with Calls().
type Problem struct {
	InputsSize int

	// FitnessFn returns the fitness of a newly created individual (default:
	// uniform random in [0, 1000)).
	FitnessFn func(origin string, rnd *rand.Rand) float64
	// Errors / panics injected by method name ("ComputeFitness", "Crossover",
	// "Mutation", "NewRandomIndividual", "NewPopulation").
	ErrorIn map[string]error
	PanicIn map[string]bool
	// NilRandomIndividual makes NewRandomIndividual return (nil, nil).
	NilRandomIndividual bool
	// EpochDelay sleeps inside ComputeFitness to slow epochs down.
	EpochDelay time.Duration

	mu         sync.Mutex
	calls      []string
	nextID     int
	population *problem.Population
	protected  []problem.Individual

	// Observations
	LastCrossoverInput int
	LastPromoteCount   int
}

func (p *Problem) record(name string) error {
	p.mu.Lock()
	p.calls = append(p.calls, name)
	p.mu.Unlock()
	if p.PanicIn[name] {
		panic(fmt.Sprintf("fake panic in %s", name))
	}
	if err := p.ErrorIn[name]; err != nil {
		return err
	}
	return nil
}

// Calls returns a copy of the recorded call sequence.
func (p *Problem) Calls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.calls...)
}

// ResetCalls clears the recorded calls.
func (p *Problem) ResetCalls() {
	p.mu.Lock()
	p.calls = nil
	p.mu.Unlock()
}

// Protected returns the individuals passed to the last Protect call.
func (p *Problem) Protected() []problem.Individual {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]problem.Individual(nil), p.protected...)
}

func (p *Problem) newIndividual(origin string, rnd *rand.Rand) *Individual {
	p.mu.Lock()
	p.nextID++
	id := p.nextID
	p.mu.Unlock()
	f := rnd.Float64() * 1000
	if p.FitnessFn != nil {
		f = p.FitnessFn(origin, rnd)
	}
	return &Individual{ID: id, Fitness: f, Origin: origin}
}

func (p *Problem) GetInputsSize() int { return p.InputsSize }

func (p *Problem) GetPopulation() *problem.Population {
	_ = p.record("GetPopulation")
	return p.population
}

func (p *Problem) SetPopulation(population *problem.Population) {
	_ = p.record("SetPopulation")
	p.population = population
}

func (p *Problem) NewPopulation(size int, eliteCount int, rnd *rand.Rand) error {
	if err := p.record("NewPopulation"); err != nil {
		return err
	}
	species := make([]problem.Individual, size)
	for i := range species {
		species[i] = p.newIndividual("initial", rnd)
	}
	p.population = &problem.Population{Species: &species}
	return nil
}

func (p *Problem) NewRandomIndividual(rnd *rand.Rand) (problem.Individual, error) {
	if err := p.record("NewRandomIndividual"); err != nil {
		return nil, err
	}
	if p.NilRandomIndividual {
		return nil, nil
	}
	return p.newIndividual("random", rnd), nil
}

func (p *Problem) ComputeFitness() error {
	if err := p.record("ComputeFitness"); err != nil {
		return err
	}
	if p.EpochDelay > 0 {
		time.Sleep(p.EpochDelay)
	}
	for _, ind := range *p.population.Species {
		if fi, ok := ind.(*Individual); ok {
			fi.Computed = true
		}
	}
	return nil
}

func (p *Problem) SortSpecies() {
	_ = p.record("SortSpecies")
	species := *p.population.Species
	sort.SliceStable(species, func(i, j int) bool {
		return species[i].GetFitnessValue() < species[j].GetFitnessValue()
	})
}

func (p *Problem) Protect(individuals []problem.Individual) {
	_ = p.record("Protect")
	p.mu.Lock()
	p.protected = append([]problem.Individual(nil), individuals...)
	p.mu.Unlock()
}

func (p *Problem) Crossover(crossoverSet []*problem.Individual, rnd *rand.Rand) ([]*problem.Individual, int, error) {
	if err := p.record("Crossover"); err != nil {
		return nil, 0, err
	}
	p.LastCrossoverInput = len(crossoverSet)
	out := make([]*problem.Individual, 0, len(crossoverSet))
	for i := 0; i+1 < len(crossoverSet); i += 2 {
		var a problem.Individual = p.newIndividual("crossover", rnd)
		var b problem.Individual = p.newIndividual("crossover", rnd)
		out = append(out, &a, &b)
	}
	return out, 0, nil
}

func (p *Problem) Mutation(mutationRate float64, mutationSet []*problem.Individual, rnd *rand.Rand) (int, int, int, error) {
	if err := p.record("Mutation"); err != nil {
		return 0, 0, 0, err
	}
	return 0, 0, 0, nil
}

// PromoteElites and RotateGenerations are the optional memory hooks the GA
// loop discovers by type assertion.
func (p *Problem) PromoteElites(eliteCount int) {
	_ = p.record("PromoteElites")
	p.LastPromoteCount = eliteCount
}

func (p *Problem) RotateGenerations() { _ = p.record("RotateGenerations") }

// Compile-time interface check.
var _ problem.Problem = (*Problem)(nil)
