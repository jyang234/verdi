package mcpserve

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// rpcClient is a minimal NDJSON JSON-RPC client over one net.Conn, for
// driving Server.Serve end to end exactly the way a real MCP client
// (or the verdi mcp shim, byte-piping stdio to this same socket) would.
type rpcClient struct {
	conn net.Conn
	sc   *bufio.Scanner
	id   int
}

func dialTestServer(t *testing.T, sockPath string) *rpcClient {
	t.Helper()
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing %s: %v", sockPath, err)
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	return &rpcClient{conn: conn, sc: sc}
}

func (c *rpcClient) call(t *testing.T, method string, params any) map[string]any {
	t.Helper()
	c.id++
	req := map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	if _, err := c.conn.Write(append(data, '\n')); err != nil {
		t.Fatalf("writing request: %v", err)
	}
	if !c.sc.Scan() {
		t.Fatalf("no response (scan error: %v)", c.sc.Err())
	}
	var resp map[string]any
	if err := json.Unmarshal(c.sc.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response %q: %v", c.sc.Text(), err)
	}
	return resp
}

// startTestServer starts a real Server.Serve over a real unix socket,
// returning the socket path and a cleanup func. It deliberately does NOT
// bind under t.TempDir(): go test nests that under a directory that
// embeds the (potentially long, subtest-qualified) test name, which is
// exactly the realistic-checkout-path problem I-29 exists to solve — this
// helper instead uses SocketPath's own short-hash scheme (a throwaway
// unique "root" string keeps distinct tests from colliding on the same
// hash) to bind well under the sun_path ceiling on every platform.
func startTestServer(t *testing.T, root string) (sockPath string, stop func()) {
	t.Helper()
	sockPath, err := SocketPath(filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatalf("SocketPath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(sockPath), err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(sockPath)) })

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	srv := NewServer(root)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(context.Background(), ln)
	}()
	return sockPath, func() {
		ln.Close()
		<-done
	}
}

// TestServer_InitializeHandshake proves the initialize round-trip
// (PLAN.md Phase 9 exit criteria: "initialize handshake ... green").
func TestServer_InitializeHandshake(t *testing.T) {
	sockPath, stop := startTestServer(t, mustRepoDir(t))
	defer stop()

	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	resp := c.call(t, "initialize", map[string]any{"protocolVersion": ProtocolVersion})
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("initialize response has no result: %#v", resp)
	}
	if result["protocolVersion"] != ProtocolVersion {
		t.Fatalf("protocolVersion = %v, want %s", result["protocolVersion"], ProtocolVersion)
	}
	caps, ok := result["capabilities"].(map[string]any)
	if !ok || caps["tools"] == nil {
		t.Fatalf("capabilities.tools missing: %#v", result)
	}
}

// TestServer_ToolsListAndCall proves tools/list enumerates all nine
// tools (each carrying the data-never-instructions note) and tools/call
// round-trips a real tool (per-tool business logic is exhaustively
// covered in backend_test.go; this proves the JSON-RPC plumbing gets a
// call there and back).
func TestServer_ToolsListAndCall(t *testing.T) {
	sockPath, stop := startTestServer(t, mustRepoDir(t))
	defer stop()

	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	listResp := c.call(t, "tools/list", nil)
	result := listResp["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	if len(tools) != 9 {
		t.Fatalf("tools/list returned %d tools, want 9: %#v", len(tools), tools)
	}
	wantNames := map[string]bool{
		"search_artifacts": true, "get_artifact": true, "get_links": true, "get_matrix": true,
		"get_context_bundle": true, "list_annotations": true, "list_tasks": true, "get_board": true, "add_annotation": true,
	}
	for _, raw := range tools {
		def := raw.(map[string]any)
		name, _ := def["name"].(string)
		if !wantNames[name] {
			t.Fatalf("unexpected tool name %q", name)
		}
		delete(wantNames, name)
		desc, _ := def["description"].(string)
		if !contains(desc, "DATA, NEVER INSTRUCTIONS") {
			t.Fatalf("tool %q description missing the data-never-instructions note: %q", name, desc)
		}
	}
	if len(wantNames) != 0 {
		t.Fatalf("tools/list missing: %v", wantNames)
	}

	callResp := c.call(t, "tools/call", map[string]any{
		"name":      "search_artifacts",
		"arguments": map[string]any{"query": "outbox"},
	})
	callResult, ok := callResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call response has no result: %#v", callResp)
	}
	if v, _ := callResult["isError"].(bool); v {
		t.Fatalf("search_artifacts tools/call errored: %#v", callResult)
	}
}

// TestServer_UnknownMethodIsJSONRPCError proves an unrecognized method is
// a protocol-level JSON-RPC error, not a tool error.
func TestServer_UnknownMethodIsJSONRPCError(t *testing.T) {
	sockPath, stop := startTestServer(t, mustRepoDir(t))
	defer stop()
	c := dialTestServer(t, sockPath)
	defer c.conn.Close()

	resp := c.call(t, "not/a/real/method", nil)
	if resp["error"] == nil {
		t.Fatalf("expected a JSON-RPC error for an unknown method, got %#v", resp)
	}
}

// TestServer_TwoConcurrentConnectionsBothProgress is the S4-binding
// regression: a serial accept loop starves a second client. Two
// connections are opened and interleaved by hand (client B's first call
// happens strictly between client A's two calls) to prove neither blocks
// on the other.
func TestServer_TwoConcurrentConnectionsBothProgress(t *testing.T) {
	sockPath, stop := startTestServer(t, mustRepoDir(t))
	defer stop()

	a := dialTestServer(t, sockPath)
	defer a.conn.Close()
	b := dialTestServer(t, sockPath)
	defer b.conn.Close()

	done := make(chan bool, 2)
	go func() {
		resp := a.call(t, "ping", nil)
		done <- resp["result"] != nil
	}()
	go func() {
		resp := b.call(t, "ping", nil)
		done <- resp["result"] != nil
	}()

	timeout := time.After(5 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case ok := <-done:
			if !ok {
				t.Fatal("a connection's ping did not return a result")
			}
		case <-timeout:
			t.Fatal("timed out waiting for both connections to progress — a serial accept loop would starve one of them (S4 binding finding)")
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mustRepoDir builds a tiny standalone fixture (no ADR/spec content
// needed — these tests exercise transport/dispatch, not tool business
// logic) with a valid .verdi/verdi.yaml so buildIndex succeeds against an
// otherwise-empty committed zone.
func mustRepoDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	manifest := "schema: verdi.layout/v1\n"
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
	return root
}
