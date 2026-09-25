package specstate

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memoGit is a content-addressed fake gitReader for the corpus-memo tests.
// commits maps a commit id to its tree (path to bytes) and heads maps a
// ref to the commit it points at, so a read at a ref and a read at the
// commit that ref resolves to see the same tree, as with real git. It
// counts every LsTree call by the revision it was asked for (one LsTree
// call is one successor-corpus scan) and is safe for concurrent use.
type memoGit struct {
	commits map[string]map[string][]byte

	mu        sync.Mutex
	heads     map[string]string
	revErr    error // when set, RevParse fails with it
	lsTreeErr error // when set, LsTree fails with it, naming the revision it was asked for
	lsTrees   map[string]int
	revParses []string
}

func newMemoGit(commits map[string]map[string][]byte) *memoGit {
	return &memoGit{commits: commits, heads: map[string]string{}, lsTrees: map[string]int{}}
}

// set points ref "main" at head and sets the injected failures.
func (g *memoGit) set(head string, revErr, lsTreeErr error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.heads["main"] = head
	g.revErr = revErr
	g.lsTreeErr = lsTreeErr
}

// scans returns how many corpus scans (LsTree calls) g has served.
func (g *memoGit) scans() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, c := range g.lsTrees {
		n += c
	}
	return n
}

// scansByRev returns a copy of g's LsTree count per revision.
func (g *memoGit) scansByRev() map[string]int {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]int, len(g.lsTrees))
	for rev, n := range g.lsTrees {
		out[rev] = n
	}
	return out
}

// treeLocked resolves rev (a commit id or a ref in heads) to its tree.
func (g *memoGit) treeLocked(rev string) (map[string][]byte, error) {
	if tree, ok := g.commits[rev]; ok {
		return tree, nil
	}
	if commit, ok := g.heads[rev]; ok {
		return g.commits[commit], nil
	}
	return nil, fmt.Errorf("memoGit: unknown revision %q", rev)
}

func (g *memoGit) Show(ctx context.Context, dir, rev, path string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	tree, err := g.treeLocked(rev)
	if err != nil {
		return nil, err
	}
	content, ok := tree[path]
	if !ok {
		return nil, fmt.Errorf("memoGit: %s not in %s", path, rev)
	}
	return content, nil
}

func (g *memoGit) BlobAt(ctx context.Context, dir, rev, path string) (string, bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	tree, err := g.treeLocked(rev)
	if err != nil {
		return "", false, err
	}
	content, ok := tree[path]
	if !ok {
		return "", false, nil
	}
	return fmt.Sprintf("%x", sha1.Sum(content)), true, nil
}

func (g *memoGit) FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (string, bool, error) {
	return "landing-" + oid[:8], true, nil
}

