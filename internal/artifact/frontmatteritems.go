package artifact

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// ErrNonstandardLineBreak reports a frontmatter line break other than LF or
// CRLF: a lone CR (one not followed by LF), NEL (U+0085), LS (U+2028), or
// PS (U+2029). YAML counts each as a line break and Git does not, so after
// one YAML's line numbers are no longer the document's lines.
var ErrNonstandardLineBreak = errors.New("artifact: frontmatter line break is not LF or CRLF")

// The line breaks YAML counts besides LF and CR.
const (
	nextLine           rune = 0x85
	lineSeparator      rune = 0x2028
	paragraphSeparator rune = 0x2029
)

// FrontmatterItem is one item of a top-level frontmatter sequence whose
// items are string-to-string mappings, with the lines it occupies in the
// whole document.
type FrontmatterItem struct {
	// Fields is the item's mapping. Every key and value is a plain string
	// scalar, and keys are unique.
	Fields map[string]string
	// FirstLine and LastLine are 1-based, inclusive line numbers in the
	// whole document (the opening "---" delimiter is line 1). They span
	// every line on which a node of the item starts: its first and last
	// key or value, and any comment lines between them. A line holding
	// only the item's "-" indicator, and the continuation lines of a
	// scalar that wraps past the item's last node, are outside the span.
	// They are Git's line numbers too, because FrontmatterMappingItems
	// refuses every frontmatter line break YAML and Git count differently.
	FirstLine, LastLine int
}

// FrontmatterMappingItems locates every item of the sequence under the
// top-level frontmatter key in doc, with each item's line span. It is for
// callers that must attribute or remove an item's exact lines (for
// example with git blame) and so need node positions, which the strict
// typed decode does not expose.
//
// The frontmatter passes the same dialect wall as DecodeStrict (no
// anchors, aliases, or custom tags). The top level must be a mapping with
// unique plain-string keys. An absent key, or a key whose value is null,
// yields no items and no error. Any other value that is not a sequence of
// string-to-string mappings is an error.
//
// The frontmatter must also keep one line-break convention (ledger SI-256),
// so that YAML's line numbers are the document's: it must be UTF-8, and its
// only line breaks must be LF and CRLF. A lone CR, NEL, LS, or PS is refused
// with ErrNonstandardLineBreak, and a frontmatter that is not UTF-8 with
// another error. The body is not parsed and may hold any bytes: every item
// line precedes it, so it cannot shift a span.
func FrontmatterMappingItems(doc []byte, key string) ([]FrontmatterItem, error) {
	if key == "" {
		return nil, fmt.Errorf("artifact: frontmatter sequence key must be nonempty")
	}
	start, end, err := FrontmatterRange(doc)
	if err != nil {
		return nil, err
	}
	if err := checkLineBreaks(doc, start, end); err != nil {
		return nil, err
	}
	fm := doc[start:end]
	var root yaml.Node
	if err := yaml.Unmarshal(fm, &root); err != nil {
		return nil, fmt.Errorf("artifact: yaml parse: %w", err)
	}
	if err := checkDialect(&root); err != nil {
		return nil, err
	}
	if root.Kind == 0 || (root.Kind == yaml.DocumentNode && len(root.Content) == 0) {
		return nil, nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("artifact: frontmatter is not a mapping")
	}
	value, err := topLevelValue(root.Content[0], key)
	if err != nil || value == nil {
		return nil, err
	}
	if value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("artifact: frontmatter %q is not a sequence", key)
	}
	items := make([]FrontmatterItem, 0, len(value.Content))
	for i, n := range value.Content {
		fields, ok, err := RawNodeStringMapping(n)
		if err != nil {
			return nil, fmt.Errorf("artifact: frontmatter %s[%d]: %w", key, i, err)
		}
		if !ok {
			return nil, fmt.Errorf("artifact: frontmatter %s[%d] is not a mapping", key, i)
		}
		first, last := nodeLineRange(n)
		// Frontmatter line k is document line k+1: the opening delimiter
		// occupies line 1.
		items = append(items, FrontmatterItem{Fields: fields, FirstLine: first + 1, LastLine: last + 1})
	}
	return items, nil
}

// checkLineBreaks enforces the one line-break convention on the frontmatter
// doc[start:end]. The frontmatter must be UTF-8: YAML reads a UTF-16
// frontmatter by its byte-order mark, and there a byte Git counts as a line
// break can sit inside a character YAML does not break on. Its only line
// breaks must be LF and CRLF. The CR of a CRLF ending the frontmatter's last
// line is followed by that LF in doc, just before the closing delimiter.
func checkLineBreaks(doc []byte, start, end int) error {
	fm := doc[start:end]
	if !utf8.Valid(fm) {
		return fmt.Errorf("artifact: frontmatter is not UTF-8, so its line breaks cannot be shown to be the LF bytes Git counts")
	}
	line := 2 // the document line; the opening delimiter is line 1
	for i, r := range string(fm) {
		switch r {
		case '\n':
			line++
		case '\r':
			if next := start + i + 1; next >= len(doc) || doc[next] != '\n' {
				return fmt.Errorf("%w: a lone CR on document line %d", ErrNonstandardLineBreak, line)
			}
		case nextLine, lineSeparator, paragraphSeparator:
			return fmt.Errorf("%w: %U on document line %d", ErrNonstandardLineBreak, r, line)
		}
	}
	return nil
}

// topLevelValue returns the value under key in the top-level mapping, or
// nil when the key is absent, refusing non-string or duplicate keys.
func topLevelValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	if len(mapping.Content)%2 != 0 {
		return nil, fmt.Errorf("artifact: malformed frontmatter mapping")
	}
	var value *yaml.Node
	seen := make(map[string]bool, len(mapping.Content)/2)
	for i := 0; i < len(mapping.Content); i += 2 {
		k, ok := RawNodeStringScalar(mapping.Content[i])
		if !ok {
			return nil, fmt.Errorf("artifact: frontmatter key at line %d is not a plain string", mapping.Content[i].Line)
		}
		if seen[k] {
			return nil, fmt.Errorf("artifact: frontmatter key %q is a duplicate", k)
		}
		seen[k] = true
		if k == key {
			value = mapping.Content[i+1]
		}
	}
	return value, nil
}

// nodeLineRange returns the smallest and largest start line of n and every
// node beneath it.
func nodeLineRange(n *yaml.Node) (first, last int) {
	first, last = n.Line, n.Line
	for _, c := range n.Content {
		f, l := nodeLineRange(c)
		first = min(first, f)
		last = max(last, l)
	}
	return first, last
}
