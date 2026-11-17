// Package oracle validates csp2d solutions independently of production code.
//
// It re-implements the slicing-tree geometry from the specification
// (docs/dev/design.md §6) and checks, for every solution:
//
//   - structure: each sheet tree is a valid postfix expression over the
//     requested labels and the operators S and L;
//   - coverage: every label appears exactly as often as requested, and no
//     unknown label appears;
//   - geometry: every sheet fits the sheet size (skipped when unconstrained);
//   - fitness: fitnessVector[0] is the sheet count, fitnessVector[1] the sum
//     of the sheet bounding areas, and fitness = sheets*sheetArea + area
//     (just area when unconstrained);
//   - bounds: sheets >= ceil(total part area / sheet area);
//   - ordering: solutions are sorted by ascending fitness.
//
// It must never import production packages: otherwise a decoder bug would be
// "verified" by the same buggy decoder.
//
// Geometry (all nodes normalised so that Long >= Short):
//
//	S ("end to end"):   long sides add.  If one child's short side exceeds the
//	                    other child's long side, that other child is rotated
//	                    (strict comparison, left child checked first).
//	                    Long = l1 + l2 + kerf, Short = max(s1, s2)
//	L ("side by side"): short sides add.
//	                    Long = max(l1, l2), Short = s1 + s2 + kerf
//
// The kerf (saw width) is added once per join, never at the sheet edge.
package oracle

import (
	"csp2d/test/support/api"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Eps is the relative tolerance used for all floating-point comparisons.
const Eps = 1e-9

// Dims are normalised rectangle dimensions (Long >= Short).
type Dims struct {
	Long  float64
	Short float64
}

func (d Dims) Area() float64 { return d.Long * d.Short }

func NewDims(a, b float64) Dims {
	if a >= b {
		return Dims{Long: a, Short: b}
	}
	return Dims{Long: b, Short: a}
}

// Reduce joins two rectangles with operator "S" or "L".
func Reduce(op string, a, b Dims, kerf float64) (Dims, error) {
	l1, s1, l2, s2 := a.Long, a.Short, b.Long, b.Short
	switch op {
	case "S":
		if s1 > l2 { // b fits across a's short side: rotate b
			s2, l2 = l2, s2
		} else if s2 > l1 { // a fits across b's short side: rotate a
			s1, l1 = l1, s1
		}
		return NewDims(l1+l2+kerf, math.Max(s1, s2)), nil
	case "L":
		return NewDims(math.Max(l1, l2), s1+s2+kerf), nil
	default:
		return Dims{}, fmt.Errorf("unknown operator %q", op)
	}
}

// Instance is the problem definition the oracle validates against.
type Instance struct {
	Sheet  *Dims // nil when unconstrained
	Kerf   float64
	Parts  map[string]Dims // label -> dimensions
	Counts map[string]int  // label -> requested count
}

// NewInstance builds an Instance from a start request. Requests that reuse a
// label for different dimensions are ambiguous and rejected.
func NewInstance(req *api.StartRequest) (*Instance, error) {
	inst := &Instance{Parts: map[string]Dims{}, Counts: map[string]int{}}
	if req.Sheets != nil {
		d := NewDims(req.Sheets.Width, req.Sheets.Length)
		inst.Sheet = &d
	}
	if req.SawWidth != nil {
		inst.Kerf = *req.SawWidth
	}
	for _, p := range req.Parts {
		d := NewDims(p.Width, p.Length)
		if prev, ok := inst.Parts[p.Label]; ok && prev != d {
			return nil, fmt.Errorf("label %q is used for different dimensions", p.Label)
		}
		inst.Parts[p.Label] = d
		inst.Counts[p.Label] += p.Count
	}
	return inst, nil
}

// TotalParts is the number of individual parts (sum of counts).
func (inst *Instance) TotalParts() int {
	n := 0
	for _, c := range inst.Counts {
		n += c
	}
	return n
}

// TotalPartArea is the summed area of all individual parts.
func (inst *Instance) TotalPartArea() float64 {
	a := 0.0
	for label, c := range inst.Counts {
		a += float64(c) * inst.Parts[label].Area()
	}
	return a
}

// LowerBound is ceil(total part area / sheet area); 1 when unconstrained.
func (inst *Instance) LowerBound() int {
	if inst.Sheet == nil {
		return 1
	}
	ratio := inst.TotalPartArea() / inst.Sheet.Area()
	lb := int(math.Ceil(ratio - Eps))
	if lb < 1 {
		lb = 1
	}
	return lb
}

// Fits reports whether d fits the sheet (always true when unconstrained).
func (inst *Instance) Fits(d Dims) bool {
	if inst.Sheet == nil {
		return true
	}
	return leq(d.Long, inst.Sheet.Long) && leq(d.Short, inst.Sheet.Short)
}

// ---- Tree parsing -----------------------------------------------------------

// Node is a parsed slicing-tree node.
type Node struct {
	Op    string // "S", "L" or "" for a leaf
	Label string // leaf label
	Dims  Dims
	Left  *Node
	Right *Node
}

func (n *Node) IsLeaf() bool { return n.Op == "" }

// Leaves returns the leaf labels in left-to-right order.
func (n *Node) Leaves() []string {
	if n.IsLeaf() {
		return []string{n.Label}
	}
	return append(n.Left.Leaves(), n.Right.Leaves()...)
}

// ParseTree evaluates a space-separated postfix tree ("1 2 S 3 L").
func (inst *Instance) ParseTree(tree string) (*Node, error) {
	tokens := strings.Fields(tree)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty tree")
	}
	var stack []*Node
	for i, tok := range tokens {
		if tok == "S" || tok == "L" {
			if len(stack) < 2 {
				return nil, fmt.Errorf("token %d (%s): operator needs two operands", i, tok)
			}
			right, left := stack[len(stack)-1], stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			d, err := Reduce(tok, left.Dims, right.Dims, inst.Kerf)
			if err != nil {
				return nil, err
			}
			stack = append(stack, &Node{Op: tok, Dims: d, Left: left, Right: right})
			continue
		}
		d, ok := inst.Parts[tok]
		if !ok {
			return nil, fmt.Errorf("token %d: unknown label %q", i, tok)
		}
		stack = append(stack, &Node{Label: tok, Dims: d})
	}
	if len(stack) != 1 {
		return nil, fmt.Errorf("tree leaves %d unreduced operands", len(stack))
	}
	return stack[0], nil
}

