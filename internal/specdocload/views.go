package specdocload

// Closed-spec object supersession views for every document consumer
// (design §6; SI-263, SI-278, SI-279; the controller's I-1 ruling on lane
// L5's docs-site review): the objsupersede view index of ONE tree,
// computed once and cached, supplied by Load to every document as
// Facts.Supersession — so the CLI, MCP, the docs site and the board's
// Document tab render one set of bytes (spec/spec-documents ac-6) — and
// handed to the board's cards through BoardIndexes.
//
// Every view depends on the default branch's history (acceptance, the
// closed date, carrying), so every cache key carries the RESOLVED
// DEFAULT-BRANCH HEAD (or "unresolved") beside the tree's identity: a
// commit's views are keyed by the repository (its git common dir, shared
// by every worktree), the commit and the head; a working tree's by the
// root, a content digest of its records and the head. An unchanged tree
// under an unchanged head never recomputes; a moved head, a default
// branch that becomes resolvable, or an edited record does. The digest
// is taken from the exact bytes the index decodes (one snapshot read
// feeds both), so a record edited between two reads can never be cached
// under the wrong key. The cache is a bounded LRU with single-flight
// builds, so concurrent loads of one tree share one build.

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
// text from them), and the content digest of those records' bytes.
type Views struct {
	Records *objsupersede.Records
	Index   *objsupersede.Index
	digest  string
}

// Digest is the content digest of the record bytes the views were built
// from (recordsDigest).
func (v *Views) Digest() string { return v.digest }

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

// viewKey names one cache entry: the tree's home (a repository for a
// commit, a root for a working tree), its kind, its identity (the commit,
// or the records digest) and the default-branch head the views were
// computed against.
func viewKey(home, kind, id, head string) string {
	return home + "\x00" + kind + "\x00" + id + "\x00" + head
}

// unresolvedHead is the key's head when the default branch cannot be
// resolved: every history fact then reads unproven, and the entry is
// invalidated the moment the branch resolves.
const unresolvedHead = "unresolved"

// defaultHistory is the history every view is computed against: the
// default branch's head commit at root — unresolvedHead when specstate
// cannot resolve the branch or its ref — and whether the repository's
// history is shallow. Shallowness is a history input too (History reads
// it for acceptance and closed dates), so it is part of every key:
// deepening a clone in place (`git fetch --unshallow`) is a new key and
// the stale "shallow history" views are never served again.
type defaultHistory struct {
	head     string // a full commit id, or unresolvedHead
	resolved bool
	shallow  bool // shallow, or unknowable (treated as shallow: unproven)
	// branch is the default branch the head was resolved from (zero when
	// it could not be resolved) — never part of the key.
	branch specstate.Branch
}

// accepted is h as the AcceptedHead a Result exposes.
func (h defaultHistory) accepted() AcceptedHead {
	a := AcceptedHead{Branch: h.branch.Name, Ref: h.branch.Ref}
	if h.resolved {
		a.Commit = h.head
	}
	return a
}

// key is the history's component of a cache key — never a git revision.
func (h defaultHistory) key() string {
	if h.shallow {
		return h.head + "+shallow"
	}
	return h.head
}

// resolveDefaultHistory reads root's default history.
func resolveDefaultHistory(ctx context.Context, root string) defaultHistory {
	h := defaultHistory{head: unresolvedHead}
	if branch, ok := specstate.ResolveDefaultBranch(ctx, root); ok {
		h.branch = branch
		if sha, err := gitx.RevParse(ctx, root, branch.Ref); err == nil {
			h.head, h.resolved = sha, true
		}
	}
	if shallow, err := gitx.IsShallow(ctx, root); err != nil || shallow {
		h.shallow = true
	}
	return h
}

