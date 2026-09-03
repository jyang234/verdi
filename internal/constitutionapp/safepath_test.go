package constitutionapp

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckNoSymlinkedComponent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "policy", "overlays"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "policy", "overlays", "present.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".verdi", "policy", "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, ".verdi", "policy", "overlays", "present.md"), filepath.Join(root, ".verdi", "policy", "overlays", "linked.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "policy", "notadir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		rel  string
		ok   bool
	}{
		{"existing regular destination", ".verdi/policy/overlays/present.md", true},
		{"absent destination under real directories", ".verdi/policy/overlays/absent.md", true},
		{"absent intermediate directory ends the walk", ".verdi/policy/nothere/absent.md", true},
		{"symlinked intermediate directory", ".verdi/policy/linked/escape.md", false},
		{"symlinked final component", ".verdi/policy/overlays/linked.md", false},
		{"intermediate component is a regular file", ".verdi/policy/notadir/x.md", false},
		{"destination is a directory", ".verdi/policy/overlays", false},
		{"empty path", "", false},
		{"absolute path", "/etc/passwd", false},
		{"dot-dot escape", ".verdi/policy/../../escape.md", false},
		{"empty component", ".verdi//policy/x.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkNoSymlinkedComponent(root, tc.rel)
			if tc.ok && err != nil {
				t.Fatalf("checkNoSymlinkedComponent(%q) = %v, want nil", tc.rel, err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatalf("checkNoSymlinkedComponent(%q) = nil, want a refusal", tc.rel)
				}
				if !errors.Is(err, errUnsafePathComponent) {
					t.Fatalf("refusal is not classified as an unsafe path: %v", err)
				}
			}
		})
	}
}

func TestEnsureDirectoryChain(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "policy"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root, ".verdi", "policy", "linked")); err != nil {
		t.Fatal(err)
	}

	// Happy path: the missing components are created as real directories.
	if err := ensureDirectoryChain(root, ".verdi/policy/overlays"); err != nil {
		t.Fatalf("ensureDirectoryChain: %v", err)
	}
	info, err := os.Lstat(filepath.Join(root, ".verdi", "policy", "overlays"))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected a real directory, got info=%v err=%v", info, err)
	}
	// Idempotent over an already-real chain.
	if err := ensureDirectoryChain(root, ".verdi/policy/overlays"); err != nil {
		t.Fatalf("second ensureDirectoryChain: %v", err)
	}

	negatives := []string{
		".verdi/policy/linked",          // the chain itself ends at a symlink
		".verdi/policy/linked/deeper",   // a symlinked ancestor
		"",                              // not a repository-relative path
		"/absolute",                     // not a repository-relative path
		".verdi/policy/../../escape",    // dot-dot escape
		".verdi/policy/overlays/./here", // dot element
	}
	for _, rel := range negatives {
		t.Run(rel, func(t *testing.T) {
			err := ensureDirectoryChain(root, rel)
			if err == nil {
				t.Fatalf("ensureDirectoryChain(%q) = nil, want a refusal", rel)
			}
			if !errors.Is(err, errUnsafePathComponent) {
				t.Fatalf("refusal is not classified as an unsafe path: %v", err)
			}
		})
	}
}

func TestReadRegularFile(t *testing.T) {
	root := t.TempDir()
	present := filepath.Join(root, "present.txt")
	if err := os.WriteFile(present, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(root, "linked.txt")
	if err := os.Symlink(present, linked); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, "dir")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}

	data, exists, err := readRegularFile(present)
	if err != nil || !exists || string(data) != "content" {
		t.Fatalf("readRegularFile(present) = %q, %v, %v", data, exists, err)
	}

	data, exists, err = readRegularFile(filepath.Join(root, "absent.txt"))
	if err != nil || exists || data != nil {
		t.Fatalf("readRegularFile(absent) = %q, %v, %v — a clean absence is not an error", data, exists, err)
	}

	for _, path := range []string{linked, directory} {
		if _, _, err := readRegularFile(path); err == nil || !errors.Is(err, errUnsafePathComponent) {
			t.Fatalf("readRegularFile(%q) = %v, want an unsafe-path refusal", path, err)
		}
	}
}