func (g *memoGit) LsTree(ctx context.Context, dir, rev, prefix string) ([]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lsTrees[rev]++
	if g.lsTreeErr != nil {
		return nil, fmt.Errorf("memoGit: ls-tree %s: %w", rev, g.lsTreeErr)
	}
	tree, err := g.treeLocked(rev)
	if err != nil {
		return nil, err
	}
	var out []string
	for path := range tree {
		if strings.HasPrefix(path, prefix+"/") {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (g *memoGit) RevParse(ctx context.Context, dir, rev string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.revParses = append(g.revParses, rev)
	if g.revErr != nil {
		return "", g.revErr
	}
	commit, ok := g.heads[strings.TrimSuffix(rev, "^{commit}")]
	if !ok {
		return "", fmt.Errorf("memoGit: unknown revision %q", rev)
	}
	return commit, nil
}

const (
	memoPredPath = ".verdi/specs/active/old-feature/spec.md"
	memoSuccPath = ".verdi/specs/active/new-feature/spec.md"
	memoPred     = "---\nid: spec/old-feature\nkind: spec\nclass: feature\ntitle: Old Feature\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n---\nbody\n"
)

// memoCommits is the fixture history every memo test reads: c1 holds only
// the predecessor; c2 adds a valid successor of it. b1..bN hold the same
// tree as c1 under distinct commit ids, for the eviction row.
func memoCommits() map[string]map[string][]byte {
	commits := map[string]map[string][]byte{
		"c1": {memoPredPath: []byte(memoPred)},
		"c2": {memoPredPath: []byte(memoPred), memoSuccPath: validSuccessorSpec("new-feature", "old-feature")},
	}
	for i := 1; i <= corpusCacheLimit+1; i++ {
		commits[fmt.Sprintf("b%d", i)] = map[string][]byte{memoPredPath: []byte(memoPred)}
	}
	return commits
}

// memoStep is one Resolve of the predecessor with the default branch at
// head and the given failures injected.
type memoStep struct {
	head      string
	revErr    error
	lsTreeErr error
	want      State // ignored when wantErr
	wantErr   bool
	wantScans int // corpus scans the memoizing projector has made after this step
}

// boundSteps walks corpusCacheLimit+1 distinct commits, so the first is
// evicted, then shows the newest stay cached and the evicted one is read
// again.
func boundSteps() []memoStep {
	n := corpusCacheLimit + 1
	var steps []memoStep
	for i := 1; i <= n; i++ {
		steps = append(steps, memoStep{head: fmt.Sprintf("b%d", i), want: AcceptedPendingBuild, wantScans: i})
	}
	steps = append(steps,
		memoStep{head: fmt.Sprintf("b%d", n), want: AcceptedPendingBuild, wantScans: n},
		memoStep{head: fmt.Sprintf("b%d", n-1), want: AcceptedPendingBuild, wantScans: n},
		memoStep{head: "b1", want: AcceptedPendingBuild, wantScans: n + 1},
	)
	return steps
}

// TestProjector_CorpusMemo drives one memoizing Projector through a
// sequence of Resolves per row and checks two things at every step: how
// many times the successor corpus has been read, and that the Result or
// error is exactly what a Projector without the memo (today's scan at the
// ref, every call) returns over an identical fake.
func TestProjector_CorpusMemo(t *testing.T) {
	ctx := context.Background()
	errBoom := errors.New("boom")

	tests := []struct {
		name  string
		steps []memoStep
	}{
		{
			name: "the same commit is read once across many resolves",
			steps: []memoStep{
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
			},
		},
		{
			name: "a moved default branch is read again; entries are keyed by commit, not by ref",
			steps: []memoStep{
				{head: "c1", want: AcceptedPendingBuild, wantScans: 1},
				{head: "c2", want: Superseded, wantScans: 2},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 2},
				{head: "c2", want: Superseded, wantScans: 2},
			},
		},
		{
			name: "a failed scan is not cached and the next resolve retries",
			steps: []memoStep{
				// The pinned scan fails, then the unpinned scan at the
				// ref runs so the error is the one it has always been.
				{head: "c1", lsTreeErr: errBoom, wantErr: true, wantScans: 2},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 3},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 3},
			},
		},
		{
			name: "an unresolvable commit is scanned at the ref every time and never cached",
			steps: []memoStep{
				{head: "c1", revErr: errBoom, want: AcceptedPendingBuild, wantScans: 1},
				{head: "c1", revErr: errBoom, want: AcceptedPendingBuild, wantScans: 2},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 3},
				{head: "c1", want: AcceptedPendingBuild, wantScans: 3},
			},
		},
		{
			name:  "at most corpusCacheLimit commits stay cached and the least recently used is read again",
			steps: boundSteps(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := buildResolvableRepo(t)
			memoFake := newMemoGit(memoCommits())
			plainFake := newMemoGit(memoCommits())
			memo := newProjector(memoFake)
			plain := Projector{git: plainFake} // no corpus cache: the pre-memo scan on every call
			candidate := Candidate{Path: memoPredPath, Content: []byte(memoPred)}

			for i, step := range tt.steps {
				memoFake.set(step.head, step.revErr, step.lsTreeErr)
				plainFake.set(step.head, step.revErr, step.lsTreeErr)

				got, gotErr := memo.Resolve(ctx, repo.Dir, candidate)
				want, wantErr := plain.Resolve(ctx, repo.Dir, candidate)

				if step.wantErr {
					if gotErr == nil || wantErr == nil {
						t.Fatalf("step %d: errors = (%v, %v), want both non-nil", i, gotErr, wantErr)
					}
					if gotErr.Error() != wantErr.Error() {
						t.Fatalf("step %d: memo error %q differs from the unmemoized error %q", i, gotErr, wantErr)
					}
				} else {
					if gotErr != nil || wantErr != nil {
						t.Fatalf("step %d: errors = (%v, %v), want none", i, gotErr, wantErr)
					}
					if got.State != step.want {
						t.Fatalf("step %d (head %s): state = %s, want %s", i, step.head, got.State, step.want)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("step %d: memo result %+v differs from the unmemoized result %+v", i, got, want)
					}
				}
				if n := memoFake.scans(); n != step.wantScans {
					t.Fatalf("step %d (head %s): %d corpus scans so far, want %d", i, step.head, n, step.wantScans)
				}
				if n := len(memo.corpora.entries); n > corpusCacheLimit {
					t.Fatalf("step %d: %d cached corpora, want at most %d", i, n, corpusCacheLimit)
				}
			}
		})
	}
}

