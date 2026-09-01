package sealedexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/mcpserve"
)

const (
	scopedTestProfileDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	scopedTestWorkspaceID   = "flight--0123456789ab"
)

var scopedTestRequest = []byte("{\"schema\":\"verdi.context-execution-request/v1\",\"test\":\"alpha\"}\n")

func scopedTestDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func scopedTestSet() RequiredMCPSet {
	return RequiredMCPSet{
		Claim: RequiredMCP{
			Name: RequiredClaimMCPName, URL: "http://127.0.0.1:45001/mcp",
			Authorization: "Bearer sha256:" + strings.Repeat("c", 64), Tools: []string{ToolClaimPaths},
		},
		Context: RequiredMCP{
			Name: RequiredContextMCPName, URL: "http://127.0.0.1:45002/mcp",
			Authorization: "Bearer sha256:" + strings.Repeat("d", 64),
			Tools:         []string{ToolGetFlightPlan, ToolRequestContext},
		},
	}
}

func TestRequiredMCPSetValidate(t *testing.T) {
	if err := scopedTestSet().Validate(); err != nil {
		t.Fatalf("valid required MCP set refused: %v", err)
	}
	for name, mutate := range map[string]func(*RequiredMCPSet){
		"claim renamed":          func(s *RequiredMCPSet) { s.Claim.Name = "vatc2" },
		"context renamed":        func(s *RequiredMCPSet) { s.Context.Name = "verdi_context" },
		"claim missing":          func(s *RequiredMCPSet) { s.Claim = RequiredMCP{} },
		"context missing":        func(s *RequiredMCPSet) { s.Context = RequiredMCP{} },
		"swapped names":          func(s *RequiredMCPSet) { s.Claim.Name, s.Context.Name = s.Context.Name, s.Claim.Name },
		"shared origin":          func(s *RequiredMCPSet) { s.Claim.URL = s.Context.URL },
		"shared capability":      func(s *RequiredMCPSet) { s.Claim.Authorization = s.Context.Authorization },
		"overlapping catalogues": func(s *RequiredMCPSet) { s.Context.Tools = []string{ToolGetFlightPlan, ToolClaimPaths} },
		"claim extra tool":       func(s *RequiredMCPSet) { s.Claim.Tools = []string{ToolClaimPaths, ToolGetFlightPlan} },
		"claim no tools":         func(s *RequiredMCPSet) { s.Claim.Tools = nil },
		"context reordered":      func(s *RequiredMCPSet) { s.Context.Tools = []string{ToolRequestContext, ToolGetFlightPlan} },
		"bare capability":        func(s *RequiredMCPSet) { s.Claim.Authorization = strings.TrimPrefix(s.Claim.Authorization, "Bearer ") },
		"wrong scheme":           func(s *RequiredMCPSet) { s.Context.Authorization = "Token sha256:" + strings.Repeat("d", 64) },
		"short capability":       func(s *RequiredMCPSet) { s.Context.Authorization = "Bearer sha256:" + strings.Repeat("d", 63) },
		"uppercase capability":   func(s *RequiredMCPSet) { s.Context.Authorization = "Bearer sha256:" + strings.Repeat("D", 64) },
		"https origin":           func(s *RequiredMCPSet) { s.Claim.URL = "https://127.0.0.1:45001/mcp" },
		"non-loopback origin":    func(s *RequiredMCPSet) { s.Claim.URL = "http://127.0.0.2:45001/mcp" },
		"hostname origin":        func(s *RequiredMCPSet) { s.Claim.URL = "http://localhost:45001/mcp" },
		"userinfo":               func(s *RequiredMCPSet) { s.Claim.URL = "http://u:p@127.0.0.1:45001/mcp" },
		"query":                  func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:45002/mcp?a=1" },
		"fragment":               func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:45002/mcp#a" },
		"trailing slash":         func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:45002/mcp/" },
		"wrong path":             func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:45002/rpc" },
		"no port":                func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1/mcp" },
		"zero port":              func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:0/mcp" },
		"leading-zero port":      func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:0450/mcp" },
		"out-of-range port":      func(s *RequiredMCPSet) { s.Context.URL = "http://127.0.0.1:70000/mcp" },
	} {
		t.Run(name, func(t *testing.T) {
			set := scopedTestSet()
			mutate(&set)
			if err := set.Validate(); err == nil {
				t.Fatalf("Validate accepted a %s set", name)
			}
		})
	}
}

