package workbench

// Closed-spec object supersession on the board (design §6, §8; SI-278 as
// clarified; the controller's rulings on lane L5's stop report and its
// fix pass 2): a closed spec's reference card renders the object's
// original text and the objsupersede views' §6 lines computed from
// DEFAULT-BRANCH records — only where the views report lines; every other
// reference card is unchanged — and the successor's own decision card
// renders its decision views from the board's OWN tree, so a design
// branch reads "proposed". Every line shows at rest: the enrichment is
// computed BEFORE the pure projector runs, so the layout reserves each
// grown card's height (boardlayout.Object.Height) and no card renders
// under another. The views come from the one process-wide cache every
// document consumer shares (specdocload.BoardIndexes), and
// internal/specdoc's conversion is the one place the lines are given
// their links; this file adds the board's link targets, the height
// estimates its stylesheet rules imply, and the wire shape.

import (
	"context"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// supersessionEnrichment is what computeObjectSupersession found for one
// board: the closed objects among the spec's edge targets that the
// default-branch views supersede, by reference-card ref, and each
// decision's views from the board's own tree, by decision id.
type supersessionEnrichment struct {
	objects   map[string]*refObjectView
	decisions map[string][]supersessionView
}

// computeObjectSupersession computes the board's closed-spec object
// supersession enrichment (SI-278) in the I/O tier, before buildProjection:
// each unpinned spec-object ref the spec's links and its decisions' links
// name — the reference cards buildProjection derives from the same edges
// — whose object the default-branch views report lines for, with the
// object's original text; and each decision's views of its fragment
// `supersedes` edges from the board's own tree. ix resolves link targets;
// fixedBranch keeps a per-branch board's links inside its branch
// (ADJ-70). Views that cannot be computed are the load's error: a board
// never renders a superseded object as untouched because its records
// could not be read.
func computeObjectSupersession(ctx context.Context, name string, fm *artifact.SpecFrontmatter, ix *index.Index, root, fixedBranch string) (*supersessionEnrichment, error) {
	b, err := specdocload.BoardIndexes(ctx, root)
	if err != nil {
		// vocab:identity — "closed-spec object supersession" is the design's feature name (design §2), not a lifecycle state label
		return nil, fmt.Errorf("workbench: closed-spec object supersession views for %s: %w", name, err)
	}
	objects := b.Default
	if objects == nil {
		objects = b.Tree
	}
	e := &supersessionEnrichment{objects: map[string]*refObjectView{}, decisions: map[string][]supersessionView{}}
	for _, ref := range objectRefs(name, fm) {
		r, err := artifact.ParseRef(ref)
		if err != nil {
			continue
		}
		v := objects.Index.Object(r.Name, r.Object)
		if v.State == objsupersede.ObjectNotSuperseded {
			continue
		}
		s, err := specdoc.ObjectSupersession(v, boardSupersessionLinks(name, ix, fixedBranch, objects, v.By, v.Revision, v.Conflict))
		if err != nil {
			return nil, fmt.Errorf("workbench: %s on %s: %w", ref, name, err)
		}
		e.objects[ref] = &refObjectView{Text: specdocload.ObjectText(objects, r), Supersession: objectSupersessionView(v, *s)}
	}
	for _, d := range fm.Decisions {
		for _, v := range b.Tree.Index.Decisions(name, d.ID) {
			s, err := specdoc.DecisionSupersession(v, boardSupersessionLinks(name, ix, fixedBranch, objects, v.Object, v.Establisher, v.Conflict))
			if err != nil {
				return nil, fmt.Errorf("workbench: %s#%s: %w", name, d.ID, err)
			}
			e.decisions[d.ID] = append(e.decisions[d.ID], decisionSupersessionView(v, s))
		}
	}
	return e, nil
}

// objectRefs lists, in declaration order and without duplicates, every
// spec-object ref outside spec name that its top-level links and its
// decisions' links name, keyed as buildProjection keys the reference
// card (the pin dropped: "spec/T#o").
func objectRefs(name string, fm *artifact.SpecFrontmatter) []string {
	var out []string
	seen := map[string]bool{}
	add := func(links []artifact.Link) {
		for _, l := range links {
			r, err := artifact.ParseRef(l.Ref)
			if err != nil || r.Kind != artifact.KindSpec || !r.Fragment() || r.Name == name {
				continue
			}
			key := "spec/" + r.Name + "#" + r.Object
			if !seen[key] {
				seen[key] = true
				out = append(out, key)
			}
		}
	}
	add(fm.Links)
	for _, d := range fm.Decisions {
		add(d.Links)
	}
	return out
}

// heights are the rendered heights the layout must reserve, by layout
// key: a reference card's ref, a decision card's id.
func (e *supersessionEnrichment) heights() map[string]float64 {
	if e == nil {
		return nil
	}
	h := map[string]float64{}
	for ref, o := range e.objects {
		h[ref] = refCardHeightPx(refCardView{Ref: ref, Object: o})
	}
	for id, views := range e.decisions {
		h[id] = cardHeightPx(cardView{ID: id, Supersessions: views})
	}
	return h
}

// attach lands the enrichment on the built projection's cards.
func (e *supersessionEnrichment) attach(proj *BoardProjection) {
	if e == nil {
		return
	}
	for i := range proj.RefCards {
		if o := e.objects[proj.RefCards[i].Ref]; o != nil {
			proj.RefCards[i].Object = o
		}
	}
	for i := range proj.Cards {
		if v := e.decisions[proj.Cards[i].ID]; len(v) > 0 {
			proj.Cards[i].Supersessions = v
		}
	}
}

// Height estimates in px, mirrored by style.css's rules for these cards
// (.refcard--object, .refcard-object-text, .objsupersede-lines). The
// layout reserves what these say, so every figure is conservative — a
// chars-per-line count below what the fonts fit, a line box a little
// taller than the rule's — and an estimate is never shorter than the
// render.
const (
	heightTextLinePx   = 17 // .refcard-object-text: 0.78rem × 1.35
	heightTextChars    = 24 // a sans line of the 12.5rem card, with slack
	heightLinePx       = 16 // .objsupersede-lines li: 0.68rem × 1.35
	heightLineChars    = 24 // a mono line of the card, with slack
	heightLineGapPx    = 3  // .objsupersede-lines gap 0.2rem
	heightBlockGapPx   = 7  // the list's and the text's 0.4rem margin
	heightPeekCuePx    = 22 // .refcard--object's 1.35rem room for the peek cue
	heightBadgeRowPx   = 18 // one .card-badges row in flow: a 0.56rem chip with its border, plus the row gap
	heightBadgesPerRow = 2  // chips are about half a card wide
)

// wrappedLines is how many visual lines text takes at chars per line
// under word wrap: tokens (whitespace-separated) move to the next line
// whole when they do not fit, and a token longer than a line breaks
// anywhere (overflow-wrap: anywhere). Word wrap wastes the tail of a line
// a character count never sees — two 15-character tokens need two
// 24-character lines, not one and a bit — so this is the conservative
// figure the layout reserves (the board closure's C-4).
func wrappedLines(text string, chars int) float64 {
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return 0
	}
	lines, used := 1, 0
	for _, tok := range tokens {
		n := utf8.RuneCountInString(tok)
		if used > 0 && used+1+n <= chars {
			used += 1 + n
			continue
		}
		if used > 0 {
			lines++
		}
		for n > chars { // an unbreakable token breaks anywhere across lines
			lines++
			n -= chars
		}
		used = n
	}
	return float64(lines)
}

