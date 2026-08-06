// Real, built-binary end-to-end tests for `verdi gc`'s execution slice
// (spec/execution-workspace §GC slice, invention SI-11): bare `verdi gc`
// now runs the execution slice alongside the pre-existing managed-worktree
// slice, and the scope-disclosure line grows from a closed pair to a
// closed triple. Mirrors gc_test.go's own buildVerdiBinary pattern; builds
// a real execution workspace via internal/execworkspace's own production
// entry points (Materializer + GitReconciler + Releaser) rather than
// hand-crafting on-disk state, so these tests exercise the same seam a
// real consumer (CI, CSE) would.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// gcExecutionCLIFixture builds a real store root (mirroring gcCLIFixture's
// own shape) with no managed-worktree activity at all, so these tests
// isolate the execution slice's own output.
func gcExecutionCLIFixture(t *testing.T) (root, headSHA string) {
	t.Helper()
	manifest := "schema: verdi.layout/v1\n"
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": manifest, ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	return repo.Dir, repo.Head
}

// materializeExecutionWorkspace cuts a real execution workspace at root
// (used as both storeRoot and repoRoot, exactly as cmdGc's own
// execworkspace.GC(ctx, root, root) call addresses them) through the real
// production Materializer/GitReconciler pair — never hand-built on-disk
// state — and returns its workspace id.
func materializeExecutionWorkspace(t *testing.T, root, runID, headSHA string) string {
	t.Helper()
	reconciler := execworkspace.NewGitReconciler(root)
	m, err := execworkspace.NewMaterializer(root, root, reconciler)
	if err != nil {
		t.Fatalf("NewMaterializer: %v", err)
	}
	id, err := execworkspace.NewExactIdentity(runID, headSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	res, err := m.Materialize(context.Background(), execworkspace.Request{Identity: id})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	return res.WorkspaceID
}

// TestGc_CLI_ExecutionSlice_ReclaimsReleasedWorkspace is (a): bare
// `verdi gc` reclaims a released, clean execution workspace, prints its
// reclaim line, prints the grown (closed-triple) scope disclosure, and
// actually removes the workspace from disk.
func TestGc_CLI_ExecutionSlice_ReclaimsReleasedWorkspace(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, headSHA := gcExecutionCLIFixture(t)

	workspaceID := materializeExecutionWorkspace(t, root, "run-reclaimable", headSHA)
	unitPath := execworkspace.UnitPath(root, workspaceID)

	releaser := execworkspace.NewReleaser(root)
	if err := releaser.Release(workspaceID); err != nil {
		t.Fatalf("Release(%s): %v", workspaceID, err)
	}

	cmd := exec.Command(bin, "gc")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi gc: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if _, statErr := os.Stat(unitPath); !os.IsNotExist(statErr) {
		t.Fatalf("released execution workspace %s still on disk after `verdi gc`: err=%v", unitPath, statErr)
	}

	out := stdout.String()
	if !strings.Contains(out, "reclaimed") || !strings.Contains(out, workspaceID) {
		t.Fatalf("verdi gc stdout = %q, want a printed execution-slice reclaim line naming %s", out, workspaceID)
	}
	if !strings.Contains(out, "execution workspaces") {
		t.Fatalf("verdi gc stdout = %q, want the grown scope-disclosure line naming execution workspaces", out)
	}
}

// TestGc_CLI_ExecutionSlice_KeepsNonReleasedWorkspace is (b): bare
// `verdi gc` keeps a materialized-but-never-released execution workspace,
// disclosing keep-not-eligible, and leaves it on disk.
func TestGc_CLI_ExecutionSlice_KeepsNonReleasedWorkspace(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, headSHA := gcExecutionCLIFixture(t)

	workspaceID := materializeExecutionWorkspace(t, root, "run-not-released", headSHA)
	unitPath := execworkspace.UnitPath(root, workspaceID)

	cmd := exec.Command(bin, "gc")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi gc: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if _, statErr := os.Stat(unitPath); statErr != nil {
		t.Fatalf("non-released execution workspace removed by `verdi gc`, want it kept: %v", statErr)
	}
	out := stdout.String()
	if !strings.Contains(out, "kept") || !strings.Contains(out, "not eligible") || !strings.Contains(out, workspaceID) {
		t.Fatalf("verdi gc stdout = %q, want a disclosed kept-not-eligible line naming %s", out, workspaceID)
	}
}

// TestGc_CLI_ReclaimUnmanaged_DisclosesExecutionSliceNotRun is (c):
// `verdi gc --reclaim-unmanaged` must disclose the execution slice as
// available but NOT run this invocation, and must never print the
// execution slice's own reclaim/keep lines even when a live, releasable
// execution workspace exists.
func TestGc_CLI_ReclaimUnmanaged_DisclosesExecutionSliceNotRun(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, headSHA := gcExecutionCLIFixture(t)

	workspaceID := materializeExecutionWorkspace(t, root, "run-untouched", headSHA)
	unitPath := execworkspace.UnitPath(root, workspaceID)
	releaser := execworkspace.NewReleaser(root)
	if err := releaser.Release(workspaceID); err != nil {
		t.Fatalf("Release(%s): %v", workspaceID, err)
	}

	cmd := exec.Command(bin, "gc", "--reclaim-unmanaged")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi gc --reclaim-unmanaged: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "execution-workspace reclamation") || !strings.Contains(out, "NOT run this invocation") {
		t.Fatalf("verdi gc --reclaim-unmanaged stdout = %q, want it to disclose execution-workspace reclamation as available but not run", out)
	}
	if strings.Contains(out, "execution: reclaimed") || strings.Contains(out, "execution: kept") {
		t.Fatalf("verdi gc --reclaim-unmanaged stdout = %q, must never print the execution slice's own per-unit lines", out)
	}
	// A released, clean execution workspace: --reclaim-unmanaged truly never
	// ran that slice, so it must still be on disk, untouched.
	if _, statErr := os.Stat(unitPath); statErr != nil {
		t.Fatalf("execution workspace %s touched by --reclaim-unmanaged (must be out of scope for this mode): %v", unitPath, statErr)
	}
}

