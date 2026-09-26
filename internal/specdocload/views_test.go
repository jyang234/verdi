package specdocload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/specdoc"
)

// The shared closed-spec object supersession view cache (design §6;
// SI-278; the controller's I-1 ruling): one process-wide cache keyed by
// (root, tree identity) — a commit, or a working tree's records digest —
// bounded, single-flight; the loader supplies every document its views
// from it, and the board reads its two views through it.

// viewsEnv neutralizes the CI environment the default-branch resolution
// consults, so a scenario store's origin/main is the one default branch.
func viewsEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
}

// cacheGet drives the cache's core with a counting build.
func cacheGet(t *testing.T, c *viewCache, key string, builds *atomic.Int32, delay time.Duration) *Views {
	t.Helper()
	v, err := c.get(context.Background(), key, func(context.Context) (*Views, error) {
		builds.Add(1)
		time.Sleep(delay)
		return &Views{digest: key}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestViewCache_HitsAndBound(t *testing.T) {
	c := newViewCache(3)
	var builds atomic.Int32
	a := cacheGet(t, c, "a", &builds, 0)
	if again := cacheGet(t, c, "a", &builds, 0); again != a || builds.Load() != 1 {
		t.Fatalf("a second get of the same key built again: builds=%d same=%v", builds.Load(), again == a)
	}
	// The bound: a fourth distinct key evicts the least recently used one;
	// "a" was touched most recently below, so "b" goes.
	cacheGet(t, c, "b", &builds, 0)
	cacheGet(t, c, "c", &builds, 0)
	cacheGet(t, c, "a", &builds, 0)
	cacheGet(t, c, "d", &builds, 0)
	if builds.Load() != 4 || len(c.entries) != 3 {
		t.Fatalf("builds = %d, entries = %d; want 4 builds within the bound 3", builds.Load(), len(c.entries))
	}
	cacheGet(t, c, "a", &builds, 0)
	cacheGet(t, c, "b", &builds, 0)
	if builds.Load() != 5 {
		t.Fatalf("builds = %d, want 5 (b rebuilt after eviction, a still cached)", builds.Load())
	}
	if _, err := c.get(context.Background(), "x", func(context.Context) (*Views, error) { return nil, errors.New("boom") }); err == nil {
		t.Fatal("get returned no error for a failing build")
	}
	if _, ok := c.entries["x"]; ok {
		t.Fatal("a failed build was cached")
	}
	if len(c.inflight) != 0 {
		t.Fatalf("inflight = %d after every get returned, want 0", len(c.inflight))
	}
}

func TestViewCache_SingleFlight(t *testing.T) {
	c := newViewCache(4)
	var builds atomic.Int32
	var wg sync.WaitGroup
	results := make([]*Views, 16)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = cacheGet(t, c, "shared", &builds, 50*time.Millisecond)
		}(i)
	}
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1: concurrent gets of one key share one build", builds.Load())
	}
	for i, r := range results {
		if r != results[0] {
			t.Fatalf("result %d is a different entry than result 0", i)
		}
	}
	var attempts, errs atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.get(context.Background(), "failing", func(context.Context) (*Views, error) {
				attempts.Add(1)
				time.Sleep(20 * time.Millisecond)
				return nil, errors.New("boom")
			})
			if err != nil {
				errs.Add(1)
			}
		}()
	}
	wg.Wait()
	if attempts.Load() != 1 || errs.Load() != 8 {
		t.Fatalf("attempts = %d, errs = %d; want one shared attempt and every waiter failed", attempts.Load(), errs.Load())
	}
	if _, ok := c.entries["failing"]; ok {
		t.Fatal("a failed single-flight build was cached")
	}
}

