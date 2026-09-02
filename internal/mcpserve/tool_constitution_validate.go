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
// returns. It takes no fields.
func (b *Backend) ConstitutionValidate(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) != 0 {
		if err := strictUnmarshal(argsRaw, &struct{}{}); err != nil {
			return toolError("constitution_validate: malformed arguments: " + err.Error())
		}
	}
	app := constitutionapp.NewService()
	result, appErr := app.Validate(ctx, b.Root, constitutionapp.ValidateRequest{})
	if appErr != nil {
		return toolErrorForConstitutionApp(appErr)
	}
	return toolJSON(result)
}