func TestRequiredMCPSetProtectedValues(t *testing.T) {
	set := scopedTestSet()
	want := [][]byte{
		[]byte("sha256:" + strings.Repeat("c", 64)),
		[]byte("Bearer sha256:" + strings.Repeat("c", 64)),
		[]byte("sha256:" + strings.Repeat("d", 64)),
		[]byte("Bearer sha256:" + strings.Repeat("d", 64)),
	}
	if got := set.ProtectedValues(); !reflect.DeepEqual(got, want) {
		t.Fatalf("protected values = %q, want both raw capabilities and both authorizations %q", got, want)
	}
	if got := (RequiredMCPSet{}).ProtectedValues(); len(got) != 0 {
		t.Fatalf("zero set protected values = %q, want none", got)
	}
}

func TestScopedMCPCapabilityDomainsAreDisjointAndRequestBound(t *testing.T) {
	requestDigest := scopedTestDigest(scopedTestRequest)
	claim, err := ClaimMCPCapability(requestDigest)
	if err != nil {
		t.Fatalf("ClaimMCPCapability: %v", err)
	}
	wantClaim := scopedTestDigest([]byte(fmt.Sprintf(`{"request_digest":%q,"schema":"vatc.claim-mcp-capability/v1"}`, requestDigest)))
	if claim != wantClaim {
		t.Fatalf("claim capability = %q, want %q over the exact canonical preimage", claim, wantClaim)
	}

	contextCapability, err := ContextMCPCapability(requestDigest, scopedTestProfileDigest, scopedTestWorkspaceID)
	if err != nil {
		t.Fatalf("ContextMCPCapability: %v", err)
	}
	// Amendment 003 narrowly supersedes only the Claude file and init rows: the
	// existing context capability preimage is unchanged, byte for byte.
	wantContext := scopedTestDigest([]byte(fmt.Sprintf(
		`{"profile_digest":%q,"request_digest":%q,"schema":"verdi.claude-mcp-capability/v1","workspace_id":%q}`,
		scopedTestProfileDigest, requestDigest, scopedTestWorkspaceID)))
	if contextCapability != wantContext {
		t.Fatalf("context capability = %q, want the unchanged Amendment 002 preimage digest %q", contextCapability, wantContext)
	}
	if claim == contextCapability {
		t.Fatal("claim and context capabilities collide; the two digest domains must stay disjoint")
	}

	// Every capability is bound to its exact request; no other digest reproduces it.
	other := scopedTestDigest([]byte("{\"schema\":\"verdi.context-execution-request/v1\",\"test\":\"beta\"}\n"))
	otherClaim, err := ClaimMCPCapability(other)
	if err != nil {
		t.Fatalf("ClaimMCPCapability(other): %v", err)
	}
	if otherClaim == claim {
		t.Fatal("claim capability is not bound to its request digest")
	}
	otherContext, err := ContextMCPCapability(other, scopedTestProfileDigest, scopedTestWorkspaceID)
	if err != nil {
		t.Fatalf("ContextMCPCapability(other): %v", err)
	}
	if otherContext == contextCapability {
		t.Fatal("context capability is not bound to its request digest")
	}

	for name, args := range map[string][3]string{
		"empty request":     {"", scopedTestProfileDigest, scopedTestWorkspaceID},
		"unprefixed digest": {strings.TrimPrefix(requestDigest, "sha256:"), scopedTestProfileDigest, scopedTestWorkspaceID},
		"bad profile":       {requestDigest, "sha256:zz", scopedTestWorkspaceID},
		"bad workspace":     {requestDigest, scopedTestProfileDigest, "../escape"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ContextMCPCapability(args[0], args[1], args[2]); err == nil {
				t.Fatalf("ContextMCPCapability accepted %s", name)
			}
		})
	}
	if _, err := ClaimMCPCapability("not-a-digest"); err == nil {
		t.Fatal("ClaimMCPCapability accepted a non-canonical request digest")
	}
}

