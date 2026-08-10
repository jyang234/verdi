package policyauthority

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoad_AdmitsProjectionManifestsWithoutReadingThem pins the DC-1
// boundary the projections grammar row adds: a generated manifest under
// .verdi/policy/projections/ is admitted by the walker and NEVER decoded
// as authority — even unreadable garbage bytes there cannot fail Load,
// because a projection is an output of the constitution, not an input
// to it. (internal/instructionprojection owns verifying manifests
// against the resolved authority.)
func TestLoad_AdmitsProjectionManifestsWithoutReadingThem(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())

	projDir := filepath.Join(root, ".verdi", "policy", "projections")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "codex.json"), []byte("{not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() with a garbage projection manifest: %v", err)
	}
	if _, err := Resolve(s); err != nil {
		t.Fatalf("Resolve() with a garbage projection manifest present: %v", err)
	}

	// A non-kebab manifest name still fails closed through the grammar.
	if err := os.WriteFile(filepath.Join(projDir, "Bad Name.json"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("Load() accepted a projections entry outside the grammar")
	}
}
