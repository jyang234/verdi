// Regression proofs for the frontmatter-scoping defect in `verdi close`:
// closeAcceptedStatusLineRe used to be counted and applied over the WHOLE
// spec.md document, so a status-SHAPED line in the markdown BODY (prose
// quoting the legacy field, an unfenced example) was read as if it were the
// frontmatter status field. Two consequences, both proven below:
//
//   - a STATUSLESS (merge-accepted) spec whose body carries such a line was
//     steered onto the legacy-flip path and had its BODY rewritten — a
//     content mutation of an immutable, accepted artifact;
//   - a legacy statused spec whose body carries the same line counted two
//     "status lines" and was refused outright, though only one of them is a
//     status field at all.
//
// Both sites are now scoped to the frontmatter byte range (the one
// frontmatter seam, artifact.FrontmatterRange), so the body is never read as
// a status field and never rewritten.
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// bodyStatusLineSpecMD renders the fixture story: an evidence: [runtime]
// story whose BODY quotes the legacy status field verbatim, on its own
// unfenced line — the exact byte shape closeAcceptedStatusLineRe matches.
// frontmatterStatus selects the two shapes under test: "" is the statusless
// (merge-accepted) spec, and a non-empty value is spliced in as a real
// frontmatter status line alongside the legacy frozen: stamp.
func bodyStatusLineSpecMD(frontmatterStatus string) string {
	statusLine := ""
	frozenLine := ""
	if frontmatterStatus != "" {
		statusLine = "status: " + frontmatterStatus + "\n"
		frozenLine = "frozen: { at: 2024-01-01, commit: " + gateFakeFrozenCommit + "}\n"
	}
	return `---
id: spec/body-status-line
kind: spec
class: story
title: "Body status line story"
` + statusLine + `owners: [platform-team]
story: jira:BODY-STATUS-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the body status-line fixture check holds", evidence: [runtime] }
` + frozenLine + `---
# Body status line story
## Problem
Legacy specs carried their lifecycle in a frontmatter field. This paragraph
quotes one verbatim, unfenced, on a line of its very own:

status: accepted-pending-build

It is prose about the field, never this spec's own status.
## Outcome
y
`
}

// buildBodyStatusLineRepo mirrors buildStatuslessCloseFixtureRepo
// (close_statusless_test.go) for this file's own fixture story.
// CI_DEFAULT_BRANCH pins the projector's default-branch resolution (a
// fixturegit repo has no origin remote).
func buildBodyStatusLineRepo(t *testing.T, specMD string) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md":        featureV1SpecMD,
			".verdi/specs/active/body-status-line/spec.md": specMD,
		},
		Message: "body status-line fixture: feature + story whose body quotes the legacy status field",
	}})
}

// writeBodyStatusLineGateReport mirrors writeCloseGateReport for this
// file's differently-named fixture (closure-gate condition 4 needs a living,
// fully-dispositioned, head-covering report).
func writeBodyStatusLineGateReport(t *testing.T, root, covers string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "body-status-line")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nschema: verdi.deviation/v1\ncovers: " + covers + "\nfindings:\n" + dispositionedFindingYAML + "\ndigest: sha256:" + strings.Repeat("0", 64) + "\n---\n# Alignment report\n"
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

