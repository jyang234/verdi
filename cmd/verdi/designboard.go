// verdi design board <spec-ref> (AC-8's get_board, CLI equivalent):
// prints the same deterministic board projection MCP's get_board tool and
// `verdi serve`'s board page share, over designapp.Service.GetBoard. No
// forge is wired from a bare CLI invocation (workbenchBoardLoader's own
// production default), matching the "no forge configured" silent,
// legitimate case tool_get_board.go's own doc comment already names.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/designapp"
)

func cmdDesignBoard(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI usage grammar (identity arg placeholder)
		fmt.Fprintln(stderr, "usage: verdi design board <spec-ref>")
		return 2
	}
	svc := designapp.NewService()
	result, appErr := svc.GetBoard(context.Background(), ".", designapp.GetBoardRequest{Spec: args[0]})
	if appErr != nil {
		return renderDesignAppResult(stdout, stderr, "design board", nil, appErr)
	}
	return renderDesignAppResult(stdout, stderr, "design board", result, nil)
}
