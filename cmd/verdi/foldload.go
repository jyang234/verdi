// foldStoryEvidence (spec/shared-homes ac-5, co-3: lands in a NEW file
// rather than moving code out of the sites it replaces) is the shared
// story-level fold-load prologue close.go, closuregate.go, gate.go, and
// matrix.go each repeated near-verbatim: join the story's derived
// evidence directory, load its records at commit, then fold them via
// internal/evidence.Fold. preview is the one real parameter across the
// four sites — close.go/closuregate.go/gate.go always pass false (co-1:
// the closure gate and merge gate fold ONLY source: ci evidence, never
// the --preview escape hatch); matrix.go threads its own --preview flag
// through.
//
// This helper does its own error wrapping ("loading evidence records:
// %w" / "folding evidence: %w") matching close.go's and gate.go's
// existing, unprefixed wording exactly. A call site whose own wrapping
// differs only by an outer prefix (closuregate.go's "closure gate: ")
// wraps this helper's returned error again at the call site rather than
// this file guessing at every site's phrasing — dc-4's "bit-for-bit
// identical" bar applies to error text too.
package main

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/store"
)

// foldStoryEvidence loads spec's derived evidence records at commit and
// folds them (internal/evidence.Fold), consulting spec's own story-slug
// waivers/attestations. preview mirrors evidence.Input.Preview — false
// folds source:ci only, true also folds source:local (advisory) records.
func foldStoryEvidence(ctx context.Context, root string, spec *artifact.SpecFrontmatter, commit string, preview bool) (evidence.StoryResult, error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		return evidence.StoryResult{}, fmt.Errorf("loading evidence records: %w", err)
	}
	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: preview, StoreRoot: root, StorySlug: slug})
	if err != nil {
		return evidence.StoryResult{}, fmt.Errorf("folding evidence: %w", err)
	}
	return result, nil
}
