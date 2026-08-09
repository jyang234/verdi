package instructionprojection

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGenerate_SymlinkedAncestorDirectory_FailsClosed proves Generate
// refuses to write a managed projection whose ANCESTOR directory is a
// symlink, before writing anything. internal/atomicfile MkdirAlls and
// renames through such a directory, so the store's generated output —
// and every subsequent verification read — would land outside the
// repository the constitution governs (AC-1/DC-1: a projection is this
// repository's own generated file; CO-1: an unsafe target fails closed).
func TestGenerate_SymlinkedAncestorDirectory_FailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := newFixtureRoot(t)
	outside := t.TempDir()
	// The fixture adapter manages docs/AGENTS.md; make docs itself a link.
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	_, err := Generate(root)
	if err == nil {
		t.Fatal("Generate() = nil error, want a fail-closed error naming the symlinked ancestor")
	}
	if !strings.Contains(err.Error(), "docs") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Generate() error = %v, want it to name the symlinked component and say symlink", err)
	}
	if !strings.Contains(err.Error(), "docs/AGENTS.md") {
		t.Fatalf("Generate() error = %v, want it to name the managed path", err)
	}
	// Nothing was written, here or through the link.
	if entries, rerr := os.ReadDir(outside); rerr != nil || len(entries) != 0 {
		t.Fatalf("Generate() wrote through the symlink: entries=%v err=%v", entries, rerr)
	}
	if _, serr := os.Lstat(filepath.Join(root, "AGENTS.md")); serr == nil {
		t.Fatal("Generate() wrote a managed file despite failing closed; the whole projection surface must be proven safe before any write")
	}
}

// TestVerify_SymlinkedAncestorDirectory_FailsClosed proves Verify
// refuses to READ a managed projection through a symlinked ancestor. The
// bytes behind the link may match exactly — that is the dangerous case:
// without this guard Verify would report a clean, proven projection
// chain for a file that does not live in this repository at all.
func TestVerify_SymlinkedAncestorDirectory_FailsClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on windows")
	}
	root := newFixtureRoot(t)
	if _, err := Generate(root); err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if report, err := Verify(root); err != nil || !report.Clean() {
		t.Fatalf("baseline Verify() = %+v, %v; want a clean report", report, err)
	}

	// Move the generated docs/ tree outside the repository and leave a
	// symlink behind: byte-identical content, reached through a link.
	outside := t.TempDir()
	generated, err := os.ReadFile(filepath.Join(root, "docs", "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "AGENTS.md"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "docs")); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	report, err := Verify(root)
	if err == nil {
		t.Fatalf("Verify() = %+v, nil error; want a fail-closed error rather than a match proven through a symlink", report)
	}
	if !strings.Contains(err.Error(), "docs") || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Verify() error = %v, want it to name the symlinked component and say symlink", err)
	}
}
