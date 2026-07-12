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
		Min int `json:"min" yaml:"min"`
		Max int `json:"max" yaml:"max"`
	} `json:"populationSize" yaml:"populationSize"`
	EliteCount     *int     `json:"eliteCount"            yaml:"eliteCount"`
	MutationRate   *float64 `json:"mutationRate"          yaml:"mutationRate"`
	MaxRate        *float64 `json:"maxRate"               yaml:"maxRate"`
	MinRate        *float64 `json:"minRate"               yaml:"minRate"`
	MaxEpochsCount *int     `json:"maxEpochsCount"        yaml:"maxEpochsCount"`
	WindowSize     *int     `json:"windowSize"            yaml:"windowSize"`
}

type Request struct {
	Sheets     *Rectangle            `json:"sheets"`
	Parts      []PartDefinition      `json:"parts"`
	SawWidth   *float64              `json:"sawWidth"`
	Parameters *SimulationParameters `json:"parameters"`
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

func (r *Request) BuildParameters(defaultParameters SimulationParameters, inputsCount int) ga.Parameters {
	result := ga.Parameters{}
	r.buildParameters(inputsCount, defaultParameters, &result)
	if r.Parameters != nil {
		r.buildParameters(inputsCount, *r.Parameters, &result)
	}
	return result
}

func (r *Request) buildParameters(inputsCount int, parameters SimulationParameters, result *ga.Parameters) {
	if parameters.EliteCount != nil {
		result.EliteCount = *parameters.EliteCount
	}
	if parameters.MutationRate != nil {
		result.MutationRate = *parameters.MutationRate
	}
	if parameters.MaxRate != nil {
		result.MaxRate = *parameters.MaxRate
	}
	if parameters.MinRate != nil {
		result.MinRate = *parameters.MinRate
	}
	if parameters.PopulationSize != nil {
		result.PopulationSize = int(math.Max(
			float64(parameters.PopulationSize.Min),
			math.Min(
				math.Pow(float64(inputsCount), 2),
				float64(parameters.PopulationSize.Max),
			),
		))
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
