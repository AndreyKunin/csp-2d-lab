package ga

type Parameters struct {
	EliteCount     int     `json:"eliteCount"`     // Max elite count
	MutationRate   float64 `json:"mutationRate"`   // Global Mutation Rate (across the whole generation)
	MinRate        float64 `json:"minRate"`        // Minimum allowed rate for any value
	MaxRate        float64 `json:"maxRate"`        // Maximum allowed rate for any value
	PopulationSize int     `json:"populationSize"` // Population Size
	MaxEpochsCount int     `json:"maxEpochsCount"` // Maximum number of epochs
	WindowSize     int     `json:"windowSize"`     // Window size
	// CrossoverRate is the share of the non-elite slots filled by crossover; the
	// rest are copies of selected individuals, left to mutation. 0 means 1 (all).
	CrossoverRate float64 `json:"crossoverRate"`
	// CycleCount is the number of cycles before the championship: a cycle ends
	// when the best fitness has not improved for WindowSize epochs, its elites
	// are kept and the population starts over. 0 = the run ends at the first
	// stagnation.
	CycleCount int `json:"cycleCount"`
}