// TestBoardIndexes drives the real cache over a scenario store: one index
// when the working tree is the default branch's tree, no rebuild on an
// unchanged tree, a separate working-tree index when the records
// diverge, the alias back when they agree again, and a rebuild when the
// default-branch head moves.
func TestBoardIndexes(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	before := ViewBuilds()

	b, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Default == nil || b.Tree != b.Default {
		t.Fatalf("on the default branch's clean checkout the tree views must be the default-branch views: def=%p tree=%p", b.Default, b.Tree)
	}
	if v := b.Default.Index.Object("closed-feature", "dc-1"); v.State != objsupersede.ObjectSuperseded {
		t.Fatalf("closed-feature#dc-1 = %+v, want superseded", v)
	}
	if b.Default.Records.Specs["closed-feature"] == nil {
		t.Fatal("the views carry no records")
	}
	if got := ViewBuilds() - before; got != 1 {
		t.Fatalf("builds = %d after the first BoardIndexes, want 1", got)
	}
	if _, err := BoardIndexes(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	}
	if got := ViewBuilds() - before; got != 1 {
		t.Fatalf("builds = %d after an unchanged tree, want still 1 (never recompute per request)", got)
	}

	// An uncommitted edit to a record: the tree gets its own views while
	// the default-branch views stay.
	other := filepath.Join(repo.Dir, ".verdi", "specs", "active", "other-feature", "spec.md")
	raw, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, append(raw, []byte("\nEdited in the working tree.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b2.Default != b.Default || b2.Tree == b2.Default || ViewBuilds()-before != 2 {
		t.Fatalf("an edited tree must keep the default views and build its own once: def same=%v tree aliased=%v builds=%d", b2.Default == b.Default, b2.Tree == b2.Default, ViewBuilds()-before)
	}
	if _, err := BoardIndexes(ctx, repo.Dir); err != nil || ViewBuilds()-before != 2 {
		t.Fatalf("the same edited tree rebuilt: err=%v builds=%d", err, ViewBuilds()-before)
	}
	if err := os.WriteFile(other, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	b3, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b3.Tree != b.Default || ViewBuilds()-before != 2 {
		t.Fatalf("a reverted tree must alias the default views without a build: aliased=%v builds=%d", b3.Tree == b.Default, ViewBuilds()-before)
	}

	// The default-branch head moves (a commit on main, fetched — the store
	// resolves its default branch through refs/remotes/origin/main).
	gitIn(t, repo.Dir, "commit", "--allow-empty", "-q", "-m", "move main")
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	b4, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b4.Default == b.Default || ViewBuilds()-before != 3 || b4.Tree != b4.Default {
		t.Fatalf("a moved head must rebuild the default views once and the clean tree alias them: rebuilt=%v builds=%d aliased=%v", b4.Default != b.Default, ViewBuilds()-before, b4.Tree == b4.Default)
	}
}

// TestBoardIndexes_NoDefaultBranch: without a provable default branch the
// tree's own views serve both cards and every history fact reads
// unproven — never silence, never a guess.
func TestBoardIndexes_NoDefaultBranch(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	files := map[string]string{}
	src := scenario.Build(t, "accepted").Dir
	for _, rel := range []string{".verdi/verdi.yaml", ".verdi/specs/archive/closed-feature/spec.md", ".verdi/specs/active/successor/spec.md", ".verdi/conflicts/successor-closed-feature.md"} {
		data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		files[rel] = string(data)
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "no origin"}})
	b, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Default != nil || b.Tree == nil {
		t.Fatalf("without a default branch: def=%v tree=%v; want no default views and tree views", b.Default != nil, b.Tree != nil)
	}
	if v := b.Tree.Index.Object("closed-feature", "dc-1"); v.State != objsupersede.ObjectUnproven {
		t.Fatalf("closed-feature#dc-1 = %+v, want unproven", v)
	}
}

func TestCommitViews_Errors(t *testing.T) {
	viewsEnv(t)
	repo := scenario.Build(t, "accepted")
	if _, err := CommitViews(context.Background(), repo.Dir, "no-such-commit"); err == nil {
		t.Fatal("CommitViews accepted an unresolvable commit")
	}
	if _, err := CommitViews(context.Background(), t.TempDir(), "HEAD"); err == nil {
		t.Fatal("CommitViews accepted a directory that is not a repository")
	}
}

