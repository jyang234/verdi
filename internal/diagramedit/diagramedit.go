// Package diagramedit is the board editor's structural-op → text-edit
// grammar (spec/board-editor ac-2/ac-3, dc-2): each operation — add-node,
// connect, rename, delete-node, delete-edge — is a PURE function of
// (source bytes, operation) to source bytes. Server-computed (one writer,
// no client-side duplicate of the edit logic), line-oriented, and
// byte-preserving: an op changes only the lines dc-2's grammar names for
// it and leaves every other byte — indentation, comments, blank lines,
// ordering, trailing whitespace — bit-identical (co-1). Nothing here ever
// round-trips the source through a graph representation: every edit is a
// byte-span splice on the original slice.
//
// The ops recognize ONLY the flowchart subset dc-2 names: a
// flowchart/graph header, %% comment lines, blank lines, bare-node lines
// (`<id>`), quoted-square-bracket node declarations (`<id>["<label>"]`),
// and plain arrow edges (`<from> --> <to>`). Any other source the pinned
// renderer accepts (sequence, state, labeled edges, other node shapes,
// subgraphs, ...) is OUTSIDE the subset: Parse returns a typed
// *OutsideSubsetError and every op fails with it — the ops never guess at
// source they do not parse and never rewrite it to something they do
// (ac-2's disclosed-unavailable posture; the code pane stays live, this
// package is simply not consulted for edits).
//
// No LLM and no graph analysis anywhere (co-3): this is line
// classification and byte splicing only.
package diagramedit

import (
	"fmt"
	"regexp"
	"strings"
)

// idPattern is the node-id token the OPS grammar accepts: a leading
// letter or underscore, then letters/digits/underscores/dashes. This is
// deliberately WIDER than internal/diagramverify's nodeIDPattern (which
// excludes '-'): flowmap-generated bases use dashed ShortNames
// (notification-svc) and a derived proposal must stay op-editable. The
// two grammars serve different claims — the extractor's binds coverage,
// this one binds only which lines an op may touch — so they are declared
// independently (dc-2: the grammar never guesses).
const idPattern = `[A-Za-z_][A-Za-z0-9_-]*`

var (
	headerRe    = regexp.MustCompile(`^(?:flowchart|graph)\s+(?:TB|TD|BT|RL|LR)$`)
	bareNodeRe  = regexp.MustCompile(`^(` + idPattern + `)$`)
	labelNodeRe = regexp.MustCompile(`^(` + idPattern + `)\["([^"]*)"\]$`)
	edgeRe      = regexp.MustCompile(`^(` + idPattern + `)\s*-->\s*(` + idPattern + `)$`)
)

// reservedKeywords are mermaid's own construct keywords: a bare line
// equal to one of these is a construct this grammar does NOT claim (a
// subgraph opener/closer, a styling statement, ...) — never a node named
// "end". Such a line puts the source outside the subset rather than
// being silently mis-parsed (dc-2: a partial parse never rewrites what
// it did not understand).
var reservedKeywords = map[string]bool{
	"end": true, "subgraph": true, "direction": true,
	"style": true, "linkStyle": true, "classDef": true, "class": true, "click": true,
}

// OutsideSubsetError is the typed ops-unavailable result (ac-2): the
// source contains a construct outside the op grammar's flowchart subset.
// The offending line is named so the disclosure is concrete.
type OutsideSubsetError struct {
	Line int    // 1-based line number of the first unrecognized construct
	Text string // the unrecognized line, trimmed
}

func (e *OutsideSubsetError) Error() string {
	return fmt.Sprintf("diagramedit: source is outside the op grammar's flowchart subset (line %d: %q); structural operations are unavailable — the code pane stays live", e.Line, e.Text)
}

// OpError is a typed refusal of one operation against in-subset source
// (unknown node, label the grammar cannot carry, ...). It never carries a
// rewritten source: the operation simply did not happen.
type OpError struct{ Reason string }

func (e *OpError) Error() string { return "diagramedit: " + e.Reason }

// Node is one recognized node: its immutable id and its current label
// ("" for a bare node). Order is source order of first appearance.
type Node struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Edge is one recognized edge line's ordered endpoint pair.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// line is one physical source line with its byte span in the original
// slice. span excludes the "\n" terminator; nl is true when the line is
// terminated by "\n" (false only for a final unterminated line).
type line struct {
	start, end int
	nl         bool
}

