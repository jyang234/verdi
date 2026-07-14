// Annotation deletion: the scratch tier's other exit — records
// "graduate … or they die" (05 §Workbench), and deletion is the dying
// half, spec-sanctioned and owner-directed (round-6 UAT item 3). Like
// graduation (graduate.go's doc comment carries the full rationale),
// deletion is a low-frequency, single-actor flip on records that already
// exist, so it shares the same disclosed, narrow exception to
// per-record append-only: rewrite the affected stream files without the
// dead records via an atomic whole-file replace. A file containing no
// deleted record is left byte-identical.
package boardio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/atomicfile"
)

// DeleteAnnotations removes every annotation record in dir whose id is
// in ids, across however many *.jsonl files those records live in.
// Returns how many records were deleted. Missing directories and
// unknown ids are calm no-ops — the caller decides whether zero is an
// error.
func DeleteAnnotations(dir string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("boardio: listing %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	total := 0
	for _, name := range names {
		n, err := deleteFromFile(filepath.Join(dir, name), want)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// deleteFromFile rewrites path atomically without the records whose ids
// are in want; a file containing none of them is not rewritten at all.
func deleteFromFile(path string, want map[string]bool) (int, error) {
	records, err := ReadAnnotationFile(path)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}

	deleted := 0
	var buf bytes.Buffer
	for _, a := range records {
		if want[a.ID] {
			deleted++
			continue
		}
		line, err := json.Marshal(a)
		if err != nil {
			return 0, fmt.Errorf("boardio: encoding %s: %w", a.ID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if deleted == 0 {
		return 0, nil
	}
	if err := atomicfile.Write(path, buf.Bytes(), 0o600); err != nil {
		return 0, fmt.Errorf("boardio: %w", err)
	}
	return deleted, nil
}
