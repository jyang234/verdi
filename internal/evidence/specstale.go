package evidence

import "github.com/OWNER/verdi/internal/artifact"

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
	// (audit.deviations_stale_threshold — DeviationsStaleThreshold reads
	// it from verdi.yaml). A zero or negative Threshold is read as "use
	// the default" (DefaultDeviationsStaleThreshold) rather than as a
	// caller's deliberate zero, since a real zero would flag on the very
	// first accepted-deviation, which 03's "tunable, default 3" framing
	// never intends as the out-of-the-box behavior.
	Threshold int
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
	threshold := in.Threshold
	if threshold <= 0 {
		threshold = DefaultDeviationsStaleThreshold
	}

	var result SpecStaleResult
	for _, f := range in.Findings {
		if f.Disposition != artifact.FindingAcceptedDeviation {
			continue
		}
		result.AcceptedDeviationCount++
		if in.StoryACIDs[f.ID] {
			result.OwnTextFindingIDs = append(result.OwnTextFindingIDs, f.ID)
		}
	}

	result.TriggeredByThreshold = result.AcceptedDeviationCount > threshold
	result.Flagged = len(result.OwnTextFindingIDs) > 0 || result.TriggeredByThreshold
	return result
}
