package humanartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveScaffold_EmbeddedFallback proves a store with no
// .verdi/templates/ override resolves to the embedded canonical template
// — identity "embedded:<filename>", digest the sha256 of its own bytes
// — the same absence-changes-nothing posture designscaffold.LoadTemplate
// already established.
func TestResolveScaffold_EmbeddedFallback(t *testing.T) {
	root := t.TempDir()
	s, err := ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	if s.Identity != "embedded:policy.md" {
		t.Fatalf("Identity = %q, want %q", s.Identity, "embedded:policy.md")
	}
	sum := sha256.Sum256(s.Template)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if s.Digest != wantDigest {
		t.Fatalf("Digest = %q, want %q", s.Digest, wantDigest)
	}
	if len(s.Template) == 0 {
		t.Fatal("Template bytes are empty")
	}
}

// TestResolveScaffold_StoreOverrideWins proves a store's own
// .verdi/templates/<filename> override wins over the embedded default —
// identity "store:.verdi/templates/<filename>", digest of the override's
// OWN bytes.
func TestResolveScaffold_StoreOverrideWins(t *testing.T) {
	root := t.TempDir()
	override := []byte("---\nsome: override\n---\nbody\n")
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "templates", "policy.md"), override, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	if s.Identity != "store:.verdi/templates/policy.md" {
		t.Fatalf("Identity = %q, want %q", s.Identity, "store:.verdi/templates/policy.md")
	}
	sum := sha256.Sum256(override)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if s.Digest != wantDigest {
		t.Fatalf("Digest = %q, want %q", s.Digest, wantDigest)
	}
	if string(s.Template) != string(override) {
		t.Fatalf("Template = %q, want the override's own bytes %q", s.Template, override)
	}
}

// TestResolveScaffold_UnsafeFilename proves designscaffold.LoadOverride's
// own bare-filename refusal propagates through ResolveScaffold, never
// silently ignored.
func TestResolveScaffold_UnsafeFilename(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"../evil.md", "sub/dir.md", "/abs.md", ".", ".."} {
		_, err := ResolveScaffold(root, bad)
		if err == nil {
			t.Errorf("ResolveScaffold(%q) = nil error, want a bare-filename refusal", bad)
			continue
		}
		if !strings.Contains(err.Error(), "bare filename") {
			t.Errorf("ResolveScaffold(%q) error = %v, want it to name the bare-filename rule", bad, err)
		}
	}
}

// TestResolveScaffold_UnknownFilename proves a filename with neither a
// store override nor an embedded canonical default fails closed.
func TestResolveScaffold_UnknownFilename(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveScaffold(root, "no-such-template.md"); err == nil {
		t.Fatal("ResolveScaffold(unknown filename) = nil error, want a not-found failure")
	}
}
