package main

// objSupersedeFixture (design docs/superpowers/specs/2026-09-24-closed-
// spec-object-supersession-design.md §8; L3c report's "Item 5 handoff"):
// the surface lane's e2e proof needs one board and docs-site view per
// closed-spec-object-supersession scenario — a criterion target and a
// decision target, in force, carried, dropped, not yet accepted, and not
// established — and the shared harness store carries none of that
// authority shape. Rather than growing the shared store with six more
// specs and conflicts (which would also drift from the scenario fixture
// internal/objsupersede/scenario already commits and the Go package
// already tests against), this provisions EACH scenario's SCRATCH GIT
// REPOSITORY the same way (scenario.Materialize — never reimplementing
// the replay) and serves EACH from its OWN isolated `verdi serve`
// subprocess, the same build-then-exec seam unprovenboard.go and
// specimportfixture.go use, sharing ONE binary build across all six.
// Loopback only; every store is materialized and every serve started
// lazily on the control server's first GET /objsupersede-fixture, reused
// thereafter, and stopped with the harness. Test-only.
//
// A single serve per store is enough to answer both the default-branch
// view and the design-branch view that design §8's "not yet accepted"
// case needs: internal/workbench's per-branch board route
// (/b/{branch}/board/spec/{name}, branchboard.go) renders ANY branch
// present in the served repository — cutting a managed worktree for it
// on first request — regardless of what the serving checkout has
// currently checked out (proven in production by provision_draftboards.go
// and e2e/tests/38-draft-boards.spec.ts's single shared serve carrying
// five branches' boards at once). So the "proposed" and "no-conflict"
// stores, whose scenario.Checkout is design/successor, answer the
// default-branch (main) view at /b/main/board/spec/successor without a
// second subprocess, and every store's checked-out branch answers at the
// unprefixed /board/spec/<name>.
//
// Every store's successor decisions target the SAME two closed-spec
// objects (testdata/objsupersede/records/specs/successor*.md: dc-1 ->
// spec/closed-feature#dc-1, dc-2 -> spec/closed-story#ac-1) — design §8's
// one decision target and one criterion target — so those two refs are
// constants, not per-store facts.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// objSupersedeStores is the fixed, deterministic set of scenario stores
// the surface lane needs (L3c report's Item 5 handoff): the happy path
// (accepted), the carried and dropped whole-spec-revision chains (chain,
// chain-drop), the not-yet-accepted pair that together show both the
// default-branch and the design-branch view (proposed, no-conflict), and
// the broken-chain not-in-force case (chain-not-in-force). Order is fixed
// so the fixture's provisioning is deterministic, not map iteration.
var objSupersedeStores = []string{"accepted", "chain", "chain-drop", "proposed", "no-conflict", "chain-not-in-force"}

// objSupersedeSuccessor names, for each store, the ref of the successor
// spec whose board is the one under test on that store's checkout
// (testdata/objsupersede/scenarios.json's layers; the L3c report's own
// naming of the carried/dropped heads).
var objSupersedeSuccessor = map[string]string{
	"accepted":           "spec/successor",
	"chain":              "spec/successor-v3",
	"chain-drop":         "spec/successor-v2",
	"proposed":           "spec/successor",
	"no-conflict":        "spec/successor",
	"chain-not-in-force": "spec/successor-v2",
}

// objSupersedeDesignBranch names, for a store whose default-branch view
// (main) and design-branch view differ, the design branch to reach
// through /b/{branch}/board/spec/<name> — proposed and no-conflict are
// both checked out AT that design branch already (Checkout equals this
// value), and chain-not-in-force's checkout is also its own design
// branch; the field exists so a caller need not infer it from Checkout.
// accepted, chain, and chain-drop have no separate not-yet-accepted view
// left to show (every proposing branch there is already merged), so they
// carry none.
var objSupersedeDesignBranch = map[string]string{
	"proposed":           "design/successor",
	"no-conflict":        "design/successor",
	"chain-not-in-force": "design/successor-v2",
}

