//go:build testhooks

// Test hooks: thin exported wrappers around unexported csp2d algorithms and
// state, so that tests living under test/ (outside this package) can reach
// them. This file is compiled ONLY with `-tags testhooks`; production builds
// never contain it. It must hold no logic beyond forwarding and conversion.

package csp2d

import (
	"csp2d/problem"
	"math/rand"
)

// ---- Problem construction --------------------------------------------------

// NewProblemForTest returns the concrete problem type (NewCsp2dProblem returns
// the problem.Problem interface).
func NewProblemForTest(inputs []InputDefinition, constraints *Constraints) *Csp2dProblemImpl {
	return &Csp2dProblemImpl{inputs: inputs, constraints: constraints,
		repairWeight: defaultRepairWeight, flipWeight: defaultFlipWeight, donorFraction: defaultDonorFraction}
}

// InitForTest prepares label ranks and the arena exactly as NewPopulation does,
// without creating a population. Required before calling any algorithm that
// allocates from the arena or computes protection keys.
func (p *Csp2dProblemImpl) InitForTest(populationSize, eliteCount int) {
	p.buildLabelRanks()
	p.arena = NewArena(len(p.inputs)*2-1, populationSize, eliteCount)
	if p.protectedGenotype == nil {
		p.protectedGenotype = make(map[string]struct{})
	}
}

func (p *Csp2dProblemImpl) ArenaForTest() *Arena             { return p.arena }
func (p *Csp2dProblemImpl) ConstraintsForTest() *Constraints { return p.constraints }
func (p *Csp2dProblemImpl) InputsForTest() []InputDefinition { return p.inputs }

// ---- Individuals -----------------------------------------------------------

func NewIndividualForTest(postfix []int) *Csp2dIndividualImpl {
	return newIndividual(postfix).(*Csp2dIndividualImpl)
}

// AsIndividualForTest converts a problem.Individual to the concrete type
// (returns nil for a nil interface).
func AsIndividualForTest(ind problem.Individual) *Csp2dIndividualImpl {
	if ind == nil {
		return nil
	}
	return ind.(*Csp2dIndividualImpl)
}

func (individual *Csp2dIndividualImpl) PostfixForTest() []int { return individual.postfix }
func (individual *Csp2dIndividualImpl) FitnessVectorForTest() [3]float64 {
	return individual.fitnessVector
}
func (individual *Csp2dIndividualImpl) FitnessValueForTest() float64 { return individual.fitnessValue }
func (individual *Csp2dIndividualImpl) ForestForTest() []NodeView {
	if individual.meteredForest == nil {
		return nil
	}
	return toViews(individual.meteredForest)
}
func (individual *Csp2dIndividualImpl) InvalidateCacheForTest() { individual.invalidateCache() }

// ---- Metered nodes ---------------------------------------------------------

// NodeView is an exported mirror of MeteredNode.
type NodeView struct {
	Postfix []int
	Long    float64
	Short   float64
}

func toViews(nodes []MeteredNode) []NodeView {
	views := make([]NodeView, len(nodes))
	for i, n := range nodes {
		views[i] = NodeView{Postfix: n.postfix, Long: n.measurements.longSide, Short: n.measurements.shortSide}
	}
	return views
}

func fromViews(views []NodeView) []MeteredNode {
	nodes := make([]MeteredNode, len(views))
	for i, v := range views {
		nodes[i] = MeteredNode{postfix: v.Postfix, measurements: Metrics{longSide: v.Long, shortSide: v.Short}}
	}
	return nodes
}

// ---- Geometry, decoding, fitness ------------------------------------------

func (p *Csp2dProblemImpl) ReduceForTest(op int, long1, short1, long2, short2, gap float64) (long, short float64) {
	m := p.reduce(op, Metrics{longSide: long1, shortSide: short1}, Metrics{longSide: long2, shortSide: short2}, gap)
	return m.longSide, m.shortSide
}

func (p *Csp2dProblemImpl) IsValidMetricsForTest(long, short float64) bool {
	return p.isValidMetrics(Metrics{longSide: long, shortSide: short}, p.constraints)
}

func (p *Csp2dProblemImpl) GetForestForTest(individual *Csp2dIndividualImpl) ([]NodeView, error) {
	forest, err := p.getForest(individual)
	if err != nil {
		return nil, err
	}
	return toViews(forest), nil
}

