package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// closureHygieneFixtureStorySpecMD is a minimal, valid, active-zone story
// spec.md at status: accepted-pending-build — this file's own fixture (a
// test literal, not shared production logic), distinct from
// closeFixtureStorySpecMD (close_test.go's own fixture, entangled with
// that file's loan-mgmt feature and self-hosted evidence bindings, which
// this integration test does not need).
const closureHygieneFixtureStorySpecMD = `---
id: spec/ch-widget
kind: spec
class: story
title: "Closure hygiene widget"
status: accepted-pending-build
owners: [platform-team]
story: jira:CH-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/ch-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [static, behavioral] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Closure hygiene widget
`

// TestRunAudit_ClosureHygieneSection_AppearsAndCoexists is the wiring
// proof dc-1 requires of cmd/verdi/audit.go itself (distinct from
// internal/residue's own unit-level obligation proofs): a fixture that
// crosses the EXISTING exemption threshold AND carries an AC-1 pattern
// (a) stranded ritual in the SAME run asserts ALL THREE sections' own
// content appears together, that the existing two sections' content is
// exactly what TestAudit_ExemptionThresholdEndToEnd already proves alone
// (co-2: byte-for-byte unchanged by this addition), and that the run
// exits 1 for the closure-hygiene finding.
func TestRunAudit_ClosureHygieneSection_AppearsAndCoexists(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                     "schema: verdi.layout/v1\naudit:\n  exempts_conflict_threshold: 3\n  deviations_stale_threshold: 3\n",
				".verdi/.gitignore":                     "data/\n",
				".verdi/adr/retry-policy.md":            adrMD("retry-policy", "accepted"),
				".verdi/specs/active/spec-a/spec.md":    componentSpecWithExempts("spec-a", "dc-1", "adr/retry-policy", "reason A"),
				".verdi/specs/active/spec-b/spec.md":    componentSpecWithExempts("spec-b", "dc-1", "adr/retry-policy", "reason B"),
				".verdi/specs/active/spec-c/spec.md":    componentSpecWithExempts("spec-c", "dc-1", "adr/retry-policy", "reason C"),
				".verdi/specs/active/ch-widget/spec.md": closureHygieneFixtureStorySpecMD,
			},
			Message: "seed an exemption-threshold-crossing corpus alongside an in-flight story",
		},
	})
	root := repo.Dir
	ctx := context.Background()

	// Cut close/ch-widget and strand it: archive the spec on the branch's
	// own tip, but never merge — AC-1 pattern (a)'s own shape.
	if err := gitx.CheckoutNewBranch(ctx, root, "close/ch-widget"); err != nil {
		t.Fatalf("CheckoutNewBranch(close/ch-widget): %v", err)
	}
	if err := store.ArchiveMove(root, "ch-widget"); err != nil {
		t.Fatalf("store.ArchiveMove(ch-widget): %v", err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "close: archive spec/ch-widget (jira:CH-1)")
	wantTip := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := runAudit(ctx, root, 3, 3, "main", nil, &stdout, &stderr)
	out := stdout.String()

	if got != 1 {
		t.Fatalf("runAudit = %d, want 1 (both the exemption threshold AND the closure-hygiene stranded ritual flag); stdout=%s stderr=%s", got, out, stderr.String())
	}

	// The two EXISTING sections' own content, unchanged (co-2) — the same
	// assertions TestAudit_ExemptionThresholdEndToEnd makes alone.
	if !strings.Contains(out, "== Exemption audit ==") {
		t.Errorf("stdout missing == Exemption audit == header:\n%s", out)
	}
	if !strings.Contains(out, "FILED:") {
		t.Errorf("stdout missing a FILED: line:\n%s", out)
	}
	if !strings.Contains(out, "adr/retry-policy: 3 active exemption(s)") {
		t.Errorf("stdout missing the exemption count line:\n%s", out)
	}
	if !strings.Contains(out, "== Spec-stale audit ==") {
		t.Errorf("stdout missing == Spec-stale audit == header:\n%s", out)
	}

	// The NEW third section: header, and a witness line naming the spec,
	// the close/ch-widget branch, and its tip sha.
	if !strings.Contains(out, "== Closure hygiene audit ==") {
		t.Fatalf("stdout missing == Closure hygiene audit == header:\n%s", out)
	}
	if !strings.Contains(out, "spec/ch-widget") {
		t.Errorf("stdout missing the spec name in the closure-hygiene witness:\n%s", out)
	}
	if !strings.Contains(out, "close/ch-widget") {
		t.Errorf("stdout missing the close/ch-widget branch name in the witness:\n%s", out)
	}
	if !strings.Contains(out, wantTip) {
		t.Errorf("stdout missing the stranded branch's own tip sha %s:\n%s", wantTip, out)
	}

	if !strings.Contains(out, "audit: FLAGGED") {
		t.Errorf("stdout missing the audit: FLAGGED trailer:\n%s", out)
	}

	// The closure-hygiene section is the LAST thing printed before the
	// trailer (co-2: "appended", never interleaved with the first two).
	hygieneIdx := strings.Index(out, "== Closure hygiene audit ==")
	trailerIdx := strings.Index(out, "audit: FLAGGED")
	exemptIdx := strings.Index(out, "== Exemption audit ==")
	staleIdx := strings.Index(out, "== Spec-stale audit ==")
	if exemptIdx >= staleIdx || staleIdx >= hygieneIdx || hygieneIdx >= trailerIdx {
		t.Errorf("section order wrong: exemption=%d stale=%d hygiene=%d trailer=%d, want ascending", exemptIdx, staleIdx, hygieneIdx, trailerIdx)
	}
}

