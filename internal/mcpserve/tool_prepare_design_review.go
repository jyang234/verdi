package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designapp"
)

// PrepareDesignReview implements the prepare_design_review tool (AC-8):
// derives AC-6's semantic review packet without changing governance
// state. The agent may prepare or explain this packet but this server has
// no tool that could mark it approved, accept the design, or merge
// anything (AC-6) — the human's PR merge remains the sole acceptance
// decision.
func (b *Backend) PrepareDesignReview(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	ref, errResult, ok := decodeRefArg("prepare_design_review", argsRaw)
	if !ok {
		return errResult
	}
	if ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return toolError("prepare_design_review: ref must be an unpinned whole spec ref")
	}

	app := designapp.NewService()
	result, appErr := app.PrepareDesignReview(ctx, b.Root, designapp.PrepareDesignReviewRequest{Spec: ref.String()})
	if appErr != nil {
		return toolErrorForDesignApp(appErr)
	}
	return toolJSON(result)
}
