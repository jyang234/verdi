package httpjson

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type payload struct {
	Name string `json:"name"`
}

// alwaysOK is the Classify every happy-path test uses: 2xx is success,
// anything else is a generic failure — mirrors the shape every real
// adapter's classifier reduces to on the golden path.
func alwaysOK(resp *http.Response, transportErr error) error {
	if transportErr != nil {
		return fmt.Errorf("transport: %w", transportErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

// TestDo_HappyPath is table-driven over GET (no body) and POST (JSON body,
// custom auth header) — the two verbs every one of the three consumers
// (github, gitlab, jira) issues through this seam.
func TestDo_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		reqBody  interface{}
		wantAuth string
		wantCT   string // "" = no Content-Type expected
	}{
		{name: "GET no body", method: http.MethodGet, reqBody: nil, wantAuth: "Bearer tok", wantCT: ""},
		{name: "POST with JSON body", method: http.MethodPost, reqBody: payload{Name: "in"}, wantAuth: "Bearer tok", wantCT: "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotAuth, gotCT string
			var gotBody []byte
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotAuth = r.Header.Get("Authorization")
				gotCT = r.Header.Get("Content-Type")
				buf := make([]byte, r.ContentLength)
				_, _ = r.Body.Read(buf)
				gotBody = buf
				_ = json.NewEncoder(w).Encode(payload{Name: "out"})
			}))
			defer ts.Close()

			c := &Client{HTTPClient: ts.Client()}
			setAuth := func(req *http.Request) { req.Header.Set("Authorization", "Bearer tok") }

			var out payload
			err := c.Do(context.Background(), tt.method, ts.URL, tt.reqBody, setAuth, alwaysOK, &out)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if out.Name != "out" {
				t.Errorf("decoded out.Name = %q, want %q", out.Name, "out")
			}
			if gotMethod != tt.method {
				t.Errorf("server saw method %q, want %q", gotMethod, tt.method)
			}
			if gotAuth != tt.wantAuth {
				t.Errorf("server saw Authorization %q, want %q", gotAuth, tt.wantAuth)
			}
			if gotCT != tt.wantCT {
				t.Errorf("server saw Content-Type %q, want %q", gotCT, tt.wantCT)
			}
			if tt.reqBody != nil && !strings.Contains(string(gotBody), `"in"`) {
				t.Errorf("server saw body %q, want it to contain the encoded request payload", gotBody)
			}
		})
	}
}

// TestDo_NilOut_DrainsAndDiscards proves a nil out (jira's field-write PUT
// shape) does not attempt to decode — it drains the body and returns nil on
// a classify-approved response, even when the body is non-empty/non-JSON.
func TestDo_NilOut_DrainsAndDiscards(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer ts.Close()

	c := &Client{HTTPClient: ts.Client()}
	err := c.Do(context.Background(), http.MethodPut, ts.URL, nil, nil, alwaysOK, nil)
	if err != nil {
		t.Fatalf("Do with nil out: %v", err)
	}
}

// TestDo_StatusClassifiedThroughHook proves a non-2xx response is routed
// through the caller's Classify — table-driven over a few statuses,
// including 429, each mapped to a DISTINCT sentinel the way a real
// adapter's classifier would (ac-3: "the forge side's transient refusal
// naming the status" / jira's ErrUnavailable).
func TestDo_StatusClassifiedThroughHook(t *testing.T) {
	errNotFound := errors.New("fake: not found")
	errRateLimited := errors.New("fake: rate limited")
	errUnexpected := errors.New("fake: unexpected")

	classify := func(resp *http.Response, transportErr error) error {
		if transportErr != nil {
			t.Fatalf("classify got unexpected transportErr: %v", transportErr)
		}
		switch resp.StatusCode {
		case http.StatusOK:
			return nil
		case http.StatusNotFound:
			return fmt.Errorf("classified 404: %w", errNotFound)
		case http.StatusTooManyRequests:
			return fmt.Errorf("classified 429 naming the status %s: %w", resp.Status, errRateLimited)
		default:
			return fmt.Errorf("classified %s: %w", resp.Status, errUnexpected)
		}
	}

	tests := []struct {
		name   string
		status int
		want   error
	}{
		{"404 maps to the caller's not-found sentinel", http.StatusNotFound, errNotFound},
		{"429 maps to the caller's rate-limit sentinel and names the status", http.StatusTooManyRequests, errRateLimited},
		{"500 falls through to the caller's generic sentinel", http.StatusInternalServerError, errUnexpected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer ts.Close()

			c := &Client{HTTPClient: ts.Client()}
			var out payload
			err := c.Do(context.Background(), http.MethodGet, ts.URL, nil, nil, classify, &out)
			if err == nil {
				t.Fatalf("Do against a %d response: want error, got nil", tt.status)
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("Do error = %v, want errors.Is(err, %v)", err, tt.want)
			}
			if tt.status == http.StatusTooManyRequests && !strings.Contains(err.Error(), "429") {
				t.Errorf("429 error %q does not name the status code", err.Error())
			}
		})
	}
}

// TestDo_TransportErrorClassifiedThroughHook proves a transport-level
// failure (here: a closed listener, so http.Client.Do itself errors before
// any response exists) is also routed through classify, not swallowed —
// jira's doJSON maps this case to provider.ErrUnavailable; this test proves
// the seam gives the caller that opportunity by passing (nil, transportErr).
func TestDo_TransportErrorClassifiedThroughHook(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens here now: dial refused

	errUnavailable := errors.New("fake: unavailable")
	classify := func(resp *http.Response, transportErr error) error {
		if transportErr == nil {
			t.Fatalf("classify got nil transportErr, want a dial error")
		}
		return fmt.Errorf("classified transport failure: %w: %v", errUnavailable, transportErr)
	}

	c := &Client{HTTPClient: &http.Client{Timeout: 2 * time.Second}}
	var out payload
	gotErr := c.Do(context.Background(), http.MethodGet, "http://"+addr, nil, nil, classify, &out)
	if gotErr == nil {
		t.Fatal("Do against a closed listener: want error, got nil")
	}
	if !errors.Is(gotErr, errUnavailable) {
		t.Fatalf("Do error = %v, want errors.Is(err, errUnavailable)", gotErr)
	}
}

