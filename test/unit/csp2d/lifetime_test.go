//go:build testhooks

package csp2d_test

import (
	"csp2d/problem"
	"csp2d/problem/csp2d"
	"csp2d/test/support/testutil"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
)

// epochDriver reproduces the per-epoch sequence of ga.Simulate (ga/ga.go)
// step by step, so the state can be inspected after every epoch.
type epochDriver struct {
	p          *csp2d.Csp2dProblemImpl
	popSize    int
	eliteCount int
	rate       float64
	rnd        *rand.Rand
}

func (d *epochDriver) init() error {
	return d.p.NewPopulation(d.popSize, d.eliteCount, d.rnd)
}

func (d *epochDriver) epoch() error {
	population := d.p.GetPopulation()
	if err := d.p.ComputeFitness(); err != nil {
		return err
	}
	d.p.SortSpecies()
	species := *population.Species

	elites := append([]problem.Individual(nil), species[:d.eliteCount]...)
	d.p.PromoteElites(d.eliteCount)

	crossoverSet := make([]*problem.Individual, 0, d.popSize-d.eliteCount)
	for len(crossoverSet) < d.popSize-d.eliteCount {
		best := d.rnd.Intn(d.popSize)
		for k := 1; k < 3; k++ {
			if c := d.rnd.Intn(d.popSize); c < best {
				best = c
			}
		}
		crossoverSet = append(crossoverSet, &species[best])
	}
	population.Selected = elites
	d.p.Protect(elites)

	next, _, err := d.p.Crossover(crossoverSet, d.rnd)
	if err != nil {
		return err
	}
	if _, _, _, err := d.p.Mutation(d.rate, next, d.rnd); err != nil {
		return err
	}
	for i := d.eliteCount; i < d.popSize; i++ {
		if i-d.eliteCount < len(next) {
			species[i] = *next[i-d.eliteCount]
			continue
		}
		ind, err := d.p.NewRandomIndividual(d.rnd)
		if err != nil {
			return err
		}
		species[i] = ind
	}
	d.p.RotateGenerations()
	return nil
}

