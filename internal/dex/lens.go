// The v2 lens inputs (V1-P8, 05 §Lenses): everything the feature-lens
// section, the story-page ladder badges, and the per-ADR exemption pages
// render, computed ONCE per build from the same packages the CLI verbs
// use — decisionsweep's exemption scan and spec-stale scan, and
// evidence's pending-supersession fold — never a dex-private
// re-derivation ("computed the same way — no separate logic path").
package dex

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/decisionsweep"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// pendingState is one story ref's pending-supersession outcome — a proper
// three-valued record (03 §The amendment ladder's race-window flag +
// CLAUDE.md's three-valued honesty): flagged-with-witness, proven
// unflagged, or disclosed-unproven when open MRs could not be enumerated.
type pendingState struct {
	// Unproven is true when the story implements at least one feature but
	// no forge (or no default branch) was available to enumerate open
	// supersession MRs — the badge does NOT render (unproven is not
	// flagged), and the page discloses it instead of silently passing.
	Unproven bool
	Result   evidence.PendingSupersessionResult
}

// lensData is the build-wide computed input set for the v2 page sections.
type lensData struct {
	// exemptions is decisionsweep.ScanExemptions' per-ADR backlink set,
	// keyed by unpinned ADR ref ("adr/<name>").
	exemptions map[string]*decisionsweep.ExemptionCount
	// staleByRef maps a story spec ref to its computed spec-stale result
	// (absent = no deviation report on disk = trivially unflagged, the
	// same skip rule `verdi audit` applies).
	staleByRef map[string]evidence.SpecStaleResult
	// pendingByRef maps a story spec ref to its pending-supersession
	// state; absent = the story implements no feature (nothing to prove).
	pendingByRef map[string]pendingState
}

// computeLensData runs the corpus-wide scans. It builds a lint.Snapshot —
// the exact input `verdi audit` scans — and, when a forge is available,
// probes each implemented feature's conventional candidate path
// (.verdi/specs/active/<name>-v2/spec.md, R4-I-14) on every open MR
// against defaultBranch, exactly as the closure gate's
// pending-supersession condition does (cmd/verdi/closuregate.go).
func computeLensData(ctx context.Context, root string, f forge.Forge, defaultBranch string, pages []*artifactPage) (*lensData, error) {
	snap, err := lint.BuildSnapshot(root, lint.Options{})
	if err != nil {
		return nil, fmt.Errorf("dex: building corpus snapshot: %w", err)
	}

	threshold := 0
	if snap.Manifest != nil && snap.Manifest.Audit != nil {
		threshold = snap.Manifest.Audit.DeviationsStaleThreshold
	}
	staleEntries, err := decisionsweep.ScanSpecStale(root, snap, threshold)
	if err != nil {
		return nil, fmt.Errorf("dex: scanning spec-stale: %w", err)
	}
	staleByRef := make(map[string]evidence.SpecStaleResult, len(staleEntries))
	for _, e := range staleEntries {
		staleByRef[e.StoryRef] = e.Result
	}

	pendingByRef, err := computePendingStates(ctx, f, defaultBranch, pages)
	if err != nil {
		return nil, err
	}

	return &lensData{
		exemptions:   decisionsweep.ScanExemptions(snap),
		staleByRef:   staleByRef,
		pendingByRef: pendingByRef,
	}, nil
}

// computePendingStates folds every story-class page's implements edges
// against the open supersession MRs of each feature it implements.
// Candidates are loaded once per feature (cached), in sorted feature
// order, so the fold — and therefore the built bytes — are deterministic
// for a given forge state.
func computePendingStates(ctx context.Context, f forge.Forge, defaultBranch string, pages []*artifactPage) (map[string]pendingState, error) {
	out := make(map[string]pendingState)
	candidatesByFeature := make(map[string][]evidence.OpenSupersessionCandidate)

	for _, p := range pages {
		if !isStoryPage(p) {
			continue
		}
		byFeature := evidence.ImplementsByFeature(p.Entry.Links)
		if len(byFeature) == 0 {
			continue // implements no feature: nothing to prove, no state
		}
		if f == nil || defaultBranch == "" {
			out[p.Entry.Ref] = pendingState{Unproven: true}
			continue
		}

		featureNames := make([]string, 0, len(byFeature))
		for n := range byFeature {
			featureNames = append(featureNames, n)
		}
		sort.Strings(featureNames)

		var merged evidence.PendingSupersessionResult
		for _, featureName := range featureNames {
			candidates, ok := candidatesByFeature[featureName]
			if !ok {
				candidatePath := store.ActiveSpecRelPath(featureName + "-v2")
				var err error
				candidates, err = evidence.LoadPendingSupersessionCandidates(ctx, f, defaultBranch, "spec/"+featureName, candidatePath)
				if err != nil {
					return nil, fmt.Errorf("dex: loading pending-supersession candidates for %s: %w", featureName, err)
				}
				candidatesByFeature[featureName] = candidates
			}
			r := evidence.PendingSupersession(evidence.PendingSupersessionInput{ObjectIDs: byFeature[featureName], Candidates: candidates})
			if r.Flagged {
				merged.Flagged = true
				merged.Touched = append(merged.Touched, r.Touched...)
				merged.MRIDs = append(merged.MRIDs, r.MRIDs...)
			}
		}
		sort.Strings(merged.Touched)
		sort.Strings(merged.MRIDs)
		out[p.Entry.Ref] = pendingState{Result: merged}
	}
	return out, nil
}

// isStoryPage reports whether p is a round-four story-class spec page.
func isStoryPage(p *artifactPage) bool {
	return p.Entry.Kind == "spec" && p.Meta.Class == artifact.ClassStory
}

// isRoundFourFeaturePage mirrors cmd/verdi's feature discriminator
// (featurematrix.go: class feature AND carrying problem/outcome — a
// grandfathered v0 "feature" spec has Problem == nil and gets no lens
// section, keeping its v0 page byte-stable).
func isRoundFourFeaturePage(p *artifactPage) bool {
	return p.Entry.Kind == "spec" && p.Meta.Class == artifact.ClassFeature && p.Meta.Problem != nil
}
