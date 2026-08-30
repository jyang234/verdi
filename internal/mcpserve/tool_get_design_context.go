package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designapp"
)

// GetDesignContext implements the get_design_context tool (AC-8): the
// bounded, authoritative material needed to assist with one draft (AC-5)
// — never a second, opaque harness-memory context. child_stories is an
// optional list of already-known story refs the caller explicitly wants
// resolved (designapp's own ChildStories field; see
// internal/designapp/context.go's ChildStory doc comment for why this
// direction, unlike the parent feature, has no automatic back-index).
func (b *Backend) GetDesignContext(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Ref          string   `json:"ref"`
		ChildStories []string `json:"child_stories,omitempty"`
	}
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_design_context: malformed arguments: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_design_context: ref is required")
	}
	ref, err := artifact.ParseRef(args.Ref)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return toolError("get_design_context: ref must be an unpinned whole spec ref")
	}

	app := designapp.NewService()
	result, appErr := app.GetDesignContext(ctx, b.Root, designapp.GetDesignContextRequest{Spec: ref.String(), ChildStories: args.ChildStories})
	if appErr != nil {
		return toolErrorForDesignApp(appErr)
	}
	return toolJSON(result)
}
