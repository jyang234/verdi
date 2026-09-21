package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// absModuleRoot resolves testModuleRoot absolutely: provisionStore fetches
// ancestry from the module's own git repository by path, relative to the
// scratch store's cwd, exactly as main.go's os.Getwd module root is.
func absModuleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(testModuleRoot)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// TestProvisionSharedStore_AfterMainRunsOnMainBeforeBranches pins the
// extracted sequence's one ordering guarantee: afterMain (main.go's dex
// build) runs exactly once, after the base store exists and while the
// checkout still sits on main with no design branch cut; the finished
// store then sits on the serving design branch with every returned side
// artifact present on disk.
func TestProvisionSharedStore_AfterMainRunsOnMainBeforeBranches(t *testing.T) {
	ctx := context.Background()
	scratch := t.TempDir()

	calls := 0
	var branchAtHook, designRefsAtHook string
	afterMain := func(ctx context.Context, storeRoot string) error {
		calls++
		if storeRoot != filepath.Join(scratch, "store") {
			t.Errorf("afterMain storeRoot = %q, want %q", storeRoot, filepath.Join(scratch, "store"))
		}
		if _, err := os.Stat(filepath.Join(storeRoot, ".verdi", "verdi.yaml")); err != nil {
			t.Errorf("afterMain ran before the base store existed: %v", err)
		}
		branchAtHook, _ = gitOutput(ctx, storeRoot, "rev-parse", "--abbrev-ref", "HEAD")
		designRefsAtHook, _ = gitOutput(ctx, storeRoot, "for-each-ref", "--format=%(refname)", "refs/heads/design/")
		return nil
	}

	store, err := provisionSharedStore(ctx, absModuleRoot(t), scratch, afterMain)
	if err != nil {
		t.Fatalf("provisionSharedStore: %v", err)
	}
	if calls != 1 {
		t.Fatalf("afterMain ran %d times, want exactly once", calls)
	}
	if branchAtHook != "main" {
		t.Errorf("checkout at afterMain = %q, want main (the dex build must see main)", branchAtHook)
	}
	if designRefsAtHook != "" {
		t.Errorf("design branches already cut at afterMain: %q, want none", designRefsAtHook)
	}

	if store.storeRoot != filepath.Join(scratch, "store") {
		t.Errorf("storeRoot = %q, want %q", store.storeRoot, filepath.Join(scratch, "store"))
	}
	for label, path := range map[string]string{
		"feedPath":             store.feedPath,
		"verificationPath":     store.verificationPath,
		"readinessRequestPath": store.readinessRequestPath,
	} {
		if path == "" {
			t.Errorf("%s is empty", label)
			continue
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s = %q is not on disk: %v", label, path, err)
		}
	}
	if branch, _ := gitOutput(ctx, store.storeRoot, "rev-parse", "--abbrev-ref", "HEAD"); branch != designBranch {
		t.Errorf("finished checkout = %q, want the serving branch %s", branch, designBranch)
	}
	if head, _ := gitOutput(ctx, store.storeRoot, "symbolic-ref", "refs/remotes/origin/HEAD"); head != "refs/remotes/origin/main" {
		t.Errorf("origin/HEAD = %q, want refs/remotes/origin/main (the shared store proves its default branch)", head)
	}
}

// TestProvisionSharedStore_Negative_AfterMainError: a failing afterMain
// stops the sequence there — its error surfaces unwrapped-in-meaning, and
// no design-branch provisioner ran after it.
func TestProvisionSharedStore_Negative_AfterMainError(t *testing.T) {
	ctx := context.Background()
	scratch := t.TempDir()
	sentinel := errors.New("dex build refused")

	_, err := provisionSharedStore(ctx, absModuleRoot(t), scratch, func(context.Context, string) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want the afterMain error", err)
	}
	storeRoot := filepath.Join(scratch, "store")
	if refs, _ := gitOutput(ctx, storeRoot, "for-each-ref", "--format=%(refname)", "refs/heads/design/"); refs != "" {
		t.Errorf("design branches cut after afterMain failed: %q", refs)
	}
	if remotes, _ := gitOutput(ctx, storeRoot, "remote"); strings.Contains(remotes, "origin") {
		t.Errorf("provisionBoard ran after afterMain failed: remotes=%q", remotes)
	}
}

// TestProvisionSharedStore_Negative_MissingModuleRoot: a module root with
// no examples/showcase corpus is a disclosed provisioning error, never an
// empty store dressed as the shared one.
func TestProvisionSharedStore_Negative_MissingModuleRoot(t *testing.T) {
	calls := 0
	_, err := provisionSharedStore(context.Background(), t.TempDir(), t.TempDir(), func(context.Context, string) error { calls++; return nil })
	if err == nil || !strings.Contains(err.Error(), "provisioning scratch store") {
		t.Fatalf("err = %v, want the base-store stage named", err)
	}
	if calls != 0 {
		t.Errorf("afterMain ran %d times after the base store failed, want 0", calls)
	}
}
