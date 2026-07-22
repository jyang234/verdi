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

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/model"
)

// applyModelVocabulary resolves p's display labels through the store's
// resolved operating model (spec/vocabulary-surfaces ac-2): ClassLabel
// through model.DisplayClass over the same effective name the class tag
// renders (Class, or "spike" for a spike story — not a model class, so it
// falls back to itself), StatusLabel through model.DisplayState — the
// identical lookups the CLI surfaces use, never a board-private rename
// table. A label is set ONLY when the resolved word differs from the bare
// id, so a no-rename model (the embedded canonical included) leaves the
// projection — and therefore both its rendered HTML and get_board's JSON
// marshaling of this struct — byte-identical. Nil-safe on both receivers.
func (p *BoardProjection) applyModelVocabulary(m *model.Model) {
	if p == nil || m == nil {
		return
	}
	// The render-side class-word vocabulary rides the projection so the
	// one renderer (boardspecrender.go) speaks resolved words in its
	// prose too — unexported, so get_board's wire JSON is untouched
	// (L-M9 added ClassLabel/StatusLabel to the wire deliberately; the
	// prose vocabulary deliberately stays off it).
	p.words = classWords{m: m}
	name := p.Class
	if p.Spike {
		name = "spike"
	}
	if label := m.DisplayClass(name); label != name {
		p.ClassLabel = label
	}
	if label := m.DisplayState(p.Class, p.Status); label != p.Status {
		p.StatusLabel = label
	}
}

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
	// Obligations is a STORY AC card's evidence-obligation disclosures
	// (spec/obligation-wall ac-2/dc-3): one entry PER DECLARED evidence kind,
	// each carrying either that kind's authored obligation (title + prose) or
	// a disclosed "no obligation" marker (dc-2). Empty on every non-AC card
	// and on every non-story wall (a feature AC wears its coverage receipt
	// instead). It is a STORE-DERIVED enrichment — attached by
	// attachObligations (boardspec.go) from the one loader both the board and
	// `verdi matrix` consume (evidence.Obligations, dc-1: "not two readers"),
	// NOT a projection of buildProjection's four in-memory inputs, which is
	// why it is populated after buildProjection returns (the same posture
	// proj.Notices takes). JSON-tagged because get_board (internal/mcpserve)
	// re-marshals cardView as its wire result: an agent reads the same
	// obligations a human sees on the wall.
	Obligations []obligationView `json:"obligations,omitempty"`
	// Badges is this card's computed wall badges (spec/badge-computes
	// ac-1/ac-2/dc-2): every derivation record whose Target names this
	// object's own id, attached by attachBadges (badges.go) AFTER
	// buildProjection returns — a store-derived I/O enrichment, exactly
	// like Obligations above, never a projection of buildProjection's
	// four in-memory inputs. Empty on every card no locus-declaring VL
	// finding names. The frontend phase renders each entry as a chip
	// carrying data-badge-source and the entry's own serialized JSON
	// (dc-4's opener contract) — this field only carries the DATA; no
	// chip markup is emitted here.
	Badges []badgeView `json:"badges,omitempty"`
}

// obligationView is one declared evidence kind's obligation as it renders on
// a STORY AC card (spec/obligation-wall ac-2). Present is true when an
// obligation has been authored for that kind, carrying its Title — the
// specific demand, read from the obligation's own content, never recovered
// from verdi.bindings.yaml (feature co-3, legible-without-the-sidecar) — and
// its prose Body. Present is false for a declared kind with no obligation
// yet: the card shows a disclosed "no obligation" badge (dc-2, the
// wall-receipts posture — disclosure, never refusal; the activation gate is
// what refuses at accept, so a draft in progress still renders legibly).
// The Slot fields are spec/evidence-slot ac-3/dc-2: the kind's
// fold-derived record state JOINS this same view — one row per declared
// kind carrying both what the kind demands (the obligation) and what it
// holds (the record-state chip) — rather than riding a second per-kind
// list. Slot is "empty" (the fold's own per-kind no-record state, dc-1)
// or "held"; SlotRecords counts the CURRENT records of the kind
// (attestation: 1 when the attestation file exists). Both are
// store-derived enrichments attached by attachBadges (badges.go) from
// wallbadge.EmptySlotBadges — populated after attachObligations, in the
// same I/O tier; Slot is "" only when badges have not been attached.
// Presence disclosure only, never the fold's AC verdicts (dc-4).
type obligationView struct {
	Kind        string `json:"kind"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body,omitempty"`
	Present     bool   `json:"present"`
	Slot        string `json:"slot,omitempty"`
	SlotRecords int    `json:"slot_records,omitempty"`
}

