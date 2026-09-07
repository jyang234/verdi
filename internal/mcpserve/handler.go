package mcpserve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// HandlerTool is one consumer-owned tool definition carried over the shared
// MCP framing.
type HandlerTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// HandlerTerminal is a command terminal that becomes actionable only after
// the associated result frame has been written.
type HandlerTerminal struct {
	ExitCode int
}

// HandlerCallResult is one typed handler result. Text is the sole MCP content
// value; IsError marks a tool-level failure rather than a protocol failure.
type HandlerCallResult struct {
	Text     string
	IsError  bool
	Terminal *HandlerTerminal
}

// ToolHandler supplies a closed consumer-defined tool registry and dispatch.
type ToolHandler interface {
	Tools() []HandlerTool
	Call(context.Context, string, json.RawMessage) (HandlerCallResult, error)
}

// ServeHandler runs one consumer-defined typed handler over the exact framing
// used by ServeConn. It returns a requested terminal only after its framed
// tool result has been written successfully.
func ServeHandler(ctx context.Context, r io.Reader, w io.Writer, handler ToolHandler) (*HandlerTerminal, error) {
	if ctx == nil {
		return nil, errors.New("mcpserve: nil context")
	}
	if r == nil || w == nil || handler == nil {
		return nil, errors.New("mcpserve: handler requires reader, writer, and tool handler")
	}
	return serveFrames(ctx, r, w, &HandlerTerminal{ExitCode: 2}, decodeHandlerRequest, func(ctx context.Context, request rpcRequest) (rpcResponse, *HandlerTerminal) {
		return dispatchHandler(ctx, handler, request)
	})
}

func decodeHandlerRequest(data []byte, request *rpcRequest) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	if err := decodeHandlerJSON(data, request); err != nil {
		return err
	}
	if request.JSONRPC != "2.0" {
		return errors.New("jsonrpc must be exactly 2.0")
	}
	if request.Method == "" {
		return errors.New("method is required")
	}
	return nil
}

func dispatchHandler(ctx context.Context, handler ToolHandler, request rpcRequest) (rpcResponse, *HandlerTerminal) {
	response := rpcResponse{JSONRPC: "2.0", ID: request.ID}
	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "verdi", "version": ServerVersion},
		}
	case "ping":
		response.Result = map[string]any{}
	case "tools/list":
		response.Result = map[string]any{"tools": handler.Tools()}
	case "tools/call":
		result, terminal := callHandler(ctx, handler, request.Params)
		response.Result = result
		return response, terminal
	default:
		response.Error = &rpcError{Code: -32601, Message: "method not found: " + request.Method}
		return response, &HandlerTerminal{ExitCode: 2}
	}
	return response, nil
}

func callHandler(ctx context.Context, handler ToolHandler, params json.RawMessage) (map[string]any, *HandlerTerminal) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeHandlerCall(params, &call); err != nil {
		return toolError("malformed tools/call params: " + err.Error()), &HandlerTerminal{ExitCode: 2}
	}
	if call.Name == "" || len(call.Arguments) == 0 {
		return toolError("malformed tools/call params: name and arguments are required"), &HandlerTerminal{ExitCode: 2}
	}
	result, err := handler.Call(ctx, call.Name, call.Arguments)
	if err != nil {
		return toolError(err.Error()), &HandlerTerminal{ExitCode: 2}
	}
	if result.Terminal != nil && result.Terminal.ExitCode != 1 && result.Terminal.ExitCode != 2 {
		return toolError(fmt.Sprintf("invalid handler terminal exit code %d", result.Terminal.ExitCode)), &HandlerTerminal{ExitCode: 2}
	}
	framed := map[string]any{"content": []map[string]any{{"type": "text", "text": result.Text}}}
	if result.IsError {
		framed["isError"] = true
	}
	return framed, result.Terminal
}

// decodeHandlerCall decodes the tools/call PARAMS envelope (name/arguments,
// plus whatever else the caller put beside them). Unlike decodeHandlerJSON,
// it does not disallow unknown fields: see decodeTolerantJSON's doc comment
// for why (SI-188, spec/fail-loud dc-2).
func decodeHandlerCall(data []byte, target any) error {
	if err := rejectDuplicateJSONFields(data); err != nil {
		return err
	}
	return decodeTolerantJSON(data, target)
}

func decodeHandlerJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

// decodeTolerantJSON decodes data into target exactly like decodeHandlerJSON
// does, EXCEPT it never calls DisallowUnknownFields (SI-188). It feeds
// decodeHandlerCall's tools/call params envelope, which is a JSON-RPC/MCP
// protocol envelope: spec/fail-loud dc-2 keeps those TOLERANT of unknown
// members ("expected forward-compat, not a mistake to catch"), the exact
// posture this package's decode.go doc comment already claims for "wire.go's
// rpcRequest, server.go's tools/call name/arguments" and server.go's
// callTool already gives its own params (bare json.Unmarshal). MCP itself
// defines a `_meta` member on every request's params — Claude Code 2.1.261
// sends `_meta.progressToken` on every tools/call — and before this helper
// existed, decodeHandlerCall's DisallowUnknownFields refused it outright
// ("json: unknown field \"_meta\""), ending the sealed run at Claude's first
// tool call. An unknown member decoded here is never read, projected,
// hashed, or logged: target simply has no field for it. `name` and
// `arguments` stay REQUIRED (callHandler's own presence check) and
// type-checked — a wrong-typed value is still a decode error — and
// rejectDuplicateJSONFields, the caller's preceding step, still refuses a
// duplicate key or a trailing JSON value at any depth, unchanged.
//
// The outer JSON-RPC frame decode (decodeHandlerRequest, wire.go's
// rpcRequest) was inspected for the same defect and deliberately LEFT
// strict: real MCP/Claude Code traffic never puts an unrecognized member at
// that level (_meta nests inside params, never beside jsonrpc/id/method),
// and TestServeHandlerRejectsMalformedEnvelopeBeforeToolDispatch's "unknown
// envelope field" case already codifies refusing one there on purpose.
func decodeTolerantJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := consumeUniqueJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func consumeUniqueJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object field name is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate field %q", key)
			}
			seen[key] = struct{}{}
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := consumeUniqueJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("malformed JSON array")
		}
	default:
		return errors.New("malformed JSON delimiter")
	}
	return nil
}