func TestCanonicalRequestDigestFailsClosed(t *testing.T) {
	digest, err := CanonicalRequestDigest(scopedTestRequest)
	if err != nil || digest != scopedTestDigest(scopedTestRequest) {
		t.Fatalf("CanonicalRequestDigest = %q/%v, want the exact byte digest", digest, err)
	}
	for name, request := range map[string][]byte{
		"empty":            nil,
		"not an object":    []byte("[]\n"),
		"null":             []byte("null\n"),
		"trailing data":    append(append([]byte(nil), scopedTestRequest...), []byte("{}\n")...),
		"non-canonical":    []byte("{\"test\":\"alpha\",\"schema\":\"verdi.context-execution-request/v1\"}\n"),
		"missing final LF": []byte("{\"schema\":\"verdi.context-execution-request/v1\",\"test\":\"alpha\"}"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := CanonicalRequestDigest(request); err == nil {
				t.Fatalf("CanonicalRequestDigest accepted %s", name)
			}
		})
	}
}

func TestRequiredClaimMCPCrossMatchesTheInvocation(t *testing.T) {
	requestDigest := scopedTestDigest(scopedTestRequest)
	registration := ClaimMCPRegistration{
		Name: RequiredClaimMCPName, Type: RequiredMCPType, URL: "http://127.0.0.1:45001/mcp",
		Tools: []string{ToolClaimPaths}, RequestDigest: requestDigest,
	}
	claim, err := RequiredClaimMCP(registration, requestDigest)
	if err != nil {
		t.Fatalf("RequiredClaimMCP: %v", err)
	}
	capability, err := ClaimMCPCapability(requestDigest)
	if err != nil {
		t.Fatal(err)
	}
	if claim.Authorization != "Bearer "+capability {
		t.Fatalf("claim authorization = %q, want the locally derived capability", claim.Authorization)
	}

	// A registration minted for any other invocation is refused outright.
	stale := registration
	stale.RequestDigest = scopedTestDigest([]byte("other"))
	if _, err := RequiredClaimMCP(stale, requestDigest); err == nil {
		t.Fatal("RequiredClaimMCP accepted a registration bound to another request")
	}
	for name, mutate := range map[string]func(*ClaimMCPRegistration){
		"renamed":     func(r *ClaimMCPRegistration) { r.Name = "vatc-shadow" },
		"wrong tools": func(r *ClaimMCPRegistration) { r.Tools = []string{ToolGetFlightPlan} },
		"bad origin":  func(r *ClaimMCPRegistration) { r.URL = "http://example.invalid/mcp" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := registration
			mutate(&bad)
			if _, err := RequiredClaimMCP(bad, requestDigest); err == nil {
				t.Fatalf("RequiredClaimMCP accepted a %s registration", name)
			}
		})
	}
}

