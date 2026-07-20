// obligation/gc-reclaim--ac-3--behavioral's built-binary tests: both
// invocation shapes (a plain `verdi gc` and `verdi gc --reclaim-unmanaged`)
// disclose the OTHER slice as not-run alongside their own pre-existing
// content, and a golden transcript proves every ac-1 exclusion reason
// renders as its own single line. Drives the BUILT verdi binary (not `go
// run`), matching this repository's own established convention for
// CLI-behavioral proof (PLAN.md phase 1) and gc_test.go's own
// buildVerdiBinary idiom.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// runGcBinary runs bin with args against dir, with a hermetic
// (co-1: no network) default-branch resolution pinned via CI_DEFAULT_BRANCH,
// returning stdout, stderr, and the process exit code (never failing the
// test on a non-zero exit itself — callers assert the exit code they want).
func runGcBinary(t *testing.T, bin, dir string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err == nil {
		return out.String(), errOut.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running %s %v (dir %s): %v", bin, args, dir, err)
	}
	return out.String(), errOut.String(), exitErr.ExitCode()
}

// TestGcReclaimUnmanaged_CLI_BothInvocationShapes_DiscloseTheOtherAsNotRun
// is ac-3--behavioral's first half: a plain `verdi gc` run's own output
// still contains its pre-existing managed-worktree behavior plus the new
// available-but-not-run-this-invocation disclosure naming
// --reclaim-unmanaged; a `verdi gc --reclaim-unmanaged` run's own output
// contains the mirrored managed-slice-not-run disclosure plus the
// pre-existing derived-cache/layout-cache disclosure. Neither run prints
// the other's own reclaim/kept lines.
func TestGcReclaimUnmanaged_CLI_BothInvocationShapes_DiscloseTheOtherAsNotRun(t *testing.T) {
	bin := buildVerdiBinary(t)
	ctx := context.Background()

	// Two INDEPENDENT fixtures/repos, one per invocation shape — not one
	// repo driven by two sequential gc invocations. Plain gc's own
	// managed-worktree reclaim deletes a worktree but (by internal/
	// wtmanager's own existing, unchanged design) never its branch, so a
	// SECOND, later --reclaim-unmanaged run against the SAME repo would
	// legitimately see that now-worktree-less branch as its own eligible
	// branch-only row — a real, correct interaction, but not what THIS
	// obligation is proving (each shape's own disclosure, in isolation).
	t.Run("plain gc: unchanged managed behavior + new disclosure", func(t *testing.T) {
		root, managedBranch := gcCLIFixture(t) // gc_test.go's own fixture
		managedPath, err := wtmanager.EnsureWorktree(ctx, root, managedBranch)
		if err != nil {
			t.Fatalf("EnsureWorktree seeding managed slice: %v", err)
		}

		out, stderr, exit := runGcBinary(t, bin, root, "gc")
		if exit != 0 {
			t.Fatalf("verdi gc: exit %d; stdout=%s stderr=%s", exit, out, stderr)
		}
		if _, statErr := os.Stat(managedPath); !os.IsNotExist(statErr) {
			t.Fatalf("managed worktree still on disk after plain `verdi gc`: err=%v", statErr)
		}
		if !strings.Contains(out, "reclaimed") {
			t.Fatalf("plain gc stdout missing its pre-existing reclaim line:\n%s", out)
		}
		if !strings.Contains(out, "reclaims managed worktrees only") {
			t.Fatalf("plain gc stdout missing its pre-existing managed-only scope line:\n%s", out)
		}
		if !strings.Contains(out, "--reclaim-unmanaged") || !strings.Contains(out, "NOT run this invocation") {
			t.Fatalf("plain gc stdout missing the new available-but-not-run disclosure naming --reclaim-unmanaged:\n%s", out)
		}
	})

	t.Run("--reclaim-unmanaged: mirrored disclosure", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
			Message: "store root",
		}})
		root := repo.Dir
		const unmanagedBranch = "design/unmanaged-eligible"
		if err := gitx.CheckoutNewBranch(ctx, root, unmanagedBranch); err != nil {
			t.Fatalf("CheckoutNewBranch(%s): %v", unmanagedBranch, err)
		}
		writeGcFile(t, root, "unmanaged.txt", "u\n")
		runGitCmd(t, root, "add", "-A")
		runGitCmd(t, root, "commit", "--quiet", "-m", "unmanaged work")
		if err := gitx.Checkout(ctx, root, "main"); err != nil {
			t.Fatalf("Checkout(main): %v", err)
		}
		runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+unmanagedBranch, unmanagedBranch)
		unmanagedWT := filepath.Join(t.TempDir(), "unmanaged-wt")
		if err := gitx.WorktreeAdd(ctx, root, unmanagedWT, unmanagedBranch); err != nil {
			t.Fatalf("WorktreeAdd(%s): %v", unmanagedBranch, err)
		}

		out, stderr, exit := runGcBinary(t, bin, root, "gc", "--reclaim-unmanaged")
		if exit != 0 {
			t.Fatalf("verdi gc --reclaim-unmanaged: exit %d; stdout=%s stderr=%s", exit, out, stderr)
		}
		if !strings.Contains(out, unmanagedBranch) {
			t.Fatalf("--reclaim-unmanaged stdout missing the unmanaged pair's own branch:\n%s", out)
		}
		if !strings.Contains(out, "managed-worktree reclamation") || !strings.Contains(out, "NOT run this invocation") {
			t.Fatalf("--reclaim-unmanaged stdout missing the mirrored managed-not-run disclosure:\n%s", out)
		}
		if !strings.Contains(out, "derived-cache") || !strings.Contains(strings.ToLower(out), "layout") {
			t.Fatalf("--reclaim-unmanaged stdout missing the pre-existing derived-cache/layout-cache disclosure:\n%s", out)
		}
		if strings.Contains(out, "reclaims managed worktrees only") {
			t.Fatalf("--reclaim-unmanaged stdout must never print the plain run's own managed-only scope line:\n%s", out)
		}
		if strings.Contains(out, "reclaimed:") {
			t.Fatalf("--reclaim-unmanaged (no --apply) must never print a reclaimed: line:\n%s", out)
		}
	})
}

