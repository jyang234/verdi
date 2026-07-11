package mcpserve

import (
	"fmt"

	"github.com/OWNER/verdi/internal/canonjson"
)

// toolText wraps text as an MCP tool result (one text content item),
// matching groundwork's own toolText/the S4 spike's wire.ToolText.
func toolText(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// toolError wraps msg as a FAILED tool result (isError: true) — a tool
// failure is an MCP tool result the calling agent can read and correct
// from, never a JSON-RPC protocol error (05 §MCP server's own contract,
// matching groundwork's convention).
func toolError(msg string) map[string]any {
	r := toolText(msg)
	r["isError"] = true
	return r
}

// toolJSON renders v (a tool's structured result) as canonical JSON
// (internal/canonjson: sorted keys, deterministic — I-18's discipline,
// applied here so two calls against unchanged state answer with
// byte-identical text) inside a toolText content item. A marshal failure
// (only reachable for a genuinely unmarshalable Go value — never for the
// plain structs this package's tools return) becomes a toolError rather
// than a panic or a malformed response.
func toolJSON(v any) map[string]any {
	data, err := canonjson.Marshal(v)
	if err != nil {
		return toolError(fmt.Sprintf("mcpserve: rendering tool result: %v", err))
	}
	return toolText(string(data))
}
