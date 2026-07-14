package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
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
		// The evidence-slot join (spec/evidence-slot ac-3/dc-2): each
		// declared kind's fold-derived record state lands ON the card's
		// existing per-kind obligation row (attachObligations built one
		// view per declared kind, in the same declared order the slot
		// compute walked) — never a second per-kind list.
		if states, ok := badges.EvidenceSlots[proj.Cards[i].ID]; ok {
			mergeSlotStates(proj.Cards[i].Obligations, states)
		}
	}
	for i := range proj.StubViews {
		key := "stub:" + proj.StubViews[i].Slug
		if recs, ok := badges.ByObject[key]; ok {
			proj.StubViews[i].Badges = badgeViewsFrom(recs)
		}
	}
	caseFile := badges.CaseFile
	// Ladder disclosures are CASE-FILE lines, not board-chrome notices
	// (spec/case-file-flags dc-4): a disclosed-unproven outcome renders on
	// the case-file lockup in the board's notice vocabulary — where the
	// stamp it stands in for would hang — never as a stamp, never silent.
	caseFileDisclosures := append([]string{}, badges.Disclosures...)

	// The judged-findings case-file chip (spec/derivation-drawer ac-3,
	// dc-2): the spec's own decision-conflict report surfaced on the case
	// file, wearing its sweep provenance and dc-3's staleness comparisons.
	// Added here — the one attachment point every wall badge shares (badge-
	// computes dc-1) — so the page, the fragment, and get_board all carry
	// it identically. Its three-valued outcome mirrors the ladder badges':
	// chip, disclosed-unproven case-file line (an unreadable report), or
	// nothing (no report — absence of a sweep is not a finding). The
	// disclosure rides CaseFileDisclosures, not Notices — same case-file-
	// flags dc-4 posture as the ladder disclosures above: it stands in for
	// a chip on the case-file lockup, not board chrome.
	judged, judgedDisclosure, err := wallbadge.JudgedSweepBadge(ctx, root, specName, specRevision, fm, gitCoversResolver{root: root})
	if err != nil {
		return fmt.Errorf("workbench: computing judged-sweep badge for %s: %w", specName, err)
	}
	if judged != nil {
		caseFile = append(caseFile, *judged)
	}
	if judgedDisclosure != "" {
		caseFileDisclosures = append(caseFileDisclosures, judgedDisclosure)
	}

	proj.CaseFileBadges = badgeViewsFrom(caseFile)
	proj.CaseFileDisclosures = append(proj.CaseFileDisclosures, caseFileDisclosures...)
	return nil
}

// gitCoversResolver is wallbadge.CoversResolver over the store's own git
// checkout (04 §port pattern: the interface lives at its consumer,
// internal/wallbadge; this is the gitx-backed adapter). A covers sha that
// does not exist in this checkout — or a spec path absent at it — is the
// port's disclosed-unproven case (ok=false), never an error: dc-3's
// comparison then discloses its own inability instead of claiming a
// mismatch.
type gitCoversResolver struct {
	root string
}

func (g gitCoversResolver) SpecDigestAtCommit(ctx context.Context, commit, relPath string) (string, bool, error) {
	exists, err := gitx.CommitExists(ctx, g.root, commit)
	if err != nil || !exists {
		// A root that is not a git repository at all surfaces as err here:
		// disclosed-unproven (the comparison cannot be made), never a
		// wall-breaking failure — the drawer is a reading aid (co-2).
		return "", false, nil
	}
	data, err := gitx.Show(ctx, g.root, commit, relPath)
	if err != nil {
		return "", false, nil // path absent at the pinned commit: unprovable, disclosed
	}
	return contentDigest(data), true, nil
}

// mergeSlotStates folds one AC card's computed slot states onto its
// existing obligation views, matched by declared kind — the join that
// keeps demand and holdings ONE row per kind (spec/evidence-slot ac-3):
// no view is added or removed here, only enriched. A kind with no
// matching state (impossible when both sides walk the same declared
// list, but never assumed) keeps Slot == "" and renders chip-free rather
// than guessing.
func mergeSlotStates(views []obligationView, states []wallbadge.SlotState) {
	for i := range views {
		for _, st := range states {
			if st.Kind != views[i].Kind {
				continue
			}
			if st.Empty {
				views[i].Slot = "empty"
			} else {
				views[i].Slot = "held"
			}
			views[i].SlotRecords = st.Records
			break
		}
	}
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
			Provenance:  r.Provenance,
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
