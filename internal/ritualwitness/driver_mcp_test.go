package ritualwitness

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/mcpserve"
)

// mcpFake is a stand-in MCP server: it reads one request line and records
// it and the root it serves. With an answer it writes that response line,
// reads until its connection ends, and returns after; with none it ends at
// once, returning after, without answering.
type mcpFake struct {
	answer string
	after  error

	mu      sync.Mutex
	request string
	root    string
}

func (f *mcpFake) serve(_ context.Context, root string, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	f.mu.Lock()
	f.request, f.root = line, root
	f.mu.Unlock()
	if err != nil {
		return err
	}
	if f.answer == "" {
		return f.after
	}
	if _, err := io.WriteString(w, f.answer+"\n"); err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, br)
	return f.after
}

func (f *mcpFake) seen() (request, root string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.request, f.root
}

// mcpResult is a tools/call response line answering request id 1.
func mcpResult(result string) string {
	return `{"jsonrpc":"2.0","id":1,"result":` + result + `}`
}

// TestMCP_Run maps every answer an MCP server can give to one tools/call
// (ledger SI-341 (5)): a tool result whose isError is false is exit 0; a
// result whose isError is true, or a JSON-RPC error, is 2 with the
// server's words in the error; and a transport failure or a response that
// is neither is -1, no verb's exit. The command log is always unavailable.
func TestMCP_Run(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	tests := []struct {
		name     string
		fake     *mcpFake
		args     json.RawMessage
		wantExit int
		wantErr  []string
	}{
		{"a result is a clean run", &mcpFake{answer: mcpResult(`{"content":[{"type":"text","text":"ok"}],"isError":false}`)}, json.RawMessage(`{"query":"x"}`), 0, nil},
		{"a result without isError is a clean run", &mcpFake{answer: mcpResult(`{"content":[{"type":"text","text":"ok"}]}`)}, nil, 0, nil},
		{"an empty result content is a clean run", &mcpFake{answer: mcpResult(`{"content":[]}`)}, nil, 0, nil},
		{"a tool error is exit 2 naming its words", &mcpFake{answer: mcpResult(`{"content":[{"type":"text","text":"import_apply: refused: the preview digest is stale"}],"isError":true}`)}, nil, 2,
			[]string{"probe_tool", "answered isError", "the preview digest is stale"}},
		{"a JSON-RPC error is exit 2 naming its code and message", &mcpFake{answer: `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found: tools/call"}}`}, nil, 2,
			[]string{"JSON-RPC error -32601", "method not found"}},
		{"a JSON-RPC error with a null id is exit 2", &mcpFake{answer: `{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"parse error","data":"x"}}`}, nil, 2,
			[]string{"JSON-RPC error -32700", "parse error"}},
		{"a line that is not JSON is malformed", &mcpFake{answer: `not json`}, nil, -1, []string{"malformed response"}},
		{"an unknown envelope field is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":1,"result":{"content":[]},"extra":1}`}, nil, -1, []string{"malformed response"}},
		{"an unknown result field is malformed", &mcpFake{answer: mcpResult(`{"content":[],"structuredContent":{}}`)}, nil, -1, []string{"malformed response"}},
		{"a non-text content item is malformed", &mcpFake{answer: mcpResult(`{"content":[{"type":"image","data":"AA==","mimeType":"image/png"}]}`)}, nil, -1, []string{"malformed response"}},
		{"a result without content is malformed", &mcpFake{answer: mcpResult(`{"isError":true}`)}, nil, -1, []string{"malformed response", "content"}},
		{"a non-boolean isError is malformed", &mcpFake{answer: mcpResult(`{"content":[],"isError":"true"}`)}, nil, -1, []string{"malformed response"}},
		{"a null result is malformed", &mcpFake{answer: mcpResult(`null`)}, nil, -1, []string{"malformed response"}},
		{"another request's id is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":2,"result":{"content":[]}}`}, nil, -1, []string{"malformed response", "id"}},
		{"a string id is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":"1","result":{"content":[]}}`}, nil, -1, []string{"malformed response", "id"}},
		{"a missing id is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","result":{"content":[]}}`}, nil, -1, []string{"malformed response", "id"}},
		{"a null id on a result is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":null,"result":{"content":[]}}`}, nil, -1, []string{"malformed response", "id"}},
		{"another protocol version is malformed", &mcpFake{answer: `{"jsonrpc":"1.0","id":1,"result":{"content":[]}}`}, nil, -1, []string{"malformed response", "jsonrpc"}},
		{"both a result and an error is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":1,"result":{"content":[]},"error":{"code":1,"message":"x"}}`}, nil, -1, []string{"malformed response"}},
		{"neither a result nor an error is malformed", &mcpFake{answer: `{"jsonrpc":"2.0","id":1}`}, nil, -1, []string{"malformed response"}},
		{"trailing data after the response is malformed", &mcpFake{answer: mcpResult(`{"content":[]}`) + ` {}`}, nil, -1, []string{"malformed response"}},
		{"a server that ends without answering is a transport failure", &mcpFake{}, nil, -1, []string{"transport"}},
		{"a server that fails after answering is a transport failure", &mcpFake{answer: mcpResult(`{"content":[]}`), after: errors.New("connection reset")}, nil, -1, []string{"transport", "connection reset"}},
		{"arguments that are not JSON are no verb's exit", &mcpFake{answer: mcpResult(`{"content":[]}`)}, json.RawMessage(`{`), -1, []string{"arguments"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := MCP{Serve: tt.fake.serve, Tool: "probe_tool", Arguments: tt.args}
			exit, log, err := d.Run(boundedContext(t, ctx), dir)
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d (err %v)", exit, tt.wantExit, err)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable: the MCP server supplies no command log", log)
			}
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("err = %v, want an error naming %q", err, tt.wantErr)
			}
			for _, w := range tt.wantErr {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %q, want it to name %q", err, w)
				}
			}
		})
	}
}

