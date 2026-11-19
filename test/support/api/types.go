// Package api declares the REST payloads of the csp2d server, independently of
// production code. Functional tests and the oracle use these types; they must
// never import production packages.
package api

// ---- Requests ---------------------------------------------------------------

type Rectangle struct {
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
}

type Part struct {
	Label  string  `json:"label"`
	Width  float64 `json:"width"`
	Length float64 `json:"length"`
	Count  int     `json:"count"`
}

type PopulationSize struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Parameters is the optional parameter block of start/configure requests.
// Pointer fields are omitted when nil.
type Parameters struct {
	PopulationSize *PopulationSize `json:"populationSize,omitempty"`
	EliteCount     *int            `json:"eliteCount,omitempty"`
	MutationRate   *float64        `json:"mutationRate,omitempty"`
	CrossoverRate  *float64        `json:"crossoverRate,omitempty"`
	MaxRate        *float64        `json:"maxRate,omitempty"`
	MinRate        *float64        `json:"minRate,omitempty"`
	MaxEpochsCount *int            `json:"maxEpochsCount,omitempty"`
	WindowSize     *int            `json:"windowSize,omitempty"`
}

// StartRequest is the body of POST /api/v1/simulation/start (and of the
// "request" field of the PDF endpoint).
type StartRequest struct {
	Sheets     *Rectangle  `json:"sheets,omitempty"`
	Parts      []Part      `json:"parts"`
	SawWidth   *float64    `json:"sawWidth,omitempty"`
	Parameters *Parameters `json:"parameters,omitempty"`
}

type ConfigureRequest struct {
	Parameters *Parameters `json:"parameters"`
}

// ---- Responses --------------------------------------------------------------

type ErrorBody struct {
	Message string `json:"message"`
}

type RuntimeParameters struct {
	EliteCount     int     `json:"eliteCount"`
	MutationRate   float64 `json:"mutationRate"`
	CrossoverRate  float64 `json:"crossoverRate"`
	MinRate        float64 `json:"minRate"`
	MaxRate        float64 `json:"maxRate"`
	PopulationSize int     `json:"populationSize"`
	MaxEpochsCount int     `json:"maxEpochsCount"`
	WindowSize     int     `json:"windowSize"`
}

type EpochState struct {
	BestFitness           float64 `json:"bestFitness"`
	AverageFitness        float64 `json:"averageFitness"`
	FitnessDeviation      float64 `json:"fitnessDeviation"`
	AverageEliteFitness   float64 `json:"averageEliteFitness"`
	EliteFitnessDeviation float64 `json:"eliteFitnessDeviation"`
	EpochNumber           int     `json:"epochNumber"`
	Failures              int     `json:"failures"`
}

type WindowState struct {
	AverageFitness          float64 `json:"averageFitness"`
	AverageFitnessDeviation float64 `json:"averageFitnessDeviation"`
	AverageEliteFitness     float64 `json:"averageEliteFitness"`
	EliteFitnessDeviation   float64 `json:"eliteFitnessDeviation"`
	SaturationRate          float64 `json:"saturationRate"`
}

type State struct {
	EpochState     EpochState  `json:"epochState"`
	WindowState    WindowState `json:"windowState"`
	SaturatedSince int         `json:"saturatedSince"`
	ConvergedSince int         `json:"convergedSince"`
	NewEpochSince  int         `json:"newEpochSince"`
}

type Statistics struct {
	Parameters *RuntimeParameters `json:"parameters,omitempty"`
	State      *State             `json:"state,omitempty"`
}

type ProblemInsights struct {
	BestFitnessVector []float64   `json:"bestFitnessVector"`
	SelectedSpecies   [][]float64 `json:"selectedSpecies"`
}

// JobStatus is returned by start/pause/continue/configure/delete and (with
// Statistics and Problem filled) by status.
type JobStatus struct {
	ID         string           `json:"id"`
	Status     string           `json:"status"`
	Error      *string          `json:"error,omitempty"`
	CreatedAt  string           `json:"created_at"`
	UpdatedAt  string           `json:"updated_at"`
	Message    string           `json:"message"`
	Statistics *Statistics      `json:"statistics,omitempty"`
	Problem    *ProblemInsights `json:"problem,omitempty"`
}

const (
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusError     = "error"
)

type SheetLayout struct {
	Number int    `json:"number"`
	Tree   string `json:"tree"`
}

type Solution struct {
	Sheets        []SheetLayout `json:"sheets"`
	Fitness       float64       `json:"fitness"`
	FitnessVector []float64     `json:"fitnessVector"`
}

type Result struct {
	Solutions []Solution `json:"solutions"`
}

// DefaultParameters is returned by GET /api/v1/service/default-parameters.
type DefaultParameters = Parameters

// ---- Helpers ----------------------------------------------------------------

func Int(v int) *int               { return &v }
func Float(v float64) *float64     { return &v }
func Rect(w, l float64) *Rectangle { return &Rectangle{Width: w, Length: l} }
