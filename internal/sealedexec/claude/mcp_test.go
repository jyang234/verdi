package claude

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/mcpserve"
)

const (
	testProfileDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	testWorkspaceID   = "flight--0123456789ab"
)

var testCanonicalRequest = []byte("{\"schema\":\"verdi.context-execution-request/v1\",\"test\":\"alpha\"}\n")

func TestScopedMCPConfigLifecycle(t *testing.T) {
	envRoot := t.TempDir()
	sentinel := filepath.Join(envRoot, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	listener := listenScopedMCP(t)
	handler := &scopedHTTPTestHandler{}

	config, _, closeMCP, err := StartScopedMCP(context.Background(), listener, envRoot, testCanonicalRequest, testProfileDigest, testWorkspaceID, handler)
	if err != nil {
		t.Fatalf("StartScopedMCP: %v", err)
	}
	registerScopedMCPCleanup(t, closeMCP)

	requestDigest := testDigest(testCanonicalRequest)
	preimage := fmt.Sprintf(`{"profile_digest":%q,"request_digest":%q,"schema":"verdi.claude-mcp-capability/v1","workspace_id":%q}`, testProfileDigest, requestDigest, testWorkspaceID)
	wantToken := testDigest([]byte(preimage))
	wantAuthorization := "Bearer " + wantToken
	wantURL := "http://" + listener.Addr().String() + "/mcp"
	if config.Path != filepath.Join(envRoot, "claude-mcp.json") || config.URL != wantURL || config.Authorization != wantAuthorization {
		t.Fatalf("config = %#v, want path=%q url=%q authorization=%q", config, filepath.Join(envRoot, "claude-mcp.json"), wantURL, wantAuthorization)
	}
	wantConfig := fmt.Sprintf("{\"mcpServers\":{\"verdi-context\":{\"alwaysLoad\":true,\"headers\":{\"Authorization\":%q},\"type\":\"http\",\"url\":%q}}}\n", wantAuthorization, wantURL)
	configBytes, err := os.ReadFile(config.Path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(configBytes) != wantConfig {
		t.Fatalf("config bytes = %q, want %q", configBytes, wantConfig)
	}
	info, err := os.Stat(config.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %04o, want 0600", info.Mode().Perm())
	}
	entries, err := os.ReadDir(envRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("env root entries = %v, want only config and sentinel (no atomic temp debris)", entryNames(entries))
	}

	client := &http.Client{Timeout: 2 * time.Second}
	for _, request := range []struct {
		id        int
		method    string
		params    string
		wantToken string
	}{
		{id: 1, method: "initialize", wantToken: `"protocolVersion":"2024-11-05"`},
		{id: 2, method: "tools/list", wantToken: `"name":"get_flight_plan"`},
		{id: 3, method: "tools/call", params: `,"params":{"name":"get_flight_plan","arguments":{}}`, wantToken: `\"kind\":\"flight-plan\"`},
		{id: 4, method: "tools/call", params: `,"params":{"name":"request_context","arguments":{"purpose":"implementation","ref":"spec/extra"}}`, wantToken: `\"kind\":\"context-approved\"`},
	} {
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":%q%s}`, request.id, request.method, request.params)
		responseBody := callScopedMCP(t, client, config, body)
		if !strings.Contains(responseBody, request.wantToken) {
			t.Fatalf("%s response = %q, want token %q", request.method, responseBody, request.wantToken)
		}
	}
	if got, want := handler.Calls(), []string{"get_flight_plan", "request_context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tool calls = %v, want %v", got, want)
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := closeMCP(nil); err == nil {
		t.Fatal("close MCP accepted nil context")
	}
	if err := closeMCP(closeCtx); err != nil {
		t.Fatalf("close MCP: %v", err)
	}
	if err := closeMCP(closeCtx); err != nil {
		t.Fatalf("idempotent close MCP: %v", err)
	}
	if _, err := os.Stat(config.Path); !os.IsNotExist(err) {
		t.Fatalf("config remains after close: %v", err)
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep\n" {
		t.Fatalf("close changed sibling file: content=%q err=%v", content, err)
	}

	request, err := http.NewRequest(http.MethodPost, config.URL, strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"initialize"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", config.Authorization)
	request.Header.Set("Content-Type", "application/json")
	if response, err := client.Do(request); err == nil {
		_ = response.Body.Close()
		t.Fatal("HTTP MCP still accepted a connection after close")
	}
}

func TestScopedMCPCapabilityBindsAllInputs(t *testing.T) {
	tests := []struct {
		name      string
		request   []byte
		profile   string
		workspace string
	}{
		{name: "base", request: testCanonicalRequest, profile: testProfileDigest, workspace: testWorkspaceID},
		{name: "request digest mutation", request: []byte("{\"schema\":\"verdi.context-execution-request/v1\",\"test\":\"beta\"}\n"), profile: testProfileDigest, workspace: testWorkspaceID},
		{name: "profile digest mutation", request: testCanonicalRequest, profile: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", workspace: testWorkspaceID},
		{name: "workspace mutation", request: testCanonicalRequest, profile: testProfileDigest, workspace: "flight--abcdef012345"},
	}
	seen := make(map[string]string)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listener := listenScopedMCP(t)
			config, _, closeMCP, err := StartScopedMCP(context.Background(), listener, t.TempDir(), test.request, test.profile, test.workspace, &scopedHTTPTestHandler{})
			if err != nil {
				t.Fatalf("StartScopedMCP: %v", err)
			}
			registerScopedMCPCleanup(t, closeMCP)
			closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := closeMCP(closeCtx); err != nil {
				t.Fatalf("close MCP: %v", err)
			}
			if prior, exists := seen[config.Authorization]; exists {
				t.Fatalf("capability %q reused by %q and %q", config.Authorization, prior, test.name)
			}
			seen[config.Authorization] = test.name
		})
	}
}

func TestStartScopedMCPRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		ctx       context.Context
		listener  bool
		envRoot   func(*testing.T) string
		request   []byte
		profile   string
		workspace string
		handler   mcpserve.Handler
		cancelled bool
	}{
		{name: "nil context", ctx: nil, listener: true},
		{name: "cancelled context", ctx: context.Background(), listener: true, cancelled: true},
		{name: "nil listener", ctx: context.Background()},
		{name: "relative env root", ctx: context.Background(), listener: true, envRoot: func(*testing.T) string { return "relative" }},
		{name: "unclean env root", ctx: context.Background(), listener: true, envRoot: func(t *testing.T) string {
			root := t.TempDir()
			return root + string(os.PathSeparator) + ".." + string(os.PathSeparator) + filepath.Base(root)
		}},
		{name: "empty request", ctx: context.Background(), listener: true, request: []byte{}},
		{name: "request missing canonical LF", ctx: context.Background(), listener: true, request: []byte(`{"schema":"verdi.context-execution-request/v1","test":"alpha"}`)},
		{name: "request keys not canonical", ctx: context.Background(), listener: true, request: []byte("{\"test\":\"alpha\",\"schema\":\"verdi.context-execution-request/v1\"}\n")},
		{name: "invalid profile digest", ctx: context.Background(), listener: true, profile: "sha256:nope"},
		{name: "invalid workspace id", ctx: context.Background(), listener: true, workspace: "ambient"},
		{name: "nil handler", ctx: context.Background(), listener: true, handler: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := test.ctx
			if test.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			var listener net.Listener
			if test.listener {
				listener = listenScopedMCP(t)
				defer listener.Close()
			}
			envRoot := t.TempDir()
			if test.envRoot != nil {
				envRoot = test.envRoot(t)
			}
			request := testCanonicalRequest
			if test.request != nil {
				request = test.request
			}
			profile := testProfileDigest
			if test.profile != "" {
				profile = test.profile
			}
			workspace := testWorkspaceID
			if test.workspace != "" {
				workspace = test.workspace
			}
			handler := test.handler
			if test.name != "nil handler" {
				handler = &scopedHTTPTestHandler{}
			}
			if _, _, _, err := StartScopedMCP(ctx, listener, envRoot, request, profile, workspace, handler); err == nil {
				t.Fatal("StartScopedMCP accepted invalid input")
			}
			if _, err := os.Stat(filepath.Join(envRoot, "claude-mcp.json")); !os.IsNotExist(err) {
				t.Fatalf("failed start left config: %v", err)
			}
		})
	}
}

