package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
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

	// ErrLog receives one line per dropped socket connection (spec/fail-loud
	// dc-3): a non-EOF, non-shutdown error returned by ServeConn for a
	// connection Serve accepted. nil (the zero value) is silent — matching
	// the stdio path (cmd/verdi/mcp.go's serveStandalone), which already
	// inspects its own ServeConn error and stays untouched by this field.
	// `verdi serve` wires os.Stderr; tests inject a bytes.Buffer.
	ErrLog io.Writer

	// model is the store's resolved operating model (set by NewServer,
	// once, from store.Open): toolDefs interpolates its class display
	// words into the tool catalog's description text
	// (spec/vocabulary-surfaces ac-3). nil serves bare ids.
	model *model.Model
}

// NewServer constructs a Server over root (a resolved store root —
// internal/store.FindRoot's result). It resolves the store's operating
// model ONCE here (store.Open — spec/vocabulary-surfaces ac-3: the tool
// catalog's assembly step reads Config.Model for class display words;
// construction is the entrypoint, never a per-request open). A root
// whose config cannot be opened serves bare ids (nil model) — the exact
// posture a model with no renames has, and the same fail-soft NewServer
// has always had (per-call store problems surface as tool errors).
func NewServer(root string) *Server {
	var mdl *model.Model
	if cfg, err := store.Open(root); err == nil {
		mdl = cfg.Model
	}
	return &Server{Backend: &Backend{Root: root}, model: mdl}
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
		resp.Result = map[string]any{"tools": toolDefs(s.model)}
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
	case "get_board":
		return s.Backend.GetBoard(ctx, call.Arguments)
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
			if cerr := ServeConn(ctx, conn, conn, s); cerr != nil {
				s.logConnErr(conn, cerr)
			}
		}()
	}
	wg.Wait()
	return acceptErr
}

// logConnErr writes one line to s.ErrLog for a dropped connection (dc-3):
// a genuine ServeConn error (a scan failure such as an oversized line, or a
// write failure such as a broken pipe) leaves a trace. Two cases are
// deliberately excluded, matching a CLEAN close: io.EOF (bufio.Scanner
// already treats plain EOF as termination, not an error — ServeConn only
// ever returns io.EOF itself if a caller's io.Reader does, which none of
// this package's callers do, but the check stays defensive) and
// net.ErrClosed (the connection or its underlying fd was closed out from
// under the read/write, e.g. during shutdown — not a protocol-level
// failure worth a line). s.ErrLog nil (the zero-value Server) is silent.
func (s *Server) logConnErr(conn net.Conn, err error) {
	if s.ErrLog == nil || err == nil {
		return
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return
	}
	remote := "unknown"
	if conn != nil {
		if addr := conn.RemoteAddr(); addr != nil && addr.String() != "" {
			remote = addr.String()
		}
	}
	fmt.Fprintf(s.ErrLog, "mcpserve: dropped connection (remote %s): %v\n", remote, err)
}
