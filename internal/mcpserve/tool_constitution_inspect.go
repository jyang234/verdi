package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/constitutionapp"
)

// ConstitutionInspect implements the constitution_inspect tool (Wave 6 Task
// 3): the whole accepted/proposed constitution — every source layer and the
// complete effective rule ledger, unflattened — over the server's own
// configured root. It takes no arguments beyond the empty object: root is
// never caller-supplied (constitutionapp.InspectRequest's own doc comment).
func (b *Backend) ConstitutionInspect(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	// This tool takes no fields; an omitted arguments object and an
	// explicit {} are both accepted, but any other value (including a
	// caller-supplied, unknown field) fails closed.
	if len(argsRaw) != 0 {
		if err := strictUnmarshal(argsRaw, &struct{}{}); err != nil {
			return toolError("constitution_inspect: malformed arguments: " + err.Error())
		}
	}
	app := constitutionapp.NewService()
	result, appErr := app.Inspect(ctx, b.Root, constitutionapp.InspectRequest{})
	if appErr != nil {
		return toolErrorForConstitutionApp(appErr)
	}
	return toolJSON(result)
}
