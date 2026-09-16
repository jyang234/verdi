package specimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// EngineIdentity resolves engine_digest: the SHA-256 of the actual binary
// currently running (spec-import-contract.md: "The determinism/replay
// domain includes engine_digest, SHA-256 of the actual running executable
// (through an injectable internal binary-identity reader for hermetic
// tests)"). Production always uses binaryEngineIdentity; hermetic tests
// inject a fixed-value fake so a rebuilt test binary never perturbs a
// pinned expectation.
type EngineIdentity interface {
	Digest(ctx context.Context) (string, error)
}

// binaryEngineIdentity is the production EngineIdentity: it hashes the
// exact bytes of the currently running executable, resolving a symlink
// (e.g. a `go test` temp binary or an installed wrapper) to the real file
// first so the digest binds actual code, not a link's own bytes.
type binaryEngineIdentity struct{}

func (binaryEngineIdentity) Digest(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrIOFailure, err)
	}
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("%w: resolving running executable: %v", ErrIOFailure, err)
	}
	resolved, err := filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("%w: resolving executable symlink: %v", ErrIOFailure, err)
	}
	f, err := os.Open(resolved)
	if err != nil {
		return "", fmt.Errorf("%w: opening running executable: %v", ErrIOFailure, err)
	}
	defer func() { _ = f.Close() }() // read-only handle; close error is unactionable

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("%w: hashing running executable: %v", ErrIOFailure, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
