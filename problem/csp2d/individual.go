package csp2d

import (
	"csp2d/problem"
)

type Csp2dIndividualImpl struct {
	postfix       []int
	meteredForest []MeteredNode // cached metered forest
	fitnessVector [3]float64
	fitnessValue  float64
}

func newIndividual(postfix []int) problem.Individual {
	return &Csp2dIndividualImpl{
		postfix: postfix,
	}
}

func (individual *Csp2dIndividualImpl) IsFitnessComputed() bool {
	return individual.meteredForest != nil
}

func (individual *Csp2dIndividualImpl) GetFitnessValue() float64 {
	return individual.fitnessValue
}

func (individual *Csp2dIndividualImpl) GetFitnessVector() []float64 {
	return individual.fitnessVector[:2]
}

func (individual *Csp2dIndividualImpl) invalidateCache() {
	individual.meteredForest = nil
	individual.fitnessVector = [3]float64{}
	individual.fitnessValue = 0
}
