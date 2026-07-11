// I-29's socket path resolution: 01 §D3 names the store-relative
// `data/serve.sock`, but the wave-4 S4 spike proved macOS's 103-byte
// `sun_path` ceiling is real and hard — realistic worktree checkout paths
// breach it (`bind: invalid argument`). The chosen fix (I-29(a)): bind at
// `$TMPDIR/verdi-<short-hash-of-checkout-path>/serve.sock` (short,
// per-checkout, collision-free) and write the REAL bound path into a
// legible, cat-able pointer file at `.verdi/data/serve.path` — the same
// legibility goal I-12's lock JSON serves. Loud failure (I-29(b)) is the
// backstop if even this short form overflows.
package mcpserve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// maxSockPathLen is the conservative usable length for a unix domain
// socket's sun_path on macOS (spike S4's measured ceiling: 103 bytes,
// leaving room for the NUL terminator within the traditional 104-byte
// sockaddr_un.sun_path array).
const maxSockPathLen = 103

// hashPrefixLen is how many hex characters of the checkout path's SHA-256
// name the per-checkout directory — short enough to keep the whole
// socket path well under maxSockPathLen even under a long $TMPDIR, long
// enough that two distinct checkouts colliding is not a practical concern
// (48 bits of hash space).
const hashPrefixLen = 12

// PointerFileRelPath is data/serve.path's path relative to the store
// root's .verdi/ directory (01 §Directory layout).
const pointerFileRelPath = "data/serve.path"

// SocketPath computes I-29(a)'s bind path for the checkout rooted at
// root: `$TMPDIR/verdi-<hash>/serve.sock`, where hash is a short SHA-256
// prefix of root's absolute path (so distinct checkouts — including
// sibling worktrees — never collide, and a repeat call for the same
// checkout always agrees). It fails loudly, naming the path and the
// limit, when the computed path would still overflow the sun_path
// ceiling (I-29(b), the documented backstop) — a case only a pathological
// $TMPDIR should ever hit, since the per-checkout segment is fixed-width.
func SocketPath(root string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("mcpserve: SocketPath(%q): %w", root, err)
	}

	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}

	sum := sha256.Sum256([]byte(absRoot))
	hash := hex.EncodeToString(sum[:])[:hashPrefixLen]
	sockPath := filepath.Join(tmpDir, "verdi-"+hash, "serve.sock")

	if len(sockPath) > maxSockPathLen {
		return "", fmt.Errorf("mcpserve: SocketPath(%q): computed socket path %q is %d bytes, over the %d-byte unix sun_path ceiling (I-29); shorten $TMPDIR and retry", root, sockPath, len(sockPath), maxSockPathLen)
	}
	return sockPath, nil
}

// WritePointerFile atomically writes sockPath into root's
// .verdi/data/serve.path pointer file (D3: temp-then-rename), creating
// data/ if needed. This is the legible, cat-able record of where the
// socket actually is — I-29's other half, since the socket itself no
// longer lives at the spec-literal store-relative path.
func WritePointerFile(root, sockPath string) error {
	dataDir := filepath.Join(root, ".verdi", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("mcpserve: WritePointerFile: %w", err)
	}
	target := filepath.Join(dataDir, "serve.path")

	tmp, err := os.CreateTemp(dataDir, ".serve.path.tmp-*")
	if err != nil {
		return fmt.Errorf("mcpserve: WritePointerFile: %w", err)
	}
	tmpName := tmp.Name()
	if _, werr := tmp.WriteString(sockPath + "\n"); werr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("mcpserve: WritePointerFile: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("mcpserve: WritePointerFile: %w", cerr)
	}
	if rerr := os.Rename(tmpName, target); rerr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("mcpserve: WritePointerFile: %w", rerr)
	}
	return nil
}

// ReadPointerFile reads back the real socket path WritePointerFile wrote —
// `verdi mcp`'s (cmd/verdi/mcp.go) way of finding a running serve without
// itself recomputing the hash (the pointer file is the single source of
// truth for "where is the socket right now").
func ReadPointerFile(root string) (string, error) {
	target := filepath.Join(root, ".verdi", "data", "serve.path")
	data, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("mcpserve: ReadPointerFile: %w", err)
	}
	path := string(data)
	// Trim exactly the trailing newline WritePointerFile writes; a pointer
	// file with any other trailing whitespace is treated as literal
	// (legibility cuts both ways — no silent whitespace-slurping).
	if len(path) > 0 && path[len(path)-1] == '\n' {
		path = path[:len(path)-1]
	}
	if path == "" {
		return "", fmt.Errorf("mcpserve: ReadPointerFile: %s is empty", target)
	}
	return path, nil
}
