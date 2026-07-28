package evidence

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func decodeWaiverBytes(t *testing.T, content string) (*artifact.WaiverFrontmatter, string) {
	t.Helper()
	fm, body, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	w, err := artifact.DecodeWaiver(fm)
	if err != nil {
		t.Fatalf("DecodeWaiver: %v\ncontent:\n%s", err, content)
	}
	return w, string(body)
}

func baseWaiverInput() WaiverInput {
	return WaiverInput{
		StorySlug:   "jira-loan-1482",
		ACID:        "ac-1",
		StoryRefArg: "task/retry-worker",
		VerifiesRef: "spec/retry-worker",
		Owners:      []string{"loansvc-team"},
		Reason:      "hotfix for PSP outage; tracked in PAY-1519",
		Expiry:      "2026-08-01",
		Frozen:      artifact.NewFrozen("2026-07-19", "8c2d41f"),
	}
}

// TestRenderWaiver_Happy proves a freshly-created waiver decodes cleanly,
// with status: active, the given reason/expiry, owners copied verbatim,
// and exactly one "waived" log entry in its body.
func TestRenderWaiver_Happy(t *testing.T) {
	in := baseWaiverInput()
	content := RenderWaiver(in)

	w, body := decodeWaiverBytes(t, content)
	if w.ID != "waiver/jira-loan-1482--ac-1" {
		t.Errorf("ID = %q", w.ID)
	}
	if w.Status != "active" {
		t.Errorf("Status = %q, want active", w.Status)
	}
	if w.Reason != in.Reason {
		t.Errorf("Reason = %q, want %q", w.Reason, in.Reason)
	}
	if w.Expiry != in.Expiry {
		t.Errorf("Expiry = %q, want %q", w.Expiry, in.Expiry)
	}
	if len(w.Owners) != 1 || w.Owners[0] != "loansvc-team" {
		t.Errorf("Owners = %v", w.Owners)
	}
	if w.Frozen == nil || w.Frozen.At != "2026-07-19" || w.Frozen.Commit != "8c2d41f" {
		t.Errorf("Frozen = %+v", w.Frozen)
	}
	if strings.Count(body, waiverReaffirmationLogMarker) != 1 {
		t.Fatalf("body must carry exactly one log marker, got:\n%s", body)
	}
	if !strings.Contains(body, "waived") {
		t.Errorf("body missing a %q log entry:\n%s", waiverLogKindWaived, body)
	}
	if strings.Contains(body, "reaffirmed") {
		t.Errorf("a fresh waiver's body must not yet carry a reaffirmed entry:\n%s", body)
	}
}

// TestRenderWaiver_NoExpiry proves an absent --expires renders with no
// expiry: field at all (WaiverFrontmatter's own omitempty contract) and
// the log entry discloses "no expiry" rather than a fabricated date.
func TestRenderWaiver_NoExpiry(t *testing.T) {
	in := baseWaiverInput()
	in.Expiry = ""
	content := RenderWaiver(in)

	if strings.Contains(content, "expiry:") {
		t.Errorf("no-expiry render must omit the expiry: field entirely:\n%s", content)
	}
	w, body := decodeWaiverBytes(t, content)
	if w.Expiry != "" {
		t.Errorf("Expiry = %q, want empty", w.Expiry)
	}
	if !strings.Contains(body, "no expiry") {
		t.Errorf("body must disclose \"no expiry\":\n%s", body)
	}
}

