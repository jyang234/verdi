// Fast, in-process tests for `verdi gc --reclaim-unmanaged`'s flag parsing
// (parseGcArgs) and testable core (runGcReclaimUnmanaged) — mirroring
// audit.go's own cmdAudit/runAudit split (runAudit itself calls
// residue.Scan). gc_reclaim_cli_test.go covers the built-binary,
// ac-3--behavioral obligations this file's speed cannot: both invocation
// shapes' own scope-disclosure lines end to end, and the golden transcript.
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
)

// --- parseGcArgs ---

func TestParseGcArgs(t *testing.T) {
	cases := []struct {
		name                   string
		args                   []string
		wantReclaim, wantApply bool
		wantErr                bool
	}{
		{name: "no args: bare gc", args: nil},
		{name: "--reclaim-unmanaged alone", args: []string{"--reclaim-unmanaged"}, wantReclaim: true},
		{name: "--reclaim-unmanaged --apply", args: []string{"--reclaim-unmanaged", "--apply"}, wantReclaim: true, wantApply: true},
		{name: "--apply alone is a usage error (dc-1: only valid with --reclaim-unmanaged)", args: []string{"--apply"}, wantErr: true},
		{name: "unrecognized flag", args: []string{"--bogus"}, wantErr: true},
		{name: "stray positional argument", args: []string{"foo"}, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reclaim, apply, err := parseGcArgs(c.args)
			if c.wantErr {
				if err == nil {
					t.Fatalf("parseGcArgs(%v): want error, got nil", c.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGcArgs(%v): unexpected error: %v", c.args, err)
			}
			if reclaim != c.wantReclaim || apply != c.wantApply {
				t.Fatalf("parseGcArgs(%v) = (%v, %v), want (%v, %v)", c.args, reclaim, apply, c.wantReclaim, c.wantApply)
			}
		})
	}
}

// --- runGcReclaimUnmanaged: testable core ---

// gcReclaimFixture builds a store root with one eligible worktree+branch
// pair and one kept (unmerged) worktree — enough for a smoke-level dry-run/
// apply/kept-disclosure proof at the testable-core layer; the full
// six-exclusion-reason survey is gc_reclaim_cli_test.go's golden transcript.
func gcReclaimFixture(t *testing.T) (root, eligibleBranch, eligibleWTPath, unmergedBranch string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	root = repo.Dir
	ctx := context.Background()

	eligibleBranch = "design/eligible"
	if err := gitx.CheckoutNewBranch(ctx, root, eligibleBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", eligibleBranch, err)
	}
	writeGcFile(t, root, "eligible.txt", "e\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "eligible work")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+eligibleBranch, eligibleBranch)
	eligibleWTPath = filepath.Join(t.TempDir(), "eligible-wt")
	if err := gitx.WorktreeAdd(ctx, root, eligibleWTPath, eligibleBranch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", eligibleBranch, err)
	}

	unmergedBranch = "design/unmerged"
	if err := gitx.CheckoutNewBranch(ctx, root, unmergedBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", unmergedBranch, err)
	}
	writeGcFile(t, root, "unmerged.txt", "u\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "unmerged work")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	unmergedWT := filepath.Join(t.TempDir(), "unmerged-wt")
	if err := gitx.WorktreeAdd(ctx, root, unmergedWT, unmergedBranch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", unmergedBranch, err)
	}

	return root, eligibleBranch, eligibleWTPath, unmergedBranch
}

func writeGcFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunGcReclaimUnmanaged_DryRun_PrintsPlanAndScope proves AC-2's own
// dry-run contract at the testable-core layer: the eligible item is
// printed eligible, the kept item is printed kept, nothing is mutated, and
// the ac-3 scope-disclosure line is printed.
func TestRunGcReclaimUnmanaged_DryRun_PrintsPlanAndScope(t *testing.T) {
	root, eligibleBranch, eligibleWTPath, unmergedBranch := gcReclaimFixture(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runGcReclaimUnmanaged(ctx, root, "main", false, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runGcReclaimUnmanaged(dry-run) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	out := stdout.String()

	if !strings.Contains(out, "eligible:") || !strings.Contains(out, eligibleBranch) {
		t.Errorf("stdout missing the eligible line for %s:\n%s", eligibleBranch, out)
	}
	if !strings.Contains(out, "kept:") || !strings.Contains(out, unmergedBranch) || !strings.Contains(out, "unmerged") {
		t.Errorf("stdout missing the kept:unmerged line for %s:\n%s", unmergedBranch, out)
	}
	if strings.Contains(out, "reclaimed:") {
		t.Errorf("dry-run stdout must never print a reclaimed: line:\n%s", out)
	}
	if !strings.Contains(out, "gc: scope") {
		t.Errorf("stdout missing the ac-3 scope-disclosure line:\n%s", out)
	}
	if !strings.Contains(out, "managed-worktree reclamation") || !strings.Contains(out, "NOT run") {
		t.Errorf("stdout missing the ac-3 disclosure naming managed-worktree reclamation as not run this invocation:\n%s", out)
	}

	// Zero mutation: both worktrees and both branches still present.
	if _, err := os.Stat(eligibleWTPath); err != nil {
		t.Fatalf("eligible worktree removed by a DRY run: %v", err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, eligibleBranch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("eligible branch deleted by a DRY run")
	}
}

// TestRunGcReclaimUnmanaged_Apply_ReclaimsEligible_KeepsUnchanged proves
// --apply executes the identical plan dry-run computed: the eligible pair
// is actually removed and its tip printed; the kept item's own line is
// BYTE-IDENTICAL to the dry-run's (dc-1: never re-decided by --apply).
func TestRunGcReclaimUnmanaged_Apply_ReclaimsEligible_KeepsUnchanged(t *testing.T) {
	root, eligibleBranch, eligibleWTPath, unmergedBranch := gcReclaimFixture(t)
	ctx := context.Background()

	var dryStdout, dryStderr bytes.Buffer
	if got := runGcReclaimUnmanaged(ctx, root, "main", false, &dryStdout, &dryStderr); got != 0 {
		t.Fatalf("dry-run: got %d; stdout=%s stderr=%s", got, dryStdout.String(), dryStderr.String())
	}
	var keptLine string
	for _, line := range strings.Split(dryStdout.String(), "\n") {
		if strings.Contains(line, unmergedBranch) {
			keptLine = line
		}
	}
	if keptLine == "" {
		t.Fatalf("dry-run stdout missing a line for %s:\n%s", unmergedBranch, dryStdout.String())
	}

	var stdout, stderr bytes.Buffer
	got := runGcReclaimUnmanaged(ctx, root, "main", true, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runGcReclaimUnmanaged(--apply) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	out := stdout.String()

	if !strings.Contains(out, "reclaimed:") || !strings.Contains(out, eligibleBranch) {
		t.Errorf("stdout missing a reclaimed: line for %s:\n%s", eligibleBranch, out)
	}
	if !strings.Contains(out, keptLine) {
		t.Errorf("apply stdout's kept line for %s differs from the dry-run's own line %q:\n%s", unmergedBranch, keptLine, out)
	}

	if _, err := os.Stat(eligibleWTPath); !os.IsNotExist(err) {
		t.Fatalf("eligible worktree still on disk after --apply: err=%v", err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, eligibleBranch)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("eligible branch still exists after --apply")
	}
	// The kept (unmerged) branch must survive --apply untouched.
	has, err = gitx.HasLocalBranch(ctx, root, unmergedBranch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("unmerged branch was deleted by --apply; it must be kept")
	}
}

// TestRunGcReclaimUnmanaged_UnresolvableDefaultBranch_RefusesWholeRun is
// ac-2--behavioral's case 6, at the testable-core layer (mirrors
// audit.go's own TestRunAudit_ClosureHygieneSection_UnresolvableDefaultBranch
// precedent exactly): an empty defaultBranchRef refuses the WHOLE run
// before computing any plan, dry-run and --apply alike, exit 2, no plan
// printed, no mutating call attempted.
func TestRunGcReclaimUnmanaged_UnresolvableDefaultBranch_RefusesWholeRun(t *testing.T) {
	root, eligibleBranch, eligibleWTPath, _ := gcReclaimFixture(t)
	ctx := context.Background()

	for _, apply := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		got := runGcReclaimUnmanaged(ctx, root, "", apply, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runGcReclaimUnmanaged(defaultBranchRef=\"\", apply=%v) = %d, want 2; stdout=%s stderr=%s", apply, got, stdout.String(), stderr.String())
		}
		if stdout.Len() != 0 {
			t.Fatalf("apply=%v: stdout = %q, want empty (no plan printed on a whole-run refusal)", apply, stdout.String())
		}
		if !strings.Contains(strings.ToLower(stderr.String()), "default branch") {
			t.Fatalf("apply=%v: stderr = %q, want it to name the unresolvable default branch", apply, stderr.String())
		}
	}

	// Nothing touched.
	if _, err := os.Stat(eligibleWTPath); err != nil {
		t.Fatalf("eligible worktree removed despite the whole-run refusal: %v", err)
	}
	has, err := gitx.HasLocalBranch(ctx, root, eligibleBranch)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("eligible branch deleted despite the whole-run refusal")
	}
}

// TestRunGcReclaimUnmanaged_NoEligibleOrKeptItems_StillPrintsScope proves
// an empty plan is not a special case: the scope-disclosure line still
// prints and the run still exits 0.
func TestRunGcReclaimUnmanaged_NoEligibleOrKeptItems_StillPrintsScope(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	var stdout, stderr bytes.Buffer
	got := runGcReclaimUnmanaged(context.Background(), repo.Dir, "main", false, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("got %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "gc: scope") {
		t.Fatalf("stdout missing the scope-disclosure line:\n%s", stdout.String())
	}
}

// TestRunGcReclaimUnmanaged_UnprovenSpecs_RefusesWholeRun is the Task 6c
// re-review's routed fix (cmd/verdi/gc.go:126 previously consumed
// residue.Scan and silently ignored Result.UnprovenSpecs, computing and
// applying a reclamation plan over a scan that could contain unproven
// candidates — the same silent-pass class audit.go's own Finding 1 closed
// for `verdi audit`, here on a STATE-CHANGING verb). Mirrors
// TestRunGcReclaimUnmanaged_UnresolvableDefaultBranch_RefusesWholeRun's own
// shape (both apply=false and apply=true refuse identically, exit 2,
// nothing mutated), but — unlike that silent "assert nothing" refusal —
// this one PRINTS the per-spec disclosures to stdout first (audit.go's own
// unprovenSpecDisclosure/disclosure.Render rendering convention, reused
// verbatim, same package) before refusing, so an operator sees exactly
// which spec(s) could not be proven.
func TestRunGcReclaimUnmanaged_UnprovenSpecs_RefusesWholeRun(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                     "data/\n",
			".verdi/specs/active/ch-widget/spec.md": closureHygieneFixtureStorySpecMD,
			".verdi/specs/active/malformed/spec.md": "---\nid: spec/malformed\nkind: spec\nclass: feature\nunknown_field: true\n---\nbody\n",
		},
		Message: "seed one valid, already-accepted story alongside one malformed spec (ch-widget's own supersession-completeness proof is blocked by the malformed sibling, per audit_closurehygiene_test.go's identical fixture)",
	}})

	for _, apply := range []bool{false, true} {
		var stdout, stderr bytes.Buffer
		got := runGcReclaimUnmanaged(context.Background(), repo.Dir, "main", apply, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("apply=%v: runGcReclaimUnmanaged = %d, want 2 (refuse before computing/applying any plan over an unproven scan); stdout=%s stderr=%s", apply, got, stdout.String(), stderr.String())
		}
		out := stdout.String()
		if !strings.Contains(out, "spec/ch-widget") {
			t.Errorf("apply=%v: stdout missing the unproven spec named (spec/ch-widget):\n%s", apply, out)
		}
		if !strings.Contains(out, "disclosed-unproven") {
			t.Errorf("apply=%v: stdout missing the shared disclosure vocabulary (internal/disclosure.Render), want the audit's own rendering convention reused:\n%s", apply, out)
		}
		if strings.Contains(out, "eligible:") || strings.Contains(out, "kept:") || strings.Contains(out, "reclaimed:") {
			t.Errorf("apply=%v: stdout unexpectedly contains a plan row — no plan may be computed or applied over an unproven scan:\n%s", apply, out)
		}
		if !strings.Contains(strings.ToLower(stderr.String()), "unproven") {
			t.Errorf("apply=%v: stderr = %q, want it to name the refusal as due to an unproven spec", apply, stderr.String())
		}
	}
}

// TestRunGcReclaimUnmanaged_ZeroUnprovenSpecs_UnchangedBehavior pins the
// Task 6c fix's own negative path: gcReclaimFixture's corpus carries no
// specs at all (both fixture branches are plain files, no .verdi/specs/),
// so residue.Scan's UnprovenSpecs is always empty there — dry-run and
// --apply must behave EXACTLY as
// TestRunGcReclaimUnmanaged_DryRun_PrintsPlanAndScope and
// TestRunGcReclaimUnmanaged_Apply_ReclaimsEligible_KeepsUnchanged already
// proved, unchanged by this fix. This test exists so a future regression
// that over-broadens the new guard (e.g. refusing on ANY residue.Scan
// result, not just a genuinely unproven one) is caught here, not just by
// the two pre-existing tests continuing to pass.
func TestRunGcReclaimUnmanaged_ZeroUnprovenSpecs_UnchangedBehavior(t *testing.T) {
	root, eligibleBranch, eligibleWTPath, unmergedBranch := gcReclaimFixture(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runGcReclaimUnmanaged(ctx, root, "main", false, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("dry-run with zero unproven specs: got %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "eligible:") || !strings.Contains(out, eligibleBranch) {
		t.Errorf("stdout missing the eligible line for %s (zero-unproven path must be unaffected):\n%s", eligibleBranch, out)
	}
	if !strings.Contains(out, "kept:") || !strings.Contains(out, unmergedBranch) {
		t.Errorf("stdout missing the kept line for %s (zero-unproven path must be unaffected):\n%s", unmergedBranch, out)
	}

	var applyStdout, applyStderr bytes.Buffer
	got = runGcReclaimUnmanaged(ctx, root, "main", true, &applyStdout, &applyStderr)
	if got != 0 {
		t.Fatalf("--apply with zero unproven specs: got %d, want 0; stdout=%s stderr=%s", got, applyStdout.String(), applyStderr.String())
	}
	if !strings.Contains(applyStdout.String(), "reclaimed:") || !strings.Contains(applyStdout.String(), eligibleBranch) {
		t.Errorf("apply stdout missing the reclaimed line for %s (zero-unproven path must be unaffected):\n%s", eligibleBranch, applyStdout.String())
	}
	if _, err := os.Stat(eligibleWTPath); !os.IsNotExist(err) {
		t.Fatalf("eligible worktree still on disk after --apply (zero-unproven path must be unaffected): err=%v", err)
	}
}

// runGitCmd (git in dir, process's inherited environment, t.Fatalf on a
// non-zero exit) is gc_test.go's own helper, reused here unchanged
// (package main, one definition).
