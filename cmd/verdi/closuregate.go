// The CLOSURE gate (V1-P4, 03 §Gates' "closure gate"): "the story may
// close only when eligible is true, and no unresolved spec-stale or
// pending-supersession flag is present on its edges." Distinct from the
// merge gate above (gate.go): 03 is explicit these two flags "block
// closure, not merge — builds keep moving," so they are NOT folded into
// runGate's condition list. `verdi close` (the verb that would dispatch a
// closure-MR run of these conditions) stays out of this phase's scope —
// see gate.go's doc comment for why this function is deliberately built
// self-contained and unwired rather than invented onto a CLI surface this
// phase does not own.
//
// Condition 4 (X-13/X-16/X-17, added at the round's final fix wave) is a
// tooling addition, not itself named in 03's closure-gate text: it exists
// because `verdi close`'s own freeze step (runAlignForSpec, align.go) has
// exactly two behaviors — freeze the LIVING report in place verbatim when
// it already covers head with every finding dispositioned, or fall through
// and REGENERATE the report (fresh computed+judged findings, always
// undispositioned on a first run) and freeze THAT in the same motion. The
// round hit the second path as a silent trap three times: X-13 (a fresh,
// undispositioned report rode straight into the archive), X-16 (committing
// dispositions before close moved HEAD, so close's freeze-align saw stale
// covers and regenerated over them), X-17 (a feature with no prior report
// at all got one created-and-frozen, undispositioned, by close itself).
// Condition 4 refuses BEFORE any freeze is attempted whenever close's own
// freeze step would NOT take the safe freeze-in-place path — the identical
// precondition align.go's own fork checks — turning the silent archive
// into a named, exit-1 verdict instead.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// runClosureGate evaluates 03 §Gates' closure gate for spec at head:
// eligible (the story-level fold, authoritative evidence only), no
// unresolved spec-stale flag, and no unresolved pending-supersession flag.
// f may be nil (no forge configured / unreachable, or no network in tests).
// When the story implements a feature whose open supersession MRs cannot be
// enumerated because f is nil, the pending-supersession condition is
// reported disclosed-unproven — rendered through the shared
// internal/disclosure seam (spec/disclosure-seam-v2, ac-1), never a silent
// pass (constitution 2/10: silence is never a pass) — rather than being
// read as "no pending MRs exist". Only a story that implements no feature at all
// (nothing to prove) passes that condition outright with a nil forge.
func runClosureGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, f forge.Forge, defaultBranchRef string, manifest *store.Manifest, mdl *model.Model, head string, stdout io.Writer) (bool, error) {
	cond1, err := checkClosureEligible(ctx, root, spec, head, mdl)
	if err != nil {
		return false, err
	}
	cond2, err := checkSpecStaleCondition(root, spec, manifest)
	if err != nil {
		return false, err
	}
	cond3, err := checkPendingSupersessionCondition(ctx, f, defaultBranchRef, spec)
	if err != nil {
		return false, err
	}
	cond4, err := checkDispositionCompleteCondition(root, spec, head)
	if err != nil {
		return false, err
	}

	allOK := true
	for _, c := range []gateCondition{cond1, cond2, cond3, cond4} {
		switch {
		case c.Disclosed:
			// Three-valued honesty (constitution 2/10): the input was
			// unavailable, so this is neither a pass nor a fail — rendered
			// through the shared internal/disclosure seam
			// (spec/disclosure-seam-v2, ac-1), the same Render function
			// gate.go's reportGateConditions and lint's Finding.String() use.
			fmt.Fprint(stdout, "closure: ")
			fmt.Fprintln(stdout, disclosure.Render(disclosure.New(c.Source, "", c.Reason)))
		case c.OK:
			fmt.Fprintf(stdout, "[PASS] closure: %s\n", c.Name)
		default:
			allOK = false
			fmt.Fprintf(stdout, "[FAIL] closure: %s\n", c.Name)
			fmt.Fprintf(stdout, "       %s\n", c.Reason)
		}
		// spec/evidence-resilience ac-2: per-record disclosed-unproven
		// detail (e.g. checkClosureEligible's quarantine disclosures),
		// printed regardless of which branch fired above.
		for _, extra := range c.Extra {
			fmt.Fprintln(stdout, extra)
		}
	}
	return allOK, nil
}

