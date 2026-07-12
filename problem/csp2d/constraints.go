package csp2d

type Constraints struct {
	maxLongSide  float64
	maxShortSide float64
	sawWidth     float64
}

func NewConstraints(maxLongSide, maxShortSide, sawWidth float64) *Constraints {
	return &Constraints{
		maxLongSide:  maxLongSide,
		maxShortSide: maxShortSide,
		sawWidth:     sawWidth,
	}
}
