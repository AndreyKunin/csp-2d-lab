package csp2d

import (
	"csp2d/problem"
	"fmt"
	"math"
	"math/rand"
	"slices"
)

type OperatorCandidate struct {
	Index int
	Score float64
}

func (p *Csp2dProblemImpl) Mutation(mutationRate float64, mutationSet []*problem.Individual, rnd *rand.Rand) (int, int, int, error) {
	setSize := len(mutationSet)
	failures := 0
	if setSize == 0 {
		return 0, 0, failures, nil
	}

	firstIndividual := (*mutationSet[0]).(*Csp2dIndividualImpl)
	individualSize := len(firstIndividual.postfix)
	totalSize := individualSize * setSize
	mutationsAllowed := int(math.Ceil(float64(totalSize) * mutationRate))

	totalWeight := 0
	totalMutations := 0

	// owned[i] reports that the individual in slot i was created by this call,
	// so nothing else references its postfix buffer. The child that supersedes
	// it is written into that buffer instead of taking new arena space: at the
	// default rate a slot is mutated several times per epoch (MEM-02).
	owned := make([]bool, setSize)

	for mutationsAllowed > 0 {
		mutated := false
		// randomly choose an individual
		individualIndex := rnd.Intn(setSize)
		parent := (*mutationSet[individualIndex]).(*Csp2dIndividualImpl)

		err := p.evaluateFitness(parent)
		if err != nil {
			return totalMutations, totalWeight, failures, err
		}
		var reuse []int               // the postfix buffer a kept child may overwrite
		var reuseForest []MeteredNode // and its forest's node slice (MEM-04)
		if owned[individualIndex] {
			reuse, reuseForest = parent.postfix, parent.meteredForest
		}

		// Each step returns a candidate built in scratch; it takes arena space
		// only once it is kept (MEM-02).

		// 1) Try "eliminate-and-repair"
		var weight = p.repairWeight
		child, improved, err := p.mutateEliminateAndRepair(parent, rnd)
		if err != nil {
			return totalMutations, totalWeight, failures, err
		}
		if child != nil && improved && !p.isProtected(child.postfix) {
			p.commitRepairChild(child, reuse, reuseForest)
			var result problem.Individual = child
			mutationSet[individualIndex] = &result
			owned[individualIndex] = true
			totalMutations++
			totalWeight += weight
			mutationsAllowed -= weight
			mutated = true
		}

		// 2) Fallback: critical operator flip
		weight = p.flipWeight
		if !mutated {
			child, improved, err = p.mutateCriticalOperatorFlip(parent, rnd)
			if err != nil {
				return totalMutations, totalWeight, failures, err
			}
			// The candidate carries the fitness of its trial decode, so it is
			// compared before anything of the parent's is overwritten.
			if child != nil && improved && !p.isProtected(child.postfix) && LessFitness(child, parent) {
				// Decodes the candidate's scratch postfix and caches the
				// forest; then the postfix is committed.
				if err1 := p.cacheForest(child, reuseForest); err1 != nil {
					return totalMutations, totalWeight, failures, err1
				}
				child.postfix = p.commitPostfix(child.postfix, reuse)
				var result problem.Individual = child
				mutationSet[individualIndex] = &result
				owned[individualIndex] = true
				totalMutations++
				totalWeight += weight
				mutationsAllowed -= weight
				mutated = true
			}
		}

		// 3) If nothing improved, fall back to the destructive mutation
		if !mutated {
			var operator = rnd.Intn(2)
			// The operators write the new postfix into scratch (MEM-05: no copy
			// of the parent beforehand).
			trial := &Csp2dIndividualImpl{postfix: parent.postfix}
			if operator < 1 {
				weight, err = p.changeOperator(trial, rnd)
			} else {
				weight, err = p.replaceElements(trial, rnd)
			}
			if err != nil {
				return totalMutations, totalWeight, failures, err
			}
			if !p.isProtected(trial.postfix) {
				trial.postfix = p.commitPostfix(trial.postfix, reuse)
				var result problem.Individual = trial
				mutationSet[individualIndex] = &result
				owned[individualIndex] = true
				totalMutations++
				totalWeight += weight
				mutationsAllowed -= weight
				mutated = true
			}
		}

		// 4) If the gene pool is full, insert a migrant
		if !mutated {
			o1, err1 := p.NewRandomIndividual(rnd)
			if err1 != nil {
				return totalMutations, totalWeight, failures, err1
			}
			if o1 == nil {
				return totalMutations, totalWeight, failures, fmt.Errorf("Unable to mutate %d species. All possible combinations exhausted.\n", failures)
			}

			mutationSet[individualIndex] = &o1
			owned[individualIndex] = true
			failures = failures + 1
		}
	}

	return totalMutations, totalWeight, failures, nil
}

