// Shared JSONL I/O for the three annotation tools (list_annotations,
// list_tasks, add_annotation): reading every record out of one stream
// file and enumerating the whole mutable/annotations/ directory.
package mcpserve

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
)

// readAnnotationFile decodes every non-blank line of path as a
// verdi.annotation/v1 record (02 §Record schemas), in file order. A
// missing file is not an error — an artifact or board with no
// annotations yet is a legitimate, common state — and returns an empty
// slice.
func readAnnotationFile(path string) ([]*artifact.Annotation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("mcpserve: reading %s: %w", path, err)
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
			return nil, fmt.Errorf("mcpserve: %s:%d: %w", path, lineNo, derr)
		}
		out = append(out, a)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("mcpserve: scanning %s: %w", path, err)
	}
	return out, nil
}

// readAllAnnotations decodes every record in every *.jsonl file under
// dir, in a deterministic file-name-sorted order (list_tasks's "every
// open agent-task annotation across the whole mutable zone" needs a
// stable-enough traversal order; file content order is preserved within
// each file).
func readAllAnnotations(dir string) ([]*artifact.Annotation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("mcpserve: listing %s: %w", dir, err)
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
		as, rerr := readAnnotationFile(filepath.Join(dir, name))
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, as...)
	}
	return out, nil
}