// Every store's successor decisions supersede these same two closed-spec
// objects — one decision target, one criterion target (design §8).
const (
	objSupersedeClosedDecision  = "spec/closed-feature#dc-1"
	objSupersedeClosedCriterion = "spec/closed-story#ac-1"
)

// objSupersedeStoreInfo is one store's facts, as the fixture endpoint
// reports them: enough for a Playwright spec to reach both the
// default-branch and the design-branch view without recomputing the
// scenario fixture's own shape.
type objSupersedeStoreInfo struct {
	Scenario            string `json:"scenario"`
	URL                 string `json:"url"`
	Checkout            string `json:"checkout"`
	MainBranch          string `json:"main_branch"`
	DesignBranch        string `json:"design_branch,omitempty"`
	Successor           string `json:"successor"`
	SupersededDecision  string `json:"superseded_decision"`
	SupersededCriterion string `json:"superseded_criterion"`
}

// objSupersedeFixtureInfo is the endpoint's whole JSON body: every store,
// keyed by scenario name.
type objSupersedeFixtureInfo struct {
	Stores map[string]objSupersedeStoreInfo `json:"stores"`
}

// objSupersedeServe is one started store: its reported facts, the
// repository root it serves, and the handles stop() reaps.
type objSupersedeServe struct {
	info   objSupersedeStoreInfo
	root   string
	cancel context.CancelFunc
	done   chan error
}

// objSupersedeFixture lazily materializes every store, builds the binary
// once, and starts one `verdi serve` subprocess per store — the same
// start-once cache shape as the other subprocess fixtures.
type objSupersedeFixture struct {
	moduleRoot string

	// start performs the real materialize -> build -> serve -> healthz
	// sequence (startAll) for every store. Tests substitute a fake to pin
	// the handler's lazy, start-once contract without six subprocesses.
	start func(ctx context.Context) (map[string]*objSupersedeServe, error)

	mu      sync.Mutex
	started bool
	serves  map[string]*objSupersedeServe
}

func newObjSupersedeFixture(moduleRoot string) *objSupersedeFixture {
	f := &objSupersedeFixture{moduleRoot: moduleRoot}
	f.start = f.startAll
	return f
}