// ---- Invariants -------------------------------------------------------------

// ValidateSolution checks one solution and returns every violation found.
func (inst *Instance) ValidateSolution(sol api.Solution) []string {
	var v []string
	add := func(format string, args ...any) { v = append(v, fmt.Sprintf(format, args...)) }

	if len(sol.Sheets) == 0 {
		add("solution has no sheets")
		return v
	}
	if inst.Sheet == nil && len(sol.Sheets) != 1 {
		add("unconstrained solution must have exactly 1 tree, got %d", len(sol.Sheets))
	}

	counts := map[string]int{}
	area := 0.0
	numbers := make([]int, 0, len(sol.Sheets))
	for _, sh := range sol.Sheets {
		numbers = append(numbers, sh.Number)
		root, err := inst.ParseTree(sh.Tree)
		if err != nil {
			add("sheet %d: %v (tree %q)", sh.Number, err, sh.Tree)
			continue
		}
		for _, l := range root.Leaves() {
			counts[l]++
		}
		if !inst.Fits(root.Dims) {
			add("sheet %d: %gx%g does not fit sheet %gx%g", sh.Number, root.Dims.Long, root.Dims.Short, inst.Sheet.Long, inst.Sheet.Short)
		}
		area += root.Dims.Area()
	}

	sort.Ints(numbers)
	for i, n := range numbers {
		if n != i {
			add("sheet numbers must be 0..%d, got %v", len(numbers)-1, numbers)
			break
		}
	}

	for label, want := range inst.Counts {
		if counts[label] != want {
			add("label %q appears %d times, want %d", label, counts[label], want)
		}
	}
	for label := range counts {
		if _, ok := inst.Counts[label]; !ok {
			add("unexpected label %q", label)
		}
	}

	if len(v) > 0 {
		return v // fitness checks are meaningless on a broken layout
	}

	sheets := len(sol.Sheets)
	if len(sol.FitnessVector) < 2 {
		add("fitnessVector has %d elements, want >= 2", len(sol.FitnessVector))
	} else {
		if sol.FitnessVector[0] != float64(sheets) {
			add("fitnessVector[0] = %g, want sheet count %d", sol.FitnessVector[0], sheets)
		}
		if !approx(sol.FitnessVector[1], area) {
			add("fitnessVector[1] = %g, want sum of sheet bounding areas %g", sol.FitnessVector[1], area)
		}
	}
	wantFitness := area
	if inst.Sheet != nil {
		wantFitness = float64(sheets)*inst.Sheet.Area() + area
	}
	if !approx(sol.Fitness, wantFitness) {
		add("fitness = %g, want %g", sol.Fitness, wantFitness)
	}
	if lb := inst.LowerBound(); sheets < lb {
		add("%d sheets is below the area lower bound %d", sheets, lb)
	}
	if area < inst.TotalPartArea()*(1-Eps) {
		add("bounding area %g is below the total part area %g", area, inst.TotalPartArea())
	}
	return v
}

// ValidateResult checks every solution and the ordering of the list.
func (inst *Instance) ValidateResult(res *api.Result) []string {
	var v []string
	for i, sol := range res.Solutions {
		for _, msg := range inst.ValidateSolution(sol) {
			v = append(v, fmt.Sprintf("solution %d: %s", i, msg))
		}
	}
	for i := 1; i < len(res.Solutions); i++ {
		if res.Solutions[i].Fitness < res.Solutions[i-1].Fitness {
			v = append(v, fmt.Sprintf("solutions not sorted by fitness at index %d (%g < %g)",
				i, res.Solutions[i].Fitness, res.Solutions[i-1].Fitness))
		}
	}
	return v
}

// Waste is sheets*sheetArea - total part area (0 when unconstrained).
func (inst *Instance) Waste(sol api.Solution) float64 {
	if inst.Sheet == nil {
		return 0
	}
	return float64(len(sol.Sheets))*inst.Sheet.Area() - inst.TotalPartArea()
}

func leq(a, b float64) bool { return a <= b+Eps*math.Max(1, math.Abs(b)) }

func approx(a, b float64) bool {
	return math.Abs(a-b) <= Eps*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}
