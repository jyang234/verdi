// Shared JSONL I/O for annotation streams (02 §Record schemas): reading
// every record out of one stream file, enumerating a whole
// mutable/annotations/ directory, locating the right stream for a given
// target or board, and appending one record.
package boardio

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
)

// AnnotationsDir is data/mutable/annotations/ under the store root
// (01 §Directory layout).
func AnnotationsDir(root string) string {
	return filepath.Join(root, ".verdi", "data", "mutable", "annotations")
}

// AnnotationFileForTarget names the JSONL file a targeted annotation
// belongs in: <kind>--<name>.jsonl, keyed by the TARGET artifact's own
// kind/name (02 §Record schemas' literal path shape), independent of the
// pin's commit — every annotation pinned against any commit of the same
// artifact lives in the same stream.
func AnnotationFileForTarget(ref artifact.Ref) string {
	return fmt.Sprintf("%s--%s.jsonl", ref.Kind, ref.Name)
}

// AnnotationFileForBoard names the JSONL file a free-floating (board-only,
// no target) sticky belongs in, keyed by its board's story slug — see
// mcpserve's original backend.go doc comment (preserved here) for why
// "board" is treated as a pseudo-kind paired with the story's ref slug.
func AnnotationFileForBoard(storySlug string) string {
	return fmt.Sprintf("board--%s.jsonl", storySlug)
}

// ReadAnnotationFile decodes every non-blank line of path as a
// verdi.annotation/v1 record (02 §Record schemas), in file order. A
// missing file is not an error — an artifact or board with no
// annotations yet is a legitimate, common state — and returns an empty
// slice.
func ReadAnnotationFile(path string) ([]*artifact.Annotation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("boardio: reading %s: %w", path, err)
	}

	var out []*artifact.Annotation
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		a, derr := artifact.DecodeAnnotation(line)
		if derr != nil {
			return nil, fmt.Errorf("boardio: %s:%d: %w", path, lineNo, derr)
		}
		out = append(out, a)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("boardio: scanning %s: %w", path, err)
	}
	return out, nil
}

// ReadAllAnnotations decodes every record in every *.jsonl file under
// dir, in a deterministic file-name-sorted order. File content order is
// preserved within each file.
func ReadAllAnnotations(dir string) ([]*artifact.Annotation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("boardio: listing %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var out []*artifact.Annotation
	for _, name := range names {
		as, rerr := ReadAnnotationFile(filepath.Join(dir, name))
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, as...)
	}
	return out, nil
}

// AppendAnnotation appends a's JSON encoding as one line to
// dir/fileName (creating dir if needed). Streams are append-only JSONL
// (D3) — no temp-then-rename here, unlike board state: D3 names exactly
// this exception ("Streams (annotations) are append-only JSONL").
func AppendAnnotation(dir, fileName string, a *artifact.Annotation) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("boardio: creating %s: %w", dir, err)
	}

	line, err := json.Marshal(a)
	if err != nil {
		return fmt.Errorf("boardio: encoding annotation: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(dir, fileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("boardio: opening %s: %w", fileName, err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("boardio: appending to %s: %w", fileName, err)
	}
	// Check the write-path Close: a swallowed close on this appended file
	// could hide a lost flush (previously deferred and dropped).
	if err := f.Close(); err != nil {
		return fmt.Errorf("boardio: closing %s: %w", fileName, err)
	}
	return nil
}
