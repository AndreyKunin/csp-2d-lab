package csp2d

import "csp2d/problem"

type LexicographicalSort []problem.Individual

func (a LexicographicalSort) Len() int { return len(a) }
func (a LexicographicalSort) Less(i, j int) bool {
	return LessFitness(a[i].(*Csp2dIndividualImpl), a[j].(*Csp2dIndividualImpl))
}
func (a LexicographicalSort) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func LessFitness(ind1 *Csp2dIndividualImpl, ind2 *Csp2dIndividualImpl) bool {
	for k := 0; k < len(ind1.fitnessVector); k++ {
		if ind1.fitnessVector[k] < ind2.fitnessVector[k] {
			return true
		}
		if ind1.fitnessVector[k] > ind2.fitnessVector[k] {
			return false
		}
	}
	return false
}

func LessFitnessRaw(fit1 [3]float64, fit2 [3]float64) bool {
	for k := 0; k < len(fit1); k++ {
		if fit1[k] < fit2[k] {
			return true
		}
		if fit1[k] > fit2[k] {
			return false
		}
	}
	return false
}
