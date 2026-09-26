package workbench

// Closed-spec object supersession on the board (design §6, §8; SI-278 as
// clarified; the controller's rulings on lane L5's stop report): a closed
// spec's reference card renders the object's original text and the
// objsupersede views' §6 lines computed from DEFAULT-BRANCH records — only
// where the views report lines; every other reference card is unchanged —
// and the successor's own decision card renders its decision views from
// the board's OWN tree, so a design branch reads "proposed". The views
// come from the one process-wide cache every document consumer shares
// (specdocload.BoardIndexes: the default-branch views once per head, the
// tree's once per records digest, bounded, single-flight), and
// internal/specdoc's conversion is the one place the lines are given
// their links; this file adds the board's link targets and the wire
// shape.

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
)

// attachObjectSupersession enriches proj (SI-278) in the I/O tier, after
// buildProjection, exactly as attachFamilyLinks does: each reference card
// whose target is a closed spec's object that the default-branch views
// report lines for gains the object's original text and those lines, and
// each decision card gains the views of its fragment `supersedes` edges
// from the board's own tree. ix resolves link targets; fixedBranch keeps
// a per-branch board's links inside its branch (ADJ-70).
func attachObjectSupersession(ctx context.Context, proj *BoardProjection, ix *index.Index, root, fixedBranch string) error {
	b, err := specdocload.BoardIndexes(ctx, root)
	if err != nil {
		return fmt.Errorf("workbench: closed-spec object supersession views for %s: %w", proj.Spec, err)
	}
	objects := b.Default
	if objects == nil {
		objects = b.Tree
	}
	for i := range proj.RefCards {
		rc := &proj.RefCards[i]
		ref, err := artifact.ParseRef(rc.Ref)
		if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || !ref.Fragment() {
			continue
		}
		v := objects.Index.Object(ref.Name, ref.Object)
		if v.State == objsupersede.ObjectNotSuperseded {
			continue
		}
		s, err := specdoc.ObjectSupersession(v, boardSupersessionLinks(proj.Spec, ix, fixedBranch, v.By, v.Revision, v.Conflict))
		if err != nil {
			return fmt.Errorf("workbench: %s on %s: %w", rc.Ref, proj.Spec, err)
		}
		rc.Object = &refObjectView{Text: specdocload.ObjectText(objects, ref), Supersession: objectSupersessionView(v, *s)}
	}
	for i := range proj.Cards {
		c := &proj.Cards[i]
		if c.Kind != string(boardlayout.ZoneDecision) {
			continue
		}
		for _, v := range b.Tree.Index.Decisions(proj.Spec, c.ID) {
			s, err := specdoc.DecisionSupersession(v, boardSupersessionLinks(proj.Spec, ix, fixedBranch, v.Object, v.Establisher, v.Conflict))
			if err != nil {
				return fmt.Errorf("workbench: %s#%s: %w", proj.Spec, c.ID, err)
			}
			c.Supersessions = append(c.Supersessions, decisionSupersessionView(v, s))
		}
	}
	return nil
}

// boardSupersessionLinks resolves each non-empty ref to its board-side
// href, keeping only the ones that resolve.
func boardSupersessionLinks(own string, ix *index.Index, fixedBranch string, refs ...string) map[string]string {
	links := map[string]string{}
	for _, ref := range refs {
		if href := boardSupersessionLink(ref, own, ix, fixedBranch); href != "" {
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
// card anchor on a board and the body heading's id on a corpus page; a
// conflict goes to its corpus page. A per-branch board links to no corpus
// page, where no surface provably serves the branch's tree (the same
// posture servableSurface takes for the archive). A pinned ref, a ref the
// index lacks, or anything else gets no link and renders as plain text.
func boardSupersessionLink(ref, own string, ix *index.Index, fixedBranch string) string {
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
			return href + "#" + r.Object
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
