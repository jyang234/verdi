package dex

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFile writes data to outDir/relPath, creating parent directories as
// needed, with a fixed 0o644 mode — never derived from umask or an
// existing file's mode, so the output tree's file metadata is as
// deterministic as its content (Phase 12's "byte-identical rebuilds"
// requirement extends to the whole tree a hash-walk test inspects, not
// just page bytes).
func writeFile(outDir, relPath string, data []byte) error {
	full := filepath.Join(outDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return fmt.Errorf("dex: creating directory for %s: %w", relPath, err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("dex: writing %s: %w", relPath, err)
	}
	return nil
}
