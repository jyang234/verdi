package decisionsweep

import (
	"testing"
	"time"
)

func waiverMD(acID, status, reason, expiry string) string {
	expiryLine := ""
	if expiry != "" {
		expiryLine = "expiry: " + expiry + "\n"
	}
	return "---\nid: waiver/jira-loan-1--" + acID + "\nkind: waiver\ntitle: \"waiver\"\nowners: [platform-team]\n" +
		"status: " + status + "\nreason: \"" + reason + "\"\n" + expiryLine +
		"links:\n  - { type: verifies, ref: spec/my-story }\n" +
		"frozen: { at: 2026-07-19, commit: 8c2d41f }\n---\nbody\n"
}

// TestAudit_WaiverStale_ActiveUnderThreshold proves an active, unexpired
// waiver counts and passes clean when under the configured threshold.
func TestAudit_WaiverStale_ActiveUnderThreshold(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-1.md", waiverMD("ac-1", "active", "hotfix", "2026-08-01"))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.WaiverStale) != 1 {
		t.Fatalf("WaiverStale = %+v, want exactly 1 entry", result.WaiverStale)
	}
	entry := result.WaiverStale[0]
	if entry.StoryRef != "spec/my-story" {
		t.Fatalf("StoryRef = %q", entry.StoryRef)
	}
	if entry.ActiveCount != 1 || entry.Flagged {
		t.Fatalf("entry = %+v, want ActiveCount 1, not flagged (1 <= threshold 3)", entry)
	}
	if len(entry.Waivers) != 1 || !entry.Waivers[0].CountsActive || entry.Waivers[0].Lapsed {
		t.Fatalf("Waivers = %+v, want one counting-active, non-lapsed row", entry.Waivers)
	}
}

// TestAudit_WaiverStale_ThresholdCrossedFlags proves crossing the
// configured threshold flags the story by name and contributes to Audit's
// overall FLAGGED verdict.
func TestAudit_WaiverStale_ThresholdCrossedFlags(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1", "ac-2"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-1.md", waiverMD("ac-1", "active", "hotfix one", "2026-08-01"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-2.md", waiverMD("ac-2", "active", "hotfix two", ""))

	// threshold 1: two active waivers exceeds it.
	result, err := Audit(root, 3, 3, 1, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.WaiverStale) != 1 {
		t.Fatalf("WaiverStale = %+v, want exactly 1 entry", result.WaiverStale)
	}
	entry := result.WaiverStale[0]
	if entry.ActiveCount != 2 || !entry.Flagged {
		t.Fatalf("entry = %+v, want ActiveCount 2, flagged (2 > threshold 1)", entry)
	}
}

// TestAudit_WaiverStale_ExpiredStatusExcludedButDisclosed proves a waiver
// whose committed status is already "expired" is excluded from the active
// count but still listed in the disclosure, never silently dropped.
func TestAudit_WaiverStale_ExpiredStatusExcludedButDisclosed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-1.md", waiverMD("ac-1", "expired", "old hotfix", "2026-01-01"))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.WaiverStale) != 1 {
		t.Fatalf("WaiverStale = %+v, want exactly 1 entry (disclosed even with 0 active)", result.WaiverStale)
	}
	entry := result.WaiverStale[0]
	if entry.ActiveCount != 0 {
		t.Fatalf("ActiveCount = %d, want 0 (status: expired never counts)", entry.ActiveCount)
	}
	if len(entry.Waivers) != 1 || entry.Waivers[0].CountsActive {
		t.Fatalf("Waivers = %+v, want the row present but CountsActive false", entry.Waivers)
	}
}

// TestAudit_WaiverStale_LapsedByDateExcludedButDisclosed proves a waiver
// still marked status: active, but whose recorded expiry has passed by
// wall-clock as of the scan's `now`, is excluded from the active count
// (guide 8.4: "past expiry the waiver lapses") while still disclosed with
// Lapsed=true — never silently dropped.
func TestAudit_WaiverStale_LapsedByDateExcludedButDisclosed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-1.md", waiverMD("ac-1", "active", "hotfix", "2026-08-01"))

	lapsedNow := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC) // the day after expiry
	result, err := Audit(root, 3, 3, 3, lapsedNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	entry := result.WaiverStale[0]
	if entry.ActiveCount != 0 {
		t.Fatalf("ActiveCount = %d, want 0 (lapsed by date)", entry.ActiveCount)
	}
	if len(entry.Waivers) != 1 || entry.Waivers[0].CountsActive || !entry.Waivers[0].Lapsed {
		t.Fatalf("Waivers = %+v, want the row present, CountsActive false, Lapsed true", entry.Waivers)
	}
}

// TestAudit_WaiverStale_StoryWithNoWaiversSkipped proves a story with no
// waiver files at all is skipped entirely, mirroring ScanSpecStale's own
// "no report yet, skip" posture — never flagged, never listed.
func TestAudit_WaiverStale_StoryWithNoWaiversSkipped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))

	result, err := Audit(root, 3, 3, 3, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if len(result.WaiverStale) != 0 {
		t.Fatalf("WaiverStale = %+v, want none (no waiver files at all)", result.WaiverStale)
	}
}

// TestAudit_WaiverStale_ThresholdAbsentDefaults proves threshold <= 0
// (verdi.yaml's audit.waivers_stale_threshold absent) substitutes
// DefaultWaiversStaleThreshold, exactly as the deviations threshold's own
// consumer already does.
func TestAudit_WaiverStale_ThresholdAbsentDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeFile(t, root, ".verdi/specs/active/my-story/spec.md", storySpecMD("my-story", "ac-1"))
	writeFile(t, root, ".verdi/waivers/jira-loan-1/ac-1.md", waiverMD("ac-1", "active", "hotfix", ""))

	result, err := Audit(root, 3, 3, 0, testAuditNow)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if result.WaiverStale[0].Threshold != DefaultWaiversStaleThreshold {
		t.Fatalf("Threshold = %d, want default %d", result.WaiverStale[0].Threshold, DefaultWaiversStaleThreshold)
	}
}

// TestWaiverLapsed is waiverLapsed's own happy/negative table: day-
// granularity boundary (still active THROUGH the expiry day itself, lapsed
// starting the day after), no expiry, and a malformed date degrading to
// "never lapsed" rather than erroring.
func TestWaiverLapsed(t *testing.T) {
	tests := []struct {
		name   string
		expiry string
		now    time.Time
		want   bool
	}{
		{"no expiry never lapses", "", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), false},
		{"on the expiry day itself is not yet lapsed", "2026-08-01", time.Date(2026, 8, 1, 23, 59, 0, 0, time.UTC), false},
		{"the day after has lapsed", "2026-08-01", time.Date(2026, 8, 2, 0, 0, 1, 0, time.UTC), true},
		{"well before is not lapsed", "2026-08-01", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), false},
		{"malformed expiry degrades to not-lapsed", "not-a-date", time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waiverLapsed(tt.expiry, tt.now); got != tt.want {
				t.Errorf("waiverLapsed(%q, %v) = %v, want %v", tt.expiry, tt.now, got, tt.want)
			}
		})
	}
}
