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
