// Package atomicfile provides the one shared atomic-write primitive for
// the store's mutable-zone files (spec/shared-homes ac-1). At extraction it
// collapsed four hand copies of the same temp-then-rename idiom — boardio's
// boardstate.go, graduate.go, and reposition.go's own writeFileAtomic;
// boardlayout/file.go — which had drifted (only boardstate.go did
// MkdirAll) and uniformly lacked an fsync, so none was crash-durable. A
// fifth, mcpserve/sockpath.go's WritePointerFile, predated the extraction,
// was missed by that sweep, and was migrated later; it had the same
// missing fsync. Treat the count as historical, not as an inventory: this
// package is the single home, so any new temp-then-rename outside it is a
// defect regardless of how many there once were.
//
// Write follows D3's temp-then-rename pattern: MkdirAll the parent
// directory, CreateTemp in that same directory (so the final Rename is a
// same-filesystem, atomic replace), write the data, fsync the file's
// content, Chmod to the caller's requested permissions, Close, then Rename
// into place. dc-1 disclosed this story's one behavior addition beyond
// pure extraction: the fsync before rename, closing the crash-durability
// gap the audit found uniform across every copy. Parent-directory
// fsync is deliberately NOT added (dc-1) — macOS/CI filesystems differ on
// dir-fsync semantics and no witness demands it; the smallest reversible
// step. The temp file is removed on every error path so a failed write
// never leaves debris behind.
package atomicfile

import (
	"errors"
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

// CreateImmutable atomically publishes an immutable record at path with
// no-clobber semantics, for the policy-conflict D4 cache (authority design
// §7, ledger SI-96/SI-101): "same-directory temp file, content fsync,
// no-clobber atomic publication, then byte-read of an already-existing
// winner." It is deliberately narrow — the one shared primitive this
// caller needs — and must never replace Write or broaden writer ownership:
// nothing here acquires or asserts any lock; the caller (policyconflict's
// cache adapter) owns pairing this call with D3's writer.lock.
//
// The sequence mirrors Write's temp-then-publish shape (MkdirAll,
// same-directory CreateTemp so the eventual publish is a same-filesystem
// operation, write, fsync, Chmod, Close) but publishes via os.Link instead
// of os.Rename: POSIX link(2) fails with EEXIST iff newpath already names
// ANY existing directory entry — including a symlink, which link(2) never
// dereferences to decide that — so a symlink planted at path is refused
// exactly like a genuine existing record, never silently traversed or
// overwritten. Exactly one caller's Link call can succeed for a given
// path, regardless of how many processes or goroutines race it: the OS
// enforces newpath's atomic first-writer-wins semantics, so this function
// needs no lock of its own to be race-safe. On success (created == true)
// the temp file's second name is dropped (the content survives under
// path, now linked from two names sharing one inode, so removing the temp
// alias never touches it). On EEXIST (created == false) the temp file is
// discarded unpublished, path's existing entry is Lstat'd (never a
// symlink — refused outright) and, if a regular file, read back so the
// caller can compare bytes and decide reuse vs. collision itself; this
// function makes no judgment about content equality.
func CreateImmutable(path string, data []byte, perm os.FileMode) (created bool, existing []byte, err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, nil, fmt.Errorf("atomicfile: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".atomicfile-immutable-*.tmp")
	if err != nil {
		return false, nil, fmt.Errorf("atomicfile: creating temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, nil, fmt.Errorf("atomicfile: writing %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, nil, fmt.Errorf("atomicfile: syncing %s: %w", tmpName, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return false, nil, fmt.Errorf("atomicfile: setting permissions on %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, nil, fmt.Errorf("atomicfile: closing %s: %w", tmpName, err)
	}

	linkErr := os.Link(tmpName, path)
	if linkErr == nil {
		_ = os.Remove(tmpName)
		return true, nil, nil
	}
	_ = os.Remove(tmpName)
	if !errors.Is(linkErr, os.ErrExist) {
		return false, nil, fmt.Errorf("atomicfile: publishing %s: %w", path, linkErr)
	}

	info, statErr := os.Lstat(path)
	if statErr != nil {
		return false, nil, fmt.Errorf("atomicfile: %s already exists but could not be inspected: %w", path, statErr)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, nil, fmt.Errorf("atomicfile: refusing to read through existing symlink at %s", path)
	}
	if !info.Mode().IsRegular() {
		return false, nil, fmt.Errorf("atomicfile: refusing existing non-regular entry at %s (mode %s)", path, info.Mode())
	}
	existingData, readErr := os.ReadFile(path)
	if readErr != nil {
		return false, nil, fmt.Errorf("atomicfile: reading existing %s: %w", path, readErr)
	}
	return false, existingData, nil
}
