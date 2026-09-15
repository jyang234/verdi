package main

// specImportFixture (spec-import-contract Task 4 UI; e2e/tests/
// 72-spec-import.spec.ts): the browser import journey needs a serve whose
// checkout is CLEAN (the importer's own clean-context gate refuses an
// uncommitted or untracked corpus input) and whose default branch is
// provable, so a created design/<slug> board renders in authoring mode.
// The shared harness store is neither by the time the suite reaches this
// file — earlier specs autosave, mutate and commit onto its serving
// checkout — so this provisions a SEPARATE, hermetic, REAL minimal store:
// the manifest and data-zone gitignore committed on main, a bare local
// origin whose HEAD names main (the same synthetic default-branch proof
// provision_board.go/emptyglance.go give their stores — never CI, never
// an owner approval), NO adopted assistance policy, NO model override, NO
// forge/tracker configuration. It serves that store through the SAME
// build-then-exec seam main.go and unprovenboard.go use: the real `verdi
// serve` subprocess of the binary built from this tree, under
// hermeticServeEnv (every feed/verification injection variable and CI
// identity variable stripped), so the human import the browser drives
// proceeds with zero model, provider, forge or tracker calls and the
// writer lock is serve's own lifetime lock. Loopback only; started lazily
// on GET /spec-import-fixture, reused thereafter, stopped with the
// harness. Test-only.
//
// Two helper endpoints serve the suite's honesty cases:
//
//   - GET  /spec-import-fixture/info   facts about the provisioned store the
//     spec asserts its "no configuration dependency" claim against.
//   - POST /spec-import-fixture/tamper?branch=design/<slug>&spec=<slug>
//     truncates that branch's committed import record.json as an ordinary
//     out-of-band descendant commit (through a DETACHED temporary
//     worktree, so neither the serving checkout nor the branch's managed
//     worktree is touched) — the corrupted-proof case the record view must
//     disclose as unavailable, never as the original.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// specImportSlugRe is the bare spec-name grammar the tamper endpoint
// accepts for ?spec= — the same shape internal/workbench's specNameRe pins.
var specImportSlugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// specImportStoreGitignore keeps the data zone (managed worktrees, locks,
// annotations) untracked so the importer's clean-context gate never trips
// on verdi's own runtime state.
const specImportStoreGitignore = "data/\n"

// specImportStoreManifest is the layout manifest plus ONE synthetic tracker
// provider — the exact shape internal/specimport/compose_external_test.go's
// trackerManifestYAML gives its story fixtures, because VL-005 requires a
// configured scheme before a story's tracker ref counts. Test-only: the
// base URL is never contacted (lint checks configuration, not reachability),
// and no forge, model override or policy is configured.
const specImportStoreManifest = "schema: verdi.layout/v1\n" +
	"providers:\n" +
	"  jira:\n" +
	"    base_url: https://example.atlassian.net\n" +
	"    rollup_field: customfield_00000\n"

// specImportTrackerScheme/specImportParentSlug name the synthetic tracker
// scheme and the landed parent feature the browser's story import
// implements. Mirrored by e2e/tests/fixtures.ts; change them together.
const (
	specImportTrackerScheme = "jira"
	specImportParentSlug    = "widget-parent"
)

// specImportParentSpecRel is the committed parent feature fixture (a
// statusless landed feature with one criterion), read from the module root
// exactly like unprovenBoardSpecRel — shared with internal/workbench's own
// handler tests so both suites implement the same parent.
var specImportParentSpecRel = filepath.Join("internal", "workbench", "testdata", "specimport", "parent-feature.md")

type specImportFixture struct {
	moduleRoot string

	mu     sync.Mutex
	url    string
	root   string
	env    []string
	cancel context.CancelFunc
	done   chan error
}

