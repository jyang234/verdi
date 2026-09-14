package specimport

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gmtext "github.com/yuin/goldmark/text"
)

// markdownResult is the automatic structural-recognition output for one
// primary source: every automatically resolved statement/object Field and
// every structural Finding (spec-import-contract.md, "Deterministic
// structural mapping").
type markdownResult struct {
	fields   []Field
	findings []Finding
	// blocked carries the source item behind every
	// source-id-requires-mapping finding, so resolution can be bound to
	// that item rather than to its id alone.
	blocked []blockedSourceID
}

// blockedSourceID is one list item whose leading token declares an
// ac-/co-/dc-/oq- ID, together with the source and the exact item that
// declared it (spec-import-contract.md: "do not silently present a
// generated ID as the source's identity").
type blockedSourceID struct {
	id       string
	sourceID string
	item     bulletItem
}

// objectPrefixes is the fixed processing order for object sections —
// order only affects Finding/Field slice order, never which bytes map to
// which target.
var objectPrefixes = []string{"ac", "co", "dc", "oq"}

var objectSectionDisplayName = map[string]string{
	"ac": "acceptance-criteria",
	"co": "constraints",
	"dc": "decisions",
	"oq": "open-questions",
}

// canonicalHeadingLabel maps a trimmed, case-insensitive heading label to
// its canonical field key (spec-import-contract.md: "Literal heading
// labels are case-insensitive after trimming whitespace: Problem/Problem
// Statement, Outcome/Outcome Statement, Acceptance Criteria, Constraints,
// Decisions, Open Questions. No synonyms beyond these."). Exactly one of
// statementKey/objectPrefix is set when recognized is true.
func canonicalHeadingLabel(label string) (statementKey, objectPrefix string, recognized bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "problem", "problem statement":
		return "problem", "", true
	case "outcome", "outcome statement":
		return "outcome", "", true
	case "acceptance criteria":
		return "", "ac", true
	case "constraints":
		return "", "co", true
	case "decisions":
		return "", "dc", true
	case "open questions":
		return "", "oq", true
	default:
		return "", "", false
	}
}

