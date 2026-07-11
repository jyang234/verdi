package specsplice

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
		return nil, fmt.Errorf("specsplice: document does not start with a %q frontmatter delimiter", "---")
	}
	closeLine := -1
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimRight(lines[i], "\r")) == "---" {
			closeLine = i + 1 // 1-indexed
			break
		}
	}
	if closeLine == -1 {
		return nil, fmt.Errorf("specsplice: no closing %q frontmatter delimiter found", "---")
	}

	fmStart := offsets[2] // first byte of line 2
	fmEnd := offsets[closeLine]
	fmText := src[fmStart:fmEnd]

	var root yaml.Node
	if err := yaml.Unmarshal(fmText, &root); err != nil {
		return nil, fmt.Errorf("specsplice: parsing frontmatter: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("specsplice: frontmatter is not a single YAML mapping")
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
		return "", fmt.Errorf("specsplice: object id %q has no known block prefix (ac-/co-/dc-/oq-)", id)
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
		return nil, fmt.Errorf("specsplice: spec has no %s block", block)
	}
	elem := seqFindByID(seq, id)
	if elem == nil {
		return nil, fmt.Errorf("specsplice: no object %q in %s", id, block)
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
		return Edit{}, fmt.Errorf("specsplice: object %q has no text field", id)
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
		return Edit{}, fmt.Errorf("specsplice: %q is not a decision id", dcID)
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
			return Edit{}, fmt.Errorf("specsplice: decision %s is not a flow-style map (block style is unproven — S7); fail closed", dcID)
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

// appendToFlowSeq inserts entry as the last element of a flow-style
// sequence node.
func (d *Doc) appendToFlowSeq(seq *yaml.Node, entry string) (Edit, error) {
	if seq.Kind != yaml.SequenceNode {
		return Edit{}, fmt.Errorf("specsplice: links is not a sequence")
	}
	if seq.Style&yaml.FlowStyle == 0 {
		return Edit{}, fmt.Errorf("specsplice: sequence is not flow-style (block style append uses appendToBlockSeq)")
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
		return nil, fmt.Errorf("specsplice: a new acceptance criterion needs at least one evidence kind (VL-006)")
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
		return Edit{}, fmt.Errorf("specsplice: block-style sequence is empty (an empty block sequence cannot exist in YAML; fail closed)")
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
		return Edit{}, fmt.Errorf("specsplice: block sequence element does not start its own line with a %q marker (got %q); fail closed", "- ", prefix)
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
		return fmt.Errorf("specsplice: validate-before-write: %w", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return fmt.Errorf("specsplice: validate-before-write: %w", err)
	}
	if err := spec.ResolveObjectAnchors(body); err != nil {
		return fmt.Errorf("specsplice: validate-before-write: %w", err)
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
