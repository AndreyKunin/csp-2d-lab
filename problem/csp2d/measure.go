package csp2d

type Metrics struct {
	longSide  float64
	shortSide float64
}

func (metrics Metrics) GetArea() float64 {
	return metrics.longSide * metrics.shortSide
}

type MeteredNode struct {
	postfix      []int
	measurements Metrics
}

func (mn *MeteredNode) leafIDs() []int {
	var result []int
	for _, id := range mn.postfix {
		if id >= 0 {
			result = append(result, id)
		}
	}
	return result
}

func (mn *MeteredNode) itemArea(inputs []InputDefinition) float64 {
	var area float64
	for _, id := range mn.postfix {
		if id >= 0 {
			area += inputs[id].LongSide * inputs[id].ShortSide
		}
	}
	return area
}