// recognizeMarkdown structurally recognizes selected (one primary source's
// selected bytes) as markdown-v1: an optional leading YAML frontmatter
// block, an optional run of retained-only leading prose, a title heading,
// and direct-child field-section headings one level deeper
// (spec-import-contract.md, "Deterministic structural mapping").
//
// Every reported Span offset is relative to selected itself (never rebased
// to the post-frontmatter body), per "Maintain offsets into the original
// selected bytes when parsing the remainder".
func recognizeMarkdown(sourceID string, selected []byte) (markdownResult, error) {
	bodyStart, err := stripFrontmatter(selected)
	if err != nil {
		return markdownResult{}, err
	}
	body := selected[bodyStart:]

	md := goldmark.New()
	doc := md.Parser().Parse(gmtext.NewReader(body))

	var top []ast.Node
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		top = append(top, c)
	}

	titleIdx := -1
	for i, n := range top {
		if n.Kind() == ast.KindHeading {
			titleIdx = i
			break
		}
	}
	if titleIdx == -1 {
		return markdownResult{findings: []Finding{{
			Code:     FindingUnsupportedStructure,
			Target:   "primary",
			Message:  "the primary source has no Markdown heading; a title heading is required before field sections can be recognized",
			Blocking: true,
		}}}, nil
	}
	titleHeading := top[titleIdx].(*ast.Heading)
	titleLevel := titleHeading.Level

	statementOccurrences := map[string][]int{}
	objectOccurrences := map[string][]int{}
	var findings []Finding

	for i := titleIdx + 1; i < len(top); i++ {
		h, isHeading := top[i].(*ast.Heading)
		if !isHeading {
			continue
		}
		if h.Level <= titleLevel {
			label := headingLabel(h, body)
			findings = append(findings, Finding{
				Code:     FindingMultipleTargets,
				Target:   label,
				Message:  fmt.Sprintf("heading %q is a peer or higher-level heading after the title; a primary selection must resolve to exactly one target", label),
				Blocking: true,
			})
			break
		}
		if h.Level != titleLevel+1 {
			continue // deeper than a direct child: not a section boundary.
		}
		label := headingLabel(h, body)
		stKey, objPrefix, ok := canonicalHeadingLabel(label)
		if !ok {
			continue // an unrecognized field-level heading: left unmapped.
		}
		if stKey != "" {
			statementOccurrences[stKey] = append(statementOccurrences[stKey], i)
		} else {
			objectOccurrences[objPrefix] = append(objectOccurrences[objPrefix], i)
		}
	}

	var fields []Field
	var blocked []blockedSourceID
	for _, key := range []string{"problem", "outcome"} {
		occ := statementOccurrences[key]
		switch len(occ) {
		case 0:
			findings = append(findings, Finding{
				Code:     FindingMissingStatement,
				Target:   key,
				Message:  fmt.Sprintf("no %s section was found in the primary source", key),
				Blocking: true,
			})
		case 1:
			field, finding := extractStatementField(sourceID, body, bodyStart, top, occ[0], key)
			if finding != nil {
				findings = append(findings, *finding)
			} else {
				fields = append(fields, field)
			}
		default:
			findings = append(findings, Finding{
				Code:     FindingAmbiguousField,
				Target:   key,
				Message:  fmt.Sprintf("%d headings alias to the %s field; a single unambiguous heading is required", len(occ), key),
				Blocking: true,
			})
		}
	}

	for _, prefix := range objectPrefixes {
		occ := objectOccurrences[prefix]
		name := objectSectionDisplayName[prefix]
		switch len(occ) {
		case 0:
			// No such section at all: nothing to report or extract.
		case 1:
			objFields, finding := extractObjectSection(sourceID, body, bodyStart, top, occ[0], prefix)
			if finding != nil {
				findings = append(findings, *finding)
			} else {
				// One entry per DECLARING ITEM, not per id: two items can
				// declare the same id, and each needs its own disclosure.
				// unresolvedSourceIDFindings turns these into findings once
				// the request's explicit mappings are known.
				blocked = append(blocked, blockedSourceIDs(sourceID, body, bodyStart, top, occ[0])...)
			}
			fields = append(fields, objFields...)
		default:
			findings = append(findings, Finding{
				Code:     FindingAmbiguousField,
				Target:   name,
				Message:  fmt.Sprintf("%d headings alias to the %s section; a single unambiguous heading is required", len(occ), name),
				Blocking: true,
			})
		}
	}

	return markdownResult{fields: fields, findings: findings, blocked: blocked}, nil
}

// headingLabel returns a heading's trimmed label text. For an ATX heading
// this is the text after the leading '#'s (goldmark already strips the
// marker and trailing '#'s); for a setext heading it is the underlined
// paragraph's text — see atx_heading.go/setext_headings.go: both forms
// populate exactly the label bytes on Lines(), never the marker/bar.
func headingLabel(h *ast.Heading, source []byte) string {
	return strings.TrimSpace(string(h.Lines().Value(source)))
}

// sectionContentNodes returns the top-level sibling nodes strictly between
// top[headingIdx] and the heading that actually CLOSES that section — the
// next heading at top[headingIdx]'s own level or higher — plus that
// heading's index (len(top) if none).
//
// A deeper subheading does not close a section: the contract recognizes
// field sections at exactly one level below the title and then requires
// "the entire section body... preserving all interior bytes and line
// breaks", so an interior "### Detail" is content. Treating it as a
// boundary silently truncated the recognized statement and dropped the
// remainder into retained-only with no finding. For an object section the
// same rule makes that subheading mixed non-list content, which
// extractObjectSection reports as unresolved rather than truncating.
func sectionContentNodes(top []ast.Node, headingIdx int) (content []ast.Node, nextHeadingIdx int) {
	level := 0
	if h, ok := top[headingIdx].(*ast.Heading); ok {
		level = h.Level
	}
	nextHeadingIdx = len(top)
	for j := headingIdx + 1; j < len(top); j++ {
		if h, ok := top[j].(*ast.Heading); ok && h.Level <= level {
			nextHeadingIdx = j
			break
		}
		content = append(content, top[j])
	}
	return content, nextHeadingIdx
}