func TestScopedMCPProviderChildFDInventory(t *testing.T) {
	if os.Getenv("VERDI_TEST_MCP_FD_CHILD") == "1" {
		controllerPath := os.Getenv("VERDI_TEST_MCP_CONTROLLER_PATH")
		controllerFD, err := strconv.ParseUint(os.Getenv("VERDI_TEST_MCP_CONTROLLER_FD"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		controllerInfo, err := os.Stat(controllerPath)
		if err != nil {
			t.Fatal(err)
		}
		candidate := os.NewFile(uintptr(controllerFD), "inherited-controller-candidate")
		if candidate != nil {
			if info, statErr := candidate.Stat(); statErr == nil && os.SameFile(controllerInfo, info) {
				t.Fatalf("provider child inherited parent controller descriptor %d", controllerFD)
			}
		}
		fmt.Println("controller-descriptor-absent")
		return
	}

	controllerPath := filepath.Join(t.TempDir(), "controller")
	if err := os.WriteFile(controllerPath, []byte("controller-only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := os.Open(controllerPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := controller.Close(); err != nil {
			t.Errorf("close fake controller: %v", err)
		}
	})

	listener := listenScopedMCP(t)
	config, _, closeMCP, err := StartScopedMCP(context.Background(), listener, t.TempDir(), testCanonicalRequest, testProfileDigest, testWorkspaceID, &scopedHTTPTestHandler{})
	if err != nil {
		t.Fatal(err)
	}
	registerScopedMCPCleanup(t, closeMCP)
	command := exec.Command(os.Args[0], "-test.run=^TestScopedMCPProviderChildFDInventory$")
	command.Env = append(os.Environ(),
		"VERDI_TEST_MCP_FD_CHILD=1",
		"VERDI_TEST_MCP_CONTROLLER_PATH="+controllerPath,
		"VERDI_TEST_MCP_CONTROLLER_FD="+strconv.FormatUint(uint64(controller.Fd()), 10),
		"VERDI_TEST_MCP_CONFIG="+config.Path,
	)
	// Provider children receive the HTTP config path, never ExtraFiles.
	command.ExtraFiles = nil
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fake provider child: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "controller-descriptor-absent") {
		t.Fatalf("fake provider FD inventory = %q", output)
	}
	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := closeMCP(closeCtx); err != nil {
		t.Fatal(err)
	}
}

