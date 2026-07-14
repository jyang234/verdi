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

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
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
	defer func() { _ = os.RemoveAll(scratch) }()

	// Signal handling installs before build/provisioning touches the
	// scratch dir: an interrupt from here on cancels ctx, which every
	// exec/HTTP call below observes, instead of killing the process
	// outright and skipping the deferred RemoveAll above.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	binPath := filepath.Join(scratch, "verdi")
	if err := buildBinary(ctx, moduleRoot, binPath); err != nil {
		return fmt.Errorf("building verdi binary: %w", err)
	}

	storeRoot := filepath.Join(scratch, "store")
	if err := provisionStore(moduleRoot, storeRoot); err != nil {
		return fmt.Errorf("provisioning scratch store: %w", err)
	}

	// The pending-supersession fixture (e2e/tests/16-dex-v2.spec.ts): one
	// open MR against main carrying testdata/dexoverlay's candidate v2
	// spec for accepted-pending-build, served through internal/forge's
	// hermetic fake — no network (CLAUDE.md), same seam the Go tests use.
	supersessionForge, err := seedSupersessionForge(moduleRoot)
	if err != nil {
		return fmt.Errorf("seeding supersession forge: %w", err)
	}

	dexOut := filepath.Join(scratch, "dexsite")
	if err := dex.Build(ctx, dex.Options{Root: storeRoot, OutDir: dexOut, Forge: supersessionForge, DefaultBranch: "main"}); err != nil {
		return fmt.Errorf("building dex site: %w", err)
	}

	// The v1 board fixtures land on a design branch AFTER the dex build,
	// so the static site keeps reflecting main while `verdi serve`'s
	// working tree sits on the design branch (authoring mode's branch
	// state — 05 §Workbench "Two modes").
	feedPath, err := provisionBoard(scratch, storeRoot)
	if err != nil {
		return fmt.Errorf("provisioning v1 board fixtures: %w", err)
	}

	dexSrv := &http.Server{Addr: dexAddr, Handler: http.FileServer(http.Dir(dexOut))}
	dexLn, err := net.Listen("tcp", dexAddr)
	if err != nil {
		return fmt.Errorf("binding dex server: %w", err)
	}
	go func() { _ = dexSrv.Serve(dexLn) }()
	log.Printf("e2eharness: dex site at http://%s (source: %s)", dexAddr, dexOut)

	serveCmd := exec.CommandContext(ctx, binPath, "serve", "--http", workbenchAddr)
	// A graceful stop on interrupt — SIGTERM, then up to 5s before the
	// stdlib force-kills — rather than exec.CommandContext's default of
	// killing the subprocess the instant ctx is done.
	serveCmd.Cancel = func() error { return serveCmd.Process.Signal(syscall.SIGTERM) }
	serveCmd.WaitDelay = 5 * time.Second
	serveCmd.Dir = storeRoot
	// The hermetic review-mode feed (workbench.CommentFeed's canned-file
	// implementation): REVIEW_SPEC reads as under MR review, with the
	// three fixtures.ts comments — no network (CLAUDE.md).
	serveCmd.Env = append(os.Environ(), "VERDI_REVIEW_FEED="+feedPath)
	serveCmd.Stdout = os.Stdout
	serveCmd.Stderr = os.Stderr
	if err := serveCmd.Start(); err != nil {
		return fmt.Errorf("starting verdi serve: %w", err)
	}
	log.Printf("e2eharness: verdi serve (pid %d) at http://%s (store: %s)", serveCmd.Process.Pid, workbenchAddr, storeRoot)

	if err := waitHealthy(ctx, "http://"+workbenchAddr+"/healthz", 20*time.Second); err != nil {
		_ = serveCmd.Process.Kill()
		return fmt.Errorf("waiting for verdi serve to become healthy: %w", err)
	}
	log.Println("e2eharness: ready")

	<-ctx.Done()
	log.Println("e2eharness: signal received, shutting down")
	_ = serveCmd.Wait()
	_ = dexSrv.Close()
	return nil
}

// seedSupersessionForge builds the in-memory forge double the dex build
// reads open supersession MRs through: MR "mr-7" open against main, its
// source branch carrying the dexoverlay candidate at the conventional
// R4-I-14 path — the exact seeding internal/dex's own tests use.
func seedSupersessionForge(moduleRoot string) (*fake.Forge, error) {
	candidate, err := os.ReadFile(filepath.Join(moduleRoot, "testdata", "dexoverlay", "mr", "accepted-pending-build-v2.spec.md"))
	if err != nil {
		return nil, fmt.Errorf("reading MR candidate fixture: %w", err)
	}
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "mr-7", SourceBranch: "design/accepted-pending-build-v2"})
	f.SeedFile("design/accepted-pending-build-v2", ".verdi/specs/active/accepted-pending-build-v2/spec.md", candidate)
	return f, nil
}

// buildBinary builds ./cmd/verdi from moduleRoot into out (build-then-exec,
// mirroring the Makefile's lint-store target — the e2e suite exercises the
// real binary, never `go run`).
func buildBinary(ctx context.Context, moduleRoot, out string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./cmd/verdi")
	cmd.Dir = moduleRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// waitHealthy polls url until it returns 200, ctx is done, or timeout
// elapses. Each poll is bounded by its own client timeout rather than
// blocking indefinitely on a wedged connection.
func waitHealthy(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("building healthz request: %w", err)
		}
		resp, err := client.Do(req)
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
