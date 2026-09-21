// Command versioncompare is spike/verification-rules oq-4 evidence: it
// decodes named pairs of versioned specs through the real artifact
// decoder, aligns their acceptance_criteria/constraints by id, and
// mechanically classifies each aligned pair as identical (byte-equal
// text) or different (any byte differs) — the mechanical half of oq-4's
// classification; reworded-same-claim vs different-claim among the
// "different" pairs is a hand judgment recorded in README.md, not this
// program's job.
//
// Usage: go run ./docs/spikes/verification-rules/_scratch/versioncompare [root]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
)

type obj struct {
	kind string // "ac" or "co"
	text string
}

func load(root, name string) (*artifact.SpecFrontmatter, map[string]obj, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", name, "spec.md"))
	if err != nil {
		return nil, nil, err
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, nil, err
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, nil, err
	}
	m := map[string]obj{}
	for _, ac := range spec.AcceptanceCriteria {
		m[ac.ID] = obj{"ac", ac.Text}
	}
	for _, co := range spec.Constraints {
		m[co.ID] = obj{"co", co.Text}
	}
	return spec, m, nil
}

func compare(root, a, b string) {
	fmt.Printf("\n===== %s vs %s =====\n", a, b)
	specA, mA, err := load(root, a)
	if err != nil {
		fmt.Printf("LOAD ERROR %s: %v\n", a, err)
		return
	}
	specB, mB, err := load(root, b)
	if err != nil {
		fmt.Printf("LOAD ERROR %s: %v\n", b, err)
		return
	}
	fmt.Printf("%s status=%q; %s status=%q\n", a, specA.Status, b, specB.Status)
	var supersedesB string
	for _, l := range specB.Links {
		if string(l.Type) == "supersedes" {
			supersedesB = l.Ref
		}
	}
	fmt.Printf("%s links[type=supersedes].ref = %q\n", b, supersedesB)

	identical, different, onlyA, onlyB := 0, 0, 0, 0
	// Sort ids before iterating (VR-6 correction): Go map iteration order
	// is randomized per run, and a committed evidence file whose lines
	// reorder on every regeneration invites a false "does not reproduce"
	// reading even when its totals are unchanged.
	idsA := make([]string, 0, len(mA))
	for id := range mA {
		idsA = append(idsA, id)
	}
	sort.Strings(idsA)
	for _, id := range idsA {
		oa := mA[id]
		ob, ok := mB[id]
		if !ok {
			onlyA++
			fmt.Printf("  ONLY IN %s: %s\n", a, id)
			continue
		}
		if oa.text == ob.text {
			identical++
			fmt.Printf("  IDENTICAL   %s\n", id)
		} else {
			different++
			fmt.Printf("  DIFFERENT   %s\n", id)
			fmt.Printf("    %s: %s\n", a, oa.text)
			fmt.Printf("    %s: %s\n", b, ob.text)
		}
	}
	idsB := make([]string, 0, len(mB))
	for id := range mB {
		idsB = append(idsB, id)
	}
	sort.Strings(idsB)
	for _, id := range idsB {
		if _, ok := mA[id]; !ok {
			onlyB++
			fmt.Printf("  ONLY IN %s: %s\n", b, id)
		}
	}
	fmt.Printf("  totals: identical=%d different=%d only-%s=%d only-%s=%d\n", identical, different, a, onlyA, b, onlyB)
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	compare(root, "guided-lifecycle-governance", "guided-lifecycle-governance-v2")
	compare(root, "guided-lifecycle-governance-v2", "guided-lifecycle-governance-v3")
	compare(root, "comparative-spike-experiments", "comparative-spike-experiments-v2")
	compare(root, "comparative-spike-experiments-v2", "comparative-spike-experiments-v3")
}
