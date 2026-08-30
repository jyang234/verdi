package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designapp"
)

// GetDesignCapabilities implements the get_design_capabilities tool
// (AC-8): the active schema versions, checkout/branch/HEAD/spec identity,
// policy digest/mode, permitted operations, and fixed provenance/review/
// direct-Markdown posture (AC-3).
func (b *Backend) GetDesignCapabilities(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	ref, errResult, ok := decodeRefArg("get_design_capabilities", argsRaw)
	if !ok {
		return errResult
	}
	if ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return toolError("get_design_capabilities: ref must be an unpinned whole spec ref")
	}

	app := designapp.NewService()
	result, appErr := app.GetDesignCapabilities(ctx, b.Root, designapp.GetDesignCapabilitiesRequest{Spec: ref.String()})
	if appErr != nil {
		return toolErrorForDesignApp(appErr)
	}
	return toolJSON(result)
}
