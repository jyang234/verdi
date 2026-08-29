// Shared bridge between mcpserve's tool_*.go files and
// internal/designapp.Service, used by all six ASD MCP tools
// (get_board plus the five new ones — Wave 6 Task 1). One error
// projection and one board-loader adapter live here rather than being
// repeated per tool file.
package mcpserve

import (
	"context"

	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/wallbadge"
	"github.com/jyang234/verdi/internal/workbench"
)

// toolErrorForDesignApp projects a *designapp.Error into this server's
// tool-error shape (map[string]any, isError: true — 05 §MCP server's own
// contract: "tool failures are MCP tool results ..., never protocol
// errors"). The verdict/operational Classification is not otherwise
// surfaced over the wire — MCP tool results carry no separate exit-code
// channel — but the message names the exact Code so a caller can still
// distinguish a refusal from an operational failure by text.
func toolErrorForDesignApp(toolName string, err *designapp.Error) map[string]any {
	return toolError(toolName + ": " + err.Code + ": " + err.Detail)
}

// backendBoardLoader adapts one Backend's Forge (or its absence) into
// designapp.BoardLoader, reproducing tool_get_board.go's own pre-Wave-6
// forge/review-feed construction exactly — the three I-1(b) states
// (silent no-forge, disclosed-unavailable, live) are this adapter's
// concern, never designapp's own (board.go's doc comment: "designapp
// never learns forge/review-feed concepts itself").
type backendBoardLoader struct {
	backend *Backend
}

func (l backendBoardLoader) LoadProjection(ctx context.Context, root, name string) (*workbench.BoardProjection, string, error) {
	var feed workbench.CommentFeed
	var superseLoader wallbadge.SupersessionCandidateLoader
	if l.backend.Forge != nil {
		feed = backendCommentFeed{f: l.backend.Forge, root: l.backend.Root}
		superseLoader = backendSupersessionLoader{f: l.backend.Forge, root: l.backend.Root}
	}
	return workbench.LoadProjection(ctx, root, name, feed, l.backend.ReviewUnavailable, superseLoader)
}
