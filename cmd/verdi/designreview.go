// verdi design review <spec-ref> (AC-7's own literal example — "run
// `verdi design review <spec-ref>` to derive the semantic packet" — and
// AC-8's prepare_design_review): derives AC-6's semantic review packet
// without changing governance state, over
// designapp.Service.PrepareDesignReview. It never accepts, approves, or
// merges anything — this verb, like every ASD adapter, has no such
// operation to call (AC-6: the human's PR merge remains the sole
// acceptance decision).
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/designapp"
)

func cmdDesignReview(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholder)
		fmt.Fprintln(stderr, "usage: verdi design review <spec-ref>")
		return 2
	}
	svc := designapp.NewService()
	result, appErr := svc.PrepareDesignReview(context.Background(), ".", designapp.PrepareDesignReviewRequest{Spec: args[0]})
	if appErr != nil {
		return renderDesignAppResult(stdout, stderr, "design review", nil, appErr)
	}
	return renderDesignAppResult(stdout, stderr, "design review", result, nil)
}
