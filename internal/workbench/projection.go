// The board projection (05 §Workbench "Board as projection", R4):
// generation is a PURE FUNCTION of four inputs — (1) the spec revision's
// parsed object model AND, for the problem/outcome attributes, the body
// prose their anchor resolves to (R4 board polish's placard body seam,
// attributebody.go), (2) layout.json positions, (3) the mutable-zone
// annotation streams, and (4), in review mode, the live MR comment feed.
// Same four inputs, same board: no LLM, no randomness, no wall clock
// anywhere in this file. Nothing here is board-native except position.
package workbench

import (
	"fmt"
	"html/template"
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
// layout position, with any anchored review stickies riding on it. JSON
// tags are load-bearing beyond the board's own HTML template (which
// accesses fields by name, tags irrelevant there): get_board
// (internal/mcpserve) re-marshals this exact struct as its tool result, so
// the tags ARE the wire contract for the machine read surface (05 §MCP
// server's get_board row).
type cardView struct {
	ID       string             `json:"id"`
	Kind     string             `json:"kind"` // data-object-kind value (the boardlayout zone names)
	Text     string             `json:"text"`
	X        float64            `json:"x"`
	Y        float64            `json:"y"`
	Anchored []reviewStickyView `json:"anchored,omitempty"`
}

// refCardView is a reference card — an edge target outside this spec,
// or a pinned reference (type pin, 02 §Record schemas: planning
// material on the wall before any edge exists). One card per ref, ever:
// a ref both pinned and edge-derived renders once, and while the pin
// record lives its stored board position wins.
type refCardView struct {
	Ref    string  `json:"ref"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Pinned bool    `json:"pinned,omitempty"`
	PinID  string  `json:"pinId,omitempty"`
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
	ID     string  `json:"id"`
	Type   string  `json:"type"`
	Body   string  `json:"body"`
	Author string  `json:"author"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// reviewStickyView is one MR comment rendered as a review sticky —
// anchored to its object's card, or in the inbox tray (never dropped).
type reviewStickyView struct {
	Anchor   string `json:"anchor,omitempty"` // resolved object id, or "" for the tray
	Author   string `json:"author"`
	Body     string `json:"body"`
	Resolved bool   `json:"resolved"`
}

// StubView is one declared stub entry, projected verbatim from a feature
// spec's `stubs:` frontmatter (spec/scoping-canvas ac-3: "declared stubs
// render on the wall as first-class scoping cards"). A pure copy of
// artifact.Stub's fields under the projection's own JSON contract (the
// wire shape get_board and the board's own client share, mirroring
// cardView's convention), plus the card's board position — the zone's
// computed lane slot absent a stored one, or the stored `stub:<slug>`
// position verbatim when layout.json holds one (round 5.5 dc-6 amendment:
// a stub is draggable now, exactly like an object card, just keyed in its
// own `stub:` namespace since a stub is not an object, VL-018).
type StubView struct {
	Slug               string   `json:"slug"`
	Spike              bool     `json:"spike,omitempty"`
	Resolves           []string `json:"resolves,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	X                  float64  `json:"x"`
	Y                  float64  `json:"y"`
}

// BoardProjection is the full render model for one spec's board — the
// element taxonomy (05 §Workbench), computed once and consumed by both the
// HTML board (boardspecrender.go, by field access) and get_board
// (internal/mcpserve, by JSON marshaling this struct directly: one
// computation, two presentations, never a reimplementation).
type BoardProjection struct {
	Spec  string        `json:"spec"`
	Title string        `json:"title"`
	Mode  boardModeKind `json:"mode"`
	// Status is the spec's own frontmatter status (02 §Kind registry) —
	// additive (R4 board polish, spec/scoping-canvas): stub-instantiate
	// gates on a feature wall's status being accepted-pending-build, which
	// Mode alone cannot distinguish from any other read-only reason (an
	// authoring board's status is always "draft", by Mode's own
	// construction, so Status only adds information off the authoring
	// path).
	Status string `json:"status,omitempty"`
	// Class identity (02 §Kind registry): which kind of wall this is —
	// a feature (outcome ACs + stubs, downward-blind) or a story (the
	// unit of build, pointing up at its feature's AC fragments), with
	// the story's tracker ref and spike flag carried alongside so both
	// presentations can stamp it (additive fields, R4 board polish).
	Class    string `json:"class"`
	StoryRef string `json:"story_ref,omitempty"`
	Spike    bool   `json:"spike,omitempty"`
	Problem  string `json:"problem,omitempty"`
	Outcome  string `json:"outcome,omitempty"`
	// ProblemBodyHTML and OutcomeBodyHTML are the fuller authored argument
	// under the spec body's "## Problem"/"## Outcome" heading the
	// attribute's own anchor resolves to (02 §Object model) — the body
	// prose behind Problem/Outcome's concise headline above, rendered
	// through the SAME render.RenderMarkdown path the corpus artifact page
	// uses (attributebody.go): never a second markdown implementation.
	// Empty when the attribute is nil, carries no anchor, the anchor
	// resolves to no heading, or the section is blank — fail-soft, never
	// an error: this is read-only reference content a follow-on
	// click-to-read-full-prose pass renders, falling back to the headline
	// text above when absent.
	ProblemBodyHTML template.HTML       `json:"problem_body_html,omitempty"`
	OutcomeBodyHTML template.HTML       `json:"outcome_body_html,omitempty"`
	Cards           []cardView          `json:"cards"`
	RefCards        []refCardView       `json:"ref_cards"`
	Edges           []edgeView          `json:"edges"`
	Stickies        []scratchStickyView `json:"stickies"`
	Tray            []reviewStickyView  `json:"tray"`
	// StubViews, ACCoverage, and OQClaims are the scoping canvas's
	// additive projection (spec/scoping-canvas ac-3/ac-4/ac-5): declared
	// stubs verbatim, each AC's covering-stub count ("covered by N
	// stubs", 0 meaning "no stub"), and each open question's resolving-
	// spike-stub count (0 = unclaimed, >1 = the multi-claim smell — a
	// soft observation, never a rule, ac-5). Pure functions of the
	// frontmatter alone (co-2: "no LLM, no position, no inference from
	// proximity"); keyed maps are populated for every declared AC/OQ, so
	// an absent key never has to be told apart from a real zero.
	StubViews  []StubView     `json:"stub_views,omitempty"`
	ACCoverage map[string]int `json:"ac_coverage,omitempty"`
	OQClaims   map[string]int `json:"oq_claims,omitempty"`
	// Notices are disclosed-unavailable banners rendered in the board
	// chrome in EVERY mode (I-1(b)/I-2/M-4): a configured-but-unreachable
	// review feed, or an assumed default branch. Not a projection of the
	// four inputs — a render-time disclosure the loader attaches, so the
	// board never renders as if a skipped input were simply absent
	// (constitution 2/10: silence is never a pass).
	Notices []string `json:"notices,omitempty"`
}

// buildProjection computes the deterministic projection of the four
// inputs. comments is nil outside review mode. body is the spec
// document's markdown body (post-frontmatter) — used ONLY to resolve the
// problem/outcome attributes' anchors to their fuller body-section prose
// (attributebody.go); every other field is computed from fm alone,
// exactly as before. nil is a legitimate body (no anchor ever resolves,
// so ProblemBodyHTML/OutcomeBodyHTML both stay empty) — every caller that
// builds a projection from a bare in-memory SpecFrontmatter literal
// (rather than a parsed document) passes nil.
func buildProjection(specName string, fm *artifact.SpecFrontmatter, body []byte, stored map[string]artifact.Position, annotations []*artifact.Annotation, comments []MRComment, mode boardModeKind) (*BoardProjection, error) {
	p := &BoardProjection{
		Spec: specName, Title: fm.Title, Mode: mode, Status: string(fm.Status),
		Class: string(fm.Class), StoryRef: fm.Story, Spike: fm.Spike,
	}
	if fm.Problem != nil {
		p.Problem = fm.Problem.Text
	}
	if fm.Outcome != nil {
		p.Outcome = fm.Outcome.Text
	}
	p.ProblemBodyHTML = attributeBodyHTML(body, fm.Problem)
	p.OutcomeBodyHTML = attributeBodyHTML(body, fm.Outcome)

	// StubViews/ACCoverage/OQClaims: the scoping canvas's pure-frontmatter
	// projection (co-2). Keyed for every declared AC/OQ up front so "no
	// stub"/"unclaimed" is an explicit 0, not a missing key.
	p.ACCoverage = make(map[string]int, len(fm.AcceptanceCriteria))
	for _, ac := range fm.AcceptanceCriteria {
		p.ACCoverage[ac.ID] = 0
	}
	p.OQClaims = make(map[string]int, len(fm.OpenQuestions))
	for _, q := range fm.OpenQuestions {
		p.OQClaims[q.ID] = 0
	}
	for _, st := range fm.Stubs {
		p.StubViews = append(p.StubViews, StubView{
			Slug: st.Slug, Spike: st.Spike, Resolves: st.Resolves, AcceptanceCriteria: st.AcceptanceCriteria,
		})
		if st.Spike {
			for _, oqID := range st.Resolves {
				p.OQClaims[oqID]++
			}
			continue
		}
		for _, acID := range st.AcceptanceCriteria {
			p.ACCoverage[acID]++
		}
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

	// (1c) Scoping-layer edges (owner directive, the ac-3 "coverage yarn
	// projected" fix): each stub's declared attributions hang as yarn —
	// story stub → covered AC as "covers", spike stub → resolved OQ as
	// "resolves" — under layer "scoping". These are PROJECTIONS of the
	// stubs block, not document links: presentation-owned, no graduate/
	// delete/retype affordances, not gate material, and the closed
	// five-type spec-edge vocabulary is untouched. Both endpoints are
	// papers on this wall (the stub card via its "stub:<slug>" key, the
	// declared AC/OQ card); an attribution naming an undeclared id
	// projects nothing — a dangling attribution is the linter's finding,
	// never a phantom endpoint or a minted reference card.
	for _, st := range fm.Stubs {
		stubKey := "stub:" + st.Slug
		attrType, attrs := "covers", st.AcceptanceCriteria
		if st.Spike {
			attrType, attrs = "resolves", st.Resolves
		}
		for _, id := range attrs {
			if !declared[id] {
				continue
			}
			p.Edges = append(p.Edges, edgeView{Type: attrType, From: stubKey, To: id, Layer: "scoping"})
		}
	}

	// (3) Annotation streams: this board's free-floating stickies, its
	// untyped relates threads, and its pinned references. Graduated
	// records have already become spec content — they no longer render
	// (05 §Workbench: graduation is an ordinary edit; the sticky dies
	// into the document, and a graduated pin's card files into the
	// references lane, held by its typed edge instead).
	type pinView struct {
		id   string
		x, y float64
	}
	pins := map[string]pinView{}

	// liveByID is every non-graduated, non-pin, non-relates, non-review
	// annotation on THIS board, keyed by id — the set a relates thread's
	// endpoint may name directly (02 §Record schemas, round 5.4: "a
	// relates endpoint may name a board annotation by id ... as well as
	// an artifact ref"). Computed in its own pass since a relates thread
	// may appear before the sticky it names, in append order.
	liveByID := map[string]bool{}
	for _, a := range annotations {
		if a.Status == artifact.AnnotationGraduated {
			continue
		}
		switch a.Type {
		case artifact.AnnotationPin, artifact.AnnotationRelates, artifact.AnnotationReview:
			continue
		}
		if a.Board != nil && a.Board.Story == specName {
			liveByID[a.ID] = true
		}
	}

	for _, a := range annotations {
		if a.Status == artifact.AnnotationGraduated {
			continue
		}
		switch a.Type {
		case artifact.AnnotationPin:
			if a.Board == nil || a.Board.Story != specName || a.Target == nil {
				continue
			}
			r, err := artifact.ParseRef(a.Target.Ref)
			if err != nil {
				continue // unreachable: DecodeAnnotation validated the ref
			}
			ref := string(r.Kind) + "/" + r.Name
			if _, dup := pins[ref]; dup {
				continue // one card per ref, ever: the first record wins
			}
			pins[ref] = pinView{id: a.ID, x: a.Board.X, y: a.Board.Y}
		case artifact.AnnotationRelates:
			from, okA := relatesEndpoint(specName, declared, liveByID, a.Target)
			to, okB := relatesEndpoint(specName, declared, liveByID, a.TargetB)
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

	// (2) Positions: stored coordinates, everything else zoned — with
	// stored collisions resolved AT DISPLAY TIME (owner directive, ledgered
	// R4-I-35: cards never render stacked in any mode; a store can hold
	// positions saved before the uniform-footprint enlargement whose
	// footprints now collide, and a read-only board could never repair
	// them by drag). layout.json is never rewritten by rendering — the
	// nudge exists only in this projection, so a nudged card snaps back
	// to its stored spot once the contested footprint frees up.
	// Reference cards are every external edge endpoint, ordered by ref.
	refSet := map[string]bool{}
	for _, e := range p.Edges {
		// A scoping edge's endpoints are both on-wall papers by
		// construction (the stub card and a declared AC/OQ card) — its
		// "stub:<slug>" key must never mint a reference card.
		if e.Layer == "scoping" {
			continue
		}
		for _, end := range []string{e.From, e.To} {
			// An annotation-id endpoint (round 5.4: an attribution
			// thread tied to a live sticky) is the sticky's own paper —
			// never a reference card (a second card would split the
			// endpoint and steal the thread's tie).
			if end != "spec" && !declared[end] && !artifact.IsAnnotationID(end) {
				refSet[end] = true
			}
		}
	}
	// Pinned refs are reference cards too — DEDUPED against the
	// edge-derived set (one card per ref, ever), and while the pin record
	// lives its stored board position wins, injected as a stored position
	// the display resolver treats like any other.
	for ref := range pins {
		refSet[ref] = true
	}
	if len(pins) > 0 {
		merged := make(map[string]artifact.Position, len(stored)+len(pins))
		for k, v := range stored {
			merged[k] = v
		}
		for ref, pv := range pins {
			merged[ref] = artifact.Position{X: pv.x, Y: pv.y}
		}
		stored = merged
	}
	refs := make([]string, 0, len(refSet))
	for r := range refSet {
		refs = append(refs, r)
	}
	sort.Strings(refs)

	layoutObjs := make([]boardlayout.Object, 0, len(objects)+len(refs)+len(p.StubViews))
	for _, o := range objects {
		layoutObjs = append(layoutObjs, boardlayout.Object{Kind: boardlayout.ZoneKind(o.kind), ID: o.id, DocOrder: o.order})
	}
	for i, r := range refs {
		layoutObjs = append(layoutObjs, boardlayout.Object{Kind: boardlayout.ZoneReference, ID: r, DocOrder: i})
	}
	// Stub cards slot into the kind-locked stubs band in declaration order
	// (dc-6), keyed "stub:<slug>" — a namespace no object id or artifact
	// ref can produce, so a stub's key can never collide with another
	// card's. `stored` may hold a "stub:<slug>" entry (round 5.5: the
	// position action writes one when a stub is dragged) — Generate and
	// ResolveDisplayOverlaps below treat it exactly like a stored object
	// position, verbatim pass-through and all, since both operate
	// generically over Object.ID.
	for i, sv := range p.StubViews {
		layoutObjs = append(layoutObjs, boardlayout.Object{Kind: boardlayout.ZoneStub, ID: "stub:" + sv.Slug, DocOrder: i})
	}
	positions, err := boardlayout.ResolveDisplayOverlaps(layoutObjs, stored)
	if err != nil {
		return nil, fmt.Errorf("workbench: board layout for %s: %w", specName, err)
	}

	for _, o := range objects {
		pos := positions[o.id]
		p.Cards = append(p.Cards, cardView{ID: o.id, Kind: o.kind, Text: o.text, X: pos.X, Y: pos.Y, Anchored: anchored[o.id]})
	}
	for _, r := range refs {
		pos := positions[r]
		rc := refCardView{Ref: r, X: pos.X, Y: pos.Y}
		if pv, ok := pins[r]; ok {
			rc.Pinned = true
			rc.PinID = pv.id
		}
		p.RefCards = append(p.RefCards, rc)
	}
	for i := range p.StubViews {
		pos := positions["stub:"+p.StubViews[i].Slug]
		p.StubViews[i].X = pos.X
		p.StubViews[i].Y = pos.Y
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
// an annotation id (a-<ULID>) naming a live sticky on THIS board (per
// liveByID) is that sticky's own id — the round-5.4 attribution-thread
// case (02 §Record schemas: "a relates endpoint may name a board
// annotation by id"), e.g. a story sticky's coverage yarn to an AC, or a
// spike sticky's resolution yarn to an open question; a target naming
// this spec with a heading selector is that object's card; any other
// artifact target is a reference card. ok=false when the target names a
// sticky that is not live on this board, or belongs to some other spec's
// board entirely.
func relatesEndpoint(specName string, declared, liveByID map[string]bool, t *artifact.Target) (string, bool) {
	if t == nil {
		return "", false
	}
	if artifact.IsAnnotationID(t.Ref) {
		return t.Ref, liveByID[t.Ref]
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
