// verdi close --preflight <feature-spec> — the feature half of --preflight
// (spec/close-preflight dc-3, ADJ-33-ratified widening of closure-
// ergonomics ac-1's literal "a named story" text: the shared
// runFeatureClosureGate already exists, is already pure until its own
// caller mutates, and Phase 4's feature closes need co-3 as much as story
// closes). Mirrors runStoryPreflightGate (closepreflight.go) in shape but
// recomputes EXACTLY runCloseFeature's (closefeature.go) own prologue —
// discoverImplementingStories, foldFeature, reconcileFeatureStubs, all
// already pure reads — before calling the identical runFeatureClosureGate
// function (dc-2) close.go:170-172 dispatches to for a feature-class
// target.
package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// runFeaturePreflightGate runs the SAME evaluation function a real
// feature-class `verdi close` calls first (runFeatureClosureGate,
// closuregatefeature.go — dc-2), printing its unchanged
// PASS/FAIL/disclosed "closure(feature):" lines, then enriches condition
// 1's per-AC breakdown the same way the story path does (unmetACDetail,
// closepreflight.go) — keyed by the feature spec's own Name (dc-6's
// FeatureSlug convention, never the story-scope StorySlug helper, "the
// single easiest correctness mistake a fold-reusing implementation could
// make"). Conditions 2/3 (stub reconciliation, implementing stories closed)
// already name their own unreconciled slugs/still-open refs (dc-2: "already
// itemized enough") and need no enrichment; conditions 4/5 (spec-stale,
// pending-supersession) are the identical checkSpecStaleCondition/
// checkPendingSupersessionCondition the story gate calls, so
// printSpecStalePathIfFailing (closepreflight.go) applies unchanged.
func runFeaturePreflightGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest, mdl *model.Model, f forge.Forge, defaultBranchRef, head string, stdout io.Writer) (bool, error) {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: internal error: resolved spec has an invalid id: %w", err)
	}

	ix, err := index.Build(root)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	stories, storiesByAC, _, err := discoverImplementingStories(ctx, root, head, ix, specRef.Name, spec)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	fold, err := foldFeature(ctx, root, spec, specRef, head, storiesByAC)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	reconciliation, err := reconcileFeatureStubs(spec, stories)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}

	ok, err := runFeatureClosureGate(ctx, root, spec, fold, reconciliation, stories, f, defaultBranchRef, manifest, mdl, stdout)
	if err != nil {
		return false, err
	}

	derivedRel, excluded, err := preflightDerivedContext(ctx, root, spec.ID, head)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	printACDetail(stdout, unmetFeatureACDetail(fold.ACs, specRef.Name, derivedRel, excluded))

	if err := printSpecStalePathIfFailing(stdout, root, spec, manifest); err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}

	return ok, nil
}

// unmetFeatureACDetail renders the feature outcome-floor detail for every
// unmet feature AC, consuming the fold's OWN floor evaluation
// (evidence.FeatureACResult.Floor) — the OR-across-signals semantics, never
// the story fold's AND-across-declared-kinds (ADJ-56 finding 2): a floor
// already satisfied via one signal prints NO remedy at all, even for an AC
// still unmet for another reason (its implementing story open, which the
// gate's own conditions 2/3 already name). The feature fold carries no waived
// status (03 §The feature fold), so evidenced is the only met outcome. slug
// is the FeatureSlug (spec's own Name, dc-6), never the story-scope helper.
func unmetFeatureACDetail(acs []evidence.FeatureACResult, slug, derivedRel string, excluded []string) []string {
	var lines []string
	for _, ac := range acs {
		if ac.Status == evidence.StatusEvidenced {
			continue
		}
		if line := renderFeatureFloorGap(ac.ID, ac.Floor, slug, derivedRel, excluded); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// renderFeatureFloorGap maps one feature AC's evaluated outcome floor to its
// disclosure line (or "" when the floor is already cleared — ADJ-56 finding
// 2's exact requirement). A violated floor names its failing witness (finding
// 3); an unsatisfied floor names the two disjunctive ways to clear it — an
// authored outcome attestation at the FeatureSlug path (dc-6/dc-7) OR any
// passing outcome record under the derived-tree root — never a per-kind AND.
func renderFeatureFloorGap(acID string, floor evidence.FloorResult, slug, derivedRel string, excluded []string) string {
	if floor.Satisfied {
		return ""
	}
	if floor.Violating != nil {
		return fmt.Sprintf("%s outcome floor: current outcome record FAILED (witness %q); fix or supersede it — derived-tree root probed: %s", acID, floor.Violating.Witness, derivedRel)
	}

	excludedNote := ""
	if len(excluded) > 0 {
		excludedNote = fmt.Sprintf(" (found but excluded as non-ancestor: %v)", excluded)
	}
	if floor.DeclaresAttestation {
		path := filepath.ToSlash(evidence.AttestationPath("", slug, acID))
		if floor.Attestation == evidence.AttestationUnauthored {
			return fmt.Sprintf("%s outcome floor unsatisfied: a scaffold is present at %s but the claim is unauthored (sentinel present) — author it, or provide any passing outcome record under %s", acID, path, derivedRel) + excludedNote
		}
		return fmt.Sprintf("%s outcome floor unsatisfied: needs an authored outcome attestation at %s, or any passing outcome record under %s", acID, path, derivedRel) + excludedNote
	}
	return fmt.Sprintf("%s outcome floor unsatisfied: needs any passing outcome record under %s", acID, derivedRel) + excludedNote
}
