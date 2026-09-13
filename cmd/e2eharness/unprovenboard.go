package main

// unprovenBoardFixture (MVP release amendment R2 — merge-signaled
// acceptance AC-7: "Missing default-branch or ancestry evidence produces
// an explicit unproven result, never an assumed acceptance"): the shared
// harness store deliberately PROVES its default branch (a bare local
// origin whose HEAD names main — provision_board.go), so the one lifecycle
// shape the browser suite could never reach there is the baseline report's
// B-06: a spec whose acceptance is UNPROVEN because no default branch
// resolves at all. Rather than unset origin/HEAD on the shared store
// mid-run (every other suite depends on it resolving), this provisions a
// SEPARATE, hermetic, REAL minimal store on disk — git init on main, the
// manifest committed, the CLI's statusless scaffold shape (the committed
// testdata fixture) committed on its design branch, NO remote, no
// origin/HEAD, no remote-tracking main/master — and serves it through the
// SAME build-then-exec seam main.go uses for the shared store: the real
// `verdi serve` subprocess of the binary built from this tree. Every route
// the browser drives is therefore the shipped binary's own wiring —
// including the design bridge, so a refused mutation is the adapter's
// read-only 403 and never the "design-service-unwired" 500 an in-process
// handler without Deps.Design would answer. Loopback only; started lazily
// on the control server's GET /unproven-board-fixture, reused thereafter,
// and stopped with the harness (main.go defers stop). Test-only. The child
// runs under hermeticServeEnv: the shared store's feed/verification
// injection variables and every CI identity variable are stripped from
// whatever ambient environment spawned the harness or a test, so an
// ambient CI_DEFAULT_BRANCH can never make this store's default branch
// resolvable and no ambient feed URL is ever consulted.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// serveInjectionEnvVars are the serve-side injection seams the SHARED
// harness store uses (main.go's serveCmd.Env): the canned review feed, the
// control server's open-MR feed URL, and the canned diagram verification
// report. The unproven-board fixture claims a hermetic, feed-less, no-CI
// serve, so none of them — nor any CI identity variable (ciEnvVars) — may
// reach its child, whatever ambient environment spawned the harness or a
// test.
var serveInjectionEnvVars = []string{"VERDI_REVIEW_FEED", "VERDI_OPENMR_FEED", "VERDI_DIAGRAM_VERIFICATION"}

