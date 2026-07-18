package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const alignDesignSpecMD = `---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
decisions:
  - { id: dc-1, text: "some decision", anchor: "#dc-1", links: [ { type: supersedes, ref: adr/current-policy } ] }
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [static] }
---
# body
`

const alignDesignADRMD = `---
id: adr/current-policy
kind: adr
title: "Current policy"
status: accepted
owners: [platform-team]
decided: 2026-01-01
frozen: { at: 2026-01-01, commit: 3e91ab2 }
---
body
`

// buildAlignDesignRepo builds a fixturegit repo with a design-branch spec
// carrying an unresolved declared supersedes edge, then checks out
// design/stale-decline (`verdi design start`'s branch convention).
func buildAlignDesignRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         "schema: verdi.layout/v1\n",
				".verdi/specs/active/stale-decline/spec.md": alignDesignSpecMD,
				".verdi/adr/current-policy.md":              alignDesignADRMD,
			},
			Message: "scaffold design-branch spec with an unresolved supersedes edge",
		},
	})
	checkoutBranch(t, repo.Dir, "design/stale-decline")
	return repo
}

// readDecisionReport reads the design-branch spec's decision-conflict-report.md
// — the design-branch analogue of align_test.go's readReport.
func readDecisionReport(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", "stale-decline", "decision-conflict-report.md"))
	if err != nil {
		t.Fatalf("reading decision-conflict-report.md: %v", err)
	}
	return data
}

// decodeDecisionReportFile reads, splits, and strict-decodes the
// decision-conflict report at path — align_test.go's decodeReportFile, for
// the decision-conflict schema.
func decodeDecisionReportFile(t *testing.T, path string) *artifact.DecisionConflictFrontmatter {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter(%s): %v", path, err)
	}
	decoded, err := artifact.DecodeDecisionConflict(fm)
	if err != nil {
		t.Fatalf("DecodeDecisionConflict(%s): %v", path, err)
	}
	return decoded
}

// TestAlign_DesignBranch_RegeneratePreservesGenuineReportOnJudgeFailure is
// D6-24's regression proof against the LITERALLY witnessed scenario: round
// 6's board-editor design sweep succeeded on run 1 (2 real findings,
// judge_integrity recorded), then a re-run made to fold in dispositions
// timed out and overwrote the genuine exchange with a synthetic
// judged-decision-coverage-absent finding, destroying both real findings
// and their dispositions. A design-branch `verdi align` regeneration must
// never do that: keep the prior report byte-for-byte and exit 2 when a
// genuine prior exchange exists on disk and this run's judge fails.
func TestAlign_DesignBranch_RegeneratePreservesGenuineReportOnJudgeFailure(t *testing.T) {
	repo := buildAlignDesignRepo(t)
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "decision-conflict-report.md")

	// A living design sweep: the judge succeeds genuinely (judge_integrity
	// recorded).
	living := alignDeps{JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, living, &out, &errb); got != 0 {
		t.Fatalf("runAlign (living design sweep) = %d, want 0; stderr=%s", got, errb.String())
	}
	fm := decodeDecisionReportFile(t, reportPath)
	if fm.JudgeIntegrity == nil {
		t.Fatal("test setup: living decision-conflict report is not genuine (no judge_integrity)")
	}

	// The human dispositions every finding — the witness's "2 real findings +
	// dispositions" shape (here: the declared supersedes edge, and the
	// judge's own finding).
	for i := range fm.Findings {
		if fm.Findings[i].Kind == artifact.FindingJudged {
			fm.Findings[i].Disposition = artifact.ConflictNoConflict
			fm.Findings[i].Note = "owner-ratified: no real conflict"
		} else {
			fm.Findings[i].Disposition = artifact.ConflictExempt
			fm.Findings[i].Note = "owner-ratified: narrow implementation clarification"
		}
	}
	_, body, err := artifact.SplitFrontmatter(readDecisionReport(t, repo.Dir))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if err := os.WriteFile(reportPath, align.RenderDecisionMarkdown(fm, string(body)), 0o644); err != nil {
		t.Fatalf("writing dispositioned living decision-conflict report: %v", err)
	}
	genuineBefore, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading dispositioned living report: %v", err)
	}

	// Re-run align (NOT --freeze) with a judge that now fails outright — the
	// witness's "timed out at the 2m ceiling" stand-in.
	failingDeps := alignDeps{JudgeCmd: alignFakeJudgeFailing(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out2, errb2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, failingDeps, &out2, &errb2)

	if got != 2 {
		t.Fatalf("runAlign (design regenerate, judge failing, genuine prior) = %d, want 2 (operational failure); stdout=%s stderr=%s", got, out2.String(), errb2.String())
	}
	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading report after failed regenerate: %v", err)
	}
	if !bytes.Equal(genuineBefore, after) {
		t.Fatalf("genuine living decision-conflict report was NOT preserved byte-for-byte across a failed-judge regeneration:\n--- before ---\n%s\n--- after ---\n%s", genuineBefore, after)
	}
	if !strings.Contains(errb2.String(), "D6-24") {
		t.Fatalf("stderr = %q, want a loud disclosure naming why the report was preserved (D6-24)", errb2.String())
	}
}

// TestAlign_DesignBranchMode_WritesDecisionConflictReport proves `verdi
// align` on a design branch writes decision-conflict-report.md (not
// deviation-report.md), and that an unresolved declared edge is reported
// as not-yet-proven (never a bare pass).
func TestAlign_DesignBranchMode_WritesDecisionConflictReport(t *testing.T) {
	repo := buildAlignDesignRepo(t)

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, alignDeps{ModelDigest: testResolveModelDigest(t, repo.Dir)}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlign = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "decision-conflict-report.md") {
		t.Fatalf("stdout = %q, want it to name decision-conflict-report.md", stdout.String())
	}
	if !strings.Contains(stdout.String(), "not yet proven") {
		t.Fatalf("stdout = %q, want the computed status to disclose it is not yet proven (unresolved supersedes edge)", stdout.String())
	}
	if !strings.Contains(stdout.String(), "disclosed-unproven-complete") {
		t.Fatalf("stdout = %q, want the judged status to be disclosed-unproven-complete (no judge configured)", stdout.String())
	}
}
