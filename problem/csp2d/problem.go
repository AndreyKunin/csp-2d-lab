package csp2d

import (
	"csp2d/problem"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"sort"
	"strings"
	"unsafe"
)

const MaxIndividualCreationRetries = 128

type Csp2dProblem interface {
	problem.Problem
	toString(postfixNotation []int) string
}

type Csp2dProblemImpl struct {
	// Search parameters (SetSearchParameters, mutation.go).
	repairWeight, flipWeight int
	donorFraction            float64
	inputs                   []InputDefinition
	constraints              *Constraints
	population               *problem.Population
	// protectedGenotype keys are compact byte-strings produced from the
	// numeric-label representation of a postfix (see toNumLabelKey).
	protectedGenotype map[string]struct{}
	arena             *Arena
	eliteCount        int // remembered to size the elite arena on rotation

	// labelRank maps an input id to a small positive int32 rank that uniquely
	// identifies the input's Label. Two inputs sharing the same Label share
	// the same rank — this matches the original toString-based equivalence.
	// Ranks start at 1; 0 is reserved.
	labelRank []int32
}

func NewCsp2dProblem(inputs []InputDefinition, constraints *Constraints) problem.Problem {
	return &Csp2dProblemImpl{
		inputs:        inputs,
		constraints:   constraints,
		repairWeight:  defaultRepairWeight,
		flipWeight:    defaultFlipWeight,
		donorFraction: defaultDonorFraction,
	}
}

// Release implements ga.Releaser. The job calls it once its goroutine has
// ended: the arenas go back to the process-wide arena budget, and the
// population and protection set become garbage. Statistics and results were
// published as copies, so nothing reads them any more; GetInputsSize keeps
// working.
func (p *Csp2dProblemImpl) Release() {
	if p.arena != nil {
		p.arena.Release()
		p.arena = nil
	}
	p.population = nil
	p.protectedGenotype = nil
}

func (p *Csp2dProblemImpl) GetInputsSize() int {
	return len(p.inputs)
}

func (p *Csp2dProblemImpl) GetPopulation() *problem.Population {
	return p.population
}

func (p *Csp2dProblemImpl) Protect(individuals []problem.Individual) {
	var size = len(individuals)
	p.protectedGenotype = make(map[string]struct{}, size)
	for i := 0; i < size; i += 1 {
		ind, _ := individuals[i].(*Csp2dIndividualImpl)
		genotype := p.toNumLabelKey(ind.postfix)
		p.protectedGenotype[genotype] = struct{}{}
	}
}

func (p *Csp2dProblemImpl) isProtected(postfix []int) bool {
	postfixKey := p.toNumLabelKey(postfix)
	_, exists := p.protectedGenotype[postfixKey]
	return exists
}

func (p *Csp2dProblemImpl) SetPopulation(population *problem.Population) {
	p.population = population
}

func (p *Csp2dProblemImpl) NewPopulation(size int, eliteCount int, rnd *rand.Rand) error {
	species := make([]problem.Individual, 0, size)
	p.protectedGenotype = make(map[string]struct{})

	postfixLen := len(p.inputs)*2 - 1
	p.buildLabelRanks()
	if p.arena != nil {
		p.arena.Release() // a previous population's arena goes back to the budget
	}
	p.arena = NewArena(postfixLen, size, eliteCount)

	for len(species) < size {
		ind, err := p.NewRandomIndividual(rnd)
		if err != nil {
			return err
		}
		if ind == nil { // cannot create any more individuals, the gene pool is full
			return fmt.Errorf("unable to prepare a population with size %d. All possible combinations exhausted", size)
		}
		cspInd, _ := ind.(*Csp2dIndividualImpl)
		postfixKey := p.toNumLabelKey(cspInd.postfix)
		p.protectedGenotype[postfixKey] = struct{}{}
		species = append(species, ind)
	}
	p.population = &problem.Population{
		Species: &species,
	}
	p.protectedGenotype = make(map[string]struct{})
	return nil
}