// TestGc_CLI_ExecutionSlice_ThreeRunLifecycle_ReclaimThenOrphanedThenSteadyState
// is contract item 5: bare `verdi gc`, run three times over the SAME
// released, clean workspace built through the real production seam
// (materializeExecutionWorkspace + Release, exactly as (a)/(c) above).
// Run 1 reclaims it (rank 5) and prints the grown (closed-triple) scope
// disclosure; run 2 resolves the `git worktree add` registration that
// materialize produced and rank 5 never reconciles (rank 0,
// reclaim-orphaned); run 3 is steady state — nothing execution-specific
// in its output except the scope line, which prints unconditionally on
// every run. Exit 0 on all three (AD-10: per-unit outcomes, including
// every keep and partial, are folded into their own disclosed line and
// never fail the run).
func TestGc_CLI_ExecutionSlice_ThreeRunLifecycle_ReclaimThenOrphanedThenSteadyState(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, headSHA := gcExecutionCLIFixture(t)

	workspaceID := materializeExecutionWorkspace(t, root, "run-three-cycle", headSHA)
	releaser := execworkspace.NewReleaser(root)
	if err := releaser.Release(workspaceID); err != nil {
		t.Fatalf("Release(%s): %v", workspaceID, err)
	}

	runGC := func(t *testing.T, label string) (stdout string, exitCode int) {
		t.Helper()
		cmd := exec.Command(bin, "gc")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err := cmd.Run()
		if err == nil {
			return stdoutBuf.String(), 0
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdoutBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("%s: verdi gc failed to run at all: %v\nstdout: %s\nstderr: %s", label, err, stdoutBuf.String(), stderrBuf.String())
		return "", -1
	}

	// Run 1: reclaim (rank 5).
	out1, code1 := runGC(t, "run 1")
	if code1 != 0 {
		t.Fatalf("run 1 exit code = %d, want 0 (AD-10: per-unit outcomes never fail the run)\nstdout: %s", code1, out1)
	}
	if !strings.Contains(out1, "execution: reclaimed") || !strings.Contains(out1, workspaceID) {
		t.Fatalf("run 1 stdout = %q, want the execution reclaimed line naming %s", out1, workspaceID)
	}
	if !strings.Contains(out1, "execution workspaces") {
		t.Fatalf("run 1 stdout = %q, want the grown (closed-triple) scope disclosure naming execution workspaces", out1)
	}

	// Run 2: the real `git worktree add` registration materialize
	// produced survives rank 5 (which never reconciles the registry, per
	// spec's own fixed five-step deletion order) — this run resolves it
	// at rank 0, disclosed as reclaim-orphaned.
	out2, code2 := runGC(t, "run 2")
	if code2 != 0 {
		t.Fatalf("run 2 exit code = %d, want 0\nstdout: %s", code2, out2)
	}
	if !strings.Contains(out2, "execution: reclaim-orphaned") || !strings.Contains(out2, workspaceID) {
		t.Fatalf("run 2 stdout = %q, want the execution reclaim-orphaned line naming %s", out2, workspaceID)
	}

	// Run 3: steady state — nothing left names this workspace at all, so
	// nothing execution-specific should print except the scope line,
	// which prints on every run regardless of whether anything happened.
	out3, code3 := runGC(t, "run 3")
	if code3 != 0 {
		t.Fatalf("run 3 exit code = %d, want 0\nstdout: %s", code3, out3)
	}
	for _, marker := range []string{"execution: reclaimed", "execution: reclaim-orphaned", "execution: kept", "execution: partial", workspaceID} {
		if strings.Contains(out3, marker) {
			t.Fatalf("run 3 stdout = %q, steady state must not mention %q", out3, marker)
		}
	}
	if !strings.Contains(out3, "execution workspaces") {
		t.Fatalf("run 3 stdout = %q, want the scope line still naming execution workspaces (it prints unconditionally, not only when something happened)", out3)
	}
}