// TestGcReclaimUnmanaged_CLI_Apply_ReclaimsAndPrintsTip proves --apply
// through the REAL dispatch/flag-parsing path (parseGcArgs, cmdGc), not
// only the testable core: an eligible worktree+branch pair is actually
// removed on disk and its own pre-delete tip commit is printed, mirroring
// gc_test.go's own TestGc_CLI_ReclaimsAndDisclosesScope shape for the
// managed slice.
func TestGcReclaimUnmanaged_CLI_Apply_ReclaimsAndPrintsTip(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	root := repo.Dir
	ctx := context.Background()

	const branch = "design/apply-me"
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	writeGcFile(t, root, "x.txt", "x\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "work")
	wantTip, err := gitx.RevParse(ctx, root, branch)
	if err != nil {
		t.Fatal(err)
	}
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
	wtPath := filepath.Join(t.TempDir(), "apply-me-wt")
	if err := gitx.WorktreeAdd(ctx, root, wtPath, branch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", branch, err)
	}

	out, stderr, exit := runGcBinary(t, bin, root, "gc", "--reclaim-unmanaged", "--apply")
	if exit != 0 {
		t.Fatalf("verdi gc --reclaim-unmanaged --apply: exit %d; stdout=%s stderr=%s", exit, out, stderr)
	}
	if !strings.Contains(out, "reclaimed:") || !strings.Contains(out, branch) || !strings.Contains(out, wantTip) {
		t.Fatalf("stdout = %q, want a reclaimed: line naming %s and its tip %s", out, branch, wantTip)
	}
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Fatalf("worktree %s still on disk after --apply: err=%v", wtPath, statErr)
	}
	has, err := gitx.HasLocalBranch(ctx, root, branch)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatalf("branch %s still exists after --apply", branch)
	}
}