// commitPostfix copies a kept candidate's postfix out of scratch: into reuse,
// the buffer of the child it supersedes in the same slot, when there is one,
// otherwise into the generation arena. reuse must be referenced by nothing
// else; the candidate's forest never points into it (forests are copies).
func (p *Csp2dProblemImpl) commitPostfix(src, reuse []int) []int {
	if len(src) > 0 && cap(reuse) >= len(src) {
		dst := reuse[:len(src)]
		copy(dst, src)
		return dst
	}
	return p.arena.GenAlloc(src)
}

// commitRepairChild keeps an eliminate-and-repair candidate: its forest (still
// the reinsertion scratch) is copied into reuseForest when that is large
// enough, else into a new slice; the sheets it re-packed are copied out of the
// trial buffer into the arena (the others are shared with the parent's
// forest); then its postfix is committed. It must directly follow
// mutateEliminateAndRepair, whose scratch and freshness flags it reads.
func (p *Csp2dProblemImpl) commitRepairChild(child *Csp2dIndividualImpl, reuse []int, reuseForest []MeteredNode) {
	var kept []MeteredNode
	if cap(reuseForest) >= len(child.meteredForest) {
		kept = reuseForest[:len(child.meteredForest)]
	} else {
		kept = make([]MeteredNode, len(child.meteredForest))
	}
	copy(kept, child.meteredForest)
	for i, fresh := range p.arena.reinsertionFresh {
		if fresh {
			kept[i].postfix = p.arena.GenAlloc(kept[i].postfix)
		}
	}
	child.meteredForest = kept
	child.postfix = p.commitPostfix(child.postfix, reuse)
}