// handler answers GET with the fixture's JSON body (objSupersedeFixtureInfo),
// materializing and starting every store on the first call.
func (f *objSupersedeFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// ensureStarted runs start once and returns the same info on every call
// thereafter, unchanged. A failed start caches nothing, so the next call
// retries. ctx bounds provisioning, the build, and every store's
// readiness wait — the request's own lifetime; the subprocesses
// themselves live until stop(). The zero value is usable: an unset start
// defaults to the real sequence.
func (f *objSupersedeFixture) ensureStarted(ctx context.Context) (objSupersedeFixtureInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.started {
		return f.infoLocked(), nil
	}
	if f.start == nil {
		f.start = f.startAll
	}
	serves, err := f.start(ctx)
	if err != nil {
		return objSupersedeFixtureInfo{}, err
	}
	if len(serves) == 0 {
		return objSupersedeFixtureInfo{}, fmt.Errorf("objsupersede fixture: start returned no stores")
	}
	f.serves, f.started = serves, true
	return f.infoLocked(), nil
}

// infoLocked assembles the JSON body from the started serves. Callers
// hold f.mu.
func (f *objSupersedeFixture) infoLocked() objSupersedeFixtureInfo {
	info := objSupersedeFixtureInfo{Stores: make(map[string]objSupersedeStoreInfo, len(f.serves))}
	for name, s := range f.serves {
		info.Stores[name] = s.info
	}
	return info
}

// stop terminates every started store's serve subprocess (SIGTERM, then
// the WaitDelay force-kill) and waits for each. Safe when never started,
// and idempotent.
func (f *objSupersedeFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.serves {
		if s.cancel == nil {
			continue
		}
		s.cancel()
		<-s.done
		s.cancel = nil
	}
}

// startAll builds the verdi binary once, materializes every store named
// by objSupersedeStores into its own scratch repository with
// scenario.Materialize (never reimplementing the replay), and starts one
// isolated `verdi serve` subprocess per store. On any failure it reaps
// whatever it already started before returning the error.
func (f *objSupersedeFixture) startAll(ctx context.Context) (map[string]*objSupersedeServe, error) {
	scratch, err := os.MkdirTemp("", "verdi-e2e-objsupersede-*")
	if err != nil {
		return nil, err
	}
	binPath := filepath.Join(scratch, "verdi")
	if err := buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return nil, fmt.Errorf("building verdi binary for the objsupersede fixture: %w", err)
	}

	fixtureDir := scenario.Dir()
	m, err := scenario.Load(fixtureDir)
	if err != nil {
		return nil, fmt.Errorf("loading the objsupersede scenario manifest: %w", err)
	}

	serves := make(map[string]*objSupersedeServe, len(objSupersedeStores))
	for _, name := range objSupersedeStores {
		sc, ok := m.Scenarios[name]
		if !ok {
			stopObjSupersedeServes(serves)
			return nil, fmt.Errorf("objsupersede fixture: scenario %q is not defined in the manifest", name)
		}
		root := filepath.Join(scratch, name)
		if err := os.MkdirAll(root, 0o755); err != nil {
			stopObjSupersedeServes(serves)
			return nil, err
		}
		if _, err := scenario.Materialize(ctx, fixtureDir, root, name); err != nil {
			stopObjSupersedeServes(serves)
			return nil, fmt.Errorf("materializing objsupersede scenario %q: %w", name, err)
		}

		serve, err := startObjSupersedeServe(ctx, binPath, root)
		if err != nil {
			stopObjSupersedeServes(serves)
			return nil, fmt.Errorf("starting verdi serve for objsupersede scenario %q: %w", name, err)
		}
		serve.info = objSupersedeStoreInfo{
			Scenario:            name,
			URL:                 serve.info.URL,
			Checkout:            sc.Checkout,
			MainBranch:          m.Commit.InitialBranch,
			DesignBranch:        objSupersedeDesignBranch[name],
			Successor:           objSupersedeSuccessor[name],
			SupersededDecision:  objSupersedeClosedDecision,
			SupersededCriterion: objSupersedeClosedCriterion,
		}
		serves[name] = serve
	}
	return serves, nil
}

// stopObjSupersedeServes reaps every already-started serve in serves —
// startAll's own cleanup when a later store fails to materialize or
// start, so a partial failure never leaks a subprocess.
func stopObjSupersedeServes(serves map[string]*objSupersedeServe) {
	for _, s := range serves {
		if s.cancel == nil {
			continue
		}
		s.cancel()
		<-s.done
	}
}

// startObjSupersedeServe starts one isolated `verdi serve` subprocess
// (the binary built from this tree) over root, loopback only, and waits
// for its healthz — the same build-then-exec seam as unprovenboard.go and
// specimportfixture.go. The returned serve's info carries only URL; the
// caller fills in the rest.
func startObjSupersedeServe(ctx context.Context, binPath, root string) (*objSupersedeServe, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// The subprocess outlives this request: its context is the fixture's
	// own, cancelled by stop() — SIGTERM first (main.go's exact posture),
	// then the stdlib's force-kill after WaitDelay.
	childCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(childCtx, binPath, "serve", "--http", addr)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	url := "http://" + addr + "/"
	if err := waitHealthy(ctx, url+"healthz", 20*time.Second); err != nil {
		cancel()
		<-done
		return nil, fmt.Errorf("waiting for healthz: %w", err)
	}
	return &objSupersedeServe{info: objSupersedeStoreInfo{URL: url}, root: root, cancel: cancel, done: done}, nil
}
