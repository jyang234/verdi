package refindex

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/specstate"
)

// mapStatusGroup maps a COMPONENT-class default-branch spec's raw
// frontmatter status field to dc-2's four-value StatusGroup vocabulary,
// through a total function that fails closed (returns an error, never a
// silent default bucket) for a status value it does not recognize —
// CLAUDE.md: "unknown enum values fail closed" (ac-3's static obligation).
//
// Task 6a narrows this function's live callers to class: component entries
// only (refindex.go's computeDefaultBranchEntries): component specs are
// "authored-living, never frozen" — draft/active/superseded persisted
// display-only vocabulary this package leaves untouched, never git-derived
// (CLAUDE.md: "leave display-only vocabulary and component-status logic
// intact") — unlike class: feature/story, whose default-branch StatusGroup
// now routes through effectiveStatusGroup below (internal/specstate: "merge
// IS acceptance", so trusting a persisted status: field alone is exactly
// the defect that migration fixes). The switch still covers every value
// legal on ANY class (accepted-pending-build/closed included) defensively,
// even though a component spec's own validation never produces them.
func mapStatusGroup(status artifact.Status) (StatusGroup, error) {
	switch status {
	case "draft":
		return StatusGroupDraftsInProgress, nil
	case "accepted-pending-build":
		return StatusGroupAcceptedPendingBuild, nil
	case "active":
		return StatusGroupActiveComponents, nil
	case "closed", "superseded":
		return StatusGroupTerminal, nil
	default:
		return "", fmt.Errorf("refindex: spec status %q does not map to any known StatusGroup (fail-closed)", status)
	}
}

// effectiveStatusGroup maps a class: feature/story default-branch entry's
// specstate-resolved Result to dc-2's StatusGroup vocabulary (Task 6a):
// AcceptedPendingBuild lands in the accepted bucket; Superseded and Closed
// both land in Terminal (both are permanent, non-actionable end states from
// a directory-listing reader's point of view — the same Terminal bucket
// mapStatusGroup's own "closed", "superseded" case already grouped
// together). Every other state — Unproven (the default branch or a
// required ancestry witness could not be resolved) and, defensively,
// Proposed (never actually reachable here in production: a default-branch
// entry's Candidate.Content is always read FROM the default branch itself,
// so Relation is always Exact — Proposed cannot occur — but the switch
// still handles it, never a fail-open assumption) — resolves to
// DraftsInProgress, the safe "not yet certified accepted" bucket, carrying
// a Disclosure: an unproven entry must never silently enter the accepted
// group (Task 6 brief's own ac).
//
// A PROVEN state with non-empty Result.Disclosures — the projector's
// compatibility notes (a legacy `status: draft` landed and reported
// accepted, a legacy terminal status honored from the field alone) —
// carries those disclosures too (final fix wave I5): the projector spoke
// them for a reason, and this seam stripping them silenced the
// compatibility story on every directory surface. Distinct source id, so
// a reader can tell "unproven" from "proven, with a compatibility note".
func effectiveStatusGroup(ref string, r specstate.Result) (StatusGroup, *disclosure.Disclosure) {
	switch r.State {
	case specstate.AcceptedPendingBuild:
		return StatusGroupAcceptedPendingBuild, provenStateDisclosure(ref, r)
	case specstate.Superseded, specstate.Closed:
		return StatusGroupTerminal, provenStateDisclosure(ref, r)
	default: // specstate.Unproven, and defensively specstate.Proposed
		text := "specstate could not prove this entry's effective lifecycle state"
		if len(r.Disclosures) > 0 {
			text = joinDisclosures(r.Disclosures)
		}
		d := disclosure.New("refindex:unproven-spec-state", ref, text)
		return StatusGroupDraftsInProgress, &d
	}
}

// provenStateDisclosure carries a proven state's own compatibility
// disclosures forward (nil when the projector disclosed nothing — the
// ordinary clean-spec case, unchanged).
func provenStateDisclosure(ref string, r specstate.Result) *disclosure.Disclosure {
	if len(r.Disclosures) == 0 {
		return nil
	}
	d := disclosure.New("refindex:spec-state-compatibility", ref, joinDisclosures(r.Disclosures))
	return &d
}

// joinDisclosures renders specstate's own (already sorted, already
// deterministic) Disclosures slice as one line — Disclosure.Text is a
// single string, never a slice, so multiple specstate witnesses are
// flattened here rather than dropped.
func joinDisclosures(lines []string) string {
	out := lines[0]
	for _, l := range lines[1:] {
		out += "; " + l
	}
	return out
}
