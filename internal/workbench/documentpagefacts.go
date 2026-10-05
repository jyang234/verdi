package workbench

// The Document page's chrome facts (spec/document-page-v2 ac-1, ac-2;
// ledger SI-340): what the page states around the shared body — the
// temporal stamp, the identity card, the contents rail, and which object
// anchors may carry an id chip. One value per render, built by a pure
// function from what the page's own load already resolved (its document,
// its HTML, its specdocload.Result, and what it read of the checkout):
// nothing here resolves anything, so the facts can never disagree with
// the body they frame. The page's snapshot carries the same value and its
// revision token covers it (SI-340 (8)), so a poll that swaps the body
// brings the chrome of the same revision.
//
// The facts are chrome only: the body's bytes are the shared renderer's,
// unchanged (ac-3, co-1), and nothing here is rendered into them.

import (
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/headings"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

// documentPageFacts is everything the Document page's chrome states about
// the document it frames.
type documentPageFacts struct {
	Stamp    documentStamp       `json:"stamp"`
	Identity documentIdentity    `json:"identity"`
	Rail     []documentRailEntry `json:"rail"`
	// Chips names the object anchors an id chip may be drawn at; empty
	// wherever a chip could not work (SI-340 (10)).
	Chips []documentChip `json:"chips"`
}

// documentStamp is the temporal stamp's server half (SI-340 (5), (6)):
// the state and its words, and the commit the bytes were read at. The
// refreshed time is computed in the browser (dc-2), never here.
type documentStamp struct {
	// State is "proposed" or "accepted" (the token's id); Words is what
	// the stamp says for it.
	State  string `json:"state"`
	Words  string `json:"words"`
	Commit string `json:"commit"`
}

// The stamp's states and the words the proposed one speaks.
const (
	documentStateProposed = "proposed"
	documentStateAccepted = "accepted"
	// vocab:identity — "proposed"/"accepted" name the merge event of the spec MR (the body's own blockquote, specdoc), not a lifecycle state label
	documentProposedWords = "proposed, not accepted"
)

// documentIdentity is the identity card (SI-340 (4)): the ref, the class,
// the checkout's branch, the owners, and the files the load rendered from.
type documentIdentity struct {
	Ref string `json:"ref"`
	// Class is the class chip's id and ClassLabel its display word, as the
	// top bar states them; both are empty for a spec that declares none.
	Class      string `json:"class,omitempty"`
	ClassLabel string `json:"classLabel,omitempty"`
	// Branch is the checked-out branch as the bar states it: proven, an
	// empty text with Detached on a detached HEAD, or disclosed-unproven
	// with its reason.
	Branch   barFact  `json:"branch"`
	Detached bool     `json:"detached,omitempty"`
	Owners   []string `json:"owners"`
	Files    []string `json:"files"`
}

// documentRailEntry is one contents-rail entry: an h2 section of the
// body, with the id the body carries (SI-340 (1), (7)) and, for a section
// that enumerates items, the number it renders.
type documentRailEntry struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Count *int   `json:"count,omitempty"`
}

// documentChip is one object anchor an id chip may be drawn at: the
// object's id (its anchor and its wall card's) and its kind, in the
// wall's own object-kind words.
type documentChip struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// newDocumentPageFacts builds the page's facts for spec name from what its
// load resolved: doc and html are the document it built and rendered, res
// the load's Result, and checkout the Git state it read. A pure function.
func newDocumentPageFacts(name string, doc specdoc.Document, html string, res specdocload.Result, checkout documentCheckout, m *model.Model) documentPageFacts {
	return documentPageFacts{
		Stamp:    newDocumentStamp(doc.Stamp),
		Identity: newDocumentIdentity(name, doc.Stamp.Ref, res, checkout, m),
		Rail:     documentRail(doc, html),
		Chips:    documentChips(doc, res.RelPath == store.ActiveSpecRelPath(name)),
	}
}

// newDocumentStamp states the document's own stamp: proposed exactly when
// the body's stamp is.
func newDocumentStamp(s specdoc.Stamp) documentStamp {
	if s.Proposed {
		return documentStamp{State: documentStateProposed, Words: documentProposedWords, Commit: s.Commit}
	}
	return documentStamp{State: documentStateAccepted, Words: documentStateAccepted, Commit: s.Commit}
}

// newDocumentIdentity states the identity card. The class goes through the
// same projection head and display chain the top bar's does
// (specBarFacts), and the branch is the checkout's as the bar reads it.
func newDocumentIdentity(name, ref string, res specdocload.Result, checkout documentCheckout, m *model.Model) documentIdentity {
	id := documentIdentity{Ref: ref, Owners: []string{}, Files: []string{res.RelPath}}
	id.Branch, id.Detached = checkout.branchFact()
	if fm := res.Input.Spec; fm != nil {
		id.Owners = append(id.Owners, fm.Owners...)
		p := projectionHead(name, fm, "", "")
		p.applyModelVocabulary(m)
		if p.Class != "" {
			id.Class = p.classChipID()
			id.ClassLabel = p.ClassLabel
			if id.ClassLabel == "" {
				id.ClassLabel = id.Class
			}
		}
	}
	return id
}

