//go:build unix

package execworkspace

import (
	"path/filepath"
	"syscall"
	"testing"
)

// TestLstatType_Fifo_IsPathOther exercises the PathOther arm with a real
// non-regular, non-directory, non-symlink object: a named pipe. It is
// build-tagged unix because FIFO creation has no portable Go API — on a
// platform without one this file is simply not built, and the arm stays
// unexercised there rather than silently mis-asserted.
func TestLstatType_Fifo_IsPathOther(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("Mkfifo unsupported on this platform/filesystem: %v", err)
	}
	kind, err := LstatType(fifo)
	if err != nil {
		t.Fatalf("LstatType: unexpected error: %v", err)
	}
	if kind != PathOther {
		t.Fatalf("kind = %v, want PathOther for a named pipe", kind)
	}
	if kind.String() != "other" {
		t.Fatalf("kind.String() = %q, want %q", kind.String(), "other")
	}
}
