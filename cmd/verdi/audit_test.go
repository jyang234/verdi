package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// auditTestNow is the fixed reference "now" every runAudit test in this
// package passes — a deterministic instant (never time.Now()) for the
// same reason internal/decisionsweep's own audit_test.go pins one.
var auditTestNow = time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

// componentSpecWithExempts renders a minimal component-class spec.md
// carrying one decision object with an `exempts` link against adrRef —
// this file's own copy of the shape internal/decisionsweep's own tests use
// (a test fixture string, not shared production logic — CLAUDE.md's
// no-copy-paste rule governs logic, not per-package test fixtures).
func componentSpecWithExempts(name, decisionID, adrRef, reason string) string {
	return "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: component\nstatus: draft\nowners: [platform-team]\n" +
		"decisions:\n  - { id: " + decisionID + ", text: \"some decision\", anchor: \"#" + decisionID + "\", links: [ { type: exempts, ref: " + adrRef + ", note: \"" + reason + "\" } ] }\n" +
		"---\nbody\n"
}

func adrMD(name, status string) string {
	extra := ""
	if status == "accepted" {
		extra = "decided: 2026-01-01\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n"
	}
	return "---\nid: adr/" + name + "\nkind: adr\ntitle: \"" + name + "\"\nstatus: " + status + "\nowners: [platform-team]\n" + extra + "---\nbody\n"
}

// TestAudit_ExemptionThresholdEndToEnd is this phase's exit criterion,
// driven through cmd/verdi's own testable core against a fixturegit repo:
// seeding audit.exempts_conflict_threshold: 3 and filing three exempts
// edges against one ADR, `verdi audit` auto-files a .verdi/conflicts/
// record naming that ADR via challenges:, and reports the flag (exit 1).
//
// guide-claim: 8.1-align-deviation-disposition
func TestAudit_ExemptionThresholdEndToEnd(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                  "schema: verdi.layout/v1\naudit:\n  exempts_conflict_threshold: 3\n  deviations_stale_threshold: 3\n",
				".verdi/adr/retry-policy.md":         adrMD("retry-policy", "accepted"),
				".verdi/specs/active/spec-a/spec.md": componentSpecWithExempts("spec-a", "dc-1", "adr/retry-policy", "reason A"),
				".verdi/specs/active/spec-b/spec.md": componentSpecWithExempts("spec-b", "dc-1", "adr/retry-policy", "reason B"),
				".verdi/specs/active/spec-c/spec.md": componentSpecWithExempts("spec-c", "dc-1", "adr/retry-policy", "reason C"),
			},
			Message: "seed three exempts edges against one ADR",
		},
	})

	var stdout, stderr bytes.Buffer
	got := runAudit(context.Background(), repo.Dir, 3, 3, 3, "main", auditTestNow, nil, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAudit = %d, want 1 (flagged: threshold crossed); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FILED:") {
		t.Fatalf("stdout = %q, want a FILED: line", stdout.String())
	}
	if !strings.Contains(stdout.String(), "adr/retry-policy: 3 active exemption(s)") {
		t.Fatalf("stdout = %q, want the exemption count line", stdout.String())
	}

	// Re-running must be idempotent — clean the second time (nothing NEW to
	// file), even though the exemptions themselves are still listed.
	var stdout2, stderr2 bytes.Buffer
	got2 := runAudit(context.Background(), repo.Dir, 3, 3, 3, "main", auditTestNow, nil, &stdout2, &stderr2)
	if got2 != 0 {
		t.Fatalf("runAudit (second run) = %d, want 0 (idempotent, nothing new); stdout=%s", got2, stdout2.String())
	}
	if strings.Contains(stdout2.String(), "FILED:") {
		t.Fatalf("stdout (second run) = %q, want no FILED: line (idempotent)", stdout2.String())
	}
}

func TestAudit_BelowThreshold_Clean(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                  "schema: verdi.layout/v1\naudit:\n  exempts_conflict_threshold: 3\n  deviations_stale_threshold: 3\n",
				".verdi/adr/retry-policy.md":         adrMD("retry-policy", "accepted"),
				".verdi/specs/active/spec-a/spec.md": componentSpecWithExempts("spec-a", "dc-1", "adr/retry-policy", "reason A"),
			},
			Message: "seed one exempts edge, below threshold",
		},
	})

	var stdout, stderr bytes.Buffer
	got := runAudit(context.Background(), repo.Dir, 3, 3, 3, "main", auditTestNow, nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAudit = %d, want 0 (below threshold); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "FILED:") {
		t.Fatalf("stdout = %q, want no FILED: line", stdout.String())
	}
}

func TestAudit_Negative_NoStoreRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	got := cmdAudit(nil, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdAudit (no store root) = %d, want 2", got)
	}
}

func TestAudit_Negative_UnexpectedArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdAudit([]string{"bogus"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdAudit(bogus arg) = %d, want 2", got)
	}
}