// closeBodyStatusLineFixture runs the whole ritual for specMD and returns
// the archived spec.md's bytes plus the git name-status diff of the closure.
func closeBodyStatusLineFixture(t *testing.T, specMD string) ([]byte, string, *fixturegit.Repo) {
	t.Helper()
	t.Setenv("CI", "true") // a genuine, detected CI environment: stamps source: ci
	repo := buildBodyStatusLineRepo(t, specMD)
	ctx := context.Background()

	_, rtDeps, rtStdout, rtStderr := fakeRuntimeDeps()
	if code := runProduceRuntime(ctx, repo.Dir, repo.Head, "spec/body-status-line", "ac-1", "GET /healthz -> 200", artifact.VerdictPass, false, rtDeps); code != 0 {
		t.Fatalf("runProduceRuntime = %d, want 0; stdout=%s stderr=%s", code, rtStdout.String(), rtStderr.String())
	}
	writeBodyStatusLineGateReport(t, repo.Dir, repo.Head)

	closeD := closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	if got := runClose(ctx, repo.Dir, "spec/body-status-line", &store.Manifest{}, closeD, &stdout, &stderr); got != 0 {
		t.Fatalf("runClose = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	archived, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "body-status-line", "spec.md"))
	if err != nil {
		t.Fatalf("reading archived spec.md: %v", err)
	}
	return archived, gitOutput(t, repo.Dir, "diff", "--name-status", "-M", repo.Head, "HEAD"), repo
}

// assertNoVL010AfterClose re-lints the post-close store and fails on any
// VL-010 finding — the archive move must stay admissible under the
// frozen-immutability rule (pure rename, or the D6-11 status-only flip).
func assertNoVL010AfterClose(t *testing.T, root, diffBase string) {
	t.Helper()
	findings, err := lint.NewEngine().Run(context.Background(), root, lint.Context{DiffBase: diffBase}, lint.Options{})
	if err != nil {
		t.Fatalf("re-lint of post-close store: %v", err)
	}
	for _, f := range findings {
		if f.Rule == "VL-010" {
			t.Fatalf("re-lint of post-close store fired VL-010 on the archive move: %s", f.String())
		}
	}
}

// TestRunClose_StatuslessSpec_BodyStatusLineNotFlipped is the immutability
// proof: a STATUSLESS, merge-accepted spec whose BODY carries a literal
// `status: accepted-pending-build` line still takes the PURE-RENAME path —
// the archived spec.md is byte-identical to the original, body included.
// Pre-fix, the whole-document match counted that body line as the status
// field, took the legacy-flip path, and rewrote the prose.
func TestRunClose_StatuslessSpec_BodyStatusLineNotFlipped(t *testing.T) {
	specMD := bodyStatusLineSpecMD("")
	archived, diffOut, repo := closeBodyStatusLineFixture(t, specMD)

	if !bytes.Equal(archived, []byte(specMD)) {
		t.Fatalf("archived spec.md is not byte-identical to the statusless original:\n--- got ---\n%s\n--- want ---\n%s", archived, specMD)
	}
	pureRename := regexp.MustCompile(`R100\t\.verdi/specs/active/body-status-line/spec\.md\t\.verdi/specs/archive/body-status-line/spec\.md`)
	if !pureRename.MatchString(diffOut) {
		t.Fatalf("git diff --name-status -M did not report a PURE (R100) rename for the statusless spec.md:\n%s", diffOut)
	}
	assertNoVL010AfterClose(t, repo.Dir, repo.Head)
}

// TestRunClose_LegacyStatus_OnlyFrontmatterStatusLineFlipped is the
// counterpart: a legacy spec carrying BOTH a frontmatter
// `status: accepted-pending-build` field AND the same literal line in its
// body flips exactly one line — the frontmatter one — and every body byte
// survives. Pre-fix the whole-document count read two status lines and
// refused the closure outright (exit 2) even though only one of them was
// ever a status field.
func TestRunClose_LegacyStatus_OnlyFrontmatterStatusLineFlipped(t *testing.T) {
	specMD := bodyStatusLineSpecMD("accepted-pending-build")
	archived, diffOut, repo := closeBodyStatusLineFixture(t, specMD)

	// Exactly one replacement, and it is the FIRST occurrence — the
	// frontmatter field, which precedes the body by construction.
	want := strings.Replace(specMD, "status: accepted-pending-build", "status: closed", 1)
	if !bytes.Equal(archived, []byte(want)) {
		t.Fatalf("archived spec.md is not the original with a sole FRONTMATTER status flip:\n--- got ---\n%s\n--- want ---\n%s", archived, want)
	}

	// The body's own status-shaped line is untouched, byte for byte.
	_, body, err := artifact.SplitFrontmatter(archived)
	if err != nil {
		t.Fatalf("SplitFrontmatter(archived): %v", err)
	}
	_, wantBody, err := artifact.SplitFrontmatter([]byte(specMD))
	if err != nil {
		t.Fatalf("SplitFrontmatter(original): %v", err)
	}
	if !bytes.Equal(body, wantBody) {
		t.Fatalf("archived body bytes changed:\n--- got ---\n%s\n--- want ---\n%s", body, wantBody)
	}
	if !strings.Contains(string(body), "\nstatus: accepted-pending-build\n") {
		t.Fatalf("the body's quoted status line did not survive the flip:\n%s", body)
	}

	// The move is a rename with content (the status flip), not R100 — and
	// VL-010's D6-11 status-only exception still admits it.
	renamed := regexp.MustCompile(`R\d+\t\.verdi/specs/active/body-status-line/spec\.md\t\.verdi/specs/archive/body-status-line/spec\.md`)
	if !renamed.MatchString(diffOut) {
		t.Fatalf("git diff --name-status -M did not report the archive rename:\n%s", diffOut)
	}
	assertNoVL010AfterClose(t, repo.Dir, repo.Head)
}