// NewRandomIndividual
// pre-condition: ids must be all integers in range [0, idsSize)
func (p *Csp2dProblemImpl) NewRandomIndividual(rnd *rand.Rand) (problem.Individual, error) {
	idsSize := p.GetInputsSize()
	postfixLen := idsSize*2 - 1 // the number of Ops is always idsSize - 1 for the binary tree

	for j := 0; j < MaxIndividualCreationRetries; j++ {
		shuffledPostfix := make([]int, postfixLen)
		for i := 0; i < idsSize; i++ {
			shuffledPostfix[i] = i
		}
		for i := idsSize; i < postfixLen; i++ {
			shuffledPostfix[i] = opInfoByOp[rnd.Intn(OpsCount)].Num
		}
		rnd.Shuffle(postfixLen, func(i, j int) {
			shuffledPostfix[i], shuffledPostfix[j] = shuffledPostfix[j], shuffledPostfix[i]
		})

		// constructing a valid postfix notation directly from
		// a pre-randomized slice of operands (>=0) and operators (<0),

		// Each entry in completeStack is a postfix fragment (slice of ints).
		completeStack := make([][]int, 0, postfixLen)
		incompleteStack := make([]int, 0, postfixLen)

		for _, element := range shuffledPostfix {
			if element < 0 { // operator
				incompleteStack = append(incompleteStack, element)
			} else { // operand
				completeStack = append(completeStack, []int{element})
			}
			for len(completeStack) > 1 && len(incompleteStack) > 0 {
				op := incompleteStack[len(incompleteStack)-1]
				incompleteStack = incompleteStack[:len(incompleteStack)-1]

				right := completeStack[len(completeStack)-1]
				left := completeStack[len(completeStack)-2]
				completeStack = completeStack[:len(completeStack)-2]

				// postfix: left right op
				merged := make([]int, 0, len(left)+len(right)+1)
				merged = append(merged, left...)
				merged = append(merged, right...)
				merged = append(merged, op)
				completeStack = append(completeStack, merged)
			}
		}
		if len(incompleteStack) > 0 || len(completeStack) != 1 {
			return nil, errors.New("invalid elements: unable to build a postfix notation")
		}
		// Protected attempts are rejected before they take arena space (MEM-02).
		if p.isProtected(completeStack[0]) {
			continue
		}
		return newIndividual(p.arena.GenAlloc(completeStack[0])), nil
	}
	// cannot create even a random individual, the gene pool is full
	return nil, nil
}