// TestProjector_CorpusMemo_ReadsTheResolvedCommit proves the cached scan
// reads the commit the default branch resolved to, never the ref name, and
// that a resolve that needs no supersession answer never resolves it.
func TestProjector_CorpusMemo_ReadsTheResolvedCommit(t *testing.T) {
	ctx := context.Background()

	t.Run("a scan is pinned to the resolved commit", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		g := newMemoGit(memoCommits())
		g.set("c2", nil, nil)
		if _, err := newProjector(g).Resolve(ctx, repo.Dir, Candidate{Path: memoPredPath, Content: []byte(memoPred)}); err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if got, want := g.scansByRev(), map[string]int{"c2": 1}; !reflect.DeepEqual(got, want) {
			t.Fatalf("scans by revision = %v, want %v", got, want)
		}
		if want := []string{"main^{commit}"}; !reflect.DeepEqual(g.revParses, want) {
			t.Fatalf("RevParse calls = %q, want %q", g.revParses, want)
		}
	})

	t.Run("an empty commit id is not a key: the ref is scanned and nothing is cached", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		g := newMemoGit(memoCommits())
		g.set("", nil, nil) // RevParse answers "" with no error
		g.commits[""] = g.commits["c1"]
		p := newProjector(g)
		for i := 0; i < 2; i++ {
			if _, err := p.Resolve(ctx, repo.Dir, Candidate{Path: memoPredPath, Content: []byte(memoPred)}); err != nil {
				t.Fatalf("Resolve %d: %v", i, err)
			}
		}
		if got, want := g.scansByRev(), map[string]int{"main": 2}; !reflect.DeepEqual(got, want) {
			t.Fatalf("scans by revision = %v, want %v", got, want)
		}
		if n := len(p.corpora.entries); n != 0 {
			t.Fatalf("%d cached corpora, want none", n)
		}
	})

	t.Run("a candidate that needs no supersession answer never resolves the commit", func(t *testing.T) {
		repo := buildResolvableRepo(t)
		g := newMemoGit(memoCommits())
		g.set("c1", nil, nil)
		// A path the default branch does not hold: Proposed/new, no corpus.
		result, err := newProjector(g).Resolve(ctx, repo.Dir, Candidate{Path: memoSuccPath, Content: []byte("x")})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if result.State != Proposed {
			t.Fatalf("state = %s, want %s", result.State, Proposed)
		}
		if len(g.revParses) != 0 || g.scans() != 0 {
			t.Fatalf("RevParse calls %q and %d scans, want none", g.revParses, g.scans())
		}
	})
}

// TestProjector_CorpusMemo_ConcurrentResolves runs many Resolves of one
// Projector at once (run it under -race): every one gets the same result
// and the corpus is read once.
func TestProjector_CorpusMemo_ConcurrentResolves(t *testing.T) {
	ctx := context.Background()
	repo := buildResolvableRepo(t)
	g := newMemoGit(memoCommits())
	g.set("c2", nil, nil)
	p := newProjector(g)

	const n = 32
	results := make([]Result, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = p.Resolve(ctx, repo.Dir, Candidate{Path: memoPredPath, Content: []byte(memoPred)})
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("Resolve %d: %v", i, errs[i])
		}
		if results[i].State != Superseded || !reflect.DeepEqual(results[i], results[0]) {
			t.Fatalf("Resolve %d = %+v, want Superseded and equal to %+v", i, results[i], results[0])
		}
	}
	if got := g.scans(); got != 1 {
		t.Fatalf("%d corpus scans across %d concurrent resolves, want 1", got, n)
	}
}

