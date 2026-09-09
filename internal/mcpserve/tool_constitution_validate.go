package mcpserve

import (
	"context"
	"encoding/json"

	"github.com/jyang234/verdi/internal/constitutionapp"
)

// ConstitutionValidate implements the constitution_validate tool (Wave 6
// Task 3): strict-decodes and cross-validates the proposed constitution
// store over the server's own configured root, reporting the same
// three-valued proof outcome the CLI's `context constitution validate`
// returns. Beyond its own envelope version it takes no field, and it decodes
// through constitutionapp's own DecodeValidateRequest so both adapters accept
// exactly one request grammar.
func (b *Backend) ConstitutionValidate(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) == 0 {
		argsRaw = json.RawMessage(`{}`)
	}
	req, err := constitutionapp.DecodeValidateRequest(argsRaw)
	if err != nil {
		return toolError("constitution_validate: malformed arguments: " + err.Error())
	}
	app := constitutionapp.NewService()
	result, appErr := app.Validate(ctx, b.Root, req)
	if appErr != nil {
		return toolErrorForConstitutionApp(appErr)
	}
	return toolJSON(result)
}
