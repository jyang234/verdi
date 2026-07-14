// Package atomicfile provides the one shared atomic-write primitive for
// the store's mutable-zone files (spec/shared-homes ac-1). It collapses
// four hand copies of the same temp-then-rename idiom — boardio's
// boardstate.go, graduate.go, and reposition.go's own writeFileAtomic;
// boardlayout/file.go — which had drifted (only boardstate.go did
// MkdirAll) and uniformly lacked an fsync, so none was crash-durable.
//
// Write follows D3's temp-then-rename pattern: MkdirAll the parent
// directory, CreateTemp in that same directory (so the final Rename is a
// same-filesystem, atomic replace), write the data, fsync the file's
// content, Chmod to the caller's requested permissions, Close, then Rename
// into place. dc-1 disclosed this story's one behavior addition beyond
// pure extraction: the fsync before rename, closing the crash-durability
// gap the audit found uniform across all four copies. Parent-directory
// fsync is deliberately NOT added (dc-1) — macOS/CI filesystems differ on
// dir-fsync semantics and no witness demands it; the smallest reversible
// step. The temp file is removed on every error path so a failed write
// never leaves debris behind.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write atomically replaces path's contents with data, creating any
// missing parent directories and setting the final file's permission bits
// to perm.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("atomicfile: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".atomicfile-*.tmp")
	if err != nil {
		return fmt.Errorf("atomicfile: creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomicfile: writing %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomicfile: syncing %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomicfile: setting permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomicfile: closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("atomicfile: replacing %s: %w", path, err)
	}
	return nil
}
