// Sticky graduation: commit-to-design's "stickies then carry
// status: graduated in the mutable stream" (05 §Workbench; PLAN.md
// deliverable text for the mechanical half of I-20).
//
// D3 documents annotation JSONL streams as append-only, and the
// high-frequency add_annotation write path (boardio.AppendAnnotation)
// honors that literally: one O_APPEND write per call, nothing ever
// rewritten. Graduation is different in kind: it is a low-frequency,
// whole-ritual, single-actor operation (commit-to-design runs once per
// design-branch board, never concurrently with itself) that flips one
// field on records that already exist, and JSONL has no schema-legal way
// to represent "this record superseded that one" short of inventing a
// revision-linkage the artifact contract does not define. GraduateStickies
// therefore rewrites the affected records' lines in place via an atomic
// whole-file replace (temp-then-rename, D3's own pattern for every OTHER
// mutable-zone write) rather than appending a second record under the
// same id — a disclosed, deliberate, narrow exception to per-record
// append-only, scoped to this one low-frequency caller.
package boardio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// GraduateStickies rewrites every annotation record in dir whose id is in
// ids to carry status "graduated", across however many *.jsonl files
// those ids' records actually live in (mirroring ReadAllAnnotations'
// convention of not assuming any particular file-naming scheme). Returns
// how many records were graduated. A file untouched by any id in ids is
// left byte-identical (not rewritten at all, so an unrelated file's mtime
// and any concurrent reader of it are undisturbed).
func GraduateStickies(dir string, ids []string) (int, error) {
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
		path := filepath.Join(dir, name)
		n, err := graduateFile(path, want)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// graduateFile rewrites path in place (atomically) if it contains any
// record whose id is in want, setting that record's status to
// "graduated". Returns how many records in this file were graduated.
func graduateFile(path string, want map[string]bool) (int, error) {
	records, err := ReadAnnotationFile(path)
	if err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}

	changed := 0
	var buf bytes.Buffer
	for _, a := range records {
		if want[a.ID] && a.Status != "graduated" {
			a.Status = "graduated"
			changed++
		}
		line, err := json.Marshal(a)
		if err != nil {
			return 0, fmt.Errorf("boardio: encoding %s: %w", a.ID, err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if changed == 0 {
		return 0, nil
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".annotations.tmp-*")
	if err != nil {
		return 0, fmt.Errorf("boardio: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, werr := tmp.Write(buf.Bytes()); werr != nil {
		tmp.Close()
		os.Remove(tmpName)
		return 0, fmt.Errorf("boardio: writing temp file: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpName)
		return 0, fmt.Errorf("boardio: closing temp file: %w", cerr)
	}
	if rerr := os.Rename(tmpName, path); rerr != nil {
		os.Remove(tmpName)
		return 0, fmt.Errorf("boardio: renaming into place: %w", rerr)
	}
	return changed, nil
}
