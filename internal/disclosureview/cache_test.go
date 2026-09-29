package disclosureview

// Cache tests (SI-295). None runs in parallel: they swap the package's
// enumerateLint and now seams and set environment variables.

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

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// ciEnvVars are the process variables lint reads (lint/cienv.go,
// specstate/defaultbranch.go).
var ciEnvVars = []string{"CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF", "CI", "GITHUB_ACTIONS"}

// cacheFixture is a git-backed store: the bare-clone VL-017 disclosure
// store of buildFixtureStore, committed in two layers, with origin/main at
// the first layer and origin/HEAD naming it, so the default branch
// resolves and HEAD sits one commit ahead of it.
type cacheFixture struct {
	root  string
	heads []string
}

func newCacheFixture(t *testing.T) cacheFixture {
	t.Helper()
	for _, v := range ciEnvVars {
		t.Setenv(v, "")
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Message: "seed the store", Files: map[string]string{
			".verdi/verdi.yaml":                         manifestYAML,
			".verdi/.gitignore":                         "data/\nspecs/active/ignored-*/\n",
			".verdi/specs/active/panel-fixture/spec.md": storySpecMD,
			".gitattributes":                            ".verdi/specs/*/*/board.json          gitlab-generated\n.verdi/specs/*/*/rollup.json         gitlab-generated\n.verdi/specs/*/*/deviation-report.md gitlab-generated\n",
		}},
		{Message: "second layer", Files: map[string]string{"README.md": "store\n"}},
	})
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Heads[0])
	gitIn(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return cacheFixture{root: repo.Dir, heads: repo.Heads}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=Fixture", "-c", "user.email=fixture@verdi.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// countEnumerations wraps the enumeration seam and returns its call count.
func countEnumerations(t *testing.T) *atomic.Int64 {
	t.Helper()
	orig := enumerateLint
	var n atomic.Int64
	enumerateLint = func(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
		n.Add(1)
		return orig(ctx, root)
	}
	t.Cleanup(func() { enumerateLint = orig })
	return &n
}

// pastRacyWindow shifts the cache's clock an hour ahead, so inputs a test
// just wrote are older than racyWindow and results can be stored.
func pastRacyWindow(t *testing.T) {
	t.Helper()
	orig := now
	now = func() time.Time { return orig().Add(time.Hour) }
	t.Cleanup(func() { now = orig })
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// rewriteKeepingTimes rewrites path with content, then restores its
// access and modification times, so only the bytes (and, if content's
// length differs, the size) change.
func rewriteKeepingTimes(t *testing.T, path, content string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, content)
	if err := os.Chtimes(path, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(fi.ModTime()) {
		t.Fatalf("modification time not restored: %v, want %v", after.ModTime(), fi.ModTime())
	}
}

// fresh is the uncached enumeration the cache must always agree with.
func fresh(t *testing.T, root string, extras ...disclosure.Disclosure) ([]disclosure.Disclosure, error) {
	t.Helper()
	return Current(context.Background(), root, extras...)
}

func sameResult(t *testing.T, label string, got []disclosure.Disclosure, gotErr error, want []disclosure.Disclosure, wantErr error) {
	t.Helper()
	if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
		t.Fatalf("%s: error %v, want %v", label, gotErr, wantErr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s:\n got %+v\nwant %+v", label, got, want)
	}
}

func TestCache_HitWhenNothingChanged(t *testing.T) {
	fx := newCacheFixture(t)
	n := countEnumerations(t)
	pastRacyWindow(t)
	var c Cache
	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")

	want, wantErr := fresh(t, fx.root)
	if wantErr != nil || len(want) == 0 {
		t.Fatalf("fixture must enumerate at least one disclosure: %v %v", want, wantErr)
	}
	for i := range 3 {
		got, err := c.Current(context.Background(), fx.root)
		sameResult(t, fmt.Sprintf("call %d", i+1), got, err, want, wantErr)
	}
	wantExtra, _ := fresh(t, fx.root, extra)
	got, err := c.Current(context.Background(), fx.root, extra)
	sameResult(t, "call with an extra", got, err, wantExtra, nil)

	if got := n.Load(); got != 1 {
		t.Fatalf("enumerations = %d over four calls on an unchanged store, want 1", got)
	}
}

func TestCache_MissOnEachInputClass(t *testing.T) {
	spec := func(root string) string {
		return filepath.Join(root, ".verdi", "specs", "active", "panel-fixture", "spec.md")
	}
	sameSizeEdit := strings.Replace(storySpecMD, `title: "Panel Fixture"`, `title: "Panel Fixturf"`, 1)
	annotation := `{"id":"q-1","type":"question","status":"open","body":"why?","target":{"ref":"spec/panel-fixture"}}` + "\n"

	tests := []struct {
		name   string
		setup  func(t *testing.T, fx cacheFixture)
		change func(t *testing.T, fx cacheFixture)
	}{
		{name: "tracked file: same size, different content", change: func(t *testing.T, fx cacheFixture) {
			writeFile(t, spec(fx.root), sameSizeEdit)
		}},
		{name: "tracked file: same size and modification time, different content", change: func(t *testing.T, fx cacheFixture) {
			rewriteKeepingTimes(t, spec(fx.root), sameSizeEdit)
		}},
		{name: "tracked file: same modification time, different size", change: func(t *testing.T, fx cacheFixture) {
			rewriteKeepingTimes(t, spec(fx.root), storySpecMD+"\nMore prose.\n")
		}},
		{name: "untracked file added to the store", change: func(t *testing.T, fx cacheFixture) {
			writeFile(t, filepath.Join(fx.root, ".verdi", "specs", "active", "second", "spec.md"),
				strings.Replace(storySpecMD, "spec/panel-fixture", "spec/second", 1))
		}},
		{name: "ignored file the walk reads", change: func(t *testing.T, fx cacheFixture) {
			path := filepath.Join(fx.root, ".verdi", "specs", "active", "ignored-one", "spec.md")
			writeFile(t, path, strings.Replace(storySpecMD, "spec/panel-fixture", "spec/ignored-one", 1))
			gitIn(t, fx.root, "check-ignore", "-q", path)
		}},
		{name: "mutable zone appears", change: func(t *testing.T, fx cacheFixture) {
			if err := os.MkdirAll(filepath.Join(fx.root, ".verdi", "data", "mutable"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{
			name: "mutable-zone annotation file changed",
			setup: func(t *testing.T, fx cacheFixture) {
				writeFile(t, filepath.Join(fx.root, ".verdi", "data", "mutable", "annotations", "a.jsonl"), "")
			},
			change: func(t *testing.T, fx cacheFixture) {
				writeFile(t, filepath.Join(fx.root, ".verdi", "data", "mutable", "annotations", "a.jsonl"), annotation)
			},
		},
		{name: "a commit", change: func(t *testing.T, fx cacheFixture) {
			gitIn(t, fx.root, "commit", "-q", "--allow-empty", "-m", "another commit")
		}},
		{
			// "other" exists before the first call, so the switch moves
			// only HEAD's symbolic name: no ref, object or file changes.
			name:   "a branch switch to an existing branch at the same commit",
			setup:  func(t *testing.T, fx cacheFixture) { gitIn(t, fx.root, "branch", "other") },
			change: func(t *testing.T, fx cacheFixture) { gitIn(t, fx.root, "checkout", "-q", "other") },
		},
		{name: "HEAD detached at the same commit", change: func(t *testing.T, fx cacheFixture) {
			gitIn(t, fx.root, "checkout", "-q", "--detach")
		}},
		{name: "the default-branch ref moves", change: func(t *testing.T, fx cacheFixture) {
			gitIn(t, fx.root, "update-ref", "refs/remotes/origin/main", fx.heads[1])
		}},
		{name: "the index changes", change: func(t *testing.T, fx cacheFixture) {
			gitIn(t, fx.root, "rm", "-q", "--cached", "README.md")
		}},
		{name: "an object added that no ref reaches", change: func(t *testing.T, fx cacheFixture) {
			cmd := exec.Command("git", "hash-object", "-w", "--stdin")
			cmd.Dir = fx.root
			cmd.Stdin = strings.NewReader("a dangling blob\n")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("hash-object: %v\n%s", err, out)
			}
		}},
		{name: "the history becomes shallow", change: func(t *testing.T, fx cacheFixture) {
			writeFile(t, filepath.Join(fx.root, ".git", "shallow"), fx.heads[1]+"\n")
		}},
		{name: "git configuration changes", change: func(t *testing.T, fx cacheFixture) {
			gitIn(t, fx.root, "config", "diff.renameLimit", "7")
		}},
		{name: "a service appears", change: func(t *testing.T, fx cacheFixture) {
			writeFile(t, filepath.Join(fx.root, "svc", ".flowmap.yaml"), "service: svc\n")
		}},
		{
			name: "a service companion appears",
			setup: func(t *testing.T, fx cacheFixture) {
				writeFile(t, filepath.Join(fx.root, "svc", ".flowmap.yaml"), "service: svc\n")
			},
			change: func(t *testing.T, fx cacheFixture) {
				writeFile(t, filepath.Join(fx.root, "svc", "api", "openapi.yaml"), "openapi: 3.0.0\n")
			},
		},
		{name: "the root .gitattributes changes", change: func(t *testing.T, fx cacheFixture) {
			writeFile(t, filepath.Join(fx.root, ".gitattributes"), "*.md text\n")
		}},
		{name: "env CI_DEFAULT_BRANCH", change: func(t *testing.T, _ cacheFixture) { t.Setenv("CI_DEFAULT_BRANCH", "master") }},
		{name: "env CI_MERGE_REQUEST_TARGET_BRANCH_NAME", change: func(t *testing.T, _ cacheFixture) {
			t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")
		}},
		{name: "env GITHUB_BASE_REF", change: func(t *testing.T, _ cacheFixture) { t.Setenv("GITHUB_BASE_REF", "main") }},
		{name: "env CI", change: func(t *testing.T, _ cacheFixture) { t.Setenv("CI", "true") }},
		{name: "env GITHUB_ACTIONS", change: func(t *testing.T, _ cacheFixture) { t.Setenv("GITHUB_ACTIONS", "true") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newCacheFixture(t)
			if tt.setup != nil {
				tt.setup(t, fx)
			}
			n := countEnumerations(t)
			pastRacyWindow(t)
			var c Cache

			before, _ := readInputs(context.Background(), fx.root)
			if _, err := c.Current(context.Background(), fx.root); err != nil {
				t.Fatalf("first call: %v", err)
			}
			if _, err := c.Current(context.Background(), fx.root); err != nil || n.Load() != 1 {
				t.Fatalf("before the change: enumerations = %d (err %v), want 1 (a stored result)", n.Load(), err)
			}

			tt.change(t, fx)

			after, err := readInputs(context.Background(), fx.root)
			if err != nil {
				t.Fatalf("key after the change: %v", err)
			}
			if after.key == before.key {
				t.Fatal("the change left the key unchanged")
			}
			want, wantErr := fresh(t, fx.root)
			got, gotErr := c.Current(context.Background(), fx.root)
			sameResult(t, "after the change", got, gotErr, want, wantErr)
			if got := n.Load(); got != 2 {
				t.Fatalf("enumerations = %d, want 2: the change must be a miss", got)
			}
			if _, err := c.Current(context.Background(), fx.root); err != nil || n.Load() != 2 {
				t.Fatalf("after re-enumerating: enumerations = %d (err %v), want 2 (the new result stored)", n.Load(), err)
			}
		})
	}
}

func TestCache_UncomputableKeyEnumeratesFresh(t *testing.T) {
	tests := []struct {
		name  string
		store func(t *testing.T) string
	}{
		{"not a git repository", buildFixtureStore},
		{"no .verdi in a git repository", func(t *testing.T) string {
			fx := newCacheFixture(t)
			if err := os.RemoveAll(filepath.Join(fx.root, ".verdi")); err != nil {
				t.Fatal(err)
			}
			return fx.root
		}},
		{"a symbolic link to a directory under .verdi", func(t *testing.T) string {
			fx := newCacheFixture(t)
			outside := t.TempDir()
			writeFile(t, filepath.Join(outside, "spec.md"), strings.Replace(storySpecMD, "spec/panel-fixture", "spec/linked", 1))
			if err := os.Symlink(outside, filepath.Join(fx.root, ".verdi", "specs", "active", "linked")); err != nil {
				t.Fatal(err)
			}
			return fx.root
		}},
		{"a second object store in objects/info/alternates", func(t *testing.T) string {
			fx := newCacheFixture(t)
			other := fixturegit.Build(t, []fixturegit.Layer{{Message: "other", Files: map[string]string{"x": "x\n"}}})
			writeFile(t, filepath.Join(fx.root, ".git", "objects", "info", "alternates"), filepath.Join(other.Dir, ".git", "objects")+"\n")
			return fx.root
		}},
		{"a second object store from the environment", func(t *testing.T) string {
			fx := newCacheFixture(t)
			other := fixturegit.Build(t, []fixturegit.Layer{{Message: "other", Files: map[string]string{"x": "x\n"}}})
			t.Setenv("GIT_ALTERNATE_OBJECT_DIRECTORIES", filepath.Join(other.Dir, ".git", "objects"))
			return fx.root
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := tt.store(t)
			if _, err := readInputs(context.Background(), root); !errors.Is(err, errUncomputable) {
				t.Fatalf("readInputs error = %v, want errUncomputable", err)
			}
			n := countEnumerations(t)
			pastRacyWindow(t)
			var c Cache
			want, wantErr := fresh(t, root)
			for i := range 2 {
				got, err := c.Current(context.Background(), root)
				sameResult(t, fmt.Sprintf("call %d", i+1), got, err, want, wantErr)
			}
			if got := n.Load(); got != 2 {
				t.Fatalf("enumerations = %d over two calls, want 2: an uncomputable key never serves a cached value", got)
			}
		})
	}
}

func TestCache_WriteDuringEnumerationIsNotStored(t *testing.T) {
	tests := []struct {
		name    string
		restore bool
	}{
		{"changed and changed back (same bytes)", true},
		{"changed and left changed", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newCacheFixture(t)
			pastRacyWindow(t)
			path := filepath.Join(fx.root, ".verdi", "specs", "active", "panel-fixture", "spec.md")
			orig := enumerateLint
			var n atomic.Int64
			enumerateLint = func(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
				if n.Add(1) == 1 {
					writeFile(t, path, storySpecMD+"\nEdited.\n")
					if tt.restore {
						writeFile(t, path, storySpecMD)
					}
				}
				return orig(ctx, root)
			}
			t.Cleanup(func() { enumerateLint = orig })

			var c Cache
			if _, err := c.Current(context.Background(), fx.root); err != nil {
				t.Fatal(err)
			}
			want, wantErr := fresh(t, fx.root)
			got, err := c.Current(context.Background(), fx.root)
			sameResult(t, "second call", got, err, want, wantErr)
			if got := n.Load(); got != 2 {
				t.Fatalf("enumerations = %d, want 2: a result read while an input was written is never stored", got)
			}
		})
	}
}

func TestCache_RecentWritesAreNotStored(t *testing.T) {
	fx := newCacheFixture(t)
	n := countEnumerations(t)
	var c Cache

	// The fixture was written moments ago: inside the racy window.
	for i := range 2 {
		if _, err := c.Current(context.Background(), fx.root); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("enumerations = %d, want 2: inputs written within racyWindow are not stored", got)
	}

	pastRacyWindow(t)
	for i := range 2 {
		if _, err := c.Current(context.Background(), fx.root); err != nil {
			t.Fatalf("call %d: %v", i+3, err)
		}
	}
	if got := n.Load(); got != 3 {
		t.Fatalf("enumerations = %d, want 3: once past the window the result is stored and reused", got)
	}
}

func TestCache_EnumerationErrorIsNotStored(t *testing.T) {
	fx := newCacheFixture(t)
	writeFile(t, filepath.Join(fx.root, "svc", ".flowmap.yaml"), "- not a mapping\n")
	n := countEnumerations(t)
	pastRacyWindow(t)
	var c Cache

	_, wantErr := fresh(t, fx.root)
	if wantErr == nil {
		t.Fatal("fixture must fail to enumerate")
	}
	for i := range 2 {
		got, err := c.Current(context.Background(), fx.root)
		sameResult(t, fmt.Sprintf("call %d", i+1), got, err, nil, wantErr)
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("enumerations = %d, want 2: an error is never stored", got)
	}
}

func TestCache_ConcurrentCalls(t *testing.T) {
	a, b := newCacheFixture(t), newCacheFixture(t)
	n := countEnumerations(t)
	pastRacyWindow(t)
	var c Cache

	type call struct {
		root  string
		extra []disclosure.Disclosure
	}
	var calls []call
	for i := range 16 {
		root := a.root
		if i%2 == 1 {
			root = b.root
		}
		var extra []disclosure.Disclosure
		if i%3 == 0 {
			extra = append(extra, disclosure.New("mcp:review-feed", "", fmt.Sprintf("extra %d", i)))
		}
		calls = append(calls, call{root, extra})
	}
	got := make([][]disclosure.Disclosure, len(calls))
	errs := make([]error, len(calls))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, cl := range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got[i], errs[i] = c.Current(context.Background(), cl.root, cl.extra...)
		}()
	}
	close(start)
	wg.Wait()

	for i, cl := range calls {
		want, wantErr := fresh(t, cl.root, cl.extra...)
		sameResult(t, fmt.Sprintf("call %d", i), got[i], errs[i], want, wantErr)
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("enumerations = %d for 16 concurrent calls over two unchanged roots, want 2 (one per root)", got)
	}
}

// TestCache_PanickingEnumerationReleasesItsEntry: a leader whose
// enumeration panics still finishes its entry, so a later call with the
// same key enumerates instead of waiting on an entry nobody will finish.
func TestCache_PanickingEnumerationReleasesItsEntry(t *testing.T) {
	fx := newCacheFixture(t)
	pastRacyWindow(t)
	orig := enumerateLint
	var n atomic.Int64
	enumerateLint = func(ctx context.Context, root string) ([]disclosure.Disclosure, error) {
		if n.Add(1) == 1 {
			panic("enumeration failed")
		}
		return orig(ctx, root)
	}
	t.Cleanup(func() { enumerateLint = orig })
	var c Cache

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the first call must panic")
			}
		}()
		_, _ = c.Current(context.Background(), fx.root)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	want, wantErr := fresh(t, fx.root)
	got, err := c.Current(ctx, fx.root)
	sameResult(t, "the call after the panic", got, err, want, wantErr)
	if got := n.Load(); got != 2 {
		t.Fatalf("enumerations = %d, want 2", got)
	}
}

