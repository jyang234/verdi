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
	// AdditionalSets are further finding sets whose accepted-deviation
	// dispositions count toward the SAME budget as Findings, unioned by
	// unique content identity (align.Identity's Kind+ID+Text hash) rather
	// than concatenated — spec/finding-identity's counterweight hardening
	// (ledger L-N2):
	//
	//   - ac-3 (within one report): the closure gate passes the report's own
	//     not-resurfaced: section here, so a finding that stops reproducing
	//     under a fresh judge run never drains out of the budget just
	//     because it moved out of findings: (the X-18 laundering drain this
	//     union closes) — proven a no-op for a single well-formed report,
	//     where ids (and so identities) are already unique by construction,
	//     exactly as L-N2 itself records ("the within-report unique-identity
	//     framing was proven a no-op").
	//   - ac-4 (across reports, the actual cross-report X-18 fix): the
	//     feature-closure gate passes every closed implementing story's
	//     ARCHIVED report's findings: + not-resurfaced: here, so a story-
	//     archived accepted deviation counts exactly once toward the
	//     feature-close budget — never zero (silently dropped because the
	//     feature's own report never reproduced it) and never twice
	//     (double-counted across the story and feature reports
	//     independently).
	//
	// Deliberately excluded from trigger (a)'s "own text" join: an
	// AdditionalSets entry's finding ids are drawn from a POSSIBLY DIFFERENT
	// spec's own AC-id namespace (a story's archived report, at the feature
	// level) — an id collision there must never be misread as "this spec's
	// own declared AC text was targeted". Only Findings feeds trigger (a).
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

	// Trigger (a) reads Findings ONLY — the primary/own set — never
	// AdditionalSets (AdditionalSets' doc comment: an id collision against a
	// DIFFERENT spec's AC-id namespace must never be misread as "this spec's
	// own text was targeted").
	for _, f := range in.Findings {
		if f.Disposition == artifact.FindingAcceptedDeviation && in.StoryACIDs[f.ID] {
			result.OwnTextFindingIDs = append(result.OwnTextFindingIDs, f.ID)
		}
	}

	// Trigger (b) — the accepted-deviation budget itself — unions Findings
	// with every AdditionalSets entry by unique content identity
	// (align.Identity), so the SAME standing adjudication reproduced across
	// more than one set counts exactly once (spec/finding-identity ac-3/
	// ac-4's counterweight hardening; AdditionalSets' own doc comment has
	// the full rationale). For a single, well-formed report with only
	// Findings populated, every id — and so every identity — is already
	// unique by schema construction, so this union is a no-op there: the
	// exact same count SpecStale always produced.
	seenIdentity := make(map[string]bool)
	sets := make([][]artifact.Finding, 0, 1+len(in.AdditionalSets))
	sets = append(sets, in.Findings)
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