func TestRecordsDigest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	digest := func() string {
		t.Helper()
		d, err := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	write(".verdi/specs/active/a/spec.md", "a")
	write(".verdi/conflicts/c.md", "c")
	write(".verdi/data/mutable/x.json", "ignored")
	d1 := digest()
	write(".verdi/data/mutable/x.json", "changed")
	if d1 == "" || digest() != d1 {
		t.Fatalf("digest not deterministic or not blind to the data zone: %q %q", d1, digest())
	}
	seen := map[string]bool{d1: true}
	for _, step := range []func(){
		func() { write(".verdi/specs/active/a/spec.md", "a2") },
		func() { write(".verdi/specs/archive/b/spec.md", "b") },
		func() { write(".verdi/conflicts/d.md", "d") },
		func() {
			if err := os.Symlink("c.md", filepath.Join(root, ".verdi", "conflicts", "e.md")); err != nil {
				t.Fatal(err)
			}
		},
	} {
		step()
		d := digest()
		if seen[d] {
			t.Fatalf("a record change left the digest unchanged: %q", d)
		}
		seen[d] = true
	}
	// The same records at a commit digest the same as in the working tree.
	viewsEnv(t)
	repo := scenario.Build(t, "accepted")
	wt, err := recordsDigest(ctx, objsupersede.WorkTree{Root: repo.Dir})
	if err != nil {
		t.Fatal(err)
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	ct, err := recordsDigest(ctx, objsupersede.CommitTree{Root: repo.Dir, Commit: head})
	if err != nil {
		t.Fatal(err)
	}
	if wt != ct {
		t.Fatalf("working tree %q and its commit %q digest differently", wt, ct)
	}
	if _, err := recordsDigest(ctx, objsupersede.CommitTree{Root: repo.Dir, Commit: "no-such-commit"}); err == nil {
		t.Fatal("an unresolvable commit digested without error")
	}
}

func TestSupersessionLink(t *testing.T) {
	viewsEnv(t)
	v, err := WorkTreeViews(context.Background(), scenario.Build(t, "accepted").Dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, ref, want string
	}{
		{"a spec's decision: its corpus page at the declared anchor", "spec/successor#dc-1", "/a/spec/successor#dc-1"},
		{"a closed spec's criterion", "spec/closed-story#ac-1", "/a/spec/closed-story#ac-1"},
		{"a whole spec", "spec/successor", "/a/spec/successor"},
		{"a conflict", "conflict/successor-closed-feature", "/a/conflict/successor-closed-feature"},
		{"a conflict fragment", "conflict/successor-closed-feature#x", ""},
		{"an undeclared object", "spec/successor#dc-9", ""},
		{"a spec the tree lacks", "spec/nowhere#dc-1", ""},
		{"a conflict the tree lacks", "conflict/nowhere", ""},
		{"a pinned ref is never linked", "spec/successor@0123abc#dc-1", ""},
		{"not a ref", "nonsense", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SupersessionLink(v, tc.ref); got != tc.want {
				t.Errorf("SupersessionLink(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
	if got := SupersessionLink(nil, "spec/successor#dc-1"); got != "" {
		t.Errorf("nil views linked %q", got)
	}
	// A declared anchor with a leading "#" and a missing anchor.
	fm := &artifact.SpecFrontmatter{Decisions: []artifact.Decision{{ID: "dc-1", Anchor: "#dc-one"}, {ID: "dc-2"}}}
	for _, tc := range []struct{ id, want string }{{"dc-1", "dc-one"}, {"dc-2", "dc-2"}} {
		if got, ok := objectAnchor(fm, tc.id); !ok || got != tc.want {
			t.Errorf("objectAnchor(%s) = %q, %v; want %q", tc.id, got, ok, tc.want)
		}
	}
	if _, ok := objectAnchor(fm, "dc-3"); ok {
		t.Error("objectAnchor found an undeclared object")
	}
}

func TestSupersessionFacts(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	v, err := WorkTreeViews(ctx, scenario.Build(t, "accepted").Dir)
	if err != nil {
		t.Fatal(err)
	}
	closed := v.Records.Specs["closed-feature"].FM
	f := SupersessionFacts(v, "closed-feature", closed)
	if ov, ok := f.Objects["dc-1"]; !ok || ov.State != objsupersede.ObjectSuperseded || ov.By != "spec/successor#dc-1" {
		t.Fatalf("closed-feature facts = %+v", f.Objects)
	}
	if _, ok := f.Objects["ac-1"]; ok {
		t.Error("closed-feature#ac-1 is not superseded but gained a view")
	}
	if len(f.Decisions["dc-1"]) != 0 {
		t.Errorf("a closed spec's decision gained edge views: %+v", f.Decisions)
	}
	wantLinks := map[string]string{"spec/successor#dc-1": "/a/spec/successor#dc-1", "conflict/successor-closed-feature": "/a/conflict/successor-closed-feature"}
	if !reflect.DeepEqual(f.Links, wantLinks) {
		t.Errorf("links = %+v, want %+v", f.Links, wantLinks)
	}
	succ := SupersessionFacts(v, "successor", v.Records.Specs["successor"].FM)
	if len(succ.Decisions["dc-1"]) != 1 || succ.Decisions["dc-1"][0].State != objsupersede.DecisionInForce || len(succ.Objects) != 0 {
		t.Errorf("successor facts = %+v / %+v", succ.Decisions, succ.Objects)
	}
	if succ.Links["spec/closed-feature#dc-1"] != "/a/spec/closed-feature#dc-1" {
		t.Errorf("successor links = %+v", succ.Links)
	}
	// Empty, never nil, for a spec no view touches; nil inputs are harmless.
	other := SupersessionFacts(v, "other-feature", v.Records.Specs["other-feature"].FM)
	if len(other.Objects) != 0 || len(other.Decisions) != 0 || len(other.Links) != 0 {
		t.Errorf("other-feature facts = %+v", other)
	}
	if f := SupersessionFacts(nil, "x", nil); f == nil || f.Objects == nil {
		t.Errorf("nil views gave %+v", f)
	}
	if got := ObjectText(v, artifact.Ref{Kind: artifact.KindSpec, Name: "closed-feature", Object: "dc-1"}); got != "the governed records are listed newest first" {
		t.Errorf("ObjectText = %q", got)
	}
	if got := ObjectText(v, artifact.Ref{Kind: artifact.KindSpec, Name: "closed-feature", Object: "dc-9"}); got != "" {
		t.Errorf("ObjectText of an undeclared object = %q", got)
	}
}

// TestLoad_SuppliesSupersession: Load supplies every consumer the views
// from the tree the document is read from — the commit for ModeAccepted
// and ModeAt, the checkout for ModeWorkingTree — and computes them once
// across loads of one tree.
func TestLoad_SuppliesSupersession(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	before := ViewBuilds()
	var docs []string
	for _, req := range []Request{
		{Root: repo.Dir, Name: "closed-feature", Mode: ModeAccepted, Kind: specdoc.KindSpec},
		{Root: repo.Dir, Name: "closed-feature", Mode: ModeAt, At: "main", Kind: specdoc.KindSpec},
		{Root: repo.Dir, Name: "closed-feature", Mode: ModeWorkingTree, Kind: specdoc.KindSpec},
	} {
		res, err := Load(ctx, req)
		if err != nil {
			t.Fatalf("Load(%v): %v", req.Mode, err)
		}
		sup := res.Input.Facts.Supersession
		if sup == nil {
			t.Fatalf("Load(%v) supplied no supersession facts", req.Mode)
		}
		if v, ok := sup.Objects["dc-1"]; !ok || v.State != objsupersede.ObjectSuperseded {
			t.Fatalf("Load(%v): closed-feature#dc-1 = %+v", req.Mode, sup.Objects)
		}
		doc, err := specdoc.Build(res.Input)
		if err != nil {
			t.Fatal(err)
		}
		docs = append(docs, specdoc.RenderMarkdown(doc))
	}
	// One tree, one build: the commit views and the clean working tree's
	// views are distinct cache keys, so at most two builds, and a second
	// load of each is a hit.
	if got := ViewBuilds() - before; got < 1 || got > 2 {
		t.Fatalf("builds = %d across three loads of one tree, want 1 or 2", got)
	}
	mid := ViewBuilds()
	if _, err := Load(ctx, Request{Root: repo.Dir, Name: "successor", Mode: ModeAccepted, Kind: specdoc.KindSpec}); err != nil {
		t.Fatal(err)
	}
	if ViewBuilds() != mid {
		t.Fatalf("a load of another spec at the same commit rebuilt the views")
	}
	// The three renders are byte-identical except the working tree's
	// stamp names HEAD's commit rather than the default branch's; on this
	// clean main checkout those are one commit, so all three agree.
	if docs[0] != docs[1] || docs[0] != docs[2] {
		t.Fatalf("the modes rendered different bytes:\n--- accepted ---\n%s\n--- at ---\n%s\n--- working tree ---\n%s", docs[0], docs[1], docs[2])
	}
	if !strings.Contains(docs[0], `data-testid="objsupersede-dc-1-since"`) {
		t.Fatalf("the document lacks the since line:\n%s", docs[0])
	}
}

// gitIn runs git in dir with a fixed identity for a test fixture.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_AUTHOR_DATE=2024-03-01T00:00:00Z", "GIT_COMMITTER_DATE=2024-03-01T00:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The default-branch head in every key (lane L5 fix pass 2, item 1; the
// docs review's N-1 and the board review's I-1): a working tree's views,
// and any commit's, depend on default-branch history, so a merge that
// accepts the successor, an unrelated non-empty move of the head, or a
// default branch that becomes resolvable each rebuild — in the same
// process, with the tree's own bytes unchanged.
func TestViews_KeyedByDefaultBranchHead(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "proposed") // checkout design/successor
	decision := func(v *Views) objsupersede.DecisionState {
		t.Helper()
		ds := v.Index.Decisions("successor", "dc-1")
		if len(ds) != 1 {
			t.Fatalf("successor#dc-1 views = %+v", ds)
		}
		return ds[0].State
	}
	before := ViewBuilds()
	tree, err := WorkTreeViews(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if decision(tree) != objsupersede.DecisionProposed {
		t.Fatalf("before acceptance dc-1 = %s, want proposed", decision(tree))
	}
	branchCommit, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if v, err := CommitViews(ctx, repo.Dir, branchCommit); err != nil || decision(v) != objsupersede.DecisionProposed {
		t.Fatalf("CommitViews(design tip) = %v, %v; want proposed", err, decision(v))
	}
	built := ViewBuilds() - before

	// Accept the successor: a real, non-empty merge into main, fetched
	// (origin/main is the store's default branch), the checkout back on
	// the design branch with its bytes unchanged.
	gitIn(t, repo.Dir, "checkout", "-q", "main")
	gitIn(t, repo.Dir, "merge", "-q", "--no-ff", "-m", "Accept spec/successor", "design/successor")
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	gitIn(t, repo.Dir, "checkout", "-q", "design/successor")

	tree, err = WorkTreeViews(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if decision(tree) != objsupersede.DecisionInForce {
		t.Fatalf("after acceptance, the same process reads dc-1 = %s, want in-force (a stale entry)", decision(tree))
	}
	if v, err := CommitViews(ctx, repo.Dir, branchCommit); err != nil || decision(v) != objsupersede.DecisionInForce {
		t.Fatalf("CommitViews(design tip) after acceptance = %v, %v; want in-force", err, decision(v))
	}
	if res, err := Load(ctx, Request{Root: repo.Dir, Name: "successor", Mode: ModeWorkingTree, Kind: specdoc.KindSpec}); err != nil {
		t.Fatal(err)
	} else if got := res.Input.Facts.Supersession.Decisions["dc-1"]; len(got) != 1 || got[0].State != objsupersede.DecisionInForce {
		t.Fatalf("Load(ModeWorkingTree) after acceptance = %+v, want in-force", got)
	}
	if ViewBuilds()-before <= built {
		t.Fatalf("no rebuild after the head moved: builds %d then %d", built, ViewBuilds()-before)
	}
	built = ViewBuilds() - before

	// An UNRELATED non-empty move of the head rebuilds too: history may
	// have changed in ways the tree cannot see.
	gitIn(t, repo.Dir, "checkout", "-q", "main")
	other := filepath.Join(repo.Dir, ".verdi", "specs", "active", "other-feature", "spec.md")
	raw, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, append(raw, []byte("\nAn unrelated edit on main.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo.Dir, "commit", "-q", "-am", "Edit spec/other-feature on main")
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	gitIn(t, repo.Dir, "checkout", "-q", "design/successor")
	if _, err := WorkTreeViews(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	}
	if ViewBuilds()-before <= built {
		t.Fatalf("no rebuild after an unrelated head move: builds %d then %d", built, ViewBuilds()-before)
	}
	if _, err := WorkTreeViews(ctx, repo.Dir); err != nil || ViewBuilds()-before != built+1 {
		t.Fatalf("the same tree under the same head rebuilt again: err=%v builds=%d", err, ViewBuilds()-before)
	}
}

// TestBoardIndexes_UnresolvedThenResolved (the board review's witness B):
// with the default branch gone the tree's views serve both cards with
// every history fact unproven; once the branch is back — the notice's own
// remedy — the board reads the current facts, in the same process.
func TestBoardIndexes_UnresolvedThenResolved(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "chain-not-in-force") // checkout design/successor-v2
	dc2 := func(b BoardViews) objsupersede.DecisionView {
		t.Helper()
		ds := b.Tree.Index.Decisions("successor-v2", "dc-2")
		if len(ds) != 1 {
			t.Fatalf("successor-v2#dc-2 views = %+v", ds)
		}
		return ds[0]
	}
	gitIn(t, repo.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	b, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Default != nil || dc2(b).State != objsupersede.DecisionNotEstablished || !strings.Contains(dc2(b).Reason, "acceptance unproven") {
		t.Fatalf("unresolved default branch: Default=%v dc-2=%+v; want no Default and an acceptance-unproven reason", b.Default != nil, dc2(b))
	}
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	b, err = BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.Default == nil || dc2(b).State != objsupersede.DecisionProposed || !dc2(b).Carried {
		t.Fatalf("after the remedy the same process still reads: Default=%v dc-2=%+v; want Default and a proposed, carried dc-2", b.Default != nil, dc2(b))
	}
}

// TestCommitViews_SharedAcrossWorktrees (the board review's M-8): the
// default-branch views are keyed by the repository, not the worktree, so a
// second worktree of the same repository is a hit, not a second build.
func TestCommitViews_SharedAcrossWorktrees(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	wt := filepath.Join(t.TempDir(), "wt")
	gitIn(t, repo.Dir, "worktree", "add", "-q", "--detach", wt, "main")
	before := ViewBuilds()
	a, err := CommitViews(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatal(err)
	}
	b, err := CommitViews(ctx, wt, "main")
	if err != nil {
		t.Fatal(err)
	}
	if a != b || ViewBuilds()-before != 1 {
		t.Fatalf("two worktrees of one repository built %d times (same entry: %v), want one shared build", ViewBuilds()-before, a == b)
	}
	// Their default-branch board views are the same entry too.
	ba, err := BoardIndexes(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := BoardIndexes(ctx, wt)
	if err != nil {
		t.Fatal(err)
	}
	if ba.Default != bb.Default || ba.Default != a {
		t.Fatalf("board Default views differ across worktrees")
	}
	if ViewBuilds()-before != 1 {
		t.Fatalf("builds = %d across both boards, want still 1", ViewBuilds()-before)
	}
}

// flakyTree returns different bytes on each read of the same path — an
// edit landing between a digest pass and a decode pass.
type flakyTree struct {
	inner objsupersede.TreeReader
	reads atomic.Int32
}

func (f *flakyTree) Files(ctx context.Context, dir string) ([]objsupersede.TreeFile, error) {
	return f.inner.Files(ctx, dir)
}

func (f *flakyTree) ReadFile(ctx context.Context, p string) ([]byte, error) {
	data, err := f.inner.ReadFile(ctx, p)
	if err != nil {
		return nil, err
	}
	n := f.reads.Add(1)
	return append(append([]byte(nil), data...), []byte(fmt.Sprintf("\n# read %d\n", n))...), nil
}

// TestSnapshot_DigestCoversTheDecodedBytes (the board review's M-7): the
// digest and the index come from ONE read of each record, so a tree that
// changes between reads can never be cached under another tree's digest.
func TestSnapshot_DigestCoversTheDecodedBytes(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	tr := &flakyTree{inner: objsupersede.WorkTree{Root: repo.Dir}}
	snap, digest, err := snapshotRecords(ctx, tr)
	if err != nil {
		t.Fatal(err)
	}
	// The snapshot's own digest is the digest returned, and the records
	// decoded from it carry the very bytes that digest covers.
	again, err := recordsDigest(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	if again != digest {
		t.Fatalf("the snapshot digests %q, the read returned %q", again, digest)
	}
	recs, err := objsupersede.ReadRecords(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	for name, s := range recs.Specs {
		if !strings.Contains(string(s.Raw), "# read ") {
			t.Fatalf("spec/%s was decoded from bytes the snapshot never read", name)
		}
		if got := snap.files[s.Path].data; string(got) != string(s.Raw) {
			t.Fatalf("spec/%s decoded bytes differ from the snapshot's", name)
		}
	}
	// A second read of the flaky tree digests differently — the trees
	// differ — and never aliases the first.
	if d2, err := recordsDigest(ctx, tr); err != nil || d2 == digest {
		t.Fatalf("a changed tree digested the same: %v %v", d2 == digest, err)
	}
	// The snapshot refuses a directory it never listed.
	if _, err := snap.Files(ctx, "somewhere/else"); err == nil {
		t.Fatal("the snapshot listed a directory it never read")
	}
	if _, err := snap.ReadFile(ctx, "no/such/file"); err == nil {
		t.Fatal("the snapshot read a file it never had")
	}
}

// TestBoardIndexes_Errors (the board review's M-5): a default branch that
// resolves to something that is not a commit, and a record that cannot be
// read, each fail the lookup — never a board rendered as if the object
// were untouched.
func TestBoardIndexes_Errors(t *testing.T) {
	viewsEnv(t)
	ctx := context.Background()
	t.Run("the default branch names a blob", func(t *testing.T) {
		repo := scenario.Build(t, "accepted")
		blob, err := gitx.RevParse(ctx, repo.Dir, "HEAD:.verdi/verdi.yaml")
		if err != nil {
			t.Fatal(err)
		}
		gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", blob)
		if _, err := BoardIndexes(ctx, repo.Dir); err == nil {
			t.Fatal("BoardIndexes succeeded with a default branch that is not a commit")
		}
		if _, err := WorkTreeViews(ctx, repo.Dir); err != nil {
			t.Fatalf("the working tree's own views must still compute (history unproven): %v", err)
		}
	})
	t.Run("an unreadable record", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads every file")
		}
		repo := scenario.Build(t, "accepted")
		p := filepath.Join(repo.Dir, ".verdi", "specs", "active", "successor", "spec.md")
		if err := os.Chmod(p, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
		if _, err := os.ReadFile(p); err == nil {
			t.Skip("this environment reads mode-0 files")
		}
		if _, err := BoardIndexes(ctx, repo.Dir); err == nil {
			t.Fatal("BoardIndexes succeeded over an unreadable record")
		}
		if _, err := WorkTreeViews(ctx, repo.Dir); err == nil {
			t.Fatal("WorkTreeViews succeeded over an unreadable record")
		}
	})
}
