package store

import (
	"os"
	"path/filepath"
	"testing"
)

// buildStoreTree creates root/.verdi/verdi.yaml and root/a/b/c (a nested
// directory with no manifest of its own) inside a fresh temp dir, and
// returns root and the nested leaf directory.
func buildStoreTree(t *testing.T) (root, nested string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
		t.Fatalf("write verdi.yaml: %v", err)
	}
	nested = filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	return root, nested
}

func TestFindRoot_Happy(t *testing.T) {
	root, nested := buildStoreTree(t)

	// From the root itself.
	got, err := FindRoot(root)
	if err != nil {
		t.Fatalf("FindRoot(root): %v", err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != mustEvalSymlinks(t, root) {
		t.Fatalf("FindRoot(root) = %q, want %q", got, root)
	}

	// From a deeply nested descendant.
	got, err = FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot(nested): %v", err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != mustEvalSymlinks(t, root) {
		t.Fatalf("FindRoot(nested) = %q, want %q", got, root)
	}

	// From a file path inside the tree (not a directory) — still resolves
	// via the file's parent directory.
	filePath := filepath.Join(root, ".verdi", "verdi.yaml")
	got, err = FindRoot(filePath)
	if err != nil {
		t.Fatalf("FindRoot(file path): %v", err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != mustEvalSymlinks(t, root) {
		t.Fatalf("FindRoot(file path) = %q, want %q", got, root)
	}
}

func TestFindRoot_Negative(t *testing.T) {
	if _, err := FindRoot(""); err == nil {
		t.Fatal("FindRoot(\"\"): want error, got nil")
	}

	// A directory tree with no .verdi/verdi.yaml anywhere above it: use
	// t.TempDir() itself, which on most systems is not itself inside a
	// verdi store.
	orphan := t.TempDir()
	nested := filepath.Join(orphan, "x", "y")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := FindRoot(nested); err == nil {
		t.Fatal("FindRoot(orphan tree): want error, got nil")
	}

	if _, err := FindRoot(filepath.Join(orphan, "does-not-exist")); err == nil {
		t.Fatal("FindRoot(nonexistent path): want error, got nil")
	}
}

func TestRootAt_Happy(t *testing.T) {
	root, _ := buildStoreTree(t)
	got, err := RootAt(root)
	if err != nil {
		t.Fatalf("RootAt(root): %v", err)
	}
	if resolved, _ := filepath.EvalSymlinks(got); resolved != mustEvalSymlinks(t, root) {
		t.Fatalf("RootAt(root) = %q, want %q", got, root)
	}
}

func TestRootAt_Negative(t *testing.T) {
	if _, err := RootAt(""); err == nil {
		t.Fatal("RootAt(\"\"): want error, got nil")
	}

	root, nested := buildStoreTree(t)
	_ = root
	// RootAt does NOT walk ancestors — a nested dir without its own
	// .verdi/verdi.yaml must fail even though FindRoot from the same path
	// would succeed.
	if _, err := RootAt(nested); err == nil {
		t.Fatal("RootAt(nested, no ancestor walk): want error, got nil")
	}

	if _, err := RootAt(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("RootAt(nonexistent path): want error, got nil")
	}
}

func mustEvalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", path, err)
	}
	return resolved
}
