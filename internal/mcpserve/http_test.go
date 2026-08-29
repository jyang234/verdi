package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const httpTestToken = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestHTTPMCPProtocol(t *testing.T) {
	handler := &httpTestHandler{tools: []HandlerTool{
		{Name: "get_flight_plan", Description: "Inspect the flight.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`)},
		{Name: "request_context", Description: "Request context.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{"purpose":{"type":"string"},"ref":{"type":"string"}},"required":["ref","purpose"],"type":"object"}`)},
	}}
	httpHandler, err := NewHTTPHandler(httpTestToken, handler)
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}

	t.Run("initialize", func(t *testing.T) {
		response := postHTTPMCP(t, httpHandler, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
		assertHTTPJSONResponse(t, response, http.StatusOK)
		var frame struct {
			JSONRPC string `json:"jsonrpc"`
			ID      int    `json:"id"`
			Result  struct {
				ProtocolVersion string `json:"protocolVersion"`
				ServerInfo      struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"serverInfo"`
			} `json:"result"`
		}
		decodeHTTPFrame(t, response, &frame)
		if frame.JSONRPC != "2.0" || frame.ID != 1 || frame.Result.ProtocolVersion != ProtocolVersion || frame.Result.ServerInfo.Name != "verdi" || frame.Result.ServerInfo.Version != ServerVersion {
			t.Fatalf("initialize frame = %#v", frame)
		}
	})

	t.Run("tools list is exact handler registry", func(t *testing.T) {
		response := postHTTPMCP(t, httpHandler, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
		assertHTTPJSONResponse(t, response, http.StatusOK)
		var frame struct {
			Result struct {
				Tools []HandlerTool `json:"tools"`
			} `json:"result"`
		}
		decodeHTTPFrame(t, response, &frame)
		if !reflect.DeepEqual(frame.Result.Tools, handler.tools) {
			t.Fatalf("tools = %#v, want exact two-tool registry %#v", frame.Result.Tools, handler.tools)
		}
	})

	for _, test := range []struct {
		name      string
		id        int
		tool      string
		arguments string
		wantText  string
	}{
		{name: "first tool", id: 3, tool: "get_flight_plan", arguments: `{}`, wantText: `{"kind":"flight-plan"}`},
		{name: "second tool", id: 4, tool: "request_context", arguments: `{"purpose":"implementation","ref":"spec/extra"}`, wantText: `{"kind":"context-approved"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":` + jsonInt(test.id) + `,"method":"tools/call","params":{"name":"` + test.tool + `","arguments":` + test.arguments + `}}`
			response := postHTTPMCP(t, httpHandler, body)
			assertHTTPJSONResponse(t, response, http.StatusOK)
			var frame struct {
				Result struct {
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			decodeHTTPFrame(t, response, &frame)
			if len(frame.Result.Content) != 1 || frame.Result.Content[0].Type != "text" || frame.Result.Content[0].Text != test.wantText {
				t.Fatalf("tool response = %#v", frame.Result.Content)
			}
		})
	}
	if got, want := handler.callNames, []string{"get_flight_plan", "request_context"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("handler calls = %v, want %v", got, want)
	}

	t.Run("notification has no response", func(t *testing.T) {
		response := postHTTPMCP(t, httpHandler, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
		if response.Code != http.StatusAccepted || response.Body.Len() != 0 {
			t.Fatalf("notification status/body = %d/%q, want 202/empty", response.Code, response.Body.String())
		}
	})

	t.Run("unknown tool frames error before terminal", func(t *testing.T) {
		response := postHTTPMCP(t, httpHandler, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"extra_tool","arguments":{}}}`)
		assertHTTPJSONResponse(t, response, http.StatusOK)
		var frame struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		decodeHTTPFrame(t, response, &frame)
		if !frame.Result.IsError || len(frame.Result.Content) != 1 || !strings.Contains(frame.Result.Content[0].Text, "unknown scoped tool") {
			t.Fatalf("unknown-tool frame = %#v", frame.Result)
		}
	})
}

func TestHTTPMCPRejectsWrongRequestShape(t *testing.T) {
	handler := &httpTestHandler{tools: []HandlerTool{}}
	httpHandler, err := NewHTTPHandler(httpTestToken, handler)
	if err != nil {
		t.Fatal(err)
	}
	validBody := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`

	tests := []struct {
		name        string
		method      string
		target      string
		authorizers []string
		contentType []string
		body        string
		wantStatus  int
	}{
		{name: "wrong method", method: http.MethodGet, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong path", method: http.MethodPost, target: "/other", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusNotFound},
		{name: "path suffix", method: http.MethodPost, target: "/mcp/", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusNotFound},
		{name: "encoded path is not exact", method: http.MethodPost, target: "/%6dcp", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusNotFound},
		{name: "query is not exact path", method: http.MethodPost, target: "/mcp?session=ambient", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusNotFound},
		{name: "missing token", method: http.MethodPost, target: "/mcp", contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusUnauthorized},
		{name: "wrong token", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusUnauthorized},
		{name: "duplicate token", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken, "Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: validBody, wantStatus: http.StatusUnauthorized},
		{name: "missing content type", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken}, body: validBody, wantStatus: http.StatusUnsupportedMediaType},
		{name: "content type parameter", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json; charset=utf-8"}, body: validBody, wantStatus: http.StatusUnsupportedMediaType},
		{name: "duplicate content type", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json", "application/json"}, body: validBody, wantStatus: http.StatusUnsupportedMediaType},
		{name: "body over established framing limit", method: http.MethodPost, target: "/mcp", authorizers: []string{"Bearer " + httpTestToken}, contentType: []string{"application/json"}, body: strings.Repeat("x", (1<<24)+1), wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			for _, value := range test.authorizers {
				request.Header.Add("Authorization", value)
			}
			for _, value := range test.contentType {
				request.Header.Add("Content-Type", value)
			}
			response := httptest.NewRecorder()
			httpHandler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
	if len(handler.callNames) != 0 {
		t.Fatalf("rejected requests reached handler: %v", handler.callNames)
	}
}

func TestHTTPMCPMalformedAndUnknownJSONRPC(t *testing.T) {
	httpHandler, err := NewHTTPHandler(httpTestToken, &httpTestHandler{tools: []HandlerTool{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{
		``,
		`{not-json}`,
		`{"jsonrpc":"2.0","jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","extra":true}`,
		`{"jsonrpc":"1.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","id":1}`,
		"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"initialize\"}",
	} {
		response := postHTTPMCP(t, httpHandler, body)
		assertHTTPJSONResponse(t, response, http.StatusOK)
		var frame struct {
			JSONRPC string `json:"jsonrpc"`
			ID      any    `json:"id"`
			Error   struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		decodeHTTPFrame(t, response, &frame)
		if frame.JSONRPC != "2.0" || frame.ID != nil || frame.Error.Code != -32700 || frame.Error.Message != "parse error" {
			t.Fatalf("malformed response = %#v for body %q", frame, body)
		}
	}

	response := postHTTPMCP(t, httpHandler, `{"jsonrpc":"2.0","id":9,"method":"ambient/method"}`)
	assertHTTPJSONResponse(t, response, http.StatusOK)
	var frame struct {
		ID    int `json:"id"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeHTTPFrame(t, response, &frame)
	if frame.ID != 9 || frame.Error.Code != -32601 || frame.Error.Message != "method not found: ambient/method" {
		t.Fatalf("unknown-method response = %#v", frame)
	}
}

func TestNewHTTPHandlerRejectsInvalidConfiguration(t *testing.T) {
	validHandler := &httpTestHandler{tools: []HandlerTool{}}
	for _, test := range []struct {
		name    string
		token   string
		handler Handler
	}{
		{name: "empty token", token: "", handler: validHandler},
		{name: "bare digest", token: strings.Repeat("a", 64), handler: validHandler},
		{name: "uppercase digest", token: "sha256:" + strings.Repeat("A", 64), handler: validHandler},
		{name: "nil handler", token: httpTestToken, handler: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewHTTPHandler(test.token, test.handler); err == nil {
				t.Fatal("NewHTTPHandler accepted invalid configuration")
			}
		})
	}
}

type httpTestHandler struct {
	tools     []HandlerTool
	callNames []string
}

func (h *httpTestHandler) Tools() []HandlerTool {
	return append([]HandlerTool(nil), h.tools...)
}

func (h *httpTestHandler) Call(_ context.Context, name string, _ json.RawMessage) (HandlerCallResult, error) {
	h.callNames = append(h.callNames, name)
	switch name {
	case "get_flight_plan":
		return HandlerCallResult{Text: `{"kind":"flight-plan"}`}, nil
	case "request_context":
		return HandlerCallResult{Text: `{"kind":"context-approved"}`, Terminal: &HandlerTerminal{ExitCode: 1}}, nil
	default:
		return HandlerCallResult{}, errors.New("unknown scoped tool " + name)
	}
}

func postHTTPMCP(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+httpTestToken)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertHTTPJSONResponse(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status || response.Header().Get("Content-Type") != "application/json" || !strings.HasSuffix(response.Body.String(), "\n") {
		t.Fatalf("status/content-type/body = %d/%q/%q, want %d/application/json/LF-framed", response.Code, response.Header().Get("Content-Type"), response.Body.String(), status)
	}
}

func decodeHTTPFrame(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}

func jsonInt(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
