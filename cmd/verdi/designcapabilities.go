// verdi design capabilities <spec-ref> (AC-8's get_design_capabilities,
// CLI equivalent): declares the active schema versions, checkout/branch/
// HEAD/spec identity, policy digest/mode, permitted operations, and fixed
// provenance/review/direct-Markdown posture (AC-3), over
// designapp.Service.GetDesignCapabilities.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/designapp"
)

func cmdDesignCapabilities(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholder)
		fmt.Fprintln(stderr, "usage: verdi design capabilities <spec-ref>")
		return 2
	}
	svc := designapp.NewService()
	result, appErr := svc.GetDesignCapabilities(context.Background(), ".", designapp.GetDesignCapabilitiesRequest{Spec: args[0]})
	if appErr != nil {
		return renderDesignAppResult(stdout, stderr, "design capabilities", nil, appErr)
	}
	return renderDesignAppResult(stdout, stderr, "design capabilities", result, nil)
}
