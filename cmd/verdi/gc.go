// verdi gc (spec/worktree-manager ac-5): flips dispatch.go's `"gc": 0,
// // out of v0 (PLAN.md §5)` phase-0 stub to a real, implemented verb —
// the same incremental flip `close` already made for its own verb in
// round 6 (dispatch.go's `"close": 14`). This story implements ONLY the
// managed-worktree reclamation slice (internal/wtmanager.GC, dc-1..4):
// verdi-store-layout's "Garbage collection" section also ratifies
// derived-cache pruning and layout/tree-hash-cache pruning, and NEITHER
// is touched here (dc-5) — cmdGc's own output says so, verbatim, on
// every run, so a human never infers full gc coverage from a partial
// implementation.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// gcScopeDisclosure is dc-5's mandatory, verbatim scope line — printed on
// EVERY `verdi gc` run, never merely implied by omission (ac-5's
// behavioral obligation: "a printed line disclosing that derived-cache/
// layout-cache pruning were not run by this invocation").
const gcScopeDisclosure = "gc: scope — this run reclaims managed worktrees only (spec/worktree-manager); derived-cache pruning and layout/tree-hash-cache pruning (verdi-store-layout's other ratified gc bullets) are OUT OF SCOPE and were NOT run"

// cmdGc is `verdi gc`'s real entry point, invoked by dispatch.go.
func cmdGc(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "gc: unexpected argument(s) %v\n", args)
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}
	defaultBranchRef := lint.ResolveDefaultBranch(ctx, root)

	results, err := wtmanager.GC(ctx, root, defaultBranchRef)
	if err != nil {
		fmt.Fprintln(stderr, "gc:", err)
		return 2
	}

	for _, r := range results {
		fmt.Fprintln(stdout, r.Line())
	}
	fmt.Fprintln(stdout, gcScopeDisclosure)
	return 0
}
