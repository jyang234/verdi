package splice

// The low-level splice primitives, carried over from spike S7's proven
// prototype (docs/spikes/v1/spike-s7-findings.md §2): byte spans located
// from yaml.Node positions plus quote-aware end scanning, and tail-to-head
// batched application against the pristine original buffer.

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// lineOffsets returns, for each 1-indexed line number, the byte offset of
// that line's first character. offsets[0] is unused (offsets[1] is line 1).
func lineOffsets(src []byte) []int {
	offsets := []int{0, 0}
	for i, b := range src {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// byteOffset converts a 1-indexed (line, column) pair to a byte offset
// into the buffer lineOffsets was computed over. Column arithmetic is
// byte-wise; callers must verify the byte found at the offset is the one
// they expect (see nodeSpan) so a multi-byte rune earlier on the line
// fails closed instead of splicing a wrong span (S7 disclosed non-ASCII
// columns as unproven).
func byteOffset(offsets []int, line, col int) (int, error) {
	if line < 1 || line >= len(offsets) {
		return -1, fmt.Errorf("splice: line %d out of range", line)
	}
	return offsets[line] + (col - 1), nil
}

// scanQuotedSpan returns the exclusive end offset of a quoted scalar that
// starts at src[start] (which must be the opening '"' or '\”, matching
// where yaml.v3 points a quoted scalar node). Handles double-quote
// backslash escapes and single-quote doubled-quote escapes.
func scanQuotedSpan(src []byte, start int) (int, error) {
	if start >= len(src) {
		return -1, fmt.Errorf("splice: start %d out of range", start)
	}
	quote := src[start]
	if quote != '"' && quote != '\'' {
		return -1, fmt.Errorf("splice: byte at %d is %q, not a quote", start, quote)
	}
	i := start + 1
	for i < len(src) {
		c := src[i]
		if c == quote {
			if quote == '\'' && i+1 < len(src) && src[i+1] == '\'' {
				i += 2
				continue
			}
			return i + 1, nil
		}
		if quote == '"' && c == '\\' && i+1 < len(src) {
			i += 2
			continue
		}
		i++
	}
	return -1, fmt.Errorf("splice: unterminated quoted scalar starting at %d", start)
}

// findMatchingClose returns the exclusive end offset of a flow container
// ('{'..'}' or '['..']') starting at src[start], skipping quoted spans so
// a brace inside a string never confuses the depth count.
func findMatchingClose(src []byte, start int) (int, error) {
	if start >= len(src) {
		return -1, fmt.Errorf("splice: start %d out of range", start)
	}
	open := src[start]
	var closeCh byte
	switch open {
	case '{':
		closeCh = '}'
	case '[':
		closeCh = ']'
	default:
		return -1, fmt.Errorf("splice: byte at %d is %q, not '{' or '['", start, open)
	}
	depth := 0
	for i := start; i < len(src); {
		switch c := src[i]; c {
		case '"', '\'':
			end, err := scanQuotedSpan(src, i)
			if err != nil {
				return -1, err
			}
			i = end
			continue
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 && c == closeCh {
				return i + 1, nil
			}
		}
		i++
	}
	// vocab:identity — non-vocabulary homograph: "close" names the closing bracket/brace this scanner failed to find (identity)
	return -1, fmt.Errorf("splice: no matching close for %q starting at %d", open, start)
}

// scanPlainFlowScalar returns the exclusive end offset of a plain
// (unquoted) scalar in flow context starting at src[start]: it ends at
// the first ',', '}', ']', '#'-comment, or end of line, with trailing
// spaces excluded. This extends S7's quoted-only coverage; the board
// always writes replacements back double-quoted, so the plain form only
// ever needs to be READ (its span located), never emitted.
func scanPlainFlowScalar(src []byte, start int) (int, error) {
	if start >= len(src) {
		return -1, fmt.Errorf("splice: start %d out of range", start)
	}
	i := start
	for i < len(src) {
		switch src[i] {
		case ',', '}', ']', '\n':
			goto done
		case '#':
			// A comment only starts after whitespace.
			if i > start && (src[i-1] == ' ' || src[i-1] == '\t') {
				goto done
			}
		}
		i++
	}
done:
	end := i
	for end > start && (src[end-1] == ' ' || src[end-1] == '\t') {
		end--
	}
	if end == start {
		return -1, fmt.Errorf("splice: empty plain scalar at %d", start)
	}
	return end, nil
}

// nodeSpan returns the [start, end) byte span of a scalar or flow
// container node within src. It verifies the byte at the computed start
// matches the node's own first character (fail closed on any coordinate
// drift, e.g. from multi-byte runes earlier on the line).
func nodeSpan(src []byte, offsets []int, n *yaml.Node) (start, end int, err error) {
	start, err = byteOffset(offsets, n.Line, n.Column)
	if err != nil {
		return -1, -1, err
	}
	if start >= len(src) {
		return -1, -1, fmt.Errorf("splice: node start offset %d out of range (line %d col %d)", start, n.Line, n.Column)
	}
	switch src[start] {
	case '"', '\'':
		end, err = scanQuotedSpan(src, start)
	case '{', '[':
		end, err = findMatchingClose(src, start)
	default:
		if n.Kind == yaml.ScalarNode && n.Style == 0 {
			// Plain flow scalar: verify the buffer agrees with the node
			// before trusting the span.
			if len(n.Value) == 0 || src[start] != n.Value[0] {
				return -1, -1, fmt.Errorf("splice: buffer/node mismatch at offset %d (line %d col %d): fail closed", start, n.Line, n.Column)
			}
			end, err = scanPlainFlowScalar(src, start)
		} else {
			return -1, -1, fmt.Errorf("splice: node at line %d col %d is not a quoted scalar, plain flow scalar, or flow container (block scalars are unproven — S7); fail closed", n.Line, n.Column)
		}
	}
	if err != nil {
		return -1, -1, err
	}
	return start, end, nil
}

// mapGet returns the value node for key in a mapping node, or nil.
// Content alternates key, value, key, value...
func mapGet(mapNode *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i+1]
		}
	}
	return nil
}

// seqFindByID finds the element (a mapping) in a sequence node whose "id"
// field equals id, or nil.
func seqFindByID(seqNode *yaml.Node, id string) *yaml.Node {
	for _, elem := range seqNode.Content {
		if idNode := mapGet(elem, "id"); idNode != nil && idNode.Value == id {
			return elem
		}
	}
	return nil
}

// Edit is a single byte-range replacement against the original buffer.
// End == Start is a pure insertion.
type Edit struct {
	Start, End int
	Replace    string
}

// applyEdits applies edits to src, all computed against src's original
// offsets, tail-to-head (descending Start) so an edit never invalidates
// the recorded offsets of edits to its left. Overlapping edits fail.
func applyEdits(src []byte, edits []Edit) ([]byte, error) {
	sorted := make([]Edit, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start > sorted[j].Start })

	out := append([]byte(nil), src...)
	for k, e := range sorted {
		if e.Start < 0 || e.End > len(out) || e.Start > e.End {
			return nil, fmt.Errorf("splice: edit %d: invalid span [%d,%d) against %d-byte buffer", k, e.Start, e.End, len(out))
		}
		if k > 0 && e.End > sorted[k-1].Start {
			return nil, fmt.Errorf("splice: edit %d: overlaps previous edit (end %d > prior start %d)", k, e.End, sorted[k-1].Start)
		}
		var buf []byte
		buf = append(buf, out[:e.Start]...)
		buf = append(buf, e.Replace...)
		buf = append(buf, out[e.End:]...)
		out = buf
	}
	return out, nil
}
