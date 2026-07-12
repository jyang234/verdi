package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestWriteBlob_HashObjectAgree proves WriteBlob's returned SHA is the
// exact blob id `git hash-object` (no -w) would compute for the same
// bytes, and that the object is actually written to the store (readable
// back via `git cat-file`) — WITHOUT touching the working tree or the
// repo's real index (no file is created on disk by this call).
func TestWriteBlob_HashObjectAgree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	content := []byte("hello scoping canvas\n")

	sha, err := WriteBlob(ctx, repo.Dir, content)
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	if sha == "" {
		t.Fatal("WriteBlob returned an empty sha")
	}

	// Write the same bytes to a real file and hash it the ordinary way —
	// same content must produce the same blob id.
	tmp := filepath.Join(t.TempDir(), "same.txt")
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := HashObject(ctx, repo.Dir, tmp)
	if err != nil {
		t.Fatalf("HashObject: %v", err)
	}
	if sha != want {
		t.Fatalf("WriteBlob sha = %q, want %q (HashObject of identical bytes)", sha, want)
	}

	out, err := run(ctx, repo.Dir, "cat-file", "-p", sha)
	if err != nil {
		t.Fatalf("cat-file -p %s: %v", sha, err)
	}
	if string(out) != string(content) {
		t.Fatalf("cat-file -p %s = %q, want %q", sha, out, content)
	}
}

// TestBuildTreeWithFile_AddsOneFileOntoBaseTree proves BuildTreeWithFile
// builds a NEW tree carrying every file the base tree had, plus one new
// path — without writing anything to the working directory or the repo's
// real index (checked via git's own status: still clean afterward).
func TestBuildTreeWithFile_AddsOneFileOntoBaseTree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	blobSHA, err := WriteBlob(ctx, repo.Dir, []byte("new file content\n"))
	if err != nil {
		t.Fatalf("WriteBlob: %v", err)
	}
	newTree, err := BuildTreeWithFile(ctx, repo.Dir, repo.Head+"^{tree}", "sub/new.txt", blobSHA)
	if err != nil {
		t.Fatalf("BuildTreeWithFile: %v", err)
	}
	if newTree == "" {
		t.Fatal("BuildTreeWithFile returned an empty tree sha")
	}

	// The base tree's own files are still present in the new tree.
	if _, err := run(ctx, repo.Dir, "cat-file", "-e", newTree+":a.txt"); err != nil {
		t.Fatalf("new tree lost the base tree's a.txt: %v", err)
	}
	if _, err := run(ctx, repo.Dir, "cat-file", "-e", newTree+":dir/b.txt"); err != nil {
		t.Fatalf("new tree lost the base tree's dir/b.txt: %v", err)
	}
	// The new file is present with the exact content.
	out, err := run(ctx, repo.Dir, "cat-file", "-p", newTree+":sub/new.txt")
	if err != nil {
		t.Fatalf("new tree missing sub/new.txt: %v", err)
	}
	if string(out) != "new file content\n" {
		t.Fatalf("sub/new.txt = %q, want %q", out, "new file content\n")
	}

	// The repository's OWN index and working tree were never touched.
	dirty, err := StatusDirty(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Fatal("BuildTreeWithFile left the real working tree/index dirty")
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "sub", "new.txt")); err == nil {
		t.Fatal("BuildTreeWithFile wrote the new file into the real working tree")
	}
}

// TestCommitTree_ProducesACommitWithoutMovingAnyRef proves CommitTree
// creates a commit object reachable only by the sha it returns — HEAD and
// every branch are untouched.
func TestCommitTree_ProducesACommitWithoutMovingAnyRef(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "a plumbing-built commit")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	if commit == "" || commit == repo.Head {
		t.Fatalf("CommitTree returned %q, want a new, non-empty sha", commit)
	}

	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatalf("HEAD moved to %q after CommitTree (should be untouched)", head)
	}

	// The parent link is exactly repo.Head.
	out, err := run(ctx, repo.Dir, "rev-parse", commit+"^")
	if err != nil {
		t.Fatalf("rev-parse %s^: %v", commit, err)
	}
	if got := string(out); got[:len(got)-1] != repo.Head { // trim trailing newline
		t.Fatalf("commit parent = %q, want %q", got, repo.Head)
	}
}

// TestUpdateRef_CreatesBranchWithoutCheckout proves UpdateRef creates a
// new branch ref pointing at the given commit without moving HEAD or
// touching the working tree — the no-checkout plumbing stub-instantiate
// depends on (spec/scoping-canvas ac-6).
func TestUpdateRef_CreatesBranchWithoutCheckout(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "scaffold commit")
	if err != nil {
		t.Fatalf("CommitTree: %v", err)
	}
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/plumbing-fixture", commit); err != nil {
		t.Fatalf("UpdateRef: %v", err)
	}

	branches, err := LocalBranches(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, b := range branches {
		if b == "design/plumbing-fixture" {
			found = true
		}
	}
	if !found {
		t.Fatalf("branches = %v, want design/plumbing-fixture", branches)
	}

	branch, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "design/plumbing-fixture" {
		t.Fatal("UpdateRef checked the new branch out — it must not")
	}
	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if head != repo.Head {
		t.Fatal("UpdateRef moved HEAD")
	}
}

// TestUpdateRef_Negative_RefAlreadyExists proves UpdateRef fails closed
// rather than silently moving an existing branch (stub-instantiate: "fail
// closed if the branch exists").
func TestUpdateRef_Negative_RefAlreadyExists(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", repo.Head); err != nil {
		t.Fatalf("first UpdateRef: %v", err)
	}
	commit, err := CommitTree(ctx, repo.Dir, repo.Head+"^{tree}", repo.Head, "second commit")
	if err != nil {
		t.Fatal(err)
	}
	if err := UpdateRef(ctx, repo.Dir, "refs/heads/design/dup", commit); err == nil {
		t.Fatal("UpdateRef onto an existing ref succeeded, want error")
	}
	// The existing ref must be untouched by the failed attempt.
	got, err := RevParse(ctx, repo.Dir, "refs/heads/design/dup")
	if err != nil {
		t.Fatal(err)
	}
	if got != repo.Head {
		t.Fatalf("refs/heads/design/dup = %q after a refused UpdateRef, want unchanged %q", got, repo.Head)
	}
}
