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

// TestLoad_DispositionHappyPath proves Load recognizes the dispositions/
// directory, decodes a valid policy-disposition/v1 artifact through
// policyartifact.DecodeDisposition, and keys it by its full kinded id
// (authority-design §8).
func TestLoad_DispositionHappyPath(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/review-no-conflict.md"] = dispositionFile(t, "review-no-conflict")
	root := t.TempDir()
	writeTree(t, root, files)

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	d, ok := s.Dispositions["policy-disposition/review-no-conflict"]
	if !ok {
		t.Fatalf("Load() Store.Dispositions missing policy-disposition/review-no-conflict, got %v", s.Dispositions)
	}
	if d.Name() != "review-no-conflict" {
		t.Fatalf("disposition Name() = %q, want review-no-conflict", d.Name())
	}
	if _, err := d.Digest(); err != nil {
		t.Fatalf("disposition Digest() error: %v", err)
	}
}

// TestLoad_DispositionFilenameStemMismatch mirrors
// TestLoad_FilenameStemMismatch for the new disposition kind: the file's
// declared id must match its own filename stem.
func TestLoad_DispositionFilenameStemMismatch(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/renamed.md"] = dispositionFile(t, "review-no-conflict")
	root := t.TempDir()
	writeTree(t, root, files)

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a filename-stem-mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want stem-mismatch text", err)
	}
}

// TestLoad_DispositionWrongKind proves a well-formed artifact of a
// DIFFERENT kind filed under dispositions/ fails closed on schema/kind
// mismatch, never silently decoded as a disposition.
func TestLoad_DispositionWrongKind(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/legacy-service-go.md"] = files[".verdi/policy/exemptions/legacy-service-go.md"]
	root := t.TempDir()
	writeTree(t, root, files)

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded on a policy-exemption artifact filed under dispositions/, want error")
	}
	// A policy-exemption document's own frontmatter shape (its "witnesses"
	// key) does not decode as a disposition's strict shape at all — the
	// mismatch is caught by the strict decoder itself, one layer before a
	// same-shaped-but-wrong-kind document would reach the schema/kind check.
	if !strings.Contains(err.Error(), "decoding dispositions/legacy-service-go.md") {
		t.Fatalf("error = %v, want it to name the offending disposition entry", err)
	}
}

// TestLoad_DispositionWrongSchema is TestLoad_DispositionWrongKind's own
// case for a document that is disposition-SHAPED (every field the strict
// decoder expects is present) but declares a different schema — proving
// the schema/kind agreement check itself, one layer past the strict-decode
// shape check TestLoad_DispositionWrongKind exercises.
func TestLoad_DispositionWrongSchema(t *testing.T) {
	files := minimalStoreFiles()
	content := strings.Replace(dispositionFile(t, "review-no-conflict"), "schema: verdi.policy-disposition/v1", "schema: verdi.policy-exemption/v1", 1)
	files[".verdi/policy/dispositions/review-no-conflict.md"] = content
	root := t.TempDir()
	writeTree(t, root, files)

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded on a disposition-shaped document declaring the wrong schema, want error")
	}
	if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("error = %v, want a schema-mismatch error", err)
	}
}

// TestLoad_DispositionDirectorySymlinkRejected proves a symlinked
// dispositions/ directory fails closed exactly like every other known
// policy subdirectory (mirrors TestLoad_PolicyRootSymlinkRejected's own
// posture, one level down).
func TestLoad_DispositionDirectorySymlinkRejected(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())

	outside := filepath.Join(root, "outside-dispositions")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(root, ".verdi", "policy", "dispositions")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() followed or accepted a symlinked dispositions directory, want an error")
	}
	if !strings.Contains(err.Error(), "symlink") || !strings.Contains(err.Error(), "dispositions") {
		t.Fatalf("error = %v, want a symlink error naming dispositions", err)
	}
}

// TestLoad_DispositionFileSymlinkRejected is
// TestLoad_SymlinkedArtifactRejected's own case for a disposition FILE.
func TestLoad_DispositionFileSymlinkRejected(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())

	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	target := filepath.Join(outside, "extra-disposition.md")
	if err := os.WriteFile(target, []byte(dispositionFile(t, "extra")), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	dispositionsDir := filepath.Join(root, ".verdi", "policy", "dispositions")
	if err := os.MkdirAll(dispositionsDir, 0o755); err != nil {
		t.Fatalf("mkdir dispositions dir: %v", err)
	}
	link := filepath.Join(dispositionsDir, "extra.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() followed a symlinked disposition artifact, want an error")
	}
	if !strings.Contains(err.Error(), "symlink") || !strings.Contains(err.Error(), "dispositions/extra.md") {
		t.Fatalf("error = %v, want a symlink error naming the path", err)
	}
}