// extractStatementField resolves one problem/outcome section: its body is
// the entire section content excluding leading/trailing blank lines,
// preserving all interior bytes and line breaks exactly
// (spec-import-contract.md). A section with no content block at all is
// empty-field, not a zero-length Field.
func extractStatementField(sourceID string, body []byte, bodyStart int, top []ast.Node, headingIdx int, key string) (Field, *Finding) {
	content, nextHeadingIdx := sectionContentNodes(top, headingIdx)
	if len(content) == 0 {
		return Field{}, &Finding{
			Code:     FindingEmptyField,
			Target:   key,
			Message:  fmt.Sprintf("the %s section has no content", key),
			Blocking: true,
		}
	}
	rawStart, ok := blockLineStart(body, content[0])
	if !ok {
		return Field{}, &Finding{
			Code:     FindingUnsupportedStructure,
			Target:   key,
			Message:  fmt.Sprintf("the %s section's first content block has no locatable source position; its exact body bytes cannot be determined", key),
			Blocking: true,
		}
	}
	rawEnd := len(body)
	if nextHeadingIdx < len(top) {
		end, ok := blockLineStart(body, top[nextHeadingIdx])
		if !ok {
			return Field{}, &Finding{
				Code:     FindingUnsupportedStructure,
				Target:   key,
				Message:  fmt.Sprintf("the heading closing the %s section has no locatable source position; its exact body bytes cannot be determined", key),
				Blocking: true,
			}
		}
		rawEnd = end
	}
	start, end := trimBodyRange(body, rawStart, rawEnd)
	if start >= end {
		return Field{}, &Finding{
			Code:     FindingEmptyField,
			Target:   key,
			Message:  fmt.Sprintf("the %s section has no content once blank lines are trimmed", key),
			Blocking: true,
		}
	}
	return Field{
		Target: key,
		Text:   string(body[start:end]),
		Origin: OriginCopiedSource,
		Spans: []Span{{
			SourceID:  sourceID,
			Start:     bodyStart + start,
			End:       bodyStart + end,
			Transform: TransformTrimBlankLines,
		}},
	}, nil
}

// sourceDeclaredIDRe recognizes a list item whose leading token is an
// ac-/co-/dc-/oq- id followed by a colon (spec-import-contract.md: "When a
// list item's leading token has an ac-/co-/dc-/oq- ID followed by a colon,
// emit source-id-requires-mapping"). It is independently derived from the
// contract's own prose, matching mappingTargetIDRe's four-kind shape plus
// a trailing colon, not a copy of any internal/artifact regex.
var sourceDeclaredIDRe = regexp.MustCompile(`^((?:ac|co|dc|oq)-[a-z0-9]+(?:-[a-z0-9]+)*):`)