func (p *Csp2dProblemImpl) mutateEliminateAndRepair(parent *Csp2dIndividualImpl, rnd *rand.Rand) (*Csp2dIndividualImpl, bool, error) {
	var forest []MeteredNode
	var err error
	if parent.meteredForest != nil {
		forest = parent.meteredForest
	} else {
		forest, err = p.getForest(parent)
		if err != nil {
			return nil, false, err
		}
	}
	if len(forest) <= 1 {
		return nil, false, nil
	}

	worstIndex := p.selectWorstTree(forest)
	removedTree := forest[worstIndex]
	baseForest := append(p.arena.baseForest[:0], forest[:worstIndex]...)
	baseForest = append(baseForest, forest[worstIndex+1:]...)
	p.arena.baseForest = baseForest[:0] // keep the buffer if it grew
	items := removedTree.appendLeafIDs(p.arena.leafItems[:0])
	p.arena.leafItems = items[:0]
	if len(items) == 0 {
		return nil, false, nil
	}

	orders := buildReinsertionOrders(items, p.inputs, rnd, p.arena.orders)
	rnd.Shuffle(len(orders), func(i, j int) {
		orders[i], orders[j] = orders[j], orders[i]
	})

	parentFitness := parent.fitnessVector

	// bestForest is the WINNING forest. It is still the reinsertion scratch:
	// the caller copies it if it keeps the child (commitRepairChild).
	var bestForest []MeteredNode
	var bestFitness [3]float64
	improved := false

	for _, order := range orders {
		// Each trial reuses the SAME scratch buffer. After the next call to
		// reinsertItemsForest, the previous trial's data is gone — so we
		// must capture the winner before we move on.
		candidate, err1 := p.reinsertItemsForest(baseForest, order)
		if err1 != nil {
			return nil, false, err1
		}

		fit := p.getFitnessVector(candidate)

		if LessFitnessRaw(fit, parentFitness) {
			// The first improvement wins, so no later trial overwrites the
			// scratch before the caller commits or drops the child.
			bestForest = candidate
			bestFitness = fit
			improved = true
			break // first improvement wins, matching the original behavior
		}
	}

	if !improved {
		return parent, false, nil
	}

	// Materialize the top-level postfix only for the winner, with random
	// operators between trees (this matches the original semantics). The
	// candidate's postfix and re-packed sheets stay in scratch until the
	// caller keeps it (commitRepairChild).
	childPostfix := p.arena.GetCandidatePostfix()
	for i, mn := range bestForest {
		childPostfix = append(childPostfix, mn.postfix...)
		if i > 0 {
			childPostfix = append(childPostfix, opInfoByOp[rnd.Intn(OpsCount)].Num)
		}
	}

	child := newIndividual(childPostfix).(*Csp2dIndividualImpl)
	child.meteredForest = bestForest
	child.fitnessVector = bestFitness
	child.fitnessValue = p.getFitnessValue(child, p.constraints)

	return child, true, nil
}

// selectWorstTree returns the index of the tree with the lowest fill of its
// bounding box, Σ part area / bounding area; on a tie the first one. Ranking
// by the fill of the sheet area instead (the emptiest sheet) was measured and
// is worse (design.md §6, GA-07 disproved).
func (p *Csp2dProblemImpl) selectWorstTree(forest []MeteredNode) int {
	worstIdx := 0
	worstFill := math.Inf(1)

	for i, t := range forest {
		boundingArea := t.measurements.GetArea()
		itemArea := t.itemArea(p.inputs)

		fill := 1.0
		if boundingArea > 0 {
			fill = itemArea / boundingArea
		}

		if fill < worstFill {
			worstFill = fill
			worstIdx = i
		}
	}

	return worstIdx
}

// insertItemGreedyInPlace merges the item into the best tree of forest and
// returns that tree's index, or -1 if the item fits none. The merged postfix is
// built in the arena's trial buffer.
func (p *Csp2dProblemImpl) insertItemGreedyInPlace(forest []MeteredNode, itemID int) int {
	itemMetrics := Metrics{
		longSide:  p.inputs[itemID].LongSide,
		shortSide: p.inputs[itemID].ShortSide,
	}

	var bestFitness [3]float64
	bestIndex := -1
	bestOpNum := 0
	bestItemLeft := false
	bestMeasurements := Metrics{}
	found := false

	for i, tree := range forest {
		for _, opInfo := range opInfoByOp {
			// tree + item, compute measurements without cloning the node
			m1 := p.reduce(opInfo.Num, tree.measurements, itemMetrics, p.constraints.sawWidth)
			if p.isValidMetrics(m1, p.constraints) {
				// Compute fitness by temporarily substituting measurements only
				fit := p.getFitnessVectorWithReplacement(forest, i, m1)
				if !found || LessFitnessRaw(fit, bestFitness) {
					// Defer the actual node construction to after the loop
					bestIndex = i
					bestOpNum = opInfo.Num
					bestItemLeft = false
					bestMeasurements = m1
					bestFitness = fit
					found = true
				}
			}

			// item + tree
			m2 := p.reduce(opInfo.Num, itemMetrics, tree.measurements, p.constraints.sawWidth)
			if p.isValidMetrics(m2, p.constraints) {
				fit := p.getFitnessVectorWithReplacement(forest, i, m2)
				if !found || LessFitnessRaw(fit, bestFitness) {
					bestIndex = i
					bestOpNum = opInfo.Num
					bestItemLeft = true
					bestMeasurements = m2
					bestFitness = fit
					found = true
				}
			}
		}
	}

	if !found {
		return -1
	}

	// Merge postfixes: item + existing + op OR existing + item + op
	existing := forest[bestIndex]
	arr := p.arena.TrialAllocN(len(existing.postfix) + 2)
	if bestItemLeft {
		arr = append(arr, itemID)
		arr = append(arr, existing.postfix...)
		arr = append(arr, bestOpNum)
	} else {
		arr = append(arr, existing.postfix...)
		arr = append(arr, itemID)
		arr = append(arr, bestOpNum)
	}

	forest[bestIndex] = MeteredNode{
		postfix:      arr,
		measurements: bestMeasurements,
	}

	return bestIndex
}

