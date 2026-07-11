// Mutable board state I/O: data/mutable/boards/<story>.json
// (05 §Workbench board model table: "position ... stored
// data/mutable/boards/<story>.json, autosaved, never committed
// per-drag"). <story> is an opaque, filesystem-safe token — the board's
// own key, used verbatim as the filename stem (matching the literal
// store path and the phase-2 corpus fixture's own
// mutable/boards/STORY-1482.json, which is not a slug of any story ref
// elsewhere in the store — see PLAN.md ledger I-30/I-31 on why the two
// identifier spaces are deliberately not bridged here).
package boardio

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/canonjson"
)

// boardStateSchema is verdi.board/v1 (02 §Record schemas). Duplicated here
// as a literal (internal/artifact's own boardSchema constant is
// unexported) rather than exposed as a new artifact.BoardSchema export,
// since this package only ever needs the one literal to construct a fresh
// empty board.
const boardStateSchema = "verdi.board/v1"

// storyKeyRe is the accepted shape for a board's <story> path/filename
// token: safe on every filesystem this store targets, and immune to path
// traversal ("..", "/") — deliberately permissive about punctuation
// (colons included, for a bare scheme:key story ref used directly as a
// board key) beyond that.
var storyKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]*$`)

// ValidStoryKey reports whether key is safe to use as a board state (or
// board-only annotation stream) filename stem.
func ValidStoryKey(key string) bool {
	return storyKeyRe.MatchString(key)
}

// BoardsDir is data/mutable/boards/ under the store root.
func BoardsDir(root string) string {
	return filepath.Join(root, ".verdi", "data", "mutable", "boards")
}

// BoardStatePath returns the path a board keyed by storyKey lives at, or
// an error if storyKey is not a safe filename stem.
func BoardStatePath(root, storyKey string) (string, error) {
	if !ValidStoryKey(storyKey) {
		return "", fmt.Errorf("boardio: %q is not a valid board key", storyKey)
	}
	return filepath.Join(BoardsDir(root), storyKey+".json"), nil
}

// LoadBoardState reads path as a mutable board state document. A missing
// file is not an error — it returns a fresh, empty board — since a
// story's board is created lazily by first use (05 §Workbench's board
// model table names no separate "create a board" step).
func LoadBoardState(path string) (*artifact.Board, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &artifact.Board{Schema: boardStateSchema}, nil
		}
		return nil, fmt.Errorf("boardio: reading %s: %w", path, err)
	}
	b, err := artifact.DecodeBoard(data)
	if err != nil {
		return nil, fmt.Errorf("boardio: decoding %s: %w", path, err)
	}
	return b, nil
}

// SaveBoardState atomically (temp-then-rename, D3) writes b to path in
// canonical JSON form (I-18). b must validate (artifact.Board.Validate) —
// a caller must never persist a board this package's own decoder would
// then refuse to read back.
func SaveBoardState(path string, b *artifact.Board) error {
	if b.Schema == "" {
		b.Schema = boardStateSchema
	}
	if err := b.Validate(); err != nil {
		return fmt.Errorf("boardio: refusing to save invalid board: %w", err)
	}

	data, err := canonjson.Marshal(b)
	if err != nil {
		return fmt.Errorf("boardio: encoding board: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("boardio: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".board.json.tmp-*")
	if err != nil {
		return fmt.Errorf("boardio: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, werr := tmp.Write(data); werr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: writing temp file: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: closing temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: renaming into place: %w", rerr)
	}
	return nil
}
