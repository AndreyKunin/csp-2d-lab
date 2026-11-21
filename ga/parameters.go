package ga

type Parameters struct {
	EliteCount     int     `json:"eliteCount"`     // Max elite count
	MutationRate   float64 `json:"mutationRate"`   // Global Mutation Rate (across the whole generation)
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
	// TournamentSize is the tournament size of the selection; 0 or less = 5.
	TournamentSize int `json:"tournamentSize"`
	// RepairWeight and FlipWeight are the mutation budget units an accepted
	// eliminate-and-repair and an accepted critical flip cost; 0 or less = 3
	// and 1. DonorFraction is the largest donor subtree of the crossover, as a
	// share of the parts; 0 or less, or above 1 = 0.5. They are passed to the
	// problem every epoch (its optional SetSearchParameters).
	RepairWeight  int     `json:"repairWeight"`
	FlipWeight    int     `json:"flipWeight"`
	DonorFraction float64 `json:"donorFraction"`
}
