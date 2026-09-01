package gitx

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func TestAheadBehind_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// A branch cut at layer 1 while HEAD sits at layer 2: HEAD is one
	// ahead of it and zero behind.
	branch := exec.Command("git", "branch", "base", repo.Heads[0])
	branch.Dir = repo.Dir
	if out, err := branch.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}
	ahead, behind, err := AheadBehind(ctx, repo.Dir, "HEAD", "base")
	if err != nil {
		t.Fatalf("AheadBehind(HEAD, base): %v", err)
	}
	if ahead != 1 || behind != 0 {
		t.Fatalf("AheadBehind(HEAD, base) = %d/%d, want 1/0", ahead, behind)
	}
	// The mirror orientation counts the other side.
	ahead, behind, err = AheadBehind(ctx, repo.Dir, "base", "HEAD")
	if err != nil {
		t.Fatalf("AheadBehind(base, HEAD): %v", err)
	}
	if ahead != 0 || behind != 1 {
		t.Fatalf("AheadBehind(base, HEAD) = %d/%d, want 0/1", ahead, behind)
	}
	// Identical revs: clean zero on both sides.
	ahead, behind, err = AheadBehind(ctx, repo.Dir, "HEAD", "HEAD")
	if err != nil || ahead != 0 || behind != 0 {
		t.Fatalf("AheadBehind(HEAD, HEAD) = %d/%d, %v; want 0/0, nil", ahead, behind, err)
	}
}

func TestAheadBehind_Negative(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{"a.txt": "x\n"}, Message: "seed",
	}})
	if _, _, err := AheadBehind(context.Background(), repo.Dir, "HEAD", "no-such-ref"); err == nil {
		t.Fatal("AheadBehind against a missing ref = nil error, want a failure")
	}
	if _, _, err := AheadBehind(context.Background(), t.TempDir(), "HEAD", "HEAD"); err == nil || !strings.Contains(err.Error(), "git") {
		t.Fatalf("AheadBehind outside a repository = %v, want a git failure", err)
	}
}
