package ga

// EpochState Some state metrics are actual in the single epoch only
type EpochState struct {
	BestFitness           float64 `json:"bestFitness"`           // Best Fitness
	AverageFitness        float64 `json:"averageFitness"`        // Average Fitness
	FitnessDeviation      float64 `json:"fitnessDeviation"`      // Current Fitness Deviation
	AverageEliteFitness   float64 `json:"averageEliteFitness"`   // Average Elite Fitness
	EliteFitnessDeviation float64 `json:"eliteFitnessDeviation"` // Current Elite Fitness Deviation
	EpochNumber           int     `json:"epochNumber"`           // Epoch number
	Failures              int     `json:"failures"`              // Number of crossover and mutation failures
}

// WindowState Some state metrics are computed for some sliding window
type WindowState struct {
	Epochs                  []*EpochState `json:"-"`                       // Sliding window
	AverageFitness          float64       `json:"averageFitness"`          // Current Fitness
	AverageFitnessDeviation float64       `json:"averageFitnessDeviation"` // Average Fitness Deviation (Diversity Index)
	AverageEliteFitness     float64       `json:"averageEliteFitness"`     // Average Elite Fitness
	EliteFitnessDeviation   float64       `json:"eliteFitnessDeviation"`   // Current Elite Fitness Deviation
	SaturationRate          float64       `json:"saturationRate"`          // Crossover/mutation failures (migrants) per individual, averaged over the window
}

func (window *WindowState) addEpoch(newState *EpochState, windowSize int) {
	var epochs = window.Epochs
	var index = newState.EpochNumber - 1
	if index < windowSize {
		window.Epochs = append(window.Epochs, newState)
	} else {
		for i := 1; i < windowSize; i++ {
			epochs[i-1] = epochs[i]
		}
		epochs[windowSize-1] = newState
	}
}

type State struct {
	CurrentEpoch  EpochState  `json:"epochState"`  // State changed each epoch
	CurrentWindow WindowState `json:"windowState"` // Summary state over a sliding window
	// Cycles and championship (ga.go): finished cycles and the epoch the
	// championship began (0 = not yet).
	Cycles            int `json:"cycles"`
	ChampionshipSince int `json:"championshipSince"`
}

func (state *State) computeWindowState(populationSize int) {
	var window = state.CurrentWindow
	var epochs = window.Epochs
	var size = len(epochs)
	var avgFitness = 0.0
	var avgFitnessDeviation = 0.0
	var avgEliteFitness = 0.0
	var avgEliteFitnessDeviation = 0.0
	failures := 0
	for i := 0; i < size; i++ {
		avgFitness += epochs[i].AverageFitness
		avgFitnessDeviation += epochs[i].FitnessDeviation
		avgEliteFitness += epochs[i].AverageEliteFitness
		avgEliteFitnessDeviation += epochs[i].EliteFitnessDeviation
		failures += epochs[i].Failures
	}
	window.AverageFitness = avgFitness / float64(size)
	window.AverageFitnessDeviation = avgFitnessDeviation / float64(size)
	window.AverageEliteFitness = avgEliteFitness / float64(size)
	window.EliteFitnessDeviation = avgEliteFitnessDeviation / float64(size)
	window.SaturationRate = 0
	if populationSize > 0 {
		window.SaturationRate = float64(failures) / float64(size*populationSize)
	}
	state.CurrentWindow = window
}
