package csp2d

import (
	"csp2d/problem"
	"fmt"
	"math"
	"math/rand"
	"sort"
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
		var reuse []int // the buffer a kept child may overwrite
		if owned[individualIndex] {
			reuse = parent.postfix
		}

		// Each step returns a candidate built in scratch; it takes arena space
		// only once it is kept (MEM-02).

		// 1) Try "eliminate-and-repair"
		var weight = 3
		child, improved, err := p.mutateEliminateAndRepair(parent, rnd)
		if err != nil {
			return totalMutations, totalWeight, failures, err
		}
		if child != nil && improved && !p.isProtected(child.postfix) {
			p.commitRepairChild(child, reuse)
			var result problem.Individual = child
			mutationSet[individualIndex] = &result
			owned[individualIndex] = true
			totalMutations++
			totalWeight += weight
			mutationsAllowed -= weight
			mutated = true
		}

		// 2) Fallback: critical operator flip
		weight = 1
		if !mutated {
			child, improved, err = p.mutateCriticalOperatorFlip(parent, rnd)
			if err != nil {
				return totalMutations, totalWeight, failures, err
			}
			if child != nil && improved && !p.isProtected(child.postfix) {
				// Decodes the candidate's scratch postfix; the forest goes to
				// the arena, the postfix is committed below.
				err1 := p.evaluateFitness(child)
				if err1 != nil {
					return totalMutations, totalWeight, failures, err1
				}
				if LessFitness(child, parent) {
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

// commitRepairChild keeps an eliminate-and-repair candidate: the sheets it
// re-packed are copied out of the trial buffer into the arena (the others are
// shared with the parent's forest), then its postfix is committed. It must
// directly follow mutateEliminateAndRepair, whose freshness flags it reads.
func (p *Csp2dProblemImpl) commitRepairChild(child *Csp2dIndividualImpl, reuse []int) {
	for i, fresh := range p.arena.reinsertionFresh {
		if fresh {
			child.meteredForest[i].postfix = p.arena.GenAlloc(child.meteredForest[i].postfix)
		}
	}
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
	baseForest := make([]MeteredNode, 0, len(forest)-1)
	baseForest = append(baseForest, forest[:worstIndex]...)
	baseForest = append(baseForest, forest[worstIndex+1:]...)
	items := removedTree.leafIDs()
	if len(items) == 0 {
		return nil, false, nil
	}

	orders := buildReinsertionOrders(items, p.inputs, rnd)
	rnd.Shuffle(len(orders), func(i, j int) {
		orders[i], orders[j] = orders[j], orders[i]
	})

	parentFitness := parent.fitnessVector

	// bestForest is the WINNING forest — owned by us, not the scratch.
	// It is populated only when an improvement is found.
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
			// Capture the winner: copy the []MeteredNode headers out of the
			// scratch into a fresh slice so subsequent trials (or future
			// mutateEliminateAndRepair calls) cannot overwrite it. Each
			// node's postfix already lives in the generational postfix
			// arena, so we don't need to copy postfix data.
			bestForest = make([]MeteredNode, len(candidate))
			copy(bestForest, candidate)
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
	return newIndividual(best).(*Csp2dIndividualImpl), true, nil
}

func (p *Csp2dProblemImpl) rankCriticalOperators(postfix []int) []OperatorCandidate {
	type entry struct {
		metrics Metrics
	}
	stack := make([]entry, 0, len(postfix)/2+1)
	var result []OperatorCandidate

	for i, id := range postfix {
		if id >= 0 { // leaf
			stack = append(stack, entry{
				metrics: Metrics{
					longSide:  p.inputs[id].LongSide,
					shortSide: p.inputs[id].ShortSide,
				},
			})
		} else { // operator
			if len(stack) < 2 {
				continue
			}
			right := stack[len(stack)-1]
			left := stack[len(stack)-2]
			stack = stack[:len(stack)-2]

			current := p.reduce(id, left.metrics, right.metrics, p.constraints.sawWidth)
			subtreeArea := current.GetArea()
			childrenArea := left.metrics.GetArea() + right.metrics.GetArea()
			penalty := subtreeArea - childrenArea

			result = append(result, OperatorCandidate{
				Index: i, // postfix index — can flip directly
				Score: penalty,
			})

			stack = append(stack, entry{metrics: current})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

func buildReinsertionOrders(items []int, inputs []InputDefinition, rnd *rand.Rand) [][]int {
	orders := make([][]int, 0, 4)

	byArea := append([]int(nil), items...)
	sort.Slice(byArea, func(i, j int) bool {
		ai := inputs[byArea[i]].LongSide * inputs[byArea[i]].ShortSide
		aj := inputs[byArea[j]].LongSide * inputs[byArea[j]].ShortSide
		return ai > aj
	})
	orders = append(orders, byArea)

	byLong := append([]int(nil), items...)
	sort.Slice(byLong, func(i, j int) bool {
		return inputs[byLong[i]].LongSide > inputs[byLong[j]].LongSide
	})
	orders = append(orders, byLong)

	byShort := append([]int(nil), items...)
	sort.Slice(byShort, func(i, j int) bool {
		return inputs[byShort[i]].ShortSide > inputs[byShort[j]].ShortSide
	})
	orders = append(orders, byShort)

	randomOrder := append([]int(nil), items...)
	rnd.Shuffle(len(randomOrder), func(i, j int) {
		randomOrder[i], randomOrder[j] = randomOrder[j], randomOrder[i]
	})
	orders = append(orders, randomOrder)

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
