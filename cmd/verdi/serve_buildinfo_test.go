// Regression test for spec/uat-round-1 ac-1's `verdi serve` half: serve
// must print the SAME build identification line internal/buildinfo.Line
// produces (also `verdi version`/`--version`, version_test.go; also,
// separately, the workbench footer — a Fable lane) once at startup, on
// the stream it already logs to (stdout) — so a result from a running
// serve process can always be traced back to the exact build that
// produced it (closes UAT-003).
//
// A real subprocess is required here, not cmdServeWithDeps's own unit
// tests (serve_integration_test.go): those inject a fake serveRunner
// specifically to avoid ever binding a real socket, so they never reach
// the real runServe function this line lives in. This mirrors
// TestD3_ConcurrentSecondProcessRoutesThroughSocket's exact
// build/start/cleanup idiom (same file) and reuses its
// newIntegrationStoreRoot/syncBuffer helpers (align_test.go) rather than
// duplicating them.
package main

import (
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestServe_PrintsBuildIdentificationLineAtStartup(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serveCmd.Dir = root
	var out syncBuffer
	serveCmd.Stdout = &out
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	t.Cleanup(func() {
		_ = serveCmd.Process.Signal(syscall.SIGTERM)
		_ = serveCmd.Wait()
	})

	// The identification line is the very first thing runServe ever
	// writes to stdout (serve.go), before the MCP-socket/workbench lines
	// — so "stdout has anything at all yet" is exactly "the
	// identification line has been written", no pointer-file/socket
	// synchronization needed.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && out.String() == "" {
		time.Sleep(20 * time.Millisecond)
	}

	got := out.String()
	if got == "" {
		t.Fatalf("verdi serve produced no stdout within the deadline")
	}
	firstLine, _, _ := strings.Cut(got, "\n")
	if !strings.HasPrefix(firstLine, "verdi ") {
		t.Fatalf("serve's first stdout line = %q, want it to start with %q (internal/buildinfo.Line())", firstLine, "verdi ")
	}
}
