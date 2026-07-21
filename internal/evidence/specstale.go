package evidence

import (
	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
)

// DefaultDeviationsStaleThreshold is the spec-stale flag's threshold-count
// trigger's default (03 §The amendment ladder: "more than a configured
// count of accepted-deviation dispositions accumulated on one story
// (verdi.yaml: audit.deviations_stale_threshold, default 3, tunable —
// a watch item)"). internal/store decodes the raw manifest value and, per
// its AuditConfig doc comment, leaves applying this default (and
// disambiguating an absent field from an explicit 0) to this consuming
// phase: SpecStale substitutes it whenever Threshold is zero or negative.
const DefaultDeviationsStaleThreshold = 3

// SpecStaleInput is SpecStale's input for one story.
type SpecStaleInput struct {
	// Findings are the story's frozen (or living) deviation report's
	// findings (artifact.DeviationFrontmatter.Findings).
	Findings []artifact.Finding
	// StoryACIDs is the set of AC ids the story's OWN spec declares — the
	// trigger (a)'s join key. See SpecStale's doc comment for the
	// disclosed judgment call this join implements.
	StoryACIDs map[string]bool
	// Threshold is the configured accumulation count
	// (audit.deviations_stale_threshold, decoded by internal/store into
	// Manifest.Audit.DeviationsStaleThreshold — the caller reads it there
	// and passes it in). A zero or negative Threshold is read as "use the
	// default" (DefaultDeviationsStaleThreshold) rather than as a
	// caller's deliberate zero, since a real zero would flag on the very
	// first accepted-deviation, which 03's "tunable, default 3" framing
	// never intends as the out-of-the-box behavior.
	Threshold int
	// OwnNotResurfaced is the SAME report's own not-resurfaced: section — the
	// prior rulings a fresh judge run did not re-emit (artifact.
	// DeviationFrontmatter.NotResurfaced), which SHARE Findings' AC-id
	// namespace because they are the same report/spec. It feeds BOTH triggers:
	//
	//   - trigger (a): an own-text accepted-deviation (id == one of the story's
	//     own declared AC ids) keeps raising spec-stale after it moves from
	//     findings: into this section — a fresh judge run failing to reproduce
	//     it must never silently un-flag a standing own-text adjudication
	//     (judged-spec-stale-own-text-not-resurfaced; the X-18 un-flag drain).
	//   - trigger (b): unioned into the accepted-deviation budget by unique
	//     content identity, so a finding that stops reproducing never drains
	//     out of the count just because it moved out of findings: (ac-3's
	//     laundering drain — a no-op for a single well-formed report, whose
	//     findings: and not-resurfaced: are disjoint by id, L-N2).
	//
	// Distinct from AdditionalSets precisely on the namespace boundary: this
	// set is the report's OWN, AdditionalSets is possibly cross-report.
	OwnNotResurfaced []artifact.Finding
	// AdditionalSets are further finding sets whose accepted-deviation
	// dispositions count toward the SAME budget as Findings, unioned by
	// unique content identity (align.Identity's Kind+ID+Text hash) rather
	// than concatenated — spec/finding-identity's counterweight hardening
	// (ledger L-N2). Used for CROSS-REPORT sets only (ac-4, the actual
	// cross-report X-18 fix): the feature-closure gate passes every closed
	// implementing story's ARCHIVED report's findings: + not-resurfaced: here,
	// so a story-archived accepted deviation counts exactly once toward the
	// feature-close budget — never zero (silently dropped because the feature's
	// own report never reproduced it) and never twice (double-counted across
	// the story and feature reports independently).
	//
	// Deliberately excluded from trigger (a)'s "own text" join: an
	// AdditionalSets entry's finding ids are drawn from a DIFFERENT spec's own
	// AC-id namespace (a story's archived report, at the feature level) — an id
	// collision there must never be misread as "this spec's own declared AC
	// text was targeted". A report's OWN not-resurfaced: (same namespace) goes
	// in OwnNotResurfaced, which DOES feed trigger (a); only Findings and
	// OwnNotResurfaced feed trigger (a).
	AdditionalSets [][]artifact.Finding
}

// SpecStaleResult is SpecStale's outcome.
type SpecStaleResult struct {
	Flagged bool
	// OwnTextFindingIDs are the accepted-deviation findings whose id
	// equals one of the story's own declared AC ids — trigger (a).
	OwnTextFindingIDs []string
	// AcceptedDeviationCount is the total count of accepted-deviation
	// dispositions on the story, regardless of trigger.
	AcceptedDeviationCount int
	// TriggeredByThreshold reports whether AcceptedDeviationCount exceeded
	// Threshold — trigger (b).
	TriggeredByThreshold bool
}