// TestGcReclaimUnmanaged_CLI_UnresolvableDefaultBranch_ExitsTwo is
// ac-2--behavioral's case 6, driven through the REAL flag-parsing/dispatch
// path (cmdGc itself, not the testable core alone — gc_reclaim_test.go's
// own TestRunGcReclaimUnmanaged_UnresolvableDefaultBranch_RefusesWholeRun
// covers that layer): a fixturegit repo with no configured remote and no
// CI_DEFAULT_BRANCH exposed to the child process genuinely cannot resolve
// a default branch, so lint.ResolveDefaultBranch itself returns "".
func TestGcReclaimUnmanaged_CLI_UnresolvableDefaultBranch_ExitsTwo(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})

	cmd := exec.Command(bin, "gc", "--reclaim-unmanaged")
	cmd.Dir = repo.Dir
	// Deliberately no CI_DEFAULT_BRANCH override — and filtered OUT from
	// whatever the outer test-runner's own environment carries (e.g. a real
	// CI system running `make verify` itself), so this test's own
	// hermeticity does not depend on where it happens to run.
	cmd.Env = filteredEnv("CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("verdi gc --reclaim-unmanaged with no resolvable default branch: want an ExitError, got %v (stdout=%s stderr=%s)", err, stdout.String(), stderr.String())
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%s stderr=%s", exitErr.ExitCode(), stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty (no plan printed on a whole-run refusal)", stdout.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "default branch") {
		t.Fatalf("stderr = %q, want it to name the unresolvable default branch", stderr.String())
	}
}

// filteredEnv returns the current process's environment with every
// variable named in remove stripped out.
func filteredEnv(remove ...string) []string {
	skip := make(map[string]bool, len(remove))
	for _, k := range remove {
		skip[k] = true
	}
	var out []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if !skip[key] {
			out = append(out, kv)
		}
	}
	return out
}

// unresolvedDetailRe collapses the ONE volatile piece of an otherwise fully
// deterministic transcript: an unresolved-state row's own parenthetical
// detail text is git's own "prunable" reason (its exact wording is a git-
// version detail, the same D6-8-class parity concern this codebase already
// treats as never safe to string-match exactly — internal/gitx.WorktreeAdd's
// own doc comment names the identical caution). internal/residue's own
// test (TestScanWorktrees_StaleWorktreeDisclosedNotAborted) already asserts
// only a loose substring ("prunable") for the same reason; this golden
// normalizes it to a fixed placeholder rather than pin git's own text.
var unresolvedDetailRe = regexp.MustCompile(`unresolved-state \([^)]*\)`)

