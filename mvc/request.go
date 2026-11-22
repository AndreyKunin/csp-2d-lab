package mvc

import (
	"csp2d/ga"
	"csp2d/problem/csp2d"
	"csp2d/util"
	"math"
	"strconv"
)

type Rectangle struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
}

type PartDefinition struct {
	Label  string  `json:"label"`
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
	Count  int     `json:"count"`
}

type SimulationParameters struct {
	PopulationSize *struct {
		Min *int `json:"min,omitempty" yaml:"min"` // omitted: the configured minimum
		Max *int `json:"max,omitempty" yaml:"max"` // omitted: the configured maximum
	} `json:"populationSize" yaml:"populationSize"`
	EliteCount     *int     `json:"eliteCount"            yaml:"eliteCount"`
	MutationRate   *float64 `json:"mutationRate"          yaml:"mutationRate"`
	CrossoverRate  *float64 `json:"crossoverRate"         yaml:"crossoverRate"`
	CycleCount     *int     `json:"cycleCount"            yaml:"cycleCount"`
	TournamentSize *int     `json:"tournamentSize"        yaml:"tournamentSize"`
	RepairWeight   *int     `json:"repairWeight"          yaml:"repairWeight"`
	FlipWeight     *int     `json:"flipWeight"            yaml:"flipWeight"`
	DonorFraction  *float64 `json:"donorFraction"         yaml:"donorFraction"`
	MaxEpochsCount *int     `json:"maxEpochsCount"        yaml:"maxEpochsCount"`
	WindowSize     *int     `json:"windowSize"            yaml:"windowSize"`
}

type Request struct {
	Sheets     *Rectangle            `json:"sheets"`
	Parts      []PartDefinition      `json:"parts"`
	SawWidth   *float64              `json:"sawWidth"`
	Parameters *SimulationParameters `json:"parameters"`
}

// MinPartsCount is the smallest total part count a simulation accepts: fewer
// parts have too few distinct layouts for the GA (GA-05). The UI checks the
// same limit (Model.validate, MIN_TOTAL_COUNT).
const MinPartsCount = 4

// PartsCount returns the total part count of the request: the sum of the
// counts, before BuildInputs expands them.
func (r *Request) PartsCount() int {
	total := 0
	for _, part := range r.Parts {
		total += part.Count
	}
	return total
}

func (r *Request) BuildInputs() ([]csp2d.InputDefinition, error) {
	var inputs []csp2d.InputDefinition
	index := 0
	for _, part := range r.Parts {
		for j := 0; j < part.Count; j++ {
			intLabel, err := strconv.Atoi(part.Label)
			if err != nil {
				return nil, err
			}
			inputs = append(inputs, csp2d.InputDefinition{
				Id:        index,
				Label:     part.Label,
				IntLabel:  intLabel,
				LongSide:  math.Max(part.Width, part.Length),
				ShortSide: math.Min(part.Width, part.Length),
			})
			index++
		}
	}
	return inputs, nil
}

func (r *Request) BuildConstraints() *csp2d.Constraints {
	var constraints *csp2d.Constraints
	_max := -1.0
	_min := -1.0
	_width := 0.0
	if r.Sheets != nil {
		_max = math.Max(r.Sheets.Width, r.Sheets.Length)
		_min = math.Min(r.Sheets.Width, r.Sheets.Length)
	}
	if r.SawWidth != nil {
		_width = *r.SawWidth
	}
	constraints = csp2d.NewConstraints(_max, _min, _width)

	return constraints
}

// BuildParameters merges the request parameters over the defaults, applies the
// small-input overrides (smallInputOverrides) and computes the population size:
// n² clamped to [min, max] (min wins over max), n = inputsCount.
func (r *Request) BuildParameters(defaultParameters SimulationParameters, inputsCount int) ga.Parameters {
	result := ga.Parameters{}
	var popMin, popMax int
	r.buildParameters(defaultParameters, &result, &popMin, &popMax)
	if r.Parameters != nil {
		r.buildParameters(*r.Parameters, &result, &popMin, &popMax)
	}
	smallInputOverrides(inputsCount, &result, &popMin)
	result.PopulationSize = int(math.Max(
		float64(popMin),
		math.Min(math.Pow(float64(inputsCount), 2), float64(popMax)),
	))
	return result
}

// smallInputOverrides replaces parameters that do not suit small inputs
// (GA-05), whatever the defaults or the request say: for n parts in [4, 10]
// the population minimum is n²/2, so the population is n² (fewer distinct
// genotypes exist than the configured minimum, e.g. 40 for 4 identical parts);
// the elite count is 1 for n < 5 and 2 for n in [5, 10]. The UI disables these
// fields when they are overridden (static/src/utils/parameterDefinitions.ts).
func smallInputOverrides(n int, result *ga.Parameters, popMin *int) {
	if n >= 4 && n <= 10 {
		*popMin = n * n / 2
	}
	switch {
	case n < 5:
		result.EliteCount = 1
	case n <= 10:
		result.EliteCount = 2
	}
}

func (r *Request) buildParameters(parameters SimulationParameters, result *ga.Parameters, popMin, popMax *int) {
	if parameters.EliteCount != nil {
		result.EliteCount = *parameters.EliteCount
	}
	if parameters.MutationRate != nil {
		result.MutationRate = *parameters.MutationRate
	}
	if parameters.CrossoverRate != nil {
		result.CrossoverRate = *parameters.CrossoverRate
	}
	if parameters.CycleCount != nil {
		result.CycleCount = *parameters.CycleCount
	}
	if parameters.TournamentSize != nil {
		result.TournamentSize = *parameters.TournamentSize
	}
	if parameters.RepairWeight != nil {
		result.RepairWeight = *parameters.RepairWeight
	}
	if parameters.FlipWeight != nil {
		result.FlipWeight = *parameters.FlipWeight
	}
	if parameters.DonorFraction != nil {
		result.DonorFraction = *parameters.DonorFraction
	}
	if parameters.PopulationSize != nil {
		if parameters.PopulationSize.Min != nil {
			*popMin = *parameters.PopulationSize.Min
		}
		if parameters.PopulationSize.Max != nil {
			*popMax = *parameters.PopulationSize.Max
		}
	}
	if parameters.MaxEpochsCount != nil {
		result.MaxEpochsCount = *parameters.MaxEpochsCount
	}
	if parameters.WindowSize != nil {
		result.WindowSize = *parameters.WindowSize
	}
}
func (r *Request) ToPdfInput() *util.PdfInput {
	parts := make([]util.PdfPart, 0, len(r.Parts))
	for _, p := range r.Parts {
		parts = append(parts, util.PdfPart{
			Label:  p.Label,
			Width:  p.Width,
			Length: p.Length,
			Count:  p.Count,
		})
	}
	var sheets *util.PdfSheet
	if r.Sheets != nil {
		sheets = &util.PdfSheet{
			Width:  r.Sheets.Width,
			Length: r.Sheets.Length,
		}
	}
	sawWidth := 0.0
	if r.SawWidth != nil {
		sawWidth = *r.SawWidth
	}
	return &util.PdfInput{Parts: parts, Sheets: sheets, SawWidth: sawWidth}
}
