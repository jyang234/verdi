// Command e2eharness is the Playwright e2e suite's Go test-server launcher
// (CLAUDE.md: "every browser-facing behavioral path ... a Playwright e2e
// test"; PLAN.md Phase 10 deliverable 4's "harness: a Go test-server
// launcher ... that builds the binary, provisions a scratch fixturegit
// store, starts `verdi serve`, runs playwright, tears down"). It does the
// "build + provision + start" third of that: builds the real verdi binary
// (build-then-exec, not `go run`, so the suite exercises the exact binary
// CI would ship — mirroring the Makefile's own lint-store target),
// provisions a scratch store from testdata/corpus (the same fixture
// internal/workbench's own Go tests build on — provision.go seeds it as
// one throwaway real git commit, not fixturegit's golden-SHA-pinned
// build, since nothing here asserts a specific commit hash), builds a
// static dex site from it, and serves both:
//
//   - http://127.0.0.1:4173 — `verdi serve`'s workbench (real subprocess)
//   - http://127.0.0.1:4174 — the built dex site (plain http.FileServer)
//
// e2e/playwright.config.ts's webServer stanza runs this as its command and
// polls :4173/healthz for readiness; Playwright itself owns "runs
// playwright" and "tears down" (SIGTERM to this process on suite exit,
// which this program's signal handler turns into a clean subprocess stop).
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/OWNER/verdi/internal/dex"
)

const (
	workbenchAddr = "127.0.0.1:4173"
	dexAddr       = "127.0.0.1:4174"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("e2eharness: %v", err)
	}
}

func run() error {
	moduleRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	// Sanity check: cwd must be the verdi Go module root (go.mod present) —
	// e2e/playwright.config.ts's webServer sets cwd: ".." to guarantee this.
	if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err != nil {
		return fmt.Errorf("cwd %s has no go.mod — run this from the verdi module root (playwright.config.ts sets cwd: \"..\")", moduleRoot)
	}

	scratch, err := os.MkdirTemp("", "verdi-e2e-*")
	if err != nil {
		return fmt.Errorf("creating scratch dir: %w", err)
	}
	defer os.RemoveAll(scratch)

	binPath := filepath.Join(scratch, "verdi")
	if err := buildBinary(moduleRoot, binPath); err != nil {
		return fmt.Errorf("building verdi binary: %w", err)
	}

	storeRoot := filepath.Join(scratch, "store")
	if err := provisionStore(moduleRoot, storeRoot); err != nil {
		return fmt.Errorf("provisioning scratch store: %w", err)
	}

	dexOut := filepath.Join(scratch, "dexsite")
	if err := dex.Build(context.Background(), dex.Options{Root: storeRoot, OutDir: dexOut}); err != nil {
		return fmt.Errorf("building dex site: %w", err)
	}

	dexSrv := &http.Server{Addr: dexAddr, Handler: http.FileServer(http.Dir(dexOut))}
	dexLn, err := net.Listen("tcp", dexAddr)
	if err != nil {
		return fmt.Errorf("binding dex server: %w", err)
	}
	go func() { _ = dexSrv.Serve(dexLn) }()
	log.Printf("e2eharness: dex site at http://%s (source: %s)", dexAddr, dexOut)

	serveCmd := exec.Command(binPath, "serve", "--http", workbenchAddr)
	serveCmd.Dir = storeRoot
	serveCmd.Stdout = os.Stdout
	serveCmd.Stderr = os.Stderr
	if err := serveCmd.Start(); err != nil {
		return fmt.Errorf("starting verdi serve: %w", err)
	}
	log.Printf("e2eharness: verdi serve (pid %d) at http://%s (store: %s)", serveCmd.Process.Pid, workbenchAddr, storeRoot)

	if err := waitHealthy("http://"+workbenchAddr+"/healthz", 20*time.Second); err != nil {
		_ = serveCmd.Process.Kill()
		return fmt.Errorf("waiting for verdi serve to become healthy: %w", err)
	}
	log.Println("e2eharness: ready")

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc
	log.Println("e2eharness: signal received, shutting down")

	_ = serveCmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { serveCmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = serveCmd.Process.Kill()
	}
	_ = dexSrv.Close()
	return nil
}

// buildBinary builds ./cmd/verdi from moduleRoot into out (build-then-exec,
// mirroring the Makefile's lint-store target — the e2e suite exercises the
// real binary, never `go run`).
func buildBinary(moduleRoot, out string) error {
	cmd := exec.Command("go", "build", "-o", out, "./cmd/verdi")
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// waitHealthy polls url until it returns 200 or timeout elapses.
func waitHealthy(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s: %w", url, lastErr)
}
