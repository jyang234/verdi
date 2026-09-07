package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestServeHandlerUsesEstablishedFraming(t *testing.T) {
	handler := &cannedToolHandler{
		tools:  []HandlerTool{{Name: "only_tool", Description: "Only the sealed tool.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`)}},
		result: HandlerCallResult{Text: `{"kind":"epoch-invalidated"}`, Terminal: &HandlerTerminal{ExitCode: 1}},
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"only_tool","arguments":{}}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	terminal, err := ServeHandler(context.Background(), strings.NewReader(input), &output, handler)
	if err != nil {
		t.Fatalf("ServeHandler: %v", err)
	}
	if terminal == nil || terminal.ExitCode != 1 {
		t.Fatalf("terminal = %#v, want exit 1 after result", terminal)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("frames = %d\n%s", len(lines), output.String())
	}
	var listed struct {
		Result struct {
			Tools []HandlerTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &listed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed.Result.Tools, handler.tools) {
		t.Fatalf("tools = %#v, want exact typed registry %#v", listed.Result.Tools, handler.tools)
	}
	var called struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &called); err != nil {
		t.Fatal(err)
	}
	if len(called.Result.Content) != 1 || called.Result.Content[0].Type != "text" || called.Result.Content[0].Text != handler.result.Text {
		t.Fatalf("tool result frame = %s, want exact handler text before terminal", lines[2])
	}
	if handler.calls != 1 || handler.lastName != "only_tool" || string(handler.lastArguments) != `{}` {
		t.Fatalf("handler calls/name/arguments = %d/%q/%s", handler.calls, handler.lastName, handler.lastArguments)
	}
}

func TestServeHandlerDefersTerminalUntilWrite(t *testing.T) {
	handler := &cannedToolHandler{
		tools:  []HandlerTool{},
		result: HandlerCallResult{Text: "denied", Terminal: &HandlerTerminal{ExitCode: 2}},
	}
	writer := &failingHandlerWriter{}
	terminal, err := ServeHandler(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","arguments":{}}}`+"\n"), writer, handler)
	if err == nil || terminal != nil || writer.calls != 1 {
		t.Fatalf("terminal/error/write calls = %#v/%v/%d, want no terminal after failed frame", terminal, err, writer.calls)
	}
}

func TestServeHandlerProtocolAndHandlerFailures(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   string
		handler *cannedToolHandler
		want    string
	}{
		{name: "parse error", input: "{not-json}\n", handler: &cannedToolHandler{tools: []HandlerTool{}}, want: `"code":-32700`},
		{name: "unknown method", input: `{"jsonrpc":"2.0","id":2,"method":"unknown"}` + "\n", handler: &cannedToolHandler{tools: []HandlerTool{}}, want: `"code":-32601`},
		{name: "malformed call", input: `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{}}` + "\n", handler: &cannedToolHandler{tools: []HandlerTool{}}, want: `"isError":true`},
		{name: "handler failure", input: `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"x","arguments":{}}}` + "\n", handler: &cannedToolHandler{tools: []HandlerTool{}, callErr: errors.New("controller unavailable")}, want: "controller unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			terminal, err := ServeHandler(context.Background(), strings.NewReader(test.input), &output, test.handler)
			if err != nil || terminal == nil || terminal.ExitCode != 2 {
				t.Fatalf("terminal/error = %#v/%v, want framed operational terminal", terminal, err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("protocol/handler frame missing %q:\n%s", test.want, output.String())
			}
		})
	}
}

func TestServeHandlerRejectsMalformedEnvelopeBeforeToolDispatch(t *testing.T) {
	mutatingParams := `"params":{"name":"request_context","arguments":{"ref":"spec/extra","purpose":"needed"}}`
	for _, test := range []struct {
		name  string
		input string
	}{
		{name: "missing jsonrpc", input: `{"id":1,"method":"tools/call",` + mutatingParams + `}`},
		{name: "wrong jsonrpc", input: `{"jsonrpc":"1.0","id":1,"method":"tools/call",` + mutatingParams + `}`},
		{name: "unknown envelope field", input: `{"jsonrpc":"2.0","id":1,"method":"tools/call","unexpected":true,` + mutatingParams + `}`},
		{name: "duplicate envelope field", input: `{"jsonrpc":"2.0","id":1,"method":"tools/list","method":"tools/call",` + mutatingParams + `}`},
		{name: "duplicate call name", input: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_flight_plan","name":"request_context","arguments":{"ref":"spec/extra","purpose":"needed"}}}`},
		{name: "duplicate call arguments", input: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{},"arguments":{"ref":"spec/extra","purpose":"needed"}}}`},
		{name: "trailing envelope value", input: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` + mutatingParams + `}{"jsonrpc":"2.0"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := &cannedToolHandler{result: HandlerCallResult{Text: "installed"}}
			var output bytes.Buffer
			terminal, err := ServeHandler(context.Background(), strings.NewReader(test.input+"\n"), &output, handler)
			if err != nil || terminal == nil || terminal.ExitCode != 2 {
				t.Fatalf("terminal/error = %#v/%v, want framed operational termination", terminal, err)
			}
			if handler.calls != 0 {
				t.Fatalf("handler calls = %d, malformed request crossed the mutating tool boundary", handler.calls)
			}
			if !strings.Contains(output.String(), `"error"`) {
				t.Fatalf("malformed request frame = %q, want protocol error", output.String())
			}
		})
	}
}

// TestDecodeHandlerCallToleratesUnknownParamsMembers is SI-188's core
// witness at the decode function itself: Claude Code 2.1.261 sends a
// `_meta` member (e.g. `{"progressToken":1}`) beside `name`/`arguments` on
// every tools/call — MCP itself defines `_meta` on every request's params —
// and spec/fail-loud dc-2 keeps a JSON-RPC/MCP protocol envelope TOLERANT
// of unknown members, matching this package's own decode.go doc comment and
// server.go's callTool (bare json.Unmarshal on the identical params shape).
// Before the fix (decodeHandlerJSON's DisallowUnknownFields), every
// "tolerates" case below failed with `json: unknown field "_meta"` (or
// "x"), which is exactly the RED this table proves and pins against
// regression. Table-driven per co-1: the tolerated cases sit beside the
// still-refused ones (duplicate keys, trailing data, a wrong-typed name) so
// the exact boundary of the tolerance is visible in one place.
func TestDecodeHandlerCallToleratesUnknownParamsMembers(t *testing.T) {
	for _, test := range []struct {
		name       string
		data       string
		wantErr    bool
		wantErrSub string
		wantName   string
		wantArgs   string
	}{
		{
			name:     "tolerates the MCP _meta member",
			data:     `{"name":"request_context","arguments":{"ref":"spec/story-alpha","purpose":"p"},"_meta":{"progressToken":1}}`,
			wantName: "request_context",
			wantArgs: `{"ref":"spec/story-alpha","purpose":"p"}`,
		},
		{
			name:     "tolerates an unrelated unknown member",
			data:     `{"name":"get_flight_plan","arguments":{},"x":1}`,
			wantName: "get_flight_plan",
			wantArgs: `{}`,
		},
		{
			name:       "still refuses a duplicate name key",
			data:       `{"name":"a","arguments":{},"name":"b"}`,
			wantErr:    true,
			wantErrSub: "duplicate",
		},
		{
			name:       "still refuses a duplicate arguments key",
			data:       `{"name":"a","arguments":{},"arguments":{"x":1}}`,
			wantErr:    true,
			wantErrSub: "duplicate",
		},
		{
			name:    "still refuses trailing data",
			data:    `{"name":"a","arguments":{}}{}`,
			wantErr: true,
		},
		{
			name:    "still refuses a wrong-typed name",
			data:    `{"name":1,"arguments":{}}`,
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var call struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			err := decodeHandlerCall([]byte(test.data), &call)
			if test.wantErr {
				if err == nil {
					t.Fatalf("decodeHandlerCall(%s) = nil error, want refusal", test.data)
				}
				if test.wantErrSub != "" && !strings.Contains(err.Error(), test.wantErrSub) {
					t.Fatalf("decodeHandlerCall(%s) error = %q, want substring %q", test.data, err.Error(), test.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeHandlerCall(%s): unexpected error: %v", test.data, err)
			}
			if call.Name != test.wantName || string(call.Arguments) != test.wantArgs {
				t.Fatalf("decodeHandlerCall(%s) = name %q arguments %s, want name %q arguments %s", test.data, call.Name, call.Arguments, test.wantName, test.wantArgs)
			}
		})
	}
}

// TestServeHandlerToolsCallParamsToleranceReachesTheHandlerUnobserved proves
// the SI-188 tolerance end to end over the real framing (ServeHandler, not
// just the decode function): a tolerated unknown member reaches
// handler.Call with EXACTLY the enclosed `arguments` bytes and is not
// observable anywhere in the written frame — no projection, no digest, no
// log — while `name` and `arguments` remain required even when `_meta` is
// present alongside them.
func TestServeHandlerToolsCallParamsToleranceReachesTheHandlerUnobserved(t *testing.T) {
	for _, test := range []struct {
		name        string
		input       string
		wantSuccess bool
		wantName    string
		wantArgs    string
	}{
		{
			name:        "the MCP _meta member",
			input:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"request_context","arguments":{"ref":"spec/story-alpha","purpose":"p"},"_meta":{"progressToken":1}}}`,
			wantSuccess: true,
			wantName:    "request_context",
			wantArgs:    `{"ref":"spec/story-alpha","purpose":"p"}`,
		},
		{
			name:        "an unrelated unknown member",
			input:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_flight_plan","arguments":{},"x":1}}`,
			wantSuccess: true,
			wantName:    "get_flight_plan",
			wantArgs:    `{}`,
		},
		{
			name:  "name and arguments stay required beside _meta",
			input: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"_meta":{"progressToken":1}}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := &cannedToolHandler{result: HandlerCallResult{Text: "installed"}}
			var output bytes.Buffer
			terminal, err := ServeHandler(context.Background(), strings.NewReader(test.input+"\n"), &output, handler)
			if err != nil {
				t.Fatalf("ServeHandler: %v", err)
			}
			if test.wantSuccess {
				if terminal != nil {
					t.Fatalf("terminal = %#v, want nil (run continues)", terminal)
				}
				if handler.calls != 1 || handler.lastName != test.wantName {
					t.Fatalf("handler calls/name = %d/%q, want exactly one %q call", handler.calls, handler.lastName, test.wantName)
				}
				if string(handler.lastArguments) != test.wantArgs {
					t.Fatalf("handler arguments = %s, want exactly %s (the tolerated member must not bleed in)", handler.lastArguments, test.wantArgs)
				}
				if strings.Contains(output.String(), "_meta") || strings.Contains(output.String(), "progressToken") {
					t.Fatalf("response frame observed the tolerated member: %s", output.String())
				}
				return
			}
			if terminal == nil || terminal.ExitCode != 2 {
				t.Fatalf("terminal = %#v, want operational termination", terminal)
			}
			if handler.calls != 0 {
				t.Fatalf("handler calls = %d, want 0 (name/arguments still required)", handler.calls)
			}
			if !strings.Contains(output.String(), "name and arguments are required") {
				t.Fatalf("response = %s, want the name/arguments-required refusal", output.String())
			}
		})
	}
}

func TestServeHandlerCleanEOF(t *testing.T) {
	terminal, err := ServeHandler(context.Background(), strings.NewReader(""), &bytes.Buffer{}, &cannedToolHandler{tools: []HandlerTool{}})
	if err != nil || terminal != nil {
		t.Fatalf("clean EOF = %#v/%v, want nil/nil", terminal, err)
	}
}

type cannedToolHandler struct {
	tools         []HandlerTool
	result        HandlerCallResult
	callErr       error
	calls         int
	lastName      string
	lastArguments json.RawMessage
}

func (h *cannedToolHandler) Tools() []HandlerTool {
	return append([]HandlerTool(nil), h.tools...)
}

func (h *cannedToolHandler) Call(_ context.Context, name string, arguments json.RawMessage) (HandlerCallResult, error) {
	h.calls++
	h.lastName = name
	h.lastArguments = append(json.RawMessage(nil), arguments...)
	return h.result, h.callErr
}

type failingHandlerWriter struct{ calls int }

func (w *failingHandlerWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, errors.New("write failed")
}
