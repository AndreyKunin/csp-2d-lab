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

// appendLeafIDs appends the node's part ids to dst.
func (mn *MeteredNode) appendLeafIDs(dst []int) []int {
	for _, id := range mn.postfix {
		if id >= 0 {
			dst = append(dst, id)
		}
	}
	return dst
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