// TestNewProjector_SharesOneProcessCorpusCache proves the production
// constructor hands every Projector the same process-wide cache (callers
// build a fresh NewProjector per spec), while the test constructor gives
// each Projector its own.
func TestNewProjector_SharesOneProcessCorpusCache(t *testing.T) {
	a, b := NewProjector(), NewProjector()
	if a.corpora == nil || a.corpora != b.corpora || a.corpora != processCorpora {
		t.Fatalf("NewProjector caches = %p, %p, want both the process cache %p", a.corpora, b.corpora, processCorpora)
	}
	if processCorpora.limit != corpusCacheLimit {
		t.Fatalf("process cache limit = %d, want %d", processCorpora.limit, corpusCacheLimit)
	}
	c, d := newProjector(stubGit{}), newProjector(stubGit{})
	if c.corpora == nil || c.corpora == d.corpora || c.corpora == processCorpora {
		t.Fatalf("newProjector caches = %p, %p, want two distinct caches, neither the process cache", c.corpora, d.corpora)
	}
}

// TestCorpusCache_Get walks one cache through a sequence of gets per row,
// checking at every step whether scan ran, which corpus came back, and
// that the cache stays within its limit with no scan left in flight.
func TestCorpusCache_Get(t *testing.T) {
	ctx := context.Background()
	errBoom := errors.New("boom")
	k := func(root, commit string) corpusKey { return corpusKey{root: root, commit: commit} }

	type getStep struct {
		key      corpusKey
		scanErr  error
		wantScan bool
	}
	tests := []struct {
		name  string
		limit int
		steps []getStep
	}{
		{
			name:  "a stored corpus answers every later get",
			limit: 4,
			steps: []getStep{{k("/r", "c1"), nil, true}, {k("/r", "c1"), nil, false}, {k("/r", "c1"), nil, false}},
		},
		{
			name:  "the same commit under another root is its own entry",
			limit: 4,
			steps: []getStep{{k("/r1", "c1"), nil, true}, {k("/r2", "c1"), nil, true}, {k("/r1", "c1"), nil, false}, {k("/r2", "c1"), nil, false}},
		},
		{
			name:  "another commit under the same root is its own entry",
			limit: 4,
			steps: []getStep{{k("/r", "c1"), nil, true}, {k("/r", "c2"), nil, true}, {k("/r", "c1"), nil, false}},
		},
		{
			name:  "a failed scan stores nothing and the next get scans again",
			limit: 4,
			steps: []getStep{{k("/r", "c1"), errBoom, true}, {k("/r", "c1"), errBoom, true}, {k("/r", "c1"), nil, true}, {k("/r", "c1"), nil, false}},
		},
		{
			name:  "past the limit the least recently used entry is dropped",
			limit: 2,
			steps: []getStep{
				{k("/r", "c1"), nil, true},
				{k("/r", "c2"), nil, true},
				{k("/r", "c1"), nil, false}, // c1 is now the most recently used
				{k("/r", "c3"), nil, true},  // drops c2
				{k("/r", "c1"), nil, false},
				{k("/r", "c3"), nil, false},
				{k("/r", "c2"), nil, true}, // drops c1
				{k("/r", "c1"), nil, true},
			},
		},
		{
			name:  "the limit bounds entries across roots, not per root",
			limit: 2,
			steps: []getStep{{k("/r1", "c1"), nil, true}, {k("/r2", "c1"), nil, true}, {k("/r3", "c1"), nil, true}, {k("/r1", "c1"), nil, true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCorpusCache(tt.limit)
			stored := map[corpusKey]*successorCorpus{}
			for i, step := range tt.steps {
				fresh := &successorCorpus{failures: map[string]string{"step": fmt.Sprint(i)}}
				scanned := false
				got, err := c.get(ctx, step.key, func() (*successorCorpus, error) {
					scanned = true
					if step.scanErr != nil {
						return nil, step.scanErr
					}
					return fresh, nil
				})
				if scanned != step.wantScan {
					t.Fatalf("step %d (%v): scanned = %v, want %v", i, step.key, scanned, step.wantScan)
				}
				switch {
				case step.scanErr != nil:
					if !errors.Is(err, step.scanErr) || got != nil {
						t.Fatalf("step %d: get = (%p, %v), want (nil, %v)", i, got, err, step.scanErr)
					}
				case scanned:
					if err != nil || got != fresh {
						t.Fatalf("step %d: get = (%p, %v), want the fresh scan %p", i, got, err, fresh)
					}
					stored[step.key] = fresh
				default:
					if err != nil || got != stored[step.key] {
						t.Fatalf("step %d: get = (%p, %v), want the stored corpus %p", i, got, err, stored[step.key])
					}
				}
				if n := len(c.entries); n > tt.limit {
					t.Fatalf("step %d: %d entries, want at most %d", i, n, tt.limit)
				}
				if n := len(c.flights); n != 0 {
					t.Fatalf("step %d: %d scans left in flight, want 0", i, n)
				}
			}
		})
	}
}

// TestCorpusCache_ConcurrentGetsShareOneScan starts many gets for one key
// and holds the first scan until every other caller is waiting on it:
// scan runs once and every caller gets its corpus.
func TestCorpusCache_ConcurrentGetsShareOneScan(t *testing.T) {
	c := newCorpusCache(corpusCacheLimit)
	key := corpusKey{root: "/r", commit: "c1"}
	want := &successorCorpus{}
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	var scans atomic.Int32

	const n = 32
	got := make([]*successorCorpus, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i], errs[i] = c.get(context.Background(), key, func() (*successorCorpus, error) {
				scans.Add(1)
				once.Do(func() { close(started) })
				<-release
				return want, nil
			})
		}(i)
	}
	<-started
	waitForWaiters(t, c, n-1) // every caller but the leader holds its flight
	close(release)
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil || got[i] != want {
			t.Fatalf("get %d = (%p, %v), want (%p, nil)", i, got[i], errs[i], want)
		}
	}
	if s := scans.Load(); s != 1 {
		t.Fatalf("scan ran %d times, want 1", s)
	}
}