// SpecStale implements 03 §The amendment ladder's rung-arbitrage
// counter-pressure: the `spec-stale` flag raised by either of two
// deterministic triggers — (a) an accepted-deviation disposition whose
// finding targets an AC's own declared text, or (b) more than
// audit.deviations_stale_threshold accepted-deviation dispositions
// accumulated on one story.
//
// Trigger (a)'s join, disclosed as a judgment call (no spec section
// defines it — see the phase report): artifact.Finding carries no
// structured pointer to which spec object it targets, only free-text
// (ID, Text). This function reads "targets an AC's own declared text" as
// Finding.ID exactly equaling one of the story's own declared AC ids —
// the smallest reversible reading available without inventing a new
// internal/artifact field (this phase may not touch that package): AC ids
// are already the natural, stable identity used as a join key everywhere
// else in this system (evidence_for entries, binding ACs, stub AC sets),
// so a computed finding whose id IS an AC id is the one unambiguous,
// zero-new-schema way to say "this finding is about that AC". A prose
// heuristic (substring-matching Finding.Text against AC.Text) was
// considered and rejected: it is non-deterministic in spirit (wording
// drift breaks the match silently) and CLAUDE.md forbids exactly that
// class of invented parsing convention.
func SpecStale(in SpecStaleInput) SpecStaleResult {
	// Documented rule (adjudicated, not silent): a threshold of 0 (or
	// absent — Go's zero value collapses the two, matching this codebase's
	// config idiom) means the default (DefaultDeviationsStaleThreshold, 3),
	// NOT "flag on the first accepted-deviation". A store therefore cannot
	// configure a zero threshold; the loosest configurable-and-honored value
	// is 1. 03's "tunable, default 3" framing never intends a zero
	// out-of-the-box, and disambiguating an explicit 0 from an absent field
	// would need a decode-boundary pointer that internal/store deliberately
	// does not carry. Negative values are rejected upstream by
	// AuditConfig.Validate.
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = DefaultDeviationsStaleThreshold
	}

	var result SpecStaleResult

	// Trigger (a) reads the report's OWN sets — Findings and OwnNotResurfaced,
	// which share this spec's AC-id namespace — never AdditionalSets
	// (AdditionalSets' doc comment: an id collision against a DIFFERENT spec's
	// AC-id namespace must never be misread as "this spec's own text was
	// targeted"). Scanning OwnNotResurfaced closes the un-flag drain: an
	// own-text accepted-deviation that stops reproducing and moves to
	// not-resurfaced: must keep raising the flag
	// (judged-spec-stale-own-text-not-resurfaced). Deduped by id so the
	// candidate+backing shape (same id undispositioned in Findings, dispositioned
	// in OwnNotResurfaced) can never list an id twice.
	ownTextSeen := make(map[string]bool)
	for _, set := range [][]artifact.Finding{in.Findings, in.OwnNotResurfaced} {
		for _, f := range set {
			if f.Disposition == artifact.FindingAcceptedDeviation && in.StoryACIDs[f.ID] && !ownTextSeen[f.ID] {
				ownTextSeen[f.ID] = true
				result.OwnTextFindingIDs = append(result.OwnTextFindingIDs, f.ID)
			}
		}
	}

	// Trigger (b) — the accepted-deviation budget itself — unions Findings
	// with OwnNotResurfaced and every AdditionalSets entry by unique content
	// identity (align.Identity), so the SAME standing adjudication reproduced
	// across more than one set counts exactly once (spec/finding-identity ac-3/
	// ac-4's counterweight hardening; the field doc comments have the full
	// rationale). For a single, well-formed report with only Findings
	// populated, every id — and so every identity — is already unique by schema
	// construction, so this union is a no-op there: the exact same count
	// SpecStale always produced.
	seenIdentity := make(map[string]bool)
	sets := make([][]artifact.Finding, 0, 2+len(in.AdditionalSets))
	sets = append(sets, in.Findings)
	sets = append(sets, in.OwnNotResurfaced)
	sets = append(sets, in.AdditionalSets...)
	for _, set := range sets {
		for _, f := range set {
			if f.Disposition != artifact.FindingAcceptedDeviation {
				continue
			}
			id := align.Identity(f)
			if seenIdentity[id] {
				continue
			}
			seenIdentity[id] = true
			result.AcceptedDeviationCount++
		}
	}

	result.TriggeredByThreshold = result.AcceptedDeviationCount > threshold
	result.Flagged = len(result.OwnTextFindingIDs) > 0 || result.TriggeredByThreshold
	return result
}
