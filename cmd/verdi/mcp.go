// verdi mcp (05 §MCP server, I-13, PLAN.md Phase 9): the stdio<->socket
// shim. If the pointer file (.verdi/data/serve.path) names a live socket,
// byte-pipe stdin/stdout to it and exit on the FIRST EOF in either
// direction — the S4-binding finding: a naive "wait for both directions"
// shutdown hangs forever once serve dies, because closing the socket does
// not unblock a pending stdin Read. Otherwise (no serve running), acquire
// the writer lock and serve standalone directly on stdio — I-13's "the
// shim degenerates to a pipe" also means the standalone path speaks the
// exact same NDJSON framing serve does, so a client cannot tell the
// difference except by which process answers.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/store"
)

// cmdMcp is `verdi mcp`'s real entry point, invoked by dispatch.go.
func cmdMcp(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "mcp: unexpected argument(s) %v\n", args)
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "mcp:", err)
		return 2
	}

	if conn, ok := dialRunningServe(root); ok {
		defer conn.Close()
		proxyStdio(stdin, stdout, conn)
		return 0
	}

	return serveStandalone(root, stdin, stdout, stderr)
}

// dialRunningServe reads the pointer file and, only if it names a socket
// that actually accepts a connection right now, returns that connection.
// Any failure (no pointer file, a pointer to a dead/removed socket) is
// treated as "no serve running" rather than an error — the standalone
// fallback is exactly for this case (a fresh clone, or serve exited
// without a client noticing yet).
func dialRunningServe(root string) (net.Conn, bool) {
	sockPath, err := mcpserve.ReadPointerFile(root)
	if err != nil {
		return nil, false
	}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		return nil, false
	}
	return conn, true
}

// proxyStdio byte-pipes stdin/stdout to conn and returns as soon as
// EITHER direction ends — the S4-binding fix (PLAN.md Phase 9): it does
// NOT wait for both io.Copy goroutines to finish (the naive approach),
// because when serve dies, the stdin->socket goroutine stays blocked in
// Read(stdin) forever (an MCP client does not close stdin between calls),
// so waiting on it would hang the shim indefinitely even though it can
// never proxy anything again.
func proxyStdio(stdin io.Reader, stdout io.Writer, conn net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(conn, stdin)
		if uc, ok := conn.(*net.UnixConn); ok {
			_ = uc.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(stdout, conn)
		done <- struct{}{}
	}()
	<-done // exit on the FIRST direction to end; do not wait on the other
}

// serveStandalone acquires the writer lock and serves MCP directly on
// stdio — the fallback when no serve is reachable through the pointer
// file (I-13/05 §MCP server: "or acquires the writer lock and serves
// standalone when the workbench isn't up").
func serveStandalone(root string, stdin io.Reader, stdout, stderr io.Writer) int {
	dataDir := filepath.Join(root, ".verdi", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "mcp:", err)
		return 2
	}

	lockPath := filepath.Join(dataDir, "writer.lock")
	lockFile, err := mcpserve.AcquireLock(lockPath)
	if err != nil {
		var held *mcpserve.ErrLockHeld
		if errors.As(err, &held) {
			// Held by a live process we could not reach via the pointer
			// file (e.g. its own socket bind failed after taking the
			// lock, or the pointer file is stale) — an honest operational
			// error rather than silently starting a second writer.
			fmt.Fprintf(stderr, "mcp: %v, but its socket is unreachable — refusing to start a second writer (D3)\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "mcp:", err)
		return 2
	}
	defer func() { _ = mcpserve.ReleaseLock(lockFile, lockPath) }()

	srv := mcpserve.NewServer(root)
	// Best-effort (V1-P7): list_annotations' review-sticky mirrored
	// population (05 §MCP server) needs a live forge; nil is a fully
	// valid Backend.Forge zero value (review.go degrades to "no review
	// population" — every other tool is unaffected), so a missing/
	// unreachable forge is never an operational error for `verdi mcp`
	// itself. Mirrors gate_threads.go's identical tolerance. When a forge
	// is CONFIGURED but unreachable, list_annotations discloses rather than
	// silently omitting review population (I-1(b)).
	forgePort, configuredKind := forgeBestEffort(context.Background(), root)
	srv.Backend.Forge = forgePort
	if forgePort == nil && configuredKind != "" {
		srv.Backend.ReviewUnavailable = reviewUnavailableReason(configuredKind)
	}
	if err := mcpserve.ServeConn(context.Background(), stdin, stdout, srv); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintln(stderr, "mcp:", err)
		return 2
	}
	return 0
}