// TestCorpusCache_CancelledWaiterScansForItself proves a caller whose
// context has ended does not wait on another caller's scan: it runs its
// own, gets that result, and stores nothing.
func TestCorpusCache_CancelledWaiterScansForItself(t *testing.T) {
	c := newCorpusCache(corpusCacheLimit)
	key := corpusKey{root: "/r", commit: "c1"}
	leaderCorpus, ownCorpus := &successorCorpus{}, &successorCorpus{}
	started, release, leaderDone := make(chan struct{}), make(chan struct{}), make(chan struct{})

	go func() {
		defer close(leaderDone)
		_, _ = c.get(context.Background(), key, func() (*successorCorpus, error) {
			close(started)
			<-release
			return leaderCorpus, nil
		})
	}()
	<-started // the leader's flight is registered before its scan runs

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := c.get(cancelled, key, func() (*successorCorpus, error) { return ownCorpus, nil })
	if err != nil || got != ownCorpus {
		t.Fatalf("cancelled get = (%p, %v), want its own scan %p", got, err, ownCorpus)
	}

	close(release)
	<-leaderDone
	got, err = c.get(context.Background(), key, func() (*successorCorpus, error) {
		t.Fatal("scan ran again; want the leader's stored corpus")
		return nil, nil
	})
	if err != nil || got != leaderCorpus {
		t.Fatalf("get after the leader = (%p, %v), want the leader's corpus %p", got, err, leaderCorpus)
	}
}

// waitForWaiters blocks until at least n callers of c hold a flight and
// wait on it, failing the test after 10s. A caller counts itself under
// c.mu in the same critical section that reads the flight, so once it is
// counted it is bound to that flight's outcome.
func waitForWaiters(t *testing.T, c *corpusCache, n int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		c.mu.Lock()
		w := c.waiters
		c.mu.Unlock()
		if w >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d callers waiting after 10s, want %d", w, n)
		}
		runtime.Gosched()
	}
}

// within receives from ch, failing the test if nothing arrives in 10s.
func within[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatalf("%s: nothing after 10s", what)
		var zero T
		return zero
	}
}

// cacheGet is one get's outcome, sent back from a goroutine.
type cacheGet struct {
	corpus *successorCorpus
	err    error
}

