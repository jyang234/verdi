package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// ArchiveMove moves a spec's whole directory from specs/active/<name>/ to
// specs/archive/<name>/, byte for byte (round 6, spec/close-verb ac-1; 03
// §Closure ritual: "the spec directory's active→archive move"; 02
// §Identity and references: "an active→archive move changes the path but
// never the ref" — the ref stays spec/<name> regardless of which zone it
// lives under).
//
// This is a pure os.Rename: no file inside the directory is read, decoded,
// or rewritten. That is load-bearing, not incidental — it is the only way
// VL-010's sole legal exception on an otherwise-frozen spec (a
// 100%-similarity git rename, internal/lint's vl010.go) can be satisfied
// for spec.md, which has carried its `frozen:` stamp since `verdi accept`
// ran, long before any closure branch exists. Callers that need closure
// records in the archive tree (a frozen deviation-report.md, a fresh
// rollup.json) must write them into the active-zone directory BEFORE
// calling ArchiveMove, so they move with the whole target spec directory in
// one shot — never written directly into the archive zone, which would
// leave the directory momentarily split across both zones if any step
// failed partway.
//
// name must already exist under specs/active/ and contain a spec.md (the
// one file every spec directory is guaranteed to carry); the target under
// specs/archive/ must not already exist (no clobbering a prior archive —
// closure is a one-way, one-time move per spec).
func ArchiveMove(root, name string) error {
	activeDir := ActiveSpecDir(root, name)
	archiveDir := ArchiveSpecDir(root, name)

	specPath := ActiveSpecPath(root, name)
	if info, err := os.Stat(specPath); err != nil || info.IsDir() {
		if err == nil {
			err = fmt.Errorf("is a directory")
		}
		return fmt.Errorf("store: ArchiveMove: %s does not exist: %w", specPath, err)
	}

	if _, err := os.Stat(archiveDir); err == nil {
		return fmt.Errorf("store: ArchiveMove: %s already exists; refusing to overwrite a prior archive", archiveDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("store: ArchiveMove: checking %s: %w", archiveDir, err)
	}

	archiveParent := filepath.Dir(archiveDir)
	if err := os.MkdirAll(archiveParent, 0o755); err != nil {
		return fmt.Errorf("store: ArchiveMove: mkdir %s: %w", archiveParent, err)
	}
	if err := os.Rename(activeDir, archiveDir); err != nil {
		return fmt.Errorf("store: ArchiveMove: renaming %s to %s: %w", activeDir, archiveDir, err)
	}
	return nil
}