func TestScopedMCPDeliversHandlerTerminalToParent(t *testing.T) {
	for _, test := range []struct {
		name         string
		body         string
		wantExitCode int
	}{
		{
			name:         "tool result terminal",
			body:         `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{"purpose":"implementation","ref":"spec/extra"}}}`,
			wantExitCode: 1,
		},
		{
			name:         "unknown method",
			body:         `{"jsonrpc":"2.0","id":2,"method":"ambient/method"}`,
			wantExitCode: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener := listenScopedMCP(t)
			handler := &scopedHTTPTestHandler{terminal: &mcpserve.HandlerTerminal{ExitCode: 1}}
			config, terminals, closeMCP, err := StartScopedMCP(context.Background(), listener, t.TempDir(), testCanonicalRequest, testProfileDigest, testWorkspaceID, handler)
			if err != nil {
				t.Fatalf("StartScopedMCP: %v", err)
			}
			registerScopedMCPCleanup(t, closeMCP)
			select {
			case terminal := <-terminals:
				t.Fatalf("terminal %#v was observable before any scoped call", terminal)
			default:
			}

			client := &http.Client{Timeout: 5 * time.Second}
			if frame := callScopedMCP(t, client, config, test.body); frame == "" {
				t.Fatal("scoped call produced no response frame")
			}
			select {
			case terminal := <-terminals:
				if terminal == nil || terminal.ExitCode != test.wantExitCode {
					t.Fatalf("terminal = %#v, want exit %d", terminal, test.wantExitCode)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no terminal reached the parent after the response frame was written")
			}
		})
	}
}

type scopedHTTPTestHandler struct {
	mu       sync.Mutex
	calls    []string
	terminal *mcpserve.HandlerTerminal
}

func (h *scopedHTTPTestHandler) Tools() []mcpserve.HandlerTool {
	return []mcpserve.HandlerTool{
		{Name: "get_flight_plan", Description: "Inspect the current sealed flight plan.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`)},
		{Name: "request_context", Description: "Request one declared context expansion for approval.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{"purpose":{"minLength":1,"type":"string"},"ref":{"minLength":1,"type":"string"}},"required":["ref","purpose"],"type":"object"}`)},
	}
}

func (h *scopedHTTPTestHandler) Call(_ context.Context, name string, _ json.RawMessage) (mcpserve.HandlerCallResult, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.calls = append(h.calls, name)
	switch name {
	case "get_flight_plan":
		return mcpserve.HandlerCallResult{Text: `{"kind":"flight-plan"}`}, nil
	case "request_context":
		return mcpserve.HandlerCallResult{Text: `{"kind":"context-approved"}`, Terminal: h.terminal}, nil
	default:
		return mcpserve.HandlerCallResult{}, fmt.Errorf("unknown scoped tool %q", name)
	}
}

func (h *scopedHTTPTestHandler) Calls() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.calls...)
}

func listenScopedMCP(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	return listener
}

func registerScopedMCPCleanup(t *testing.T, closeMCP func(context.Context) error) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := closeMCP(ctx); err != nil {
			t.Errorf("cleanup scoped MCP: %v", err)
		}
	})
}

func callScopedMCP(t *testing.T, client *http.Client, config MCPConfig, body string) string {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, config.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", config.Authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HTTP MCP call: %v", err)
	}
	defer response.Body.Close()
	var raw json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		t.Fatalf("decode HTTP MCP response: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("HTTP MCP status = %d, body=%s", response.StatusCode, raw)
	}
	return string(raw)
}

func testDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}