// lineKind classifies a line within the subset.
type lineKind int

const (
	lineBlank lineKind = iota
	lineComment
	lineHeader
	lineBareNode
	lineLabelNode
	lineEdge
)

// parsedLine is one classified line: kind plus the byte offsets of the
// grammar's named parts (indent, id, label span for label rewrites).
type parsedLine struct {
	line
	kind       lineKind
	id         string // node id (bare/label lines) — immutable through every op
	label      string
	from, to   string // edge lines
	indent     string
	labelStart int // byte offset of the label's first byte (label lines)
	labelEnd   int // byte offset just past the label's last byte
	contentEnd int // byte offset just past the recognized construct (before trailing ws)
}

// Doc is a parsed in-subset source: the recognized node/edge model plus
// everything an op needs to splice bytes. It never stores a normalized
// form of the text — Src is the original slice, and every op result is
// derived from it by span surgery alone.
type Doc struct {
	src   []byte
	lines []parsedLine
}

// Parse classifies src against the op grammar's flowchart subset. It
// returns a typed *OutsideSubsetError naming the first unrecognized
// construct when src is outside the subset (ops disclosed unavailable),
// or an *OpError when src has no flowchart/graph header at all.
func Parse(src []byte) (*Doc, error) {
	d := &Doc{src: src}
	sawHeader := false
	offset := 0
	lineNo := 0
	s := string(src)
	for offset <= len(s) {
		lineNo++
		rel := strings.IndexByte(s[offset:], '\n')
		var l line
		if rel < 0 {
			if offset == len(s) {
				break // no final unterminated line
			}
			l = line{start: offset, end: len(s), nl: false}
			offset = len(s) + 1
		} else {
			l = line{start: offset, end: offset + rel, nl: true}
			offset += rel + 1
		}
		text := s[l.start:l.end]
		pl, ok := classify(text, l)
		if !ok {
			return nil, &OutsideSubsetError{Line: lineNo, Text: strings.TrimSpace(text)}
		}
		if pl.kind == lineHeader {
			if sawHeader {
				// A second header is a construct this grammar does not
				// claim to understand.
				return nil, &OutsideSubsetError{Line: lineNo, Text: strings.TrimSpace(text)}
			}
			sawHeader = true
		}
		if (pl.kind == lineBareNode || pl.kind == lineLabelNode || pl.kind == lineEdge) && !sawHeader {
			// Content before the flowchart header is not the flowchart
			// subset (it is some other diagram type's vocabulary).
			return nil, &OutsideSubsetError{Line: lineNo, Text: strings.TrimSpace(text)}
		}
		d.lines = append(d.lines, pl)
	}
	if !sawHeader {
		return nil, &OutsideSubsetError{Line: 1, Text: "(no flowchart/graph header)"}
	}
	return d, nil
}

// classify parses one line's text against the subset grammar. Offsets in
// the returned parsedLine are absolute (relative to the document).
func classify(text string, l line) (parsedLine, bool) {
	pl := parsedLine{line: l}
	trimmed := strings.TrimRight(text, " \t\r")
	if strings.TrimLeft(trimmed, " \t") == "" {
		pl.kind = lineBlank
		return pl, true
	}
	indentLen := len(trimmed) - len(strings.TrimLeft(trimmed, " \t"))
	pl.indent = trimmed[:indentLen]
	body := trimmed[indentLen:]
	bodyStart := l.start + indentLen
	pl.contentEnd = bodyStart + len(body)

	if strings.HasPrefix(body, "%%") {
		pl.kind = lineComment
		return pl, true
	}
	if headerRe.MatchString(body) {
		pl.kind = lineHeader
		return pl, true
	}
	if m := edgeRe.FindStringSubmatch(body); m != nil {
		pl.kind = lineEdge
		pl.from, pl.to = m[1], m[2]
		return pl, true
	}
	if m := labelNodeRe.FindStringSubmatch(body); m != nil {
		pl.kind = lineLabelNode
		pl.id, pl.label = m[1], m[2]
		// label bytes sit between `["` and `"]`: id + `["` prefix.
		pl.labelStart = bodyStart + len(m[1]) + 2
		pl.labelEnd = pl.labelStart + len(m[2])
		return pl, true
	}
	if m := bareNodeRe.FindStringSubmatch(body); m != nil && !reservedKeywords[body] {
		pl.kind = lineBareNode
		pl.id = m[1]
		return pl, true
	}
	return pl, false
}