// TestGcReclaimUnmanaged_CLI_GoldenTranscript_AllSixExclusionReasons is
// ac-3--behavioral's golden-transcript obligation: a fixture exercising
// every one of ac-1's six exclusion reasons at once (unmerged, dirty,
// unresolved-state, detached, managed, invoking) plus one eligible
// worktree+branch pair and one eligible branch-only row, asserted against a
// committed golden — one line per item, dc-4's exact templates.
//
// The comparison is over the SET of rendered lines (sorted before
// comparing), not literal top-to-bottom sequence: residue.Scan's own
// worktree ordering is a lexicographic sort over ABSOLUTE paths
// (survey.go), and this fixture deliberately spans TWO independent
// t.TempDir() roots (the primary checkout and the managed worktree's own
// root vs. every other worktree's shared parent) whose relative sort order
// is Go-test-tempdir-naming-scheme-dependent, not a fact this story's own
// contract makes — dc-4 requires one line per item and exact per-line
// wording, never a specific inter-item order. Every line is still asserted
// present EXACTLY ONCE, with no extra and no missing line, against a fully
// literal, committed golden text.
func TestGcReclaimUnmanaged_CLI_GoldenTranscript_AllSixExclusionReasons(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, invokingDir, wtParent := gcReclaimGoldenFixture(t)

	out, stderr, exit := runGcBinary(t, bin, invokingDir, "gc", "--reclaim-unmanaged")
	if exit != 0 {
		t.Fatalf("verdi gc --reclaim-unmanaged: exit %d; stdout=%s stderr=%s", exit, out, stderr)
	}

	// Normalize the two volatile t.TempDir()-rooted path prefixes (<ROOT>
	// for the primary checkout, <WTROOT> for every OTHER worktree's shared
	// parent — see gcReclaimGoldenFixture) and the one git-version-
	// dependent detail text (unresolvedDetailRe), so the remaining
	// comparison is fully deterministic.
	normalized := out
	for _, rep := range []struct{ path, placeholder string }{
		{wtParent, "<WTROOT>"},
		{root, "<ROOT>"},
	} {
		resolved, err := filepath.EvalSymlinks(rep.path)
		if err != nil {
			resolved = rep.path
		}
		normalized = strings.ReplaceAll(normalized, resolved, rep.placeholder)
		normalized = strings.ReplaceAll(normalized, rep.path, rep.placeholder)
	}
	normalized = unresolvedDetailRe.ReplaceAllString(normalized, "unresolved-state (<DETAIL>)")

	wantLines := []string{
		"eligible: worktree <WTROOT>/eligible-wt + branch design/eligible",
		"eligible: branch merged-orphan",
		"kept: worktree <WTROOT>/unmerged-wt + branch design/unmerged — unmerged",
		"kept: worktree <WTROOT>/dirty-wt + branch design/dirty — dirty",
		"kept: worktree <WTROOT>/stale-wt + branch design/stale — unresolved-state (<DETAIL>)",
		"kept: worktree <WTROOT>/detached-wt — detached",
		"kept: worktree <WTROOT>/invoking-wt/.verdi/data/worktrees/managed + branch design/managed — managed",
		"kept: worktree <WTROOT>/invoking-wt + branch design/invoking — invoking",
		gcScopeDisclosureUnmanaged,
	}

	gotLines := strings.Split(strings.TrimRight(normalized, "\n"), "\n")

	sortedGot := append([]string(nil), gotLines...)
	sortedWant := append([]string(nil), wantLines...)
	sort.Strings(sortedGot)
	sort.Strings(sortedWant)

	if len(sortedGot) != len(sortedWant) {
		t.Fatalf("got %d lines, want %d.\ngot (normalized):\n%s\nwant:\n%s", len(sortedGot), len(sortedWant), strings.Join(sortedGot, "\n"), strings.Join(sortedWant, "\n"))
	}
	for i := range sortedGot {
		if sortedGot[i] != sortedWant[i] {
			t.Errorf("line mismatch at sorted position %d:\ngot:  %q\nwant: %q", i, sortedGot[i], sortedWant[i])
		}
	}
	if t.Failed() {
		t.Logf("full normalized transcript:\n%s", normalized)
	}
}

