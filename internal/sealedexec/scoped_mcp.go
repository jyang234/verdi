package sealedexec

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"sync"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/mcpserve"
)

// Amendment 003: every sealed session receives exactly two separately owned
// required loopback HTTP MCP registrations. verdi-context is Verdi-owned and
// parent-hosted; vatc is ATC-owned and only resolved over FD 3. Neither proxies
// nor implements the other's tools.
const (
	// RequiredContextMCPName is the Verdi-owned registration name.
	RequiredContextMCPName = "verdi-context"
	// RequiredClaimMCPName is the ATC-owned registration name.
	RequiredClaimMCPName = "vatc"
	// RequiredMCPType is the only admitted transport for either registration.
	RequiredMCPType = "http"
	// ToolClaimPaths is the sole tool the ATC-owned registration exposes.
	ToolClaimPaths = "claim_paths"

	// contextMCPCapabilitySchema is unchanged from Amendment 002: the existing
	// context capability preimage stays byte-for-byte identical.
	contextMCPCapabilitySchema = "verdi.claude-mcp-capability/v1"
	// claimMCPCapabilitySchema is the disjoint VATC capability domain. The two
	// digest domains are never compared or substituted.
	claimMCPCapabilitySchema = "vatc.claim-mcp-capability/v1"

	// authorizationPrefix is the exact scheme both registrations carry.
	authorizationPrefix = "Bearer "
)

var (
	scopedMCPDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	// requiredMCPURLRE admits exactly one IPv4-loopback origin, one positive
	// decimal port with no leading zero, and the exact /mcp path. Being fully
	// anchored it also rejects userinfo, query, fragment, and every other URL
	// component.
	requiredMCPURLRE = regexp.MustCompile(`^http://127\.0\.0\.1:([1-9][0-9]{0,4})/mcp$`)
)

// requiredContextTools is the exact Verdi-owned catalogue.
func requiredContextTools() []string { return []string{ToolGetFlightPlan, ToolRequestContext} }

// requiredClaimTools is the exact ATC-owned catalogue.
func requiredClaimTools() []string { return []string{ToolClaimPaths} }

// RequiredMCP is one required, separately owned loopback HTTP MCP registration.
// Authorization is the complete header value, never only its capability.
type RequiredMCP struct {
	Name          string
	URL           string
	Authorization string
	Tools         []string
}

// Capability returns the raw capability carried by this registration's
// authorization, without its scheme.
func (m RequiredMCP) Capability() string {
	if len(m.Authorization) <= len(authorizationPrefix) {
		return ""
	}
	return m.Authorization[len(authorizationPrefix):]
}

// ValidateClaim proves one ATC-owned claim registration in isolation, before
// any parent-hosted listener is bound.
func (m RequiredMCP) ValidateClaim() error {
	return m.validate(RequiredClaimMCPName, requiredClaimTools())
}

// validate proves one registration against its fixed name and catalogue.
func (m RequiredMCP) validate(name string, tools []string) error {
	if m.Name != name {
		return fmt.Errorf("sealedexec: required MCP name %q, want %q", m.Name, name)
	}
	if err := ValidateRequiredMCPURL(m.URL); err != nil {
		return fmt.Errorf("sealedexec: required MCP %q: %w", name, err)
	}
	capability := m.Capability()
	if capability == "" || m.Authorization != authorizationPrefix+capability || !scopedMCPDigestRE.MatchString(capability) {
		return fmt.Errorf("sealedexec: required MCP %q authorization must be %q plus a canonical sha256 capability", name, authorizationPrefix)
	}
	if len(m.Tools) != len(tools) {
		return fmt.Errorf("sealedexec: required MCP %q declares %d tools, want %d", name, len(m.Tools), len(tools))
	}
	for i, tool := range tools {
		if m.Tools[i] != tool {
			return fmt.Errorf("sealedexec: required MCP %q tool %d = %q, want %q", name, i, m.Tools[i], tool)
		}
	}
	return nil
}

// RequiredMCPSet is the exact pair of required registrations handed to one
// sealed provider. No third server, ambient source, or provider-selected
// registration is ever admitted.
type RequiredMCPSet struct {
	Claim   RequiredMCP
	Context RequiredMCP
}

