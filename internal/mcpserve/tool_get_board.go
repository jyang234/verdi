package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designapp"
)

// GetBoard implements the get_board tool: the SAME deterministic board
// projection workbench.LoadProjection computes for `verdi serve`'s board
// page — element taxonomy, computed badges, mode-appropriate annotations —
// so agents work from what humans see rather than a second-hand summary
// (05 §MCP server's get_board row). Read-only: get_board never mutates
// anything. (add_annotation is no longer the server's only write tool —
// Wave 6 Task 1 added mutate_draft, the typed draft-mutation write path;
// see tooldefs.go's add_annotation row.)
//
// Wave 6 Task 1: routes through internal/designapp.Service.GetBoard
// (AC-8's own six-operation ASD surface, of which get_board is one) —
// composed here with a backendBoardLoader so this tool keeps its live
// review-feed disclosure (I-1(b), unrelated to ASD) instead of designapp's
// own no-forge production default. The underlying board projection
// algorithm is unchanged; only the call site moved.
func (b *Backend) GetBoard(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	ref, errResult, ok := decodeRefArg("get_board", argsRaw)
	if !ok {
		return errResult
	}
	if ref.Kind != artifact.KindSpec {
		return toolError("get_board: ref must name a spec (kind \"spec\"); got kind " + string(ref.Kind))
	}
	if ref.Object != "" {
		return toolError("get_board: ref must name a whole spec, not an object fragment")
	}
	if ref.Pinned() {
		return toolError("get_board: ref must be unpinned — the board always projects the current working tree, never a pinned historical commit")
	}

	app := designapp.NewService()
	app.Board = backendBoardLoader{backend: b}
	result, appErr := app.GetBoard(ctx, b.Root, designapp.GetBoardRequest{Spec: ref.String()})
	if appErr != nil {
		return toolErrorForDesignApp("get_board", appErr)
	}
	return toolJSON(result)
}