func (p *Csp2dProblemImpl) ComputeFitness() error {
	for _, ind := range *p.population.Species {
		individual, _ := ind.(*Csp2dIndividualImpl)
		err := p.evaluateFitness(individual)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Csp2dProblemImpl) evaluateFitness(individual *Csp2dIndividualImpl) error {
	if individual.IsFitnessComputed() {
		return nil
	}
	return p.cacheForest(individual, nil)
}

// cacheForest decodes the individual and caches its forest and fitness. The
// forest's node slice reuses `reuse` when it is large enough; reuse must be
// referenced by nothing else (the forest of a superseded child, MEM-04).
func (p *Csp2dProblemImpl) cacheForest(individual *Csp2dIndividualImpl, reuse []MeteredNode) error {
	var forest, err = p.getForestInto(individual, reuse)
	if err != nil {
		return err
	}
	individual.meteredForest = forest
	individual.fitnessVector = p.getFitnessVector(forest)
	individual.fitnessValue = p.getFitnessValue(individual, p.constraints)

	return nil
}

// getForest decodes the individual and copies the sheets' postfixes into the
// generation arena, so the forest can be cached on the individual.
func (p *Csp2dProblemImpl) getForest(individual *Csp2dIndividualImpl) ([]MeteredNode, error) {
	return p.getForestInto(individual, nil)
}

// getForestInto is getForest with the node slice taken from reuse when it is
// large enough.
func (p *Csp2dProblemImpl) getForestInto(individual *Csp2dIndividualImpl, reuse []MeteredNode) ([]MeteredNode, error) {
	forest, err := p.decodeForest(individual.postfix)
	if err != nil {
		return nil, err
	}
	// Copy surviving nodes out of the pool so the pool can be reused.
	// Postfix slices are allocated from the generational arena: they live
	// for at most one epoch unless the individual becomes an elite.
	var result []MeteredNode
	if cap(reuse) >= len(forest) {
		result = reuse[:len(forest)]
	} else {
		result = make([]MeteredNode, len(forest))
	}
	for i, node := range forest {
		copied := node
		copied.postfix = p.arena.GenAlloc(node.postfix)
		result[i] = copied
	}
	return result, nil
}

// decodeForest decodes a postfix into its sheets without taking arena space:
// the returned slice and the sheets' postfixes live in scratch buffers and are
// valid only until the next decode. Use it for trials that are usually
// discarded.
func (p *Csp2dProblemImpl) decodeForest(postfix []int) ([]MeteredNode, error) {
	// Borrow from pools
	nodePool := p.arena.GetForestBuffer()
	buf := p.arena.GetForestSliceBuffer()

	nodeIdx := 0
	bufOK := true
	stack := p.arena.decodeStack[:0]
	forest := p.arena.decodeSheets[:0]

	for _, id := range postfix {
		if id < 0 {
			if len(stack) < 2 {
				return nil, fmt.Errorf("invalid postfix: %s", p.toString(postfix))
			}
			right := stack[len(stack)-1]
			left := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			metrics := p.reduce(id, left.measurements, right.measurements, p.constraints.sawWidth)

			mergedLen := len(left.postfix) + len(right.postfix) + 1
			var merged []int
			if bufOK && len(buf)+mergedLen <= cap(buf) {
				start := len(buf)
				buf = append(buf, left.postfix...)
				buf = append(buf, right.postfix...)
				buf = append(buf, id)
				merged = buf[start : start+mergedLen : start+mergedLen]
			} else { // last resort
				bufOK = false
				merged = make([]int, 0, mergedLen)
				merged = append(merged, left.postfix...)
				merged = append(merged, right.postfix...)
				merged = append(merged, id)
			}

			element := nodePool[nodeIdx]
			nodeIdx++
			element.postfix = merged
			element.measurements = metrics

			if p.isValidMetrics(metrics, p.constraints) {
				stack = append(stack, element)
			} else {
				var smallNode, largeNode MeteredNode
				if left.measurements.GetArea() > right.measurements.GetArea() {
					largeNode = left
					smallNode = right
				} else {
					largeNode = right
					smallNode = left
				}
				err := p.validateMetered(largeNode, p.constraints)
				if err != nil {
					return nil, err
				}
				forest = append(forest, largeNode)
				stack = append(stack, smallNode)
			}
		} else {
			element := nodePool[nodeIdx]
			nodeIdx++
			if bufOK && len(buf)+1 <= cap(buf) {
				start := len(buf)
				buf = append(buf, id)
				element.postfix = buf[start : start+1 : start+1]
			} else { // last resort
				bufOK = false
				element.postfix = []int{id}
			}
			element.measurements = Metrics{
				longSide:  p.inputs[id].LongSide,
				shortSide: p.inputs[id].ShortSide,
			}
			stack = append(stack, element)
		}
	}

	if len(stack) != 1 {
		return nil, fmt.Errorf("invalid postfix: %s", p.toString(postfix))
	}
	err := p.validateMetered(stack[0], p.constraints)
	if err != nil {
		return nil, err
	}
	forest = append(forest, stack[0])
	return forest, nil
}

func (p *Csp2dProblemImpl) validateMetered(node MeteredNode, constraints *Constraints) error {
	if len(node.postfix) == 1 && node.postfix[0] >= 0 && !p.isValidMetrics(node.measurements, constraints) {
		return errors.New(fmt.Sprintf("The element: (%g, %g) does not fit the area constraints (%g, %g).",
			node.measurements.longSide,
			node.measurements.shortSide,
			constraints.maxLongSide,
			constraints.maxShortSide,
		))
	}
	return nil
}

func (p *Csp2dProblemImpl) toString(postfixNotation []int) string {
	var result []string
	for _, element := range postfixNotation {
		if element < 0 {
			result = append(result, OpByNum(element).Str)
		} else {
			result = append(result, p.inputs[element].Label)
		}
	}
	return strings.Join(result, " ")
}

// buildLabelRanks computes a stable mapping from input id to a small positive
// int32 rank. Inputs with identical Labels share the same rank, mirroring the
// equivalence the previous toString-based key relied on.
func (p *Csp2dProblemImpl) buildLabelRanks() {
	p.labelRank = make([]int32, len(p.inputs))
	byLabel := make(map[string]int32, len(p.inputs))
	var next int32 = 1
	for i, in := range p.inputs {
		rank, ok := byLabel[in.Label]
		if !ok {
			rank = next
			byLabel[in.Label] = rank
			next++
		}
		p.labelRank[i] = rank
	}
}

// toNumLabelKey produces a string suitable for use as a map key. The string's
// bytes are the little-endian-by-architecture in-memory representation of the
// int32 ranks — its semantic meaning is opaque, but two postfixes produce the
// same key if their numeric-label projections are equal.
func (p *Csp2dProblemImpl) toNumLabelKey(postfix []int) string {
	nums := p.arena.GetNumLabelScratch(len(postfix))
	for _, element := range postfix {
		if element < 0 {
			nums = append(nums, int32(element))
		} else {
			nums = append(nums, p.labelRank[element])
		}
	}
	// View the int32 slice as a byte slice (no copy), then convert to string
	// (one allocation + copy — the resulting string is independent of nums).
	byteLen := len(nums) * 4
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&nums[0])), byteLen)
	return string(bytes)
}

