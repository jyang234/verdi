// Real two-process integration tests for `verdi serve` + `verdi mcp`
// (PLAN.md Phase 9 exit criteria): builds the actual verdi binary and
// exercises it as real OS processes — never a mocked transport — proving
// D3's single-writer guarantee, I-12's lock takeover after SIGKILL, and
// the S4-binding shim-shutdown fix end to end.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/mcpserve"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// buildVerdiBinary builds the real verdi binary once per test run
// (shared across every test in this file via sync.Once) and returns its
// path.
func buildVerdiBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		// t.TempDir() is per-test and would be removed at test cleanup,
		// which would delete the shared binary out from under later
		// tests in this file — build into a fresh, unmanaged temp dir
		// instead (shared for the whole test binary's run).
		binDir, err := os.MkdirTemp("", "verdi-bin")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(binDir, "verdi")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("building verdi binary: %w\n%s", err, out.String())
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("buildVerdiBinary: %v", buildErr)
	}
	return builtBin
}

// newIntegrationStoreRoot builds a minimal, real store root (a real git
// checkout via internal/fixturegit — no golden SHAs pinned or asserted;
// this test only needs A store root that store.FindRoot accepts).
func newIntegrationStoreRoot(t *testing.T) string {
	t.Helper()
	manifest := "schema: verdi.layout/v1\n"
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": manifest, ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	return repo.Dir
}

// waitForPointerFile polls for root's .verdi/data/serve.path to appear
// and be readable, returning the socket path it names. Fails the test if
// it doesn't appear within timeout.
func waitForPointerFile(t *testing.T, root string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sockPath, err := mcpserve.ReadPointerFile(root); err == nil {
			return sockPath
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pointer file at %s/.verdi/data/serve.path did not appear within %s", root, timeout)
	return ""
}

// readLockInfo reads and decodes root's writer.lock.
func readLockInfo(t *testing.T, root string) mcpserve.LockInfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "writer.lock"))
	if err != nil {
		t.Fatalf("reading writer.lock: %v", err)
	}
	var info mcpserve.LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decoding writer.lock: %v", err)
	}
	return info
}

// waitForLockPID polls until root's writer.lock names wantPID, or fails
// the test after timeout. Used instead of waitForPointerFile when a PRIOR
// holder crashed without cleanup: the pointer file's socket path is a
// deterministic function of the checkout root (I-29), so a stale pointer
// file left behind by a crashed holder is indistinguishable, by content
// alone, from a fresh one the new holder just wrote — polling the lock's
// pid is what actually proves a specific process is the current writer.
func waitForLockPID(t *testing.T, root string, wantPID int, timeout time.Duration) mcpserve.LockInfo {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last mcpserve.LockInfo
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "writer.lock"))
		if err == nil {
			var info mcpserve.LockInfo
			if json.Unmarshal(data, &info) == nil {
				last = info
				if info.PID == wantPID {
					return info
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("writer.lock never showed pid %d within %s (last seen: %+v)", wantPID, timeout, last)
	return mcpserve.LockInfo{}
}

// ndjsonRPC sends one JSON-RPC request over w and reads/decodes one
// response line from sc — the same wire shape internal/mcpserve.wire.go
// speaks, driven here from the OUTSIDE as a real client would.
func ndjsonRPC(t *testing.T, w io.Writer, sc *bufio.Scanner, id int, method string, params any) map[string]any {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		t.Fatalf("writing request: %v", err)
	}
	if !sc.Scan() {
		t.Fatalf("no response to %s (scan error: %v)", method, sc.Err())
	}
	var resp map[string]any
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response %q: %v", sc.Text(), err)
	}
	return resp
}

