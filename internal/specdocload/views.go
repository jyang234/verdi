package specdocload

// Closed-spec object supersession views for every document consumer
// (design §6; SI-263, SI-278, SI-279; the controller's I-1 ruling on lane
// L5's docs-site review): the objsupersede view index of ONE tree,
// computed once and cached per (root, tree identity), supplied by Load to
// every document as Facts.Supersession — so the CLI, MCP, the docs site
// and the board's Document tab render one set of bytes (spec/spec-documents
// ac-6) — and handed to the board's cards through BoardIndexes. The cache
// is the one process-wide cache: a commit's views are keyed by the commit,
// a working tree's by a content digest of its records, so an unchanged
// tree never recomputes, a moved head or an edited record does; it is a
// bounded LRU with single-flight builds, so concurrent loads of one tree
// share one build instead of racing a duplicate.

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
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// Views are one tree's closed-spec object supersession views: the index,
// the records it was built from (a surface renders an object's original
// text from them), and, on demand, the records' content digest (Digest).
type Views struct {
	Records *objsupersede.Records
	Index   *objsupersede.Index

	tree   objsupersede.TreeReader
	mu     sync.Mutex
	digest string
}

// Digest is the content digest of the records the views were built from
// (recordsDigest), computed once on first use; a working tree's views
// carry it from their cache key.
func (v *Views) Digest(ctx context.Context) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.digest == "" {
		d, err := recordsDigest(ctx, v.tree)
		if err != nil {
			return "", err
		}
		v.digest = d
	}
	return v.digest, nil
}

// viewCache is the bounded, single-flight LRU behind CommitViews,
// WorkTreeViews and BoardIndexes.
type viewCache struct {
	mu       sync.Mutex
	limit    int
	entries  map[string]*Views
	order    []string // keys, least recently used first
	inflight map[string]*viewCall
	builds   atomic.Int32
}

type viewCall struct {
	done  chan struct{}
	views *Views
	err   error
}

func newViewCache(limit int) *viewCache {
	return &viewCache{limit: limit, entries: map[string]*Views{}, inflight: map[string]*viewCall{}}
}

// views is the process-wide cache every consumer shares.
var views = newViewCache(8)

// ViewBuilds reports how many view indexes this process has built — a
// test's witness that a build or a load computed the index once.
func ViewBuilds() int32 { return views.builds.Load() }

// get returns key's views, building them with build on a miss. A build in
// flight for the same key is joined, never duplicated; a failed build is
// returned to every waiter and caches nothing. The build runs under a
// context detached from the leader's cancellation, so a waiter never
// inherits a stranger's cancel; each waiter still honors its own.
func (c *viewCache) get(ctx context.Context, key string, build func(context.Context) (*Views, error)) (*Views, error) {
	c.mu.Lock()
	if v, ok := c.entries[key]; ok {
		c.touch(key)
		c.mu.Unlock()
		return v, nil
	}
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			return call.views, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &viewCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	c.builds.Add(1)
	v, err := build(context.WithoutCancel(ctx))
	c.mu.Lock()
	delete(c.inflight, key)
	if err == nil {
		c.entries[key] = v
		c.touch(key)
		c.evict()
	}
	c.mu.Unlock()
	call.views, call.err = v, err
	close(call.done)
	return v, err
}

// touch marks key most recently used; c.mu held.
func (c *viewCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}

// evict drops least recently used entries past the limit; c.mu held.
func (c *viewCache) evict() {
	for len(c.entries) > c.limit && len(c.order) > 0 {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}

func viewKey(root, kind, id string) string {
	return root + "\x00" + kind + "\x00" + id
}

// CommitViews returns the views of the records at commit (any revision git
// resolves; it is resolved to its full id for the key) in root's
// repository, against the default branch's history at root.
func CommitViews(ctx context.Context, root, commit string) (*Views, error) {
	full, err := gitx.RevParse(ctx, root, commit)
	if err != nil {
		return nil, fmt.Errorf("specdocload: resolving %q: %w", commit, err)
	}
	return views.get(ctx, viewKey(root, "commit", full), func(ctx context.Context) (*Views, error) {
		return buildViews(ctx, root, objsupersede.CommitTree{Root: root, Commit: full}, "")
	})
}

// WorkTreeViews returns the views of root's working-tree records, keyed by
// their content digest: an edit to any record is a new key, an unchanged
// tree a hit.
func WorkTreeViews(ctx context.Context, root string) (*Views, error) {
	tr := objsupersede.WorkTree{Root: root}
	digest, err := recordsDigest(ctx, tr)
	if err != nil {
		return nil, err
	}
	return views.get(ctx, viewKey(root, "tree", digest), func(ctx context.Context) (*Views, error) {
		return buildViews(ctx, root, tr, digest)
	})
}

// BoardViews are the two views a board reads (SI-278 as clarified):
// Default, the default branch's records at its head — nil when the
// default branch cannot be resolved, in which case the tree's own views,
// whose history facts all read unproven, serve both — and Tree, the board
// tree's own records: the very same *Views as Default when the tree's
// records equal the default branch's (the default-branch board: one
// index).
type BoardViews struct{ Default, Tree *Views }

// BoardIndexes returns root's board views, computing only what the cache
// lacks: the default-branch views once per head, the tree's once per
// records digest.
func BoardIndexes(ctx context.Context, root string) (BoardViews, error) {
	treeDigest, err := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	if err != nil {
		return BoardViews{}, err
	}
	buildTree := func(ctx context.Context) (*Views, error) {
		return buildViews(ctx, root, objsupersede.WorkTree{Root: root}, treeDigest)
	}
	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		tree, err := views.get(ctx, viewKey(root, "tree", treeDigest), buildTree)
		if err != nil {
			return BoardViews{}, err
		}
		return BoardViews{Tree: tree}, nil
	}
	def, err := CommitViews(ctx, root, branch.Ref)
	if err != nil {
		return BoardViews{}, err
	}
	defDigest, err := def.Digest(ctx)
	if err != nil {
		return BoardViews{}, err
	}
	if defDigest == treeDigest {
		return BoardViews{Default: def, Tree: def}, nil
	}
	tree, err := views.get(ctx, viewKey(root, "tree", treeDigest), buildTree)
	if err != nil {
		return BoardViews{}, err
	}
	return BoardViews{Default: def, Tree: tree}, nil
}