// checkClosureEligible is the closure gate's "eligible is true" condition:
// the same story-level fold checkNoACViolated (gate.go) uses, checked for
// full eligibility (every AC evidenced or waived) rather than merely
// "not violated".
func checkClosureEligible(ctx context.Context, root string, spec *artifact.SpecFrontmatter, head string, mdl *model.Model) (gateCondition, error) {
	// The condition's class word is display prose and resolves (L-M13(1),
	// nil-safe bare-id fallback); "eligible" is the fold's verdict
	// vocabulary, not a lifecycle state — bare.
	storyWord := mdl.DisplayClass("story")
	name := "1. " + storyWord + " eligible (every AC evidenced or waived, authoritative evidence)"

	// Preview stays false — co-1: the closure gate folds ONLY source: ci
	// evidence, never the --preview escape hatch.
	result, err := foldStoryEvidence(ctx, root, spec, head, false)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %w", err)
	}

	cond := gateCondition{Name: name}
	if result.Eligible {
		cond.OK = true
	} else {
		cond.Reason = storyWord + " is not eligible (not every AC is evidenced or waived)"
	}

	// spec/evidence-resilience ac-2 (X-15/X-11b, L-N3): surface per-record
	// disclosed-unproven detail for any unmet AC whose would-be evidence
	// was excluded because its commit is not reachable from HEAD — this is
	// the closure gate's OWN evidence ancestry consumer (the exact seam
	// that used to hard-fail operationally on this shape); the fold above
	// already stayed honest by construction (an excluded record never
	// counts toward "evidenced"), so this adds legibility only, never a
	// verdict change: an unmet AC whose gap traces to a quarantined or
	// otherwise-unreachable record reads as WHY, not as silent absence.
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	quarantined, undecodable, qErr := evidence.QuarantinedRecords(ctx, root, derivedRoot, head)
	if qErr != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %w", qErr)
	}
	cond.Extra = quarantineDisclosures(result.ACs, quarantined)
	// spec/evidence-resilience ac-2 (finding 2): a record file that failed
	// strict decode inside quarantined data is disclosed unconditionally
	// (it cannot be tied to a specific AC — it did not decode) rather than
	// bricking the gate operationally, so the exact stale-poisoned-bundle
	// debris X-15 leaves reads as WHY, not as a hard fail.
	cond.Extra = append(cond.Extra, undecodableDisclosures(undecodable)...)
	return cond, nil
}

// undecodableDisclosures renders one disclosed-unproven line per record file
// that failed strict decode inside quarantined data (spec/evidence-resilience
// ac-2, finding 2). It is disclosed unconditionally — not per-AC, the way
// quarantineDisclosures is — because an undecodable file cannot be read to
// learn which AC its records would have evidenced; disclosing it at all is
// what keeps the debris from passing silently while the closure run stays
// non-operational (ac-2: "the closure run itself does not exit operationally
// just because that one record degraded").
func undecodableDisclosures(undecodable []evidence.UndecodableFile) []string {
	var lines []string
	for _, u := range undecodable {
		text := fmt.Sprintf("a quarantined evidence record file %s is undecodable and was excluded from the fold: %s", u.Path, u.Reason)
		lines = append(lines, disclosure.Render(disclosure.New("gate:evidence-quarantine", "", text)))
	}
	return lines
}