// Nodes returns the recognized nodes in order of first appearance: every
// defining line's id, plus every edge endpoint not otherwise defined
// (mermaid materializes those implicitly). Labels come from the id's
// FIRST defining occurrence — the same occurrence rename rewrites.
func (d *Doc) Nodes() []Node {
	var out []Node
	seen := map[string]int{}
	add := func(id, label string, defining bool) {
		if i, ok := seen[id]; ok {
			if defining && out[i].Label == "" && label != "" {
				// an edge introduced the id first; the defining line
				// carries the label. First DEFINING occurrence wins.
				out[i].Label = label
			}
			return
		}
		seen[id] = len(out)
		out = append(out, Node{ID: id, Label: label})
	}
	for _, pl := range d.lines {
		switch pl.kind {
		case lineBareNode:
			add(pl.id, "", true)
		case lineLabelNode:
			add(pl.id, pl.label, true)
		case lineEdge:
			add(pl.from, "", false)
			add(pl.to, "", false)
		}
	}
	return out
}

// Edges returns every recognized edge line's (from, to) pair in source
// order, duplicates included (each is its own line).
func (d *Doc) Edges() []Edge {
	var out []Edge
	for _, pl := range d.lines {
		if pl.kind == lineEdge {
			out = append(out, Edge{From: pl.from, To: pl.to})
		}
	}
	return out
}

// hasNode reports whether id is a recognized node (defined or edge-named).
func (d *Doc) hasNode(id string) bool {
	for _, n := range d.Nodes() {
		if n.ID == id {
			return true
		}
	}
	return false
}

// prevailingIndent is the indentation add-node/connect append at (dc-2:
// "at the source's prevailing indentation"): the most common indent
// string among node/edge lines, ties broken by first appearance; two
// spaces when the source has no node/edge line yet.
func (d *Doc) prevailingIndent() string {
	counts := map[string]int{}
	var order []string
	for _, pl := range d.lines {
		switch pl.kind {
		case lineBareNode, lineLabelNode, lineEdge:
			if counts[pl.indent] == 0 {
				order = append(order, pl.indent)
			}
			counts[pl.indent]++
		}
	}
	best, bestN := "  ", 0
	for _, ind := range order {
		if counts[ind] > bestN {
			best, bestN = ind, counts[ind]
		}
	}
	return best
}

// checkLabel refuses a label the `<id>["<label>"]` grammar cannot carry
// verbatim — never escaped, never repaired (the grammar does not guess).
func checkLabel(label string) error {
	if label == "" {
		return &OpError{Reason: `label must not be empty`}
	}
	if strings.Contains(label, `"`) {
		return &OpError{Reason: `label contains a double quote, which the <id>["<label>"] grammar cannot carry`}
	}
	if strings.ContainsAny(label, "\n\r") {
		return &OpError{Reason: "label contains a line break, which a one-line node declaration cannot carry"}
	}
	return nil
}

// appendLine appends one grammar-named line after the source's last byte:
// the ONLY bytes added are the new line itself (plus the separating "\n"
// when the source does not end with one — the pre-existing final line's
// own bytes are untouched, and a source without a trailing newline stays
// without one).
func appendLine(src []byte, text string) []byte {
	out := make([]byte, 0, len(src)+len(text)+2)
	out = append(out, src...)
	if len(src) > 0 && src[len(src)-1] == '\n' {
		out = append(out, text...)
		out = append(out, '\n')
		return out
	}
	out = append(out, '\n')
	out = append(out, text...)
	return out
}

// AddNode appends one `<id>["<label>"]` line with id the lowest unused
// n<k> identifier not present in the source (dc-2), returning the new
// source bytes and the minted id.
func AddNode(src []byte, label string) (out []byte, id string, err error) {
	d, err := Parse(src)
	if err != nil {
		return nil, "", err
	}
	if err := checkLabel(label); err != nil {
		return nil, "", err
	}
	used := map[string]bool{}
	for _, n := range d.Nodes() {
		used[n.ID] = true
	}
	for k := 1; ; k++ {
		id = fmt.Sprintf("n%d", k)
		if !used[id] {
			break
		}
	}
	lineText := d.prevailingIndent() + id + `["` + label + `"]`
	return appendLine(src, lineText), id, nil
}