// TestRunAudit_ClosureHygieneSection_Clean proves a fixture with nothing
// for any of the three sections to find prints every section's own
// "clean" disclosure and exits 0.
func TestRunAudit_ClosureHygieneSection_Clean(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                     "data/\n",
			".verdi/specs/active/ch-widget/spec.md": closureHygieneFixtureStorySpecMD,
		},
		Message: "an ordinary in-flight story, nothing contradicts it",
	}})

	var stdout, stderr bytes.Buffer
	got := runAudit(context.Background(), repo.Dir, 3, 3, "main", nil, &stdout, &stderr)
	out := stdout.String()
	if got != 0 {
		t.Fatalf("runAudit = %d, want 0 (clean); stdout=%s stderr=%s", got, out, stderr.String())
	}
	if !strings.Contains(out, "== Closure hygiene audit ==") {
		t.Fatalf("stdout missing == Closure hygiene audit == header:\n%s", out)
	}
	if !strings.Contains(out, "(no status/git-reality contradictions found)") {
		t.Errorf("stdout missing the clean disclosure for AC-1/AC-2:\n%s", out)
	}
	if !strings.Contains(out, "(no merged-but-undeleted branches)") {
		t.Errorf("stdout missing the clean disclosure for AC-3(a):\n%s", out)
	}
	if !strings.Contains(out, "(no other registered worktrees)") {
		t.Errorf("stdout missing the clean disclosure for AC-3(b):\n%s", out)
	}
	if !strings.Contains(out, "audit: CLEAN") {
		t.Errorf("stdout missing the audit: CLEAN trailer:\n%s", out)
	}
}

// TestRunAudit_ClosureHygieneSection_UnresolvableDefaultBranch proves the
// three-valued "assert nothing" posture end to end: an empty
// defaultBranchRef prints the disclosure line, never a guessed clean
// report, and never flags.
func TestRunAudit_ClosureHygieneSection_UnresolvableDefaultBranch(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/.gitignore": "data/\n"},
		Message: "root",
	}})

	var stdout, stderr bytes.Buffer
	got := runAudit(context.Background(), repo.Dir, 3, 3, "", nil, &stdout, &stderr)
	out := stdout.String()
	if got != 0 {
		t.Fatalf("runAudit (unresolvable default branch) = %d, want 0; stdout=%s stderr=%s", got, out, stderr.String())
	}
	if !strings.Contains(out, "== Closure hygiene audit ==") {
		t.Fatalf("stdout missing == Closure hygiene audit == header:\n%s", out)
	}
	if !strings.Contains(out, "default branch could not be resolved") {
		t.Errorf("stdout missing the unresolved-default-branch disclosure:\n%s", out)
	}
}

