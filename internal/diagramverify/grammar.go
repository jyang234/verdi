package diagramverify

import (
	"regexp"
	"strings"
)

// Coverage is the whole-artifact verdict Parse returns (spec/
// verification-extractor dc-1: "coverage is an artifact-wide, not a
// line-by-line, verdict") — exactly one value per Parse call, never a
// per-element slice.
type Coverage string

const (
	// CoverageFull requires every declared line to parse within the
	// grammar below AND every extracted node to normalize to an
	// unambiguous truth-comparable identity (dc-1/dc-2).
	CoverageFull Coverage = "full"
	// CoveragePartial is the downgrade: at least one line fell outside
	// the declared grammar, or at least one extracted node's raw id
	// collided across more than one truth node's ShortName.
	CoveragePartial Coverage = "partial"
)

// Node is one extracted flowchart node. RawID is the mermaid source's
// literal node-id token — the identity space every downstream comparison
// (compare.go, AC-3) keys on directly, since a proposal's mermaid body
// never contains a full FQN (dc-2: normalization compares the raw id
// against ShortName(fqn), never the reverse). Ambiguous marks a RawID that
// collided with more than one truth node's ShortName: this node is
// excluded from the three-way comparison (dc-1) and forces the whole
// artifact's Coverage to CoveragePartial.
type Node struct {
	RawID     string
	Ambiguous bool
}

// Edge is one extracted flowchart edge. Only the ordered (From, To) raw-id
// pair is identity (dc-1) — arrow style and label text are recognized (so
// the line counts toward grammar coverage) but discarded, never part of
// identity.
type Edge struct {
	From string
	To   string
}

// Extraction is Parse's whole-artifact result: one Coverage verdict plus
// the best-effort extracted node/edge set — dc-1: "the extractor still
// attempts a best-effort parse of the recognized lines for disclosure"
// even when the overall verdict is partial.
type Extraction struct {
	Coverage Coverage
	Nodes    []Node
	Edges    []Edge
}

// nodeIDPattern is the mermaid node-id token grammar this parser accepts:
// a leading letter or underscore, then letters/digits/underscores —
// render.SanitizeID's own id alphabet in verdi-go's mermaid renderer, read
// here only as external evidence of the emitted vocabulary (CLAUDE.md:
// never import verdi-go packages; this is a independently-declared,
// independently-tested regex, not a reimplementation).
const nodeIDPattern = `[A-Za-z_][A-Za-z0-9_]*`

var (
	directionRe = regexp.MustCompile(`^(flowchart|graph)\s+\S+$`)
	commentRe   = regexp.MustCompile(`^%%`)
	classDefRe  = regexp.MustCompile(`^classDef\s+\S+`)
	bareNodeRe  = regexp.MustCompile(`^(` + nodeIDPattern + `)$`)
)

// reservedKeyword excludes mermaid's own block keywords from matching the
// bare-id node-declaration form: "end" and "subgraph" close/open a
// subgraph block, which dc-1 explicitly keeps OUTSIDE the declared
// grammar. Without this, a standalone "end" line (the closer of an
// out-of-grammar subgraph block) would be silently mis-parsed as a bare
// node named "end" instead of falling through as an unrecognized
// construct — the exact silent-mis-parse dc-1 forbids. A real generated
// or hand-authored flowchart declaring a node literally named "end" is not
// a case this grammar claims (mermaid itself reserves the keyword the
// same way).
var reservedKeyword = map[string]bool{"end": true, "subgraph": true}

var (
	edgeLabeledRe       = regexp.MustCompile(`^(` + nodeIDPattern + `)\s*-->\|([^|]*)\|\s*(` + nodeIDPattern + `)$`)
	edgePlainRe         = regexp.MustCompile(`^(` + nodeIDPattern + `)\s*-->\s*(` + nodeIDPattern + `)$`)
	edgeDashedLabeledRe = regexp.MustCompile(`^(` + nodeIDPattern + `)\s*-\.\s+(.*?)\s+\.->\s*(` + nodeIDPattern + `)$`)
	edgeDashedRe        = regexp.MustCompile(`^(` + nodeIDPattern + `)\s*-\.->\s*(` + nodeIDPattern + `)$`)
)

