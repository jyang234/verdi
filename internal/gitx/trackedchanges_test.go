package gitx

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// trackedSeed is the committed state every TestTrackedChanges case edits.
var trackedSeed = map[string]string{
	"a.txt":     "a\n",
	"b.txt":     "b\n",
	"dir/c.txt": "c\n",
	"café.txt":  "accent\n",
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestTrackedChanges is TrackedChanges' table over real repositories: each
// kind of tracked change git status reports, with both columns, the modes,
// and a rename's source.
func TestTrackedChanges(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, dir string)
		want []TrackedChange
	}{
		{
			name: "a clean tree answers nil",
			edit: func(*testing.T, string) {},
			want: nil,
		},
		{
			name: "an unstaged edit marks only the worktree column",
			edit: func(t *testing.T, dir string) { writeUntrackedFixture(t, dir, "a.txt", "edited\n") },
			want: []TrackedChange{{Path: "a.txt", Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "a staged edit marks only the index column",
			edit: func(t *testing.T, dir string) {
				writeUntrackedFixture(t, dir, "a.txt", "edited\n")
				gitIn(t, dir, "add", "a.txt")
			},
			want: []TrackedChange{{Path: "a.txt", Index: 'M', Worktree: '.', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "a staged edit reverted in the working tree marks both columns",
			edit: func(t *testing.T, dir string) {
				writeUntrackedFixture(t, dir, "a.txt", "edited\n")
				gitIn(t, dir, "add", "a.txt")
				writeUntrackedFixture(t, dir, "a.txt", "a\n")
			},
			want: []TrackedChange{{Path: "a.txt", Index: 'M', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "a mode change keeps the content and reports both modes",
			edit: func(t *testing.T, dir string) {
				if err := os.Chmod(filepath.Join(dir, "a.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: []TrackedChange{{Path: "a.txt", Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100755"}},
		},
		{
			name: "a deletion reports an absent working-tree mode",
			edit: func(t *testing.T, dir string) {
				if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
					t.Fatal(err)
				}
			},
			want: []TrackedChange{{Path: "b.txt", Index: '.', Worktree: 'D', ModeHead: "100644", ModeWorktree: "000000"}},
		},
		{
			name: "a staged rename names its source",
			edit: func(t *testing.T, dir string) { gitIn(t, dir, "mv", "dir/c.txt", "dir/moved.txt") },
			want: []TrackedChange{{Path: "dir/moved.txt", RenamedFrom: "dir/c.txt", Index: 'R', Worktree: '.', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "a non-ASCII name is reported verbatim",
			edit: func(t *testing.T, dir string) { writeUntrackedFixture(t, dir, "café.txt", "edited\n") },
			want: []TrackedChange{{Path: "café.txt", Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "an untracked file is never reported",
			edit: func(t *testing.T, dir string) { writeUntrackedFixture(t, dir, "new.txt", "new\n") },
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: trackedSeed, Message: "seed"}})
			tc.edit(t, repo.Dir)
			got, err := TrackedChanges(context.Background(), repo.Dir)
			if err != nil {
				t.Fatalf("TrackedChanges: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("TrackedChanges = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestTrackedChanges_FromSubdirectoryIsRootRelative proves the paths stay
// repository-root-relative when dir is below the top level.
func TestTrackedChanges_FromSubdirectoryIsRootRelative(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: trackedSeed, Message: "seed"}})
	writeUntrackedFixture(t, repo.Dir, "a.txt", "edited\n")
	got, err := TrackedChanges(context.Background(), filepath.Join(repo.Dir, "dir"))
	if err != nil {
		t.Fatalf("TrackedChanges: %v", err)
	}
	if len(got) != 1 || got[0].Path != "a.txt" {
		t.Fatalf("TrackedChanges from dir/ = %+v, want a.txt, root-relative", got)
	}
}

// TestTrackedChanges_NeverWritesTheIndex proves the query takes no
// optional lock: git status normally refreshes a stale index and writes it
// back, and --no-optional-locks is what keeps this read a read.
func TestTrackedChanges_NeverWritesTheIndex(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: trackedSeed, Message: "seed"}})
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(repo.Dir, "a.txt"), stale, stale); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(repo.Dir, ".git", "index")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TrackedChanges(context.Background(), repo.Dir); err != nil {
		t.Fatalf("TrackedChanges: %v", err)
	}
	after, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("TrackedChanges rewrote .git/index; the status query must take no optional lock")
	}
}

// TestTrackedChanges_Negative proves a directory outside any repository is
// an error, never an empty answer.
func TestTrackedChanges_Negative(t *testing.T) {
	if _, err := TrackedChanges(context.Background(), t.TempDir()); err == nil {
		t.Fatal("TrackedChanges outside a repo: want error, got nil")
	}
}

// TestParseTrackedStatus is the parser's table over canned porcelain v2
// output: every entry shape it accepts, and every malformed or unexpected
// shape it refuses rather than guesses at.
func TestParseTrackedStatus(t *testing.T) {
	const hash = "0123456789abcdef0123456789abcdef01234567"
	ordinary := "1 .M N... 100644 100644 100755 " + hash + " " + hash + " path with spaces.txt"
	renamed := "2 R. N... 100644 100644 100644 " + hash + " " + hash + " R100 new.txt"
	copied := "2 C. N... 100644 100644 100644 " + hash + " " + hash + " C75 copy.txt"
	unmerged := "u UU N... 100644 100644 100644 100644 " + hash + " " + hash + " " + hash + " conflict.txt"
	cases := []struct {
		name    string
		out     string
		want    []TrackedChange
		wantErr bool
	}{
		{name: "empty output", out: "", want: nil},
		{
			name: "an ordinary entry keeps spaces in the path",
			out:  ordinary + "\x00",
			want: []TrackedChange{{Path: "path with spaces.txt", Index: '.', Worktree: 'M', ModeHead: "100644", ModeWorktree: "100755"}},
		},
		{
			name: "a rename names its source",
			out:  renamed + "\x00old.txt\x00",
			want: []TrackedChange{{Path: "new.txt", RenamedFrom: "old.txt", Index: 'R', Worktree: '.', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "a copy leaves its source unnamed",
			out:  copied + "\x00source.txt\x00",
			want: []TrackedChange{{Path: "copy.txt", Index: 'C', Worktree: '.', ModeHead: "100644", ModeWorktree: "100644"}},
		},
		{
			name: "an unmerged entry is marked",
			out:  unmerged + "\x00",
			want: []TrackedChange{{Path: "conflict.txt", Index: 'U', Worktree: 'U', Unmerged: true, ModeWorktree: "100644"}},
		},
		{name: "an untracked entry is refused (the query never asks for one)", out: "? new.txt\x00", wantErr: true},
		{name: "an ignored entry is refused", out: "! ignored.txt\x00", wantErr: true},
		{name: "a header line is refused", out: "# branch.oid " + hash + "\x00", wantErr: true},
		{name: "a truncated ordinary entry", out: "1 .M N... 100644\x00", wantErr: true},
		{name: "a rename with no source field", out: renamed + "\x00", wantErr: true},
		{name: "a three-byte status column", out: "1 .MM N... 100644 100644 100644 " + hash + " " + hash + " a.txt\x00", wantErr: true},
		{name: "a truncated unmerged entry", out: "u UU N... 100644\x00", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTrackedStatus([]byte(tc.out))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseTrackedStatus(%q) = %+v, want an error", tc.out, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTrackedStatus: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseTrackedStatus = %+v, want %+v", got, tc.want)
			}
		})
	}
}
