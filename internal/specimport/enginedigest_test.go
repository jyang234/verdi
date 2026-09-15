package specimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// TestBinaryEngineIdentity_MatchesActualRunningExecutable proves the
// production EngineIdentity hashes the ACTUAL running binary
// (spec-import-contract.md: "engine_digest, SHA-256 of the actual running
// executable"), independently recomputed here by reading os.Executable's
// resolved bytes directly rather than trusting the port's own internals.
func TestBinaryEngineIdentity_MatchesActualRunningExecutable(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(self)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatalf("reading own executable: %v", err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])

	got, err := (binaryEngineIdentity{}).Digest(context.Background())
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if got != want {
		t.Fatalf("Digest() = %q, want %q (independently hashed the same running executable)", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("Digest() = %q, not 64 lowercase hex characters", got)
	}
}

// TestBinaryEngineIdentity_CancelledContext proves the port honors context
// cancellation rather than always reading the executable.
func TestBinaryEngineIdentity_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (binaryEngineIdentity{}).Digest(ctx); err == nil {
		t.Fatal("Digest(cancelled context) = nil error, want a refusal")
	}
}
