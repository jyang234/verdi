package gitx

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestLocate is Locate's table: the top level every caller resolves
// repository-root-relative paths against, and dir's own prefix below it.
func TestLocate(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{"a.txt": "a\n", "store/.verdi/.gitignore": "data/\n", "store/deep/b.txt": "b\n"},
		Message: "seed",
	}})
	top, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		dir        string
		wantPrefix string
	}{
		{name: "the top level itself has an empty prefix", dir: repo.Dir, wantPrefix: ""},
		{name: "a nested store's prefix ends in a slash", dir: filepath.Join(repo.Dir, "store"), wantPrefix: "store/"},
		{name: "a deeper directory's prefix names every level", dir: filepath.Join(repo.Dir, "store", "deep"), wantPrefix: "store/deep/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Locate(context.Background(), tc.dir)
			if err != nil {
				t.Fatalf("Locate: %v", err)
			}
			gotTop, err := filepath.EvalSymlinks(got.TopLevel)
			if err != nil {
				t.Fatalf("EvalSymlinks(%q): %v", got.TopLevel, err)
			}
			if gotTop != top {
				t.Errorf("TopLevel = %q, want %q", gotTop, top)
			}
			if got.Prefix != tc.wantPrefix {
				t.Errorf("Prefix = %q, want %q", got.Prefix, tc.wantPrefix)
			}
		})
	}
}

// TestLocate_Negative proves the two failures surface as errors, never as
// a guessed location: a directory outside any repository, and a prefix
// holding a newline, which the line-oriented answer cannot carry
// unambiguously.
func TestLocate_Negative(t *testing.T) {
	t.Run("outside any repository", func(t *testing.T) {
		if _, err := Locate(context.Background(), t.TempDir()); err == nil {
			t.Fatal("Locate outside a repo: want error, got nil")
		}
	})
	t.Run("a prefix holding a newline", func(t *testing.T) {
		repo := buildRepo(t)
		odd := filepath.Join(repo.Dir, "two\nlines")
		if err := os.Mkdir(odd, 0o755); err != nil {
			t.Skipf("this filesystem refuses a newline in a name: %v", err)
		}
		if _, err := Locate(context.Background(), odd); err == nil {
			t.Fatal("Locate with a newline in the prefix: want error, got nil")
		}
	})
}
