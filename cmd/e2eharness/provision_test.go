package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// TestCopyTreePresent proves a present source tree copies through.
func TestCopyTreePresent(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("there"), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatalf("reading copied file: %v", err)
	}
	if string(got) != "hi" {
		t.Fatalf("a.txt = %q, want %q", got, "hi")
	}
	got, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatalf("reading copied nested file: %v", err)
	}
	if string(got) != "there" {
		t.Fatalf("sub/b.txt = %q, want %q", got, "there")
	}
}

// TestCopyTreeAbsent proves a missing source is tolerated: no error, no
// destination created.
func TestCopyTreeAbsent(t *testing.T) {
	src := filepath.Join(t.TempDir(), "does-not-exist")
	dst := filepath.Join(t.TempDir(), "out")

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree on absent src: got %v, want nil", err)
	}
	if _, err := os.Stat(dst); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected dst to remain absent, stat err = %v", err)
	}
}

// TestCopyTreeUnreadable proves a stat failure other than "not exist" (here:
// the parent directory denying search/traversal permission) returns a
// wrapped error instead of being swallowed alongside the absent case.
//
// Skips under root: root bypasses the permission bits this test relies on.
func TestCopyTreeUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not block stat")
	}

	parent := t.TempDir()
	src := filepath.Join(parent, "locked")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(t.TempDir(), "out")
	err := copyTree(src, dst)
	if err == nil {
		t.Fatal("expected an error for an unreadable source, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected a non-NotExist error, got %v", err)
	}
}