// badgeInputView is one pinned input a badge's derivation record cites —
// a local mirror of wallbadge.InputRecord's JSON shape. Kept as a
// package-local view type, exactly like obligationView above (rather than
// importing internal/wallbadge's own struct into this file), so
// projection.go — the pure projector — stays free of any enrichment
// package's import; badges.go (the I/O tier) does the field-by-field copy
// when it attaches computed badges.
type badgeInputView struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}

// badgeView is one computed wall badge (spec/badge-computes dc-2) — a
// local mirror of wallbadge.DerivationRecord's JSON shape, for the same
// reason badgeInputView is: this file (the pure projector) never imports
// the compute package that produces the real value. badges.go converts
// wallbadge.DerivationRecord into this shape at attach time.
type badgeView struct {
	Source      string           `json:"source"`
	Label       string           `json:"label"`
	Target      string           `json:"target,omitempty"`
	Inputs      []badgeInputView `json:"inputs"`
	Records     []string         `json:"records"`
	Disclosures []string         `json:"disclosures,omitempty"`
	// Provenance mirrors wallbadge.DerivationRecord.Provenance
	// (spec/derivation-drawer ac-3/co-3): the optional pinned-provenance
	// block the derivation drawer stamps once at its head — populated by
	// the judged-sweep case-file chip (covers sha, adr_corpus_digest,
	// decisions_scanned), empty for every other badge.
	Provenance []string `json:"provenance,omitempty"`
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
	// EditorHref links a diagram reference card whose target is a class:
	// proposal artifact to its board editor (spec/board-editor dc-1:
	// "reachable from a spec board's pinned diagram reference card").
	// Store-derived enrichment attached in the I/O layer
	// (attachDiagramEditorHrefs), never computed by the pure projector.
	EditorHref string `json:"editorHref,omitempty"`
	// FeatureHref links a document-level implements-edge reference card to
	// its target feature's own SERVABLE surface (spec/family-board-links
	// ac-1, dc-1/dc-2): set when this card is the target of a document-level
	// implements edge AND that target's base spec ref (fragment dropped)
	// resolves in the current index. An ACTIVE feature links to its board
	// ("/board/spec/<feature-name>", parent ac-2 verbatim); an ARCHIVED
	// feature — which the board route 404s on — links to its corpus page
	// ("/a/spec/<feature-name>") with Archived set, per ADJ-39's
	// constraint-over-mandate ruling (servableSurface). Store-derived
	// enrichment attached by attachFamilyLinks, mirroring EditorHref's exact
	// posture. Never set alongside UnresolvedNotice.
	FeatureHref string `json:"featureHref,omitempty"`
	// Archived discloses, beside FeatureHref, that the target feature
	// resolves in the archive zone (spec/family-board-links ac-1, dc-1's
	// ADJ-28/ADJ-39 completion reading): the affordance links to the
	// servable corpus page, and the card says so rather than pretending the
	// archived feature has a live board. Empty for an active target.
	// Store-derived enrichment attached by attachFamilyLinks, mirroring
	// stubStoryLinkView.Archived's exact posture.
	Archived bool `json:"archived,omitempty"`
	// UnresolvedNotice discloses, on the SAME card, why no FeatureHref was
	// offered (spec/family-board-links ac-4, co-3): set only when this
	// card is a document-level implements edge's target and that target
	// does not resolve anywhere in the current index — naming the
	// unresolved ref rather than rendering a silently inert card or a
	// link that would 404. Store-derived enrichment attached by
	// attachFamilyLinks.
	UnresolvedNotice string `json:"unresolvedNotice,omitempty"`
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
	// Badges mirrors cardView.Badges (spec/badge-computes dc-3): a stub is
	// a rendered board object too (keyed "stub:<slug>"), and a dangling
	// stub reference (VL-006's checkStubACs/checkStubResolves) anchors to
	// the stub's own card, not the case file.
	Badges []badgeView `json:"badges,omitempty"`
	// StoryLinks are AC-2's matched-story board affordances
	// (spec/family-board-links ac-2, dc-1/dc-4): one entry per DISTINCT
	// story spec whose implements edges name one of this stub's declared
	// acceptance criteria, anywhere in this checkout's store (active or
	// archived zone alike) — computed via the same backlink inversion the
	// feature fold already uses, never a second graph walk or heuristic
	// title/slug match. Empty until a match resolves. Store-derived
	// enrichment attached by attachFamilyLinks.
	StoryLinks []stubStoryLinkView `json:"storyLinks,omitempty"`
	// InstantiatedNotice is AC-3's live ref-check disclosure (dc-3, dc-5):
	// set only when StoryLinks is empty (no matching story resolves
	// anywhere) AND refs/heads/design/<slug> exists in the SERVING
	// checkout. Fixed verbatim text (parent dc-5) with the branch name
	// filled in; empty otherwise, leaving the card's existing Instantiate
	// affordance as the only rendered state (today's plain
	// un-instantiated card, unchanged). Store-derived enrichment attached
	// by attachFamilyLinks.
	InstantiatedNotice string `json:"instantiatedNotice,omitempty"`
}

