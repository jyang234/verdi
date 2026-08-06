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
