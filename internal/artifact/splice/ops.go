package splice

// The board-facing edit operations: each returns Edits computed against
// the pristine whole-file buffer (never a reassembled string — S7 §2's
// gotcha), to be applied in one tail-to-head batch and validated before
// any write (Validate).

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/OWNER/verdi/internal/artifact"
)

// Doc is one parsed spec document: the pristine buffer plus the
// frontmatter node tree with the coordinate delta needed to convert
// node positions (relative to the extracted frontmatter) into
// whole-file offsets (S7 §2 step 4).
type Doc struct {
	src []byte
	// offsets are the WHOLE FILE's line offsets.
	offsets []int
	// fm is the frontmatter's top-level mapping node.
	fm *yaml.Node
	// lineDelta converts a frontmatter-relative node line to a file
	// line: fm line 1 is the file's line 2 (after the opening "---").
	lineDelta int
	// fmCloseOffset is the byte offset of the closing "---" line's first
	// character — the insertion point for a new top-level block.
	fmCloseOffset int
}

// Parse reads a spec document into a Doc. The frontmatter is extracted by
// byte range (never split/join), parsed into a yaml.Node tree for
// positions.
func Parse(src []byte) (*Doc, error) {
	offsets := lineOffsets(src)
	lines := bytes.Split(src, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimRight(lines[0], "\r")) != "---" {
		return nil, fmt.Errorf("splice: document does not start with a %q frontmatter delimiter", "---")
	}
	closeLine := -1
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimRight(lines[i], "\r")) == "---" {
			closeLine = i + 1 // 1-indexed
			break
		}
	}
	if closeLine == -1 {
		return nil, fmt.Errorf("splice: no closing %q frontmatter delimiter found", "---")
	}

	fmStart := offsets[2] // first byte of line 2
	fmEnd := offsets[closeLine]
	fmText := src[fmStart:fmEnd]

	var root yaml.Node
	if err := yaml.Unmarshal(fmText, &root); err != nil {
		return nil, fmt.Errorf("splice: parsing frontmatter: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("splice: frontmatter is not a single YAML mapping")
	}

	return &Doc{
		src:           src,
		offsets:       offsets,
		fm:            root.Content[0],
		lineDelta:     1,
		fmCloseOffset: fmEnd,
	}, nil
}

// span returns n's [start, end) whole-file byte span. It shifts the
// node's frontmatter-relative line by the document's delta, then defers
// to nodeSpan's fail-closed byte verification.
func (d *Doc) span(n *yaml.Node) (int, int, error) {
	shifted := *n
	shifted.Line = n.Line + d.lineDelta
	return nodeSpan(d.src, d.offsets, &shifted)
}

// blockForID maps an object id to its frontmatter block by prefix
// (R4-I-13's decode rule: ac-/co-/dc-/oq- per block).
func blockForID(id string) (string, error) {
	switch {
	case strings.HasPrefix(id, "ac-"):
		return "acceptance_criteria", nil
	case strings.HasPrefix(id, "co-"):
		return "constraints", nil
	case strings.HasPrefix(id, "dc-"):
		return "decisions", nil
	case strings.HasPrefix(id, "oq-"):
		return "open_questions", nil
	default:
		return "", fmt.Errorf("splice: object id %q has no known block prefix (ac-/co-/dc-/oq-)", id)
	}
}

// objectElem locates the mapping node of the object with the given id.
func (d *Doc) objectElem(id string) (*yaml.Node, error) {
	block, err := blockForID(id)
	if err != nil {
		return nil, err
	}
	seq := mapGet(d.fm, block)
	if seq == nil {
		return nil, fmt.Errorf("splice: spec has no %s block", block)
	}
	elem := seqFindByID(seq, id)
	if elem == nil {
		return nil, fmt.Errorf("splice: no object %q in %s", id, block)
	}
	return elem, nil
}

// SetObjectText replaces the `text:` scalar of the object with the given
// id. The replacement is always written double-quoted.
func (d *Doc) SetObjectText(id, newText string) (Edit, error) {
	elem, err := d.objectElem(id)
	if err != nil {
		return Edit{}, err
	}
	textNode := mapGet(elem, "text")
	if textNode == nil {
		return Edit{}, fmt.Errorf("splice: object %q has no text field", id)
	}
	start, end, err := d.span(textNode)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: start, End: end, Replace: quoteYAML(newText)}, nil
}

