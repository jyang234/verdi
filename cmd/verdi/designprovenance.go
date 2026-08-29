// verdi design provenance <spec-ref> (AC-8's get_design_provenance, CLI
// equivalent): returns the committed design-provenance sidecar only on
// this explicit request (AC-4/AC-8 — provenance never appears in
// `design board`/`design context`'s output), over
// designapp.Service.GetDesignProvenance.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/designapp"
)

func cmdDesignProvenance(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholder)
		fmt.Fprintln(stderr, "usage: verdi design provenance <spec-ref>")
		return 2
	}
	svc := designapp.NewService()
	result, appErr := svc.GetDesignProvenance(context.Background(), ".", designapp.GetDesignProvenanceRequest{Spec: args[0]})
	if appErr != nil {
		return renderDesignAppResult(stdout, stderr, "design provenance", nil, appErr)
	}
	return renderDesignAppResult(stdout, stderr, "design provenance", result, nil)
}
