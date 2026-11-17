//go:build testhooks

package csp2d_test

import (
	"csp2d/problem/csp2d"
	"csp2d/test/support/oracle"
	"strings"
	"testing"
)

// UT-RED-01..07: reduce, table driven (long1, short1, long2, short2 are
// already normalised).
func TestReduce(t *testing.T) {
	p := newProblem(t, []part{{"1", 1, 1}}, nil, 0, 1, 1)
	cases := []struct {
		name                string
		op                  int
		l1, s1, l2, s2, gap float64
		wantLong, wantShort float64
	}{
		{"S no rotation", csp2d.NumS, 40, 30, 50, 20, 0, 90, 30},
		{"S rotate right child", csp2d.NumS, 100, 60, 50, 40, 0, 140, 60},
		{"S rotate left child", csp2d.NumS, 50, 40, 100, 60, 0, 140, 60},
		{"S tie s1 == l2 does not rotate", csp2d.NumS, 100, 50, 50, 30, 0, 150, 50},
		{"L basic", csp2d.NumL, 40, 30, 50, 20, 0, 50, 50},
		{"L kerf added once", csp2d.NumL, 40, 30, 50, 20, 2, 52, 50},
		{"S kerf added once", csp2d.NumS, 40, 30, 50, 20, 1, 91, 30},
		{"L result normalised", csp2d.NumL, 10, 10, 10, 10, 0, 20, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			long, short := p.ReduceForTest(c.op, c.l1, c.s1, c.l2, c.s2, c.gap)
			if long != c.wantLong || short != c.wantShort {
				t.Fatalf("got %gx%g, want %gx%g", long, short, c.wantLong, c.wantShort)
			}
		})
	}
}

// UT-VAL-01: isValidMetrics.
func TestIsValidMetrics(t *testing.T) {
	p := newProblem(t, []part{{"1", 1, 1}}, &[2]float64{100, 50}, 0, 1, 1)
	cases := []struct {
		long, short float64
		want        bool
	}{
		{100, 50, true},  // exact fit
		{101, 10, false}, // long overflow
		{60, 51, false},  // short overflow
		{90, 40, true},
	}
	for _, c := range cases {
		if got := p.IsValidMetricsForTest(c.long, c.short); got != c.want {
			t.Errorf("%gx%g: got %v, want %v", c.long, c.short, got, c.want)
		}
	}
	unconstrained := newProblem(t, []part{{"1", 1, 1}}, nil, 0, 1, 1)
	if !unconstrained.IsValidMetricsForTest(1e9, 1e9) {
		t.Error("unconstrained problem must accept any size")
	}
}

// UT-FOR-07 / UT-FE-01 parity: the production decoder agrees with the shared
// geometry vectors (which the oracle also checks).
func TestDecoderMatchesSharedTreeVectors(t *testing.T) {
	vectors, err := oracle.LoadTreeVectors()
	if err != nil {
		t.Fatal(err)
	}
	for _, tv := range vectors {
		t.Run(tv.Name, func(t *testing.T) {
			// One input per leaf occurrence, so ids form a permutation.
			var parts []part
			var postfix []int
			for _, tok := range strings.Fields(tv.Tree) {
				switch tok {
				case "S":
					postfix = append(postfix, csp2d.NumS)
				case "L":
					postfix = append(postfix, csp2d.NumL)
				default:
					wl := tv.Parts[tok]
					postfix = append(postfix, len(parts))
					parts = append(parts, part{tok, wl[0], wl[1]})
				}
			}
			p := newProblem(t, parts, nil, tv.Kerf, 4, 1)
			forest, err := p.GetForestForTest(csp2d.NewIndividualForTest(postfix))
			if err != nil {
				t.Fatal(err)
			}
			if len(forest) != 1 {
				t.Fatalf("unconstrained decode produced %d trees", len(forest))
			}
			if forest[0].Long != tv.Long || forest[0].Short != tv.Short {
				t.Fatalf("decoder %gx%g, vector %gx%g", forest[0].Long, forest[0].Short, tv.Long, tv.Short)
			}
		})
	}
}

// UT-FOR-03: when a merge overflows the sheet, the larger child becomes a
// sheet and the smaller one continues.
func TestForestSplitRule(t *testing.T) {
	// 0: 60x60, 1: 50x50, 2: 40x40 on a 100x100 sheet.
	p := newProblem(t, []part{{"1", 60, 60}, {"2", 50, 50}, {"3", 40, 40}}, sheet100, 0, 4, 1)
	// (0 1 S): 110x60 overflows -> 0 (larger) is a sheet, 1 continues;
	// (1 2 S): 90x50 fits -> second sheet.
	ind := csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumS, 2, csp2d.NumS})
	forest, err := p.GetForestForTest(ind)
	if err != nil {
		t.Fatal(err)
	}
	if len(forest) != 2 {
		t.Fatalf("got %d sheets, want 2: %+v", len(forest), forest)
	}
	if !equalInts(forest[0].Postfix, []int{0}) {
		t.Errorf("first sheet %v, want [0]", forest[0].Postfix)
	}
	if !equalInts(forest[1].Postfix, []int{1, 2, csp2d.NumS}) || forest[1].Long != 90 || forest[1].Short != 50 {
		t.Errorf("second sheet %+v, want postfix [1 2 S] 90x50", forest[1])
	}
}

// UT-FOR-04 / UT-FOR-05: decoder errors.
func TestForestErrors(t *testing.T) {
	p := newProblem(t, []part{{"1", 101, 10}, {"2", 10, 10}}, sheet100, 0, 4, 1)
	if _, err := p.GetForestForTest(csp2d.NewIndividualForTest([]int{0, 1, csp2d.NumL})); err == nil {
		t.Error("a part larger than the sheet must be reported")
	}
	q := newProblem(t, []part{{"1", 10, 10}, {"2", 10, 10}}, sheet100, 0, 4, 1)
	for _, bad := range [][]int{{0, csp2d.NumS, 1}, {0, 1}} {
		if _, err := q.GetForestForTest(csp2d.NewIndividualForTest(bad)); err == nil {
			t.Errorf("invalid postfix %v must be reported", bad)
		}
	}
}
