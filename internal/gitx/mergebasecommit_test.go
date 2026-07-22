package gitx

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func buildTwoCommitRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "1\n"}, Message: "c1"},
		{Files: map[string]string{"a.txt": "2\n"}, Message: "c2"},
	})
}

// TestMergeBaseCommit_Found proves a real common ancestor is returned with
// found=true — the ordinary case where HEAD descends from the default branch.
func TestMergeBaseCommit_Found(t *testing.T) {
	repo := buildTwoCommitRepo(t)
	ctx := context.Background()

	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	parent, err := RevParse(ctx, repo.Dir, "HEAD^")
	if err != nil {
		t.Fatal(err)
	}

	base, found, err := MergeBaseCommit(ctx, repo.Dir, head, parent)
	if err != nil {
		t.Fatalf("MergeBaseCommit(child, parent): unexpected error: %v", err)
	}
	if !found {
		t.Fatal("MergeBaseCommit(child, parent) found=false, want true")
	}
	if base != parent {
		t.Fatalf("MergeBaseCommit(child, parent) = %q, want the parent %q", base, parent)
	}
}

// TestMergeBaseCommit_NoCommonAncestor proves disjoint histories return
// found=false and NO error — git's exit-1 "no merge base" is a clean negative
// (proceed), never conflated with an operational failure (refuse).
func TestMergeBaseCommit_NoCommonAncestor(t *testing.T) {
	repo := buildTwoCommitRepo(t)
	ctx := context.Background()

	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// A disjoint-history root: an orphan commit sharing no ancestry with head.
	id := []string{"-c", "user.email=t@example.com", "-c", "user.name=Test"}
	if _, err := run(ctx, repo.Dir, append(id, "checkout", "--orphan", "disjoint")...); err != nil {
		t.Fatalf("checkout --orphan: %v", err)
	}
	if _, err := run(ctx, repo.Dir, append(id, "commit", "--allow-empty", "-m", "disjoint root")...); err != nil {
		t.Fatalf("commit orphan root: %v", err)
	}
	disjoint, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	base, found, err := MergeBaseCommit(ctx, repo.Dir, head, disjoint)
	if err != nil {
		t.Fatalf("MergeBaseCommit(disjoint histories): unexpected error (exit-1 no-ancestor must not be an error): %v", err)
	}
	if found {
		t.Fatal("MergeBaseCommit(disjoint histories) found=true, want false")
	}
	if base != "" {
		t.Fatalf("MergeBaseCommit(disjoint histories) base=%q, want empty", base)
	}
}

// TestMergeBaseCommit_OperationalFailure proves a bad/unresolvable ref is a
// surfaced error (git exit 128), never a silent found=false — the whole reason
// this exists over the bare MergeBase: a caller must tell "no merge base" from
// "could not ask git" so it never guesses about frozen-ness on a git failure.
func TestMergeBaseCommit_OperationalFailure(t *testing.T) {
	repo := buildTwoCommitRepo(t)
	ctx := context.Background()

	head, err := RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	base, found, err := MergeBaseCommit(ctx, repo.Dir, head, "not-a-real-ref")
	if err == nil {
		t.Fatal("MergeBaseCommit(bad ref): want an operational error, got nil")
	}
	if found {
		t.Fatal("MergeBaseCommit(bad ref) found=true, want false")
	}
	if base != "" {
		t.Fatalf("MergeBaseCommit(bad ref) base=%q, want empty", base)
	}
}
