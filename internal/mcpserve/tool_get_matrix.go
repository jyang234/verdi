package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/matrixprojection"
)

// GetMatrix implements the get_matrix tool through the same typed story-or-
// feature projector as CLI text and CLI JSON. It never shells out.
func (b *Backend) GetMatrix(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Story   string `json:"story"`
		Preview bool   `json:"preview"`
	}
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_matrix: malformed arguments: " + err.Error())
	}
	if args.Story == "" {
		// vocab:identity — MCP tool ARGUMENT name (wire schema)
		return toolError("get_matrix: story is required")
	}

	projection, err := matrixprojection.Project(ctx, b.Root, args.Story, args.Preview, nil)
	if err != nil {
		return toolError("get_matrix: " + err.Error())
	}
	return toolJSON(projection.Record)
}
