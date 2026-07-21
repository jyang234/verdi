// Per-record evidence disclosures shared by the verdict-consumption gates
// (spec/evidence-resilience ac-2; the endgame disclosure-extension work item).
// One fold feeds every gate, so every gate that CONSUMES a fold verdict must
// render the same three-valued honesty over the records the fold excluded or
// could-not-prove — quarantined, undecodable, and kept-but-unprovable records —
// rather than only the closure gate doing so (constitution 2/10: silence is
// never a pass; an exclusion that changes a verdict is disclosed on the surface
// that consumed it, never silently). The three verdict-consumption surfaces are:
//
//   - the STORY closure gate (closuregate.go's checkClosureEligible),
//   - the STORY MERGE gate (gate.go's checkNoACViolated), and
//   - the FEATURE closure gate (closuregatefeature.go's checkFeatureFoldEligible),
//
// plus `verdi close --preflight`, which reaches every one of them unchanged
// through the identical evaluation functions (closepreflight.go). Each attaches
// evidenceDisclosures' lines to its condition's Extra, printed on EVERY verdict
// (PASS, FAIL, disclosed alike).
//
// SCOPE BOUNDARY (standing ledgered ruling, matrix/rollup stay OUT): `verdi
// matrix` and the rollup writer are PROJECTION surfaces, not verdict-consumption
// surfaces — they render a fold's per-AC status for a human to read, never gate a
// close/merge decision on it, so an excluded record there changes no verdict and
// needs no per-record disclosure line here. The verdict-consumption surfaces
// above are the complete set; do not extend evidenceDisclosures to matrix/rollup.
package main

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/store"
)

// foldedAC is the minimal per-AC projection quarantineDisclosures needs: an AC's
// id and its FOLDED status. Both the story fold (evidence.ACResult) and the
// feature fold (evidence.FeatureACResult) reduce to it via the adapters below, so
// the ONE met-AC/fail-record disclosure rule (judged-quarantine-disclosure-met-ac)
// serves the story closure gate, the merge gate, and the feature closure gate
// identically rather than being copy-pasted per fold shape.
type foldedAC struct {
	ID     string
	Status evidence.Status
}

// storyFoldedACs projects a story fold's per-AC results onto foldedAC.
func storyFoldedACs(acs []evidence.ACResult) []foldedAC {
	out := make([]foldedAC, len(acs))
	for i, ac := range acs {
		out[i] = foldedAC{ID: ac.ID, Status: ac.Status}
	}
	return out
}

// featureFoldedACs projects a feature fold's per-AC results onto foldedAC. A
// feature AC never carries the waived status (03 §The feature fold's four-status
// table has no waived), so quarantineDisclosures' met check reduces to "evidenced"
// for a feature — the same function serves both fold shapes without a branch.
func featureFoldedACs(acs []evidence.FeatureACResult) []foldedAC {
	out := make([]foldedAC, len(acs))
	for i, ac := range acs {
		out[i] = foldedAC{ID: ac.ID, Status: ac.Status}
	}
	return out
}

// evidenceDisclosures gathers the per-record disclosed-unproven lines a
// verdict-consumption gate renders over a spec's derived evidence at head:
// quarantined records naming an AC (quarantineDisclosures), undecodable record
// files (undecodableDisclosures), and kept-but-unprovable records
// (unprovableDisclosures) — one gather, in one order, so the story closure gate,
// the merge gate, and the feature closure gate can never disclose different
// things over the same fold. views is the caller's own fold, already computed
// (storyFoldedACs / featureFoldedACs). It changes no verdict — every record it
// names the fold already excluded or already kept; this only makes that legible.
func evidenceDisclosures(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head string, views []foldedAC) ([]string, error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	quarantined, undecodable, qErr := evidence.QuarantinedRecords(ctx, root, derivedRoot, head)
	if qErr != nil {
		return nil, qErr
	}
	unprovable, uErr := evidence.UnprovableRecords(ctx, root, derivedRoot, head)
	if uErr != nil {
		return nil, uErr
	}
	var lines []string
	lines = append(lines, quarantineDisclosures(views, quarantined)...)
	lines = append(lines, undecodableDisclosures(undecodable)...)
	lines = append(lines, unprovableDisclosures(unprovable)...)
	return lines, nil
}

// quarantineReason returns the reason to disclose for an excluded record:
// the actual reason `verdi sync` recorded on it (artifact.Evidence.Quarantine,
// ac-1) when present, else a generic reachability statement for a record this
// story's own build never had the chance to quarantine (hand-placed derived
// data, or evidence synced before this story landed).
func quarantineReason(rec artifact.Evidence) string {
	if rec.Quarantine != nil && rec.Quarantine.Reason != "" {
		return rec.Quarantine.Reason
	}
	return fmt.Sprintf("provenance.commit %s is not reachable from HEAD", rec.Provenance.Commit)
}

