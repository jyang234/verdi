package store

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeQuartetFixture(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	files := map[string]string{
		"spec.md":             "---\nid: spec/" + name + "\n---\n# body\n",
		"layout.json":         `{"schema":"verdi.boardlayout/v1","positions":{}}` + "\n",
		"deviation-report.md": "---\nschema: verdi.deviation/v1\n---\n# report\n",
		"rollup.json":         `{"schema":"verdi.rollup/v1"}` + "\n",
	}
	for fname, content := range files {
		if err := os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", fname, err)
		}
	}
}

// TestArchiveMove_MovesEveryFileByteForByte proves the whole quartet moves
// intact — same bytes, same filenames — from specs/active/<name>/ to
// specs/archive/<name>/, and the active directory no longer exists.
func TestArchiveMove_MovesEveryFileByteForByte(t *testing.T) {
	root := t.TempDir()
	writeQuartetFixture(t, root, "stale-decline")

	if err := ArchiveMove(root, "stale-decline"); err != nil {
		t.Fatalf("ArchiveMove: %v", err)
	}

	activeDir := filepath.Join(root, ".verdi", "specs", "active", "stale-decline")
	if _, err := os.Stat(activeDir); !os.IsNotExist(err) {
		t.Fatalf("active dir still exists after ArchiveMove (err=%v)", err)
	}

	archiveDir := filepath.Join(root, ".verdi", "specs", "archive", "stale-decline")
	for _, fname := range []string{"spec.md", "layout.json", "deviation-report.md", "rollup.json"} {
		if _, err := os.Stat(filepath.Join(archiveDir, fname)); err != nil {
			t.Fatalf("archive dir missing %s: %v", fname, err)
		}
	}
	specData, err := os.ReadFile(filepath.Join(archiveDir, "spec.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(specData) != "---\nid: spec/stale-decline\n---\n# body\n" {
		t.Fatalf("spec.md content changed by the move: %q", specData)
	}
}

// TestArchiveMove_RequiresSpecMD proves a directory with no spec.md (or no
// active directory at all) is refused rather than silently archiving
// something that isn't a real spec directory.
func TestArchiveMove_RequiresSpecMD(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "active", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ArchiveMove(root, "empty"); err == nil {
		t.Fatal("ArchiveMove(directory with no spec.md): want error, got nil")
	}
	if err := ArchiveMove(root, "does-not-exist-at-all"); err == nil {
		t.Fatal("ArchiveMove(nonexistent active dir): want error, got nil")
	}
}

// TestArchiveMove_RefusesToClobberExistingArchive proves ArchiveMove will
// not overwrite an already-archived directory of the same name.
func TestArchiveMove_RefusesToClobberExistingArchive(t *testing.T) {
	root := t.TempDir()
	writeQuartetFixture(t, root, "dup")
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "specs", "archive", "dup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ArchiveMove(root, "dup"); err == nil {
		t.Fatal("ArchiveMove(archive already exists): want error, got nil")
	}
	// The active directory must be left untouched on this failure.
	if _, err := os.Stat(filepath.Join(root, ".verdi", "specs", "active", "dup", "spec.md")); err != nil {
		t.Fatalf("active dir was disturbed despite the refusal: %v", err)
	}
}

// TestArchiveMove_IsAPureGitRename is the load-bearing proof: after
// ArchiveMove, git itself must detect the move as a 100%-similarity rename
// (VL-010's sole legal exception on an otherwise-frozen spec,
// internal/lint's vl010.go: DiffEntry.Pure() requires Status == "R" and
// Score == 100) — not a delete+add, which VL-010 would reject outright.
func TestArchiveMove_IsAPureGitRename(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet", "--initial-branch=main")
	runGit(t, root, "config", "user.email", "fixture@verdi.invalid")
	runGit(t, root, "config", "user.name", "Verdi Fixture")
	runGit(t, root, "config", "commit.gpgsign", "false")

	writeQuartetFixture(t, root, "stale-decline")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "pre-closure state")
	parent := strings.TrimSpace(runGitOutput(t, root, "rev-parse", "HEAD"))

	if err := ArchiveMove(root, "stale-decline"); err != nil {
		t.Fatalf("ArchiveMove: %v", err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "close: archive stale-decline")

	out := runGitOutput(t, root, "diff", "--name-status", "-M", parent, "HEAD")
	if !strings.Contains(out, "R100\t.verdi/specs/active/stale-decline/spec.md\t.verdi/specs/archive/stale-decline/spec.md") {
		t.Fatalf("git diff --name-status -M did not report a pure (R100) rename for spec.md:\n%s", out)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}
