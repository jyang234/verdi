// Package claude implements the sealed Claude Code adapter boundary.
package claude

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/sealedexec"
)

const claudeMCPConfigName = "claude-mcp.json"

// MCPConfig is the complete provider-visible scoped MCP configuration: the
// atomically written file plus the exact pair of required registrations it
// declares.
type MCPConfig struct {
	Path    string
	Servers sealedexec.RequiredMCPSet
}

// StartScopedMCP validates the ATC-owned claim registration, starts the
// parent-hosted verdi-context listener, and atomically writes Claude's scoped
// two-server MCP configuration beneath envRoot. The returned channel carries the
// first handler terminal of the run, observable only after its response frame
// was written. The returned close function is idempotent; callers invoke it only
// after the provider is reaped, and it shuts the listener down before removing
// the temporary configuration.
func StartScopedMCP(
	ctx context.Context,
	listener net.Listener,
	envRoot string,
	canonicalRequest []byte,
	profileDigest string,
	workspaceID string,
	claim sealedexec.RequiredMCP,
	handler mcpserve.Handler,
) (MCPConfig, <-chan *mcpserve.HandlerTerminal, func(context.Context) error, error) {
	if ctx == nil {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: nil context")
	}
	if err := ctx.Err(); err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: start scoped MCP: %w", err)
	}
	if !filepath.IsAbs(envRoot) || filepath.Clean(envRoot) != envRoot {
		return MCPConfig{}, nil, nil, errors.New("claude: start scoped MCP: environment root must be a clean absolute path")
	}
	// Amendment 003 §lifecycle: vatc is validated before the context listener
	// binds, so an unusable claim registration never leaves a bound socket.
	if err := claim.ValidateClaim(); err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: start scoped MCP: %w", err)
	}

	registration, terminals, closeContext, err := sealedexec.StartScopedContextMCP(
		ctx, listener, canonicalRequest, profileDigest, workspaceID, handler,
	)
	if err != nil {
		return MCPConfig{}, nil, nil, fmt.Errorf("claude: start scoped MCP: %w", err)
	}

	config := MCPConfig{
		Path:    filepath.Join(envRoot, claudeMCPConfigName),
		Servers: sealedexec.RequiredMCPSet{Claim: claim, Context: registration},
	}
	// Any post-bind prelaunch failure shuts down the exact listener it bound.
	fail := func(err error) (MCPConfig, <-chan *mcpserve.HandlerTerminal, func(context.Context) error, error) {
		return MCPConfig{}, nil, nil, errors.Join(err, closeContext(ctx))
	}
	if err := config.Servers.Validate(); err != nil {
		return fail(fmt.Errorf("claude: start scoped MCP: %w", err))
	}
	configBytes, err := encodeMCPConfig(config)
	if err != nil {
		return fail(err)
	}
	if err := atomicfile.Write(config.Path, configBytes, 0o600); err != nil {
		return fail(fmt.Errorf("claude: write scoped MCP config: %w", err))
	}

	var closeOnce sync.Once
	var closeErr error
	closeMCP := func(closeCtx context.Context) error {
		if closeCtx == nil {
			return errors.New("claude: shut down scoped MCP: nil context")
		}
		closeOnce.Do(func() {
			shutdownErr := closeContext(closeCtx)
			removeErr := os.Remove(config.Path)
			if errors.Is(removeErr, os.ErrNotExist) {
				removeErr = nil
			} else if removeErr != nil {
				removeErr = fmt.Errorf("claude: remove scoped MCP config: %w", removeErr)
			}
			closeErr = errors.Join(shutdownErr, removeErr)
		})
		return closeErr
	}

	return config, terminals, closeMCP, nil
}

// encodeMCPConfig emits Amendment 003's exact canonical two-row document plus
// one LF. Canonical key order sorts vatc before verdi-context and alwaysLoad,
// headers, type, url within each row.
func encodeMCPConfig(config MCPConfig) ([]byte, error) {
	type serverConfig struct {
		AlwaysLoad bool              `json:"alwaysLoad"`
		Headers    map[string]string `json:"headers"`
		Type       string            `json:"type"`
		URL        string            `json:"url"`
	}
	row := func(server sealedexec.RequiredMCP) serverConfig {
		return serverConfig{
			AlwaysLoad: true,
			Headers:    map[string]string{"Authorization": server.Authorization},
			Type:       sealedexec.RequiredMCPType,
			URL:        server.URL,
		}
	}
	document := struct {
		MCPServers map[string]serverConfig `json:"mcpServers"`
	}{
		MCPServers: map[string]serverConfig{
			config.Servers.Claim.Name:   row(config.Servers.Claim),
			config.Servers.Context.Name: row(config.Servers.Context),
		},
	}
	encoded, err := canonjson.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("claude: encode scoped MCP config: %w", err)
	}
	return encoded, nil
}
