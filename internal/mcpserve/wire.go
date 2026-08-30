// Wire framing: newline-delimited JSON-RPC 2.0, the MCP stdio transport's
// wire format (I-13: "the shim degenerates to a pipe" — the socket speaks
// exactly the same framing as stdio, so `verdi mcp` needs no translation,
// only byte-piping). Modeled directly on verdi-go's own hand-rolled
// server (cmd/groundwork/mcp.go: rpcRequest/rpcResponse/serveMCP) and the
// wave-4 S4 spike's prototype (read-only references, not imported).
package mcpserve

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
)

// ProtocolVersion is the MCP protocol version this server speaks
// (PLAN.md Phase 9: "protocol 2024-11-05, tools capability").
const ProtocolVersion = "2024-11-05"

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeConn runs the read-dispatch-write loop over ONE connection (or the
// stdio pair, for the standalone-serving fallback) until EOF or a write
// error: scan newline-delimited requests, dispatch each in turn, encode
// the response. Dispatch is deliberately SEQUENTIAL within a connection
// (the binding S4 finding, PLAN.md Phase 9: "keep per-connection dispatch
// sequential ... if per-request goroutines ever share a conn, a write
// mutex is mandatory" — staying sequential means no such mutex is needed
// here, because exactly one goroutine ever writes to w). Concurrency
// across DIFFERENT connections is the caller's job (Server.Serve spawns a
// goroutine per accepted connection).
func ServeConn(ctx context.Context, r io.Reader, w io.Writer, srv *Server) error {
	_, err := serveFrames(ctx, r, w, nil, nil, func(ctx context.Context, req rpcRequest) (rpcResponse, *HandlerTerminal) {
		return srv.dispatch(ctx, req), nil
	})
	return err
}

type frameDispatcher func(context.Context, rpcRequest) (rpcResponse, *HandlerTerminal)
type frameDecoder func([]byte, *rpcRequest) error

// serveFrames is the one JSON-RPC/NDJSON read-dispatch-write seam shared by
// the generic artifact catalog and consumer-defined typed handlers. A
// terminal is observable only after its response frame is written.
func serveFrames(ctx context.Context, r io.Reader, w io.Writer, parseTerminal *HandlerTerminal, decode frameDecoder, dispatch frameDispatcher) (*HandlerTerminal, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	enc := json.NewEncoder(w)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		var decodeErr error
		if decode == nil {
			decodeErr = json.Unmarshal(line, &req)
		} else {
			decodeErr = decode(line, &req)
		}
		if decodeErr != nil {
			// A malformed request: if it named an id we could recover, a
			// silently-dropped response would hang the caller forever;
			// since the id is unrecoverable from unparseable input, reply
			// with a null-id JSON-RPC parse error instead (mirrors
			// groundwork's own serveMCP).
			if encErr := enc.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}}); encErr != nil {
				return nil, encErr
			}
			if parseTerminal != nil {
				return parseTerminal, nil
			}
			continue
		}
		if req.ID == nil {
			continue // a notification: JSON-RPC produces no response
		}
		response, terminal := dispatch(ctx, req)
		if err := enc.Encode(response); err != nil {
			return nil, err
		}
		if terminal != nil {
			return terminal, nil
		}
	}
	return nil, sc.Err()
}
