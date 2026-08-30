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
// errors").
//
// The payload is designapp's own typed, versioned failure envelope rather
// than a prose line. MCP tool results carry no exit-code channel, so a
// text-only rendering would erase the verdict/operational classification
// the application core computed: two failures agreeing on code and detail
// but disagreeing on 0/1/2 would reach the agent as the same bytes, and a
// refusal it should correct would be indistinguishable from a breakage it
// should report (CO-1; CO-9 names "error classifications" as an adapter-
// conformance object in its own right). The CLI's exit code and this
// field are the same fact on two transports.
//
// The tool name is deliberately NOT part of the payload (and no longer a
// parameter): the caller named the tool it invoked, and keeping it out
// leaves the envelope comparable field-for-field with the CLI's own
// diagnostic line.
func toolErrorForDesignApp(err *designapp.Error) map[string]any {
	return designAppToolFailure(err.Failure())
}

// designAppToolFailure renders one typed failure envelope as a tool error
// — the single place this server turns a designapp classification into
// the wire, mirroring tool_experiment.go's experimentToolResult (typed
// projection preserved, isError set, never a JSON-RPC framing error).
func designAppToolFailure(failure designapp.Failure) map[string]any {
	rendered := toolJSON(failure)
	rendered["isError"] = true
	return rendered
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
