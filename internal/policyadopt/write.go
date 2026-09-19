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
// A failure names the offending path AND returns the paths that already
// landed; Write rolls none of them back. That slice is load-bearing, not
// decoration: cmd/verdi's policy adopt stages exactly what landed and, on
// a failure, prints those names as the state its refusal leaves behind on
// the branch it just cut. Disclose, never silently discard.
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
