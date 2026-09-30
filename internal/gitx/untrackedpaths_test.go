package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestUntrackedPaths is UntrackedPaths' table: which paths git reports as
// untracked, byte-exact. The unusual names are the reason for -z: without
// it git quotes a name holding a non-ASCII byte, a double quote, or a
// newline (core.quotePath), and a newline inside a name would split one
// path into two.
func TestUntrackedPaths(t *testing.T) {
	cases := []struct {
		name string
		// untracked are written to the working tree and never added.
		untracked []string
		// setup runs after they are written (staging, ignoring, editing).
		setup func(t *testing.T, dir string)
		seed  map[string]string
		want  []string
	}{
		{
			name:      "an untracked file is listed",
			untracked: []string{"untracked.txt"},
			want:      []string{"untracked.txt"},
		},
		{
			name:      "a non-ASCII name is listed verbatim, never quoted",
			untracked: []string{"café.txt"},
			want:      []string{"café.txt"},
		},
		{
			name:      "a name holding a double quote is listed verbatim",
			untracked: []string{`say"hi".txt`},
			want:      []string{`say"hi".txt`},
		},
		{
			name:      "a name holding a newline stays one path",
			untracked: []string{"two\nlines.txt"},
			want:      []string{"two\nlines.txt"},
		},
		{
			name:      "a name in a subdirectory is repository-relative with forward slashes",
			untracked: []string{"dir/nested.txt"},
			want:      []string{"dir/nested.txt"},
		},
		{
			name:      "a staged new file is not untracked",
			untracked: []string{"untracked.txt", "staged-new.txt"},
			setup: func(t *testing.T, dir string) {
				if err := AddPaths(context.Background(), dir, "staged-new.txt"); err != nil {
					t.Fatalf("AddPaths: %v", err)
				}
			},
			want: []string{"untracked.txt"},
		},
		{
			name: "an unstaged edit to a tracked file is not untracked",
			setup: func(t *testing.T, dir string) {
				writeUntrackedFixture(t, dir, "a.txt", "unstaged edit\n")
			},
			want: nil,
		},
		{
			name:      "an ignored file is not listed",
			seed:      map[string]string{".gitignore": "*.log\n"},
			untracked: []string{"ignored.log", "kept.txt"},
			want:      []string{"kept.txt"},
		},
		{
			name: "a clean tree answers nil",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seed := tc.seed
			if seed == nil {
				seed = map[string]string{"a.txt": "hello\n", "dir/b.txt": "world\n"}
			}
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: seed, Message: "seed"}})
			for _, name := range tc.untracked {
				writeUntrackedFixture(t, repo.Dir, name, "content\n")
			}
			if tc.setup != nil {
				tc.setup(t, repo.Dir)
			}

			got, err := UntrackedPaths(context.Background(), repo.Dir)
			if err != nil {
				t.Fatalf("UntrackedPaths: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("UntrackedPaths = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// TestUntrackedPaths_Negative proves a directory outside any git repository
// is a surfaced error, never a silent empty result.
func TestUntrackedPaths_Negative(t *testing.T) {
	notARepo := t.TempDir()
	if _, err := UntrackedPaths(context.Background(), notARepo); err == nil {
		t.Fatal("UntrackedPaths outside a repo: want error, got nil")
	}
}

// writeUntrackedFixture writes content at dir/rel (a slash path), creating
// parent directories. A name the filesystem refuses skips the case rather
// than failing it: the newline case needs a filesystem that allows one.
func writeUntrackedFixture(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Skipf("this filesystem refuses the name %q: %v", rel, err)
	}
}
