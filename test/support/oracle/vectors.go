package oracle

import (
	"csp2d/test/support/api"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

// TreeVector is one shared geometry case from test/testdata/trees/*.json.
// The same files are meant to be consumed by the Go unit tests, the oracle and
// the frontend tests, so all three agree on dimensions.
type TreeVector struct {
	Name  string                `json:"name"`
	Kerf  float64               `json:"kerf"`
	Parts map[string][2]float64 `json:"parts"`
	Tree  string                `json:"tree"`
	Long  float64               `json:"long"`
	Short float64               `json:"short"`
}

// Instance returns an unconstrained instance holding the vector's parts.
func (tv TreeVector) Instance() *Instance {
	req := &api.StartRequest{SawWidth: api.Float(tv.Kerf)}
	for label, wl := range tv.Parts {
		req.Parts = append(req.Parts, api.Part{Label: label, Width: wl[0], Length: wl[1], Count: 1})
	}
	inst, _ := NewInstance(req)
	return inst
}

// Postfix maps each label to a dense id 0..n-1 in sorted label order, and returns
// the postfix encoding of the tree using those ids and the given operator codes.
func (tv TreeVector) Postfix(opS, opL int) (labels []string, postfix []int, err error) {
	for label := range tv.Parts {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool { return lessLabel(labels[i], labels[j]) })
	ids := map[string]int{}
	for i, l := range labels {
		ids[l] = i
	}
	for _, tok := range strings.Fields(tv.Tree) {
		switch tok {
		case "S":
			postfix = append(postfix, opS)
		case "L":
			postfix = append(postfix, opL)
		default:
			id, ok := ids[tok]
			if !ok {
				return nil, nil, fmt.Errorf("vector %s: unknown label %q", tv.Name, tok)
			}
			postfix = append(postfix, id)
		}
	}
	return labels, postfix, nil
}

// LoadTreeVectors loads all *.json files from test/testdata/trees.
func LoadTreeVectors() ([]TreeVector, error) {
	dir, err := TestdataDir("trees")
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var all []TreeVector
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var vs []TreeVector
		if err := json.Unmarshal(data, &vs); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		all = append(all, vs...)
	}
	return all, nil
}

// TestdataDir returns test/testdata/<sub>, located relative to this source file
// so it works from any test package directory.
func TestdataDir(sub string) (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate oracle source")
	}
	// file = <root>/test/support/oracle/vectors.go
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", sub), nil
}

func lessLabel(a, b string) bool {
	ai, errA := strconv.Atoi(a)
	bi, errB := strconv.Atoi(b)
	if errA == nil && errB == nil {
		return ai < bi
	}
	return a < b
}
