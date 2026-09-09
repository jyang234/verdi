package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/constitutionapp"
)

// ConstitutionImpactReview implements the constitution_impact_review tool
// (Wave 6 Task 3): the accepted/proposed constitution diff plus complete
// exact-tree registered-consumer coverage, with caller targets retained only
// as supplemental previews, over the server's own configured root. It decodes
// its arguments through constitutionapp's own DecodeImpactReviewRequest — the
// exact same strict wire shape the CLI's `context constitution impact-review
// --request` reads — so both adapters accept byte-identical request documents.
func (b *Backend) ConstitutionImpactReview(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) == 0 {
		argsRaw = json.RawMessage(`{}`)
	}
	req, err := constitutionapp.DecodeImpactReviewRequest(argsRaw)
	if err != nil {
		return toolError("constitution_impact_review: malformed arguments: " + err.Error())
	}
	app := constitutionapp.NewService()
	result, appErr := app.ImpactReview(ctx, b.Root, req)
	if appErr != nil {
		return toolErrorForConstitutionApp(appErr)
	}
	return toolJSON(result)
}