// TestRunAudit_ClosureHygieneSection_StaleWorktreeDisclosedNotAborted is
// Defect 1's RED-direction witness: a worktree registered against the repo
// and then deleted from disk WITHOUT `git worktree remove` (git still lists
// it, marked prunable) must NOT abort the whole audit. AC-3(b)'s posture is
// "disclosed rather than guessed when a worktree's state cannot be
// resolved" — so the stale worktree is named with its unresolvable clean
// state disclosed, the run's exit code is unaffected (the survey never
// flags), and the two pre-existing sections still print. Before the fix,
// scanWorktrees propagated the `git status` failure as an operational error
// and `verdi audit` exited 2, killing all three sections' reports.
func TestRunAudit_ClosureHygieneSection_StaleWorktreeDisclosedNotAborted(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                     "data/\n",
			".verdi/specs/active/ch-widget/spec.md": closureHygieneFixtureStorySpecMD,
		},
		Message: "an ordinary in-flight story; nothing contradicts it",
	}})
	root := repo.Dir
	ctx := context.Background()

	// Register a real worktree on its own branch, then delete its directory
	// from disk without `git worktree remove` — the exact stale-registration
	// shape the spec's own 31-worktree problem statement anticipates.
	if err := gitx.CheckoutNewBranch(ctx, root, "stale-wt-branch"); err != nil {
		t.Fatalf("CheckoutNewBranch(stale-wt-branch): %v", err)
	}
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	staleWT := filepath.Join(t.TempDir(), "stale-wt")
	if err := gitx.WorktreeAdd(ctx, root, staleWT, "stale-wt-branch"); err != nil {
		t.Fatalf("WorktreeAdd(stale-wt-branch): %v", err)
	}
	if err := os.RemoveAll(staleWT); err != nil {
		t.Fatalf("RemoveAll(%s): %v", staleWT, err)
	}

	var stdout, stderr bytes.Buffer
	got := runAudit(ctx, root, 3, 3, "main", nil, &stdout, &stderr)
	out := stdout.String()

	if got == 2 {
		t.Fatalf("runAudit = 2 (aborted on a stale worktree); want the run to complete. stdout=%s stderr=%s", out, stderr.String())
	}
	if got != 0 {
		t.Fatalf("runAudit = %d, want 0 (AC-3's worktree survey never flags, so a stale worktree cannot change the exit code); stdout=%s stderr=%s", got, out, stderr.String())
	}

	// All three sections still print — the stale worktree did not go dark.
	for _, header := range []string{"== Exemption audit ==", "== Spec-stale audit ==", "== Closure hygiene audit =="} {
		if !strings.Contains(out, header) {
			t.Errorf("stdout missing %q header (a stale worktree must not suppress any section):\n%s", header, out)
		}
	}

	// The stale worktree is named, with its unresolvable state disclosed.
	if !strings.Contains(out, staleWT) {
		t.Errorf("stdout missing the stale worktree path %q:\n%s", staleWT, out)
	}
	if !strings.Contains(out, "unresolvable") {
		t.Errorf("stdout missing the 'unresolvable' disclosure for the stale worktree:\n%s", out)
	}
	if !strings.Contains(out, "audit: CLEAN") {
		t.Errorf("stdout missing the audit: CLEAN trailer (the survey never flags):\n%s", out)
	}
}

// TestRunAudit_ClosureHygieneSection_PatternB_NeverFlagsAlone proves
// dc-3's own exit-code split end to end through runAudit: a stub-complete
// unclosed feature, with nothing else in the corpus, is reported but
// leaves the run CLEAN.
func TestRunAudit_ClosureHygieneSection_PatternB_NeverFlagsAlone(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                        "data/\n",
			".verdi/specs/archive/ch-stub-one/spec.md": strings.Replace(closureHygieneFixtureStorySpecMD, "status: accepted-pending-build", "status: closed", 1),
			".verdi/specs/active/ch-feature/spec.md": `---
id: spec/ch-feature
kind: spec
class: feature
title: "Closure hygiene feature"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture outcome holds", evidence: [static, attestation] }
stubs:
  - { slug: ch-stub-one, acceptance_criteria: [ac-1] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Closure hygiene feature
`,
		},
		Message: "a stub-complete, unclosed feature, alone",
	}})

	var stdout, stderr bytes.Buffer
	got := runAudit(context.Background(), repo.Dir, 3, 3, "main", nil, &stdout, &stderr)
	out := stdout.String()
	if got != 0 {
		t.Fatalf("runAudit = %d, want 0 (dc-3: pattern (b) alone never flags); stdout=%s stderr=%s", got, out, stderr.String())
	}
	if !strings.Contains(out, "STUB-COMPLETE: spec/ch-feature") {
		t.Errorf("stdout missing the STUB-COMPLETE witness line:\n%s", out)
	}
	if !strings.Contains(out, "ch-stub-one") {
		t.Errorf("stdout missing the realized stub slug:\n%s", out)
	}
	if !strings.Contains(out, "audit: CLEAN") {
		t.Errorf("stdout missing the audit: CLEAN trailer (pattern (b) must not flag):\n%s", out)
	}
}
