//go:build testhooks

package ga_test

import (
	"csp2d/ga"
	"csp2d/problem/csp2d"
	"csp2d/test/support/fakes"
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// UT-GA-MGR-13: the statistics returned by GetSimulationStatistics are a
// snapshot. The job goroutine keeps running after the call, but what the caller
// holds (and an HTTP handler is still JSON-encoding) never changes. A live
// pointer here is a data race between the handler and the job goroutine.
func TestManagerStatisticsAreASnapshot(t *testing.T) {
	m := ga.NewSimulationManager()
	st, err := m.StartSimulation(ga.NewSimulation(params(6, 2, 100000000, 1000)),
		&fakes.Problem{InputsSize: 3, EpochDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.DeleteSimulation(st.ID)

	epochOf := func(s ga.SimulationJobStatus) int {
		if s.Statistics == nil || s.Statistics.State == nil {
			return -1
		}
		return s.Statistics.State.CurrentEpoch.EpochNumber
	}

	// Wait until the first epoch is published.
	var first ga.SimulationJobStatus
	deadline := time.Now().Add(5 * time.Second)
	for {
		first, err = m.GetSimulationStatistics(st.ID)
		if err != nil {
			t.Fatal(err)
		}
		if epochOf(first) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no epoch completed within 5s")
		}
		time.Sleep(5 * time.Millisecond)
	}
	held := epochOf(first)
	heldParams := *first.Statistics.Parameters

	// Let the job advance, then check what the first caller holds.
	for {
		now, err := m.GetSimulationStatistics(st.ID)
		if err != nil {
			t.Fatal(err)
		}
		if epochOf(now) > held+5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the job did not advance")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := epochOf(first); got != held {
		t.Fatalf("the returned statistics changed after the call: epoch %d, then %d", held, got)
	}
	if *first.Statistics.Parameters != heldParams {
		t.Fatalf("the returned parameters changed after the call")
	}
}

// UT-GA-MGR-14: GetSimulationResult returns the result snapshot published by
// the job goroutine (a *csp2d.Response for the CSP-2D problem). It is a
// self-contained value: further epochs do not change what a caller holds.
func TestManagerResultIsASnapshot(t *testing.T) {
	sizes := [][2]float64{{60, 40}, {60, 40}, {30, 70}, {30, 70}, {45, 45}, {45, 45}, {80, 20}, {25, 25}}
	inputs := make([]csp2d.InputDefinition, len(sizes))
	for i, sz := range sizes {
		label := i/2 + 1
		long, short := max(sz[0], sz[1]), min(sz[0], sz[1])
		inputs[i] = csp2d.InputDefinition{Id: i, Label: strconv.Itoa(label), IntLabel: label, LongSide: long, ShortSide: short}
	}
	m := ga.NewSimulationManager()
	st, err := m.StartSimulation(ga.NewSimulation(params(30, 4, 100000000, 1000)),
		csp2d.NewCsp2dProblem(inputs, csp2d.NewConstraints(100, 100, 1)))
	if err != nil {
		t.Fatal(err)
	}
	defer m.DeleteSimulation(st.ID)

	epoch := func() int {
		s, err := m.GetSimulationStatistics(st.ID)
		if err != nil {
			t.Fatal(err)
		}
		if s.Statistics.State == nil {
			return 0
		}
		return s.Statistics.State.CurrentEpoch.EpochNumber
	}
	deadline := time.Now().Add(5 * time.Second)
	for epoch() < 1 {
		if time.Now().After(deadline) {
			js, _ := m.GetSimulationStatus(st.ID)
			t.Fatalf("no epoch completed within 5s: %s %v", js.Status, js.Error)
		}
		time.Sleep(5 * time.Millisecond)
	}
	snapshot, err := m.GetSimulationResult(st.ID)
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := snapshot.(*csp2d.Response)
	if !ok || len(resp.Solutions) != 4 {
		t.Fatalf("want a *csp2d.Response with 4 solutions, got %#v", snapshot)
	}
	before, _ := json.Marshal(resp)
	held := epoch()
	for epoch() < held+20 {
		if time.Now().After(deadline) {
			t.Fatal("the job did not advance")
		}
		time.Sleep(5 * time.Millisecond)
	}
	after, _ := json.Marshal(resp)
	if string(before) != string(after) {
		t.Fatalf("the returned result changed after the call:\n%s\n%s", before, after)
	}
}