// documentRail lists the body's h2 sections in order, through the docs
// site's own heading extraction (internal/headings), so each entry carries
// the id the body gives it. An entry is counted only when it is provably
// the document's next section (SI-343 (1)): its text is that section's
// heading, and no other h2 of the body carries the same text. A spec's
// prose can render an h2 too, through a setext or indented heading, so a
// heading text that appears more than once is ambiguous and none of its
// entries is counted. Every other entry, the prose's own, is listed with
// no count. Matching only ever moves to the section after the previous
// match, so it never skips one: a section the body does not render as an
// h2 leaves every later entry uncounted rather than guessed.
func documentRail(doc specdoc.Document, html string) []documentRailEntry {
	entries := headings.Sections(headings.Extract(html))
	occurrences := make(map[string]int, len(entries))
	for _, h := range entries {
		occurrences[h.Text]++
	}
	rail := make([]documentRailEntry, 0, len(entries))
	next := 0
	for _, h := range entries {
		e := documentRailEntry{ID: h.ID, Text: h.Text}
		if next < len(doc.Sections) && documentSectionHeading(doc.Sections[next]) == h.Text {
			if n, ok := documentSectionItems(doc, doc.Sections[next]); ok && occurrences[h.Text] == 1 {
				e.Count = &n
			}
			next++
		}
		rail = append(rail, e)
	}
	return rail
}

// documentSectionHeading is the h2 text the shared renderer
// (specdoc.RenderMarkdown) writes for section s; TestDocumentSectionHeadings
// pins it to that renderer's output for every section.
func documentSectionHeading(s specdoc.SectionID) string {
	switch s {
	case specdoc.SectionIdentity:
		return "Identity"
	case specdoc.SectionProblem:
		return "Problem"
	case specdoc.SectionOutcome:
		return "Outcome"
	case specdoc.SectionDecisions:
		return "Decisions"
	case specdoc.SectionConstraints:
		return "Constraints"
	case specdoc.SectionCriteria:
		return "Acceptance criteria"
	case specdoc.SectionQuestions:
		return "Open questions"
	case specdoc.SectionPlan:
		return "Plan"
	case specdoc.SectionEvidence:
		return "Evidence"
	case specdoc.SectionReadiness:
		return "Readiness"
	}
	return ""
}

// documentSectionItems is the number of items section s renders, and
// whether it enumerates any: one per decision, constraint, criterion, and
// question, one per plan line, one per evidence row, and one per
// readiness concern in the attention queue. The identity table, the
// problem and outcome statements, and a section whose facts were not
// supplied for this render enumerate nothing.
func documentSectionItems(doc specdoc.Document, s specdoc.SectionID) (int, bool) {
	switch s {
	case specdoc.SectionDecisions:
		return len(doc.Decisions), true
	case specdoc.SectionConstraints:
		return len(doc.Constraints), true
	case specdoc.SectionCriteria:
		return len(doc.Criteria), true
	case specdoc.SectionQuestions:
		return len(doc.Questions), true
	case specdoc.SectionPlan:
		return len(doc.Plan), true
	case specdoc.SectionEvidence:
		if doc.EvidenceKnown {
			return len(doc.Evidence), true
		}
	case specdoc.SectionReadiness:
		if doc.ReadinessKnown && doc.Readiness != nil {
			return len(doc.Readiness.Attention), true
		}
	}
	return 0, false
}

// documentChips lists the object anchors the body renders for the
// sections of its kind, in document order — its decisions, constraints,
// acceptance criteria, and open questions, never a readiness concern's
// anchor — when the spec is the active-zone spec the wall serves (active).
// Otherwise, an archived spec whose wall answers 404, it lists none.
func documentChips(doc specdoc.Document, active bool) []documentChip {
	chips := []documentChip{}
	if !active {
		return chips
	}
	add := func(kind boardlayout.ZoneKind, id string) {
		chips = append(chips, documentChip{ID: id, Kind: string(kind)})
	}
	for _, s := range doc.Sections {
		switch s {
		case specdoc.SectionDecisions:
			for _, d := range doc.Decisions {
				add(boardlayout.ZoneDecision, d.ID)
			}
		case specdoc.SectionConstraints:
			for _, c := range doc.Constraints {
				add(boardlayout.ZoneConstraint, c.ID)
			}
		case specdoc.SectionCriteria:
			for _, c := range doc.Criteria {
				add(boardlayout.ZoneAC, c.ID)
			}
		case specdoc.SectionQuestions:
			for _, q := range doc.Questions {
				add(boardlayout.ZoneOpenQuestion, q.ID)
			}
		}
	}
	return chips
}
