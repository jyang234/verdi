package execworkspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLstatType_Absent(t *testing.T) {
	dir := t.TempDir()
	kind, err := LstatType(filepath.Join(dir, "nope"))
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathAbsent {
		t.Fatalf("kind = %v, want PathAbsent", kind)
	}
}

func TestLstatType_RegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	kind, err := LstatType(path)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathRegular {
		t.Fatalf("kind = %v, want PathRegular", kind)
	}
}

func TestLstatType_Directory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	kind, err := LstatType(sub)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathDir {
		t.Fatalf("kind = %v, want PathDir", kind)
	}
}

func TestLstatType_Symlink_NeverFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	kind, err := LstatType(link)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathSymlink {
		t.Fatalf("kind = %v, want PathSymlink (a following stat would report PathDir here) ", kind)
	}
}

func TestLstatType_SymlinkToRegularFile_NeverFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target-file")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	kind, err := LstatType(link)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathSymlink {
		t.Fatalf("kind = %v, want PathSymlink (a following stat would report PathRegular here)", kind)
	}
}

func TestLstatType_DanglingSymlink_StillReportsSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "does-not-exist"), link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	kind, err := LstatType(link)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathSymlink {
		t.Fatalf("kind = %v, want PathSymlink even when the link target is absent", kind)
	}
}

func TestLstatType_LstatFailure_NotReadAsAbsence(t *testing.T) {
	// Constructing a path through a non-directory component (a regular
	// file used as an intermediate "directory") makes the OS return
	// ENOTDIR, a real lstat failure that must never be read as absence.
	dir := t.TempDir()
	notADir := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	bogus := filepath.Join(notADir, "child")

	kind, err := LstatType(bogus)
	if err == nil {
		t.Fatalf("LstatType: want error for lstat failure through a non-directory component, got nil (kind=%v)", kind)
	}
	if kind == PathAbsent {
		t.Fatalf("LstatType: an lstat failure must never be reported as PathAbsent")
	}
}

func TestLstatType_KindsAreDistinct(t *testing.T) {
	seen := map[PathKind]bool{}
	for _, k := range []PathKind{PathUnknown, PathAbsent, PathRegular, PathDir, PathSymlink, PathOther} {
		if seen[k] {
			t.Fatalf("duplicate PathKind value %v", k)
		}
		seen[k] = true
	}
}