// quarantineDisclosures renders one disclosed-unproven line (spec/
// evidence-resilience ac-2) per (unmet AC, excluded record naming that AC)
// pair — never for a met AC (evidenced or waived has nothing to disclose,
// mirroring closepreflight.go's own unmetStoryACDetail precedent), so a
// reader sees WHY an AC still is not evidenced when a record that WOULD
// have evidenced it was excluded for being unreachable, rather than
// reading the gap as if no evidence was ever produced. Prefers the actual
// reason `verdi sync` recorded on the record (artifact.Evidence.Quarantine,
// ac-1) when present; falls back to a generic reachability statement for a
// record this story's own build never had the chance to quarantine (e.g.
// hand-placed derived data, or evidence synced before this story landed).
func quarantineDisclosures(acs []evidence.ACResult, quarantined []artifact.Evidence) []string {
	var lines []string
	for _, ac := range acs {
		if ac.Status == evidence.StatusEvidenced || ac.Status == evidence.StatusWaived {
			continue
		}
		for _, rec := range evidence.RecordsForAC(quarantined, ac.ID) {
			reason := fmt.Sprintf("provenance.commit %s is not reachable from HEAD", rec.Provenance.Commit)
			if rec.Quarantine != nil && rec.Quarantine.Reason != "" {
				reason = rec.Quarantine.Reason
			}
			text := fmt.Sprintf("a %s record (witness %q) that would have evidenced %s was excluded: %s", rec.Kind, rec.Witness, ac.ID, reason)
			lines = append(lines, disclosure.Render(disclosure.New("gate:evidence-quarantine", ac.ID, text)))
		}
	}
	return lines
}

// checkSpecStaleCondition is the closure gate's spec-stale condition
// (03 §The amendment ladder's rung-arbitrage counter-pressure): blocks
// while SpecStale is Flagged. The story's deviation report (frozen or
// living — closure reads whichever is on disk, mirroring gate condition
// 3's own read) supplies Findings; an absent report has no
// accepted-deviation dispositions to flag at all, so it is read as
// trivially unflagged, not as an error (a story with no build activity yet
// cannot be spec-stale).
func checkSpecStaleCondition(root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest) (gateCondition, error) {
	name := "2. no unresolved spec-stale flag"

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: internal error: resolved spec has an invalid id: %w", err)
	}
	path := store.DeviationReportPath(root, store.ZoneActive, specRef.Name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gateCondition{Name: name, OK: true}, nil
		}
		return gateCondition{}, fmt.Errorf("closure gate: reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %s: %w", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %s failed to decode: %w", path, err)
	}

	storyACIDs := make(map[string]bool, len(spec.AcceptanceCriteria))
	for _, ac := range spec.AcceptanceCriteria {
		storyACIDs[ac.ID] = true
	}
	threshold := 0
	if manifest != nil && manifest.Audit != nil {
		threshold = manifest.Audit.DeviationsStaleThreshold
	}

	result := evidence.SpecStale(evidence.SpecStaleInput{Findings: decoded.Findings, StoryACIDs: storyACIDs, Threshold: threshold})
	if !result.Flagged {
		return gateCondition{Name: name, OK: true}, nil
	}
	return gateCondition{Name: name, Reason: fmt.Sprintf("spec-stale: own-text finding(s) %v, accepted-deviation count %d (threshold %d)", result.OwnTextFindingIDs, result.AcceptedDeviationCount, threshold)}, nil
}