// linesHeightPx is the height of the views' §6 lines as rendered in an
// .objsupersede-lines list: each line's text and its trailing links,
// wrapped, plus the list's gaps.
func linesHeightPx(views []supersessionView) float64 {
	var h float64
	for _, v := range views {
		h += heightBlockGapPx
		for _, l := range v.Lines {
			text := l.Text
			for _, t := range l.Trailing {
				text += " " + t.Ref
			}
			h += wrappedLines(text, heightLineChars)*heightLinePx + heightLineGapPx
		}
	}
	return h
}

// refCardHeightPx is a reference card's rendered height: the uniform
// footprint, or, for a card carrying its object (SI-278), the footprint
// plus the object's FULL original text (never clamped: "its original
// text, unchanged"), the lines, and the peek cue's room.
func refCardHeightPx(rc refCardView) float64 {
	if rc.Object == nil {
		return boardlayout.RefCardHeight
	}
	text := wrappedLines(rc.Object.Text, heightTextChars)
	return boardlayout.RefCardHeight + heightBlockGapPx + text*heightTextLinePx + linesHeightPx([]supersessionView{rc.Object.Supersession}) + heightPeekCuePx
}

// cardHeightPx is an object card's rendered height: the uniform
// footprint, or, for a decision card carrying its views, the footprint
// plus the lines and — when the card also wears wall badges, which then
// sit in flow above the lines instead of pinned over them — the badge
// rows (the docs closure's B-1).
func cardHeightPx(c cardView) float64 {
	if len(c.Supersessions) == 0 {
		return boardlayout.CardHeight
	}
	h := boardlayout.CardHeight + linesHeightPx(c.Supersessions)
	if n := len(c.Badges); n > 0 {
		rows := math.Ceil(float64(n) / heightBadgesPerRow)
		h += heightBlockGapPx + rows*heightBadgeRowPx
	}
	return h
}

// boardSupersessionLinks resolves each non-empty ref to its board-side
// href, keeping only the ones that resolve.
func boardSupersessionLinks(own string, ix *index.Index, fixedBranch string, views *specdocload.Views, refs ...string) map[string]string {
	links := map[string]string{}
	for _, ref := range refs {
		if href := boardSupersessionLink(ref, own, ix, fixedBranch, views); href != "" {
			links[ref] = href
		}
	}
	return links
}