// reinsertItemsForest builds a candidate forest by greedily inserting `order`
// items into a shallow clone of baseForest. It writes into the shared
// reinsertion scratch buffer in Arena — the returned slice, the re-packed
// sheets' postfixes (trial buffer) and the freshness flags
// (Arena.reinsertionFresh) are INVALIDATED on the next call. The caller must
// copy what it keeps (commitRepairChild).
func (p *Csp2dProblemImpl) reinsertItemsForest(baseForest []MeteredNode, order []int) ([]MeteredNode, error) {
	p.arena.ResetTrialInts()
	forest := p.arena.GetReinsertionForest()
	fresh := p.arena.GetReinsertionFresh()
	defer func() { p.arena.reinsertionFresh = fresh }()
	for _, t := range baseForest {
		forest = append(forest, MeteredNode{
			postfix:      t.postfix, // share — read-only data living in arenas
			measurements: t.measurements,
		})
		fresh = append(fresh, false)
	}

	for _, itemID := range order {
		if index := p.insertItemGreedyInPlace(forest, itemID); index >= 0 {
			fresh[index] = true
			continue
		}

		metrics := Metrics{
			longSide:  p.inputs[itemID].LongSide,
			shortSide: p.inputs[itemID].ShortSide,
		}
		if !p.isValidMetrics(metrics, p.constraints) {
			return nil, fmt.Errorf("item %d cannot fit constraints", itemID)
		}
		singletonPostfix := p.arena.TrialAllocN(1)
		singletonPostfix = append(singletonPostfix, itemID)
		forest = append(forest, MeteredNode{
			postfix:      singletonPostfix,
			measurements: metrics,
		})
		fresh = append(fresh, true)
	}

	return forest, nil
}

func (p *Csp2dProblemImpl) mutateCriticalOperatorFlip(parent *Csp2dIndividualImpl, rnd *rand.Rand) (*Csp2dIndividualImpl, bool, error) {
	err := p.evaluateFitness(parent)
	if err != nil {
		return nil, false, err
	}

	candidates := p.rankCriticalOperators(parent.postfix)
	if len(candidates) == 0 {
		return nil, false, nil
	}

	maxTrials := 4
	if len(candidates) < maxTrials {
		maxTrials = len(candidates)
	}

	bestFitness := parent.fitnessVector
	improved := false

	// Trials are decoded in scratch and take no arena space; the best one is
	// returned as a candidate whose postfix is still in scratch and whose
	// fitness is not cached. The caller evaluates and commits it if it keeps
	// it (MEM-02).
	trial := p.arena.GetTrialPostfix()
	best := p.arena.GetCandidatePostfix()

	for i := 0; i < maxTrials; i++ {
		trial = append(trial[:0], parent.postfix...)
		trial[candidates[i].Index] = FlipOperator(trial[candidates[i].Index])

		forest, err1 := p.decodeForest(trial)
		if err1 != nil {
			continue
		}
		if fitness := p.getFitnessVector(forest); LessFitnessRaw(fitness, bestFitness) {
			best = append(best[:0], trial...)
			bestFitness = fitness
			improved = true
		}
	}

	if !improved {
		return parent, false, nil
	}
	child := newIndividual(best).(*Csp2dIndividualImpl)
	child.fitnessVector = bestFitness // its forest is not cached yet
	return child, true, nil
}

