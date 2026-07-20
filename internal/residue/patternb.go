package residue

import (
	"fmt"
	"os"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// PatternB is AC-1 pattern (b)'s finding: a stub-complete unclosed
// feature — every declared stubs[] slug already realized by a closed,
// merged story, yet the feature itself has not closed.
type PatternB struct {
	SpecName string
	Stubs    []string // realized stub slugs, sorted
}

// findPatternB scans specs (dc-2's superseded exclusion already applied
// by the caller) for class: feature status: accepted-pending-build specs
// whose every declared stubs[] slug resolves to an on-disk
// .verdi/specs/archive/<slug>/spec.md carrying status: closed — a
// working-tree check (unlike pattern (a)'s branch-tip git plumbing): a
// realized stub's story has, BY CONSTRUCTION of the real closure ritual,
// already reached specs/archive/ on whatever is checked out, never a
// branch that might not be (dc-1's static obligation: "resolve to an
// ON-DISK .verdi/specs/archive/<slug>/spec.md"). A feature declaring no
// stubs at all has nothing to reconcile and never fires — pattern (b)
// names a "stub-COMPLETE" feature, which presupposes a non-empty stub set
// to be complete.
func findPatternB(root string, specs []activeSpec) ([]PatternB, error) {
	var out []PatternB
	for _, s := range specs {
		if s.FM.Class != artifact.ClassFeature {
			continue
		}
		if s.FM.Status != "accepted-pending-build" {
			continue
		}
		if len(s.FM.Stubs) == 0 {
			continue
		}

		slugs := make([]string, 0, len(s.FM.Stubs))
		allRealized := true
		for _, stub := range s.FM.Stubs {
			closed, err := archiveSpecIsClosed(root, stub.Slug)
			if err != nil {
				return nil, err
			}
			if !closed {
				allRealized = false
				break
			}
			slugs = append(slugs, stub.Slug)
		}
		if !allRealized {
			continue
		}

		sort.Strings(slugs)
		out = append(out, PatternB{SpecName: s.Name, Stubs: slugs})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpecName < out[j].SpecName })
	return out, nil
}

// archiveSpecIsClosed reports whether root's specs/archive/<slug>/spec.md
// exists on disk and decodes with status: closed — pattern (b)'s per-stub
// realization check. A missing archive spec.md is "not realized" (false,
// no error, since an unrealized stub is the ordinary, expected case); a
// present-but-malformed one is a real (operational) error, never a
// silent false — an author would want to know their store has a broken
// spec.md, not have it silently read as "not yet realized".
func archiveSpecIsClosed(root, slug string) (bool, error) {
	path := store.ArchiveSpecPath(root, slug)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("residue: reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return false, fmt.Errorf("residue: %s: %w", path, err)
	}
	decoded, err := artifact.DecodeSpec(fm)
	if err != nil {
		return false, fmt.Errorf("residue: %s: %w", path, err)
	}
	return decoded.Status == "closed", nil
}