// Connect appends one `<from> --> <to>` line (dc-2). Both endpoints must
// already be recognized nodes: connect draws between what exists — it
// never mints nodes as a side effect.
func Connect(src []byte, from, to string) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if !d.hasNode(from) {
		return nil, &OpError{Reason: fmt.Sprintf("connect: %q is not a node in this source", from)}
	}
	if !d.hasNode(to) {
		return nil, &OpError{Reason: fmt.Sprintf("connect: %q is not a node in this source", to)}
	}
	return appendLine(src, d.prevailingIndent()+from+" --> "+to), nil
}

// Rename rewrites only the label text at the node's defining occurrence
// (its first bare or labeled declaration line); a bare node gains
// brackets: `A` becomes `A["label"]`. Identity ids are immutable through
// the ops (dc-2) — the id bytes, and every other byte on the line
// (indentation, trailing whitespace), survive bit-identically.
func Rename(src []byte, id, label string) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	if err := checkLabel(label); err != nil {
		return nil, err
	}
	for _, pl := range d.lines {
		switch pl.kind {
		case lineLabelNode:
			if pl.id != id {
				continue
			}
			out := make([]byte, 0, len(src)-(pl.labelEnd-pl.labelStart)+len(label))
			out = append(out, src[:pl.labelStart]...)
			out = append(out, label...)
			out = append(out, src[pl.labelEnd:]...)
			return out, nil
		case lineBareNode:
			if pl.id != id {
				continue
			}
			out := make([]byte, 0, len(src)+len(label)+4)
			out = append(out, src[:pl.contentEnd]...)
			out = append(out, `["`...)
			out = append(out, label...)
			out = append(out, `"]`...)
			out = append(out, src[pl.contentEnd:]...)
			return out, nil
		}
	}
	if d.hasNode(id) {
		return nil, &OpError{Reason: fmt.Sprintf("rename: node %q has no defining line (it appears only inside edge lines); the grammar rewrites defining occurrences only", id)}
	}
	return nil, &OpError{Reason: fmt.Sprintf("rename: %q is not a node in this source", id)}
}

// removeLines returns src minus the given lines (byte spans including
// each removed line's own "\n"; a removed final unterminated line takes
// its PRECEDING "\n" with it so no dangling separator is invented).
func removeLines(src []byte, remove []line) []byte {
	type span struct{ start, end int }
	var spans []span
	for _, l := range remove {
		s := span{start: l.start, end: l.end}
		if l.nl {
			s.end++ // the line's own terminator goes with it
		} else if s.start > 0 {
			s.start-- // final unterminated line: its preceding "\n" goes
		}
		spans = append(spans, s)
	}
	out := make([]byte, 0, len(src))
	prev := 0
	for _, s := range spans {
		if s.start < prev {
			// Adjacent removals can share one separator byte (a removed
			// terminated line followed by the removed final unterminated
			// line): never double-count it.
			s.start = prev
		}
		out = append(out, src[prev:s.start]...)
		if s.end > prev {
			prev = s.end
		}
	}
	out = append(out, src[prev:]...)
	return out
}

// DeleteNode removes the node's defining line(s) plus every edge line
// naming it (dc-2). Every other byte survives bit-identically.
func DeleteNode(src []byte, id string) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	var remove []line
	for _, pl := range d.lines {
		switch pl.kind {
		case lineBareNode, lineLabelNode:
			if pl.id == id {
				remove = append(remove, pl.line)
			}
		case lineEdge:
			if pl.from == id || pl.to == id {
				remove = append(remove, pl.line)
			}
		}
	}
	if len(remove) == 0 {
		return nil, &OpError{Reason: fmt.Sprintf("delete: %q is not a node in this source", id)}
	}
	return removeLines(src, remove), nil
}

// DeleteEdge removes that one line (dc-2): the FIRST edge line matching
// the ordered (from, to) pair. Duplicate edge lines are distinct lines;
// each delete removes exactly one.
func DeleteEdge(src []byte, from, to string) ([]byte, error) {
	d, err := Parse(src)
	if err != nil {
		return nil, err
	}
	for _, pl := range d.lines {
		if pl.kind == lineEdge && pl.from == from && pl.to == to {
			return removeLines(src, []line{pl.line}), nil
		}
	}
	return nil, &OpError{Reason: fmt.Sprintf("delete: no %s --> %s edge line in this source", from, to)}
}