func (p *Csp2dProblemImpl) rankCriticalOperators(postfix []int) []OperatorCandidate {
	// Scratch buffers (MEM-04): the result is valid until the next call.
	stack := p.arena.rankStack[:0]
	result := p.arena.rankCandidate[:0]

	for i, id := range postfix {
		if id >= 0 { // leaf
			stack = append(stack, Metrics{
				longSide:  p.inputs[id].LongSide,
				shortSide: p.inputs[id].ShortSide,
			})
		} else { // operator
			if len(stack) < 2 {
				continue
			}
			right := stack[len(stack)-1]
			left := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			current := p.reduce(id, left, right, p.constraints.sawWidth)
			subtreeArea := current.GetArea()
			childrenArea := left.GetArea() + right.GetArea()
			penalty := subtreeArea - childrenArea

			// GA-09: a join that overflows the sheet under both operators is
			// skipped (flipping it rarely changes the layout), and the rest are
			// ranked by relative waste (measured: design.md §6, former GA-09).
			flipped := p.reduce(FlipOperator(id), left, right, p.constraints.sawWidth)
			if p.isValidMetrics(current, p.constraints) || p.isValidMetrics(flipped, p.constraints) {
				result = append(result, OperatorCandidate{
					Index: i, // postfix index — can flip directly
					Score: penalty / subtreeArea,
				})
			}

			stack = append(stack, current)
		}
	}
	p.arena.rankStack, p.arena.rankCandidate = stack[:0], result[:0] // keep grown buffers

	// slices.SortFunc runs the same pattern-defeating quicksort as sort.Slice
	// (both are generated from one template) without its per-call allocations;
	// it only tests cmp < 0, which is exactly the old "less".
	slices.SortFunc(result, func(a, b OperatorCandidate) int { return descending(a.Score, b.Score) })

	return result
}

// descending orders larger values first: -1 if a > b, 1 if b > a, else 0.
func descending(a, b float64) int {
	if a > b {
		return -1
	}
	if b > a {
		return 1
	}
	return 0
}

// buildReinsertionOrders fills the 4 slices of orders (by area, long side,
// short side, random) with the items and returns orders. The slices are
// reused (MEM-04); a grown one is kept.
func buildReinsertionOrders(items []int, inputs []InputDefinition, rnd *rand.Rand, orders [][]int) [][]int {
	byArea := append(orders[0][:0], items...)
	slices.SortFunc(byArea, func(a, b int) int {
		return descending(inputs[a].LongSide*inputs[a].ShortSide, inputs[b].LongSide*inputs[b].ShortSide)
	})

	byLong := append(orders[1][:0], items...)
	slices.SortFunc(byLong, func(a, b int) int { return descending(inputs[a].LongSide, inputs[b].LongSide) })

	byShort := append(orders[2][:0], items...)
	slices.SortFunc(byShort, func(a, b int) int { return descending(inputs[a].ShortSide, inputs[b].ShortSide) })

	randomOrder := append(orders[3][:0], items...)
	rnd.Shuffle(len(randomOrder), func(i, j int) {
		randomOrder[i], randomOrder[j] = randomOrder[j], randomOrder[i]
	})

	orders[0], orders[1], orders[2], orders[3] = byArea, byLong, byShort, randomOrder
	return orders
}

