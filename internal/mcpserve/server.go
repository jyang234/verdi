package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// ServerVersion is the version string reported in initialize's
// serverInfo. There is no tagged verdi release yet (I-4's ledger note:
// "verdi-go will eventually start tagging"); "dev" is an honest
// placeholder rather than a fabricated version number.
const ServerVersion = "dev"

// Server answers MCP requests over one or more connections, backed by one
// Backend. It holds no per-connection state — every field is read-only
// after construction — so the same *Server safely answers many
// connections concurrently (Serve spawns a goroutine per accepted
// connection; ServeConn keeps dispatch sequential within each one).
type Server struct {
	Backend *Backend
}

// NewServer constructs a Server over root (a resolved store root —
// internal/store.FindRoot's result).
func NewServer(root string) *Server {
	return &Server{Backend: &Backend{Root: root}}
}

// dispatch answers one JSON-RPC request. Protocol-level failures (unknown
// method) are JSON-RPC errors; tool failures are MCP tool results
// (isError), never protocol errors — the calling agent reads and corrects
// from those (matches groundwork's own convention).
func (s *Server) dispatch(ctx context.Context, req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "verdi", "version": ServerVersion},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolDefs()}
	case "tools/call":
		resp.Result = s.callTool(ctx, req.Params)
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

// callTool decodes a tools/call envelope and routes to the named tool's
// Backend method. An unrecognized tool name or malformed params/arguments
// is a tool error (isError), never a protocol error or a panic.
func (s *Server) callTool(ctx context.Context, params json.RawMessage) map[string]any {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &call); err != nil {
		return toolError("malformed tools/call params: " + err.Error())
	}

	switch call.Name {
	case "search_artifacts":
		return s.Backend.SearchArtifacts(ctx, call.Arguments)
	case "get_artifact":
		return s.Backend.GetArtifact(ctx, call.Arguments)
	case "get_links":
		return s.Backend.GetLinks(ctx, call.Arguments)
	case "get_matrix":
		return s.Backend.GetMatrix(ctx, call.Arguments)
	case "get_context_bundle":
		return s.Backend.GetContextBundle(ctx, call.Arguments)
	case "list_annotations":
		return s.Backend.ListAnnotations(ctx, call.Arguments)
	case "list_tasks":
		return s.Backend.ListTasks(ctx, call.Arguments)
	case "add_annotation":
		return s.Backend.AddAnnotation(ctx, call.Arguments)
	default:
		return toolError(fmt.Sprintf("unknown tool: %q", call.Name))
	}
}

// Serve accepts connections on ln until it errors (typically because ln
// was Close()d by the caller for shutdown), spawning a goroutine PER
// CONNECTION — the binding S4 finding (PLAN.md Phase 9): "a serial accept
// loop fully starves a second MCP client — proven". Dispatch stays
// sequential WITHIN each connection (ServeConn's own contract), so no
// write-mutex is needed per connection — only different connections ever
// run concurrently, and each owns its own net.Conn. Serve blocks until
// every in-flight connection's handler has returned, so a caller that
// closes ln and then calls Serve's return as a shutdown barrier gets a
// clean wait.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	var wg sync.WaitGroup
	var acceptErr error
	for {
		conn, err := ln.Accept()
		if err != nil {
			acceptErr = err
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			_ = ServeConn(ctx, conn, conn, s)
		}()
	}
	wg.Wait()
	return acceptErr
}
