// Package claude implements the sealed Claude Code adapter boundary.
package claude

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
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/mcpserve"
)

const (
	claudeMCPConfigName       = "claude-mcp.json"
	claudeMCPCapabilitySchema = "verdi.claude-mcp-capability/v1"
	claudeMCPServerName       = "verdi-context"
)

var claudeMCPDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// MCPConfig is the complete provider-visible scoped MCP configuration.
type MCPConfig struct {
	Path          string
	URL           string
	Authorization string
}

// StartScopedMCP starts the injected loopback listener and atomically writes
// Claude's sole MCP server configuration beneath envRoot. The returned channel
// carries the first handler terminal of the run, observable only after its
// response frame was written. The returned close function is idempotent;
// callers invoke it only after the provider is reaped.
func StartScopedMCP(
	ctx context.Context,
	listener net.Listener,
	envRoot string,
	canonicalRequest []byte,
	profileDigest string,
	workspaceID string,
	handler mcpserve.Handler,
) (MCPConfig, <-chan *mcpserve.HandlerTerminal, func(context.Context) error, error) {
	if ctx == nil {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: nil context")
	}
	if err := ctx.Err(); err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: start scoped MCP: %w", err)
	}
	if listener == nil {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: listener is required")
	}
	if !filepath.IsAbs(envRoot) || filepath.Clean(envRoot) != envRoot {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: environment root must be a clean absolute path")
	}
	requestDigest, err := canonicalExecutionRequestDigest(canonicalRequest)
	if err != nil {
		return MCPConfig{}, nil, nil, err
	}
	if !claudeMCPDigestRE.MatchString(profileDigest) {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: profile digest must be a canonical sha256 digest")
	}
	if !execworkspace.ValidWorkspaceID(workspaceID) {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: invalid workspace id")
	}
	address, err := scopedMCPAddress(listener)
	if err != nil {
		return MCPConfig{}, nil, nil, err
	}
	token, err := scopedMCPCapability(requestDigest, profileDigest, workspaceID)
	if err != nil {
		return MCPConfig{}, nil, nil, err
	}
	httpHandler, terminals, err := mcpserve.NewHTTPHandler(token, handler)
	if err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: start scoped MCP: %w", err)
	}

	config := MCPConfig{
		Path:          filepath.Join(envRoot, claudeMCPConfigName),
		URL:           "http://" + address + "/mcp",
		Authorization: "Bearer " + token,
	}
	configBytes, err := encodeMCPConfig(config)
	if err != nil {
		return MCPConfig{}, nil, nil, err
	}
	if err := atomicfile.Write(config.Path, configBytes, 0o600); err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: write scoped MCP config: %w", err)
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
			return errors.New("claude: shut down scoped MCP: nil context")
		}
		closeOnce.Do(func() {
			shutdownErr := server.Shutdown(closeCtx)
			if shutdownErr != nil {
				shutdownErr = fmt.Errorf("claude: shut down scoped MCP: %w", errors.Join(shutdownErr, server.Close()))
			}
			serveErr := <-serveDone
			if errors.Is(serveErr, http.ErrServerClosed) {
				serveErr = nil
			} else if serveErr != nil {
				serveErr = fmt.Errorf("claude: serve scoped MCP: %w", serveErr)
			}
			removeErr := os.Remove(config.Path)
			if errors.Is(removeErr, os.ErrNotExist) {
				removeErr = nil
			} else if removeErr != nil {
				removeErr = fmt.Errorf("claude: remove scoped MCP config: %w", removeErr)
			}
			closeErr = errors.Join(shutdownErr, serveErr, removeErr)
		})
		return closeErr
	}

	return config, terminals, closeMCP, nil
}

func canonicalExecutionRequestDigest(canonicalRequest []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(canonicalRequest))
	decoder.UseNumber()
	var request map[string]any
	if err := decoder.Decode(&request); err != nil {
		return "", fmt.Errorf("claude: decode canonical execution request: %w", err)
	}
	if request == nil {
		return "", errors.New("claude: canonical execution request must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return "", errors.New("claude: canonical execution request has trailing JSON data")
		}
		return "", fmt.Errorf("claude: decode canonical execution request trailing data: %w", err)
	}
	canonical, err := canonjson.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("claude: encode canonical execution request: %w", err)
	}
	if !bytes.Equal(canonicalRequest, canonical) {
		return "", errors.New("claude: execution request is not byte-canonical")
	}
	return mcpDigestBytes(canonicalRequest), nil
}

func scopedMCPAddress(listener net.Listener) (string, error) {
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port <= 0 || !address.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		return "", errors.New("claude: scoped MCP listener must be bound to IPv4 loopback with a positive port")
	}
	return fmt.Sprintf("127.0.0.1:%d", address.Port), nil
}

func scopedMCPCapability(requestDigest, profileDigest, workspaceID string) (string, error) {
	preimage, err := canonjson.Marshal(struct {
		Schema        string `json:"schema"`
		RequestDigest string `json:"request_digest"`
		ProfileDigest string `json:"profile_digest"`
		WorkspaceID   string `json:"workspace_id"`
	}{
		Schema:        claudeMCPCapabilitySchema,
		RequestDigest: requestDigest,
		ProfileDigest: profileDigest,
		WorkspaceID:   workspaceID,
	})
	if err != nil {
		return "", fmt.Errorf("claude: encode scoped MCP capability: %w", err)
	}
	return mcpDigestBytes(bytes.TrimSuffix(preimage, []byte{'\n'})), nil
}

func encodeMCPConfig(config MCPConfig) ([]byte, error) {
	type serverConfig struct {
		Type       string            `json:"type"`
		URL        string            `json:"url"`
		Headers    map[string]string `json:"headers"`
		AlwaysLoad bool              `json:"alwaysLoad"`
	}
	document := struct {
		MCPServers map[string]serverConfig `json:"mcpServers"`
	}{
		MCPServers: map[string]serverConfig{
			claudeMCPServerName: {
				Type:       "http",
				URL:        config.URL,
				Headers:    map[string]string{"Authorization": config.Authorization},
				AlwaysLoad: true,
			},
		},
	}
	encoded, err := canonjson.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("claude: encode scoped MCP config: %w", err)
	}
	return encoded, nil
}

func mcpDigestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