func (p *Csp2dProblemImpl) changeOperator(individual *Csp2dIndividualImpl, rnd *rand.Rand) (int, error) {
	// The new postfix is built in scratch; Mutation commits it if it keeps it.
	postfixCopy := append(p.arena.GetCandidatePostfix(), individual.postfix...)
	var individualSize = len(postfixCopy)
	//find random operator to be changed
	var index = rnd.Intn(individualSize)
	var bound = 0
	for { // must be an operator
		if index+bound < individualSize && postfixCopy[index+bound] < 0 {
			index = index + bound
			break
		}
		if index-bound > 0 && postfixCopy[index-bound] < 0 {
			index = index - bound
			break
		}
		bound = bound + 1
	}
	//change the individual
	postfixCopy[index] = FlipOperator(postfixCopy[index])
	individual.postfix = postfixCopy
	individual.invalidateCache()

	return 1, nil
}

func (p *Csp2dProblemImpl) replaceElements(individual *Csp2dIndividualImpl, rnd *rand.Rand) (int, error) {
	postfix := individual.postfix
	n := len(postfix)

	// Pick two distinct random positions (non-zero to avoid trivial picks)
	i1 := rnd.Intn(n-1) + 1
	i2 := i1
	for i2 == i1 {
		i2 = rnd.Intn(n-1) + 1
	}

	// Find subtree spans
	start1 := p.subtreeSpan(postfix, i1)
	start2 := p.subtreeSpan(postfix, i2)

	// Ensure i1's span comes first
	if start1 > start2 {
		start1, i1, start2, i2 = start2, i2, start1, i1
	}

	sub1 := postfix[start1 : i1+1]
	sub2 := postfix[start2 : i2+1]

	// Check for overlap — if subtrees overlap, skip
	if i1 >= start2 {
		// Overlapping subtrees: just do an operator flip as a lighter mutation
		for idx := n - 1; idx >= 0; idx-- {
			if postfix[idx] < 0 {
				newPostfix := append(p.arena.GetCandidatePostfix(), postfix...)
				newPostfix[idx] = FlipOperator(newPostfix[idx])
				individual.postfix = newPostfix
				individual.invalidateCache()
				return 1, nil
			}
		}
		return 0, nil
	}

	// Swap the two subtree slices in a new postfix (built in scratch;
	// Mutation commits it if it keeps it)
	newPostfix := p.arena.GetCandidatePostfix()
	newPostfix = append(newPostfix, postfix[:start1]...)
	newPostfix = append(newPostfix, sub2...)
	newPostfix = append(newPostfix, postfix[i1+1:start2]...)
	newPostfix = append(newPostfix, sub1...)
	newPostfix = append(newPostfix, postfix[i2+1:]...)

	individual.postfix = newPostfix
	individual.invalidateCache()
	return 2, nil
}

// subtreeSpan returns the start index of the subtree ending at position `end` in a postfix expression.
func (p *Csp2dProblemImpl) subtreeSpan(postfix []int, end int) int {
	needed := 1 // we need 1 complete sub-expression
	for i := end; i >= 0; i-- {
		if postfix[i] >= 0 { // operand
			needed--
		} else { // operator — needs 2 operands, produces 1
			needed++
		}
		if needed == 0 {
			return i
		}
	}
	return 0
}

// Defaults of the search parameters (SetSearchParameters): budget units of an
// accepted eliminate-and-repair and critical flip, and the largest donor
// subtree of the crossover as a share of the parts.
const (
	defaultRepairWeight  = 3
	defaultFlipWeight    = 1
	defaultDonorFraction = 0.5
)

// SetSearchParameters sets the problem's search parameters (the engine passes
// them every epoch from ga.Parameters). A weight below 1 takes its default (a
// weight of 0 would never use up the mutation budget); a donor fraction outside
// (0, 1] takes its default.
func (p *Csp2dProblemImpl) SetSearchParameters(repairWeight, flipWeight int, donorFraction float64) {
	if repairWeight < 1 {
		repairWeight = defaultRepairWeight
	}
	if flipWeight < 1 {
		flipWeight = defaultFlipWeight
	}
	if !(donorFraction > 0 && donorFraction <= 1) { // also catches NaN
		donorFraction = defaultDonorFraction
	}
	p.repairWeight, p.flipWeight, p.donorFraction = repairWeight, flipWeight, donorFraction
}
