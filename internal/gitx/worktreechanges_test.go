package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestWorktreeChangedPaths_Happy(t *testing.T) {
	ctx := context.Background()
	repo := buildRepo(t)

	// Staged modification.
	if err := os.WriteFile(filepath.Join(repo.Dir, "a.txt"), []byte("staged edit\n"), 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	if err := AddPaths(ctx, repo.Dir, "a.txt"); err != nil {
		t.Fatalf("AddPaths: %v", err)
	}
	// Unstaged worktree modification.
	if err := os.WriteFile(filepath.Join(repo.Dir, "dir/b.txt"), []byte("unstaged edit\n"), 0o644); err != nil {
		t.Fatalf("writing dir/b.txt: %v", err)
	}
	// Untracked file.
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("writing untracked.txt: %v", err)
	}
	// Untracked file with unusual bytes in its name.
	weird := "weird space\ttab.txt"
	if err := os.WriteFile(filepath.Join(repo.Dir, weird), []byte("weird\n"), 0o644); err != nil {
		t.Fatalf("writing %q: %v", weird, err)
	}

	got, err := WorktreeChangedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}
	want := []string{"a.txt", "dir/b.txt", "untracked.txt", weird}
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	if !reflect.DeepEqual(got, sortedWant) {
		t.Fatalf("WorktreeChangedPaths = %#v, want %#v", got, sortedWant)
	}
}

func TestWorktreeChangedPaths_CleanIsEmpty(t *testing.T) {
	repo := buildRepo(t)
	got, err := WorktreeChangedPaths(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("WorktreeChangedPaths(clean) = %#v, want none", got)
	}
}

func TestWorktreeChangedPaths_Deletion(t *testing.T) {
	ctx := context.Background()
	repo := buildRepo(t)

	if err := os.Remove(filepath.Join(repo.Dir, "a.txt")); err != nil {
		t.Fatalf("removing a.txt: %v", err)
	}

	got, err := WorktreeChangedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}
	want := []string{"a.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WorktreeChangedPaths(deletion) = %#v, want %#v", got, want)
	}
}

// TestWorktreeChangedPaths_NamesBothSidesOfAWorktreeRename proves a rename
// staged with `git mv` — index status 'R' — surfaces both its destination
// and its original path, the same completeness StagedPaths guarantees for
// renames.
func TestWorktreeChangedPaths_NamesBothSidesOfAWorktreeRename(t *testing.T) {
	ctx := context.Background()
	repo := buildRepo(t)
	runGitForTest(t, repo.Dir, "mv", "a.txt", "renamed.txt")

	got, err := WorktreeChangedPaths(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("WorktreeChangedPaths: %v", err)
	}
	want := []string{"a.txt", "renamed.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WorktreeChangedPaths(staged rename) = %#v, want both sides %#v", got, want)
	}
}

func TestWorktreeChangedPaths_Negative(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := WorktreeChangedPaths(ctx, notARepo); err == nil {
		t.Fatal("WorktreeChangedPaths outside a repository: want error, got nil")
	}
}

func TestParseWorktreeStatus(t *testing.T) {
	nul := "\x00"
	cases := []struct {
		name    string
		out     string
		want    []string
		wantErr bool
	}{
		{name: "empty output"},
		{name: "trailing NUL only", out: nul},
		{name: "staged modification", out: "M  a.txt" + nul, want: []string{"a.txt"}},
		{name: "unstaged worktree modification", out: " M a.txt" + nul, want: []string{"a.txt"}},
		{name: "untracked file", out: "?? scratch.txt" + nul, want: []string{"scratch.txt"}},
		{name: "ignored is skipped", out: "!! build/out" + nul},
		{name: "staged deletion", out: "D  dir/b.txt" + nul, want: []string{"dir/b.txt"}},
		{name: "unmerged conflict", out: "UU merged.txt" + nul, want: []string{"merged.txt"}},
		{
			name: "staged rename contributes both sides",
			out:  "R  new.txt" + nul + "old.txt" + nul,
			want: []string{"new.txt", "old.txt"},
		},
		{
			name: "worktree rename contributes both sides",
			out:  " R new.txt" + nul + "old.txt" + nul,
			want: []string{"new.txt", "old.txt"},
		},
		{
			name: "copy contributes only its destination",
			out:  "C  copy.txt" + nul + "source.txt" + nul,
			want: []string{"copy.txt"},
		},
		{
			name: "paths carrying newlines and non-ASCII bytes survive -z",
			out:  "A  café-\nnl.txt" + nul + "M  plain.txt" + nul,
			want: []string{"café-\nnl.txt", "plain.txt"},
		},
		{name: "entry too short to carry a path", out: "M  " + nul, wantErr: true},
		{name: "entry missing the status separator", out: "MXa.txt" + nul, wantErr: true},
		{name: "rename with no original-path field", out: "R  new.txt" + nul, wantErr: true},
		{name: "rename with an empty original-path field", out: "R  new.txt" + nul + nul, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWorktreeStatus([]byte(tc.out))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseWorktreeStatus(%q) = %#v, nil; want an error", tc.out, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseWorktreeStatus(%q): %v", tc.out, err)
			}
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseWorktreeStatus(%q) = %#v, want %#v", tc.out, got, tc.want)
			}
		})
	}
}

func TestDedupeSorted(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "nil", in: nil, want: nil},
		{name: "no duplicates", in: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
		{name: "adjacent duplicates", in: []string{"a", "a", "b", "b", "b", "c"}, want: []string{"a", "b", "c"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dedupeSorted(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("dedupeSorted(%#v) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}
