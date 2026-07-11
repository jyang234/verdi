package gitx

import (
	"context"
	"os/exec"
	"testing"
)

func TestRemoteURL_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	const want = "https://example.invalid/acme/svcfix.git"
	if err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "remote", "add", "origin", want).Run(); err != nil {
		t.Fatalf("git remote add (test setup): %v", err)
	}

	got, err := RemoteURL(ctx, repo.Dir, "origin")
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if got != want {
		t.Fatalf("RemoteURL = %q, want %q", got, want)
	}
}

func TestRemoteURL_Negative_NoSuchRemote(t *testing.T) {
	repo := buildRepo(t)
	if _, err := RemoteURL(context.Background(), repo.Dir, "does-not-exist"); err == nil {
		t.Fatal("RemoteURL(nonexistent remote): want error, got nil")
	}
}

func TestRemoteURL_Negative_NotARepo(t *testing.T) {
	if _, err := RemoteURL(context.Background(), t.TempDir(), "origin"); err == nil {
		t.Fatal("RemoteURL outside a git repo: want error, got nil")
	}
}
