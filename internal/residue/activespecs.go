package residue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
)

// activeSpec is one active-zone spec.md's decoded frontmatter, keyed by
// its directory name — the "<name>" AC-1/AC-2 name close/<name> branches
// and stubs[] slugs by (02 §Identity and references: the directory name
// is the spec's own name segment).
type activeSpec struct {
	Name string
	FM   *artifact.SpecFrontmatter
}

// walkActiveSpecs reads every .verdi/specs/active/<name>/spec.md under
// root, sorted by name. A spec.md that fails to decode is skipped, never a
// hard failure (mirrors internal/decisionsweep.ScanSpecStale's own
// tolerance): this is a corpus-wide audit pass, and one malformed spec
// elsewhere in the store must not sink the closure-hygiene section —
// `verdi lint` is the dedicated tool for surfacing a decode failure
// itself. A store with no specs/active/ directory at all returns (nil,
// nil), not an error (mirrors internal/wtmanager.GC's own "nothing cut
// yet" tolerance).
func walkActiveSpecs(root string) ([]activeSpec, error) {
	base := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("residue: reading %s: %w", base, err)
	}

	var specs []activeSpec
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(base, e.Name(), "spec.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue // a spec directory with no spec.md yet: nothing to scan
			}
			return nil, fmt.Errorf("residue: reading %s: %w", path, err)
		}
		fm, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			continue // tolerant: a malformed spec is verdi lint's finding, not ours
		}
		decoded, err := artifact.DecodeSpec(fm)
		if err != nil {
			continue
		}
		specs = append(specs, activeSpec{Name: e.Name(), FM: decoded})
	}

	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs, nil
}

// excludeSuperseded returns the subset of specs whose status is not
// superseded (dc-2: status: superseded is excluded BEFORE either AC-1
// pattern's logic runs — a check that happens first, not a state that
// merely happens never to match either pattern's own conditions).
// Remaining in specs/active/ under status: superseded is correct,
// permanent behavior (02 §Kind registry) and never a finding.
func excludeSuperseded(specs []activeSpec) []activeSpec {
	var out []activeSpec
	for _, s := range specs {
		if string(s.FM.Status) == "superseded" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// activeStatusByName indexes specs by name to their raw status string —
// pattern (a)'s own lookup (patterna.go): whether <name>, the subject of a
// close/<name> branch, is STILL an active-zone spec at status:
// accepted-pending-build.
func activeStatusByName(specs []activeSpec) map[string]string {
	m := make(map[string]string, len(specs))
	for _, s := range specs {
		m[s.Name] = string(s.FM.Status)
	}
	return m
}

// supersededNames is the set of active-zone spec names CURRENTLY at
// status: superseded — dc-2's AC-2 exclusion lookup (closebranches.go's
// scanCloseBranches), built from the UNFILTERED active-spec set (this is
// the one place that must see superseded specs, not the excludeSuperseded
// subset already filtered for AC-1's own two patterns).
func supersededNames(specs []activeSpec) map[string]bool {
	m := make(map[string]bool, len(specs))
	for _, s := range specs {
		if string(s.FM.Status) == "superseded" {
			m[s.Name] = true
		}
	}
	return m
}