// Validate proves exactly two separately owned registrations with disjoint
// catalogues, distinct origins, and distinct invocation-scoped capabilities.
func (s RequiredMCPSet) Validate() error {
	if err := s.Claim.validate(RequiredClaimMCPName, requiredClaimTools()); err != nil {
		return err
	}
	if err := s.Context.validate(RequiredContextMCPName, requiredContextTools()); err != nil {
		return err
	}
	if s.Claim.URL == s.Context.URL {
		return errors.New("sealedexec: required MCP registrations share one origin")
	}
	if s.Claim.Authorization == s.Context.Authorization {
		return errors.New("sealedexec: required MCP registrations share one capability")
	}
	for _, claimed := range s.Claim.Tools {
		for _, hosted := range s.Context.Tools {
			if claimed == hosted {
				return fmt.Errorf("sealedexec: required MCP catalogues both declare %q", claimed)
			}
		}
	}
	return nil
}

// ProtectedValues returns both raw capability strings and both complete
// authorization strings. The set is installed before any provider detail is
// emitted, so neither value can reach a projection, event, or receipt.
func (s RequiredMCPSet) ProtectedValues() [][]byte {
	values := []string{
		s.Claim.Capability(), s.Claim.Authorization,
		s.Context.Capability(), s.Context.Authorization,
	}
	protected := make([][]byte, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		protected = append(protected, []byte(value))
	}
	return protected
}

// ValidateRequiredMCPURL proves one IPv4-loopback origin with a single positive
// in-range decimal port and the exact /mcp path.
func ValidateRequiredMCPURL(raw string) error {
	match := requiredMCPURLRE.FindStringSubmatch(raw)
	if match == nil {
		return fmt.Errorf("url %q is not an exact http://127.0.0.1:<port>/mcp loopback origin", raw)
	}
	port, err := strconv.Atoi(match[1])
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("url %q does not carry one positive in-range port", raw)
	}
	return nil
}

// CanonicalRequestDigest strict-decodes, canonically re-encodes, and
// byte-compares the sealed request before returning its identity digest. Both
// trusted parents derive RQ this way from the same bytes.
func CanonicalRequestDigest(canonicalRequest []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonicalRequest))
	decoder.UseNumber()
	var request map[string]any
	if err := decoder.Decode(&request); err != nil {
		return "", fmt.Errorf("sealedexec: decode canonical execution request: %w", err)
	}
	if request == nil {
		return "", errors.New("sealedexec: canonical execution request must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", errors.New("sealedexec: canonical execution request has trailing JSON data")
		}
		return "", fmt.Errorf("sealedexec: decode canonical execution request trailing data: %w", err)
	}
	canonical, err := canonjson.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("sealedexec: encode canonical execution request: %w", err)
	}
	if !bytes.Equal(canonicalRequest, canonical) {
		return "", errors.New("sealedexec: execution request is not byte-canonical")
	}
	return scopedMCPDigest(canonicalRequest), nil
}

// ClaimMCPCapability derives the invocation-scoped VATC capability locally. It
// is accepted for no different request digest, never returned by the
// controller, never reused, and emitted onto no public or durable wire.
func ClaimMCPCapability(requestDigest string) (string, error) {
	if !scopedMCPDigestRE.MatchString(requestDigest) {
		return "", errors.New("sealedexec: claim capability requires a canonical request digest")
	}
	preimage, err := canonjson.Marshal(struct {
		RequestDigest string `json:"request_digest"`
		Schema        string `json:"schema"`
	}{RequestDigest: requestDigest, Schema: claimMCPCapabilitySchema})
	if err != nil {
		return "", fmt.Errorf("sealedexec: encode claim MCP capability: %w", err)
	}
	return scopedMCPDigest(bytes.TrimSuffix(preimage, []byte{'\n'})), nil
}

// ContextMCPCapability derives the Verdi context capability. Its preimage is
// unchanged from Amendment 002, so existing capability bytes stay identical.
func ContextMCPCapability(requestDigest, profileDigest, workspaceID string) (string, error) {
	if !scopedMCPDigestRE.MatchString(requestDigest) {
		return "", errors.New("sealedexec: context capability requires a canonical request digest")
	}
	if !scopedMCPDigestRE.MatchString(profileDigest) {
		return "", errors.New("sealedexec: context capability requires a canonical profile digest")
	}
	if !execworkspace.ValidWorkspaceID(workspaceID) {
		return "", errors.New("sealedexec: context capability requires a valid workspace id")
	}
	preimage, err := canonjson.Marshal(struct {
		ProfileDigest string `json:"profile_digest"`
		RequestDigest string `json:"request_digest"`
		Schema        string `json:"schema"`
		WorkspaceID   string `json:"workspace_id"`
	}{
		ProfileDigest: profileDigest,
		RequestDigest: requestDigest,
		Schema:        contextMCPCapabilitySchema,
		WorkspaceID:   workspaceID,
	})
	if err != nil {
		return "", fmt.Errorf("sealedexec: encode context MCP capability: %w", err)
	}
	return scopedMCPDigest(bytes.TrimSuffix(preimage, []byte{'\n'})), nil
}

