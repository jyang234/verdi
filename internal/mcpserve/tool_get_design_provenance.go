package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designapp"
)

// GetDesignProvenance implements the get_design_provenance tool (AC-8):
// returns provenance only on this explicit request — never bundled into
// get_design_context or get_board (AC-4/AC-5).
func (b *Backend) GetDesignProvenance(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	ref, errResult, ok := decodeRefArg("get_design_provenance", argsRaw)
	if !ok {
		return errResult
	}
	if ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return toolError("get_design_provenance: ref must be an unpinned whole spec ref")
	}

	app := designapp.NewService()
	result, appErr := app.GetDesignProvenance(ctx, b.Root, designapp.GetDesignProvenanceRequest{Spec: ref.String()})
	if appErr != nil {
		return toolErrorForDesignApp(appErr)
	}
	return toolJSON(result)
}
