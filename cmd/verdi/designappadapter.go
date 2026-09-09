// Shared rendering for the five new read-only ASD CLI subcommands
// (design board|context|capabilities|provenance|review, Wave 6 Task 1) —
// designapp's five non-mutation operations. AC-8 names these as MCP
// tools and says "the CLI exposes equivalent structured commands for
// harnesses that prefer subprocesses"; each subcommand is a thin adapter
// over the exact same internal/designapp.Service method internal/mcpserve's
// matching tool_*.go file calls, so this file's one render helper is the
// CLI half of the conformance pairing (internal/designapp/conformance_test.go
// proves the two produce equivalent typed results for equivalent inputs).
//
// Every subcommand prints one line of this store's canonical JSON
// (internal/canonjson: sorted keys, deterministic) on success and exits
// 0; a *designapp.Error's own Classification decides 1 (verdict) or 2
// (operational), matching every other verb's 0/1/2 contract. Kept in its
// own file per the lint.go/sync.go/matrix.go/dex.go/journey.go convention
// rather than folded into design.go itself.
package main

import (
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designapp"
)

// renderDesignAppResult renders exactly one of result or appErr — never
// both, never neither — to stdout/stderr and returns the fixed CLI exit
// code. verb is the exact usage-line prefix ("design board", "design
// context", ...) every diagnostic line in this subcommand family shares.
func renderDesignAppResult(stdout, stderr io.Writer, verb string, result any, appErr *designapp.Error) int {
	if appErr != nil {
		fmt.Fprintf(stderr, "%s: %s: %s\n", verb, appErr.Code, appErr.Detail)
		return appErr.ExitCode()
	}
	encoded, err := canonjson.Marshal(result)
	if err != nil {
		fmt.Fprintf(stderr, "%s: result-invalid: encoding result: %v\n", verb, err)
		return 2
	}
	if _, err := stdout.Write(encoded); err != nil {
		fmt.Fprintf(stderr, "%s: io-failure: writing result: %v\n", verb, err)
		return 2
	}
	return 0
}