func TestStartScopedContextMCPServesOnlyItsOwnCapability(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	registration, terminals, closeMCP, err := StartScopedContextMCP(
		context.Background(), listener, scopedTestRequest,
		scopedTestProfileDigest, scopedTestWorkspaceID, &scopedContextTestHandler{},
	)
	if err != nil {
		t.Fatalf("StartScopedContextMCP: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = closeMCP(closeCtx)
	})
	if terminals == nil {
		t.Fatal("StartScopedContextMCP returned no terminal channel")
	}
	if registration.Name != RequiredContextMCPName ||
		registration.URL != "http://"+listener.Addr().String()+"/mcp" ||
		!reflect.DeepEqual(registration.Tools, []string{ToolGetFlightPlan, ToolRequestContext}) {
		t.Fatalf("context registration = %#v", registration)
	}

	requestDigest := scopedTestDigest(scopedTestRequest)
	claimCapability, err := ClaimMCPCapability(requestDigest)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	for name, authorization := range map[string]string{
		"own capability":    registration.Authorization,
		"claim capability":  "Bearer " + claimCapability,
		"bare capability":   registration.Capability(),
		"empty":             "",
		"unknown authority": "Bearer sha256:" + strings.Repeat("0", 64),
	} {
		t.Run(name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, registration.URL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
			if err != nil {
				t.Fatal(err)
			}
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}
			request.Header.Set("Content-Type", "application/json")
			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("post: %v", err)
			}
			defer response.Body.Close()
			wantOK := name == "own capability"
			if gotOK := response.StatusCode == http.StatusOK; gotOK != wantOK {
				t.Fatalf("%s status = %d, want ok=%v", name, response.StatusCode, wantOK)
			}
		})
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := closeMCP(nil); err == nil {
		t.Fatal("close accepted a nil context")
	}
	if err := closeMCP(closeCtx); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := closeMCP(closeCtx); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	if _, err := client.Post(registration.URL, "application/json", strings.NewReader("{}")); err == nil {
		t.Fatal("the listener still accepted a connection after close")
	}
}

func TestStartScopedContextMCPRejectsInvalidInputs(t *testing.T) {
	newListener := func(t *testing.T) net.Listener {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = listener.Close() })
		return listener
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for name, test := range map[string]struct {
		ctx       context.Context
		nilLis    bool
		request   []byte
		profile   string
		workspace string
	}{
		"nil context":       {ctx: nil, request: scopedTestRequest, profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
		"canceled context":  {ctx: canceled, request: scopedTestRequest, profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
		"nil listener":      {ctx: context.Background(), nilLis: true, request: scopedTestRequest, profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
		"non-canonical":     {ctx: context.Background(), request: []byte("{}"), profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
		"bad profile":       {ctx: context.Background(), request: scopedTestRequest, profile: "nope", workspace: scopedTestWorkspaceID},
		"bad workspace":     {ctx: context.Background(), request: scopedTestRequest, profile: scopedTestProfileDigest, workspace: ""},
		"empty request":     {ctx: context.Background(), profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
		"trailing document": {ctx: context.Background(), request: append(append([]byte(nil), scopedTestRequest...), '{', '}'), profile: scopedTestProfileDigest, workspace: scopedTestWorkspaceID},
	} {
		t.Run(name, func(t *testing.T) {
			var listener net.Listener
			if !test.nilLis {
				listener = newListener(t)
			}
			_, _, _, err := StartScopedContextMCP(test.ctx, listener, test.request, test.profile, test.workspace, &scopedContextTestHandler{})
			if err == nil {
				t.Fatalf("StartScopedContextMCP accepted %s", name)
			}
		})
	}
}

func TestSortedMCPNamesCanonicalizesEitherOrder(t *testing.T) {
	want := []string{RequiredClaimMCPName, RequiredContextMCPName}
	for _, observed := range [][]string{
		{RequiredClaimMCPName, RequiredContextMCPName},
		{RequiredContextMCPName, RequiredClaimMCPName},
	} {
		if got := SortedMCPNames(observed); !reflect.DeepEqual(got, want) {
			t.Fatalf("SortedMCPNames(%v) = %v, want %v", observed, got, want)
		}
	}
	// The input is never mutated in place.
	observed := []string{RequiredContextMCPName, RequiredClaimMCPName}
	_ = SortedMCPNames(observed)
	if observed[0] != RequiredContextMCPName {
		t.Fatalf("SortedMCPNames mutated its argument: %v", observed)
	}
}

// scopedContextTestHandler is an inert tool handler: these tests prosecute the
// capability boundary and lifecycle, not tool semantics.
type scopedContextTestHandler struct{}

func (*scopedContextTestHandler) Tools() []mcpserve.HandlerTool {
	return []mcpserve.HandlerTool{
		{Name: ToolGetFlightPlan, Description: "test", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`)},
	}
}

func (*scopedContextTestHandler) Call(context.Context, string, json.RawMessage) (mcpserve.HandlerCallResult, error) {
	return mcpserve.HandlerCallResult{Text: "{}"}, nil
}