// RequiredClaimMCP builds the claim registration from the controller-resolved
// registration and the locally derived capability. The controller never returns
// a bearer, so the capability is never carried over FD 3.
func RequiredClaimMCP(registration ClaimMCPRegistration, requestDigest string) (RequiredMCP, error) {
	if registration.RequestDigest != requestDigest {
		return RequiredMCP{}, errors.New("sealedexec: claim registration digest contradicts this invocation")
	}
	capability, err := ClaimMCPCapability(requestDigest)
	if err != nil {
		return RequiredMCP{}, err
	}
	claim := RequiredMCP{
		Name:          registration.Name,
		URL:           registration.URL,
		Authorization: authorizationPrefix + capability,
		Tools:         append([]string(nil), registration.Tools...),
	}
	if err := claim.validate(RequiredClaimMCPName, requiredClaimTools()); err != nil {
		return RequiredMCP{}, err
	}
	return claim, nil
}

// StartScopedContextMCP starts the parent-hosted context listener shared by both
// provider adapters. The returned channel carries the first handler terminal of
// the run, observable only after its response frame was written. The returned
// close function is idempotent and targeted; callers invoke it only after the
// provider is reaped.
func StartScopedContextMCP(
	ctx context.Context,
	listener net.Listener,
	canonicalRequest []byte,
	profileDigest string,
	workspaceID string,
	handler mcpserve.Handler,
) (RequiredMCP, <-chan *mcpserve.HandlerTerminal, func(context.Context) error, error) {
	if ctx == nil {
		return RequiredMCP{}, nil, nil, errors.New("sealedexec: start scoped context MCP: nil context")
	}
	if err := ctx.Err(); err != nil {
		return RequiredMCP{}, nil, nil, fmt.Errorf("sealedexec: start scoped context MCP: %w", err)
	}
	if listener == nil {
		return RequiredMCP{}, nil, nil, errors.New("sealedexec: start scoped context MCP: listener is required")
	}
	requestDigest, err := CanonicalRequestDigest(canonicalRequest)
	if err != nil {
		return RequiredMCP{}, nil, nil, err
	}
	address, err := scopedMCPAddress(listener)
	if err != nil {
		return RequiredMCP{}, nil, nil, err
	}
	capability, err := ContextMCPCapability(requestDigest, profileDigest, workspaceID)
	if err != nil {
		return RequiredMCP{}, nil, nil, err
	}
	httpHandler, terminals, err := mcpserve.NewHTTPHandler(capability, handler)
	if err != nil {
		return RequiredMCP{}, nil, nil, fmt.Errorf("sealedexec: start scoped context MCP: %w", err)
	}

	registration := RequiredMCP{
		Name:          RequiredContextMCPName,
		URL:           "http://" + address + "/mcp",
		Authorization: authorizationPrefix + capability,
		Tools:         requiredContextTools(),
	}
	if err := registration.validate(RequiredContextMCPName, requiredContextTools()); err != nil {
		return RequiredMCP{}, nil, nil, err
	}

	server := &http.Server{Handler: httpHandler}
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(listener)
	}()

	var closeOnce sync.Once
	var closeErr error
	closeMCP := func(closeCtx context.Context) error {
		if closeCtx == nil {
			return errors.New("sealedexec: shut down scoped context MCP: nil context")
		}
		closeOnce.Do(func() {
			shutdownErr := server.Shutdown(closeCtx)
			if shutdownErr != nil {
				shutdownErr = fmt.Errorf("sealedexec: shut down scoped context MCP: %w", errors.Join(shutdownErr, server.Close()))
			}
			serveErr := <-serveDone
			if errors.Is(serveErr, http.ErrServerClosed) {
				serveErr = nil
			} else if serveErr != nil {
				serveErr = fmt.Errorf("sealedexec: serve scoped context MCP: %w", serveErr)
			}
			closeErr = errors.Join(shutdownErr, serveErr)
		})
		return closeErr
	}

	return registration, terminals, closeMCP, nil
}

// SortedMCPNames returns the registration names in canonical order, so a
// provider inventory observed in either order projects and digests identically.
func SortedMCPNames(names []string) []string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	return sorted
}

func scopedMCPAddress(listener net.Listener) (string, error) {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port <= 0 || !address.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		return "", errors.New("sealedexec: scoped MCP listener must be bound to IPv4 loopback with a positive port")
	}
	return fmt.Sprintf("127.0.0.1:%d", address.Port), nil
}

func scopedMCPDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
