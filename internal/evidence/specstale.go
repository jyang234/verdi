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
	// DeviationFrontmatter.NotResurfaced). It feeds trigger (b) ONLY:
	//
	//   - trigger (b): unioned into the accepted-deviation budget by unique
	//     content identity, so a finding that stops reproducing never drains
	//     out of the count just because it moved out of findings: (ac-3's
	//     laundering drain — a no-op for a single well-formed report, whose
	//     findings: and not-resurfaced: are disjoint by id, L-N2).
	//
	// It does NOT feed trigger (a). The not-resurfaced: section holds only
	// judged-kind entries (align.ReconcileJudged is its sole producer), whose
	// "judged-"-prefixed ids can never equal an AC id (which matches ^ac-), so
	// an own-text join over it can never contribute — the earlier scan that
	// claimed to close an un-flag drain here was dead by construction
	// (judged-spec-stale-own-text-judged-id-prefix; the disjointness is pinned
	// in align.TestNotResurfacedIDsCanNeverBeACIDs).
	//
	// With trigger (a) reading neither, OwnNotResurfaced is now behaviourally
	// equivalent to an AdditionalSets entry (both union into trigger (b) by
	// identity). It is kept a distinct field only to name the namespace boundary
	// for the reader: this set is the report's OWN; AdditionalSets is possibly
	// cross-report.
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
	// text was targeted". Only Findings feeds trigger (a) (OwnNotResurfaced is
	// judged-only and can never carry an AC-shaped id — see its doc comment).
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

	// Trigger (a) reads the report's OWN findings: ONLY. An own-text
	// accepted-deviation is a finding whose id equals one of the spec's own
	// declared AC ids — which only a COMPUTED finding can ever be (a computed
	// finding targeting an AC's declared text), since AC ids match ^ac-
	// (artifact acIDRe) while every judged id is "judged-"-prefixed (judge.go).
	//
	// OwnNotResurfaced is deliberately NOT scanned here. The not-resurfaced:
	// section holds only judged-kind entries (align.ReconcileJudged is its sole
	// producer), whose ids can never equal an AC id, so a scan of it could never
	// contribute to trigger (a) — it was dead by construction
	// (judged-spec-stale-own-text-judged-id-prefix; disjointness pinned in
	// align.TestNotResurfacedIDsCanNeverBeACIDs). The un-flag drain the earlier
	// scan claimed to close cannot arise on this judged-only path at all: a
	// computed own-text deviation never enters not-resurfaced: — see trigger
	// (b)'s note on why dropping a vanished computed deviation is honest rather
	// than a drain. AdditionalSets is likewise never scanned for own-text: its
	// entries are a DIFFERENT spec's AC-id namespace. Findings ids are unique
	// within one report (artifact Validate), so no dedup is needed.
	for _, f := range in.Findings {
		if f.Disposition == artifact.FindingAcceptedDeviation && in.StoryACIDs[f.ID] {
			result.OwnTextFindingIDs = append(result.OwnTextFindingIDs, f.ID)
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
	//
	// This not-resurfaced counterweight covers the JUDGED path only, and
	// correctly so (judged-spec-stale-own-text-judged-id-prefix). A judged
	// accepted-deviation that stops reproducing is a judge NON-REPRODUCTION — the
	// underlying issue may well still stand — so persisting it in not-resurfaced:
	// and keeping it in the budget is the honest floor (the X-18 laundering
	// drain). A vanishing COMPUTED accepted-deviation is the opposite: its
	// disappearance is DETERMINISTIC — the regenerated boundary contract itself
	// changed, so the fact the deviation described no longer exists.
	// PreserveDispositions simply dropping such a finding is therefore honest,
	// not a drain: the budget SHOULD fall when the boundary genuinely changed.
	// That path never enters not-resurfaced: (align.ReconcileJudged, its sole
	// producer, is judged-only), so it is correctly outside this union — nothing
	// to preserve, because the disappearance is a real change of fact, not a
	// stochastic non-reproduction.
	sets := make([][]artifact.Finding, 0, 2+len(in.AdditionalSets))
	sets = append(sets, in.Findings)
	sets = append(sets, in.OwnNotResurfaced)
	sets = append(sets, in.AdditionalSets...)
	result.AcceptedDeviationCount = CountAcceptedDeviations(sets...)

	result.TriggeredByThreshold = result.AcceptedDeviationCount > threshold
	result.Flagged = len(result.OwnTextFindingIDs) > 0 || result.TriggeredByThreshold
	return result
}

// CountAcceptedDeviations returns the number of DISTINCT accepted-deviation
// dispositions across sets, unioned by unique content identity
// (align.Identity's Kind+ID+Text hash) so the SAME standing adjudication
// reproduced across more than one set counts exactly once. This is the one
// accepted-deviation-budget counting rule SpecStale's threshold trigger reads —
// and the same rule the feature-close gate reuses to count a superseded
// implementing story's archived deviations for its disclose-and-exclude line
// (spec/finding-identity ledger L-N12), never a second hand-rolled loop that
// could drift from this one.
//
// L-N14 companion (cross-level re-recording awareness): a CONFIRMED feature-level
// reaffirmation carries CarriedFrom — a human's signature that it is the SAME
// deviation as the prior ruling under that judged slug, INCLUDING a cross-level
// prior in an implementing story's archive whose text the feature judge reworded.
// Content identity (kind+id+text) alone would read the reworded texts as two
// distinct identities and double-count the archived ruling against its
// feature-level reaffirmation. So the count collapses by SLUG (kind+id) for any
// slug a carried-from accepted-deviation occupies: every accepted-deviation at
// that slug counts once. This is scoped strictly to carried-from slugs — a bare
// slug coincidence between two genuinely different rulings (no reaffirmation)
// still counts by content identity, never silently deduped. Within a single
// well-formed report ids are unique, so this collapse is a no-op there (the exact
// count SpecStale always produced); it only ever changes the CROSS-report feature
// union, exactly where L-N14's re-recording target lives.
func CountAcceptedDeviations(sets ...[]artifact.Finding) int {
	// First pass: every (kind,id) slug a carried-from accepted-deviation occupies.
	carriedSlug := make(map[string]bool)
	for _, set := range sets {
		for _, f := range set {
			if f.Disposition == artifact.FindingAcceptedDeviation && f.CarriedFrom != "" {
				carriedSlug[acceptedDeviationSlugKey(f)] = true
			}
		}
	}

	seenSlug := make(map[string]bool)
	seenIdentity := make(map[string]bool)
	n := 0
	for _, set := range sets {
		for _, f := range set {
			if f.Disposition != artifact.FindingAcceptedDeviation {
				continue
			}
			if slug := acceptedDeviationSlugKey(f); carriedSlug[slug] {
				// A slug a confirmed reaffirmation owns: every accepted-deviation at
				// this slug is the same deviation — count it once, text-independent.
				if seenSlug[slug] {
					continue
				}
				seenSlug[slug] = true
				n++
				continue
			}
			id := align.Identity(f)
			if seenIdentity[id] {
				continue
			}
			seenIdentity[id] = true
			n++
		}
	}
	return n
}

// acceptedDeviationSlugKey is the (kind,id) collapse key L-N14's cross-level
// reaffirmation counts by — the judged SLUG, text excluded (align.Identity folds
// text in; this deliberately does not). A null separator keeps kind and id
// unambiguous. Never the budget's default key: it is consulted only for slugs a
// carried-from reaffirmation occupies (see CountAcceptedDeviations).
func acceptedDeviationSlugKey(f artifact.Finding) string {
	return string(f.Kind) + "\x00" + f.ID
}