// TestRenderWaiverReaffirm_AppendsLogEntry proves a reaffirmation refreshes
// frontmatter to the new invocation while the body's log gains exactly one
// new "reaffirmed" entry AFTER the prior entry, which survives verbatim.
func TestRenderWaiverReaffirm_AppendsLogEntry(t *testing.T) {
	original := baseWaiverInput()
	createContent := RenderWaiver(original)
	_, createBody := decodeWaiverBytes(t, createContent)

	reaffirm := WaiverInput{
		StorySlug:   original.StorySlug,
		ACID:        original.ACID,
		StoryRefArg: original.StoryRefArg,
		VerifiesRef: original.VerifiesRef,
		Owners:      original.Owners,
		Reason:      "still flaking on the CI runner; PAY-1519 not yet fixed",
		Expiry:      "2026-08-15",
		Frozen:      artifact.NewFrozen("2026-07-25", "9f3a220"),
	}
	reaffirmContent := RenderWaiverReaffirm(createBody, reaffirm)
	w, body := decodeWaiverBytes(t, reaffirmContent)

	if w.Reason != reaffirm.Reason {
		t.Errorf("Reason = %q, want the fresh rationale %q", w.Reason, reaffirm.Reason)
	}
	if w.Expiry != reaffirm.Expiry {
		t.Errorf("Expiry = %q, want %q", w.Expiry, reaffirm.Expiry)
	}
	if w.Status != "active" {
		t.Errorf("Status = %q, want active (a reaffirm un-lapses)", w.Status)
	}
	if w.Frozen == nil || w.Frozen.At != "2026-07-25" || w.Frozen.Commit != "9f3a220" {
		t.Errorf("Frozen = %+v, want the fresh stamp", w.Frozen)
	}

	// The prior entry survives verbatim...
	priorEntry := waiverLogEntry(waiverLogKindWaived, original.Frozen.At, original.Reason, original.Expiry)
	if !strings.Contains(body, priorEntry) {
		t.Fatalf("reaffirmed body must carry the PRIOR log entry verbatim %q, got:\n%s", priorEntry, body)
	}
	// ...and exactly one new entry is appended after it.
	newEntry := waiverLogEntry(waiverLogKindReaffirmed, reaffirm.Frozen.At, reaffirm.Reason, reaffirm.Expiry)
	if !strings.Contains(body, newEntry) {
		t.Fatalf("reaffirmed body must carry the NEW log entry %q, got:\n%s", newEntry, body)
	}
	if strings.Index(body, priorEntry) > strings.Index(body, newEntry) {
		t.Errorf("prior entry must precede the new entry (append, not prepend):\n%s", body)
	}
	if strings.Count(body, waiverReaffirmationLogMarker) != 1 {
		t.Errorf("reaffirmed body must still carry exactly one marker (never duplicated):\n%s", body)
	}
}

// TestRenderWaiverReaffirm_NoPriorMarker proves reaffirming a body that
// never carried this mechanism's marker (a hand-authored waiver, or one
// predating it) does not fabricate history: the log simply starts fresh
// with the one entry this reaffirmation is, rather than erroring.
func TestRenderWaiverReaffirm_NoPriorMarker(t *testing.T) {
	handAuthoredBody := "Waived by hand; see the incident channel for context.\n"
	in := baseWaiverInput()
	content := RenderWaiverReaffirm(handAuthoredBody, in)

	w, body := decodeWaiverBytes(t, content)
	if w.Reason != in.Reason {
		t.Errorf("Reason = %q, want %q", w.Reason, in.Reason)
	}
	if strings.Count(body, waiverReaffirmationLogMarker) != 1 {
		t.Fatalf("body must carry exactly one marker even with no prior log:\n%s", body)
	}
	if strings.Count(body, "- 20") != 1 {
		t.Errorf("body must carry exactly one dated log entry (no fabricated prior history):\n%s", body)
	}
}

// TestExtractReaffirmationLog is extractReaffirmationLog's own happy/
// negative table.
func TestExtractReaffirmationLog(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantFound bool
	}{
		{"marker present", "prose\n" + waiverReaffirmationLogMarker + "\n" + waiverLogHeading + "\n\n- entry\n", true},
		{"no marker at all", "just hand-written prose, no log here\n", false},
		{"empty body", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, found := extractReaffirmationLog(tt.body)
			if found != tt.wantFound {
				t.Fatalf("found = %v, want %v", found, tt.wantFound)
			}
			if found && !strings.HasPrefix(log, waiverReaffirmationLogMarker) {
				t.Errorf("extracted log must start at the marker, got %q", log)
			}
			if !found && log != "" {
				t.Errorf("not-found case must return an empty log, got %q", log)
			}
		})
	}
}