func (p *Csp2dProblemImpl) reduce(id int, m1 Metrics, m2 Metrics, gap float64) Metrics {
	s1 := m1.shortSide
	s2 := m2.shortSide
	l1 := m1.longSide
	l2 := m2.longSide
	var newS, newL float64
	if id == NumS {
		if s1 > l2 {
			s2, l2 = l2, s2
		} else if s2 > l1 {
			s1, l1 = l1, s1
		}
		newL = l1 + l2 + gap
		if s1 > s2 {
			newS = s1
		} else {
			newS = s2
		}
	} else {
		if l1 > l2 {
			newL = l1
		} else {
			newL = l2
		}
		newS = s1 + s2 + gap
	}
	if newL >= newS {
		return Metrics{longSide: newL, shortSide: newS}
	}
	return Metrics{longSide: newS, shortSide: newL}
}

func (p *Csp2dProblemImpl) isValidMetrics(m Metrics, c *Constraints) bool {
	if c.maxShortSide < 0 { // means no area constraints
		return true
	}
	isOverflowing := m.longSide > c.maxLongSide || m.longSide > c.maxShortSide && m.shortSide > c.maxShortSide
	return !isOverflowing
}

func (p *Csp2dProblemImpl) getFitnessVector(forest []MeteredNode) [3]float64 {
	var area float64 = 0
	for _, t := range forest {
		area += t.measurements.GetArea()
	}
	return [3]float64{
		float64(len(forest)),
		area,
		1.0, //TODO
	}
}

// getFitnessVectorWithReplacement computes fitness as if forest[index] had the given measurements.
// NO allocations — just arithmetic.
func (p *Csp2dProblemImpl) getFitnessVectorWithReplacement(
	forest []MeteredNode,
	replaceIndex int,
	replacement Metrics,
) [3]float64 {
	var area float64
	for i, t := range forest {
		if i == replaceIndex {
			area += replacement.GetArea()
		} else {
			area += t.measurements.GetArea()
		}
	}
	return [3]float64{
		float64(len(forest)),
		area,
		1.0,
	}
}

func (p *Csp2dProblemImpl) getFitnessValue(individual *Csp2dIndividualImpl, c *Constraints) float64 {
	//The first two metrics can be converted to areas
	//The third metric: TODO
	if c.maxShortSide < 0 { // means no area constraints
		return individual.fitnessVector[1]
	}

	return individual.fitnessVector[0]*c.maxShortSide*c.maxLongSide + individual.fitnessVector[1]
}

func (p *Csp2dProblemImpl) SortSpecies() {
	sort.Sort(LexicographicalSort(*p.population.Species))
}

// PromoteElites copies the elite individuals' postfix data into a fresh
// elite arena. Must be called AFTER SortSpecies and BEFORE RotateGenerations.
//
// It does NOT reset the generational arenas — that happens in RotateGenerations
// after the epoch's crossover/mutation has finished reading parent postfixes.
func (p *Csp2dProblemImpl) PromoteElites(eliteCount int) {
	if p.arena == nil || p.population == nil {
		return
	}
	species := *p.population.Species
	if eliteCount > len(species) {
		eliteCount = len(species)
	}
	p.eliteCount = eliteCount

	// Reuse the elite buffer of two promotions ago, unless an individual of
	// the population still points into it (MEM-03).
	p.arena.NextEliteArena(eliteCount, p.populationReferences)

	for i := 0; i < eliteCount; i++ {
		ind, _ := species[i].(*Csp2dIndividualImpl)
		if ind == nil {
			continue
		}
		ind.postfix = p.arena.EliteAlloc(ind.postfix)
		for k := range ind.meteredForest {
			ind.meteredForest[k].postfix = p.arena.EliteAlloc(ind.meteredForest[k].postfix)
		}
	}
}

// populationReferences reports whether any individual of the population (its
// postfix or a cached sheet) points into the given arena.
func (p *Csp2dProblemImpl) populationReferences(arena *PostfixArena) bool {
	for _, ind := range *p.population.Species {
		individual, _ := ind.(*Csp2dIndividualImpl)
		if individual == nil {
			continue
		}
		if arena.Contains(individual.postfix) {
			return true
		}
		for _, tree := range individual.meteredForest {
			if arena.Contains(tree.postfix) {
				return true
			}
		}
	}
	return false
}

