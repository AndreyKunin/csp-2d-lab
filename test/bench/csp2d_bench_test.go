//go:build testhooks

// Benchmarks for the csp2d hot paths (decoding, PMX, random genotypes, the
// protection lookup). Build with -tags testhooks:
//
//	go test -tags testhooks -run=^$ -bench=. -benchmem ./test/bench/
package bench_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
)

func benchProblem(b *testing.B, n, pop, elite int) (*csp2d.Csp2dProblemImpl, *rand.Rand) {
	b.Helper()
	rnd := rand.New(rand.NewSource(42))
	inputs := make([]csp2d.InputDefinition, n)
	for i := range inputs {
		w, l := float64(5+rnd.Intn(60)), float64(5+rnd.Intn(110))
		if w > l {
			w, l = l, w
		}
		inputs[i] = csp2d.InputDefinition{Id: i, Label: strconv.Itoa(i%25 + 1), LongSide: l, ShortSide: w}
		// Labels repeat every 25 parts: keep dimensions consistent per label.
		if i >= 25 {
			inputs[i].LongSide, inputs[i].ShortSide = inputs[i%25].LongSide, inputs[i%25].ShortSide
		}
	}
	p := csp2d.NewProblemForTest(inputs, csp2d.NewConstraints(244, 122, 0.3))
	p.InitForTest(pop, elite)
	return p, rnd
}

var sizes = []int{20, 100, 250}

// BM-01: decoding a genotype into sheets.
func BenchmarkGetForest(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			p, rnd := benchProblem(b, n, 64, 4)
			ind, err := p.NewRandomIndividual(rnd)
			if err != nil || ind == nil {
				b.Fatal("cannot create individual", err)
			}
			c := csp2d.AsIndividualForTest(ind)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := p.GetForestForTest(c); err != nil {
					b.Fatal(err)
				}
				if i%1000 == 999 {
					p.RotateGenerations() // keep the arena from overflowing
					p.RotateGenerations()
				}
			}
		})
	}
}

// BM-02: PMX crossover.
func BenchmarkPMX(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			p, rnd := benchProblem(b, n, 64, 4)
			a, _ := p.NewRandomIndividual(rnd)
			c, _ := p.NewRandomIndividual(rnd)
			pa, pc := csp2d.AsIndividualForTest(a), csp2d.AsIndividualForTest(c)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, _, _, err := p.PMXForTest(pa, pc, rnd); err != nil {
					b.Fatal(err)
				}
				if i%500 == 499 {
					p.RotateGenerations()
					p.RotateGenerations()
				}
			}
		})
	}
}

// BM-03: random genotype construction.
func BenchmarkNewRandomIndividual(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			p, rnd := benchProblem(b, n, 64, 4)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := p.NewRandomIndividual(rnd); err != nil {
					b.Fatal(err)
				}
				if i%1000 == 999 {
					p.RotateGenerations()
					p.RotateGenerations()
				}
			}
		})
	}
}

// BM-05: the protection lookup (allocs/op is the interesting number).
func BenchmarkIsProtected(b *testing.B) {
	p, rnd := benchProblem(b, 100, 64, 4)
	elites := make([]problem.Individual, 4)
	for i := range elites {
		elites[i], _ = p.NewRandomIndividual(rnd)
	}
	p.Protect(elites)
	probe, _ := p.NewRandomIndividual(rnd)
	postfix := csp2d.AsIndividualForTest(probe).PostfixForTest()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.IsProtectedForTest(postfix)
	}
}
