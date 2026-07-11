// The board projection (05 §Workbench "Board as projection", R4):
// generation is a PURE FUNCTION of four inputs — (1) the spec revision's
// parsed object model, (2) layout.json positions, (3) the mutable-zone
// annotation streams, and (4), in review mode, the live MR comment feed.
// Same four inputs, same board: no LLM, no randomness, no wall clock
// anywhere in this file. Nothing here is board-native except position.
package workbench

import (
	"fmt"
	"sort"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/boardlayout"
)

// boardModeKind is the board's mode, keyed by branch state (05
// §Workbench "Two modes"). Accepted specs on main render read-only.
type boardModeKind string

const (
	modeAuthoring boardModeKind = "authoring"
	modeReview    boardModeKind = "review"
	modeReadOnly  boardModeKind = "readonly"
)

// cardView is one object card: a frontmatter-declared object at its
// layout position, with any anchored review stickies riding on it.
type cardView struct {
	ID       string
	Kind     string // data-object-kind value (the boardlayout zone names)
	Text     string
	X, Y     float64
	Anchored []reviewStickyView
}

// refCardView is a reference card — an edge target outside this spec.
type refCardView struct {
	Ref  string
	X, Y float64
}

// edgeView is one yarn element: a declared spec edge or an
// annotation-layer relates thread.
type edgeView struct {
	Type         string `json:"type"`
	From         string `json:"from"`
	To           string `json:"to"`
	Layer        string `json:"layer"` // "spec" | "annotation"
	AnnotationID string `json:"annotationId,omitempty"`
}

// scratchStickyView is one free-floating annotation sticky.
type scratchStickyView struct {
	ID     string
	Type   string
	Body   string
	Author string
	X, Y   float64
}

// reviewStickyView is one MR comment rendered as a review sticky —
// anchored to its object's card, or in the inbox tray (never dropped).
type reviewStickyView struct {
	Anchor   string // resolved object id, or "" for the tray
	Author   string
	Body     string
	Resolved bool
}

// boardProjection is the full render model for one spec's board.
type boardProjection struct {
	Spec     string
	Title    string
	Mode     boardModeKind
	Problem  string
	Outcome  string
	Cards    []cardView
	RefCards []refCardView
	Edges    []edgeView
	Stickies []scratchStickyView
	Tray     []reviewStickyView
	// Notices are disclosed-unavailable banners rendered in the board
	// chrome in EVERY mode (I-1(b)/I-2/M-4): a configured-but-unreachable
	// review feed, or an assumed default branch. Not a projection of the
	// four inputs — a render-time disclosure the loader attaches, so the
	// board never renders as if a skipped input were simply absent
	// (constitution 2/10: silence is never a pass).
	Notices []string
}