// lexLess compares fitness vectors lexicographically.
func lexLess(a, b [3]float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// checkIndividual validates the genotype and, when fitness is cached, the
// cached phenotype against a fresh decode of the genotype.
func checkIndividual(p *csp2d.Csp2dProblemImpl, ind *csp2d.Csp2dIndividualImpl, n int, strict bool) error {
	if ind == nil {
		return fmt.Errorf("nil individual")
	}
	postfix := ind.PostfixForTest()
	if err := validatePostfix(postfix, n); err != nil {
		return fmt.Errorf("genotype corrupted: %w (postfix %v)", err, postfix)
	}
	forest := ind.ForestForTest()
	if forest == nil {
		return nil // fitness not computed yet
	}
	// The cached phenotype must itself be a valid layout of all parts.
	seen := make([]int, n)
	for i, tree := range forest {
		if !p.IsValidMetricsForTest(tree.Long, tree.Short) {
			return fmt.Errorf("cached sheet %d (%gx%g) does not fit", i, tree.Long, tree.Short)
		}
		for _, v := range tree.Postfix {
			if v >= 0 {
				seen[v]++
			}
		}
	}
	for id, c := range seen {
		if c != 1 {
			return fmt.Errorf("cached forest uses part %d %d times", id, c)
		}
	}
	// Fresh decode of the genotype.
	fresh := csp2d.NewIndividualForTest(copyInts(postfix))
	if err := p.EvaluateFitnessForTest(fresh); err != nil {
		return fmt.Errorf("genotype does not decode: %w", err)
	}
	cached, decoded := ind.FitnessVectorForTest(), fresh.FitnessVectorForTest()
	if strict && cached != decoded {
		return fmt.Errorf("cached fitness %v != decoded fitness %v", cached, decoded)
	}
	if lexLess(cached, decoded) {
		return fmt.Errorf("cached fitness %v is better than the genotype's decoded fitness %v", cached, decoded)
	}
	return nil
}

type lifetimeInstance struct {
	name  string
	parts func(rnd *rand.Rand) []part
	kerf  float64
}

var lifetimeInstances = []lifetimeInstance{
	{"12 distinct", func(rnd *rand.Rand) []part { return distinctParts(12, rnd) }, 0},
	{"20 distinct, kerf", func(rnd *rand.Rand) []part { return distinctParts(20, rnd) }, 1},
	{"16 with repeated labels", func(rnd *rand.Rand) []part {
		parts := make([]part, 16)
		for i := range parts {
			k := i % 4
			parts[i] = part{strconv.Itoa(k + 1), float64(20 + 10*k), float64(30 + 5*k)}
		}
		return parts
	}, 0.5},
}

func runLifetime(t *testing.T, strict bool) {
	const epochs, pop, elite = 30, 40, 5
	for _, inst := range lifetimeInstances {
		for _, seed := range []int64{1, 2, 3} {
			t.Run(fmt.Sprintf("%s/seed=%d", inst.name, seed), func(t *testing.T) {
				rnd := testutil.Rand(t, seed)
				parts := inst.parts(rnd)
				p := newProblem(t, parts, sheet100, inst.kerf, pop, elite)
				d := &epochDriver{p: p, popSize: pop, eliteCount: elite, rate: 0.07, rnd: rnd}
				if err := d.init(); err != nil {
					t.Fatal(err)
				}
				for e := 1; e <= epochs; e++ {
					if err := d.epoch(); err != nil {
						t.Fatalf("epoch %d: %v", e, err)
					}
					for i, ind := range *p.GetPopulation().Species {
						if err := checkIndividual(p, csp2d.AsIndividualForTest(ind), len(parts), strict); err != nil {
							t.Fatalf("epoch %d, individual %d: %v", e, i, err)
						}
					}
				}
			})
		}
	}
}

// UT-LIFE-01: across many epochs every genotype stays intact (no
// use-after-reset / aliasing in the arenas) and every cached phenotype is a
// valid layout not worse than its own genotype's decode.
func TestLifetimeIntegrity(t *testing.T) {
	runLifetime(t, false)
}

// UT-LIFE-01 (strict) / UT-MUT-09: the cached fitness must equal the decoded
// fitness of the genotype. Eliminate-and-repair children currently cache the
// fitness of the repaired forest, not of their postfix.
func TestLifetimeCachedFitnessEqualsDecode(t *testing.T) {
	testutil.Pending(t, "eliminate-and-repair caches a fitness that differs from its genotype's decode")
	runLifetime(t, true)
}

// UT-LIFE-02: elite genotypes survive arena rotations unchanged.
func TestElitesSurviveRotation(t *testing.T) {
	rnd := testutil.Rand(t, 7)
	p := newProblem(t, distinctParts(15, rnd), sheet100, 0, 30, 4)
	d := &epochDriver{p: p, popSize: 30, eliteCount: 4, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	if err := d.epoch(); err != nil {
		t.Fatal(err)
	}
	elites := p.GetPopulation().Selected
	snapshots := make([][]int, len(elites))
	for i, e := range elites {
		snapshots[i] = copyInts(csp2d.AsIndividualForTest(e).PostfixForTest())
	}
	// Rotate the generation arenas three times without promoting again: the
	// elite postfixes live in the elite arena and must not be touched.
	for r := 0; r < 3; r++ {
		p.ArenaForTest().GenAllocN(1000) // dirty the next generation arena
		p.RotateGenerations()
	}
	for i, e := range elites {
		if got := csp2d.AsIndividualForTest(e).PostfixForTest(); !equalInts(got, snapshots[i]) {
			t.Fatalf("elite %d changed after rotations: %v -> %v", i, snapshots[i], got)
		}
	}
}

// UT-LIFE-03: a response serialized at epoch e is reproduced after one more
// generation rotation (elites live in the elite arena).
func TestResponseStableAcrossRotation(t *testing.T) {
	rnd := testutil.Rand(t, 8)
	p := newProblem(t, distinctParts(15, rnd), sheet100, 0, 30, 4)
	d := &epochDriver{p: p, popSize: 30, eliteCount: 4, rate: 0.07, rnd: rnd}
	if err := d.init(); err != nil {
		t.Fatal(err)
	}
	for e := 0; e < 3; e++ {
		if err := d.epoch(); err != nil {
			t.Fatal(err)
		}
	}
	first, err := csp2d.NewResponse(p)
	if err != nil {
		t.Fatal(err)
	}
	p.ArenaForTest().GenAllocN(500) // dirty the next generation arena
	p.RotateGenerations()
	second, err := csp2d.NewResponse(p)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first) != fmt.Sprint(second) {
		t.Fatalf("response changed after rotation:\n%v\n%v", first, second)
	}
}