func (p *Csp2dProblemImpl) EvaluateFitnessForTest(individual *Csp2dIndividualImpl) error {
	return p.evaluateFitness(individual)
}

func (p *Csp2dProblemImpl) FitnessVectorForTest(forest []NodeView) [3]float64 {
	return p.getFitnessVector(fromViews(forest))
}

func (p *Csp2dProblemImpl) FitnessVectorWithReplacementForTest(forest []NodeView, index int, long, short float64) [3]float64 {
	return p.getFitnessVectorWithReplacement(fromViews(forest), index, Metrics{longSide: long, shortSide: short})
}

func (p *Csp2dProblemImpl) FitnessValueForTest(individual *Csp2dIndividualImpl) float64 {
	return p.getFitnessValue(individual, p.constraints)
}

func (p *Csp2dProblemImpl) ToStringForTest(postfix []int) string { return p.toString(postfix) }

// ---- Identity / protection -------------------------------------------------

func (p *Csp2dProblemImpl) NumLabelKeyForTest(postfix []int) string { return p.toNumLabelKey(postfix) }
func (p *Csp2dProblemImpl) IsProtectedForTest(postfix []int) bool   { return p.isProtected(postfix) }

// ---- Crossover -------------------------------------------------------------

// SubtreeCrossoverForTest wraps the children in individuals whose postfixes
// live in the crossover scratch: valid until the next crossover.
func (p *Csp2dProblemImpl) SubtreeCrossoverForTest(parent1, parent2 *Csp2dIndividualImpl, rnd *rand.Rand) (*Csp2dIndividualImpl, *Csp2dIndividualImpl, error) {
	c1, c2, err := p.subtreeCrossover(parent1, parent2, rnd)
	if err != nil {
		return nil, nil, err
	}
	return &Csp2dIndividualImpl{postfix: c1}, &Csp2dIndividualImpl{postfix: c2}, nil
}

// SubtreeCrossoverChildrenForTest returns the scratch-backed children as they
// are (no wrapping, so it allocates nothing itself).
func (p *Csp2dProblemImpl) SubtreeCrossoverChildrenForTest(parent1, parent2 *Csp2dIndividualImpl, rnd *rand.Rand) ([]int, []int, error) {
	return p.subtreeCrossover(parent1, parent2, rnd)
}

// PickDonorSubtreeForTest returns the span [start, end] of the picked subtree.
func (p *Csp2dProblemImpl) PickDonorSubtreeForTest(postfix []int, rnd *rand.Rand) (int, int) {
	return p.pickDonorSubtree(postfix, rnd)
}

// RemoveLeavesForTest returns a copy of the scratch-backed result.
func (p *Csp2dProblemImpl) RemoveLeavesForTest(postfix, segment []int) []int {
	return append([]int{}, p.removeLeaves(postfix, segment)...)
}

func GraftAtForTest(rest, segment []int, k, op int) []int {
	return graftAt(nil, rest, segment, k, op)
}

// ---- Mutation operators ----------------------------------------------------

func (p *Csp2dProblemImpl) SubtreeSpanForTest(postfix []int, end int) int {
	return p.subtreeSpan(postfix, end)
}

// ChangeOperatorForTest and ReplaceElementsForTest copy the new postfix out of
// the candidate scratch (as Mutation commits it), so it survives later calls.
func (p *Csp2dProblemImpl) ChangeOperatorForTest(individual *Csp2dIndividualImpl, rnd *rand.Rand) (int, error) {
	weight, err := p.changeOperator(individual, rnd)
	individual.postfix = append([]int(nil), individual.postfix...)
	return weight, err
}

func (p *Csp2dProblemImpl) ReplaceElementsForTest(individual *Csp2dIndividualImpl, rnd *rand.Rand) (int, error) {
	weight, err := p.replaceElements(individual, rnd)
	individual.postfix = append([]int(nil), individual.postfix...)
	return weight, err
}

// RankCriticalOperatorsForTest returns the scratch-backed result: valid until
// the next call.
func (p *Csp2dProblemImpl) RankCriticalOperatorsForTest(postfix []int) []OperatorCandidate {
	return p.rankCriticalOperators(postfix)
}

func (p *Csp2dProblemImpl) SelectWorstTreeForTest(forest []NodeView) int {
	return p.selectWorstTree(fromViews(forest))
}

