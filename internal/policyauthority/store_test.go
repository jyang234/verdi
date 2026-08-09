package policyauthority

import (
	"errors"
	"os"
	"path/filepath"
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
