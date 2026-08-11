package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/evidence"
)

// TestCopyTreePresent proves a present source tree copies through.
func TestCopyTreePresent(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("there"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatalf("reading copied file: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("a.txt = %q, want %q", got, "hi")
	}
	got, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("reading copied nested file: %v", err)
	}
	if string(got) != "there" {
		t.Fatalf("sub/b.txt = %q, want %q", got, "there")
	}
}

// TestCopyTreeAbsent proves a missing source is tolerated: no error, no
// destination created.
func TestCopyTreeAbsent(t *testing.T) {
	src := filepath.Join(t.TempDir(), "does-not-exist")
	dst := filepath.Join(t.TempDir(), "out")

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree on absent src: got %v, want nil", err)
	}
	if _, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected dst to remain absent, stat err = %v", err)
	}
}

// TestCopyTreeUnreadable proves a stat failure other than "not exist" (here:
// the parent directory denying search/traversal permission) returns a
// wrapped error instead of being swallowed alongside the absent case.
//
// Skips under root: root bypasses the permission bits this test relies on.
func TestCopyTreeUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not block stat")
	}

	parent := t.TempDir()
	src := filepath.Join(parent, "locked")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	err := copyTree(src, dst)
	if err == nil {
		t.Fatal("expected an error for an unreadable source, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected a non-NotExist error, got %v", err)
	}
}

func TestAttachObligationQualityAdoptionAncestry(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, "fixture.txt"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(t.Context(), storeRoot); err != nil {
		t.Fatal(err)
	}
	beforeHead, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	beforeTree, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}

	if err := attachObligationQualityAdoptionAncestry(t.Context(), moduleRoot, storeRoot); err != nil {
		t.Fatalf("attachObligationQualityAdoptionAncestry: %v", err)
	}
	if err := runGit(t.Context(), storeRoot, nil, "merge-base", "--is-ancestor", evidence.ObligationQualityAdoptionCommit, "HEAD"); err != nil {
		t.Fatalf("adoption is not an ancestor of scratch HEAD: %v", err)
	}
	afterHead, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	afterTree, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead || afterTree != beforeTree {
		t.Fatalf("ancestry attachment changed scratch identity/tree: head %s -> %s, tree %s -> %s", beforeHead, afterHead, beforeTree, afterTree)
	}
}

func TestAttachObligationQualityAdoptionAncestryFromShallowSource(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	sourceRoot := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(sourceRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(t.Context(), sourceRoot, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(t.Context(), sourceRoot, nil, "fetch", "--quiet", "--no-tags", "--depth=1", moduleRoot, evidence.ObligationQualityAdoptionCommit); err != nil {
		t.Fatalf("creating shallow adoption source: %v", err)
	}
	sourceShallow, err := gitOutput(t.Context(), sourceRoot, "rev-parse", "--is-shallow-repository")
	if err != nil {
		t.Fatal(err)
	}
	if sourceShallow != "true" {
		t.Fatalf("source shallow state = %q, want true", sourceShallow)
	}
	if err := runGit(t.Context(), sourceRoot, nil, "cat-file", "-e", evidence.ObligationQualityAdoptionCommit+"^"); err == nil {
		t.Fatal("shallow source contains adoption parent, want it unavailable")
	}
	sourceAdoptionTree, err := gitOutput(t.Context(), sourceRoot, "rev-parse", evidence.ObligationQualityAdoptionCommit+"^{tree}")
	if err != nil {
		t.Fatal(err)
	}

	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, "fixture.txt"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(t.Context(), storeRoot); err != nil {
		t.Fatal(err)
	}
	beforeHead, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	beforeTree, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}

	if err := attachObligationQualityAdoptionAncestry(t.Context(), sourceRoot, storeRoot); err != nil {
		t.Fatalf("attachObligationQualityAdoptionAncestry from shallow source: %v", err)
	}
	if err := runGit(t.Context(), storeRoot, nil, "--no-replace-objects", "cat-file", "-e", evidence.ObligationQualityAdoptionCommit+"^"); err == nil {
		t.Fatal("scratch store contains adoption parent, want exact adoption imported as a boundary")
	}
	rawAdoption, err := gitOutput(t.Context(), storeRoot, "--no-replace-objects", "rev-parse", evidence.ObligationQualityAdoptionCommit+"^{commit}")
	if err != nil {
		t.Fatal(err)
	}
	rawAdoptionTree, err := gitOutput(t.Context(), storeRoot, "--no-replace-objects", "rev-parse", evidence.ObligationQualityAdoptionCommit+"^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if rawAdoption != evidence.ObligationQualityAdoptionCommit || rawAdoptionTree != sourceAdoptionTree {
		t.Fatalf("raw adoption identity/tree changed: commit %s, tree %s; want commit %s, tree %s", rawAdoption, rawAdoptionTree, evidence.ObligationQualityAdoptionCommit, sourceAdoptionTree)
	}
	// The adoption-root graft stops traversal at the available proof horizon;
	// the scratch-HEAD graft makes that exact adoption its synthetic parent.
	if err := runGit(t.Context(), storeRoot, nil, "merge-base", "--is-ancestor", evidence.ObligationQualityAdoptionCommit, "HEAD"); err != nil {
		t.Fatalf("boundary adoption is not an ancestor of scratch HEAD: %v", err)
	}
	shallow, err := gitOutput(t.Context(), storeRoot, "rev-parse", "--is-shallow-repository")
	if err != nil {
		t.Fatal(err)
	}
	if shallow != "false" {
		t.Fatalf("scratch shallow state = %q, want false for fixture-wide ancestry honesty", shallow)
	}
	afterHead, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	afterTree, err := gitOutput(t.Context(), storeRoot, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead != beforeHead || afterTree != beforeTree {
		t.Fatalf("boundary import changed scratch identity/tree: head %s -> %s, tree %s -> %s", beforeHead, afterHead, beforeTree, afterTree)
	}
}

func TestAttachObligationQualityAdoptionAncestryRejectsMissingSource(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storeRoot, "fixture.txt"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitInitAndCommit(t.Context(), storeRoot); err != nil {
		t.Fatal(err)
	}
	if err := attachObligationQualityAdoptionAncestry(t.Context(), filepath.Join(t.TempDir(), "missing"), storeRoot); err == nil {
		t.Fatal("attachObligationQualityAdoptionAncestry with missing source returned nil")
	}
}