// extractObjectSection resolves one object section (Acceptance Criteria/
// Constraints/Decisions/Open Questions) into sequential Fields. It is
// "resolved" only when the section's sole content node is a flat list with
// no nested list anywhere inside it (spec-import-contract.md: "Nested
// lists or mixed non-list prose make that section unresolved"); this
// package reports that condition as ambiguous-field (see the lane report's
// semantic-decisions section for the alternative considered).
//
// Each direct list item becomes one object, numbered in source order
// regardless of whether an earlier item was blocked by a source-declared
// id (spec-import-contract.md residual 4: "the item's ordinal is still
// counted, so following generated IDs keep their source-position
// numbering").
func extractObjectSection(sourceID string, body []byte, bodyStart int, top []ast.Node, headingIdx int, prefix string) ([]Field, *Finding) {
	name := objectSectionDisplayName[prefix]
	content, _ := sectionContentNodes(top, headingIdx)
	list, ok := sectionBulletList(content)
	if !ok {
		return nil, &Finding{
			Code:     FindingAmbiguousField,
			Target:   name,
			Message:  fmt.Sprintf("the %s section must contain exactly one flat Markdown bullet list and no other content; an ordered list, mixed prose or an interior subheading is not that grammar", name),
			Blocking: true,
		}
	}
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		if containsNestedList(item) {
			return nil, &Finding{
				Code:     FindingAmbiguousField,
				Target:   name,
				Message:  fmt.Sprintf("the %s list contains a nested list; only a flat list of direct items is supported", name),
				Blocking: true,
			}
		}
	}

	var fields []Field
	for _, item := range bulletItemsOf(body, bodyStart, list) {
		if !item.supported {
			continue // an empty list item contributes nothing.
		}
		if sourceDeclaredIDRe.MatchString(item.text) {
			// Blocked: bytes stay in the ordinary unmapped complement; no
			// automatic field is generated for this ordinal, but the
			// ordinal is still consumed.
			continue
		}
		fields = append(fields, Field{
			Target: fmt.Sprintf("%s-%d", prefix, item.ordinal),
			Text:   item.text,
			Origin: OriginCopiedSource,
			Spans: []Span{{
				SourceID:  sourceID,
				Start:     item.start,
				End:       item.end,
				Transform: TransformListItem,
			}},
		})
	}
	return fields, nil
}

// blockedSourceIDs returns one blockedSourceID per blocked list item in one
// object section, computed as a second, lightweight pass so
// extractObjectSection's happy path does not need to thread an extra return
// value through its early-return branches. It records the declaring item
// itself, not just its id, so resolution can require a mapping over THAT
// item in THAT source.
func blockedSourceIDs(sourceID string, body []byte, bodyStart int, top []ast.Node, headingIdx int) []blockedSourceID {
	content, _ := sectionContentNodes(top, headingIdx)
	list, ok := sectionBulletList(content)
	if !ok {
		return nil
	}
	var blocked []blockedSourceID
	for _, item := range bulletItemsOf(body, bodyStart, list) {
		if !item.supported {
			continue
		}
		if m := sourceDeclaredIDRe.FindStringSubmatch(item.text); m != nil {
			blocked = append(blocked, blockedSourceID{id: m[1], sourceID: sourceID, item: item})
		}
	}
	return blocked
}

// sectionBulletList returns an object section's single flat Markdown BULLET
// list, if that is exactly what the section contains (spec-import-
// contract.md: "Object sections accept a flat Markdown bullet list: one
// direct item = one object... Strip only the bullet marker and its
// following space"; "Nested lists or mixed non-list prose make that section
// unresolved").
//
// An ordered list is not a bullet list and is deliberately refused rather
// than promoted: its "1." markers are not the bullet marker the declared
// transform strips, so accepting it would present items under a grammar
// this package does not implement.
func sectionBulletList(content []ast.Node) (ast.Node, bool) {
	if len(content) != 1 {
		return nil, false
	}
	return bulletList(content[0])
}

// bulletList returns n as a flat Markdown bullet list, if that is what it
// is. An ordered list is deliberately not one: its "1." markers are not the
// bullet marker the declared list-item transform strips.
func bulletList(n ast.Node) (ast.Node, bool) {
	list, ok := n.(*ast.List)
	if !ok || list.IsOrdered() {
		return nil, false
	}
	return list, true
}

// containsNestedList reports whether any descendant of n (inclusive of n
// itself) is a List node.
func containsNestedList(n ast.Node) bool {
	if n.Kind() == ast.KindList {
		return true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if containsNestedList(c) {
			return true
		}
	}
	return false
}

