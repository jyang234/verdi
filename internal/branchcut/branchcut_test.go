package branchcut

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

func TestUnwind_EmptyCutIsUnwound(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != Unwound {
		t.Fatalf("outcome = %v, stderr %q", got, stderr.String())
	}
	if branches, _ := gitx.LocalBranches(ctx, repo.Dir); slices.Contains(branches, "close/foo") {
		t.Fatal("close/foo still exists")
	}
	if cur, _ := gitx.CurrentBranch(ctx, repo.Dir); cur != "main" {
		t.Fatalf("current branch = %q", cur)
	}
}

func TestUnwind_AheadOfCutIsLeft(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n"}, Message: "a"}})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "close/foo"); err != nil {
		t.Fatal(err)
	}
	// fixturegit.Dangle does not commit on the current branch (it commits
	// on a throwaway branch of its own, then switches back to "main" and
	// force-deletes it) — the wrong shape for "close/foo carries a commit
	// beyond its cut point". Commit directly on close/foo instead.
	if err := os.WriteFile(filepath.Join(repo.Dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitx.AddPaths(ctx, repo.Dir, "b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "own commit"); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if got := Unwind(ctx, repo.Dir, "main", "close/foo", repo.Head, "recover", &stderr); got != LeftAheadOfCut {
		t.Fatalf("outcome = %v", got)
	}
	if !strings.Contains(stderr.String(), "recover: left close/foo in place: it carries commit(s) beyond its cut point") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
