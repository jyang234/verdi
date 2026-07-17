package main

// emptyGlanceFixture (spec/home-status-glance ac-3/co-1) answers the one
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
// REAL minimal store on disk — git init + the one .verdi/verdi.yaml a
// valid store needs, with ZERO specs — served through the SAME production
// wiring the real workbench uses (workbench.NewHandler → home.Index →
// refindex.ComputeIndex). This is Controller adjudication ADJ-40
// (2026-07-16), sustaining co-1's letter: the empty-bucket claim must flow
// a REAL store through index computation, never a canned HomeDeps.Index
// standing in for the pipeline. An empty store carries no entries, so all
// three glance buckets render empty at once — the strongest, cheapest
// witness of ac-3 through the true pipe. It answers on its own loopback
// port, discovered via the control server's /empty-glance-fixture endpoint
// (control.go), started lazily on first request and reused thereafter.
import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

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

// ensureStarted provisions the real minimal store and starts the isolated
// workbench instance over it on first call, returning its URL on every
// call thereafter, unchanged. The handler is the production
// workbench.NewHandler (HomeDeps' zero value), so GET / drives the REAL
// refindex.ComputeIndex over the store — no canned index (co-1; ADJ-40).
func (f *emptyGlanceFixture) ensureStarted() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	root, err := provisionEmptyStore()
	if err != nil {
		return "", err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandler(root)}
	go func() { _ = srv.Serve(ln) }()

	f.url = "http://" + ln.Addr().String() + "/"
	return f.url, nil
}

// emptyStoreManifest is the minimal valid verdi.yaml a store needs
// (internal/store.Manifest.Validate: the schema literal alone — forge
// omitted, so it auto-detects; every other block optional). Enough for the
// directory to be a recognizable store root and for refindex.ComputeIndex
// to run its real default-branch walk over it.
const emptyStoreManifest = "schema: verdi.layout/v1\n"

// provisionEmptyStore builds a REAL, minimal, hermetic store on disk and
// returns its root: git init on main, one commit of .verdi/verdi.yaml, and
// a bare local origin whose HEAD names main. There are ZERO specs, so
// refindex.ComputeIndex's real default-branch walk (over .verdi/specs/
// active and .verdi/specs/archive, both absent at this ref) returns an
// empty index and all three glance buckets render empty through the true
// pipeline (co-1; ADJ-40).
//
// The bare origin + `remote set-head` is load-bearing: gitx.DefaultBranch
// keys off refs/remotes/origin/HEAD (internal/gitx/branch.go), so without
// it the walk would short-circuit on the no-default-branch path and the
// empty result would prove only the no-remote degradation — not an empty
// store genuinely walked to zero entries. This mirrors the main harness
// store's own origin setup (provision_board.go).
func provisionEmptyStore() (string, error) {
	tmp, err := os.MkdirTemp("", "verdi-e2e-empty-glance-*")
	if err != nil {
		return "", err
	}
	root := filepath.Join(tmp, "store")
	originDir := filepath.Join(tmp, "origin.git")

	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(emptyStoreManifest), 0o644); err != nil {
		return "", err
	}

	// git init on main + the single manifest commit — the same
	// deterministic-env, no-verify posture every other scratch store here
	// uses (git.go).
	if err := runGit(root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "commit", "--quiet", "--no-verify", "-m", "empty store: verdi.yaml only, zero specs"); err != nil {
		return "", err
	}

	// A bare local origin whose HEAD names main, so gitx.DefaultBranch
	// resolves "main" and refindex's default-branch walk runs for real.
	if err := runGit("", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}

	return root, nil
}