// quarantineDisclosures renders one disclosed-unproven line (spec/
// evidence-resilience ac-2) per (AC, excluded record naming that AC) pair the
// verdict rests on:
//
//   - an UNMET AC discloses EVERY excluded record naming it — a reader sees WHY
//     the AC still is not evidenced (a record that would have evidenced it was
//     excluded as unreachable), rather than reading the gap as if no evidence was
//     ever produced.
//   - a MET AC (evidenced/waived) discloses ONLY an excluded record that carried
//     verdict:fail (judged-quarantine-disclosure-met-ac). That is the record
//     whose exclusion could have flipped the AC violated->evidenced (fold.go's
//     any-current-fail rule): the exclusion is exactly why the AC is not violated,
//     so on a MET AC it is the load-bearing thing to name. A non-fail record
//     excluded from an already-met AC changed no verdict (the AC is met on the
//     records that DID count), so disclosing it would be noise, not honesty —
//     kept out to keep the output legible, the disclose-all latitude the finding
//     grants ("acceptable if output stays legible").
//
// A fail record is named as adverse ("recorded verdict fail"), never as would-be
// evidence, since a fail record would have VIOLATED the AC, not evidenced it; and
// when the AC does not itself read violated, the line states that the exclusion
// is why — the anti-silent-flip disclosure both the closure gate and the merge
// gate render. Prefers the sync-recorded quarantine reason (quarantineReason).
func quarantineDisclosures(acs []foldedAC, quarantined []artifact.Evidence) []string {
	var lines []string
	for _, ac := range acs {
		met := ac.Status == evidence.StatusEvidenced || ac.Status == evidence.StatusWaived
		for _, rec := range evidence.RecordsForAC(quarantined, ac.ID) {
			adverse := rec.Verdict == artifact.VerdictFail
			if met && !adverse {
				continue // a non-fail record excluded from an already-met AC changed no verdict.
			}
			reason := quarantineReason(rec)
			var text string
			switch {
			case adverse && ac.Status != evidence.StatusViolated:
				// The AC does NOT read violated, yet an excluded record recorded a
				// fail for it — the exclusion is exactly why it is not violated.
				// Name it so the quarantine-caused violated->non-violated flip is
				// never silent (judged-quarantine-disclosure-met-ac,
				// judged-merge-gate-quarantine-silence).
				text = fmt.Sprintf("a %s record (witness %q) that recorded verdict fail for %s was excluded, so %s does not read violated (folded %s): %s", rec.Kind, rec.Witness, ac.ID, ac.ID, ac.Status, reason)
			case adverse:
				// ac.Status already reads violated (another current fail stands);
				// this is just additional excluded adverse evidence.
				text = fmt.Sprintf("a %s record (witness %q) that recorded verdict fail for %s was excluded (%s already reads violated on other evidence): %s", rec.Kind, rec.Witness, ac.ID, ac.ID, reason)
			default:
				text = fmt.Sprintf("a %s record (witness %q) that would have evidenced %s was excluded: %s", rec.Kind, rec.Witness, ac.ID, reason)
			}
			lines = append(lines, disclosure.Render(disclosure.New("gate:evidence-quarantine", ac.ID, text)))
		}
	}
	return lines
}

// undecodableDisclosures renders one disclosed-unproven line per record file that
// failed strict decode (spec/evidence-resilience ac-2, finding 2). Disclosed
// unconditionally — not per-AC, the way quarantineDisclosures is — because an
// undecodable file cannot be read to learn which AC its records would have
// evidenced; disclosing it at all is what keeps the debris from passing silently
// while the run stays non-operational.
func undecodableDisclosures(undecodable []evidence.UndecodableFile) []string {
	var lines []string
	for _, u := range undecodable {
		text := fmt.Sprintf("a quarantined evidence record file %s is undecodable and was excluded from the fold: %s", u.Path, u.Reason)
		lines = append(lines, disclosure.Render(disclosure.New("gate:evidence-quarantine", "", text)))
	}
	return lines
}

// unprovableDisclosures renders one disclosed-unproven line per (kept-but-
// unprovable record, AC it evidences) pair (P2-10b): a record the fold COUNTS
// although its provenance.commit could not be proven reachable from HEAD because
// this checkout is shallow. Unlike quarantineDisclosures (excluded records),
// these ACs are typically MET — the record was counted — so this is the only
// surface that names the unprovable ancestry the verdict now rests on, keeping
// the count from passing silently (constitution 2). It changes no verdict: the
// record is kept either way; a full-history checkout would prove it outright.
func unprovableDisclosures(unprovable []artifact.Evidence) []string {
	var lines []string
	for _, rec := range unprovable {
		for _, ac := range rec.EvidenceFor {
			text := fmt.Sprintf("a %s record (witness %q) evidencing %s was counted, but its provenance.commit %s could not be proven reachable from HEAD: this checkout is shallow, and shallow history cannot prove reachability — a full-history checkout proves it", rec.Kind, rec.Witness, ac, rec.Provenance.Commit)
			lines = append(lines, disclosure.Render(disclosure.New("gate:evidence-unprovable", ac, text)))
		}
	}
	return lines
}