// AppendDecisionLink appends one typed edge to a decision object's own
// `links:` (02 §Object model: per-decision links for supersedes/exempts).
// It handles all three insertion shapes: no links key at all (the common
// first-yarn case), an empty flow list (S7's disclosed-unproven case,
// proven by this package's tests), and a non-empty list (S7 §3's proven
// append-after-last-element).
func (d *Doc) AppendDecisionLink(dcID string, l artifact.Link) (Edit, error) {
	if !strings.HasPrefix(dcID, "dc-") {
		return Edit{}, fmt.Errorf("splice: %q is not a decision id", dcID)
	}
	elem, err := d.objectElem(dcID)
	if err != nil {
		return Edit{}, err
	}
	entry := formatLink(l)

	linksNode := mapGet(elem, "links")
	if linksNode == nil {
		// First yarn ever on this decision: insert ", links: [ ... ]"
		// immediately after the map's last non-whitespace content byte,
		// before its closing '}'.
		if elem.Style&yaml.FlowStyle == 0 {
			return Edit{}, fmt.Errorf("splice: decision %s is not a flow-style map (block style is unproven — S7); fail closed", dcID)
		}
		start, end, serr := d.span(elem)
		if serr != nil {
			return Edit{}, serr
		}
		at := end - 1 // the '}'
		for at > start && isYAMLSpace(d.src[at-1]) {
			at--
		}
		return Edit{Start: at, End: at, Replace: ", links: [ " + entry + " ]"}, nil
	}
	return d.appendToFlowSeq(linksNode, entry)
}

// findDecisionLink locates one link element in a decision's links: by
// its type plus a caller-supplied ref predicate (the caller owns ref
// normalization — a stored ref may carry a pin a board endpoint drops;
// splice never guesses). Returns the flow-style sequence node, the key
// node, and the matched element's index. Only the flow-style house
// shape AppendDecisionLink writes is proven; anything else fails closed.
func (d *Doc) findDecisionLink(dcID, linkType string, refMatches func(ref string) bool) (key, seq *yaml.Node, idx int, err error) {
	if !strings.HasPrefix(dcID, "dc-") {
		return nil, nil, 0, fmt.Errorf("splice: %q is not a decision id", dcID)
	}
	elem, err := d.objectElem(dcID)
	if err != nil {
		return nil, nil, 0, err
	}
	key, seq = mapKeyValue(elem, "links")
	if seq == nil {
		return nil, nil, 0, fmt.Errorf("splice: decision %s has no links", dcID)
	}
	if seq.Kind != yaml.SequenceNode || seq.Style&yaml.FlowStyle == 0 {
		return nil, nil, 0, fmt.Errorf("splice: decision %s links is not a flow-style sequence (only the house style is proven); fail closed", dcID)
	}
	for i, li := range seq.Content {
		t := mapGet(li, "type")
		r := mapGet(li, "ref")
		if t != nil && r != nil && t.Value == linkType && refMatches(r.Value) {
			return key, seq, i, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("splice: decision %s has no %s link matching the target", dcID, linkType)
}

// RemoveDecisionLink removes one typed edge from a decision's links: —
// the exact inverse of AppendDecisionLink (05 §Workbench: authoring is
// bidirectional; owner UAT round 6, item 3). Removing the sole link
// removes the whole links: key, so append-then-remove restores the
// original buffer byte-for-byte.
func (d *Doc) RemoveDecisionLink(dcID, linkType string, refMatches func(ref string) bool) (Edit, error) {
	key, seq, idx, err := d.findDecisionLink(dcID, linkType, refMatches)
	if err != nil {
		return Edit{}, err
	}

	if len(seq.Content) == 1 {
		// Sole link: remove ", links: [ ... ]" whole — the inverse of the
		// first-yarn insertion.
		keyStart, _, kerr := d.span(key)
		if kerr != nil {
			return Edit{}, kerr
		}
		_, valEnd, verr := d.span(seq)
		if verr != nil {
			return Edit{}, verr
		}
		at := keyStart
		for at > 0 && isYAMLSpace(d.src[at-1]) {
			at--
		}
		if at == 0 || d.src[at-1] != ',' {
			return Edit{}, fmt.Errorf("splice: links key for %s is not comma-separated from a preceding field; fail closed", dcID)
		}
		return Edit{Start: at - 1, End: valEnd}, nil
	}

	elemStart, elemEnd, err := d.span(seq.Content[idx])
	if err != nil {
		return Edit{}, err
	}
	if idx > 0 {
		// Not the first element: remove ", <elem>" back through the
		// previous element's end.
		_, prevEnd, perr := d.span(seq.Content[idx-1])
		if perr != nil {
			return Edit{}, perr
		}
		return Edit{Start: prevEnd, End: elemEnd}, nil
	}
	// First of several: remove "<elem>, " forward to the next element.
	nextStart, _, nerr := d.span(seq.Content[idx+1])
	if nerr != nil {
		return Edit{}, nerr
	}
	return Edit{Start: elemStart, End: nextStart}, nil
}

// RetypeDecisionLink changes one link's type IN PLACE (owner directive,
// round 6 UAT follow-up: the relationship's type is updatable without
// delete-and-redraw): a single edit replacing only the type scalar, so
// the stored ref (pins included) and note survive verbatim and the
// document never passes through a linkless state.
func (d *Doc) RetypeDecisionLink(dcID, oldType string, refMatches func(ref string) bool, newType string) (Edit, error) {
	_, seq, idx, err := d.findDecisionLink(dcID, oldType, refMatches)
	if err != nil {
		return Edit{}, err
	}
	typeNode := mapGet(seq.Content[idx], "type")
	start, end, err := d.span(typeNode)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: start, End: end, Replace: newType}, nil
}

// mapKeyValue returns the key and value nodes for key in a mapping node
// (Content alternates key, value), or nil, nil.
func mapKeyValue(mapNode *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return mapNode.Content[i], mapNode.Content[i+1]
		}
	}
	return nil, nil
}