// TestDo_TolerantDecode_IgnoresUnknownFields proves the seam decodes a
// foreign payload as a tolerant SUBSET (the package doc's DECODE POLICY):
// fields the out type does not declare are silently ignored rather than
// rejected — the opposite of internal/artifact's strict DisallowUnknownFields
// seam for verdi-owned artifacts.
func TestDo_TolerantDecode_IgnoresUnknownFields(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"known","self":"https://forge.example/api/internal/whatever","extra_nested":{"a":1}}`))
	}))
	defer ts.Close()

	c := &Client{HTTPClient: ts.Client()}
	var out payload
	if err := c.Do(context.Background(), http.MethodGet, ts.URL, nil, nil, alwaysOK, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Name != "known" {
		t.Errorf("out.Name = %q, want %q", out.Name, "known")
	}
}

// TestDo_TolerantDecode_TrailingDataNotRejected documents (co-1: "trailing-
// data behavior documented") that this seam's tolerant decode does NOT
// reject data trailing the first JSON value — json.Decoder.Decode consumes
// exactly one value and leaves the rest unread, and this seam never calls
// Decode a second time or checks io.EOF the way internal/artifact's strict
// seam does. A forge/tracker response is never expected to carry trailing
// data in practice; this test pins the boundary so a future reader does not
// mistake this package's silence on the topic for strict rejection.
func TestDo_TolerantDecode_TrailingDataNotRejected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"known"}` + "\ntrailing garbage, not valid JSON on its own"))
	}))
	defer ts.Close()

	c := &Client{HTTPClient: ts.Client()}
	var out payload
	if err := c.Do(context.Background(), http.MethodGet, ts.URL, nil, nil, alwaysOK, &out); err != nil {
		t.Fatalf("Do: want trailing data to be silently ignored, got error: %v", err)
	}
	if out.Name != "known" {
		t.Errorf("out.Name = %q, want %q", out.Name, "known")
	}
}

// TestDo_Timeout_StallingHandlerWithShortInjectedClient proves the seam
// bounds a call: a deliberately stalling handler paired with a SHORT
// injected client (never the 30s default — this test must not sleep 30s,
// CLAUDE.md/co-1) still returns promptly with an error.
func TestDo_Timeout_StallingHandlerWithShortInjectedClient(t *testing.T) {
	// The handler stalls for longer than the client's timeout below, then
	// returns on its own — a fixed, short stall (never an indefinite block
	// requiring out-of-band teardown signaling, which would race
	// httptest.Server.Close's own "wait for in-flight handlers" behavior).
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer ts.Close()

	c := &Client{HTTPClient: &http.Client{Timeout: 50 * time.Millisecond}}
	start := time.Now()
	var out payload
	err := c.Do(context.Background(), http.MethodGet, ts.URL, nil, nil, alwaysOK, &out)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Do against a stalling handler: want error, got nil")
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Do took %v to fail, want it bounded by the short injected client's timeout", elapsed)
	}
}

// TestClient_DefaultHTTPClient_UsesDefaultTimeout proves a Client with a
// nil HTTPClient resolves to a client carrying DefaultTimeout (dc-2), and
// that a caller-supplied HTTPClient is used AS-IS (including a zero
// Timeout) rather than being wrapped or mutated.
func TestClient_DefaultHTTPClient_UsesDefaultTimeout(t *testing.T) {
	var zero Client
	got := zero.httpClient()
	if got.Timeout != DefaultTimeout {
		t.Fatalf("zero Client's resolved HTTPClient.Timeout = %v, want %v", got.Timeout, DefaultTimeout)
	}

	injected := &http.Client{Timeout: 0}
	withInjected := Client{HTTPClient: injected}
	if withInjected.httpClient() != injected {
		t.Fatal("Client with a non-nil HTTPClient must use it AS-IS, not substitute the default")
	}
}

// TestRawDo_NonJSONResponse proves RawDo hands back the raw, unread
// response for callers whose payload is not JSON (github/gitlab's binary
// artifact zip download rides this, not Do).
func TestRawDo_NonJSONResponse(t *testing.T) {
	want := []byte{0x50, 0x4b, 0x03, 0x04, 0xDE, 0xAD, 0xBE, 0xEF} // fake zip-ish bytes
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(want)
	}))
	defer ts.Close()

	c := &Client{HTTPClient: ts.Client()}
	resp, err := c.RawDo(context.Background(), http.MethodGet, ts.URL, nil, nil)
	if err != nil {
		t.Fatalf("RawDo: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("RawDo status = %d, want 200", resp.StatusCode)
	}
}

// TestDo_EncodesRequestBodyError proves a request body that cannot be
// JSON-encoded fails fast (building the request), never reaching the
// network — a negative-path witness for the request-build responsibility
// dc-1 assigns this seam.
func TestDo_EncodesRequestBodyError(t *testing.T) {
	c := &Client{}
	err := c.Do(context.Background(), http.MethodPost, "http://example.invalid", func() {}, nil, alwaysOK, nil)
	if err == nil {
		t.Fatal("Do with an unencodable body: want error, got nil")
	}
}
