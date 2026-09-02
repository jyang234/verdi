// Shared bridge between mcpserve's tool_constitution_*.go files and
// internal/constitutionapp.Service, mirroring designappbridge.go's own
// toolErrorForDesignApp precedent: one error projection lives here rather
// than being repeated per tool file.
//
// This server registers only three constitution tools — constitution_inspect,
// constitution_validate, and constitution_impact_review — the read/
// validation/review projection Wave 6 Task 3 permits over MCP. There is no
// constitution_propose or constitution_submit_preparation tool at all: MCP
// "structurally refuses commit, submission, approval, exemption ownership,
// and semantic disposition before store access" by never registering a
// handler that could reach them, never by a runtime guard inside one.
package mcpserve

import "github.com/jyang234/verdi/internal/constitutionapp"

// toolErrorForConstitutionApp projects a *constitutionapp.Error into this
// server's tool-error shape (map[string]any, isError: true), mirroring
// toolErrorForDesignApp exactly.
func toolErrorForConstitutionApp(err *constitutionapp.Error) map[string]any {
	rendered := toolJSON(err.Failure())
	rendered["isError"] = true
	return rendered
}
