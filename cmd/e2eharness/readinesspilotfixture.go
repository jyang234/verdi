package main

// readinessPilotFixture (controller ruling R-RR1-23; e2e/tests/
// 49-readiness-pilot.spec.ts): the readiness cockpit derives per request
// (spec/readiness-recovery ac-2), so its exact-array oracles describe the
// shared store's FRESH posture — the one the alphabetical full run has
// already spent by the time suite 49 starts (earlier suites leave open
// board stickies, a spike claim, an extra open question, and an edited
// spec in the shared store, and every one of them is a concern the page
// now honestly shows). Rather than re-pin the oracles to whatever the
// preceding suites happen to leave behind, this serves the readiness
// pilot from an ISOLATED store provisioned by the SAME sequence the shared
// store gets (provisionSharedStore — one piece of code, so the two cannot
// drift), built from this tree through main.go's own build-then-exec seam
// and started with the SAME serve posture the shared serve has: the
// --context-request flag and the three injection variables (its own canned
// review feed and diagram verification report, the control server's
// open-MR feed) — deliberately NOT hermeticServeEnv-stripped, because the
// suite's oracles were captured against that posture. Loopback only;
// started lazily on the control server's GET /readiness-pilot-fixture,
// reused thereafter, stopped with the harness (main.go defers stop).
// Test-only.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// readinessPilotServe is one started isolated serve: its base URL, the
// store it serves, the exact environment handed to exec, and the handles
// stop() reaps.
type readinessPilotServe struct {
	url    string
	store  sharedStore
	env    []string
	cancel context.CancelFunc
	done   chan error
}

// readinessPilotFixture lazily provisions the store, builds the binary,
// starts its `verdi serve` subprocess, and remembers the bound URL — the
// same start-once cache shape as unprovenBoardFixture and
// specImportFixture. moduleRoot locates the fixtures and the tree the
// binary is built from; openMRFeedURL is the control server's /openmrs,
// the same feed the shared serve consults.
type readinessPilotFixture struct {
	moduleRoot    string
	openMRFeedURL string

	// start performs the real provision → build → serve → healthz sequence
	// (startServe). Tests substitute a fake to pin the handler's lazy,
	// start-once contract without a subprocess.
	start func(ctx context.Context) (*readinessPilotServe, error)

	mu    sync.Mutex
	serve *readinessPilotServe
}

func newReadinessPilotFixture(moduleRoot, openMRFeedURL string) *readinessPilotFixture {
	f := &readinessPilotFixture{moduleRoot: moduleRoot, openMRFeedURL: openMRFeedURL}
	f.start = f.startServe
	return f
}

// handler answers GET with the fixture's base URL (http://127.0.0.1:<port>/)
// as a plain-text body, starting the isolated serve on the first call.
func (f *readinessPilotFixture) handler(w http.ResponseWriter, r *http.Request) {
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

// ensureStarted runs start once and returns the same URL on every call
// thereafter, unchanged. A failed start caches nothing, so the next call
// retries. ctx bounds provisioning, the build, and the readiness wait —
// the request's own lifetime; the subprocess itself lives until stop().
// The zero value is usable: an unset start defaults to the real sequence.
func (f *readinessPilotFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.serve != nil {
		return f.serve.url, nil
	}
	if f.start == nil {
		f.start = f.startServe
	}
	serve, err := f.start(ctx)
	if err != nil {
		return "", err
	}
	if serve == nil || serve.url == "" {
		return "", fmt.Errorf("readiness-pilot fixture: start returned no serve")
	}
	f.serve = serve
	return f.serve.url, nil
}

// startServe provisions the shared-shape store into its own scratch
// directory (no dex build — the fixture serves no static site), builds
// the binary from moduleRoot exactly as main.go's buildBinary does for the
// shared store, starts `verdi serve --http <loopback> --context-request
// <request>` over the store under the shared serve's exact environment,
// and waits for its healthz.
func (f *readinessPilotFixture) startServe(ctx context.Context) (*readinessPilotServe, error) {
	scratch, err := os.MkdirTemp("", "verdi-e2e-readiness-pilot-*")
	if err != nil {
		return nil, err
	}
	store, err := provisionSharedStore(ctx, f.moduleRoot, scratch, nil)
	if err != nil {
		return nil, fmt.Errorf("provisioning the readiness-pilot fixture store: %w", err)
	}
	binPath := filepath.Join(scratch, "verdi")
	if err := buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return nil, fmt.Errorf("building verdi binary for the readiness-pilot fixture: %w", err)
	}

	// A free loopback port, released to the child: the probe listener is
	// closed before serve binds the same address.
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
	cmd := exec.CommandContext(childCtx, binPath, "serve", "--http", addr, "--context-request", store.readinessRequestPath)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = store.storeRoot
	// The shared serve's exact environment (sharedServeEnv): the ambient
	// process environment plus this store's own review feed and
	// verification report and the control server's open-MR feed — NOT
	// hermeticServeEnv-stripped, on purpose (see the file comment).
	env := sharedServeEnv(os.Environ(), store, f.openMRFeedURL)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting verdi serve for the readiness-pilot fixture: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	url := "http://" + addr + "/"
	if err := waitHealthy(ctx, url+"healthz", 20*time.Second); err != nil {
		cancel()
		<-done
		return nil, fmt.Errorf("waiting for the readiness-pilot fixture's verdi serve: %w", err)
	}
	return &readinessPilotServe{url: url, store: store, env: env, cancel: cancel, done: done}, nil
}

// stop terminates the fixture's serve subprocess (SIGTERM, then the
// WaitDelay force-kill) and waits for it. Safe when never started, and
// idempotent.
func (f *readinessPilotFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.serve == nil || f.serve.cancel == nil {
		return
	}
	f.serve.cancel()
	<-f.serve.done
	f.serve.cancel = nil
}
