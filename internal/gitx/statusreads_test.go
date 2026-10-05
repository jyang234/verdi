package gitx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// staleIndexRepo builds a repository and touches a tracked file's mtime
// without changing its bytes, so a status read that refreshes the index
// has a stat entry to rewrite. It returns the repo and its index path.
func staleIndexRepo(t *testing.T) (*fixturegit.Repo, string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "a\n", "dir/b.txt": "b\n"}, Message: "seed"}})
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(repo.Dir, "a.txt"), stale, stale); err != nil {
		t.Fatal(err)
	}
	return repo, filepath.Join(repo.Dir, ".git", "index")
}

func readIndex(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestStatusReads_NeverWriteTheIndex is BL-105's index witness (ledger
// SI-343 (3), SI-352): every status read gitx makes on a read path passes
// --no-optional-locks, so a read stays a read — a poll never rewrites the
// served checkout's .git/index, never races a person's own git add or
// commit for index.lock, and never bumps the index stamp a cache guard
// reads. The control row runs the same status without the flag and must
// see the index rewritten, so the touch below is known to give git
// something to refresh: without it every other row would pass vacuously.
func TestStatusReads_NeverWriteTheIndex(t *testing.T) {
	for _, tc := range []struct {
		name    string
		read    func(ctx context.Context, dir string) error
		rewrite bool
	}{
		{name: "control: status without --no-optional-locks rewrites the index", rewrite: true, read: func(ctx context.Context, dir string) error {
			cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "--untracked-files=all")
			cmd.Dir = dir
			return cmd.Run()
		}},
		{name: "StatusDirty", read: func(ctx context.Context, dir string) error {
			_, err := StatusDirty(ctx, dir)
			return err
		}},
		{name: "StagedPaths", read: func(ctx context.Context, dir string) error {
			_, err := StagedPaths(ctx, dir)
			return err
		}},
		{name: "WorktreeChangedPaths", read: func(ctx context.Context, dir string) error {
			_, err := WorktreeChangedPaths(ctx, dir)
			return err
		}},
		{name: "TrackedChanges", read: func(ctx context.Context, dir string) error {
			_, err := TrackedChanges(ctx, dir)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, index := staleIndexRepo(t)
			before := readIndex(t, index)
			if err := tc.read(context.Background(), repo.Dir); err != nil {
				t.Fatalf("read: %v", err)
			}
			rewritten := !bytes.Equal(before, readIndex(t, index))
			if rewritten != tc.rewrite {
				t.Fatalf("index rewritten = %v, want %v", rewritten, tc.rewrite)
			}
		})
	}
}