// RotateGenerations swaps the generational buffers so the just-written
// offspring become the next epoch's parents, and the previous parents' memory
// is freed in O(1). Call this at the end of each epoch.
func (p *Csp2dProblemImpl) RotateGenerations() {
	if p.arena == nil {
		return
	}
	p.arena.RotateGenerations()
}

func (p *Csp2dProblemImpl) Crossover(crossoverSet []*problem.Individual, rnd *rand.Rand) ([]*problem.Individual, int, error) {
	var size = len(crossoverSet)
	var failures = 0
	var offspringSet = make([]*problem.Individual, 0, size)
	rnd.Shuffle(size, func(i, j int) { crossoverSet[i], crossoverSet[j] = crossoverSet[j], crossoverSet[i] })
	for i := 0; i < size-1; i += 2 {
		parent1, _ := (*crossoverSet[i]).(*Csp2dIndividualImpl)
		parent2 := (*crossoverSet[i+1]).(*Csp2dIndividualImpl)
		offspringProduced := false
		for j := 0; j < MaxIndividualCreationRetries; j++ {
			var postfix1, postfix2, err = p.subtreeCrossover(parent1, parent2, rnd)
			if err != nil {
				return nil, failures, err
			}
			if p.isProtected(postfix1) {
				continue
			}
			if p.isProtected(postfix2) {
				continue
			}
			// Accepted: copy both children out of the crossover scratch (MEM-02);
			// rejected attempts allocate nothing (MEM-04).
			var o1 problem.Individual = newIndividual(p.arena.GenAlloc(postfix1))
			var o2 problem.Individual = newIndividual(p.arena.GenAlloc(postfix2))
			offspringSet = append(offspringSet, &o1)
			offspringSet = append(offspringSet, &o2)
			offspringProduced = true
			break
		}
		if !offspringProduced {
			// Gene pool is full. Generating random individuals to increase exploration
			o1, err := p.NewRandomIndividual(rnd)
			if err != nil {
				return nil, failures, err
			}
			if o1 == nil {
				return nil, failures, fmt.Errorf("Unable to crossover %d species. All possible combinations exhausted.\n", failures)
			} else {
				offspringSet = append(offspringSet, &o1)
			}
			o2, err := p.NewRandomIndividual(rnd)
			if err != nil {
				return nil, failures, err
			}
			if o2 == nil {
				return nil, failures, fmt.Errorf("Unable to crossover %d species. All possible combinations exhausted.\n", failures)
			} else {
				offspringSet = append(offspringSet, &o2)
			}
			failures = failures + 2
		}
	}
	return offspringSet, failures, nil
}

// subtreeCrossover recombines two parents by subtree crossover (GA-01): child 1
// is parent 1 with a subtree of parent 2 grafted in (graftSubtree), child 2 the
// same with the roles swapped. A subtree of a slicing tree is a building block
// (its parts packed with their shape and operators) and is a contiguous segment
// of the postfix, so no tree is built: each child costs O(L).
//
// It returns the children's postfixes, which live in the arena's crossover
// scratch and are valid only until the next crossover: a caller that keeps a
// child copies it into the generation arena and wraps it in an individual
// (Crossover does so after the protection check).
func (p *Csp2dProblemImpl) subtreeCrossover(parent1, parent2 *Csp2dIndividualImpl, rnd *rand.Rand) ([]int, []int, error) {
	if len(parent1.postfix) != len(parent2.postfix) {
		return nil, nil, fmt.Errorf("crossover of postfixes of different lengths %d and %d", len(parent1.postfix), len(parent2.postfix))
	}
	child1, child2 := p.arena.GetCrossoverOffspring()
	child1 = p.graftSubtree(parent1.postfix, parent2.postfix, rnd, child1)
	child2 = p.graftSubtree(parent2.postfix, parent1.postfix, rnd, child2)
	return child1, child2, nil
}

// cloneRetries bounds how often graftSubtree re-picks the graft point and
// operator when the child equals the recipient.
var cloneRetries = 4

