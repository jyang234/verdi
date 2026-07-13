package gitx_test

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestLog_HappyPath(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"b.txt": "world\n"}, Message: "add b"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "update a"},
	})

	commits, err := gitx.Log(context.Background(), repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("len(commits) = %d, want 3", len(commits))
	}
	// Most-recent-first.
	if commits[0].Subject != "update a" {
		t.Errorf("commits[0].Subject = %q, want %q", commits[0].Subject, "update a")
	}
	if commits[2].Subject != "add a" {
		t.Errorf("commits[2].Subject = %q, want %q", commits[2].Subject, "add a")
	}
	if commits[0].SHA != repo.Head {
		t.Errorf("commits[0].SHA = %q, want HEAD %q", commits[0].SHA, repo.Head)
	}
	if commits[0].Author == "" {
		t.Error("commits[0].Author is empty")
	}
	if commits[0].Date == "" {
		t.Error("commits[0].Date is empty")
	}
}

func TestLog_PathScoped(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"b.txt": "world\n"}, Message: "add b"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "update a"},
	})

	commits, err := gitx.Log(context.Background(), repo.Dir, "HEAD", "b.txt")
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1 (only 'add b' touched b.txt)", len(commits))
	}
	if commits[0].Subject != "add b" {
		t.Errorf("commits[0].Subject = %q, want %q", commits[0].Subject, "add b")
	}
}

func TestLog_NoMatchingHistory_ReturnsNilNotError(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	commits, err := gitx.Log(context.Background(), repo.Dir, "HEAD", "never-existed.txt")
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if commits != nil {
		t.Fatalf("commits = %v, want nil", commits)
	}
}

func TestLog_NotARepo_Errors(t *testing.T) {
	if _, err := gitx.Log(context.Background(), t.TempDir(), "HEAD"); err == nil {
		t.Fatal("Log: expected error for a non-git directory, got nil")
	}
}

func TestLog_Deterministic(t *testing.T) {
	layers := []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"b.txt": "world\n"}, Message: "add b"},
	}
	r1 := fixturegit.Build(t, layers)
	r2 := fixturegit.Build(t, layers)

	c1, err := gitx.Log(context.Background(), r1.Dir, "HEAD")
	if err != nil {
		t.Fatalf("Log(r1): %v", err)
	}
	c2, err := gitx.Log(context.Background(), r2.Dir, "HEAD")
	if err != nil {
		t.Fatalf("Log(r2): %v", err)
	}
	if len(c1) != len(c2) {
		t.Fatalf("len mismatch: %d vs %d", len(c1), len(c2))
	}
	for i := range c1 {
		if c1[i] != c2[i] {
			t.Fatalf("commits[%d] differ across identical builds: %+v vs %+v", i, c1[i], c2[i])
		}
	}
}

func TestLastCommit_HappyPath(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
		{Files: map[string]string{"b.txt": "world\n"}, Message: "add b"},
		{Files: map[string]string{"a.txt": "hello again\n"}, Message: "update a"},
	})

	c, ok, err := gitx.LastCommit(context.Background(), repo.Dir, "HEAD", "a.txt")
	if err != nil {
		t.Fatalf("LastCommit: %v", err)
	}
	if !ok {
		t.Fatal("LastCommit: ok = false, want true")
	}
	if c.Subject != "update a" {
		t.Errorf("Subject = %q, want %q", c.Subject, "update a")
	}
}

func TestLastCommit_NoHistory_NotOK(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	c, ok, err := gitx.LastCommit(context.Background(), repo.Dir, "HEAD", "never-existed.txt")
	if err != nil {
		t.Fatalf("LastCommit: %v", err)
	}
	if ok {
		t.Fatalf("LastCommit: ok = true, want false; got %+v", c)
	}
}

func TestCommitDate_HappyPath(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	date, err := gitx.CommitDate(context.Background(), repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("CommitDate: %v", err)
	}
	// fixturegit pins every commit to 2024-01-01T00:00:00Z (+0000).
	const want = "2024-01-01T00:00:00+00:00"
	if date != want {
		t.Errorf("CommitDate = %q, want %q", date, want)
	}
}

func TestCommitDate_UnknownRev_Errors(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	if _, err := gitx.CommitDate(context.Background(), repo.Dir, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatal("CommitDate: expected error for an unknown rev, got nil")
	}
}
