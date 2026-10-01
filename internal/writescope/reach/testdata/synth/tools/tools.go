// Package tools is a synthetic MCP server: one switch over the tool
// inventory's names and one over protocol methods.
package tools

import (
	"context"

	"example.com/synth/app"
)

// Server answers tool calls.
type Server struct{}

// Call dispatches one tool by name.
func (s *Server) Call(ctx context.Context, name string) error {
	switch name {
	case "write_tool":
		return app.Direct(ctx)
	case "read_tool":
		return app.ReadOnly(ctx)
	default:
		return nil
	}
}

// Method dispatches one protocol method.
func (s *Server) Method(ctx context.Context, method string) error {
	switch method {
	case "initialize", "ping":
		return nil
	}
	return s.Call(ctx, method)
}