// TestD3_ConcurrentSecondProcessRoutesThroughSocket is PLAN.md Phase 9's
// exit criterion: "a concurrent second process routes through the socket
// (no second writer — D3 integration test)". Process A (`verdi serve`) is
// the one writer; process B (`verdi mcp`, the shim) is a second, fully
// independent OS process that answers a real MCP handshake by proxying
// through A's socket; a THIRD attempted `verdi serve` (process C),
// started while A is still the writer, is proven to fail rather than
// silently becoming a second writer.
func TestD3_ConcurrentSecondProcessRoutesThroughSocket(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	// Process A: the writer.
	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serveCmd.Dir = root
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	t.Cleanup(func() {
		_ = serveCmd.Process.Signal(syscall.SIGTERM)
		_ = serveCmd.Wait()
	})
	waitForPointerFile(t, root, 10*time.Second)

	// Process C: a second `verdi serve` attempted while A holds the lock
	// — must fail (D3's "one writer" guarantee), not silently start a
	// second writer.
	secondServe := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	secondServe.Dir = root
	var secondErrOut bytes.Buffer
	secondServe.Stderr = &secondErrOut
	if err := secondServe.Run(); err == nil {
		t.Fatal("a second `verdi serve` while the first holds the writer lock succeeded — D3's single-writer guarantee is violated")
	}
	if secondErrOut.Len() == 0 {
		t.Fatal("a second `verdi serve` failed silently with no explanation on stderr")
	}

	// Process B: `verdi mcp`, a second, independent OS process — proxies
	// through process A's socket rather than becoming a writer itself.
	mcpCmd := exec.Command(bin, "mcp")
	mcpCmd.Dir = root
	stdin, err := mcpCmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := mcpCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("starting verdi mcp: %v", err)
	}

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	resp := ndjsonRPC(t, stdin, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("verdi mcp initialize: no result in response: %#v", resp)
	}
	if result["protocolVersion"] != mcpserve.ProtocolVersion {
		t.Fatalf("verdi mcp initialize: protocolVersion = %v, want %s", result["protocolVersion"], mcpserve.ProtocolVersion)
	}

	// tools/list through the shim too — a second full round-trip proving
	// this is a real, working proxy, not a one-shot fluke.
	toolsResp := ndjsonRPC(t, stdin, sc, 2, "tools/list", nil)
	toolsResult, ok := toolsResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("verdi mcp tools/list: no result: %#v", toolsResp)
	}
	tools, _ := toolsResult["tools"].([]any)
	if len(tools) != 8 {
		t.Fatalf("verdi mcp tools/list returned %d tools through the socket, want 8", len(tools))
	}

	// Clean up process B: closing stdin signals EOF on the stdin->socket
	// direction, which the shim's fixed shutdown (I-13/S4) exits on.
	stdin.Close()
	if err := mcpCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 0 {
			t.Fatalf("verdi mcp exited abnormally after stdin close: %v", err)
		}
	}
}

// TestLockTakeover_AfterSIGKILL is PLAN.md Phase 9's exit criterion:
// "lock takeover after SIGKILL of the holder". Process A is SIGKILLed
// (no clean shutdown, no lock/socket cleanup — exactly a crash);
// process B, started against the same root afterward, must take over the
// lock (I-12) and become the new writer.
func TestLockTakeover_AfterSIGKILL(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	a := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	a.Dir = root
	if err := a.Start(); err != nil {
		t.Fatalf("starting verdi serve (A): %v", err)
	}
	waitForPointerFile(t, root, 10*time.Second)
	infoA := readLockInfo(t, root)
	if infoA.PID != a.Process.Pid {
		t.Fatalf("lock pid = %d, want A's pid %d", infoA.PID, a.Process.Pid)
	}

	if err := a.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL A: %v", err)
	}
	_ = a.Wait() // reap; ignore the (expected) killed-signal error

	b := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	b.Dir = root
	if err := b.Start(); err != nil {
		t.Fatalf("starting verdi serve (B): %v", err)
	}
	t.Cleanup(func() {
		_ = b.Process.Signal(syscall.SIGTERM)
		_ = b.Wait()
	})
	infoB := waitForLockPID(t, root, b.Process.Pid, 10*time.Second)
	waitForPointerFile(t, root, 10*time.Second) // B's own socket is up too
	if infoB.PID == infoA.PID {
		t.Fatalf("B somehow reused A's exact pid %d — test is not meaningfully distinguishing the two holders", infoA.PID)
	}
}

// TestShim_ExitsPromptlyWhenServeDies is the S4-binding regression test
// PLAN.md Phase 9 calls for: "shim exits promptly when serve dies (S4's
// hang case, now a regression test)". The shim's stdin is deliberately
// held open (never closed by this test) — an MCP client does not close
// stdin between calls — so the ONLY way the shim can exit is via the
// socket->stdout direction ending when serve dies; a naive
// wait-for-both-directions shutdown would hang here forever.
func TestShim_ExitsPromptlyWhenServeDies(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serveCmd.Dir = root
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	waitForPointerFile(t, root, 10*time.Second)

	mcpCmd := exec.Command(bin, "mcp")
	mcpCmd.Dir = root
	stdin, err := mcpCmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := mcpCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("starting verdi mcp: %v", err)
	}
	// stdin is intentionally never closed by this test — see doc comment.
	defer stdin.Close()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	resp := ndjsonRPC(t, stdin, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	if resp["result"] == nil {
		t.Fatalf("verdi mcp initialize before killing serve: no result: %#v", resp)
	}

	// Kill serve out from under the shim — no clean shutdown, exactly
	// like a crash.
	if err := serveCmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL serve: %v", err)
	}
	_ = serveCmd.Wait()

	waitDone := make(chan error, 1)
	go func() { waitDone <- mcpCmd.Wait() }()

	select {
	case <-waitDone:
		// Exited promptly — the fix. (Exit code is not asserted: on some
		// platforms a half-closed socket read surfaces as a nonzero-but-
		// clean-shutdown exit; promptness is the property under test.)
	case <-time.After(5 * time.Second):
		_ = mcpCmd.Process.Kill() // don't leak the hung process even though the test already failed
		t.Fatal("verdi mcp did not exit within 5s of serve being killed — this is the S4 hang case: a naive wait-for-both-directions shutdown blocks forever on the still-open stdin Read")
	}
}
