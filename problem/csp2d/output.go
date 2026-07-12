package csp2d

import "sort"

type SheetLayout struct {
	Number int    `json:"number"`
	Tree   string `json:"tree"`
}

type Solution struct {
	Sheets        []SheetLayout `json:"sheets"`
	Fitness       float64       `json:"fitness"`
	FitnessVector [3]float64    `json:"fitnessVector"`
}

type Response struct {
	Solutions []Solution `json:"solutions"`
}

func NewResponse(problem Csp2dProblem) (*Response, error) {
	var solutions []Solution
	population := problem.GetPopulation()
	for _, ind := range population.Selected {
		individual := ind.(*Csp2dIndividualImpl)
		var layouts []SheetLayout
		for i, mn := range individual.meteredForest {
			layouts = append(layouts, SheetLayout{
				Number: i,
				Tree:   problem.toString(mn.postfix),
			})
		}

		solutions = append(solutions, Solution{
			Sheets:        layouts,
			Fitness:       individual.fitnessValue,
			FitnessVector: individual.fitnessVector,
		})
	}

	sort.Slice(solutions, func(i, j int) bool {
		return solutions[i].Fitness < solutions[j].Fitness
	})

	return &Response{
		Solutions: solutions,
	}, nil
}
