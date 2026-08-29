package mcpserve

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
)

const httpMCPBodyLimit int64 = 1 << 24

var httpMCPTokenRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// Handler is the typed MCP registry and dispatcher accepted by transports.
// It aliases the established ToolHandler name so transports do not introduce
// a second catalog or dispatch contract.
type Handler = ToolHandler

// NewHTTPHandler exposes one capability-bound streamable-HTTP MCP endpoint.
// The returned channel carries the first requested terminal, delivered only
// after its response frame was written.
func NewHTTPHandler(token string, handler Handler) (http.Handler, <-chan *HandlerTerminal, error) {
	if !httpMCPTokenRE.MatchString(token) {
		return nil, nil, errors.New("mcpserve: HTTP capability must be a canonical sha256 digest")
	}
	if handler == nil {
		return nil, nil, errors.New("mcpserve: HTTP handler is required")
	}
	terminals := make(chan *HandlerTerminal, 1)
	return &scopedHTTPHandler{authorization: "Bearer " + token, handler: handler, terminals: terminals}, terminals, nil
}

type scopedHTTPHandler struct {
	authorization string
	handler       Handler
	terminals     chan *HandlerTerminal
}

func (h *scopedHTTPHandler) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/mcp" || request.URL.RawPath != "" || request.URL.RawQuery != "" {
		http.NotFound(w, request)
		return
	}
	if request.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if values := request.Header.Values("Authorization"); len(values) != 1 || !constantTimeEqual(values[0], h.authorization) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if values := request.Header.Values("Content-Type"); len(values) != 1 || values[0] != "application/json" {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(io.LimitReader(request.Body, httpMCPBodyLimit+1))
	if err != nil {
		http.Error(w, "read request body", http.StatusBadRequest)
		return
	}
	if int64(len(body)) > httpMCPBodyLimit {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var rpcRequest rpcRequest
	if err := decodeHandlerRequest(body, &rpcRequest); err != nil {
		if writeHTTPRPCResponse(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}}) {
			// An unparseable frame on the scoped surface is operational, as it
			// is for ServeHandler's parse terminal.
			h.observeTerminal(&HandlerTerminal{ExitCode: 2})
		}
		return
	}
	if rpcRequest.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	response, terminal := dispatchHandler(request.Context(), h.handler, rpcRequest)
	if !writeHTTPRPCResponse(w, response) {
		return
	}
	// The HTTP request has no process exit channel, so the terminal reaches the
	// sealed lifecycle owner here — only after its JSON-RPC frame was written,
	// matching ServeHandler's ordering contract.
	h.observeTerminal(terminal)
}

// observeTerminal hands the sealed lifecycle owner the first terminal of the
// run. The buffered single delivery keeps a later terminal, or an owner that
// never reads, from blocking the response path; the run ends on the first one.
func (h *scopedHTTPHandler) observeTerminal(terminal *HandlerTerminal) {
	if terminal == nil {
		return
	}
	select {
	case h.terminals <- terminal:
	default:
	}
}

func writeHTTPRPCResponse(w http.ResponseWriter, response rpcResponse) bool {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response) == nil
}

func constantTimeEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
