package evidence

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestCountAcceptedDeviations_CarriedFromCollapsesCrossLevelSlug pins L-N14's
// union arithmetic for the cross-level reaffirmation case (ledger L-N14
// companion): a confirmed feature-level reaffirmation (carried-from set) of an
// archived story ruling is the SAME deviation as that archived ruling, even
// though the feature judge reworded the text under the same slug. The two must
// collapse to ONE in the feature-close union — never counted twice (once in the
// story archive under the old text, once in the feature report under the new
// text).
//
// Red-first: the content-identity union (kind+id+text, which excludes
// carried-from) reads the reworded texts as two distinct identities and counts 2.
func TestCountAcceptedDeviations_CarriedFromCollapsesCrossLevelSlug(t *testing.T) {
	sha := strings.Repeat("a", 40)
	storyArchive := []artifact.Finding{
		{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "OLD story-level text", Disposition: artifact.FindingAcceptedDeviation, Note: "n"},
	}
	featureReport := []artifact.Finding{
		{ID: "judged-retry-semantics", Kind: artifact.FindingJudged, Text: "NEW feature-level wording", Disposition: artifact.FindingAcceptedDeviation, Note: "reaffirmed", CarriedFrom: sha},
	}
	if got := CountAcceptedDeviations(featureReport, storyArchive); got != 1 {
		t.Fatalf("CountAcceptedDeviations = %d, want 1 — a carried-from feature reaffirmation collapses with the same-slug archived ruling it reaffirms", got)
	}
}

// TestCountAcceptedDeviations_NoCarriedFrom_SameSlugDifferentText_CountsTwice
// guards the collapse's precondition: WITHOUT a carried-from reaffirmation, two
// same-slug accepted-deviations with DIFFERENT text remain distinct content
// identities and count twice — the collapse fires ONLY on a human-confirmed
// carried-from reaffirmation, never on a bare slug coincidence (which would
// silently under-count two genuinely different rulings that merely reused a slug).
func TestCountAcceptedDeviations_NoCarriedFrom_SameSlugDifferentText_CountsTwice(t *testing.T) {
	a := []artifact.Finding{{ID: "judged-s", Kind: artifact.FindingJudged, Text: "ruling one", Disposition: artifact.FindingAcceptedDeviation, Note: "n"}}
	b := []artifact.Finding{{ID: "judged-s", Kind: artifact.FindingJudged, Text: "ruling two", Disposition: artifact.FindingAcceptedDeviation, Note: "n"}}
	if got := CountAcceptedDeviations(a, b); got != 2 {
		t.Fatalf("CountAcceptedDeviations = %d, want 2 — without carried-from, same-slug different-text ADs are distinct identities", got)
	}
}

// TestCountAcceptedDeviations_CarriedFromWithinReport_CountsOnce keeps the
// ordinary within-report reaffirmation count correct: a confirmed reaffirmation
// standing alone at its slug (the disposition verb removed its backing record) is
// one accepted-deviation, exactly as before the collapse layered on.
func TestCountAcceptedDeviations_CarriedFromWithinReport_CountsOnce(t *testing.T) {
	sha := strings.Repeat("b", 40)
	report := []artifact.Finding{
		{ID: "judged-x", Kind: artifact.FindingJudged, Text: "reaffirmed ruling", Disposition: artifact.FindingAcceptedDeviation, Note: "n", CarriedFrom: sha},
		{ID: "computed-y", Kind: artifact.FindingComputed, Text: "unrelated", Disposition: artifact.FindingAcceptedDeviation, Note: "n"},
	}
	if got := CountAcceptedDeviations(report); got != 2 {
		t.Fatalf("CountAcceptedDeviations = %d, want 2 — a lone carried-from finding plus one unrelated AD", got)
	}
}

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

