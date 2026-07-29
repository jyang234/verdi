package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/decisionsweep"
	"github.com/jyang234/verdi/internal/disclosure"
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

// auditWaiverStorySpecMD renders a minimal story-class spec.md the waiver
// audit's own scan (internal/decisionsweep.ScanWaiverStale) will pick up:
// class story, a `story:` link so it has a slug to look waivers up under.
func auditWaiverStorySpecMD(name string) string {
	return "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: story\nstatus: draft\nowners: [platform-team]\n" +
		"story: jira:LOAN-1\nproblem: { text: \"p\", anchor: \"#p\" }\noutcome: { text: \"o\", anchor: \"#o\" }\n---\nbody\n"
}

// auditWaiverMD renders a minimal waiver record — this file's own copy of
// the shape internal/decisionsweep's own tests use (a per-package test
// fixture string, not shared production logic).
func auditWaiverMD(acID, status, expiry string) string {
	expiryLine := ""
	if expiry != "" {
		expiryLine = "expiry: " + expiry + "\n"
	}
	return "---\nid: waiver/jira-loan-1--" + acID + "\nkind: waiver\ntitle: \"waiver\"\nowners: [platform-team]\n" +
		"status: " + status + "\nreason: \"hotfix\"\n" + expiryLine +
		"links:\n  - { type: verifies, ref: spec/waiver-story }\n" +
		"frozen: { at: 2026-07-19, commit: 8c2d41f }\n---\nbody\n"
}

// TestLapsedWaiverDisclosure pins the disclosure VALUE the lapsed-waiver
// state constructs — its source, its scope (the waiver's own store-relative
// path: this disclosure IS about one artifact, unlike the checkout-wide
// forms), its derived id and its fixed severity — mirroring
// internal/disclosure's own constructor tests and cmd/verdi's sync tool-pin
// precedent.
func TestLapsedWaiverDisclosure(t *testing.T) {
	row := decisionsweep.WaiverAuditRow{
		ACID:   "ac-1",
		Path:   ".verdi/waivers/jira-loan-1/ac-1.md",
		Status: "active",
		Expiry: "2026-01-01",
		Lapsed: true,
	}
	d := lapsedWaiverDisclosure(row)
	if d.Source != lapsedWaiverSource {
		t.Errorf("Source = %q, want %q", d.Source, lapsedWaiverSource)
	}
	if d.Scope != row.Path {
		t.Errorf("Scope = %q, want the waiver's store-relative path %q", d.Scope, row.Path)
	}
	if d.ID != lapsedWaiverSource+"/"+row.Path {
		t.Errorf("ID = %q, want source/scope", d.ID)
	}
	if d.Severity != disclosure.SeverityDisclosedUnproven {
		t.Errorf("Severity = %q, want %q", d.Severity, disclosure.SeverityDisclosedUnproven)
	}
	if !strings.Contains(d.Text, row.Expiry) {
		t.Errorf("Text = %q, want it to name the recorded expiry %q", d.Text, row.Expiry)
	}
	// Negative path — the defect this replaced: a hand-authored "(LAPSED)"
	// marker that spoke its own severity word. Render supplies the one
	// vocabulary word; a text that restates either marker is the lookalike
	// coming back.
	if strings.Contains(d.Text, disclosure.SeverityDisclosedUnproven) {
		t.Errorf("Text = %q states the severity itself; Render already supplies it", d.Text)
	}
	if strings.Contains(d.Text, "LAPSED") {
		t.Errorf("Text = %q still carries the hand-authored (LAPSED) marker", d.Text)
	}
	if !disclosure.IsRendered(disclosure.Render(d)) {
		t.Errorf("Render(%+v) is not recognized as a disclosure line", d)
	}
	// The text must be a pure function of the row — no wall-clock read (the
	// invocation's single `now` already decided Lapsed at the boundary).
	if second := lapsedWaiverDisclosure(row); second != d {
		t.Errorf("lapsedWaiverDisclosure is not deterministic: %+v vs %+v", second, d)
	}
}

// TestAudit_LapsedWaiver_RendersThroughTheSeam exercises the producing call
// site: a story whose only waiver has lapsed by wall-clock emits exactly the
// seam's rendering of the lapsed-waiver disclosure, flush-left so
// disclosure.IsRendered recognizes it (spec/verb-surfaces ac-3's "discloses
// whether an active-status waiver's recorded expiry has already lapsed";
// judged-ac-1-vocabulary-coverage).
//
// The run stays CLEAN (exit 0): a lapsed waiver is excluded from the counted-
// active total, so it can never cross the threshold by itself — a disclosure,
// never a verdict. The WAIVER-STALE flag line is the verdict half and is
// deliberately NOT a disclosure; the negative path below pins that an
// unlapsed waiver emits no disclosure line at all.
func TestAudit_LapsedWaiver_RendersThroughTheSeam(t *testing.T) {
	build := func(t *testing.T, expiry string) *fixturegit.Repo {
		t.Helper()
		return fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                        "schema: verdi.layout/v1\n",
				".verdi/specs/active/waiver-story/spec.md": auditWaiverStorySpecMD("waiver-story"),
				".verdi/waivers/jira-loan-1/ac-1.md":       auditWaiverMD("ac-1", "active", expiry),
			},
			Message: "seed one waiver",
		}})
	}

	want := disclosure.Render(lapsedWaiverDisclosure(decisionsweep.WaiverAuditRow{
		ACID:   "ac-1",
		Path:   ".verdi/waivers/jira-loan-1/ac-1.md",
		Status: "active",
		Expiry: "2026-01-01",
		Lapsed: true,
	}))

	t.Run("lapsed expiry discloses through the seam", func(t *testing.T) {
		repo := build(t, "2026-01-01") // well before auditTestNow
		var stdout, stderr bytes.Buffer
		if got := runAudit(context.Background(), repo.Dir, 3, 3, 3, "main", auditTestNow, nil, &stdout, &stderr); got != 0 {
			t.Fatalf("runAudit = %d, want 0 (a lapsed waiver never counts active, so it never flags); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		found := false
		for _, line := range strings.Split(stdout.String(), "\n") {
			if line == want {
				found = true
			}
		}
		if !found {
			t.Errorf("stdout = %q, want the seam-rendered lapsed-waiver disclosure %q", stdout.String(), want)
		}
		if strings.Contains(stdout.String(), "(LAPSED)") {
			t.Errorf("stdout = %q still carries the hand-authored (LAPSED) marker", stdout.String())
		}
		// ac-3's listing obligation is untouched: the row still names the AC,
		// its status and its expiry.
		if !strings.Contains(stdout.String(), "ac-1: status active, expires 2026-01-01") {
			t.Errorf("stdout = %q, want the waiver row still listing AC/status/expiry (ac-3)", stdout.String())
		}
	})

	t.Run("unlapsed expiry discloses nothing", func(t *testing.T) {
		repo := build(t, "2026-12-31") // well after auditTestNow
		var stdout, stderr bytes.Buffer
		if got := runAudit(context.Background(), repo.Dir, 3, 3, 3, "main", auditTestNow, nil, &stdout, &stderr); got != 0 {
			t.Fatalf("runAudit = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		for _, line := range strings.Split(stdout.String(), "\n") {
			if disclosure.IsRendered(line) {
				t.Errorf("stdout emitted a disclosure line %q for an unlapsed waiver; the notice must never become background noise", line)
			}
		}
	})
}