// gcReclaimGoldenFixture builds a store root exercising every ac-1
// exclusion reason at once, plus one eligible worktree+branch pair and one
// eligible branch-only row (obligation/gc-reclaim--ac-3--behavioral's own
// golden fixture) — mirroring internal/reclaim/ac1_test.go's own combined-
// survey fixture, rebuilt here at the CLI/built-binary layer. Returns root
// (the primary checkout), invokingDir (a second, non-primary worktree the
// binary is invoked FROM — itself otherwise eligible-shaped, kept only by
// the invoking exclusion), and wtParent (the shared parent directory every
// OTHER linked worktree in this fixture is cut under, for golden
// normalization).
func gcReclaimGoldenFixture(t *testing.T) (root, invokingDir, wtParent string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n", ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	root = repo.Dir
	ctx := context.Background()
	wtParent = t.TempDir()

	mergedPair := func(name string) (branch, path string) {
		branch = "design/" + name
		if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
			t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
		}
		writeGcFile(t, root, name+".txt", name+"\n")
		runGitCmd(t, root, "add", "-A")
		runGitCmd(t, root, "commit", "--quiet", "-m", "add "+name)
		if err := gitx.Checkout(ctx, root, "main"); err != nil {
			t.Fatalf("Checkout(main): %v", err)
		}
		runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
		path = filepath.Join(wtParent, name+"-wt")
		if err := gitx.WorktreeAdd(ctx, root, path, branch); err != nil {
			t.Fatalf("WorktreeAdd(%s): %v", branch, err)
		}
		return branch, path
	}

	// Eligible worktree+branch pair.
	_, _ = mergedPair("eligible")

	// Eligible branch-only row: merged, no worktree at all.
	const orphanBranch = "merged-orphan"
	if err := gitx.CheckoutNewBranch(ctx, root, orphanBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", orphanBranch, err)
	}
	writeGcFile(t, root, "orphan.txt", "orphan\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "orphan work")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+orphanBranch, orphanBranch)

	// Unmerged row.
	const unmergedBranch = "design/unmerged"
	if err := gitx.CheckoutNewBranch(ctx, root, unmergedBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", unmergedBranch, err)
	}
	writeGcFile(t, root, "unmerged.txt", "u\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "unmerged work")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	if err := gitx.WorktreeAdd(ctx, root, filepath.Join(wtParent, "unmerged-wt"), unmergedBranch); err != nil {
		t.Fatalf("WorktreeAdd(%s): %v", unmergedBranch, err)
	}

	// Dirty row.
	_, dirtyPath := mergedPair("dirty")
	writeGcFile(t, dirtyPath, "wip.txt", "wip\n")

	// Unresolved-state row: worktree directory deleted WITHOUT `git
	// worktree remove` (git marks it prunable).
	_, stalePath := mergedPair("stale")
	if err := os.RemoveAll(stalePath); err != nil {
		t.Fatalf("RemoveAll(%s): %v", stalePath, err)
	}

	// Detached-HEAD row.
	detachAt, err := gitx.RevParse(ctx, root, "main")
	if err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "worktree", "add", "--detach", "--quiet", filepath.Join(wtParent, "detached-wt"), detachAt)

	// Invoking row: otherwise eligible-shaped, kept only because the sweep
	// itself is invoked from here (cmd.Dir, in the caller). Cut BEFORE the
	// managed row below, which needs invokingDir to already exist.
	_, invokingDir = mergedPair("invoking")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}

	// Managed-worktree row, cut via the real production entry point —
	// relative to invokingDir, NOT root: "managed" (internal/wtmanager.
	// WorktreesRoot) is inherently PER-CHECKOUT (.verdi/data/ is gitignored,
	// never shared across worktrees), and residue.Scan itself is always
	// invoked against wherever the sweep is RUNNING FROM (cmd/verdi/gc.go's
	// own store.FindRoot(".")) — here, invokingDir, the very directory the
	// built binary's cmd.Dir is set to below. Cutting it against root
	// instead would make this row read as UNMANAGED from invokingDir's own
	// vantage point, which is not what this fixture means to prove.
	const managedBranch = "design/managed"
	if err := gitx.CheckoutNewBranch(ctx, root, managedBranch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", managedBranch, err)
	}
	writeGcFile(t, root, "managed.txt", "m\n")
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "managed work")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+managedBranch, managedBranch)
	if _, err := wtmanager.EnsureWorktree(ctx, invokingDir, managedBranch); err != nil {
		t.Fatalf("EnsureWorktree(%s): %v", managedBranch, err)
	}

	return root, invokingDir, wtParent
}
