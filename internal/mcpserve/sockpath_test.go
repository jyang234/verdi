package mcpserve

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSocketPath_Happy proves SocketPath is deterministic for a given
// checkout, distinct across checkouts (including two paths that differ
// only in a worktree suffix), well under the sun_path ceiling, and
// actually bindable as a real unix socket — the property the whole
// scheme exists for (I-29).
func TestSocketPath_Happy(t *testing.T) {
	a, err := SocketPath("/Users/dev/code/verdi-system/verdi")
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	again, err := SocketPath("/Users/dev/code/verdi-system/verdi")
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if a != again {
		t.Fatalf("SocketPath not deterministic: %q != %q", a, again)
	}

	b, err := SocketPath("/Users/dev/code/verdi-system/verdi-wt/phase-9-a-very-long-descriptive-worktree-branch-name-for-realism")
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if a == b {
		t.Fatalf("SocketPath collided for two distinct checkouts: %q", a)
	}
	if len(b) > maxSockPathLen {
		t.Fatalf("SocketPath(%d bytes) exceeds maxSockPathLen %d even for a long checkout path", len(b), maxSockPathLen)
	}

	// Actually bind it: the whole point of the short form is that
	// net.Listen("unix", ...) accepts it where the raw checkout-rooted
	// path would not.
	dir := filepath.Dir(a)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	defer os.RemoveAll(dir)
	ln, err := net.Listen("unix", a)
	if err != nil {
		t.Fatalf("net.Listen(unix, %q): %v", a, err)
	}
	ln.Close()
}

// TestSocketPath_RelativeRootsAreAbsolutized proves a relative spelling
// ("." from within the checkout) resolves to the same socket path as the
// exact string os.Getwd() reports for that same location — SocketPath
// must key on the absolutized path, not the literal string a caller
// passed. (It deliberately does not compare against t.TempDir()'s own
// spelling: on macOS that path is itself a symlink into /private, which
// os.Getwd() resolves away after a real Chdir — an OS-level identity
// wrinkle orthogonal to what's under test here.)
func TestSocketPath_RelativeRootsAreAbsolutized(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd after Chdir: %v", err)
	}

	abs, err := SocketPath(cwd)
	if err != nil {
		t.Fatalf("SocketPath(cwd): %v", err)
	}
	rel, err := SocketPath(".")
	if err != nil {
		t.Fatalf("SocketPath(rel): %v", err)
	}
	if abs != rel {
		t.Fatalf("SocketPath(%q) = %q, SocketPath(.) = %q; want equal", cwd, abs, rel)
	}
}

// TestPointerFile_RoundTrips proves WritePointerFile/ReadPointerFile
// round-trip a socket path, that the file is plain-text legible (cat-able,
// 01's legibility goal), and that WritePointerFile writes atomically
// (temp-then-rename — D3): no partial file is ever left as the final
// name.
func TestPointerFile_RoundTrips(t *testing.T) {
	root := t.TempDir()
	sockPath := "/tmp/verdi-abc123/serve.sock"

	if err := WritePointerFile(root, sockPath); err != nil {
		t.Fatalf("WritePointerFile: %v", err)
	}

	got, err := ReadPointerFile(root)
	if err != nil {
		t.Fatalf("ReadPointerFile: %v", err)
	}
	if got != sockPath {
		t.Fatalf("ReadPointerFile = %q, want %q", got, sockPath)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "serve.path"))
	if err != nil {
		t.Fatalf("reading pointer file directly: %v", err)
	}
	if strings.TrimSpace(string(raw)) != sockPath {
		t.Fatalf("pointer file content = %q, want %q (plain text, cat-able)", string(raw), sockPath)
	}

	entries, err := os.ReadDir(filepath.Join(root, ".verdi", "data"))
	if err != nil {
		t.Fatalf("reading data dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file %q after WritePointerFile (should be renamed away)", e.Name())
		}
	}
}

// TestPointerFile_Negative covers reading a pointer file that was never
// written and an empty pointer file (a degenerate case that must not be
// silently treated as "no socket path known" turning into a crash
// elsewhere).
func TestPointerFile_Negative(t *testing.T) {
	t.Run("never written", func(t *testing.T) {
		if _, err := ReadPointerFile(t.TempDir()); err == nil {
			t.Fatal("ReadPointerFile(never written): want error, got nil")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		root := t.TempDir()
		dataDir := filepath.Join(root, ".verdi", "data")
		if err := os.MkdirAll(dataDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dataDir, "serve.path"), nil, 0o644); err != nil {
			t.Fatalf("writing empty pointer file: %v", err)
		}
		if _, err := ReadPointerFile(root); err == nil {
			t.Fatal("ReadPointerFile(empty file): want error, got nil")
		}
	})
}