// TestMCP_RunSendsOneToolCall: the server is given the fixture directory
// as its root, and reads exactly one JSON-RPC tools/call request naming
// the tool and its arguments.
func TestMCP_RunSendsOneToolCall(t *testing.T) {
	dir := t.TempDir()
	for _, tt := range []struct {
		name string
		args json.RawMessage
		want string
	}{
		{"with arguments", json.RawMessage(`{"query":"x"}`), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_artifacts","arguments":{"query":"x"}}}` + "\n"},
		{"without arguments", nil, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search_artifacts"}}` + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &mcpFake{answer: mcpResult(`{"content":[]}`)}
			if exit, _, err := (MCP{Serve: fake.serve, Tool: "search_artifacts", Arguments: tt.args}).Run(boundedContext(t, context.Background()), dir); exit != 0 || err != nil {
				t.Fatalf("Run = (%d, %v), want a clean run", exit, err)
			}
			request, root := fake.seen()
			if request != tt.want {
				t.Errorf("request = %q, want %q", request, tt.want)
			}
			if root != dir {
				t.Errorf("served root = %q, want the fixture %q", root, dir)
			}
		})
	}
}

// TestMCP_RunRefusesAMissingServerOrTool: a driver with no server or no
// tool runs nothing and reports no verb's exit.
func TestMCP_RunRefusesAMissingServerOrTool(t *testing.T) {
	fake := &mcpFake{answer: mcpResult(`{"content":[]}`)}
	for _, tt := range []struct {
		name string
		d    MCP
		want string
	}{
		{"no server", MCP{Tool: "t"}, "no server"},
		{"no tool", MCP{Serve: fake.serve}, "no tool"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exit, log, err := tt.d.Run(context.Background(), t.TempDir())
			if exit != -1 || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Run = (%d, %v), want -1 naming %q", exit, err, tt.want)
			}
			if log.OK || log.Calls != nil {
				t.Fatalf("log = %+v, want unavailable", log)
			}
		})
	}
	if request, _ := fake.seen(); request != "" {
		t.Fatalf("a refused driver reached the server with %q", request)
	}
}

// TestMCP_RunIsBoundedByTheContext: a server that never answers is
// abandoned when the caller's context ends, as no verb's exit, and its
// connection is closed so it can return.
func TestMCP_RunIsBoundedByTheContext(t *testing.T) {
	returned := make(chan error, 1)
	hang := func(_ context.Context, _ string, r io.Reader, _ io.Writer) error {
		_, err := io.Copy(io.Discard, r)
		returned <- err
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	exit, _, err := MCP{Serve: hang, Tool: "t"}.Run(ctx, t.TempDir())
	if exit != -1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run = (%d, %v), want -1 wrapping the context's deadline", exit, err)
	}
	if elapsed := time.Since(start); elapsed > helperBound {
		t.Fatalf("Run returned after %s, past its context", elapsed)
	}
	select {
	case <-returned:
	case <-time.After(helperBound):
		t.Fatal("the abandoned server's connection was never closed")
	}
}

// verdiMCP serves one connection with the repository's own MCP server
// over root, as `verdi mcp` does (story dc-1).
func verdiMCP(ctx context.Context, root string, r io.Reader, w io.Writer) error {
	return mcpserve.ServeConn(ctx, r, w, mcpserve.NewServer(root))
}

// TestMCP_RunsTheRepositoryServer drives the real in-process MCP server
// over a fixture: a tool that answers is a clean run, a tool error is exit
// 2 naming the server's words, and through RunOn the log is unavailable,
// so the run is unproven, never a pass inferred from an absent log.
func TestMCP_RunsTheRepositoryServer(t *testing.T) {
	ctx := context.Background()
	fx := BuildWith(t, ctx, SeedClean, map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"})
	for _, tt := range []struct {
		name     string
		tool     string
		args     json.RawMessage
		wantExit int
		wantErr  string
	}{
		{"a tool that answers", "search_artifacts", json.RawMessage(`{"query":"anything"}`), 0, ""},
		{"an unknown tool", "no_such_tool", json.RawMessage(`{}`), 2, `unknown tool: "no_such_tool"`},
		{"malformed arguments", "search_artifacts", json.RawMessage(`{"nope":1}`), 2, "answered isError"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exit, _, err := MCP{Serve: verdiMCP, Tool: tt.tool, Arguments: tt.args}.Run(boundedContext(t, ctx), fx.Dir)
			if exit != tt.wantExit {
				t.Fatalf("exit = %d, want %d (err %v)", exit, tt.wantExit, err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("err = %v, want it to name %q", err, tt.wantErr)
			}
		})
	}

	res := RunOn(t, boundedContext(t, ctx), fx, MCP{Serve: verdiMCP, Tool: "search_artifacts", Arguments: json.RawMessage(`{"query":"anything"}`)}, branchDecl())
	want := []Verdict{
		v("command_log", Unattributable, "the driver supplied no git command log"),
		v("index_carry", Within, "declares no_commit; observed no_commit"),
	}
	if diff := verdictDiff(res.Verdicts, want); diff != "" {
		t.Fatal(diff)
	}
	if got := Outcome(res.Verdicts); got != Unproven {
		t.Fatalf("Outcome = %s, want unproven", got)
	}
	if res.Exit != 0 || res.Err != nil {
		t.Fatalf("RunOn = exit %d, %v; want a clean run", res.Exit, res.Err)
	}
}
