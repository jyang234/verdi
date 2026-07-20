package residue

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// PatternB is AC-1 pattern (b)'s finding: a stub-complete unclosed
// feature — every declared stubs[] slug already realized by a closed,
// merged story, yet the feature itself has not closed.
type PatternB struct {
	SpecName string
	Stubs    []string // realized stub slugs, sorted
}

// findPatternB scans specs (dc-2's superseded exclusion already applied by
// the caller) for class: feature status: accepted-pending-build specs whose
// every declared stubs[] slug is realized by a closed, MERGED story —
// AC-1(b)'s own words. Realization is evaluated against defaultTip, the
// audited default-branch tip: .verdi/specs/archive/<slug>/spec.md present
// AT defaultTip carrying status: closed, read via git plumbing (the same
// audited-ref mechanics AC-2 uses), never the working tree. So a stub whose
// archive move rides only an unmerged close/<slug> branch is correctly NOT
// counted as realized — the merged condition AC-1(b) names, which a
// working-tree read silently drops.
//
// The feature spec itself is read from the active zone (walkActiveSpecs,
// the working tree) like the rest of the scan; only the per-stub
// realization check reads the audited ref. A feature declaring no stubs has
// nothing to reconcile and never fires — pattern (b) names a
// "stub-COMPLETE" feature, which presupposes a non-empty stub set to be
// complete.
func findPatternB(ctx context.Context, root, defaultTip string, specs []activeSpec) ([]PatternB, error) {
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
			closed, err := archiveSpecClosedAt(ctx, root, defaultTip, stub.Slug)
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

// archiveSpecClosedAt reports whether .verdi/specs/archive/<slug>/spec.md is
// present at ref (the audited default-branch tip) AND decodes with status:
// closed — pattern (b)'s per-stub realization check, evaluated against the
// audited ref via git plumbing (never the working tree), so an archive move
// that has not merged into the default branch is not counted as a realized,
// merged story. It reuses AC-1/AC-2's shared archiveExistsAt presence check,
// then reads the file's content at ref with gitx.Show.
//
// Absent at ref: not realized (false, no error — the ordinary unrealized
// case, exactly as an absent path is for archiveExistsAt). Present at ref
// but undecodable: a disclosed operational error, never a silent false or a
// guessed third path — an author would want to know the archived spec.md is
// broken, not have it read as "not yet realized".
func archiveSpecClosedAt(ctx context.Context, root, ref, slug string) (bool, error) {
	relPath := store.SpecRelPath(store.ZoneArchive, slug)
	present, err := archiveExistsAt(ctx, root, ref, slug)
	if err != nil {
		return false, fmt.Errorf("residue: checking %s at %s: %w", relPath, ref, err)
	}
	if !present {
		return false, nil
	}
	data, err := gitx.Show(ctx, root, ref, relPath)
	if err != nil {
		return false, fmt.Errorf("residue: reading %s at %s: %w", relPath, ref, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return false, fmt.Errorf("residue: %s at %s: %w", relPath, ref, err)
	}
	decoded, err := artifact.DecodeSpec(fm)
	if err != nil {
		return false, fmt.Errorf("residue: %s at %s: %w", relPath, ref, err)
	}
	return decoded.Status == "closed", nil
}
