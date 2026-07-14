package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/wallbadge"
)

// attachBadges enriches proj with every computed wall badge (spec/badge-
// computes dc-1): the VL-finding partition (ac-2) on every wall, plus —
// story class only, mirroring internal/dex/lens.go's own isStoryPage gate
// — the spec-stale and pending-supersession ladder badges (ac-3). It
// runs AFTER buildProjection and AFTER attachObligations, in loadBoard's
// I/O enrichment tier — the exact posture attachObligations already
// established (boardspec.go's own doc comment): the projector stays a
// pure function of its four in-memory inputs; this store-derived
// enrichment lives here instead. raw is the spec document's exact bytes,
// already read by loadBoard to build fm — attachBadges hashes them once
// (the spec input's revision, dc-5) rather than re-reading the file.
//
// This is the ONE attachment point for every wall badge (dc-1): a sibling
// wall-receipts story (evidence-slot, case-file-flags) adds its own
// compute here, never a second call site.
func attachBadges(ctx context.Context, proj *BoardProjection, root, specName string, raw []byte, fm *artifact.SpecFrontmatter, superseLoader wallbadge.SupersessionCandidateLoader) error {
	specRelPath := specRelPathFor(specName)
	specRevision := contentDigest(raw)

	badges, err := wallbadge.ComputeBadges(ctx, root, specRelPath, specRevision, fm, superseLoader)
	if err != nil {
		return fmt.Errorf("workbench: computing wall badges for %s: %w", specName, err)
	}

	for i := range proj.Cards {
		if recs, ok := badges.ByObject[proj.Cards[i].ID]; ok {
			proj.Cards[i].Badges = badgeViewsFrom(recs)
		}
	}
	for i := range proj.StubViews {
		key := "stub:" + proj.StubViews[i].Slug
		if recs, ok := badges.ByObject[key]; ok {
			proj.StubViews[i].Badges = badgeViewsFrom(recs)
		}
	}
	proj.CaseFileBadges = badgeViewsFrom(badges.CaseFile)
	// Ladder disclosures are CASE-FILE lines, not board-chrome notices
	// (spec/case-file-flags dc-4): a disclosed-unproven outcome renders on
	// the case-file lockup in the board's notice vocabulary — where the
	// stamp it stands in for would hang — never as a stamp, never silent.
	proj.CaseFileDisclosures = append(proj.CaseFileDisclosures, badges.Disclosures...)
	return nil
}

// badgeViewsFrom converts wallbadge.DerivationRecord values into this
// package's own badgeView shape (projection.go's doc comment: the pure
// projector's file never imports internal/wallbadge, so this I/O-tier
// file does the field-by-field copy instead).
func badgeViewsFrom(recs []wallbadge.DerivationRecord) []badgeView {
	if len(recs) == 0 {
		return nil
	}
	out := make([]badgeView, len(recs))
	for i, r := range recs {
		inputs := make([]badgeInputView, len(r.Inputs))
		for j, in := range r.Inputs {
			inputs[j] = badgeInputView{Name: in.Name, Path: in.Path, Revision: in.Revision}
		}
		out[i] = badgeView{
			Source:      r.Source,
			Label:       r.Label,
			Target:      r.Target,
			Inputs:      inputs,
			Records:     r.Records,
			Disclosures: r.Disclosures,
		}
	}
	return out
}

// specRelPathFor is the store-relative path of an active spec's own
// document — the exact Path every locus-declaring lint.Finding on that
// spec carries (loadBoard only ever serves specs/active/).
func specRelPathFor(specName string) string {
	return ".verdi/specs/active/" + specName + "/spec.md"
}

// contentDigest is "sha256:<hex>" over b — the honest, recomputable
// revision (dc-5) for the spec document input every VL badge cites.
func contentDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