// graftSubtree appends to dst a child of recipient that carries a subtree of
// donor:
//  1. pick a donor subtree with 2 .. max(2, n/2) leaves (n = part count);
//  2. remove its parts from recipient, collapsing each operator left with one
//     operand (removeLeaves);
//  3. join it to a random subtree of the rest with a random operator (graftAt).
//
// If the child equals recipient, step 3 is repeated, up to cloneRetries times.
// A postfix with one part is copied.
func (p *Csp2dProblemImpl) graftSubtree(recipient, donor []int, rnd *rand.Rand, dst []int) []int {
	if len(donor) < 3 {
		return append(dst, recipient...)
	}
	start, end := p.pickDonorSubtree(donor, rnd)
	segment := donor[start : end+1]
	rest := p.removeLeaves(recipient, segment)
	if len(rest) == 0 { // the subtree holds every part
		return append(dst, segment...)
	}
	base := len(dst)
	for attempt := 1; ; attempt++ {
		op := NumS
		if rnd.Intn(2) == 1 {
			op = NumL
		}
		dst = graftAt(dst[:base], rest, segment, rnd.Intn(len(rest)), op)
		if attempt == cloneRetries || !samePostfix(dst[base:], recipient) {
			return dst
		}
	}
}

// pickDonorSubtree returns the span [start, end] of a random subtree of postfix
// with 2 .. max(2, n/2) leaves. Every tree with 2 or more leaves has one: an
// operator over two leaves.
func (p *Csp2dProblemImpl) pickDonorSubtree(postfix []int, rnd *rand.Rand) (start, end int) {
	maxLeaves := max(2, int(p.donorFraction*float64(len(postfix)/2+1)))
	leaves := p.arena.crossoverLeaves[:0] // leaf count of each pending subtree
	ends := p.arena.crossoverEnds[:0]     // operator positions that qualify
	for i, id := range postfix {
		if id >= 0 {
			leaves = append(leaves, 1)
			continue
		}
		count := leaves[len(leaves)-2] + leaves[len(leaves)-1]
		leaves = append(leaves[:len(leaves)-2], count)
		if count <= maxLeaves {
			ends = append(ends, i)
		}
	}
	p.arena.crossoverLeaves, p.arena.crossoverEnds = leaves, ends
	end = ends[rnd.Intn(len(ends))]
	return p.subtreeSpan(postfix, end), end
}

// removeLeaves returns postfix without the parts of segment. An operator that
// loses one operand is dropped: the other operand, already written as one
// contiguous run, takes its place. One that loses both is dropped too.
//
// The result lives in the arena's crossover scratch and is valid until the
// next call.
func (p *Csp2dProblemImpl) removeLeaves(postfix, segment []int) []int {
	removed := p.arena.crossoverRemoved
	if need := len(postfix)/2 + 1; len(removed) < need {
		removed = make([]bool, need)
		p.arena.crossoverRemoved = removed
	}
	for _, id := range segment {
		if id >= 0 {
			removed[id] = true
		}
	}
	out := p.arena.crossoverRest[:0]
	present := p.arena.crossoverPresent[:0] // one flag per pending subtree
	for _, id := range postfix {
		if id >= 0 {
			keep := !removed[id]
			if keep {
				out = append(out, id)
			}
			present = append(present, keep)
			continue
		}
		left, right := present[len(present)-2], present[len(present)-1]
		if left && right {
			out = append(out, id)
		}
		present = append(present[:len(present)-2], left || right)
	}
	for _, id := range segment {
		if id >= 0 {
			removed[id] = false
		}
	}
	p.arena.crossoverRest, p.arena.crossoverPresent = out, present
	return out
}

// graftAt appends to dst the postfix rest with segment joined, by operator op,
// to the subtree of rest that ends at position k: rest[:k+1] segment op
// rest[k+1:]. Every token ends a subtree, so every k in [0, len(rest)) gives a
// valid postfix.
func graftAt(dst, rest, segment []int, k, op int) []int {
	dst = append(dst, rest[:k+1]...)
	dst = append(dst, segment...)
	dst = append(dst, op)
	return append(dst, rest[k+1:]...)
}

func samePostfix(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// CopyIndividual returns a copy of individual whose postfix lives in the
// generation arena (ga copies the slots that crossover skips).
func (p *Csp2dProblemImpl) CopyIndividual(individual problem.Individual) problem.Individual {
	return newIndividual(p.arena.GenAlloc(individual.(*Csp2dIndividualImpl).postfix))
}

// DetachIndividual returns a copy whose postfix lives on the Go heap, outside
// every arena, so it survives any number of epochs (championship experiment,
// ga/ga.go). CopyIndividual brings it back into the population.
func (p *Csp2dProblemImpl) DetachIndividual(individual problem.Individual) problem.Individual {
	return newIndividual(slices.Clone(individual.(*Csp2dIndividualImpl).postfix))
}
