package boardlayout

// layout.json I/O (02 §Record schemas "Board layout"): the sidecar lives
// inside the spec's own directory, is autosaved during authoring (never
// committed per-drag), and is written canonically so identical positions
// always produce identical bytes.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/canonjson"
)

const schemaID = "verdi.boardlayout/v1"

// FilePath is the layout.json sidecar path inside specDir.
func FilePath(specDir string) string {
	return filepath.Join(specDir, "layout.json")
}

// ReadFile loads specDir's layout.json. A missing file is not an error —
// a spec with no stored positions yet is the normal initial state — and
// returns an empty map.
func ReadFile(specDir string) (map[string]artifact.Position, error) {
	data, err := os.ReadFile(FilePath(specDir))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]artifact.Position{}, nil
		}
		return nil, fmt.Errorf("boardlayout: reading layout.json: %w", err)
	}
	bl, err := artifact.DecodeBoardLayout(data)
	if err != nil {
		return nil, fmt.Errorf("boardlayout: %s: %w", FilePath(specDir), err)
	}
	if bl.Positions == nil {
		return map[string]artifact.Position{}, nil
	}
	return bl.Positions, nil
}

// WriteFile persists positions to specDir's layout.json canonically
// (sorted keys, no HTML escaping, trailing newline), pruned to live —
// the adjudicated orphan-pruning policy (VL-018) — via temp-then-rename.
func WriteFile(specDir string, positions map[string]artifact.Position, live map[string]bool) error {
	pruned := Prune(positions, live)
	out, err := canonjson.Marshal(artifact.BoardLayout{Schema: schemaID, Positions: pruned})
	if err != nil {
		return fmt.Errorf("boardlayout: marshal: %w", err)
	}
	path := FilePath(specDir)
	if err := atomicfile.Write(path, out, 0o600); err != nil {
		return fmt.Errorf("boardlayout: %w", err)
	}
	return nil
}
