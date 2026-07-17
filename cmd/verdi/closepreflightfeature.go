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

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/index"
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
func runFeaturePreflightGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest, f forge.Forge, defaultBranchRef, head string, stdout io.Writer) (bool, error) {
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

	ok, err := runFeatureClosureGate(ctx, root, spec, fold, reconciliation, stories, f, defaultBranchRef, manifest, stdout)
	if err != nil {
		return false, err
	}

	unmet := make(map[string]bool, len(fold.ACs))
	for _, ac := range fold.ACs {
		// The feature fold carries no waived status (03 §The feature fold:
		// "there is no waived status at the feature level") — evidenced is
		// the only satisfied outcome.
		if ac.Status != evidence.StatusEvidenced {
			unmet[ac.ID] = true
		}
	}
	lines, err := unmetACDetail(ctx, root, spec.ID, spec.AcceptanceCriteria, unmet, specRef.Name, head)
	if err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}
	printACDetail(stdout, lines)

	if err := printSpecStalePathIfFailing(stdout, root, spec, manifest); err != nil {
		return false, fmt.Errorf("close: --preflight: %w", err)
	}

	return ok, nil
}