// TestSpecStale_OwnNotResurfaced_DoesNotFeedTriggerA is spec/finding-identity
// judged-spec-stale-own-text-judged-id-prefix: trigger (a)'s own-text join
// reads ONLY Findings, never OwnNotResurfaced. The OwnNotResurfaced scan the
// prior fix (commit 5f3e435) added was unreachable by construction — every
// entry Generate ever writes to not-resurfaced: is judged-kind, every judged
// finding id is "judged-"+slug (judge.go), and every AC id matches ^ac-
// (artifact acIDRe), so a not-resurfaced entry's id can NEVER equal an AC id
// and can never trip trigger (a). The id-shape disjointness is pinned directly
// in internal/align (TestNotResurfacedIDsCanNeverBeACIDs).
//
// This test constructs the ONLY shape that could ever have tripped the dead
// scan — a not-resurfaced accepted-deviation whose id is AC-shaped, which no
// real judged finding can carry — and pins that it does NOT fire trigger (a).
// Red-first (before removing the dead scan): the OwnNotResurfaced scan fired on
// the AC-shaped id and set Flagged=true with OwnTextFindingIDs=[ac-3].
func TestSpecStale_OwnNotResurfaced_DoesNotFeedTriggerA(t *testing.T) {
	in := SpecStaleInput{
		Findings:         nil,
		OwnNotResurfaced: []artifact.Finding{judgedAcceptedDeviation("ac-3", "an AC-shaped id no real judged finding can carry", "owner-ratified")},
		StoryACIDs:       map[string]bool{"ac-1": true, "ac-2": true, "ac-3": true},
	}
	got := SpecStale(in)
	if len(got.OwnTextFindingIDs) != 0 {
		t.Fatalf("OwnTextFindingIDs = %v, want none — trigger (a) must read only Findings, never OwnNotResurfaced (the scan is unreachable by id-shape construction)", got.OwnTextFindingIDs)
	}
	// One accepted-deviation total, default threshold 3: trigger (b) does not
	// fire either, so with trigger (a) correctly silent the report is unflagged.
	if got.Flagged {
		t.Fatalf("Flagged = true, want false — no own-text trigger from OwnNotResurfaced and only one accepted-deviation (< threshold)")
	}
	// The entry is still COUNTED toward the budget (trigger b) — here 1, under
	// threshold. The reachable protection is pinned in full below.
	if got.AcceptedDeviationCount != 1 {
		t.Fatalf("AcceptedDeviationCount = %d, want 1 (OwnNotResurfaced still feeds trigger (b)'s budget)", got.AcceptedDeviationCount)
	}
}

// TestSpecStale_OwnNotResurfaced_OnlyThere_StillCountsBudget pins the REAL,
// reachable protection OwnNotResurfaced provides — the one that STAYS after the
// dead trigger-(a) scan is removed: a realistically-shaped judged
// accepted-deviation ("judged-"-prefixed id, the only shape not-resurfaced:
// ever holds) present ONLY in OwnNotResurfaced still counts toward trigger
// (b)'s budget, so a standing adjudication that stopped reproducing never
// drains out of the count just because it moved out of findings: (ac-3's X-18
// laundering fix). This FAILs if OwnNotResurfaced is ever dropped from trigger
// (b) too — it fences the exact boundary of the removal.
func TestSpecStale_OwnNotResurfaced_OnlyThere_StillCountsBudget(t *testing.T) {
	in := SpecStaleInput{
		Findings: []artifact.Finding{
			judgedAcceptedDeviation("judged-a", "text a", "n"),
			judgedAcceptedDeviation("judged-b", "text b", "n"),
			judgedAcceptedDeviation("judged-c", "text c", "n"),
		},
		OwnNotResurfaced: []artifact.Finding{judgedAcceptedDeviation("judged-standing", "an old, settled adjudication", "owner-ratified")},
		StoryACIDs:       map[string]bool{},
		Threshold:        3,
	}
	got := SpecStale(in)
	if got.AcceptedDeviationCount != 4 {
		t.Fatalf("AcceptedDeviationCount = %d, want 4 (3 in findings + 1 present only in not-resurfaced)", got.AcceptedDeviationCount)
	}
	if !got.TriggeredByThreshold {
		t.Fatal("TriggeredByThreshold = false, want true (4 > threshold 3 — the not-resurfaced entry pushed the budget over)")
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
