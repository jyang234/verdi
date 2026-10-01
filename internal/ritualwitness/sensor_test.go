package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestFileState(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("content\n"), mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	plainFile := write("plain", 0o644)
	execFile := write("exec", 0o755)
	link := filepath.Join(dir, "link")
	if err := os.Symlink("plain", link); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		path    string
		want    FileState
		present bool
		wantErr bool
	}{
		{"a regular file", plainFile, FileState{Kind: KindFile, Digest: digest([]byte("content\n"))}, true, false},
		{"an executable file", execFile, FileState{Kind: KindFile, Exec: true, Digest: digest([]byte("content\n"))}, true, false},
		{"a symlink is read as its target, not followed", link, FileState{Kind: KindSymlink, Digest: digest([]byte("plain"))}, true, false},
		{"a directory is not a file", sub, FileState{}, false, false},
		{"a missing path is absent", filepath.Join(dir, "missing"), FileState{}, false, false},
		{"a path through a file is an error, never a deletion", filepath.Join(plainFile, "child"), FileState{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, present, err := fileState(tt.path)
			if (err != nil) != tt.wantErr || present != tt.present || got != tt.want {
				t.Fatalf("fileState = (%+v, %v, %v), want (%+v, %v, err %v)", got, present, err, tt.want, tt.present, tt.wantErr)
			}
		})
	}
}

// TestCapture_SeedStates pins the two seeded states exactly (story ac-1):
// both carry the dirty tracked file and the untracked file; only the full
// state carries the pre-staged foreign entry; both register one linked
// worktree, ignored by the working-tree sensor.
func TestCapture_SeedStates(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		state      SeedState
		wantStatus []StatusEntry
		wantFiles  []string
	}{
		{SeedClean,
			[]StatusEntry{{X: ' ', Y: 'M', Path: TrackedFile}, {X: '?', Y: '?', Path: UntrackedFile}},
			[]string{".gitignore", "owned/keep.txt", TrackedFile, UntrackedFile}},
		{SeedFull,
			[]StatusEntry{{X: 'A', Y: ' ', Path: ForeignFile}, {X: ' ', Y: 'M', Path: TrackedFile}, {X: '?', Y: '?', Path: UntrackedFile}},
			[]string{".gitignore", ForeignFile, "owned/keep.txt", TrackedFile, UntrackedFile}},
	} {
		t.Run(tt.state.String(), func(t *testing.T) {
			fx := Build(t, ctx, tt.state)
			s, err := Capture(ctx, fx.Dir, fx.Bare)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s.Status, tt.wantStatus) {
				t.Errorf("status = %+v, want %+v", s.Status, tt.wantStatus)
			}
			var files []string
			for p := range s.Files {
				files = append(files, p)
			}
			sort.Strings(files)
			if !reflect.DeepEqual(files, tt.wantFiles) {
				t.Errorf("files = %v, want %v", files, tt.wantFiles)
			}
			if s.Root != canonicalPath("", fx.Dir) || s.StoreRoot != s.Root || s.Prefix != "" {
				t.Errorf("roots = (%s, %s, %q), want the canonical fixture directory and no prefix", s.Root, s.StoreRoot, s.Prefix)
			}
			if s.Head != (Head{Ref: "refs/heads/main", Commit: fx.BaseCommit}) {
				t.Errorf("HEAD = %+v", s.Head)
			}
			wantRefs := map[string]Ref{
				"refs/heads/main":          {Object: fx.BaseCommit},
				"refs/heads/side":          {Object: fx.BaseCommit},
				"refs/remotes/origin/main": {Object: fx.BaseCommit},
				"refs/remotes/origin/HEAD": {Object: fx.BaseCommit, Symref: "refs/remotes/origin/main"},
			}
			if !reflect.DeepEqual(s.Refs, wantRefs) {
				t.Errorf("refs = %+v", s.Refs)
			}
			if !reflect.DeepEqual(s.RemoteRefs, map[string]Ref{"refs/heads/main": {Object: fx.BaseCommit}}) {
				t.Errorf("remote refs = %+v", s.RemoteRefs)
			}
			if len(s.Commits) != 1 || len(s.Commits[fx.BaseCommit].Files) != 3 {
				t.Errorf("commits = %+v, want the seed commit with its three files", s.Commits)
			}
			if len(s.Worktrees) != 1 || s.Worktrees[0].Path != fx.Registered || !s.Worktrees[0].Present ||
				s.Worktrees[0].Head != (Head{Detached: true, Commit: fx.BaseCommit}) || len(s.Worktrees[0].Index) != 3 {
				t.Errorf("linked worktrees = %+v, want the registered one", s.Worktrees)
			}
			if s.Config["user.name"] == nil || s.Config["remote.origin.url"] == nil {
				t.Errorf("config = %+v", s.Config)
			}
			_, staged := s.HeadTree[ForeignFile]
			if len(s.HeadTree) != 3 || staged {
				t.Errorf("HEAD tree = %+v", s.HeadTree)
			}
			wantIndex := 3
			if tt.state == SeedFull {
				wantIndex = 4
			}
			if len(s.Index) != wantIndex {
				t.Errorf("index = %+v, want %d entries", s.Index, wantIndex)
			}
		})
	}
}

func TestCapture_StoreBelowTheRoot(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	store := filepath.Join(fx.Dir, "store")
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Capture(ctx, store, fx.Bare)
	if err != nil {
		t.Fatal(err)
	}
	if s.Prefix != "store/" || s.StoreRoot != canonicalPath("", store) || s.Root != canonicalPath("", fx.Dir) {
		t.Fatalf("Capture of a store below the root = (%s, %s, %q)", s.Root, s.StoreRoot, s.Prefix)
	}
}

// TestCapture_NestedWorktreeIsADirectoryEntry: a linked worktree inside
// the tree at an unignored path appears to git as a "dir/" entry, which
// the working-tree sensor skips rather than failing on (A P5c).
func TestCapture_NestedWorktreeIsADirectoryEntry(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	runGitFixture(t, ctx, fx.Dir, "worktree", "add", "--quiet", "--detach", "nested/wt", "HEAD")
	s, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("Capture with a nested linked worktree: %v", err)
	}
	for p := range s.Files {
		if strings.HasPrefix(p, "nested/") {
			t.Errorf("the working-tree sensor read %s inside a nested worktree", p)
		}
	}
	if len(s.Worktrees) != 2 {
		t.Errorf("linked worktrees = %+v, want the registered and the nested one", s.Worktrees)
	}
}

func TestCapture_Errors(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	tests := []struct {
		name, dir, bare string
	}{
		{"a directory outside any repository", t.TempDir(), fx.Bare},
		{"a missing remote", fx.Dir, filepath.Join(t.TempDir(), "missing")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Capture(ctx, tt.dir, tt.bare); err == nil {
				t.Fatal("Capture succeeded, want an error")
			}
		})
	}
}