// lineStartBefore returns the byte offset of the start of the physical
// line containing pos: pos itself, backed up past any preceding bytes
// that are not a line terminator. Used to turn an ATX heading's LABEL
// start (past its "## " marker) back into that heading's own LINE start,
// so a section's raw end boundary never mistakes the marker bytes of the
// next heading for trailing body content.
func lineStartBefore(data []byte, pos int) int {
	for pos > 0 && data[pos-1] != '\n' {
		pos--
	}
	return pos
}

// blockLineStart returns the byte offset of the start of the physical line
// on which block n begins.
//
// It reads goldmark's OWN recorded block position (parser.go sets it for
// every block it opens), which exists even for blocks that carry no line
// segment at all — a thematic break, or an ATX heading with an empty label.
// A section extent must never be derived from a descendant leaf's segment:
// an unsegmented block has none, so such a boundary silently collapses to
// offset 0 and the section absorbs everything before it, including the
// document title and its own heading marker, or collapses to nothing.
// Paragraph/TextBlock override Pos() with their first line's start, which
// lineStartBefore then backs up past any indentation; a setext heading's
// position is its LABEL line, not its underline, so it is the correct
// closing boundary for the preceding section either way.
func blockLineStart(body []byte, n ast.Node) (int, bool) {
	pos := n.Pos()
	if pos < 0 {
		segs := leafSegments(n)
		if len(segs) == 0 {
			return 0, false
		}
		pos = segs[0].Start
	}
	if pos < 0 || pos > len(body) {
		return 0, false
	}
	return lineStartBefore(body, pos), true
}

// leafSegments collects every line segment of every Lines()-bearing
// descendant of n, in document order (depth-first, left to right). A
// Heading is deliberately still "Lines()-bearing" by this definition, so
// callers must never invoke this on a heading when computing body content
// — recognizeMarkdown only calls it on already-heading-filtered content.
func leafSegments(n ast.Node) []gmtext.Segment {
	var segs []gmtext.Segment
	if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
		for i := 0; i < n.Lines().Len(); i++ {
			segs = append(segs, n.Lines().At(i))
		}
		return segs
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		segs = append(segs, leafSegments(c)...)
	}
	return segs
}

// trimBodyRange narrows the raw candidate window [start,end) of data to
// exclude leading/trailing blank physical lines and the final remaining
// line's own trailing terminator (spec-import-contract.md: "Problem/
// outcome select the entire section body excluding leading/trailing blank
// lines, preserving all interior bytes and line breaks"). It operates
// purely on raw bytes — never on AST node extents — so it needs no
// assumption about whether a particular node kind's Lines() segment
// includes its own trailing newline, and it correctly treats a fenced
// code block's closing delimiter line as ordinary non-blank content
// (see TestNormalize_MarkdownHeadingInsideFencedBlockIsNotAField).
func trimBodyRange(data []byte, start, end int) (int, int) {
	for end > start {
		// Consume this line's own trailing terminator first (CRLF or LF)
		// so lineStart's backward scan always lands strictly before end
		// and every iteration decreases end by at least one byte — never
		// re-examining the same empty [end,end) window forever.
		lineEnd := end
		if data[lineEnd-1] == '\n' {
			lineEnd--
			if lineEnd > start && data[lineEnd-1] == '\r' {
				lineEnd--
			}
		}
		lineStart := lineEnd
		for lineStart > start && data[lineStart-1] != '\n' {
			lineStart--
		}
		line := data[lineStart:lineEnd]
		if len(bytes.TrimSpace(line)) == 0 {
			end = lineStart
			continue
		}
		end = lineEnd
		break
	}
	for start < end {
		lineEnd := start
		for lineEnd < end && data[lineEnd] != '\n' {
			lineEnd++
		}
		includesNL := lineEnd < end
		if includesNL {
			lineEnd++
		}
		line := data[start:lineEnd]
		if len(bytes.TrimSpace(line)) != 0 {
			break
		}
		start = lineEnd
	}
	return start, end
}
