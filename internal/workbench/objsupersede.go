package workbench

// Closed-spec object supersession on the board (design §6, §8; SI-278;
// the controller's rulings on lane L5's stop report): a closed spec's
// reference card renders the object's original text and the objsupersede
// views' §6 lines, computed from DEFAULT-BRANCH records — only where the
// views report lines; every other reference card is unchanged — and the
// successor's own decision card renders its decision views from the
// board's OWN tree, so a design branch reads "proposed". The views are
// the one source of every line (internal/objsupersede/views.go), and
// internal/specdoc's conversion is the one place the lines are given
// their links; this file adds the board's link targets and the wire
// shape. The index is cached per root (supersessionCache): the
// default-branch index is invalidated when that head moves, the tree's
// when its records change, an unchanged tree never recomputes, the cache
// is bounded, and concurrent requests share one build.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// supersessionCache holds view indexes by key: "<root>\x00default\x00<head>"
// for the default branch's records at its head, "<root>\x00tree\x00<digest>"
// for a tree's own records. A bounded LRU (limit entries across every
// root and branch) with single-flight builds: concurrent gets of one key
// wait for the one build in flight rather than racing a duplicate.
type supersessionCache struct {
	mu       sync.Mutex
	limit    int
	entries  map[string]*supersessionEntry
	order    []string // keys, least recently used first
	inflight map[string]*supersessionCall
	builds   atomic.Int32 // builds run; read by tests
}

// supersessionEntry is one cached index with the records it was built from
// (a reference card renders the object's text from them) and their digest
// (recordsDigest), so a tree whose records equal the default branch's is
// served the default-branch entry itself.
type supersessionEntry struct {
	digest string
	recs   *objsupersede.Records
	index  *objsupersede.Index
}

type supersessionCall struct {
	done  chan struct{}
	entry *supersessionEntry
	err   error
}

func newSupersessionCache(limit int) *supersessionCache {
	return &supersessionCache{limit: limit, entries: map[string]*supersessionEntry{}, inflight: map[string]*supersessionCall{}}
}

// boardSupersession is the process-wide cache every board instance — the
// serving checkout's, each per-branch draft board's, and get_board's
// LoadProjection — shares.
var boardSupersession = newSupersessionCache(8)

// get returns key's entry, building it with build on a miss. A build in
// flight for the same key is joined, never duplicated; a failed build is
// returned to every waiter and caches nothing. The build runs under a
// context detached from the leader's cancellation so a waiter never
// inherits a stranger's cancel; each waiter still honors its own.
func (c *supersessionCache) get(ctx context.Context, key string, build func(context.Context) (*supersessionEntry, error)) (*supersessionEntry, error) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.touch(key)
		c.mu.Unlock()
		return e, nil
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			return call.entry, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &supersessionCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	c.builds.Add(1)
	entry, err := build(context.WithoutCancel(ctx))
	c.mu.Lock()
	delete(c.inflight, key)
	if err == nil {
		c.entries[key] = entry
		c.touch(key)
		c.evict()
	}
	c.mu.Unlock()
	call.entry, call.err = entry, err
	close(call.done)
	return entry, err
}

