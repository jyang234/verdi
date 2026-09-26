package workbench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// The board's closed-spec object supersession view cache (SI-278; the
// L3c cost note, controller ruling 3): package-level, keyed by root; the
// default-branch index is invalidated when the default-branch head moves,
// the working-tree index when its records digest changes; an unchanged
// tree never recomputes; bounded; single-flight under concurrent
// requests.

// cacheGet drives the cache's core with a counting build so a test can
// see exactly how many builds a sequence of gets caused.
func cacheGet(t *testing.T, c *supersessionCache, key string, builds *atomic.Int32, delay time.Duration) *supersessionEntry {
	t.Helper()
	e, err := c.get(context.Background(), key, func(context.Context) (*supersessionEntry, error) {
		builds.Add(1)
		time.Sleep(delay)
		return &supersessionEntry{digest: key}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestSupersessionCache_HitsAndBound(t *testing.T) {
	c := newSupersessionCache(3)
	var builds atomic.Int32
	// A miss builds once; a hit builds nothing and returns the same entry.
	a := cacheGet(t, c, "a", &builds, 0)
	if again := cacheGet(t, c, "a", &builds, 0); again != a || builds.Load() != 1 {
		t.Fatalf("a second get of the same key built again: builds=%d same=%v", builds.Load(), again == a)
	}
	// The bound: with a limit of 3, a fourth distinct key evicts the least
	// recently used one — "a" was touched most recently below, so "b" goes.
	cacheGet(t, c, "b", &builds, 0)
	cacheGet(t, c, "c", &builds, 0)
	cacheGet(t, c, "a", &builds, 0) // touch
	cacheGet(t, c, "d", &builds, 0) // evicts b
	if builds.Load() != 4 {
		t.Fatalf("builds = %d, want 4 (a, b, c, d)", builds.Load())
	}
	if n := len(c.entries); n != 3 {
		t.Fatalf("entries = %d, want the bound 3", n)
	}
	cacheGet(t, c, "a", &builds, 0) // still cached
	cacheGet(t, c, "b", &builds, 0) // evicted: builds again
	if builds.Load() != 5 {
		t.Fatalf("builds = %d, want 5 (b rebuilt after eviction, a still cached)", builds.Load())
	}
	// A failed build caches nothing.
	if _, err := c.get(context.Background(), "x", func(context.Context) (*supersessionEntry, error) { return nil, errors.New("boom") }); err == nil {
		t.Fatal("get returned no error for a failing build")
	}
	if _, ok := c.entries["x"]; ok {
		t.Fatal("a failed build was cached")
	}
	if len(c.inflight) != 0 {
		t.Fatalf("inflight = %d after every get returned, want 0", len(c.inflight))
	}
}

func TestSupersessionCache_SingleFlight(t *testing.T) {
	c := newSupersessionCache(4)
	var builds atomic.Int32
	var wg sync.WaitGroup
	results := make([]*supersessionEntry, 16)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = cacheGet(t, c, "shared", &builds, 50*time.Millisecond)
		}(i)
	}
	wg.Wait()
	if builds.Load() != 1 {
		t.Fatalf("builds = %d, want 1: concurrent gets of one key must share one build", builds.Load())
	}
	for i, r := range results {
		if r != results[0] {
			t.Fatalf("result %d is a different entry than result 0", i)
		}
	}
	// A failing build under single-flight fails every waiter and caches
	// nothing, so the next get builds again.
	var attempts atomic.Int32
	var errs atomic.Int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.get(context.Background(), "failing", func(context.Context) (*supersessionEntry, error) {
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

// TestSupersessionCache_Boards drives the real thing over a scenario
// store: one index when the working tree is the default branch's tree,
// no rebuild on an unchanged tree, a rebuild when the default-branch head
// moves, a separate working-tree index when the records diverge, and the
// alias back when they agree again.
func TestSupersessionCache_Boards(t *testing.T) {
	neutralizeCIEnv(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	c := newSupersessionCache(8)

	b, err := c.boards(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.def == nil || b.tree != b.def {
		t.Fatalf("on the default branch's clean checkout the tree index must be the default-branch index: def=%p tree=%p", b.def, b.tree)
	}
	if v := b.def.index.Object("closed-feature", "dc-1"); v.State != objsupersede.ObjectSuperseded {
		t.Fatalf("closed-feature#dc-1 = %+v, want superseded", v)
	}
	if b.def.recs.Specs["closed-feature"] == nil {
		t.Fatal("the entry carries no records (the ref card needs the object's text)")
	}
	if builds := c.builds.Load(); builds != 1 {
		t.Fatalf("builds = %d after the first boards, want 1", builds)
	}
	if _, err := c.boards(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	}
	if builds := c.builds.Load(); builds != 1 {
		t.Fatalf("builds = %d after an unchanged tree, want still 1 (never recompute per request)", builds)
	}

	// A record edited in the working tree (not committed): the records
	// digest changes, so the tree gets its own index while the
	// default-branch index stays.
	other := filepath.Join(repo.Dir, ".verdi", "specs", "active", "other-feature", "spec.md")
	raw, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other, append(raw, []byte("\nEdited in the working tree.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	b2, err := c.boards(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b2.def != b.def || b2.tree == b2.def {
		t.Fatalf("an edited tree must keep the default index and build its own: def same=%v tree aliased=%v", b2.def == b.def, b2.tree == b2.def)
	}
	if builds := c.builds.Load(); builds != 2 {
		t.Fatalf("builds = %d after the edit, want 2", builds)
	}
	if _, err := c.boards(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	}
	if builds := c.builds.Load(); builds != 2 {
		t.Fatalf("builds = %d after the same edited tree, want still 2", builds)
	}
	// Reverted: the digests agree again and the default index serves both.
	if err := os.WriteFile(other, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	b3, err := c.boards(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b3.tree != b.def || c.builds.Load() != 2 {
		t.Fatalf("a reverted tree must alias the default index without a build: aliased=%v builds=%d", b3.tree == b.def, c.builds.Load())
	}

	// The default-branch head moves (a commit on main, fetched: the scenario
	// store resolves its default branch through refs/remotes/origin/main,
	// so the remote-tracking ref is what a head move moves): the default
	// index is rebuilt, once.
	gitIn(t, repo.Dir, "commit", "--allow-empty", "-q", "-m", "move main")
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	b4, err := c.boards(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b4.def == b.def || c.builds.Load() != 3 {
		t.Fatalf("a moved head must rebuild the default index once: rebuilt=%v builds=%d", b4.def != b.def, c.builds.Load())
	}
	if b4.tree != b4.def {
		t.Fatal("after the head move the clean tree must alias the new default index")
	}
}

// TestSupersessionCache_NoDefaultBranch: with no provable default branch
// the tree's own index serves both cards and every history fact reads
// unproven — the honest disclosure, never silence and never a guess.
func TestSupersessionCache_NoDefaultBranch(t *testing.T) {
	neutralizeCIEnv(t)
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
	c := newSupersessionCache(8)
	b, err := c.boards(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if b.def != nil || b.tree == nil {
		t.Fatalf("without a default branch: def=%v tree=%v; want no default index and a tree index", b.def != nil, b.tree != nil)
	}
	v := b.tree.index.Object("closed-feature", "dc-1")
	if v.State != objsupersede.ObjectUnproven {
		t.Fatalf("closed-feature#dc-1 = %+v, want unproven (no default branch to prove acceptance)", v)
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
	write(".verdi/specs/active/a/spec.md", "a")
	write(".verdi/conflicts/c.md", "c")
	write(".verdi/data/mutable/x.json", "ignored")
	d1, err := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	// Deterministic, and blind to files outside the record directories.
	d1b, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	write(".verdi/data/mutable/x.json", "changed")
	d1c, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	if d1 != d1b || d1 != d1c || d1 == "" {
		t.Fatalf("digest not deterministic or not blind to the data zone: %q %q %q", d1, d1b, d1c)
	}
	// Any record byte, a new record, an archived record, and a symlink on
	// a record path each change it.
	write(".verdi/specs/active/a/spec.md", "a2")
	d2, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	write(".verdi/specs/archive/b/spec.md", "b")
	d3, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	write(".verdi/conflicts/d.md", "d")
	d4, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	if err := os.Symlink("c.md", filepath.Join(root, ".verdi", "conflicts", "e.md")); err != nil {
		t.Fatal(err)
	}
	d5, _ := recordsDigest(ctx, objsupersede.WorkTree{Root: root})
	seen := map[string]bool{}
	for i, d := range []string{d1, d2, d3, d4, d5} {
		if seen[d] {
			t.Fatalf("digest %d repeats an earlier one: %q", i, d)
		}
		seen[d] = true
	}
	// The same records at a commit digest the same as in the working tree.
	neutralizeCIEnv(t)
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

// gitIn runs git in dir for a test fixture with a fixed identity and fails
// t on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
		"GIT_AUTHOR_DATE=2024-03-01T00:00:00Z", "GIT_COMMITTER_DATE=2024-03-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

var _ = fmt.Sprintf