// boardSupersessionLink is a view ref's href on the board of spec own: the
// board's own decision is its card on this page ("#obj-<id>"), and the
// board's own whole spec gets no link to itself; another spec's object or
// whole ref goes to that spec's SERVABLE surface (servableSurface: an
// active spec's board — on a per-branch board, that branch's own /b/
// board, ADJ-70 — or an archived spec's corpus page, ADJ-39) with the
// card anchor on a board and, on a corpus page, the object's DECLARED
// body anchor from views' records — the fragment every document consumer
// links (specdocload.ObjectAnchor); an object the records do not declare
// gets no link. A conflict goes to its corpus page. A per-branch board
// links to no corpus page, where no surface provably serves the branch's
// tree (the posture servableSurface takes for the archive). A pinned
// ref, a ref the index lacks, or anything else gets no link and renders
// as plain text.
func boardSupersessionLink(ref, own string, ix *index.Index, fixedBranch string, views *specdocload.Views) string {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Pinned() {
		return ""
	}
	whole := string(r.Kind) + "/" + r.Name
	switch r.Kind {
	case artifact.KindSpec:
		if r.Name == own {
			if r.Object == "" {
				return ""
			}
			return "#obj-" + r.Object
		}
		entry, ok := ix.Get(whole)
		if !ok {
			return ""
		}
		href, archived := servableSurface(whole, entry, fixedBranch)
		switch {
		case href == "" || r.Object == "":
			return href
		case archived:
			anchor, ok := specdocload.ObjectAnchor(views, r)
			if !ok {
				return ""
			}
			return href + "#" + anchor
		}
		return href + "#obj-" + r.Object
	case artifact.KindConflict:
		if r.Object != "" || fixedBranch != "" {
			return ""
		}
		if _, ok := ix.Get(whole); !ok {
			return ""
		}
		return "/a/" + whole
	}
	return ""
}

// objectSupersessionView is an object view's wire shape: the structured
// fields (the link targets) and specdoc's lines.
func objectSupersessionView(v objsupersede.ObjectView, s specdoc.Supersession) supersessionView {
	return supersessionView{
		State: string(v.State), By: v.By, Conflict: v.Conflict, Since: v.Since,
		Closed: v.Closed, ClosedWitness: v.ClosedWitness, Carry: string(v.Carry), Revision: v.Revision,
		Heads: append([]string(nil), v.Heads...), Witness: v.Witness, Lines: supersessionLines(s),
	}
}

// decisionSupersessionView is a decision view's wire shape.
func decisionSupersessionView(v objsupersede.DecisionView, s specdoc.Supersession) supersessionView {
	return supersessionView{
		State: string(v.State), Object: v.Object, Edge: v.Edge, Conflict: v.Conflict,
		Since: v.Since, Carried: v.Carried, Establisher: v.Establisher, Reason: v.Reason, Lines: supersessionLines(s),
	}
}

func supersessionLines(s specdoc.Supersession) []supersessionLineView {
	lines := make([]supersessionLineView, 0, len(s.Lines))
	for _, l := range s.Lines {
		lines = append(lines, supersessionLineView{Kind: l.Kind, Text: l.Text, Disclosure: l.Disclosure, Links: supersessionLinks(l.Links), Trailing: supersessionLinks(l.Trailing)})
	}
	return lines
}

func supersessionLinks(links []specdoc.RefLink) []supersessionLinkView {
	if len(links) == 0 {
		return nil
	}
	out := make([]supersessionLinkView, 0, len(links))
	for _, l := range links {
		out = append(out, supersessionLinkView{Ref: l.Ref, Href: l.URL})
	}
	return out
}

// specdocSupersession turns a wire view back into specdoc's shape, for the
// one line renderer both surfaces share (specdoc.SupersessionLineMarkup).
func specdocSupersession(v supersessionView) specdoc.Supersession {
	s := specdoc.Supersession{State: v.State, Object: v.Object}
	for _, l := range v.Lines {
		line := specdoc.SupersessionLine{Kind: l.Kind, Text: l.Text, Disclosure: l.Disclosure}
		for _, k := range l.Links {
			line.Links = append(line.Links, specdoc.RefLink{Ref: k.Ref, URL: k.Href})
		}
		for _, k := range l.Trailing {
			line.Trailing = append(line.Trailing, specdoc.RefLink{Ref: k.Ref, URL: k.Href})
		}
		s.Lines = append(s.Lines, line)
	}
	return s
}

// refCardSupersessionStem is a reference card's data-testid stem: its ref
// with "/" and "#" flattened, as refCardTestID flattens "/".
func refCardSupersessionStem(ref string) string {
	return strings.NewReplacer("/", "-", "#", "-").Replace(ref)
}
