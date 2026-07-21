package evidence

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func acceptedDeviation(id, note string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingComputed, Text: "some deviation text", Disposition: artifact.FindingAcceptedDeviation, Note: note}
}

func fixedFinding(id string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingComputed, Text: "fixed already", Disposition: artifact.FindingFixed}
}

// TestSpecStale_TriggerA is the exit criterion's spec-stale case for
// trigger (a): a deviation targeting an AC's own declared text.
func TestSpecStale_TriggerA(t *testing.T) {
	in := SpecStaleInput{
		Findings:   []artifact.Finding{acceptedDeviation("ac-2", "spec text itself was wrong")},
		StoryACIDs: map[string]bool{"ac-1": true, "ac-2": true},
	}
	got := SpecStale(in)
	if !got.Flagged {
		t.Fatal("Flagged = false, want true (finding id equals the story's own ac-2)")
	}
	if len(got.OwnTextFindingIDs) != 1 || got.OwnTextFindingIDs[0] != "ac-2" {
		t.Fatalf("OwnTextFindingIDs = %v, want [ac-2]", got.OwnTextFindingIDs)
	}
	if got.TriggeredByThreshold {
		t.Fatal("TriggeredByThreshold = true, want false (only one accepted-deviation)")
	}
}

// TestSpecStale_TriggerB is the exit criterion's spec-stale case for
// trigger (b): threshold-count accumulation.
func TestSpecStale_TriggerB(t *testing.T) {
	in := SpecStaleInput{
		Findings: []artifact.Finding{
			acceptedDeviation("finding-1", "n1"),
			acceptedDeviation("finding-2", "n2"),
			acceptedDeviation("finding-3", "n3"),
			acceptedDeviation("finding-4", "n4"),
		},
		StoryACIDs: map[string]bool{"ac-1": true}, // none of the finding ids match a real AC id
		Threshold:  3,
	}
	got := SpecStale(in)
	if !got.Flagged {
		t.Fatal("Flagged = false, want true (4 accepted-deviations > threshold 3)")
	}
	if !got.TriggeredByThreshold {
		t.Fatal("TriggeredByThreshold = false, want true")
	}
	if len(got.OwnTextFindingIDs) != 0 {
		t.Fatalf("OwnTextFindingIDs = %v, want none (trigger b only)", got.OwnTextFindingIDs)
	}
	if got.AcceptedDeviationCount != 4 {
		t.Fatalf("AcceptedDeviationCount = %d, want 4", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_NotFlagged_AtOrBelowThreshold proves "more than" is strict:
// exactly the threshold count does not flag.
func TestSpecStale_NotFlagged_AtOrBelowThreshold(t *testing.T) {
	in := SpecStaleInput{
		Findings: []artifact.Finding{
			acceptedDeviation("finding-1", "n1"),
			acceptedDeviation("finding-2", "n2"),
			acceptedDeviation("finding-3", "n3"),
		},
		StoryACIDs: map[string]bool{},
		Threshold:  3,
	}
	got := SpecStale(in)
	if got.Flagged {
		t.Fatal("Flagged = true, want false (exactly at threshold, not over it)")
	}
}

// TestSpecStale_DefaultThreshold proves a zero/absent Threshold falls back
// to DefaultDeviationsStaleThreshold (3) rather than flagging on the first
// accepted-deviation.
func TestSpecStale_DefaultThreshold(t *testing.T) {
	in := SpecStaleInput{
		Findings:   []artifact.Finding{acceptedDeviation("finding-1", "n1")},
		StoryACIDs: map[string]bool{},
	}
	got := SpecStale(in)
	if got.Flagged {
		t.Fatal("Flagged = true, want false (1 accepted-deviation, default threshold 3)")
	}
}

// TestSpecStale_NonAcceptedDeviationFindingsIgnored proves fixed findings
// and undispositioned findings never count toward either trigger.
func TestSpecStale_NonAcceptedDeviationFindingsIgnored(t *testing.T) {
	in := SpecStaleInput{
		Findings: []artifact.Finding{
			fixedFinding("ac-1"),
			{ID: "ac-2", Kind: artifact.FindingJudged, Text: "t", Disposition: ""}, // undispositioned
		},
		StoryACIDs: map[string]bool{"ac-1": true, "ac-2": true},
	}
	got := SpecStale(in)
	if got.Flagged {
		t.Fatal("Flagged = true, want false (no accepted-deviation dispositions at all)")
	}
	if got.AcceptedDeviationCount != 0 {
		t.Fatalf("AcceptedDeviationCount = %d, want 0", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_Negative_NoFindings proves an empty deviation report never
// flags.
func TestSpecStale_Negative_NoFindings(t *testing.T) {
	got := SpecStale(SpecStaleInput{})
	if got.Flagged {
		t.Fatal("Flagged = true, want false for an empty input")
	}
}

func judgedAcceptedDeviation(id, text, note string) artifact.Finding {
	return artifact.Finding{ID: id, Kind: artifact.FindingJudged, Text: text, Disposition: artifact.FindingAcceptedDeviation, Note: note}
}

// TestSpecStale_AdditionalSets_UnionedByIdentity_NeverDoubleCounts is
// spec/finding-identity ac-3/ac-4's "never twice" half: the identical
// accepted-deviation finding present in BOTH Findings and an AdditionalSets
// entry (e.g. a report's own findings: reproducing the same finding its own
// not-resurfaced: also names, or a feature's own report reproducing what a
// story's archived report also names) counts exactly once.
func TestSpecStale_AdditionalSets_UnionedByIdentity_NeverDoubleCounts(t *testing.T) {
	f := judgedAcceptedDeviation("judged-a", "same text", "n")
	in := SpecStaleInput{
		Findings:       []artifact.Finding{f},
		AdditionalSets: [][]artifact.Finding{{f}},
		StoryACIDs:     map[string]bool{},
	}
	got := SpecStale(in)
	if got.AcceptedDeviationCount != 1 {
		t.Fatalf("AcceptedDeviationCount = %d, want 1 (the identical finding must not double count across sets)", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_AdditionalSets_NeverDropsAFindingPresentOnlyThere is the
// "never zero" half: an accepted-deviation finding present ONLY in an
// AdditionalSets entry (never in Findings — e.g. it lives in not-resurfaced:
// only, or only in a story's archived report and the feature's own report
// never reproduced it) still counts — the X-18 "silently dropped" failure
// mode this story closes.
func TestSpecStale_AdditionalSets_NeverDropsAFindingPresentOnlyThere(t *testing.T) {
	f := judgedAcceptedDeviation("judged-only-there", "text", "n")
	in := SpecStaleInput{
		Findings:       nil,
		AdditionalSets: [][]artifact.Finding{{f}},
		StoryACIDs:     map[string]bool{},
	}
	got := SpecStale(in)
	if got.AcceptedDeviationCount != 1 {
		t.Fatalf("AcceptedDeviationCount = %d, want 1 (must not silently drop a finding present only in an additional set)", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_AdditionalSets_DistinctIdentitiesBothCount proves the union
// is a real union, not a cap of 1: two DIFFERENT accepted-deviation findings
// spread across Findings and an additional set both count.
func TestSpecStale_AdditionalSets_DistinctIdentitiesBothCount(t *testing.T) {
	in := SpecStaleInput{
		Findings:       []artifact.Finding{judgedAcceptedDeviation("judged-a", "text a", "n")},
		AdditionalSets: [][]artifact.Finding{{judgedAcceptedDeviation("judged-b", "text b", "n")}},
		StoryACIDs:     map[string]bool{},
	}
	got := SpecStale(in)
	if got.AcceptedDeviationCount != 2 {
		t.Fatalf("AcceptedDeviationCount = %d, want 2 (two distinct identities)", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_AdditionalSets_OwnTextTrigger_ScopedToPrimarySetOnly proves
// trigger (a)'s "own text" join is evaluated against Findings (the
// PRIMARY/own set) only, never AdditionalSets: at the feature level (ac-4),
// AdditionalSets carries OTHER stories' findings, whose ids (e.g. "ac-1")
// are drawn from a DIFFERENT spec's own AC-id namespace and must never be
// read as "the feature's own ac-1 text was targeted" just because the ids
// happen to collide.
func TestSpecStale_AdditionalSets_OwnTextTrigger_ScopedToPrimarySetOnly(t *testing.T) {
	in := SpecStaleInput{
		Findings:       nil,
		AdditionalSets: [][]artifact.Finding{{acceptedDeviation("ac-1", "targets some OTHER spec's ac-1")}},
		StoryACIDs:     map[string]bool{"ac-1": true},
	}
	got := SpecStale(in)
	if len(got.OwnTextFindingIDs) != 0 {
		t.Fatalf("OwnTextFindingIDs = %v, want none (own-text must not fire from an additional set)", got.OwnTextFindingIDs)
	}
	// The threshold-count trigger still sees it (it is still a real
	// accepted-deviation, just not an own-text one).
	if got.AcceptedDeviationCount != 1 {
		t.Fatalf("AcceptedDeviationCount = %d, want 1", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_OwnNotResurfaced_OwnTextTrigger_StillFiresAfterMovingSection
// is spec/finding-identity judged-spec-stale-own-text-not-resurfaced: an
// accepted-deviation whose id equals one of the story's OWN declared AC ids
// keeps raising the spec-stale flag (trigger a) after a fresh judge run stops
// reproducing it and it moves from findings: into the report's OWN
// not-resurfaced: section. Same report = same AC-id namespace, so the own-text
// join covers OwnNotResurfaced too — closing the un-flag drain a
// non-reproducing judge would otherwise open.
//
// Only ONE accepted-deviation total, so trigger (b) — the COUNT — cannot fire
// (1 <= default threshold 3): trigger (a) alone must carry the flag, isolating
// the fix. Red-first (before the fix): trigger (a) scanned only Findings, so
// the flag silently dropped the moment the finding moved to not-resurfaced:.
func TestSpecStale_OwnNotResurfaced_OwnTextTrigger_StillFiresAfterMovingSection(t *testing.T) {
	in := SpecStaleInput{
		Findings:         nil,
		OwnNotResurfaced: []artifact.Finding{judgedAcceptedDeviation("ac-3", "the spec's own ac-3 text was wrong", "owner-ratified")},
		StoryACIDs:       map[string]bool{"ac-1": true, "ac-2": true, "ac-3": true},
	}
	got := SpecStale(in)
	if !got.Flagged {
		t.Fatal("Flagged = false, want true — an own-text accepted-deviation in the report's own not-resurfaced: must still raise spec-stale (trigger a)")
	}
	if len(got.OwnTextFindingIDs) != 1 || got.OwnTextFindingIDs[0] != "ac-3" {
		t.Fatalf("OwnTextFindingIDs = %v, want [ac-3]", got.OwnTextFindingIDs)
	}
	if got.TriggeredByThreshold {
		t.Fatal("TriggeredByThreshold = true, want false (one accepted-deviation; trigger a alone must carry the flag)")
	}
}

// TestSpecStale_OwnNotResurfaced_UnionedIntoBudget_NoDoubleCount proves
// OwnNotResurfaced also feeds trigger (b) by unique content identity: the
// identical accepted-deviation present in both Findings and OwnNotResurfaced
// counts exactly once (the within-report no-op, ac-3).
func TestSpecStale_OwnNotResurfaced_UnionedIntoBudget_NoDoubleCount(t *testing.T) {
	f := judgedAcceptedDeviation("judged-a", "same text", "n")
	got := SpecStale(SpecStaleInput{
		Findings:         []artifact.Finding{f},
		OwnNotResurfaced: []artifact.Finding{f},
		StoryACIDs:       map[string]bool{},
	})
	if got.AcceptedDeviationCount != 1 {
		t.Fatalf("AcceptedDeviationCount = %d, want 1 (the identical finding must not double count across findings: and its own not-resurfaced:)", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_LaunderingReplay_CountUnchangedAcrossRegeneration is
// spec/finding-identity ac-3's exact laundering-replay proof: "round 1" has
// an accepted-deviation finding live in Findings; "round 2" simulates a
// judge re-roll that fails to reproduce it — the identical finding now
// lives ONLY in an AdditionalSets entry (not-resurfaced:, in the real
// pipeline) instead of Findings. The accepted-deviation count must be
// EXACTLY UNCHANGED across the two rounds — never decremented (the X-18
// laundering drain this story closes) and never inflated.
func TestSpecStale_LaunderingReplay_CountUnchangedAcrossRegeneration(t *testing.T) {
	f := judgedAcceptedDeviation("judged-standing", "an old, settled adjudication", "owner-ratified")

	round1 := SpecStale(SpecStaleInput{Findings: []artifact.Finding{f}, StoryACIDs: map[string]bool{}})
	round2 := SpecStale(SpecStaleInput{Findings: nil, AdditionalSets: [][]artifact.Finding{{f}}, StoryACIDs: map[string]bool{}})

	if round1.AcceptedDeviationCount != round2.AcceptedDeviationCount {
		t.Fatalf("round1.AcceptedDeviationCount = %d, round2 = %d — want EXACTLY unchanged across the re-roll (X-18 laundering drain)", round1.AcceptedDeviationCount, round2.AcceptedDeviationCount)
	}
	if round2.AcceptedDeviationCount != 1 {
		t.Fatalf("round2.AcceptedDeviationCount = %d, want 1", round2.AcceptedDeviationCount)
	}
}
