package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
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

// TestAlign_DesignBranchMode_WritesDecisionConflictReport proves `verdi
// align` on a design branch writes decision-conflict-report.md (not
// deviation-report.md), and that an unresolved declared edge is reported
// as not-yet-proven (never a bare pass).
func TestAlign_DesignBranchMode_WritesDecisionConflictReport(t *testing.T) {
	repo := buildAlignDesignRepo(t)

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, alignDeps{}, &stdout, &stderr)
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
