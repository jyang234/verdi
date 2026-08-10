package experiment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// This file layers two package-local decode guards OVER the shared
// internal/artifact decode seam; it never replaces it. Both compensate for
// a known gap in that seam and are candidate future shared-seam fixes —
// when internal/artifact grows them, these helpers collapse to the seam
// call alone:
//
//   - checkNoDuplicateJSONKeys: encoding/json (and therefore
//     artifact.DecodeStrictJSON) silently keeps the LAST of repeated
//     object keys, so one document can read one way to a human and another
//     way to the decoder.
//   - checkSingleYAMLDocument: gopkg.in/yaml.v3's single-document decode
//     (and therefore artifact.DecodeStrict) reads only the FIRST document
//     and ignores everything after the next "---", so content the schema
//     never validated can ride along inside a file that decodes clean.
//
// Every CSE artifact in this package decodes through decodeStrictJSON or
// decodeStrictYAML below rather than calling the seam directly, so neither
// gap can be reached by any artifact this package owns.

// decodeStrictJSON decodes data into out through the shared strict JSON
// seam, with the duplicate-key guard running FIRST so a two-faced document
// is rejected before any field of it is believed.
func decodeStrictJSON(data []byte, out interface{}) error {
	if err := checkNoDuplicateJSONKeys(data); err != nil {
		return err
	}
	return artifact.DecodeStrictJSON(data, out)
}

// decodeStrictYAML decodes data into out through the shared strict YAML
// seam and then requires the file to hold exactly one document.
func decodeStrictYAML(data []byte, out interface{}) error {
	if err := artifact.DecodeStrict(data, out); err != nil {
		return err
	}
	return checkSingleYAMLDocument(data)
}

// jsonFrame is one open JSON container in checkNoDuplicateJSONKeys' walk:
// for an object, the set of member names already seen at THIS level and
// whether the next scalar token is a member name rather than a value.
type jsonFrame struct {
	object    bool
	seen      map[string]bool
	expectKey bool
}

// checkNoDuplicateJSONKeys walks data's token stream and fails on the
// first object member name that repeats within the SAME object, at any
// nesting depth (objects inside arrays included). Repeating a name at
// DIFFERENT levels — or in sibling objects — is ordinary JSON and passes.
//
// It exists because encoding/json resolves a repeated key by keeping the
// last occurrence, which lets one document present one verdict to a reader
// and a different verdict to the decoder. Malformed JSON is reported here
// too rather than skipped; the shared seam reports it again in its own
// words for callers that reach it.
func checkNoDuplicateJSONKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var stack []*jsonFrame
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].expectKey = true
		}
	}

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("experiment: scanning json for duplicate keys: %w", err)
		}

		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, &jsonFrame{object: true, seen: make(map[string]bool), expectKey: true})
			case '[':
				stack = append(stack, &jsonFrame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("experiment: scanning json for duplicate keys: unbalanced %q", delim)
				}
				stack = stack[:len(stack)-1]
				valueDone()
			}
			continue
		}

		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].expectKey {
			frame := stack[n-1]
			key, ok := tok.(string)
			if !ok {
				return fmt.Errorf("experiment: scanning json for duplicate keys: object member name is not a string")
			}
			if frame.seen[key] {
				return fmt.Errorf("experiment: duplicate json key %q in the same object", key)
			}
			frame.seen[key] = true
			frame.expectKey = false
			continue
		}
		valueDone()
	}
}

// checkSingleYAMLDocument fails when data holds anything after its first
// YAML document. A second document is rejected whether it parses or not,
// and whether it carries content or is the empty document a bare trailing
// "---" introduces (yaml.v3 yields two documents for both) — the shared
// seam validates only the first one, so any second document is content no
// schema has ever seen.
//
// It reads the document markers textually rather than by re-parsing,
// because CLAUDE.md's single import seam reserves gopkg.in/yaml.v3 for the
// internal/artifact subtree (internal/artifact's own TestYAMLImportSeam
// enforces that module-wide). The text is sufficient here: a directives-end
// marker ("---") or document-end marker ("...") is only a marker at COLUMN
// 0, and inside a block-context document every continuation line — block
// scalar content, multi-line flow scalars, nested collections — is indented
// past column 0. A column-0 marker in one of these artifacts is therefore
// always a document boundary, never content. Counting documents belongs in
// the shared seam; when it moves there, this helper goes away.
func checkSingleYAMLDocument(data []byte) error {
	started := false // the first document's own content or start marker seen
	ended := false   // a "..." document-end marker seen

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")
		switch {
		case isYAMLMarker(line, "---"):
			if started {
				return fmt.Errorf("experiment: trailing yaml document: a file must hold exactly one document")
			}
			started = true
		case isYAMLMarker(line, "..."):
			ended = true
		case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#"):
			// Blank lines and comments belong to no document.
		case !started && strings.HasPrefix(line, "%"):
			// A directive in the prologue of the first document.
		default:
			if ended {
				return fmt.Errorf("experiment: trailing yaml document: content follows the %q document-end marker", "...")
			}
			started = true
		}
	}
	return nil
}

// isYAMLMarker reports whether line is the document marker marker at column
// 0 — the marker alone, or followed by whitespace and the rest of the line.
func isYAMLMarker(line, marker string) bool {
	rest, ok := strings.CutPrefix(line, marker)
	if !ok {
		return false
	}
	return rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\t")
}