// stubStoryLinkView is one AC-2 matched-story affordance on a feature
// stub card (spec/family-board-links ac-2): the matched story's own ref,
// the href of the workbench surface that SERVES it, and whether the match
// resolved under specs/archive/ (dc-1's ADJ-28 completion reading). An
// ACTIVE match's Href is its board ("/board/spec/<name>", parent ac-2
// verbatim); an ARCHIVED match's Href is its corpus page
// ("/a/spec/<name>") — the board route 404s on the archive zone, so the
// card links to the servable surface instead (ADJ-39, servableSurface) —
// with Archived disclosed on the card. Either way the card never falls
// through to AC-3's in-between notice once a match resolves.
type stubStoryLinkView struct {
	Ref      string `json:"ref"`
	Href     string `json:"href"`
	Archived bool   `json:"archived,omitempty"`
	// UnservableNotice is ADJ-70's disclosed no-link state: set (with Href
	// empty) only for an ARCHIVED match resolved on a PER-BRANCH board,
	// where no workbench surface provably serves the archive (the /a/
	// corpus is root-only and reads the serving checkout). The card renders
	// the ref, its archived badge, and this text — never an href that can
	// 404, never a silent omission (co-2). Never set alongside a non-empty
	// Href.
	UnservableNotice string `json:"unservableNotice,omitempty"`
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
	// ClassLabel and StatusLabel are the resolved model's display words
	// for Class (or "spike") and Status (spec/vocabulary-surfaces ac-2),
	// set by applyModelVocabulary ONLY when a rename actually differs from
	// the bare id — so a store with no renames (or no model at all)
	// serializes and renders byte-identically to a pre-vocabulary build
	// (the parity floor; get_board marshals this struct directly). The
	// renderers fall back to the id when these are empty; the id itself
	// stays in every CSS class, testid, and data attribute regardless.
	ClassLabel  string `json:"class_label,omitempty"`
	StatusLabel string `json:"status_label,omitempty"`
	Problem     string `json:"problem,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
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
	// CreateFields are the creation form's enumerated field descriptors
	// (spec/creation-form ac-2/ac-3): the story class's resolved template
	// (store override winning) run through designscaffold.Fields — ONE
	// contract shared by the server-rendered form and the create action's
	// submitted-values validation. Populated by the I/O enrichment tier
	// (loadBoard) only for a sealed accepted-pending-build feature wall;
	// empty everywhere else, so no other wall renders the affordance.
	// Excluded from JSON: get_board's payload is unchanged.
	CreateFields []designscaffold.Field `json:"-"`
	// Notices are disclosed-unavailable banners rendered in the board
	// chrome in EVERY mode (I-1(b)/I-2/M-4): a configured-but-unreachable
	// review feed, or an assumed default branch. Not a projection of the
	// four inputs — a render-time disclosure the loader attaches, so the
	// board never renders as if a skipped input were simply absent
	// (constitution 2/10: silence is never a pass).
	Notices []string `json:"notices,omitempty"`
	// CaseFileBadges is every spec-level computed wall badge (spec/badge-
	// computes ac-1/ac-2/ac-3/dc-2): a VL finding declaring a spec-level
	// locus, plus the spec-stale and pending-supersession ladder flags on
	// a story wall. Attached by attachBadges (badges.go), the same I/O
	// enrichment tier as Notices/Obligations — never a projection of
	// buildProjection's four in-memory inputs.
	CaseFileBadges []badgeView `json:"case_file_badges,omitempty"`
	// CaseFileDisclosures are the ladder computes' disclosed-unproven
	// outcomes (spec/case-file-flags ac-1/dc-4): spec-level state that
	// could not be proven either way — e.g. pending-supersession with no
	// forge to enumerate open MRs. Rendered as a disclosure LINE on the
	// case-file lockup in the board's notice vocabulary, never a stamp
	// (unproven is never dressed as a verdict in either direction) and
	// never silence. Kept apart from Notices — those are board-chrome
	// banners about the SERVING context (an unreachable review feed, an
	// assumed default branch); these are spec-level truths that belong on
	// the case file, exactly where the stamps they stand in for would
	// hang. Same I/O-enrichment tier as CaseFileBadges (badges.go).
	CaseFileDisclosures []string `json:"case_file_disclosures,omitempty"`
	// words is the render-side class-word display vocabulary
	// (vocabulary.go), set by applyModelVocabulary. Unexported on
	// purpose: it exists for the HTML renderers' prose only and never
	// enters get_board's JSON marshaling of this struct. Its zero value
	// resolves every id to itself, so projections constructed without a
	// model render today's bare words byte-identically.
	words classWords
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