// shapeDelims are the shape-bracket pairs the declared grammar recognizes
// around a node's quoted label (dc-1: "shape-delimited") — exactly the
// four pairs verdi-go's graphio/mermaid.go renderer emits: rectangle
// (first-party/fallible nodes), cylinder (db), hexagon (bus), stadium
// (external/blind-spot disclosure nodes). Order matters only in that each
// entry is tried as a whole-line anchored match, so "([" and "[(" never
// get confused with the bare "[" they both start with.
var shapeDelims = []struct{ open, close string }{
	{"{{", "}}"},
	{"([", "])"},
	{"[(", ")]"},
	{"[", "]"},
}

// nodeDeclRes holds one fully-anchored regex per shapeDelims entry,
// compiled once: `^<id><open>"<label>"<close>(:::<classname>)?$`. The
// trailing :::classname group is the node-class assignment flowmap's own
// renderer emits for fallible/db/bus/external/blind nodes (dc-1) —
// presentation only, never part of identity.
var nodeDeclRes = buildNodeDeclRes()

func buildNodeDeclRes() []*regexp.Regexp {
	res := make([]*regexp.Regexp, len(shapeDelims))
	for i, d := range shapeDelims {
		res[i] = regexp.MustCompile(`^(` + nodeIDPattern + `)` +
			regexp.QuoteMeta(d.open) + `"([^"]*)"` + regexp.QuoteMeta(d.close) +
			`(:::` + nodeIDPattern + `)?$`)
	}
	return res
}

// Parse extracts source — a proposal diagram's mermaid body — against the
// declared grammar subset (dc-1), normalizing every extracted node's raw
// id against truthFQNs' ShortName space (dc-2). It never returns an error:
// an out-of-grammar line or an ambiguous identity downgrades Coverage to
// CoveragePartial rather than blocking (parent spec/diagram-proposals
// dc-4: "verification never blocks"); the only failure mode this function
// has is a lower disclosed coverage.
func Parse(source string, truthFQNs []string) *Extraction {
	idx := shortNameIndex(truthFQNs)
	ext := &Extraction{Coverage: CoverageFull}
	seen := map[string]int{} // RawID -> index into ext.Nodes, for de-dup
	grammarClean := true

	addNode := func(id string) {
		if _, ok := seen[id]; ok {
			return
		}
		n := Node{RawID: id, Ambiguous: len(idx[id]) > 1}
		seen[id] = len(ext.Nodes)
		ext.Nodes = append(ext.Nodes, n)
	}

	for _, raw := range strings.Split(source, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			// A blank line carries no construct at all — not "a
			// construct outside the grammar" (dc-1), just nothing. A
			// pristine flowmap render never emits one, so this never
			// masks a real gap; it only lets a hand-authored proposal's
			// stylistic spacing avoid an unearned downgrade.
			continue
		}
		switch {
		case directionRe.MatchString(line), commentRe.MatchString(line), classDefRe.MatchString(line):
			continue
		}
		if id, ok := parseNodeDecl(line); ok {
			addNode(id)
			continue
		}
		if from, to, ok := parseEdge(line); ok {
			addNode(from)
			addNode(to)
			ext.Edges = append(ext.Edges, Edge{From: from, To: to})
			continue
		}
		grammarClean = false
	}

	if !grammarClean {
		ext.Coverage = CoveragePartial
	}
	for _, n := range ext.Nodes {
		if n.Ambiguous {
			ext.Coverage = CoveragePartial
		}
	}
	return ext
}

// parseNodeDecl recognizes the bare-id and shape-delimited-quoted-label
// (optionally :::classname) node-declaration forms (dc-1).
func parseNodeDecl(line string) (id string, ok bool) {
	if bareNodeRe.MatchString(line) && !reservedKeyword[line] {
		return line, true
	}
	for _, re := range nodeDeclRes {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// parseEdge recognizes the four declared edge forms (dc-1): `id --> id`,
// `id -->|label| id`, `id -.-> id`, `id -. label .-> id`. Arrow style and
// label text are matched (so the line counts as in-grammar) but discarded
// — only the ordered (from, to) pair is returned.
func parseEdge(line string) (from, to string, ok bool) {
	if m := edgeLabeledRe.FindStringSubmatch(line); m != nil {
		return m[1], m[3], true
	}
	if m := edgePlainRe.FindStringSubmatch(line); m != nil {
		return m[1], m[2], true
	}
	if m := edgeDashedLabeledRe.FindStringSubmatch(line); m != nil {
		return m[1], m[3], true
	}
	if m := edgeDashedRe.FindStringSubmatch(line); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}
