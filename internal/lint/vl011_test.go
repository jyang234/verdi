package lint

import (
	"path/filepath"
	"testing"
)

func TestVL011_PathIDMismatch(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-011"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-011")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl011WaiverNoOwnerNoReason = `---
id: waiver/story-1482--ac-9
kind: waiver
title: "VL-011: waiver missing owner and reason"
owners: []
status: active
frozen: { at: 2026-05-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-011: missing owner and reason
`

// TestVL011_WaiverMissingOwnerOrReason covers the rule's other clause
// ("waiver has owner + reason, expiry optional") that the one testdata
// overlay (path/id mismatch) does not exercise.
func TestVL011_WaiverMissingOwnerOrReason(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/waivers/story-1482/ac-9.md", vl011WaiverNoOwnerNoReason)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-011")
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2 (missing owner, missing reason):\n%s", len(findings), findingsString(findings))
	}
}