// appendToFlowSeq inserts entry as the last element of a flow-style
// sequence node.
func (d *Doc) appendToFlowSeq(seq *yaml.Node, entry string) (Edit, error) {
	if seq.Kind != yaml.SequenceNode {
		return Edit{}, fmt.Errorf("splice: links is not a sequence")
	}
	if seq.Style&yaml.FlowStyle == 0 {
		return Edit{}, fmt.Errorf("splice: sequence is not flow-style (block style append uses appendToBlockSeq)")
	}
	start, end, err := d.span(seq)
	if err != nil {
		return Edit{}, err
	}
	if len(seq.Content) == 0 {
		// Empty list: replace the whole "[]"/"[ ]" span with a one-element
		// list — the S7 §3 disclosed case, proven here.
		return Edit{Start: start, End: end, Replace: "[ " + entry + " ]"}, nil
	}
	last := seq.Content[len(seq.Content)-1]
	_, lastEnd, err := d.span(last)
	if err != nil {
		return Edit{}, err
	}
	return Edit{Start: lastEnd, End: lastEnd, Replace: ", " + entry}, nil
}

// AppendObject appends a new object entry to a top-level block
// (acceptance_criteria/constraints/decisions/open_questions), creating
// the block if absent, plus a body section whose heading resolves the new
// object's anchor (02 §Object model: anchor resolution is exact-match).
// evidence applies to acceptance criteria only.
func (d *Doc) AppendObject(id, text string, evidence []artifact.EvidenceKind) ([]Edit, error) {
	block, err := blockForID(id)
	if err != nil {
		return nil, err
	}
	if block == "acceptance_criteria" && len(evidence) == 0 {
		return nil, fmt.Errorf("splice: a new acceptance criterion needs at least one evidence kind (VL-006)")
	}
	entry := formatObjectEntry(block, id, text, evidence)

	var fmEdit Edit
	seq := mapGet(d.fm, block)
	switch {
	case seq == nil:
		// New top-level block, inserted as whole lines immediately before
		// the closing "---" (a line-grained insertion at the frontmatter
		// boundary — no block-style end detection involved).
		fmEdit = Edit{Start: d.fmCloseOffset, End: d.fmCloseOffset, Replace: block + ":\n  - " + entry + "\n"}
	case seq.Style&yaml.FlowStyle != 0:
		fmEdit, err = d.appendToFlowSeq(seq, entry)
		if err != nil {
			return nil, err
		}
	default:
		fmEdit, err = d.appendToBlockSeq(seq, entry)
		if err != nil {
			return nil, err
		}
	}

	// The body section: "## <id>" slugifies to exactly <id>, so the
	// object's "#<id>" anchor resolves (artifact.SlugifyHeading).
	body := "\n## " + id + "\n\n" + text + "\n"
	bodyEdit := Edit{Start: len(d.src), End: len(d.src), Replace: body}

	return []Edit{fmEdit, bodyEdit}, nil
}

