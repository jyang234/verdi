package main

// vocabFixture (spec/vocabulary-surfaces ac-2) answers the one behavioral
// case the shared harness corpus provably cannot: a served board whose
// store carries a vocab-rename model.yaml. A display rename is STORE-WIDE
// by design ("a rename can never leak partially") — planting model.yaml
// in the shared scratch store would rename every status chip and class
// tag other suites pin by their bare-id text, exactly the invasive,
// high-blast-radius mutation the empty-glance fixture's ADJ-40 rationale
// rules out. So, following that adjudicated convention verbatim
// (emptyglance.go): a SEPARATE, fully hermetic workbench instance,
// in-process, over a REAL minimal store on disk — git init + bare origin
// (load-bearing: refindex's default-branch walk keys off
// refs/remotes/origin/HEAD; without it the directory would render the
// no-default-branch degradation, not the store's entries) — served
// through the SAME production wiring the real workbench uses
// (workbench.NewHandler → RegisterRoutesWithHome → store.Open →
// Config.Model), so the rename the browser sees flowed through the real
// model-resolution pipe, never a canned label.
//
// The store's .verdi/model.yaml is internal/model/testdata's
// vocab-rename.yaml — model-schema's own frontier fixture (accept ->
// "Sign off", accepted-pending-build -> "Ready to build", feature ->
// "Initiative"), read from the module tree at provision time and reused
// verbatim, never duplicated. Its one spec, vocab-probe (a round-four
// feature, accepted-pending-build, on main), gives the home
// glance/directory a renamed status chip and the board a renamed
// case-file class tag.
import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/jyang234/verdi/internal/workbench"
)

// vocabProbeSpec is the store's one spec: a round-four feature (problem/
// outcome, so its board wears the case file and class tag), status
// accepted-pending-build (the state the vocab-rename model renames).
const vocabProbeSpec = `---
id: spec/vocab-probe
kind: spec
title: "Vocab probe"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:VOC-1
problem: { text: "display vocabulary is hard-coded per surface", anchor: problem }
outcome: { text: "one rename reaches every surface at once", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "renamed labels render in the browser", evidence: [behavioral] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Vocab probe

## Problem

Display vocabulary is hard-coded per surface.

## Outcome

One rename reaches every surface at once.
`

// vocabFixture lazily starts its isolated server and remembers its bound
// URL — the same start-once cache shape emptyGlanceFixture uses.
type vocabFixture struct {
	moduleRoot string

	mu  sync.Mutex
	url string
}

func newVocabFixture(moduleRoot string) *vocabFixture {
	return &vocabFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's URL as a plain-text body,
// starting the isolated server on the first call.
func (f *vocabFixture) handler(w http.ResponseWriter, r *http.Request) {
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

// ensureStarted provisions the vocab-rename store and starts the isolated
// workbench instance over it on first call, returning its URL on every
// call thereafter, unchanged.
func (f *vocabFixture) ensureStarted() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	root, err := provisionVocabStore(f.moduleRoot)
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

// provisionVocabStore builds a REAL, minimal, hermetic store on disk and
// returns its root: the manifest, the vocab-rename model.yaml (copied
// from the module tree's committed fixture), and the one probe spec —
// committed on main with a bare local origin whose HEAD names main
// (provisionEmptyStore's own load-bearing origin setup, mirrored), so
// refindex's real default-branch walk lists vocab-probe in the home
// directory rather than short-circuiting on the no-remote path.
func provisionVocabStore(moduleRoot string) (string, error) {
	tmp, err := os.MkdirTemp("", "verdi-e2e-vocab-*")
	if err != nil {
		return "", err
	}
	root := filepath.Join(tmp, "store")
	originDir := filepath.Join(tmp, "origin.git")

	specDir := filepath.Join(root, ".verdi", "specs", "active", "vocab-probe")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return "", fmt.Errorf("creating vocab-probe spec dir: %w", err)
	}

	modelYAML, err := os.ReadFile(filepath.Join(moduleRoot, "internal", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		return "", fmt.Errorf("reading the vocab-rename model fixture: %w", err)
	}

	files := map[string][]byte{
		filepath.Join(root, ".verdi", "verdi.yaml"): []byte(emptyStoreManifest),
		filepath.Join(root, ".verdi", "model.yaml"): modelYAML,
		filepath.Join(root, ".verdi", ".gitignore"): []byte("data/\n"),
		filepath.Join(specDir, "spec.md"):           []byte(vocabProbeSpec),
	}
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", path, err)
		}
	}

	if err := runGit(root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(root, nil, "commit", "--quiet", "--no-verify", "-m", "vocab-rename store: manifest, model.yaml, one accepted feature"); err != nil {
		return "", err
	}
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
