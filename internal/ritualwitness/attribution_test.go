package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
)

// TestPathspecMatches is SI-359 (11): a relative pathspec resolves against
// the call's own directory, while an absolute one matches by its
// canonical path relative to the calling worktree's top level, so the
// same directory spelt through a symbolic link (/var against /private/var
// on macOS) still matches, and a spec outside the worktree attributes
// nothing. Only the spec's leading part up to the top level is resolved,
// as git's abspath_part_inside_repo does (SI-359 (11) as amended; R5c1
// review R5C1R-1): a symbolic link inside the worktree (alias -> real, or
// self -> .) keeps its own spelling, so it never credits the path it
// points at.
func TestPathspecMatches(t *testing.T) {
	raw := t.TempDir()
	root := canonicalPath("", raw)
	for _, d := range []string{"dir", "sub", "real"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "real", "f"), []byte("f\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"alias": "real", "self": "."} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	outside := canonicalPath("", t.TempDir())
	sub := filepath.Join(root, "sub")
	tests := []struct {
		name     string
		dir      string
		spec     string
		repoPath string
		want     bool
	}{
		{"a relative spec from the top level", root, "a.txt", "a.txt", true},
		{"a relative spec resolves against the call's directory", sub, "a.txt", "sub/a.txt", true},
		{"a relative spec from a subdirectory names no top-level path", sub, "a.txt", "a.txt", false},
		{"an absolute spec in its canonical spelling", root, filepath.Join(root, "a.txt"), "a.txt", true},
		{"an absolute spec names a directory above the path", root, filepath.Join(root, "dir"), "dir/b.txt", true},
		{"an absolute spec naming the top level matches every path", root, root, "dir/b.txt", true},
		{"an absolute spec ignores the call's subdirectory", sub, filepath.Join(root, "a.txt"), "a.txt", true},
		{"an absolute spec is not joined onto the call's subdirectory", sub, filepath.Join(root, "a.txt"), "sub/a.txt", false},
		{"an absolute spec through a symbolic link to the worktree", root, filepath.Join(link, "dir", "b.txt"), "dir/b.txt", true},
		{"an absolute spec in the OS temporary directory's own spelling", root, filepath.Join(raw, "a.txt"), "a.txt", true},
		{"an absolute glob spec", root, filepath.Join(root, "dir", "*.txt"), "dir/b.txt", true},
		{"an absolute glob spec not matching", root, filepath.Join(root, "dir", "*.md"), "dir/b.txt", false},
		{"an absolute spec naming another path", root, filepath.Join(root, "a.txt"), "dir/b.txt", false},
		{"an absolute spec outside the worktree", root, filepath.Join(outside, "a.txt"), "a.txt", false},
		{"an absolute spec naming the worktree's parent", root, filepath.Dir(root), "a.txt", false},
		{"an absolute spec in a sibling sharing the worktree's name as a prefix", root, root + "-other/a.txt", "a.txt", false},
		{"an absolute glob spec outside the worktree", root, filepath.Join(outside, "*"), "a.txt", false},
		{"an absolute spec naming a tracked symbolic link names the link", root, filepath.Join(root, "alias"), "alias", true},
		{"an absolute spec naming a tracked symbolic link never names its target", root, filepath.Join(root, "alias"), "real/f", false},
		{"an absolute spec beneath a tracked symbolic link keeps the link's spelling", root, filepath.Join(root, "alias", "f"), "real/f", false},
		{"an absolute spec through a link to the top level inside it keeps the link", root, filepath.Join(root, "self", "a.txt"), "self/a.txt", true},
		{"an absolute spec through a link to the top level inside it names no top-level path", root, filepath.Join(root, "self", "a.txt"), "a.txt", false},
		{"an absolute spec through an outside link keeps the in-worktree link", root, filepath.Join(link, "alias"), "real/f", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := loggedCall{Dir: tt.dir, Args: []string{"add", "--", tt.spec}, Kind: primStage}
			if got := pathspecMatches(root, c, []string{tt.spec}, tt.repoPath); got != tt.want {
				t.Fatalf("pathspecMatches(%s, dir %s, %q, %q) = %v, want %v", root, tt.dir, tt.spec, tt.repoPath, got, tt.want)
			}
		})
	}
}

// TestHarness_AbsolutePathspecsAttribute drives AddPaths and
// CreateCommitPaths with absolute paths spelt from the fixture directory as
// the driver hands it over, which on macOS is /var/… against the
// snapshot's /private/var/… root: the index entry and the commit's path
// attribute through the log, as every relative spelling does (SI-359
// (11)).
func TestHarness_AbsolutePathspecsAttribute(t *testing.T) {
	ritual := steps(newBranch("ritual/abs"), writeFile("owned/a.txt", "a\n"),
		func(ctx context.Context, dir string) error {
			return gitx.AddPaths(ctx, dir, filepath.Join(dir, "owned", "a.txt"))
		},
		func(ctx context.Context, dir string) error {
			_, err := gitx.CreateCommitPaths(ctx, dir, "commit the owned file", filepath.Join(dir, "owned", "a.txt"))
			return err
		})
	for _, state := range both() {
		t.Run(state.String(), func(t *testing.T) {
			res := Run(t, context.Background(), InProcess{Fn: ritual}, scopedDecl(), state)
			want := []Verdict{
				v("refs_create", Within, "refs/heads/ritual/abs created"),
				v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/abs"),
				v("index", Within, "index entry owned/a.txt added"),
				v("working_tree", Within, "owned/a.txt created"+fileWrite),
				v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/a.txt"),
				v("index_carry", Within, "declares scoped; observed scoped"),
			}
			if diff := verdictDiff(res.Verdicts, want); diff != "" {
				t.Fatal(diff)
			}
			if got := Outcome(res.Verdicts); got != Pass {
				t.Fatalf("Outcome = %s, want pass", got)
			}
		})
	}
}