func newSpecImportFixture(moduleRoot string) *specImportFixture {
	return &specImportFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's URL as a plain-text body, starting
// the isolated serve on the first call.
func (f *specImportFixture) handler(w http.ResponseWriter, r *http.Request) {
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

// specImportFixtureInfo is the info endpoint's shape: the store facts the
// browser suite asserts (a manifest-only store, no policy, no model
// override, no forge/tracker/provider configuration, a hermetic child env).
type specImportFixtureInfo struct {
	URL              string   `json:"url"`
	Manifest         string   `json:"manifest"`
	PolicyAdopted    bool     `json:"policy_adopted"`
	ModelOverride    bool     `json:"model_override"`
	SyntheticTracker string   `json:"synthetic_tracker"`
	ParentFeature    string   `json:"parent_feature"`
	StrippedEnv      []string `json:"stripped_env"`
	Branch           string   `json:"branch"`
	Porcelain        string   `json:"porcelain"`
}

func (f *specImportFixture) infoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	f.mu.Lock()
	root := f.root
	f.mu.Unlock()
	manifest, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	branch, err := gitOutput(r.Context(), root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	porcelain, err := gitOutput(r.Context(), root, "status", "--porcelain")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, policyErr := os.Stat(filepath.Join(root, ".verdi", "policy"))
	_, modelErr := os.Stat(filepath.Join(root, ".verdi", "model.yaml"))
	info := specImportFixtureInfo{
		URL:              url,
		Manifest:         string(manifest),
		PolicyAdopted:    policyErr == nil,
		ModelOverride:    modelErr == nil,
		SyntheticTracker: specImportTrackerScheme,
		ParentFeature:    specImportParentSlug,
		StrippedEnv:      append(append([]string{}, serveInjectionEnvVars...), ciEnvVars...),
		Branch:           branch,
		Porcelain:        porcelain,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// tamperHandler truncates the committed import record.json of ?spec= on
// ?branch= to its first ten bytes as one ordinary descendant commit —
// through a detached temporary worktree, never touching the serving
// checkout or the branch's managed worktree — so the record view's
// corrupted-proof surface can be driven from the browser. Design-namespace
// branches only; the spec must be a bare spec name.
func (f *specImportFixture) tamperHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	branch := r.URL.Query().Get("branch")
	slug := r.URL.Query().Get("spec")
	if !strings.HasPrefix(branch, "design/") || !specImportSlugRe.MatchString(strings.TrimPrefix(branch, "design/")) {
		http.Error(w, "only design/<slug> branches may be tampered", http.StatusBadRequest)
		return
	}
	if !specImportSlugRe.MatchString(slug) {
		http.Error(w, "spec must be a bare spec name", http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	root := f.root
	f.mu.Unlock()
	if root == "" {
		http.Error(w, "spec-import fixture is not started", http.StatusConflict)
		return
	}
	if err := tamperImportRecord(r.Context(), root, branch, slug); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tamperImportRecord is tamperHandler's git work: detach a temporary
// worktree at branch's tip, truncate every record.json under
// .verdi/imports/<slug>/, commit, and advance the branch ref with a
// compare-and-swap against the tip it started from.
func tamperImportRecord(ctx context.Context, root, branch, slug string) error {
	tip, err := gitOutput(ctx, root, "rev-parse", "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", branch, err)
	}
	tmp, err := os.MkdirTemp("", "verdi-e2e-spec-import-tamper-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	wt := filepath.Join(tmp, "wt")
	if err := runGit(ctx, root, nil, "worktree", "add", "--detach", "--quiet", wt, tip); err != nil {
		return err
	}
	defer func() { _ = runGit(ctx, root, nil, "worktree", "remove", "--force", wt) }()

	importsDir := filepath.Join(wt, ".verdi", "imports", slug)
	entries, err := os.ReadDir(importsDir)
	if err != nil {
		return fmt.Errorf("no import records for %s on %s: %w", slug, branch, err)
	}
	truncated := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		recordPath := filepath.Join(importsDir, entry.Name(), "record.json")
		original, err := os.ReadFile(recordPath)
		if err != nil {
			continue
		}
		if len(original) <= 10 {
			return fmt.Errorf("record %s is implausibly short (%d bytes)", recordPath, len(original))
		}
		if err := os.WriteFile(recordPath, original[:10], 0o644); err != nil {
			return err
		}
		truncated++
	}
	if truncated == 0 {
		return fmt.Errorf("no record.json found under %s on %s", importsDir, branch)
	}
	if err := runGit(ctx, wt, nil, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(ctx, wt, nil, "commit", "--quiet", "--no-verify", "-m", "e2e: tamper with the import record"); err != nil {
		return err
	}
	next, err := gitOutput(ctx, wt, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	return runGit(ctx, root, nil, "update-ref", "refs/heads/"+branch, next, tip)
}

// ensureStarted provisions the real store, builds the binary from
// moduleRoot exactly as main.go's buildBinary does, starts `verdi serve
// --http <loopback>` over the store under hermeticServeEnv, waits for its
// healthz, and returns the URL on every call thereafter, unchanged.
func (f *specImportFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	root, err := provisionSpecImportStore(ctx, f.moduleRoot)
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(filepath.Dir(root), "verdi")
	if err := buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return "", fmt.Errorf("building verdi binary for the spec-import fixture: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	childCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(childCtx, binPath, "serve", "--http", addr)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = root
	env := hermeticServeEnv(os.Environ())
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("starting verdi serve for the spec-import fixture: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	url := "http://" + addr + "/"
	if err := waitHealthy(ctx, url+"healthz", 20*time.Second); err != nil {
		cancel()
		<-done
		return "", fmt.Errorf("waiting for the spec-import fixture's verdi serve: %w", err)
	}
	f.url, f.root, f.env, f.cancel, f.done = url, root, env, cancel, done
	return f.url, nil
}

// stop terminates the fixture's serve subprocess and waits for it. Safe
// when never started, and idempotent.
func (f *specImportFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel == nil {
		return
	}
	f.cancel()
	<-f.done
	f.cancel = nil
}

// provisionSpecImportStore builds the real minimal store and returns its
// root: git init on main, one commit of the manifest (layout plus the one
// synthetic tracker provider), the data-zone gitignore and the landed
// parent feature (specImportParentSpecRel, read from moduleRoot), a
// committed identity for the board's commit affordance, and a bare local
// origin whose HEAD names main (the synthetic default-branch proof). The
// checkout is left clean on main — the importer's precondition. No policy,
// model override or forge is configured.
func provisionSpecImportStore(ctx context.Context, moduleRoot string) (string, error) {
	parent, err := os.ReadFile(filepath.Join(moduleRoot, specImportParentSpecRel))
	if err != nil {
		return "", fmt.Errorf("reading spec-import parent feature fixture: %w", err)
	}
	tmp, err := os.MkdirTemp("", "verdi-e2e-spec-import-*")
	if err != nil {
		return "", err
	}
	root := filepath.Join(tmp, "store")
	originDir := filepath.Join(tmp, "origin.git")

	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(specImportStoreManifest), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", ".gitignore"), []byte(specImportStoreGitignore), 0o644); err != nil {
		return "", err
	}
	parentDir := filepath.Join(root, ".verdi", "specs", "active", specImportParentSlug)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(parentDir, "spec.md"), parent, 0o644); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	// The board's commit affordance (the ordinary supported edit the suite
	// makes after import) commits with the checkout's own identity, exactly
	// as provision_board.go configures the shared store.
	for _, kv := range [][2]string{{"user.name", "verdi-e2e"}, {"user.email", "e2e@verdi.invalid"}, {"commit.gpgsign", "false"}} {
		if err := runGit(ctx, root, nil, "config", kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "-m", "spec-import store: manifest with a synthetic tracker, one landed parent feature, no policy"); err != nil {
		return "", err
	}

	if err := runGit(ctx, "", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}
	return root, nil
}
