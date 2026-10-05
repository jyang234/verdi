package pathcanon

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCanonical is Canonical's table: a path through a symbolic link, an
// existing one or one whose tail is missing, resolves to one spelling; a
// relative path resolves against the working directory; the root and the
// empty path keep their meaning; and a path with no existing ancestor but
// the root keeps its missing tail.
func TestCanonical(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, "present"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wd, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, in, want string
	}{
		{"an existing path resolves itself", real, real},
		{"a symlinked ancestor resolves to its target", filepath.Join(link, "present"), filepath.Join(real, "present")},
		{"a missing tail is kept under its resolved ancestor", filepath.Join(link, "gone", "worktrees", "two-b"), filepath.Join(real, "gone", "worktrees", "two-b")},
		{"an unclean path is cleaned before it resolves", link + "/present/../gone/", filepath.Join(real, "gone")},
		{"a relative path resolves against the working directory", filepath.Join("no-such-dir", "x"), filepath.Join(wd, "no-such-dir", "x")},
		{"the root is itself", string(filepath.Separator), string(filepath.Separator)},
		{"the empty path is the working directory", "", wd},
		{"a path with no existing ancestor but the root keeps its tail", "/no-such-root-dir-pathcanon/a/b", "/no-such-root-dir-pathcanon/a/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Canonical(tt.in); got != tt.want {
				t.Fatalf("Canonical(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestCanonical_DoesNotResolveAnUnreadableTail is Canonical's negative
// path: a component it cannot resolve (a dangling symbolic link) stops
// resolution there, and the path keeps that component's spelling beneath
// its resolved ancestor rather than failing or guessing a target.
func TestCanonical_DoesNotResolveAnUnreadableTail(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(real, "dangling")
	if err := os.Symlink(filepath.Join(real, "nowhere"), dangling); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dangling, "x")
	if got := Canonical(in); got != in {
		t.Fatalf("Canonical(%q) = %q, want it unchanged beneath its resolved ancestor", in, got)
	}
}
