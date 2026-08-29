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
func NewHTTPHandler(token string, handler Handler) (http.Handler, error) {
	if !httpMCPTokenRE.MatchString(token) {
		return nil, errors.New("mcpserve: HTTP capability must be a canonical sha256 digest")
	}
	if handler == nil {
		return nil, errors.New("mcpserve: HTTP handler is required")
	}
	return &scopedHTTPHandler{authorization: "Bearer " + token, handler: handler}, nil
}

type scopedHTTPHandler struct {
	authorization string
	handler       Handler
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
		writeHTTPRPCResponse(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{Code: -32700, Message: "parse error"}})
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
	// The HTTP request has no process exit channel. Reaching this point is the
	// terminal barrier: dispatch terminal state is observed only after its
	// JSON-RPC frame was written, matching ServeHandler's ordering contract.
	_ = terminal
}

func writeHTTPRPCResponse(w http.ResponseWriter, response rpcResponse) bool {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(response) == nil
}

func constantTimeEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
