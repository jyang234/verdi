package policyauthority

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTree materializes files (relative-path -> content) under root,
// creating parent directories as needed.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func TestLoad_ErrNotAdopted(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	_, err := Load(root)
	if !errors.Is(err, ErrNotAdopted) {
		t.Fatalf("Load() error = %v, want errors.Is(err, ErrNotAdopted)", err)
	}
}

// TestLoad_PolicyRootSymlinkRejected proves a present authority-root entry
// never becomes genuine absence and is never followed, whether its target is
// missing or live.
func TestLoad_PolicyRootSymlinkRejected(t *testing.T) {
	tests := []struct {
		name       string
		liveTarget bool
	}{
		{name: "dangling"},
		{name: "live", liveTarget: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
				t.Fatalf("mkdir .verdi: %v", err)
			}
			target := filepath.Join(root, "policy-target")
			if tt.liveTarget {
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatalf("mkdir policy target: %v", err)
				}
			}
			if err := os.Symlink(target, filepath.Join(root, ".verdi", "policy")); err != nil {
				t.Skipf("symlinks unavailable on this platform: %v", err)
			}

			_, err := Load(root)
			if err == nil {
				t.Fatal("Load() followed or accepted a symlinked policy root, want an operational error")
			}
			if errors.Is(err, ErrNotAdopted) {
				t.Fatalf("Load() error = %v, present symlink root must not satisfy ErrNotAdopted", err)
			}
			if !strings.Contains(err.Error(), ".verdi/policy") || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("Load() error = %v, want a symlink error naming .verdi/policy", err)
			}
		})
	}
}

func TestLoad_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if s.Constitution == nil {
		t.Fatal("Load() Store.Constitution is nil")
	}
	if _, ok := s.Policies["policy/go-toolchain"]; !ok {
		t.Fatal("Load() Store.Policies missing policy/go-toolchain")
	}
	if _, ok := s.Profiles["solo-default"]; !ok {
		t.Fatal("Load() Store.Profiles missing solo-default")
	}
}
