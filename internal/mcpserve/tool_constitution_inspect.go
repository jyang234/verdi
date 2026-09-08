package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/constitutionapp"
)

// ConstitutionInspect implements the constitution_inspect tool (Wave 6 Task
// 3): the whole accepted/proposed constitution — every source layer and the
// complete effective rule ledger, unflattened — over the server's own
// configured root. Beyond its own envelope version it takes no field: root is
// never caller-supplied (constitutionapp.InspectRequest's own doc comment).
//
// The arguments are decoded through constitutionapp's OWN
// DecodeInspectRequest, not a local anonymous struct, so the MCP and CLI
// surfaces accept exactly one request grammar — including the exact
// verdi.constitution-inspect-request/v1 envelope version every constitution
// request document must carry.
func (b *Backend) ConstitutionInspect(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) == 0 {
		argsRaw = json.RawMessage(`{}`)
	}
	req, err := constitutionapp.DecodeInspectRequest(argsRaw)
	if err != nil {
		return toolError("constitution_inspect: malformed arguments: " + err.Error())
	}
	app := constitutionapp.NewService()
	result, appErr := app.Inspect(ctx, b.Root, req)
	if appErr != nil {
		return toolErrorForConstitutionApp(appErr)
	}
	return toolJSON(result)
}
