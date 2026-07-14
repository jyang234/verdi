// verdi serve (05 §MCP server, 01 §D3, PLAN.md Phase 9): the single
// writer process for a checkout. Acquires the writer lock (I-12), hosts
// the MCP endpoint on the checkout's unix socket (I-29's short-path
// scheme, pointer file at .verdi/data/serve.path), and hosts the
// localhost-only workbench HTTP skeleton (internal/workbench) alongside
// it. Runs until SIGINT/SIGTERM, then releases the lock and removes the
// socket cleanly — a crash instead leaves both behind, which I-12's
// takeover and I-29's pointer file are both designed to tolerate.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// defaultWorkbenchAddr is the workbench HTTP listener's default bind
// address — loopback only (05 §Workbench: "binds localhost only").
const defaultWorkbenchAddr = "127.0.0.1:4173"

// cmdServe is `verdi serve`'s real entry point, invoked by dispatch.go.
func cmdServe(args []string, stdout, stderr io.Writer) int {
	httpAddr := defaultWorkbenchAddr
	for i := 0; i < len(args); i++ {
		if args[i] == "--http" && i+1 < len(args) {
			httpAddr = args[i+1]
			i++
			continue
		}
		fmt.Fprintf(stderr, "serve: unknown argument %q\n", args[i])
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	dataDir := filepath.Join(root, ".verdi", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	lockPath := filepath.Join(dataDir, "writer.lock")
	lockFile, err := mcpserve.AcquireLock(lockPath)
	if err != nil {
		var held *mcpserve.ErrLockHeld
		if errors.As(err, &held) {
			fmt.Fprintf(stderr, "serve: %v — another verdi serve is already the writer for this checkout; use `verdi mcp` to reach it\n", err)
		} else {
			fmt.Fprintln(stderr, "serve:", err)
		}
		return 2
	}
	defer func() { _ = mcpserve.ReleaseLock(lockFile, lockPath) }()

	sockPath, err := mcpserve.SocketPath(root)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o755); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}
	_ = os.Remove(sockPath) // best-effort: a prior crash may have left a stale socket inode
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintln(stderr, "serve: binding MCP socket:", err)
		return 2
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
	}()

	if err := mcpserve.WritePointerFile(root, sockPath); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	// Review mode's comment feed (05 §Review stickies). The real source is
	// the forge adapter over workbench.CommentFeed (reviewfeed.go), built
	// from the same best-effort forge that feeds mcpserve's review-sticky
	// mirror below — a single forge construction, reused. When no real
	// forge is configured/reachable (the hermetic e2e harness, any offline
	// checkout) it falls back to the canned-file feed the harness injects
	// (VERDI_REVIEW_FEED — a strict-decoded local JSON file, no network).
	// Real forge config takes precedence when both are present; with
	// neither, no spec is ever under review and the board keys purely off
	// branch state.
	forgePort, configuredKind := forgeBestEffort(context.Background(), root)
	deps := workbench.Deps{}
	switch {
	case forgePort != nil:
		deps.CommentFeed = newForgeCommentFeed(forgePort, root)
	case os.Getenv("VERDI_REVIEW_FEED") != "":
		feed, ferr := workbench.LoadCannedCommentFeed(os.Getenv("VERDI_REVIEW_FEED"))
		if ferr != nil {
			fmt.Fprintln(stderr, "serve:", ferr)
			return 2
		}
		deps.CommentFeed = feed
	case configuredKind != "":
		// A forge is named in verdi.yaml but no live adapter could be built
		// (no credentials): disclose on the board rather than render as
		// silently not-under-review (I-1(b)).
		deps.ReviewUnavailable = reviewUnavailableReason(configuredKind)
	}
	if forgePort == nil && configuredKind != "" {
		// The /disclosures page's process-context input
		// (spec/disclosures-panel ac-1): the same structured seam value
		// behind reviewUnavailableReason, under the same condition
		// mcpserve's ReviewUnavailable uses below — deliberately NOT the
		// board switch's narrower case, because a canned harness feed
		// (VERDI_REVIEW_FEED) substituting for review comments does not
		// make the live forge any more reachable; the checkout's
		// disclosed context holds either way.
		deps.Disclosures = append(deps.Disclosures, reviewUnavailableDisclosure(configuredKind))
	}

	httpLn, err := net.Listen("tcp", httpAddr)
	if err != nil {
		fmt.Fprintln(stderr, "serve: binding workbench HTTP:", err)
		return 2
	}
	httpSrv := &http.Server{Handler: workbench.NewHandlerWith(root, deps)}
	go func() {
		_ = httpSrv.Serve(httpLn)
	}()
	defer httpSrv.Close()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigc
		fmt.Fprintf(stdout, "serve: signal %v received, shutting down\n", s)
		httpSrv.Close()
		ln.Close()
	}()

	fmt.Fprintf(stdout, "serve: MCP socket at %s (pointer: %s)\n", sockPath, filepath.Join(root, ".verdi", "data", "serve.path"))
	fmt.Fprintf(stdout, "serve: workbench at http://%s\n", httpLn.Addr())

	srv := mcpserve.NewServer(root)
	srv.ErrLog = os.Stderr // spec/fail-loud dc-3: a dropped socket connection leaves a trace, matching mcp.go's stdio scrutiny
	// Best-effort (V1-P7): see mcp.go's identical comment — a
	// missing/unreachable forge never blocks `verdi serve` from starting;
	// list_annotations' review-sticky mirrored population (05 §MCP
	// server) just degrades to "no review population" (Backend.Forge nil
	// is a fully valid zero value). Same instance the workbench comment
	// feed above uses — one construction per serve.
	srv.Backend.Forge = forgePort
	if forgePort == nil && configuredKind != "" {
		// Same disclosed-unavailable state on the machine read surface:
		// list_annotations returns a disclosure field rather than silently
		// omitting review population (I-1(b)).
		srv.Backend.ReviewUnavailable = reviewUnavailableReason(configuredKind)
	}
	// Serve blocks until ln errors — the expected path is ln.Close() from
	// the signal handler above, a clean shutdown rather than a failure.
	_ = srv.Serve(context.Background(), ln)
	return 0
}