func TestCacheEntry_Await(t *testing.T) {
	stored := []disclosure.Disclosure{disclosure.New("lint:VL-017", "x", "y")}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name      string
		entry     func() *cacheEntry
		ctx       context.Context
		wantItems []disclosure.Disclosure
		wantOK    bool
		wantErr   error
	}{
		{"a stored result", func() *cacheEntry {
			e := &cacheEntry{done: make(chan struct{}), items: stored, ok: true}
			close(e.done)
			return e
		}, context.Background(), stored, true, nil},
		{"a leader that stored nothing", func() *cacheEntry {
			e := &cacheEntry{done: make(chan struct{})}
			close(e.done)
			return e
		}, context.Background(), nil, false, nil},
		{"a leader still running when the waiter's context ends", func() *cacheEntry {
			return &cacheEntry{done: make(chan struct{})}
		}, canceled, nil, false, context.Canceled},
		{"a leader that finishes while the waiter waits", func() *cacheEntry {
			e := &cacheEntry{done: make(chan struct{})}
			go func() {
				e.items, e.ok = stored, true
				close(e.done)
			}()
			return e
		}, context.Background(), stored, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, ok, err := tt.entry().await(tt.ctx)
			if !errors.Is(err, tt.wantErr) || ok != tt.wantOK || !reflect.DeepEqual(items, tt.wantItems) {
				t.Fatalf("await = (%v, %v, %v), want (%v, %v, %v)", items, ok, err, tt.wantItems, tt.wantOK, tt.wantErr)
			}
		})
	}
}

