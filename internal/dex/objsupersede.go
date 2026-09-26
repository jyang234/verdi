package dex

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/specdoc"
)

// Closed-spec object supersession on the docs site (design §6, §8;
// SI-263): a closed spec's document renders a superseded criterion or
// decision's original text unchanged, followed by the objsupersede views'
// lines, and a successor's decision renders its decision view. The views
// are the ONE source of every line (internal/objsupersede/views.go); this
// file computes them once per build from the build commit's records —
// the published site is built from the default branch, so a
// default-branch surface computes from default-branch records only — and
// resolves the links the lines carry to the site's own pages.

// supersessionIndex computes every object and decision view of the
// records at commit, against the default branch's history at root. It is
// called once per build (the L3c cost note: an accepted successor costs
// about a git read of its acceptance commit) and read per spec by
// supersessionFacts. An unreadable tree or history answer is the build's
// error: a site never renders a superseded object as untouched because
// its records could not be read.
func supersessionIndex(ctx context.Context, root, commit string) (*objsupersede.Index, error) {
	recs, err := objsupersede.ReadRecords(ctx, objsupersede.CommitTree{Root: root, Commit: commit})
	if err != nil {
		// vocab:identity — "closed-spec object supersession" is the design's feature name (design §2), not a lifecycle state label
		return nil, fmt.Errorf("dex: closed-spec object supersession records at %s: %w", commit, err)
	}
	ix, err := objsupersede.NewIndex(ctx, recs, objsupersede.NewHistory(ctx, root))
	if err != nil {
		// vocab:identity — "closed-spec object supersession" is the design's feature name (design §2), not a lifecycle state label
		return nil, fmt.Errorf("dex: closed-spec object supersession views at %s: %w", commit, err)
	}
	return ix, nil
}

// supersessionFacts assembles spec name's document facts from the index:
// the view of each criterion and decision a supersession touches, the
// views of each decision's fragment `supersedes` edges, and a link for
// every ref those views name that the site has a document or page for.
// A spec none of the views touch gets empty maps, which render exactly as
// before.
func supersessionFacts(ix *objsupersede.Index, name string, fm *artifact.SpecFrontmatter, known map[string]bool, docs documentSet) *specdoc.SupersessionFacts {
	f := &specdoc.SupersessionFacts{
		Objects:   map[string]objsupersede.ObjectView{},
		Decisions: map[string][]objsupersede.DecisionView{},
		Links:     map[string]string{},
	}
	link := func(refs ...string) {
		for _, ref := range refs {
			if url := supersessionLink(ref, known, docs); url != "" {
				f.Links[ref] = url
			}
		}
	}
	object := func(id string) {
		if v := ix.Object(name, id); v.State != objsupersede.ObjectNotSuperseded {
			f.Objects[id] = v
			link(v.By, v.Revision, v.Conflict)
		}
	}
	for _, ac := range fm.AcceptanceCriteria {
		object(ac.ID)
	}
	for _, d := range fm.Decisions {
		object(d.ID)
		for _, v := range ix.Decisions(name, d.ID) {
			f.Decisions[d.ID] = append(f.Decisions[d.ID], v)
			link(v.Object, v.Establisher, v.Conflict)
		}
	}
	return f
}

// supersessionLink resolves a canonical, unpinned ref a view names to its
// URL on this site: a spec's object ("spec/S#o") or a whole spec
// ("spec/S") to that spec's document — the one docs-site surface that
// renders objects — when the build commit carries it (documentSet), and a
// conflict to its permalink page when the site has one. Anything else,
// a pinned ref included, gets no link and renders as plain text.
func supersessionLink(ref string, known map[string]bool, docs documentSet) string {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Pinned() {
		return ""
	}
	whole := string(r.Kind) + "/" + r.Name
	switch r.Kind {
	case artifact.KindSpec:
		if !docs[whole] {
			return ""
		}
		url := permalinkURL(whole) + documentPageDir + "/"
		if r.Object != "" {
			url += "#" + r.Object
		}
		return url
	case artifact.KindConflict:
		if r.Object != "" {
			return ""
		}
		url, _ := resolvableLinkURL(whole, known)
		return url
	}
	return ""
}
