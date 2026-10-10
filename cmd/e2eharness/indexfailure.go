package main

// indexFailureFixture (spec/index-v2 ac-6; ledger SI-366 (15)): the one
// harness store whose directory index computation FAILS, so the index's
// failure path is proven against a real store, never a canned erroring
// HomeDeps.Index. Breaking the shared store would break every other suite
// that reads it, so this is a SEPARATE, hermetic, REAL store —
// emptyglance.go's in-process pattern (ADJ-40) with indexdates.go's
// dated-store shape: git init on main, a bare local origin whose HEAD
// names main (load-bearing: without a provable default branch the
// default-branch walk contributes nothing and nothing fails), and on main
// one intact component spec beside one spec whose frontmatter does not
// decode. A design branch carries a valid draft, so a render that showed
// any partial index would have something to show. The production
// workbench handler serves it in-process on its own loopback port,
// discovered through the control server's GET /index-failure-fixture
// (control.go), started on first request, reused thereafter, and stopped
// (its store removed) with the harness. Test-only.
//
// The undecodable spec sits in the serving working tree too (the store's
// checkout is main), so the corpus index the other-records listing reads
// fails on it as well — the same file, read by both computations.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/jyang234/verdi/internal/workbench"
)

// The fixture's spec names and paths. The index story's Playwright files
// mirror these (e2e/tests/fixtures.ts, lane F7); change them together.
const (
	indexFailureIntactName      = "index-failure-intact"
	indexFailureUndecodableName = "index-failure-undecodable"
	indexFailureDraftName       = "index-failure-draft"
	indexFailureIntactPath      = ".verdi/specs/active/" + indexFailureIntactName + "/spec.md"
	indexFailureUndecodablePath = ".verdi/specs/active/" + indexFailureUndecodableName + "/spec.md"
	indexFailureDraftPath       = ".verdi/specs/active/" + indexFailureDraftName + "/spec.md"
)

// indexFailureSpec is a minimal valid component spec named name, carrying
// status.
func indexFailureSpec(name, status string) string {
	return fmt.Sprintf("---\nid: spec/%s\nkind: spec\nclass: component\ntitle: \"%s (index-failure e2e fixture)\"\nstatus: %s\nowners: [platform-team]\n---\n# %s\n", name, name, status, name)
}

// indexFailureUndecodableSpec splits as frontmatter but fails the strict
// decode: it carries a field no spec declares.
func indexFailureUndecodableSpec() string {
	return fmt.Sprintf("---\nid: spec/%s\nkind: spec\nclass: component\ntitle: \"An undecodable spec (index-failure e2e fixture)\"\nstatus: active\nowners: [platform-team]\nnot_a_spec_field: this field fails the strict decode\n---\n# %s\n", indexFailureUndecodableName, indexFailureUndecodableName)
}

// provisionIndexFailureStore builds the failing store under parent and
// returns its root (parent/store; its bare origin is parent/origin.git):
// main's one commit, the origin whose HEAD names main, then the design
// branch's draft — main left checked out, the serving checkout's usual
// posture.
func provisionIndexFailureStore(ctx context.Context, parent string) (string, error) {
	root := filepath.Join(parent, "store")
	originDir := filepath.Join(parent, "origin.git")
	if err := writeIndexFailureFiles(root, map[string]string{
		".verdi/verdi.yaml":         emptyStoreManifest,
		".verdi/.gitignore":         "data/\n",
		indexFailureIntactPath:      indexFailureSpec(indexFailureIntactName, "active"),
		indexFailureUndecodablePath: indexFailureUndecodableSpec(),
	}); err != nil {
		return "", err
	}
	if err := initRepo(ctx, root, false); err != nil {
		return "", err
	}
	steps := [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "--no-verify", "-m", "index-failure store: an intact and an undecodable spec"},
	}
	for _, args := range steps {
		if err := runGit(ctx, root, nil, args...); err != nil {
			return "", err
		}
	}

	// A bare local origin whose HEAD names main (emptyglance.go's
	// load-bearing reasoning): the default branch resolves, so the
	// default-branch walk reads — and fails on — the undecodable spec.
	if err := initRepo(ctx, originDir, true); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"remote", "add", "origin", originDir},
		{"push", "--quiet", "--set-upstream", "origin", "main"},
		{"remote", "set-head", "origin", "main"},
		{"checkout", "--quiet", "-b", "design/" + indexFailureDraftName, "main"},
	} {
		if err := runGit(ctx, root, nil, args...); err != nil {
			return "", err
		}
	}
	if err := writeIndexFailureFiles(root, map[string]string{
		indexFailureDraftPath: indexFailureSpec(indexFailureDraftName, "draft"),
	}); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "--no-verify", "-m", "design start: " + indexFailureDraftName},
		{"checkout", "--quiet", "main"},
	} {
		if err := runGit(ctx, root, nil, args...); err != nil {
			return "", err
		}
	}
	return root, nil
}

// writeIndexFailureFiles writes files (repo-relative) under root.
func writeIndexFailureFiles(root string, files map[string]string) error {
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// indexFailureFixture lazily provisions the failing store and serves it
// in-process — emptyglance.go's start-once shape, plus the server and
// temporary directory stop() releases.
type indexFailureFixture struct {
	mu  sync.Mutex
	url string
	tmp string // the store's parent directory, removed by stop()
	srv *http.Server
}

func newIndexFailureFixture() *indexFailureFixture { return &indexFailureFixture{} }

// handler answers GET with the fixture's base URL as a plain-text body,
// starting the isolated server on the first call.
func (f *indexFailureFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted provisions the failing store in a fresh temporary
// directory and serves it through the production workbench.NewHandler
// (HomeDeps' zero value: the REAL refindex.ComputeIndex — co-1, ADJ-40),
// returning the URL on every call thereafter, unchanged. A failed start
// removes what it created.
func (f *indexFailureFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}
	tmp, err := os.MkdirTemp("", "verdi-e2e-index-failure-*")
	if err != nil {
		return "", err
	}
	root, err := provisionIndexFailureStore(ctx, tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("provisioning the index-failure store: %w", err)
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandler(root)}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "e2eharness: index-failure fixture server: %v\n", err)
		}
	}()
	f.url, f.tmp, f.srv = "http://"+ln.Addr().String()+"/", tmp, srv
	return f.url, nil
}

// stop closes the isolated server and removes the fixture's temporary
// store — and forgets its URL, which no longer serves anything. Safe when
// never started, and idempotent.
func (f *indexFailureFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srv != nil {
		_ = f.srv.Close()
		f.srv = nil
	}
	if f.tmp != "" {
		_ = os.RemoveAll(f.tmp)
		f.tmp = ""
	}
	f.url = ""
}