// appendToBlockSeq inserts entry as a new "- { ... }" line after the last
// element of a block-style sequence whose elements are flow maps (the 02
// §Object model house style). The new line reuses the last element's own
// line prefix (indentation plus "- ") verbatim.
func (d *Doc) appendToBlockSeq(seq *yaml.Node, entry string) (Edit, error) {
	if len(seq.Content) == 0 {
		return Edit{}, fmt.Errorf("splice: block-style sequence is empty (an empty block sequence cannot exist in YAML; fail closed)")
	}
	last := seq.Content[len(seq.Content)-1]
	lastStart, lastEnd, err := d.span(last)
	if err != nil {
		return Edit{}, err
	}
	// Prefix: from the last element's line start up to the element itself
	// (e.g. "  - ").
	lineStart, err := byteOffset(d.offsets, last.Line+d.lineDelta, 1)
	if err != nil {
		return Edit{}, err
	}
	prefix := string(d.src[lineStart:lastStart])
	if strings.TrimLeft(prefix, " \t") != "- " {
		return Edit{}, fmt.Errorf("splice: block sequence element does not start its own line with a %q marker (got %q); fail closed", "- ", prefix)
	}
	return Edit{Start: lastEnd, End: lastEnd, Replace: "\n" + prefix + entry}, nil
}

// Apply applies edits (all computed against this Doc's pristine buffer)
// tail-to-head and returns the new buffer. The Doc itself is not mutated;
// re-Parse the result for further edits.
func (d *Doc) Apply(edits []Edit) ([]byte, error) {
	return applyEdits(d.src, edits)
}

// Validate is the validate-before-write gate (S7 §5): the spliced result
// must strict-decode as a spec and every object anchor must resolve
// against the body. Callers never write a buffer this rejects.
func Validate(result []byte) error {
	fm, body, err := artifact.SplitFrontmatter(result)
	if err != nil {
		return fmt.Errorf("splice: validate-before-write: %w", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return fmt.Errorf("splice: validate-before-write: %w", err)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		return fmt.Errorf("splice: validate-before-write: %w", err)
	}
	return nil
}

// NextID mints the next free object id for a prefix ("ac", "co", "dc",
// "oq") given every id already in use: the smallest positive integer
// suffix not taken. Deterministic; non-numeric suffixes are ignored.
func NextID(existing []string, prefix string) string {
	used := make(map[int]bool, len(existing))
	for _, id := range existing {
		rest, ok := strings.CutPrefix(id, prefix+"-")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(rest); err == nil && n > 0 {
			used[n] = true
		}
	}
	n := 1
	for used[n] {
		n++
	}
	return fmt.Sprintf("%s-%d", prefix, n)
}

// formatLink renders one links: entry in the 02 §Object model house
// style. The ref is always quoted (a fragment ref carries '#').
func formatLink(l artifact.Link) string {
	s := "{ type: " + string(l.Type) + ", ref: " + quoteYAML(l.Ref)
	if l.Note != "" {
		s += ", note: " + quoteYAML(l.Note)
	}
	return s + " }"
}

// formatObjectEntry renders one object entry in the house style.
func formatObjectEntry(block, id, text string, evidence []artifact.EvidenceKind) string {
	s := "{ id: " + id + ", text: " + quoteYAML(text)
	if block == "acceptance_criteria" {
		kinds := make([]string, len(evidence))
		for i, e := range evidence {
			kinds[i] = string(e)
		}
		s += ", evidence: [" + strings.Join(kinds, ", ") + "]"
	}
	s += ", anchor: " + quoteYAML("#"+id) + " }"
	return s
}

// quoteYAML renders s as a YAML double-quoted scalar. strconv.Quote's
// escape set (\", \\, \n, \t, \xNN, \uNNNN, ...) is a subset of YAML's
// double-quote escapes, so the output is always valid YAML.
func quoteYAML(s string) string {
	return strconv.Quote(s)
}

func isYAMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
