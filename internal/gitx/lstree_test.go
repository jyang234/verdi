package gitx

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

func buildLsTreeRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/specs/active/foo/spec.md":    "foo\n",
				".verdi/specs/active/bar/spec.md":    "bar\n",
				".verdi/specs/active/bar/board.json": "{}\n",
				".verdi/specs/archive/baz/spec.md":   "baz\n",
				"unrelated.txt":                      "x\n",
			},
			Message: "seed specs",
		},
	})
}

func TestLsTree_Happy(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	got, err := LsTree(ctx, repo.Dir, repo.Head, ".verdi/specs/active")
	if err != nil {
		t.Fatalf("LsTree: %v", err)
	}
	want := []string{
		".verdi/specs/active/bar/board.json",
		".verdi/specs/active/bar/spec.md",
		".verdi/specs/active/foo/spec.md",
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LsTree(active) = %v, want %v", got, want)
	}
}

// TestLsTree_MissingPath_EmptyNotError proves a path absent at ref returns
// an empty, nil-error result — the distinction spec/ref-index ac-4 needs
// between "no spec.md yet" (empty, no error) and a ref that fails to
// resolve at all (a real error, TestLsTree_Negative below).
func TestLsTree_MissingPath_EmptyNotError(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	got, err := LsTree(ctx, repo.Dir, repo.Head, ".verdi/specs/active/nonexistent")
	if err != nil {
		t.Fatalf("LsTree(missing path): unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("LsTree(missing path) = %v, want empty", got)
	}
}

func TestLsTree_Negative(t *testing.T) {
	repo := buildLsTreeRepo(t)
	ctx := context.Background()

	t.Run("ref does not resolve", func(t *testing.T) {
		if _, err := LsTree(ctx, repo.Dir, "not-a-real-ref", ".verdi/specs/active"); err == nil {
			t.Fatal("LsTree(bogus ref): want error, got nil")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := LsTree(ctx, notARepo, repo.Head, ".verdi/specs/active"); err == nil {
			t.Fatal("LsTree outside a repo: want error, got nil")
		}
	})
}
