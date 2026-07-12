package ga

type Parameters struct {
	EliteCount     int     `json:"eliteCount"`     // Max elite count
	MutationRate   float64 `json:"mutationRate"`   // Global Mutation Rate (across the whole generation)
	MinRate        float64 `json:"minRate"`        // Minimum allowed rate for any value
	MaxRate        float64 `json:"maxRate"`        // Maximum allowed rate for any value
	PopulationSize int     `json:"populationSize"` // Population Size
	MaxEpochsCount int     `json:"maxEpochsCount"` // Maximum number of epochs
	WindowSize     int     `json:"windowSize"`     // Window size
}
