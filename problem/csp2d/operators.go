package csp2d

type Op int

const (
	S Op = iota
	L
	OpsCount = 2
)

const (
	NumS = -100
	NumL = -200
)

type OpInfo struct {
	Op
	Num     int
	Str     string
	IsShort bool
}

var opInfoByOp = [OpsCount]OpInfo{
	S: {Op: S, Num: NumS, Str: "S", IsShort: true},
	L: {Op: L, Num: NumL, Str: "L", IsShort: false},
}

func OpByNum(num int) OpInfo {
	if num == NumS {
		return opInfoByOp[S]
	}
	return opInfoByOp[L]
}

func FlipOperator(opNum int) int {
	if opNum == NumS {
		return NumL
	}
	return NumS
}