// getAsync runs c.get for key on a new goroutine, with a scan that counts
// its runs in scans and returns corpus, and sends the outcome on the
// returned channel.
func getAsync(c *corpusCache, key corpusKey, corpus *successorCorpus, scans *atomic.Int32) <-chan cacheGet {
	out := make(chan cacheGet, 1)
	go func() {
		got, err := c.get(context.Background(), key, func() (*successorCorpus, error) {
			scans.Add(1)
			return corpus, nil
		})
		out <- cacheGet{corpus: got, err: err}
	}()
	return out
}

// assertStored proves key's stored corpus is want: a get returns it
// without scanning, and the cache is left with no flight or waiter.
func assertStored(t *testing.T, c *corpusCache, key corpusKey, want *successorCorpus) {
	t.Helper()
	got, err := c.get(context.Background(), key, func() (*successorCorpus, error) {
		t.Error("scan ran; want the stored corpus")
		return nil, errors.New("unexpected scan")
	})
	if err != nil || got != want {
		t.Fatalf("stored corpus = (%p, %v), want %p", got, err, want)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.flights) != 0 || c.waiters != 0 {
		t.Fatalf("%d flights and %d waiters left, want none", len(c.flights), c.waiters)
	}
}

// TestCorpusCache_WaiterScansWhenTheLeaderFails holds the leader's scan
// until a second caller is waiting on its flight, then fails it: the
// failure is not handed to the waiter, which is released, runs its own
// scan, and stores that corpus.
func TestCorpusCache_WaiterScansWhenTheLeaderFails(t *testing.T) {
	c := newCorpusCache(corpusCacheLimit)
	key := corpusKey{root: "/r", commit: "c1"}
	errBoom := errors.New("boom")
	started, fail := make(chan struct{}), make(chan struct{})

	leader := make(chan error, 1)
	go func() {
		_, err := c.get(context.Background(), key, func() (*successorCorpus, error) {
			close(started)
			<-fail
			return nil, errBoom
		})
		leader <- err
	}()
	<-started // the leader's flight is registered before its scan runs

	waiterCorpus := &successorCorpus{}
	var waiterScans atomic.Int32
	waiter := getAsync(c, key, waiterCorpus, &waiterScans)
	waitForWaiters(t, c, 1)
	if n := waiterScans.Load(); n != 0 {
		t.Fatalf("the waiter scanned %d times while the leader's scan was in flight, want 0", n)
	}

	close(fail)
	if err := within(t, leader, "leader"); !errors.Is(err, errBoom) {
		t.Fatalf("leader err = %v, want %v", err, errBoom)
	}
	got := within(t, waiter, "waiter released after the leader failed")
	if got.err != nil || got.corpus != waiterCorpus || waiterScans.Load() != 1 {
		t.Fatalf("waiter get = (%p, %v) after %d scans, want its own scan %p after 1", got.corpus, got.err, waiterScans.Load(), waiterCorpus)
	}
	assertStored(t, c, key, waiterCorpus)
}

// TestCorpusCache_PanickingScanStoresNothing holds a scan until a second
// caller is waiting on its flight, then panics it: the panic propagates,
// the panicking scan stores nothing, and the waiter is released, runs its
// own scan, and stores that corpus.
func TestCorpusCache_PanickingScanStoresNothing(t *testing.T) {
	c := newCorpusCache(corpusCacheLimit)
	key := corpusKey{root: "/r", commit: "c1"}
	started, panicNow := make(chan struct{}), make(chan struct{})

	leader := make(chan any, 1)
	go func() {
		defer func() { leader <- recover() }()
		_, _ = c.get(context.Background(), key, func() (*successorCorpus, error) {
			close(started)
			<-panicNow
			panic("scan failed")
		})
	}()
	<-started

	waiterCorpus := &successorCorpus{}
	var waiterScans atomic.Int32
	waiter := getAsync(c, key, waiterCorpus, &waiterScans)
	waitForWaiters(t, c, 1)

	close(panicNow)
	if r := within(t, leader, "leader"); r == nil {
		t.Fatal("the scan's panic did not propagate")
	}
	// The waiter runs its own scan only if the panicking scan left no
	// entry behind.
	got := within(t, waiter, "waiter released after the panic")
	if got.err != nil || got.corpus != waiterCorpus || waiterScans.Load() != 1 {
		t.Fatalf("waiter get = (%p, %v) after %d scans, want its own scan %p after 1", got.corpus, got.err, waiterScans.Load(), waiterCorpus)
	}
	assertStored(t, c, key, waiterCorpus)
}