// checkPendingSupersessionCondition is the closure gate's
// pending-supersession condition (03 §The amendment ladder: "the fold's
// input set includes open supersession MRs"): for every feature the story
// implements, probes for an open MR carrying a candidate v2 spec
// (R4-I-14's naming convention, <name>-v2, mirroring V1-P3's own
// evidence.LoadPendingSupersessionCandidates caller contract) and folds
// the story's touched object ids against it.
func checkPendingSupersessionCondition(ctx context.Context, f forge.Forge, defaultBranchRef string, spec *artifact.SpecFrontmatter) (gateCondition, error) {
	name := "3. no unresolved pending-supersession flag"

	byFeature := evidence.ImplementsByFeature(spec.Links)
	if len(byFeature) == 0 {
		// The story implements no feature — there is no open-supersession
		// input to fold at all, so the condition is genuinely satisfied.
		return gateCondition{Name: name, OK: true}, nil
	}
	if f == nil {
		// The story implements a feature, but no forge is configured or
		// reachable, so open supersession MRs cannot be enumerated. Disclose
		// the check unproven rather than reading the missing input as
		// "no pending MRs" (constitution 2/10: silence is never a pass).
		return gateCondition{
			Name:      name,
			Disclosed: true,
			Source:    "gate:pending-supersession",
			Reason:    "no forge configured/reachable, so open supersession MRs cannot be enumerated (not read as 'no pending MRs' — constitution 2/10)",
		}, nil
	}

	featureNames := make([]string, 0, len(byFeature))
	for n := range byFeature {
		featureNames = append(featureNames, n)
	}
	sort.Strings(featureNames)

	var touched, mrIDs []string
	for _, featureName := range featureNames {
		candidatePath := store.ActiveSpecRelPath(featureName + "-v2")
		candidates, err := evidence.LoadPendingSupersessionCandidates(ctx, f, defaultBranchRef, "spec/"+featureName, candidatePath)
		if err != nil {
			return gateCondition{}, fmt.Errorf("closure gate: loading pending-supersession candidates for %s: %w", featureName, err)
		}
		result := evidence.PendingSupersession(evidence.PendingSupersessionInput{ObjectIDs: byFeature[featureName], Candidates: candidates})
		if result.Flagged {
			touched = append(touched, result.Touched...)
			mrIDs = append(mrIDs, result.MRIDs...)
		}
	}
	if len(touched) == 0 {
		return gateCondition{Name: name, OK: true}, nil
	}
	sort.Strings(touched)
	sort.Strings(mrIDs)
	return gateCondition{Name: name, Reason: fmt.Sprintf("open supersession MR(s) %v touch object(s) %v", mrIDs, touched)}, nil
}

// dispositionRitual is the remedy every checkDispositionCompleteCondition
// failure names (X-13/X-16/X-17's decoded runbook, extensibility-chronicle
// 2026-07-17): align refreshes the report to head, disposition is a
// working-tree edit (never a commit — X-16: committing first moves HEAD,
// so close's own freeze-align sees stale covers and regenerates over the
// dispositions), then close freezes the now-current, fully-dispositioned
// report in place.
const dispositionRitual = "the closure ritual is align (`verdi align`) -> disposition every finding as a working-tree edit (never commit it) -> close (`verdi close`)"

// checkDispositionCompleteCondition is the closure gate's condition 4
// (X-13/X-16/X-17, see this file's top doc comment): a living, unfrozen
// deviation-report.md must be present in the spec's directory, cover head,
// and carry no undispositioned finding — precisely the precondition
// runAlignForSpec's freeze-in-place fork (align.go) requires before it
// will stamp the report Frozen VERBATIM rather than regenerating it fresh.
// Checked here, BEFORE close ever attempts to freeze anything, using
// loadExistingReport (align.go) — the exact same reader the freeze step
// itself uses, so what this condition inspects can never drift from what
// close would actually freeze.
//
// D6-24 is preserved by construction: a report that already covers head
// with every finding dispositioned (the fresh-covers-dispositioned case)
// passes this condition and then genuinely takes the freeze-in-place path
// — this condition never causes a regenerate that would discard
// dispositions; it only ever refuses BEFORE a regenerate would happen.
func checkDispositionCompleteCondition(root string, spec *artifact.SpecFrontmatter, head string) (gateCondition, error) {
	name := "4. deviation report ready to freeze (no undispositioned findings)"

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: internal error: resolved spec has an invalid id: %w", err)
	}
	path := store.DeviationReportPath(root, store.ZoneActive, specRef.Name)
	report, _, err := loadExistingReport(path)
	if err != nil {
		return gateCondition{}, fmt.Errorf("closure gate: %w", err)
	}
	if report == nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("no deviation-report.md found at %s; %s", path, dispositionRitual)}, nil
	}
	if report.Covers != head {
		return gateCondition{Name: name, Reason: fmt.Sprintf("%s covers %s, not head %s; %s", path, report.Covers, head, dispositionRitual)}, nil
	}

	var undispositioned []string
	for _, f := range report.Findings {
		if !f.Dispositioned() {
			undispositioned = append(undispositioned, f.ID)
		}
	}
	if len(undispositioned) > 0 {
		sort.Strings(undispositioned)
		return gateCondition{Name: name, Reason: fmt.Sprintf("undispositioned finding(s) %v; %s", undispositioned, dispositionRitual)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}
