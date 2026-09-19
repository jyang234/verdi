package policyadopt

import (
	"fmt"
	"path/filepath"

	"github.com/jyang234/verdi/internal/atomicfile"
)

// Write writes every file p.Compose proved, in p.Files' own order,
// returning the written store-relative paths. Each write goes through
// atomicfile.Write (create-temp, fsync, rename-into-place — the same
// crash-durability primitive every other corpus write in this repository
// shares; it creates the destination directory itself, so Write does not
// separately mkdir).
//
// A failure names the offending path and returns the paths already
// written alongside it: Write attempts no rollback of those. The caller
// is on a fresh branch it can itself report as partially written —
// disclosed, never silently discarded (the brief's own "do not attempt
// rollback — disclose").
func Write(root string, p *Plan) ([]string, error) {
	paths := make([]string, 0, len(p.Files))
	for _, f := range p.Files {
		abs := filepath.Join(root, filepath.FromSlash(f.RelPath))
		if err := atomicfile.Write(abs, f.Content, 0o644); err != nil {
			return paths, fmt.Errorf("policyadopt: writing %s: %w", f.RelPath, err)
		}
		paths = append(paths, f.RelPath)
	}
	return paths, nil
}
