package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// renameRepo mirrors fixturegit.Repo's shape (Dir/Head/Heads) but is built
// with raw git commands rather than fixturegit.Build, because fixturegit
// layers are additive-only (a layer's files are written, never removed —
// see its doc comment) and so cannot express a real git rename: `git mv`
// requires the old path to actually disappear on disk between commits.
type renameRepo struct {
	Dir   string
	Heads []string
	Head  string
}

// buildRenameRepo builds a two-commit repo: layer 1 creates a spec file
// under specs/active/ plus an unrelated file; layer 2 performs a real `git
// mv` of the spec file into specs/archive/ (a pure, content-identical
// rename — the only diff shape VL-010 permits on a frozen file) alongside
// an unrelated content edit to the other file, so the fixture exercises
// both DiffEntry shapes DiffNameStatus must distinguish.
func buildRenameRepo(t *testing.T) *renameRepo {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		runFor(t, dir, args...)
	}
	run("init", "-q", "--initial-branch=main")
	run("config", "user.email", "fixture@verdi.invalid")
	run("config", "user.name", "Verdi Fixture")
	run("config", "commit.gpgsign", "false")

	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("specs/active/foo/spec.md", "content\n")
	mustWrite("other.txt", "x\n")
	run("add", "-A")
	run("commit", "-q", "--no-verify", "-m", "layer 1")
	head1 := strings.TrimSpace(runForOutput(t, dir, "rev-parse", "HEAD"))

	// A plain remove + write (rather than shelling out to `git mv`) is all
	// git's own rename detection needs: `git diff -M` detects renames by
	// comparing a diff's deletions against its additions for content
	// similarity, independent of how the change was staged.
	if err := os.Remove(filepath.Join(dir, "specs", "active", "foo", "spec.md")); err != nil {
		t.Fatalf("removing old path: %v", err)
	}
	mustWrite("specs/archive/foo/spec.md", "content\n")
	mustWrite("other.txt", "x changed\n")
	run("add", "-A")
	run("commit", "-q", "--no-verify", "-m", "layer 2: archive move + unrelated edit")
	head2 := strings.TrimSpace(runForOutput(t, dir, "rev-parse", "HEAD"))

	return &renameRepo{Dir: dir, Heads: []string{head1, head2}, Head: head2}
}

func TestDiffNameStatus_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	entries, err := DiffNameStatus(ctx, repo.Dir, repo.Heads[0], repo.Head)
	if err != nil {
		t.Fatalf("DiffNameStatus: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("DiffNameStatus = %+v, want exactly 1 entry (a.txt modified)", entries)
	}
	e := entries[0]
	if e.Status != "M" || e.Path != "a.txt" {
		t.Fatalf("entry = %+v, want Status=M Path=a.txt", e)
	}
}

func TestDiffNameStatus_RenameDetection(t *testing.T) {
	repo := buildRenameRepo(t)
	ctx := context.Background()

	entries, err := DiffNameStatus(ctx, repo.Dir, repo.Heads[0], repo.Head)
	if err != nil {
		t.Fatalf("DiffNameStatus: %v", err)
	}

	var sawRename, sawModify bool
	for _, e := range entries {
		switch {
		case e.Status == "R" && e.OldPath == "specs/active/foo/spec.md" && e.Path == "specs/archive/foo/spec.md":
			sawRename = true
			if !e.Pure() {
				t.Fatalf("rename entry %+v: Pure() = false, want true (identical content)", e)
			}
		case e.Status == "M" && e.Path == "other.txt":
			sawModify = true
			if e.Pure() {
				t.Fatal("modify entry: Pure() = true, want false")
			}
		}
	}
	if !sawRename {
		t.Fatalf("no pure-rename entry found in %+v", entries)
	}
	if !sawModify {
		t.Fatalf("no modify entry found in %+v", entries)
	}
}

func TestDiffNameStatus_NoChanges(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	entries, err := DiffNameStatus(ctx, repo.Dir, repo.Head, repo.Head)
	if err != nil {
		t.Fatalf("DiffNameStatus(HEAD, HEAD): %v", err)
	}
	if entries != nil {
		t.Fatalf("DiffNameStatus(HEAD, HEAD) = %+v, want nil (no changes)", entries)
	}
}

func TestDiffNameStatus_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if _, err := DiffNameStatus(ctx, repo.Dir, "not-a-real-rev", repo.Head); err == nil {
		t.Fatal("DiffNameStatus(bogus base): want error, got nil")
	}
}