// touch marks key most recently used; c.mu held.
func (c *supersessionCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

// evict drops least recently used entries past the limit; c.mu held.
func (c *supersessionCache) evict() {
	for len(c.entries) > c.limit && len(c.order) > 0 {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

// boardIndexes are the two entries a board reads: def, the default
// branch's records at its head — nil when the default branch cannot be
// resolved, in which case the tree's own entry (whose history facts all
// read unproven) serves both cards; and tree, the board tree's own
// records — the very same entry as def when the tree's records equal the
// default branch's (the default-branch board: one index).
type boardIndexes struct{ def, tree *supersessionEntry }

// boards returns root's two entries, computing only what the cache lacks.
func (c *supersessionCache) boards(ctx context.Context, root string) (boardIndexes, error) {
	treeDigest, err := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	if err != nil {
		return boardIndexes{}, err
	}
	buildTree := func(ctx context.Context) (*supersessionEntry, error) {
		return buildSupersessionEntry(ctx, root, objsupersede.WorkTree{Root: root}, treeDigest)
	}
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		tree, err := c.get(ctx, supersessionKey(root, "tree", treeDigest), buildTree)
		if err != nil {
			return boardIndexes{}, err
		}
		return boardIndexes{tree: tree}, nil
	}
	head, err := gitx.RevParse(ctx, root, branch.Ref)
	if err != nil {
		return boardIndexes{}, err
	}
	def, err := c.get(ctx, supersessionKey(root, "default", head), func(ctx context.Context) (*supersessionEntry, error) {
		ct := objsupersede.CommitTree{Root: root, Commit: head}
		digest, err := recordsDigest(ctx, ct)
		if err != nil {
			return nil, err
		}
		return buildSupersessionEntry(ctx, root, ct, digest)
	})
	if err != nil {
		return boardIndexes{}, err
	}
	if def.digest == treeDigest {
		return boardIndexes{def: def, tree: def}, nil
	}
	tree, err := c.get(ctx, supersessionKey(root, "tree", treeDigest), buildTree)
	if err != nil {
		return boardIndexes{}, err
	}
	return boardIndexes{def: def, tree: tree}, nil
}

func supersessionKey(root, kind, id string) string {
	return root + "\x00" + kind + "\x00" + id
}

// buildSupersessionEntry reads tr's records and computes every view
// against the default branch's history at root.
func buildSupersessionEntry(ctx context.Context, root string, tr objsupersede.TreeReader, digest string) (*supersessionEntry, error) {
	recs, err := objsupersede.ReadRecords(ctx, tr)
	if err != nil {
		return nil, err
	}
	ix, err := objsupersede.NewIndex(ctx, recs, objsupersede.NewHistory(ctx, root))
	if err != nil {
		return nil, err
	}
	return &supersessionEntry{digest: digest, recs: recs, index: ix}, nil
}

// The record directories the digest covers: the same two the views read,
// derived from internal/store's layout accessors exactly as
// objsupersede.ReadRecords derives its own.
var (
	recordSpecsDir     = path.Dir(path.Dir(store.SpecDirRelPath(store.ZoneActive, "x")))
	recordConflictsDir = filepath.ToSlash(filepath.Dir(store.ConflictPath("", "x")))
)

// recordsDigest is a content digest of every entry under the two record
// directories of tr — path, regularity, and a regular file's bytes — so a
// working tree and a commit with the same records digest the same, and
// any record byte, a new or removed record, or a symlink on a record path
// changes it. Files outside those directories (the data zone, the body of
// the store) never move it.
func recordsDigest(ctx context.Context, tr objsupersede.TreeReader) (string, error) {
	var files []objsupersede.TreeFile
	for _, dir := range []string{recordSpecsDir, recordConflictsDir} {
		listed, err := tr.Files(ctx, dir)
		if err != nil {
			return "", err
		}
		files = append(files, listed...)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	h := sha256.New()
	for _, f := range files {
		fmt.Fprintf(h, "%s\x00%v\x00", f.Path, f.Regular)
		if !f.Regular {
			continue
		}
		data, err := tr.ReadFile(ctx, f.Path)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%d\x00", len(data))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// attachObjectSupersession enriches proj (SI-278) in the I/O tier, after
// buildProjection, exactly as attachFamilyLinks does: each reference card
// whose target is a closed spec's object that the default-branch views
// report lines for gains the object's original text and those lines, and
// each decision card gains the views of its fragment `supersedes` edges
// from the board's own tree. ix resolves link targets; fixedBranch keeps
// a per-branch board's links inside its branch (ADJ-70).
func attachObjectSupersession(ctx context.Context, proj *BoardProjection, ix *index.Index, root, fixedBranch string) error {
	b, err := boardSupersession.boards(ctx, root)
	if err != nil {
		return fmt.Errorf("workbench: closed-spec object supersession views for %s: %w", proj.Spec, err)
	}
	objects := b.def
	if objects == nil {
		objects = b.tree
	}
	for i := range proj.RefCards {
		rc := &proj.RefCards[i]
		ref, err := artifact.ParseRef(rc.Ref)
		if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || !ref.Fragment() {
			continue
		}
		v := objects.index.Object(ref.Name, ref.Object)
		if v.State == objsupersede.ObjectNotSuperseded {
			continue
		}
		s, err := specdoc.ObjectSupersession(v, boardSupersessionLinks(proj.Spec, ix, fixedBranch, v.By, v.Revision, v.Conflict))
		if err != nil {
			return fmt.Errorf("workbench: %s on %s: %w", rc.Ref, proj.Spec, err)
		}
		rc.Object = &refObjectView{Text: objectText(objects.recs, ref), Supersession: objectSupersessionView(v, *s)}
	}
	for i := range proj.Cards {
		c := &proj.Cards[i]
		if c.Kind != string(boardlayout.ZoneDecision) {
			continue
		}
		for _, v := range b.tree.index.Decisions(proj.Spec, c.ID) {
			s, err := specdoc.DecisionSupersession(v, boardSupersessionLinks(proj.Spec, ix, fixedBranch, v.Object, v.Establisher, v.Conflict))
			if err != nil {
				return fmt.Errorf("workbench: %s#%s: %w", proj.Spec, c.ID, err)
			}
			c.Supersessions = append(c.Supersessions, decisionSupersessionView(v, s))
		}
	}
	return nil
}

// objectText is the declared text of spec/T#o in recs — the closed spec's
// archived bytes, which never change — or "" when the records do not
// carry it (an undecodable tree, SI-274(6), whose view reads unproven).
func objectText(recs *objsupersede.Records, ref artifact.Ref) string {
	if recs == nil || recs.Specs[ref.Name] == nil || recs.Specs[ref.Name].FM == nil {
		return ""
	}
	fm := recs.Specs[ref.Name].FM
	for _, ac := range fm.AcceptanceCriteria {
		if ac.ID == ref.Object {
			return ac.Text
		}
	}
	for _, d := range fm.Decisions {
		if d.ID == ref.Object {
			return d.Text
		}
	}
	return ""
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
		lines = append(lines, supersessionLineView{Kind: l.Kind, Text: l.Text, Links: supersessionLinks(l.Links), Trailing: supersessionLinks(l.Trailing)})
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
		line := specdoc.SupersessionLine{Kind: l.Kind, Text: l.Text}
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
