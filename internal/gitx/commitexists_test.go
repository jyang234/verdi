package gitx

import (
	"context"
	"testing"
)

func TestCommitExists_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	ok, err := CommitExists(ctx, repo.Dir, repo.Heads[0])
	if err != nil {
		t.Fatalf("CommitExists(real commit): %v", err)
	}
	if !ok {
		t.Fatal("CommitExists(real commit) = false, want true")
	}
}

func TestCommitExists_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("well-formed but nonexistent sha", func(t *testing.T) {
		ok, err := CommitExists(ctx, repo.Dir, "0000000000000000000000000000000000000000")
		if err != nil {
			t.Fatalf("CommitExists(bogus sha): unexpected error: %v", err)
		}
		if ok {
			t.Fatal("CommitExists(bogus sha) = true, want false")
		}
	})

	t.Run("blob sha is not a commit", func(t *testing.T) {
		blobSHA, err := RevParse(ctx, repo.Dir, "HEAD:a.txt")
		if err != nil {
			t.Fatalf("RevParse(HEAD:a.txt): %v", err)
		}
		ok, err := CommitExists(ctx, repo.Dir, blobSHA)
		if err != nil {
			t.Fatalf("CommitExists(blob sha): unexpected error: %v", err)
		}
		if ok {
			t.Fatal("CommitExists(blob sha) = true, want false (it names a blob, not a commit)")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := CommitExists(ctx, notARepo, repo.Head); err == nil {
			t.Fatal("CommitExists outside a repo: want error, got nil")
		}
	})
}
