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

	"github.com/OWNER/verdi/internal/mcpserve"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/workbench"
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

	httpLn, err := net.Listen("tcp", httpAddr)
	if err != nil {
		fmt.Fprintln(stderr, "serve: binding workbench HTTP:", err)
		return 2
	}
	httpSrv := &http.Server{Handler: workbench.NewHandler(root)}
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
	// Serve blocks until ln errors — the expected path is ln.Close() from
	// the signal handler above, a clean shutdown rather than a failure.
	_ = srv.Serve(context.Background(), ln)
	return 0
}
