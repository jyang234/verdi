//go:build unix

package disclosureview

import (
	"context"
	"errors"
	"path/filepath"
	"syscall"
	"testing"
)

// TestReadInputs_NamedPipeIsUncomputable: a named pipe under .verdi is not
// a regular file the key can read (reading it would block), so the key is
// uncomputable and the call enumerates afresh.
func TestReadInputs_NamedPipeIsUncomputable(t *testing.T) {
	fx := newCacheFixture(t)
	if err := syscall.Mkfifo(filepath.Join(fx.root, ".verdi", "pipe"), 0o644); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	if _, err := readInputs(context.Background(), fx.root); !errors.Is(err, errUncomputable) {
		t.Fatalf("readInputs error = %v, want errUncomputable", err)
	}
}