// buildViews reads tr's records and computes every view against the
// default branch's history at root. An unreadable tree or an index that
// cannot be computed is the caller's error: a surface never renders a
// superseded object as untouched because its records could not be read.
func buildViews(ctx context.Context, root string, tr objsupersede.TreeReader, digest string) (*Views, error) {
	recs, err := objsupersede.ReadRecords(ctx, tr)
	if err != nil {
		return nil, err
	}
	ix, err := objsupersede.NewIndex(ctx, recs, objsupersede.NewHistory(ctx, root))
	if err != nil {
		return nil, err
	}
	return &Views{Records: recs, Index: ix, tree: tr, digest: digest}, nil
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

// SupersessionFacts assembles spec name's document facts from v: the view
// of each criterion and decision a supersession touches, the views of
// each decision's fragment `supersedes` edges, and a link for every ref
// those views name that v's records carry a page for. A spec none of the
// views touch gets empty maps, which render exactly as before. The links
// are the same for every consumer (SupersessionLink), which is what keeps
// the four renders byte-identical.
func SupersessionFacts(v *Views, name string, fm *artifact.SpecFrontmatter) *specdoc.SupersessionFacts {
	f := &specdoc.SupersessionFacts{
		Objects:   map[string]objsupersede.ObjectView{},
		Decisions: map[string][]objsupersede.DecisionView{},
		Links:     map[string]string{},
	}
	if v == nil || fm == nil {
		return f
	}
	link := func(refs ...string) {
		for _, ref := range refs {
			if url := SupersessionLink(v, ref); url != "" {
				f.Links[ref] = url
			}
		}
	}
	object := func(id string) {
		if ov := v.Index.Object(name, id); ov.State != objsupersede.ObjectNotSuperseded {
			f.Objects[id] = ov
			link(ov.By, ov.Revision, ov.Conflict)
		}
	}
	for _, ac := range fm.AcceptanceCriteria {
		object(ac.ID)
	}
	for _, d := range fm.Decisions {
		object(d.ID)
		for _, dv := range v.Index.Decisions(name, d.ID) {
			f.Decisions[d.ID] = append(f.Decisions[d.ID], dv)
			link(dv.Object, dv.Establisher, dv.Conflict)
		}
	}
	return f
}

// SupersessionLink resolves a canonical, unpinned ref a view names to the
// one address every document consumer serves: an artifact's corpus page,
// "/a/<kind>/<name>" — the docs site's permalink and the workbench's
// corpus route alike — with, for a spec's object, the object's declared
// body anchor (02 §Object model; the id when none is declared). A spec or
// conflict v's records do not carry, a pinned ref, a conflict fragment, or
// anything else gets no link and renders as plain text.
func SupersessionLink(v *Views, ref string) string {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Pinned() || v == nil || v.Records == nil {
		return ""
	}
	page := "/a/" + string(r.Kind) + "/" + r.Name
	switch r.Kind {
	case artifact.KindSpec:
		s := v.Records.Specs[r.Name]
		if s == nil || s.FM == nil {
			return ""
		}
		if r.Object == "" {
			return page
		}
		anchor, ok := objectAnchor(s.FM, r.Object)
		if !ok {
			return ""
		}
		return page + "#" + anchor
	case artifact.KindConflict:
		if r.Object != "" {
			return ""
		}
		for _, c := range v.Records.Conflicts {
			if c.Name == r.Name {
				return page
			}
		}
	}
	return ""
}

// objectAnchor is the declared body anchor of fm's criterion or decision
// id, without a leading "#", or the id itself when none is declared; ok is
// false when fm declares no such object.
func objectAnchor(fm *artifact.SpecFrontmatter, id string) (string, bool) {
	anchor := ""
	found := false
	for _, ac := range fm.AcceptanceCriteria {
		if ac.ID == id {
			anchor, found = ac.Anchor, true
		}
	}
	for _, d := range fm.Decisions {
		if d.ID == id {
			anchor, found = d.Anchor, true
		}
	}
	if !found {
		return "", false
	}
	anchor = strings.TrimPrefix(anchor, "#")
	if anchor == "" {
		anchor = id
	}
	return anchor, true
}

// ObjectText is the declared text of spec/T#o in v's records — a closed
// spec's archived bytes, which never change — or "" when the records do
// not carry it (an undecodable tree, SI-274(6), whose view reads unproven).
func ObjectText(v *Views, ref artifact.Ref) string {
	if v == nil || v.Records == nil || v.Records.Specs[ref.Name] == nil || v.Records.Specs[ref.Name].FM == nil {
		return ""
	}
	fm := v.Records.Specs[ref.Name].FM
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
