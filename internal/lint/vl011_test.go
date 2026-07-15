package lint

import (
	"path/filepath"
	"strings"
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
frozen: { at: 2026-05-01, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
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

const vl011ReaffirmationMismatchedPath = `---
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "VL-011: reaffirmation path mismatch"
schema: verdi.reaffirmation/v1
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
object: spec/stale-decline@89f9926e9739b97e23eb52efb16206d0ff10ff4f#ac-2
hash: { old: sha256:20bb0d914cc85a12dbb4c5e85f099b69cae126b0a395780d10b98327da844bfc, new: sha256:ca80c24cd423a030096c07d690b96bfd7dcc801219a5815e0679269a6d699c97 }
---
# VL-011: reaffirmation path mismatch
`

// TestVL011_ReaffirmationPathIDMismatch is VL-011's rescope-adjacent
// completion (found during this phase's own v2-fixture-corpus exit
// criterion: reaffirmation was never wired into the walk or this rule at
// all — 02 §Lint rules names it explicitly in VL-011's row): a
// reaffirmation whose id implies a different nested path than the one it
// lives at fails VL-011, the same way an attestation/waiver would.
func TestVL011_ReaffirmationPathIDMismatch(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/reaffirmations/wrong-story/ac-2.md", vl011ReaffirmationMismatchedPath)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-011")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl011ReaffirmationCorrectPath = `---
id: reaffirmation/jira-loan-1482--ac-2
kind: reaffirmation
title: "VL-011: reaffirmation at the correct nested path"
schema: verdi.reaffirmation/v1
owners: [loansvc-team]
frozen: { at: 2026-07-14, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
object: spec/stale-decline@89f9926e9739b97e23eb52efb16206d0ff10ff4f#ac-2
hash: { old: sha256:20bb0d914cc85a12dbb4c5e85f099b69cae126b0a395780d10b98327da844bfc, new: sha256:ca80c24cd423a030096c07d690b96bfd7dcc801219a5815e0679269a6d699c97 }
---
# VL-011: reaffirmation at the correct nested path
`

// TestVL011_ReaffirmationCorrectPath_Clean is the positive complement.
func TestVL011_ReaffirmationCorrectPath_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/reaffirmations/jira-loan-1482/ac-2.md", vl011ReaffirmationCorrectPath)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-011" {
			t.Fatalf("VL-011 fired on a correctly-nested reaffirmation: %s", f.String())
		}
	}
}

// TestVL007_ReaffirmationsTopLevelDir_Known proves the VL-007 fix this
// phase made (found via the same v2-fixture-corpus exit criterion):
// "reaffirmations" is a known top-level .verdi/ entry, not an unrecognized
// one.
func TestVL007_ReaffirmationsTopLevelDir_Known(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/reaffirmations/jira-loan-1482/ac-2.md", vl011ReaffirmationCorrectPath)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-007" {
			t.Fatalf("VL-007 fired on the reaffirmations/ top-level directory: %s", f.String())
		}
	}
}

// TestVL011_ObligationPathIDMismatch proves VL-011's obligation-kind
// extension (spec/obligation-artifact DC-2): an obligation id
// "<story-slug>--<ac-id>--<for-kind>" implies path
// ".verdi/obligations/<story-slug>/<ac-id>--<for-kind>.md" — one level
// deeper than attestation/waiver/reaffirmation's two-segment shape, since
// the filename itself folds in the id's last two segments. A file at any
// other path disagrees. Reuses vl019_test.go's own vl019StorySpecMD /
// vl019StoryACCleanMD fixtures (same package) rather than duplicating a
// second small story-spec-plus-obligation pair.
func TestVL011_ObligationPathIDMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-019-story/spec.md"), vl019StorySpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi/obligations/wrong-story/ac-1--behavioral.md"), vl019StoryACCleanMD)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-011")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	for _, want := range []string{"vl-019-story", "ac-1", "behavioral"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Errorf("finding does not mention %q: %s", want, findings[0].Message)
		}
	}
}

// TestVL011_ObligationCorrectPath_Clean is the positive complement: an
// obligation placed exactly where its id implies never fires VL-011.
func TestVL011_ObligationCorrectPath_Clean(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, ".verdi/specs/active/vl-019-story/spec.md"), vl019StorySpecMD)
	writeTestFile(t, filepath.Join(dir, ".verdi/obligations/vl-019-story/ac-1--behavioral.md"), vl019StoryACCleanMD)

	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-011" {
			t.Fatalf("VL-011 fired on a correctly-nested obligation: %s", f.String())
		}
	}
}