// buildProjection computes the deterministic projection of the four
// inputs. comments is nil outside review mode.
func buildProjection(specName string, fm *artifact.SpecFrontmatter, stored map[string]artifact.Position, annotations []*artifact.Annotation, comments []MRComment, mode boardModeKind) (*boardProjection, error) {
	p := &boardProjection{Spec: specName, Title: fm.Title, Mode: mode}
	if fm.Problem != nil {
		p.Problem = fm.Problem.Text
	}
	if fm.Outcome != nil {
		p.Outcome = fm.Outcome.Text
	}

	// (1) The object model, in document order per block.
	type obj struct {
		id, kind, text string
		order          int
	}
	var objects []obj
	declared := map[string]bool{}
	for i, ac := range fm.AcceptanceCriteria {
		objects = append(objects, obj{ac.ID, string(boardlayout.ZoneAC), ac.Text, i})
		declared[ac.ID] = true
	}
	for i, c := range fm.Constraints {
		objects = append(objects, obj{c.ID, string(boardlayout.ZoneConstraint), c.Text, i})
		declared[c.ID] = true
	}
	for i, d := range fm.Decisions {
		objects = append(objects, obj{d.ID, string(boardlayout.ZoneDecision), d.Text, i})
		declared[d.ID] = true
	}
	for i, q := range fm.OpenQuestions {
		objects = append(objects, obj{q.ID, string(boardlayout.ZoneOpenQuestion), q.Text, i})
		declared[q.ID] = true
	}

	// (1b) Declared edges: per-decision links (02 §Object model), plus
	// document-level implements/resolves/exempts/supersedes/depends-on
	// edges, which belong to the spec itself (From "spec").
	for _, d := range fm.Decisions {
		for _, l := range d.Links {
			if !closedEdgeType(l.Type) {
				continue
			}
			p.Edges = append(p.Edges, edgeView{Type: string(l.Type), From: d.ID, To: edgeEndpoint(specName, declared, l.Ref), Layer: "spec"})
		}
	}
	for _, l := range fm.Links {
		if !closedEdgeType(l.Type) {
			continue
		}
		p.Edges = append(p.Edges, edgeView{Type: string(l.Type), From: "spec", To: edgeEndpoint(specName, declared, l.Ref), Layer: "spec"})
	}

	// (3) Annotation streams: this board's free-floating stickies and its
	// untyped relates threads. Graduated records have already become spec
	// content — they no longer render (05 §Workbench: graduation is an
	// ordinary edit; the sticky dies into the document).
	for _, a := range annotations {
		if a.Status == artifact.AnnotationGraduated {
			continue
		}
		switch a.Type {
		case artifact.AnnotationRelates:
			from, okA := relatesEndpoint(specName, declared, a.Target)
			to, okB := relatesEndpoint(specName, declared, a.TargetB)
			if !okA || !okB {
				continue // a thread of some other board/spec
			}
			p.Edges = append(p.Edges, edgeView{Type: "relates", From: from, To: to, Layer: "annotation", AnnotationID: a.ID})
		case artifact.AnnotationReview:
			// Local review mirrors render through the live feed in review
			// mode; the mutable-zone copy is not double-rendered.
		default:
			if a.Board == nil || a.Board.Story != specName {
				continue
			}
			p.Stickies = append(p.Stickies, scratchStickyView{
				ID: a.ID, Type: string(a.Type), Body: a.Body, Author: a.Author, X: a.Board.X, Y: a.Board.Y,
			})
		}
	}

	// (4) The review feed: token-anchored comments ride their object's
	// current card; everything else lands in the inbox tray — never
	// dropped, never silently unattached (05 §Review stickies).
	anchored := map[string][]reviewStickyView{}
	for _, c := range comments {
		token := commentToken(c.Body)
		rs := reviewStickyView{Author: c.Author, Body: c.Body, Resolved: c.Resolved}
		if token != "" && declared[token] {
			rs.Anchor = token
			anchored[token] = append(anchored[token], rs)
			continue
		}
		p.Tray = append(p.Tray, rs)
	}

	// (2) Positions: stored coordinates verbatim, everything else zoned.
	// Reference cards are every external edge endpoint, ordered by ref.
	refSet := map[string]bool{}
	for _, e := range p.Edges {
		for _, end := range []string{e.From, e.To} {
			if end != "spec" && !declared[end] {
				refSet[end] = true
			}
		}
	}
	refs := make([]string, 0, len(refSet))
	for r := range refSet {
		refs = append(refs, r)
	}
	sort.Strings(refs)

	layoutObjs := make([]boardlayout.Object, 0, len(objects)+len(refs))
	for _, o := range objects {
		layoutObjs = append(layoutObjs, boardlayout.Object{Kind: boardlayout.ZoneKind(o.kind), ID: o.id, DocOrder: o.order})
	}
	for i, r := range refs {
		layoutObjs = append(layoutObjs, boardlayout.Object{Kind: boardlayout.ZoneReference, ID: r, DocOrder: i})
	}
	positions, err := boardlayout.Generate(layoutObjs, stored)
	if err != nil {
		return nil, fmt.Errorf("workbench: board layout for %s: %w", specName, err)
	}

	for _, o := range objects {
		pos := positions[o.id]
		p.Cards = append(p.Cards, cardView{ID: o.id, Kind: o.kind, Text: o.text, X: pos.X, Y: pos.Y, Anchored: anchored[o.id]})
	}
	for _, r := range refs {
		pos := positions[r]
		p.RefCards = append(p.RefCards, refCardView{Ref: r, X: pos.X, Y: pos.Y})
	}

	return p, nil
}

// closedEdgeType reports membership in the closed five-value spec-object
// edge vocabulary (02 §Link taxonomy) — the only types that render as
// yarn.
func closedEdgeType(t artifact.LinkType) bool {
	switch t {
	case artifact.LinkImplements, artifact.LinkResolves, artifact.LinkSupersedes, artifact.LinkExempts, artifact.LinkDependsOn:
		return true
	}
	return false
}

// edgeEndpoint maps a link ref to a yarn endpoint: a same-spec fragment
// becomes the bare object id (an internal card); anything else is the
// target ref with any pin dropped (position is derived at render, never
// encoded — the ref names the target, the pin is provenance).
func edgeEndpoint(specName string, declared map[string]bool, ref string) string {
	r, err := artifact.ParseRef(ref)
	if err != nil {
		return ref // renders verbatim as an external reference card
	}
	if r.Kind == artifact.KindSpec && r.Name == specName && r.Object != "" && declared[r.Object] {
		return r.Object
	}
	out := string(r.Kind) + "/" + r.Name
	if r.Object != "" {
		out += "#" + r.Object
	}
	return out
}

// relatesEndpoint maps a relates annotation's target to a yarn endpoint:
// a target naming this spec with a heading selector is that object's
// card; any other artifact target is a reference card. ok=false when the
// target belongs to some other spec's board entirely.
func relatesEndpoint(specName string, declared map[string]bool, t *artifact.Target) (string, bool) {
	if t == nil {
		return "", false
	}
	r, err := artifact.ParseRef(t.Ref)
	if err != nil {
		return "", false
	}
	if r.Kind == artifact.KindSpec && r.Name == specName {
		if t.Selector.Heading != "" && declared[t.Selector.Heading] {
			return t.Selector.Heading, true
		}
		return "", false
	}
	return string(r.Kind) + "/" + r.Name, true
}

// refCardTestID flattens a ref for its data-testid, matching the
// acceptance contract's refCardTestId helper ("/" → "-").
func refCardTestID(ref string) string {
	return "ref-card-" + strings.ReplaceAll(ref, "/", "-")
}