// hermeticServeEnv is the child's environment: ambient (PATH, HOME, TMPDIR,
// git identity — what `verdi serve` and its git calls genuinely need) with
// every serveInjectionEnvVars and ciEnvVars entry stripped. A pure function
// of its input so the isolation is directly testable; the fixture feeds it
// os.Environ().
func hermeticServeEnv(ambient []string) []string {
	drop := make(map[string]bool, len(serveInjectionEnvVars)+len(ciEnvVars))
	for _, key := range serveInjectionEnvVars {
		drop[key] = true
	}
	for _, key := range ciEnvVars {
		drop[key] = true
	}
	out := make([]string, 0, len(ambient))
	for _, kv := range ambient {
		key, _, _ := strings.Cut(kv, "=")
		if drop[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// unprovenBoardSpecName is the fixture spec's name — the board lives at
// <fixture URL>board/spec/<name>. Mirrored verbatim by
// e2e/tests/fixtures.ts (EDGE.UNPROVEN_BOARD_SPEC) and by the committed
// fixture's own `id:` line; change them together.
const unprovenBoardSpecName = "unproven-lifecycle-feature"

// unprovenBoardSpecRel is the committed fixture — the CLI's statusless
// scaffold shape (no status: field, so lifecycle derives from Git alone)
// with a problem/outcome pair and one acceptance criterion, so the wall
// carries a positioned card for the drag-refusal assertion — read from
// the module root at provisioning time, the same moduleRoot-relative idiom
// every other committed fixture this harness consumes uses (main.go's
// seedSupersessionForge, provision_vocab.go's model.yaml).
var unprovenBoardSpecRel = filepath.Join("cmd", "e2eharness", "testdata", "unproven-board", "spec.md")

// unprovenBoardFixture lazily builds the binary, provisions the store,
// starts its `verdi serve` subprocess, and remembers the bound URL — the
// same start-once cache shape as emptyGlanceFixture, plus the subprocess
// handle stop() reaps. moduleRoot locates the committed fixture bytes and
// the tree the binary is built from.
type unprovenBoardFixture struct {
	moduleRoot string

	mu     sync.Mutex
	url    string
	root   string
	env    []string // the exact environment handed to exec (hermeticServeEnv); inspected by the isolation test
	cancel context.CancelFunc
	done   chan error
}

func newUnprovenBoardFixture(moduleRoot string) *unprovenBoardFixture {
	return &unprovenBoardFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's URL as a plain-text body,
// starting the isolated serve on the first call.
func (f *unprovenBoardFixture) handler(w http.ResponseWriter, r *http.Request) {
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

// ensureStarted provisions the real no-remote store, builds the binary
// from moduleRoot exactly as main.go's buildBinary does for the shared
// store, starts `verdi serve --http <loopback>` over the store, waits for
// its healthz, and returns the URL on every call thereafter, unchanged.
// ctx bounds provisioning, the build, and the readiness wait — the
// request's own lifetime; the subprocess itself lives until stop().
func (f *unprovenBoardFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	root, err := provisionUnprovenStore(ctx, f.moduleRoot)
	if err != nil {
		return "", err
	}
	binPath := filepath.Join(filepath.Dir(root), "verdi")
	if err := buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return "", fmt.Errorf("building verdi binary for the unproven-board fixture: %w", err)
	}

	// A free loopback port, released to the child: the probe listener is
	// closed before serve binds the same address.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
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
	// A hermetic environment: the ambient process environment with every
	// serve injection seam and CI identity variable stripped
	// (hermeticServeEnv) — no review feed, no open-MR feed, no canned
	// verification, no CI default branch — so lifecycle resolution answers
	// to the store alone, whatever spawned this process.
	env := hermeticServeEnv(os.Environ())
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("starting verdi serve for the unproven-board fixture: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	url := "http://" + addr + "/"
	if err := waitHealthy(ctx, url+"healthz", 20*time.Second); err != nil {
		cancel()
		<-done
		return "", fmt.Errorf("waiting for the unproven-board fixture's verdi serve: %w", err)
	}
	f.url, f.root, f.env, f.cancel, f.done = url, root, env, cancel, done
	return f.url, nil
}

// stop terminates the fixture's serve subprocess (SIGTERM, then the
// WaitDelay force-kill) and waits for it. Safe when never started, and
// idempotent.
func (f *unprovenBoardFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel == nil {
		return
	}
	f.cancel()
	<-f.done
	f.cancel = nil
}

// provisionUnprovenStore builds the real minimal store and returns its
// root: git init on main, one commit of the manifest, then the committed
// statusless fixture spec (unprovenBoardSpecRel, read from moduleRoot)
// committed on design/<name> and left checked out — the posture
// `verdi design start` leaves behind in a fresh project. NO remote is
// added, on purpose: with no CI_DEFAULT_BRANCH, no origin/HEAD, and no
// refs/remotes/origin/{main,master}, specstate.ResolveDefaultBranch's whole
// chain is exhausted and the effective state is Unproven — the one fact
// this fixture exists to reach. Nothing in this harness may add a remote
// to this store.
func provisionUnprovenStore(ctx context.Context, moduleRoot string) (string, error) {
	spec, err := os.ReadFile(filepath.Join(moduleRoot, unprovenBoardSpecRel))
	if err != nil {
		return "", fmt.Errorf("reading unproven-board fixture spec: %w", err)
	}

	tmp, err := os.MkdirTemp("", "verdi-e2e-unproven-board-*")
	if err != nil {
		return "", err
	}
	root := filepath.Join(tmp, "store")

	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(emptyStoreManifest), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		return "", err
	}

	// git init on main + the manifest commit — the same deterministic-env,
	// no-verify posture every other scratch store here uses (git.go).
	if err := runGit(ctx, root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "-m", "unproven-board store: manifest only"); err != nil {
		return "", err
	}

	// The statusless scaffold on its own design branch, committed there
	// and left checked out — never on main.
	if err := runGit(ctx, root, nil, "checkout", "--quiet", "-b", "design/"+unprovenBoardSpecName); err != nil {
		return "", err
	}
	specDir := filepath.Join(root, ".verdi", "specs", "active", unprovenBoardSpecName)
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), spec, 0o644); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "commit", "--quiet", "--no-verify", "-m", "design start: "+unprovenBoardSpecName+" (statusless scaffold)"); err != nil {
		return "", err
	}

	return root, nil
}
