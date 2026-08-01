package residue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// activeSpec is one active-zone spec.md's decoded frontmatter, keyed by
// its directory name — the "<name>" AC-1/AC-2 name close/<name> branches
// and stubs[] slugs by (02 §Identity and references: the directory name
// is the spec's own name segment).
type activeSpec struct {
	Name string
	FM   *artifact.SpecFrontmatter
	// Content is spec.md's raw, undecoded bytes exactly as read from the
	// working tree — kept alongside FM so effectiveStates (below) can hand
	// it straight to specstate as a Candidate's in-hand content without a
	// second filesystem read. Never a git-plumbing read: Candidate.Content
	// is explicitly allowed to be working-tree content (specstate/state.go:
	// "working-tree content, a PR head's content, or anything else").
	Content []byte
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
		specs = append(specs, activeSpec{Name: e.Name(), FM: decoded, Content: data})
	}

	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs, nil
}

// effectiveStates is the map PRODUCER (Task 6a): the ONE place
// activespecs.go, patterna.go, and patternb.go's own status decisions all
// route through, resolving every active-zone spec's git-derived effective
// lifecycle state (internal/specstate — "no adapter reimplements
// reachability, and none trusts a persisted status: field alone") in a
// SINGLE ResolveMany batch call, never one Resolve call per spec (the same
// O(specs²)-avoiding discipline internal/refindex's ComputeIndex applies).
// Every pure fold downstream (excludeSuperseded, supersededNames,
// patterna.findPatternA, patternb.findPatternB's own accepted-pending-build
// gate) consumes the returned map by simple lookup — never touching Git or
// specstate itself, so teaching a pure fold about Git is never required.
//
// A statusless active-zone spec (Task 4's live scaffold) whose exact bytes
// are already committed on the default branch resolves to
// specstate.AcceptedPendingBuild here — never silently excluded the way a
// raw `FM.Status != "accepted-pending-build"` string comparison would
// exclude it (the defect this migration fixes: a stub-complete statusless
// feature now correctly participates in AC-1 pattern (b)).
func effectiveStates(ctx context.Context, root string, specs []activeSpec) (map[string]specstate.State, error) {
	out := make(map[string]specstate.State, len(specs))
	if len(specs) == 0 {
		return out, nil
	}

	candidates := make([]specstate.Candidate, len(specs))
	for i, s := range specs {
		candidates[i] = specstate.Candidate{
			Path:    store.SpecRelPath(store.ZoneActive, s.Name),
			Content: s.Content,
		}
	}

	results, err := specstate.NewProjector().ResolveMany(ctx, root, candidates)
	if err != nil {
		return nil, fmt.Errorf("residue: resolving effective spec state: %w", err)
	}
	for i, s := range specs {
		out[s.Name] = results[i].State
	}
	return out, nil
}

// excludeSuperseded returns the subset of specs whose EFFECTIVE state
// (effective, effectiveStates' own output — dc-2: status: superseded is
// excluded BEFORE either AC-1 pattern's logic runs, a check that happens
// first, not a state that merely happens never to match either pattern's
// own conditions) is not specstate.Superseded. Remaining in specs/active/
// while effectively superseded is correct, permanent behavior (02 §Kind
// registry) and never a finding. A pure fold: no ctx, no Git — every Git
// read already happened in effectiveStates.
func excludeSuperseded(specs []activeSpec, effective map[string]specstate.State) []activeSpec {
	var out []activeSpec
	for _, s := range specs {
		if effective[s.Name] == specstate.Superseded {
			continue
		}
		out = append(out, s)
	}
	return out
}

// activeClassByName indexes specs by name to their raw class string
// ("feature" or "story") — patterna.go's own lookup, so a PatternA
// finding can carry the spec's declared class for the renderer's display-
// chain resolution (spec/vocabulary-surfaces) without re-reading spec.md.
// Class is not a lifecycle status (component-status-shaped, display-only
// vocabulary a spec declares about itself, never git-derived) so it is
// untouched by the effective-state migration.
func activeClassByName(specs []activeSpec) map[string]string {
	m := make(map[string]string, len(specs))
	for _, s := range specs {
		m[s.Name] = string(s.FM.Class)
	}
	return m
}

// supersededNames is the set of active-zone spec names whose EFFECTIVE
// state is specstate.Superseded — dc-2's AC-2 exclusion lookup
// (closebranches.go's scanCloseBranches), built from the UNFILTERED
// active-spec set (this is the one place that must see superseded specs,
// not the excludeSuperseded subset already filtered for AC-1's own two
// patterns). A pure fold over effective, exactly like excludeSuperseded.
func supersededNames(specs []activeSpec, effective map[string]specstate.State) map[string]bool {
	m := make(map[string]bool, len(specs))
	for _, s := range specs {
		if effective[s.Name] == specstate.Superseded {
			m[s.Name] = true
		}
	}
	return m
}
