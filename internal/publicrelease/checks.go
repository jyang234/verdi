// Package publicrelease executes the fixed public execution contract release
// obligations. Reports are auxiliary evidence, not a new durable schema.
package publicrelease

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

const Job = "public-execution-contract-release"

//go:embed testdata/checks.json
var checkBytes []byte

type declaration struct {
	ID     string                `json:"id"`
	AC     string                `json:"ac"`
	Kind   artifact.EvidenceKind `json:"kind"`
	Checks []string              `json:"checks"`
}
type requiredTest struct {
	Repo     string   `json:"repo"`
	Package  string   `json:"package"`
	Name     string   `json:"name"`
	Required []string `json:"required"`
}
type declarations struct {
	Producers []declaration           `json:"producers"`
	Tests     map[string]requiredTest `json:"tests"`
}

func readDeclarations(data []byte) (declarations, error) {
	var d declarations
	if err := decodeDocument(data, &d); err != nil {
		return d, err
	}
	if len(d.Producers) != 10 {
		return d, fmt.Errorf("expected ten producers")
	}
	seen := map[string]bool{}
	for _, p := range d.Producers {
		valid := false
		for n := 1; n <= 5; n++ {
			for _, k := range []artifact.EvidenceKind{"static", "behavioral"} {
				if p.AC == fmt.Sprintf("ac-%d", n) && p.Kind == k && p.ID == "public-execution-contract:"+p.AC+":"+string(k) {
					valid = true
				}
			}
		}
		if !valid || seen[p.ID] || len(p.Checks) == 0 {
			return d, fmt.Errorf("invalid or duplicate producer %q", p.ID)
		}
		seen[p.ID] = true
		for _, key := range p.Checks {
			r, ok := d.Tests[key]
			if !ok || key != r.Repo+":"+r.Package+":"+r.Name || (r.Repo != "V" && r.Repo != "A") || !strings.HasPrefix(r.Name, "Test") || len(r.Required) == 0 {
				return d, fmt.Errorf("invalid test check %q", key)
			}
			root := false
			for _, name := range r.Required {
				if name == r.Name {
					root = true
				}
				if name != r.Name && !strings.HasPrefix(name, r.Name+"/") {
					return d, fmt.Errorf("unrelated required child %q", name)
				}
			}
			if !root {
				return d, fmt.Errorf("missing test root %s", r.Name)
			}
		}
	}
	return d, nil
}

type testGroup struct {
	Repo, Package   string
	Roots, Required []string
}

func groups(d declarations) []testGroup {
	by := map[string]*testGroup{}
	for _, r := range d.Tests {
		key := r.Repo + ":" + r.Package
		g := by[key]
		if g == nil {
			g = &testGroup{Repo: r.Repo, Package: r.Package}
			by[key] = g
		}
		g.Roots = append(g.Roots, r.Name)
		g.Required = append(g.Required, r.Required...)
	}
	keys := make([]string, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]testGroup, 0, len(keys))
	for _, k := range keys {
		g := by[k]
		sort.Strings(g.Roots)
		sort.Strings(g.Required)
		out = append(out, *g)
	}
	return out
}

func sameBytes(a, b []byte) bool { return bytes.Equal(a, b) }
