package gitx

import (
	"context"
	"os/exec"
	"testing"
)

func TestCurrentBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// fixturegit commits directly without necessarily naming the branch
	// "main"/"master" across every git version's default; discover it via
	// the same repo instead of hardcoding a name.
	out, err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("git symbolic-ref (test setup): %v", err)
	}
	want := string(out)
	for len(want) > 0 && (want[len(want)-1] == '\n' || want[len(want)-1] == '\r') {
		want = want[:len(want)-1]
	}

	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != want {
		t.Fatalf("CurrentBranch = %q, want %q", got, want)
	}
}

func TestCurrentBranch_Negative_DetachedHead(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "checkout", "--detach", repo.Head).Run(); err != nil {
		t.Fatalf("detaching HEAD (test setup): %v", err)
	}

	if _, err := CurrentBranch(ctx, repo.Dir); err == nil {
		t.Fatal("CurrentBranch on a detached HEAD: want error, got nil")
	}
}

func TestCurrentBranch_Negative_NotARepo(t *testing.T) {
	if _, err := CurrentBranch(context.Background(), t.TempDir()); err == nil {
		t.Fatal("CurrentBranch outside a git repo: want error, got nil")
	}
}
