package mcpserve

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/workbench"
)

// boardResult is get_board's result shape: workbench.BoardProjection
// marshaled exactly as computed — get_board never reimplements the
// projection (05 §MCP server's get_board row) — plus, as its own top-level
// field, the I-1(b) review-population disclosure (the review_unavailable
// field pattern from commit 1348e79, mirroring list_annotations): present
// only when a CONFIGURED forge could not be consulted, absent both when no
// forge is configured (silent, legitimate) and when the review feed is
// live and reachable.
type boardResult struct {
	*workbench.BoardProjection
	ReviewUnavailable string `json:"review_unavailable,omitempty"`
}

// GetBoard implements the get_board tool: the SAME deterministic board
// projection workbench.LoadProjection computes for `verdi serve`'s board
// page — element taxonomy, computed badges, mode-appropriate annotations —
// so agents work from what humans see rather than a second-hand summary
// (05 §MCP server's get_board row). Read-only: get_board never mutates
// anything; add_annotation stays the only write tool.
func (b *Backend) GetBoard(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := json.Unmarshal(argsRaw, &args); err != nil {
		return toolError("get_board: malformed arguments: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_board: ref is required")
	}

	ref, err := artifact.ParseRef(args.Ref)
	if err != nil {
		return toolError("get_board: " + err.Error())
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

	// Three I-1(b) states, exactly as list_annotations' review population
	// distinguishes them (review.go's doc comment): no forge configured
	// (b.Forge nil, b.ReviewUnavailable "") is silent; a configured forge
	// with no live adapter (b.ReviewUnavailable set) is disclosed; a live
	// forge is used to build the review-mode comment feed.
	var feed workbench.CommentFeed
	if b.Forge != nil {
		feed = backendCommentFeed{f: b.Forge, root: b.Root}
	}

	proj, reviewNotice, err := workbench.LoadProjection(ctx, b.Root, ref.Name, feed, b.ReviewUnavailable)
	if errors.Is(err, workbench.ErrBoardNotFound) {
		return toolError("get_board: no such spec board: " + args.Ref)
	}
	if err != nil {
		return toolError("get_board: " + err.Error())
	}

	return toolJSON(boardResult{BoardProjection: proj, ReviewUnavailable: reviewNotice})
}