// repoKey is the store's identity for a commit's views: the repository's
// git common dir — the one directory the main worktree and every linked
// worktree share, resolved through symlinks so two spellings of one path
// (a temp dir and its realpath) key alike — joined with the store's
// repo-relative prefix, since a commit's records are read relative to the
// store root and two stores in one repository must never share views. A
// directory git cannot answer for falls back to root itself.
func repoKey(ctx context.Context, root string) string {
	dir, err := gitx.CommonDir(ctx, root)
	if err != nil {
		return root
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	dir = filepath.Clean(dir)
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	prefix, err := gitx.RepoPrefix(ctx, root)
	if err != nil {
		return root
	}
	return dir + "\x00" + prefix
}

// CommitViews returns the views of the records at commit (any revision git
// resolves; it is resolved to its full id for the key) in root's
// repository, against the default branch's history at root.
func CommitViews(ctx context.Context, root, commit string) (*Views, error) {
	v, _, err := commitViewsWithHistory(ctx, root, commit)
	return v, err
}

// commitViewsWithHistory is CommitViews plus the default history it was
// computed against, which Load exposes (Result.Accepted).
func commitViewsWithHistory(ctx context.Context, root, commit string) (*Views, defaultHistory, error) {
	full, err := gitx.RevParse(ctx, root, commit)
	if err != nil {
		return nil, defaultHistory{}, fmt.Errorf("specdocload: resolving %q: %w", commit, err)
	}
	h := resolveDefaultHistory(ctx, root)
	v, err := commitViews(ctx, root, full, h.key())
	return v, h, err
}

// commitViews is CommitViews with the commit and the history key resolved.
func commitViews(ctx context.Context, root, full, historyKey string) (*Views, error) {
	return views.get(ctx, viewKey(repoKey(ctx, root), "commit", full, historyKey), func(ctx context.Context) (*Views, error) {
		snap, digest, err := snapshotRecords(ctx, objsupersede.CommitTree{Root: root, Commit: full})
		if err != nil {
			return nil, err
		}
		return buildViews(ctx, root, snap, digest)
	})
}

// WorkTreeViews returns the views of root's working-tree records, keyed by
// their content digest and the default head: an edit to any record, or a
// moved head, is a new key; an unchanged tree under an unchanged head a
// hit.
func WorkTreeViews(ctx context.Context, root string) (*Views, error) {
	v, _, err := workTreeViewsWithHistory(ctx, root)
	return v, err
}

// workTreeViewsWithHistory is WorkTreeViews plus the default history it
// was computed against, which Load exposes (Result.Accepted).
func workTreeViewsWithHistory(ctx context.Context, root string) (*Views, defaultHistory, error) {
	snap, digest, err := snapshotRecords(ctx, objsupersede.WorkTree{Root: root})
	if err != nil {
		return nil, defaultHistory{}, err
	}
	h := resolveDefaultHistory(ctx, root)
	v, err := views.get(ctx, viewKey(root, "tree", digest, h.key()), func(ctx context.Context) (*Views, error) {
		return buildViews(ctx, root, snap, digest)
	})
	return v, h, err
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
// (records digest, head).
func BoardIndexes(ctx context.Context, root string) (BoardViews, error) {
	snap, treeDigest, err := snapshotRecords(ctx, objsupersede.WorkTree{Root: root})
	if err != nil {
		return BoardViews{}, err
	}
	h := resolveDefaultHistory(ctx, root)
	buildTree := func(ctx context.Context) (*Views, error) {
		return buildViews(ctx, root, snap, treeDigest)
	}
	if !h.resolved {
		tree, err := views.get(ctx, viewKey(root, "tree", treeDigest, h.key()), buildTree)
		if err != nil {
			return BoardViews{}, err
		}
		return BoardViews{Tree: tree}, nil
	}
	def, err := commitViews(ctx, root, h.head, h.key())
	if err != nil {
		return BoardViews{}, err
	}
	if def.Digest() == treeDigest {
		return BoardViews{Default: def, Tree: def}, nil
	}
	tree, err := views.get(ctx, viewKey(root, "tree", treeDigest, h.key()), buildTree)
	if err != nil {
		return BoardViews{}, err
	}
	return BoardViews{Default: def, Tree: tree}, nil
}

// buildViews decodes snap's records and computes every view against the
// default branch's history at root. A tree that cannot be decoded as a
// whole or an index that cannot be computed is the caller's error: a
// surface never renders a superseded object as untouched because its
// records could not be read.
func buildViews(ctx context.Context, root string, snap *memTree, digest string) (*Views, error) {
	recs, err := objsupersede.ReadRecords(ctx, snap)
	if err != nil {
		return nil, err
	}
	ix, err := objsupersede.NewIndex(ctx, recs, objsupersede.NewHistory(ctx, root))
	if err != nil {
		return nil, err
	}
	return &Views{Records: recs, Index: ix, digest: digest}, nil
}

// The record directories a snapshot covers: the same two the views read,
// derived from internal/store's layout accessors exactly as
// objsupersede.ReadRecords derives its own.
var (
	recordSpecsDir     = path.Dir(path.Dir(store.SpecDirRelPath(store.ZoneActive, "x")))
	recordConflictsDir = filepath.ToSlash(filepath.Dir(store.ConflictPath("", "x")))
)

// memTree is one snapshot of a tree's record directories — the listings
// and the bytes read once — served back as a TreeReader, so the index
// decodes exactly the bytes the digest covers.
type memTree struct {
	lists map[string][]objsupersede.TreeFile
	files map[string]memFile
}

type memFile struct {
	regular bool
	data    []byte
}

// Files implements objsupersede.TreeReader over the snapshot's listings;
// a directory the snapshot never listed is an error, never an empty tree.
func (m *memTree) Files(_ context.Context, dir string) ([]objsupersede.TreeFile, error) {
	listed, ok := m.lists[dir]
	if !ok {
		return nil, fmt.Errorf("specdocload: the record snapshot has no listing for %q", dir)
	}
	return append([]objsupersede.TreeFile(nil), listed...), nil
}

// ReadFile implements objsupersede.TreeReader over the snapshot's bytes.
func (m *memTree) ReadFile(_ context.Context, p string) ([]byte, error) {
	f, ok := m.files[p]
	if !ok || !f.regular {
		return nil, fmt.Errorf("specdocload: %s is not a regular file of the record snapshot", p)
	}
	return f.data, nil
}

// snapshotRecords reads every entry under the two record directories of
// tr once — path, regularity, and a regular file's bytes, in a fixed
// order — into a snapshot, and digests exactly what it read: a working
// tree and a commit with the same records digest the same; any record
// byte, a new or removed record, or a symlink on a record path changes
// it; files outside those directories never move it.
func snapshotRecords(ctx context.Context, tr objsupersede.TreeReader) (*memTree, string, error) {
	m := &memTree{lists: map[string][]objsupersede.TreeFile{}, files: map[string]memFile{}}
	h := sha256.New()
	for _, dir := range []string{recordSpecsDir, recordConflictsDir} {
		listed, err := tr.Files(ctx, dir)
		if err != nil {
			return nil, "", err
		}
		sorted := append([]objsupersede.TreeFile(nil), listed...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
		m.lists[dir] = sorted
		for _, f := range sorted {
			fmt.Fprintf(h, "%s\x00%v\x00", f.Path, f.Regular)
			if !f.Regular {
				m.files[f.Path] = memFile{}
				continue
			}
			data, err := tr.ReadFile(ctx, f.Path)
			if err != nil {
				return nil, "", err
			}
			fmt.Fprintf(h, "%d\x00", len(data))
			h.Write(data)
			m.files[f.Path] = memFile{regular: true, data: data}
		}
	}
	return m, hex.EncodeToString(h.Sum(nil)), nil
}

// recordsDigest is the content digest of tr's records (snapshotRecords).
func recordsDigest(ctx context.Context, tr objsupersede.TreeReader) (string, error) {
	_, digest, err := snapshotRecords(ctx, tr)
	return digest, err
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
// body anchor (ObjectAnchor). A spec or conflict v's records do not carry,
// a pinned ref, a conflict fragment, or anything else gets no link and
// renders as plain text.
func SupersessionLink(v *Views, ref string) string {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Pinned() || v == nil || v.Records == nil {
		return ""
	}
	page := "/a/" + string(r.Kind) + "/" + r.Name
	switch r.Kind {
	case artifact.KindSpec:
		if v.Records.Specs[r.Name] == nil {
			return ""
		}
		if r.Object == "" {
			return page
		}
		anchor, ok := ObjectAnchor(v, r)
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

// ObjectAnchor is the declared body anchor (02 §Object model) of the
// criterion or decision ref names in v's records, without a leading "#",
// or the object's id when none is declared — the fragment every consumer
// links on the spec's corpus page; ok is false when the records do not
// declare the object.
func ObjectAnchor(v *Views, ref artifact.Ref) (string, bool) {
	if v == nil || v.Records == nil || v.Records.Specs[ref.Name] == nil || v.Records.Specs[ref.Name].FM == nil {
		return "", false
	}
	return objectAnchor(v.Records.Specs[ref.Name].FM, ref.Object)
}

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