func TestCache_Bounded(t *testing.T) {
	tests := []struct {
		name  string
		roots int
	}{
		{"below the bound", maxCachedRoots - 1},
		{"at the bound", maxCachedRoots},
		{"past the bound", maxCachedRoots + 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Cache
			for i := range tt.roots {
				root := fmt.Sprintf("/root-%d", i)
				e, leader := c.claim(root, "k")
				if !leader {
					t.Fatalf("claim %s: want a new entry", root)
				}
				close(e.done)
				if len(c.entries) > maxCachedRoots {
					t.Fatalf("entries = %d after %d roots, want at most %d", len(c.entries), i+1, maxCachedRoots)
				}
				if c.entries[root] != e {
					t.Fatalf("the newest root %s was evicted", root)
				}
			}
			if _, leader := c.claim(fmt.Sprintf("/root-%d", tt.roots-1), "k2"); !leader {
				t.Fatal("a new key for a root must replace its entry")
			}
		})
	}
}

// injectStale stores a result the checkout does not have under root's
// current key, standing in for a value cached before a transient failure.
func injectStale(t *testing.T, c *Cache, root string) []disclosure.Disclosure {
	t.Helper()
	in, err := readInputs(context.Background(), root)
	if err != nil {
		t.Fatalf("readInputs: %v", err)
	}
	stale := []disclosure.Disclosure{disclosure.New("lint:VL-999", "stale", "a value cached before a transient failure")}
	e := &cacheEntry{key: in.key, done: make(chan struct{}), items: stale, ok: true}
	close(e.done)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[string]*cacheEntry)
	}
	c.entries[root] = e
	return stale
}