// InsertItemGreedyForTest runs the in-place insertion on a copy of forest and
// returns the resulting forest.
func (p *Csp2dProblemImpl) InsertItemGreedyForTest(forest []NodeView, itemID int) ([]NodeView, bool) {
	nodes := fromViews(forest)
	ok := p.insertItemGreedyInPlace(nodes, itemID) >= 0
	return toViews(nodes), ok
}

// ReinsertItemsForTest returns a copy of the scratch-backed result, so it stays
// valid after the next call.
func (p *Csp2dProblemImpl) ReinsertItemsForTest(base []NodeView, order []int) ([]NodeView, error) {
	nodes, err := p.reinsertItemsForest(fromViews(base), order)
	if err != nil {
		return nil, err
	}
	return toViews(nodes), nil
}

// MutateEliminateAndRepairForTest and MutateCriticalOperatorFlipForTest keep an
// improved child exactly as Mutation does (without the protection check):
// committed into the generation arena, with its fitness cached.
func (p *Csp2dProblemImpl) MutateEliminateAndRepairForTest(parent *Csp2dIndividualImpl, rnd *rand.Rand) (*Csp2dIndividualImpl, bool, error) {
	child, improved, err := p.mutateEliminateAndRepair(parent, rnd)
	if err == nil && improved {
		p.commitRepairChild(child, nil, nil)
	}
	return child, improved, err
}

func (p *Csp2dProblemImpl) MutateCriticalOperatorFlipForTest(parent *Csp2dIndividualImpl, rnd *rand.Rand) (*Csp2dIndividualImpl, bool, error) {
	child, improved, err := p.mutateCriticalOperatorFlip(parent, rnd)
	if err == nil && improved {
		if err = p.cacheForest(child, nil); err == nil {
			child.postfix = p.commitPostfix(child.postfix, nil)
		}
	}
	return child, improved, err
}

func BuildReinsertionOrdersForTest(items []int, inputs []InputDefinition, rnd *rand.Rand) [][]int {
	return buildReinsertionOrders(items, inputs, rnd, make([][]int, 4))
}

// ---- Arena -----------------------------------------------------------------

// GenCapacitiesForTest returns the capacities (in ints) of the current and next
// generation arenas.
func (a *Arena) GenCapacitiesForTest() (current, next int) {
	return a.currentGen.Capacity(), a.nextGen.Capacity()
}

// ValuesForTest exposes the constraint values (maxShortSide < 0 means
// unconstrained).
func (c *Constraints) ValuesForTest() (maxLongSide, maxShortSide, sawWidth float64) {
	return c.maxLongSide, c.maxShortSide, c.sawWidth
}

// SetFitnessForTest sets the cached fitness directly (for ordering tests).
func (individual *Csp2dIndividualImpl) SetFitnessForTest(vector [3]float64, value float64) {
	individual.fitnessVector = vector
	individual.fitnessValue = value
}

// GenDemandForTest returns the ints requested from the next-generation arena
// since the last rotation (served plus overflowed).
func (a *Arena) GenDemandForTest() int { return a.nextGen.Demand() }

// EliteArenaForTest returns the elite arena that holds the current elites.
func (a *Arena) EliteArenaForTest() *PostfixArena { return a.eliteArena }

// BaseForTest identifies the arena's buffer (nil when it is empty).
func (a *PostfixArena) BaseForTest() *int {
	if len(a.buf) == 0 {
		return nil
	}
	return &a.buf[0]
}

// DecodeForestForTest decodes in scratch, as a critical-flip trial does, and
// returns the number of sheets.
func (p *Csp2dProblemImpl) DecodeForestForTest(postfix []int) (int, error) {
	forest, err := p.decodeForest(postfix)
	return len(forest), err
}

// SetSearchConstantsForTest sets the search constants of every problem
// (experiments).
func SetSearchConstantsForTest(retries int) {
	cloneRetries = retries
}

// SearchParametersForTest returns the problem's repair weight, flip weight and
// donor fraction (SetSearchParameters).
func (p *Csp2dProblemImpl) SearchParametersForTest() (repairWeight, flipWeight int, donorFraction float64) {
	return p.repairWeight, p.flipWeight, p.donorFraction
}

// DecodeFitnessForTest decodes in scratch, as a critical-flip trial does, and
// returns the fitness vector of the decoded forest.
func (p *Csp2dProblemImpl) DecodeFitnessForTest(postfix []int) ([3]float64, error) {
	forest, err := p.decodeForest(postfix)
	if err != nil {
		return [3]float64{}, err
	}
	return p.getFitnessVector(forest), nil
}
