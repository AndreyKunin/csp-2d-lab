//go:build testhooks

package bench_test

import (
	"csp2d/ga"
	"csp2d/problem/csp2d"
	"csp2d/util"
	"fmt"
	"io"
	"math/rand"
	"strconv"
	"testing"
)

// BM-06: one whole GA epoch on a CSP-2D problem (population 100, 10 elites),
// including whatever the job goroutine publishes for HTTP readers. Simulate may
// stop early on convergence, so the metric is ns per epoch actually run.
func BenchmarkEpoch(b *testing.B) {
	// Arena overflow warnings and the epochs/s line would interleave with the results.
	util.Info.SetOutput(io.Discard)
	util.Warning.SetOutput(io.Discard)
	for _, n := range []int{20, 60} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			rnd := rand.New(rand.NewSource(42))
			inputs := make([]csp2d.InputDefinition, n)
			for i := range inputs {
				w, l := float64(5+rnd.Intn(60)), float64(5+rnd.Intn(110))
				if w > l {
					w, l = l, w
				}
				inputs[i] = csp2d.InputDefinition{Id: i, Label: strconv.Itoa(i + 1), LongSide: l, ShortSide: w}
			}
			epochs := 0
			b.ResetTimer()
			for epochs < b.N {
				params := &ga.Parameters{
					PopulationSize: 100, EliteCount: 10, MutationRate: 0.07,
					MaxEpochsCount: b.N - epochs, WindowSize: 50,
				}
				sim := ga.NewSimulation(params)
				job := ga.NewSimulationJob("bench", sim, csp2d.NewCsp2dProblem(inputs, csp2d.NewConstraints(244, 122, 0.3)))
				if err := ga.Simulate(job); err != nil {
					b.Fatal(err)
				}
				epochs += sim.State.CurrentEpoch.EpochNumber
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(epochs), "ns/epoch")
		})
	}
}