// sharedEntry returns the process-wide cache's entry for root, if any.
func sharedEntry(root string) (*cacheEntry, bool) {
	shared.mu.Lock()
	defer shared.mu.Unlock()
	e, ok := shared.entries[root]
	return e, ok
}

// TestCurrent_LeavesTheCacheAsItFoundIt (SI-295; closed
// spec/disclosures-panel: "no file, no cache, no log is written by
// rendering the view"): the page's enumeration, Current, returns a fresh
// result and leaves the process-wide cache exactly as it found it — no
// entry stays no entry, and an entry, even a stale one, keeps its identity
// and value, so the index still reads it.
func TestCurrent_LeavesTheCacheAsItFoundIt(t *testing.T) {
	tests := []struct {
		name  string
		prime func(t *testing.T, root string)
	}{
		{"no entry", func(*testing.T, string) {}},
		{"an entry the index stored", func(t *testing.T, root string) {
			if _, err := Cached(context.Background(), root); err != nil {
				t.Fatal(err)
			}
			if _, ok := sharedEntry(root); !ok {
				t.Fatal("the index must have stored an entry")
			}
		}},
		{"a stale entry", func(t *testing.T, root string) { injectStale(t, &shared, root) }},
	}
	extra := disclosure.New("mcp:review-feed", "", "forge configured but unreachable")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fx := newCacheFixture(t)
			pastRacyWindow(t)
			tt.prime(t, fx.root)
			before, had := sharedEntry(fx.root)
			var items []disclosure.Disclosure
			if had {
				items = append(items, before.items...)
			}

			want, wantErr := fresh(t, fx.root, extra)
			n := countEnumerations(t)
			got, err := Current(context.Background(), fx.root, extra)
			sameResult(t, "the page's enumeration", got, err, want, wantErr)

			after, has := sharedEntry(fx.root)
			if has != had || after != before {
				t.Fatalf("the page changed the cache: entry present %v -> %v, same entry %v", had, has, after == before)
			}
			if had && !reflect.DeepEqual(after.items, items) {
				t.Fatalf("the page changed the cached value: %v -> %v", items, after.items)
			}
			if had {
				index, err := Cached(context.Background(), fx.root)
				if err != nil || !reflect.DeepEqual(index, items) || n.Load() != 0 {
					t.Fatalf("the index must still read the entry it found: got %v (err %v, enumerations %d), want %v", index, err, n.Load(), items)
				}
			}
		})
	}
}
