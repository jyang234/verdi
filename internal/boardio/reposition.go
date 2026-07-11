// Sticky repositioning: the v1 board's free-floating stickies carry
// their position INSIDE the annotation record (02 §Record schemas:
// `board: { story, x, y }`), so a drag must update that record. Same
// disclosed, narrow exception to per-record append-only as graduation
// (graduate.go's doc comment): a low-frequency, single-actor field flip
// on an existing record, rewritten via atomic whole-file replace —
// never a second record under the same id.
package boardio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// RepositionSticky rewrites the annotation record with the given id to
// carry board coordinates (x, y), across whichever *.jsonl stream in dir
// holds it. It fails if the record does not exist or carries no board
// anchor (a targeted-only annotation has no board position to move).
func RepositionSticky(dir, id string, x, y float64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("boardio: listing %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		path := filepath.Join(dir, name)
		moved, rerr := repositionInFile(path, id, x, y)
		if rerr != nil {
			return rerr
		}
		if moved {
			return nil
		}
	}
	return fmt.Errorf("boardio: no annotation %s found to reposition", id)
}

func repositionInFile(path, id string, x, y float64) (bool, error) {
	records, err := ReadAnnotationFile(path)
	if err != nil {
		return false, err
	}
	found := false
	for _, a := range records {
		if a.ID != id {
			continue
		}
		if a.Board == nil {
			return false, fmt.Errorf("boardio: annotation %s has no board anchor to reposition", id)
		}
		a.Board.X = x
		a.Board.Y = y
		found = true
	}
	if !found {
		return false, nil
	}

	var buf bytes.Buffer
	for _, a := range records {
		line, merr := json.Marshal(a)
		if merr != nil {
			return false, fmt.Errorf("boardio: encoding annotation %s: %w", a.ID, merr)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := writeFileAtomic(path, buf.Bytes()); err != nil {
		return false, err
	}
	return true, nil
}

// writeFileAtomic is the temp-then-rename replace graduate.go also uses
// (D3's pattern for every non-append mutable-zone write).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".annotations-*.jsonl")
	if err != nil {
		return fmt.Errorf("boardio: temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: writing %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: closing %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("boardio: replacing %s: %w", path, err)
	}
	return nil
}
