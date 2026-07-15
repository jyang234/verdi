package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

const testActiveWaiver = `---
id: waiver/story-1--ac-4
kind: waiver
title: "Runtime check deferred (test)"
status: active
owners: [platform-team]
reason: "runtime probe mechanism not yet built"
frozen: { at: 2026-05-01, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# Waiver
`

const testExpiredWaiver = `---
id: waiver/story-1--ac-3
kind: waiver
title: "Golden gap (test, expired)"
status: expired
owners: [platform-team]
reason: "golden flow pending test-data fixture"
expiry: 2026-06-01
frozen: { at: 2026-05-01, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# Waiver
`

func writeWaiver(t *testing.T, root, storySlug, acID, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "waivers", storySlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, acID+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing waiver: %v", err)
	}
}

// TestWaiverActive_Happy proves an active waiver waives and a missing
// waiver does not.
func TestWaiverActive_Happy(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "story-1", "ac-4", testActiveWaiver)

	active, err := WaiverActive(root, "story-1", "ac-4")
	if err != nil {
		t.Fatalf("WaiverActive: %v", err)
	}
	if !active {
		t.Fatal("WaiverActive(active waiver) = false, want true")
	}

	active, err = WaiverActive(root, "story-1", "ac-999")
	if err != nil {
		t.Fatalf("WaiverActive(no file): %v", err)
	}
	if active {
		t.Fatal("WaiverActive(no waiver file) = true, want false")
	}
}

// TestWaiverActive_Expired proves an expired waiver is present but does
// NOT waive (03 §The fold: "expired waivers do NOT waive").
func TestWaiverActive_Expired(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "story-1", "ac-3", testExpiredWaiver)

	active, err := WaiverActive(root, "story-1", "ac-3")
	if err != nil {
		t.Fatalf("WaiverActive: %v", err)
	}
	if active {
		t.Fatal("WaiverActive(expired waiver) = true, want false")
	}
}

// TestWaiverActive_Negative proves a malformed waiver file (fails strict
// decode) surfaces as an error rather than silently reading as "no
// waiver".
func TestWaiverActive_Negative(t *testing.T) {
	root := t.TempDir()
	writeWaiver(t, root, "story-1", "ac-1", "not-frontmatter-at-all")

	if _, err := WaiverActive(root, "story-1", "ac-1"); err == nil {
		t.Fatal("WaiverActive(malformed waiver): want error, got nil")
	}
}
