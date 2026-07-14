// Real, built-binary end-to-end tests for `verdi gc` (spec/worktree-
// manager ac-5): the CLI dispatch flip itself is proven by
// dispatch_test.go's TestRun_GcDispatchesToRealVerb and
// internal/specalign's TestV0CLIVerbInventory; this file drives the
// actual compiled binary against a real, hermetic fixturegit checkout to
// prove gc's real reclaim/keep behavior end to end, matching this
// package's own serve_integration_test.go pattern (buildVerdiBinary).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// gcCLIFixture builds a real store root (a real git checkout store.FindRoot
// accepts, mirroring newIntegrationStoreRoot's shape) with one design
// branch merged into main — a reclaim-eligible managed worktree once cut.
func gcCLIFixture(t *testing.T) (root, branch string) {
	t.Helper()
	manifest := "schema: verdi.layout/v1\n"
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": manifest, ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	root = repo.Dir
	branch = "design/reclaimable"
	ctx := context.Background()

	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
	}
	if err := os.WriteFile(filepath.Join(root, "draft.txt"), []byte("draft\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, root, "add", "-A")
	runGitCmd(t, root, "commit", "--quiet", "-m", "draft")
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGitCmd(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
	return root, branch
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v (dir %s): %v\n%s", args, dir, err, out)
	}
}

// TestGc_CLI_ReclaimsAndDisclosesScope is ac-5's behavioral obligation,
// verbatim: exit code 0, the reclaimable worktree actually removed on
// disk, a printed reclaim line, and a printed line disclosing that
// derived-cache/layout-cache pruning were NOT run by this invocation.
func TestGc_CLI_ReclaimsAndDisclosesScope(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, branch := gcCLIFixture(t)

	path, err := wtmanager.EnsureWorktree(context.Background(), root, branch)
	if err != nil {
		t.Fatalf("EnsureWorktree seeding gc CLI fixture: %v", err)
	}

	cmd := exec.Command(bin, "gc")
	cmd.Dir = root
	// No real "origin" remote exists in this hermetic fixture (co-2: no
	// network in any test), so lint.ResolveDefaultBranch's git-based
	// fallback would resolve to "" — CI_DEFAULT_BRANCH is its first,
	// higher-priority source and needs no remote at all.
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi gc: %v\nstdout: %s\nstderr: %s", err, stdout.String(), stderr.String())
	}

	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("reclaimable worktree %s still on disk after `verdi gc`: err=%v", path, statErr)
	}

	out := stdout.String()
	if !strings.Contains(out, "reclaimed") {
		t.Fatalf("verdi gc stdout = %q, want a printed reclaim line", out)
	}
	if !strings.Contains(out, "derived-cache") || !strings.Contains(out, "layout") || !strings.Contains(strings.ToLower(out), "out of scope") {
		t.Fatalf("verdi gc stdout = %q, want a verbatim scope-disclosure line naming derived-cache and layout-cache pruning as out of scope", out)
	}
}

// TestGc_CLI_KeepsDirtyAndDiscloses proves the CLI end to end for the
// dirty-keep path too: gc must still exit 0 (a kept worktree is not a
// verdict failure) and disclose the keep-reason, not just the reclaim
// path above.
func TestGc_CLI_KeepsDirtyAndDiscloses(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, branch := gcCLIFixture(t)

	path, err := wtmanager.EnsureWorktree(context.Background(), root, branch)
	if err != nil {
		t.Fatalf("EnsureWorktree seeding gc CLI fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "uncommitted.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
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

	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("dirty worktree removed by `verdi gc`, want it kept: %v", statErr)
	}
	out := stdout.String()
	if !strings.Contains(out, "kept") || !strings.Contains(out, "uncommitted") {
		t.Fatalf("verdi gc stdout = %q, want a disclosed \"kept: uncommitted changes\" line", out)
	}
}

// TestGc_CLI_UnexpectedArgument proves gc's own argument parsing refuses
// an unexpected argument operationally (exit 2) rather than silently
// ignoring it.
func TestGc_CLI_UnexpectedArgument(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, _ := gcCLIFixture(t)

	cmd := exec.Command(bin, "gc", "--bogus")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("verdi gc --bogus: want a non-zero exit, got nil error")
	}
	if !strings.Contains(stderr.String(), "unexpected argument") {
		t.Fatalf("verdi gc --bogus stderr = %q, want it to name the unexpected argument", stderr.String())
	}
}
