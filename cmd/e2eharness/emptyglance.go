package main

// emptyGlanceFixture (spec/home-status-glance ac-3/dc-4) answers the one
// behavioral case the shared harness corpus provably cannot: a render
// where a glance bucket has NO matching entries at all. Every bucket in
// the main provisioned store is populated by real, committed showcase
// fixtures OTHER suites depend on (in-flight alone: stale-decline,
// escrow-autopay, and every accepted-pending-build story/feature the dex
// suite exercises) — mutating or deleting any of them to manufacture an
// empty bucket would be exactly the invasive, high-blast-radius change
// CLAUDE.md's "smallest reversible option" rules out, and on-the-desk can
// never be emptied at all (every design-branch fixture in this run lands
// there).
//
// Rather than widen the shared corpus's risk surface, this spawns a
// SEPARATE, fully hermetic workbench instance, in-process, backed by a
// canned HomeDeps.Index (the exact seam internal/workbench's own Go tests
// already drive — see internal/workbench/glance_test.go's cannedIndex) —
// no git, no store on disk, no shared state with the main harness store.
// It answers on its own loopback port, discovered via the control
// server's /empty-glance-fixture endpoint (control.go), started lazily on
// first request and reused thereafter.
import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/jyang234/verdi/internal/refindex"
	"github.com/jyang234/verdi/internal/workbench"
)

// emptyGlanceFixture lazily starts its isolated server and remembers its
// bound URL — a sync.Mutex-guarded, start-once cache, not a pool: the
// suite only ever needs the one instance.
type emptyGlanceFixture struct {
	mu  sync.Mutex
	url string
}

func newEmptyGlanceFixture() *emptyGlanceFixture { return &emptyGlanceFixture{} }

// handler answers GET with the fixture's URL as a plain-text body,
// starting the isolated server on the first call.
func (f *emptyGlanceFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted starts the isolated workbench instance on first call and
// returns its URL on every call thereafter, unchanged.
func (f *emptyGlanceFixture) ensureStarted() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	// No git, no .verdi tree: HomeDeps.Index below is canned (bypassing
	// refindex.ComputeIndex entirely), and every other renderHome section
	// degrades to its own honest "nothing here"/"could not read" notice
	// against an empty directory — exactly like a half-initialised store,
	// which the home page is already required to serve (dc-5's own
	// "never itself a dead end" bar). This test only asserts the glance.
	root, err := os.MkdirTemp("", "verdi-e2e-empty-glance-*")
	if err != nil {
		return "", err
	}

	// One on-the-desk draft, and deliberately NOTHING for in-flight or
	// settling — the "at least one glance bucket has no matching entries"
	// shape ac-3's obligation demands, proven for TWO of the three buckets
	// at once rather than exactly the minimum one.
	entries := []refindex.Entry{
		{
			Ref:         "spec/lone-draft",
			Source:      refindex.SourceLocal,
			StatusGroup: refindex.StatusGroupDraftsInProgress,
			SpecStatus:  "draft",
			Zone:        refindex.ZoneActive,
		},
	}
	home := workbench.HomeDeps{
		Index: func(context.Context) ([]refindex.Entry, error) { return entries, nil },
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandlerWithHome(root, workbench.Deps{}, home)}
	go func() { _ = srv.Serve(ln) }()

	f.url = "http://" + ln.Addr().String() + "/"
	return f.url, nil
}
