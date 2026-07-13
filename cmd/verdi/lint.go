// Command verdi's `lint` verb: artifactlint (PLAN.md Phase 4), wired
// against internal/lint's engine. Kept in its own file so dispatch.go's
// diff for wiring this verb in stays a one-line change (another agent is
// concurrently touching dispatch.go for `sync`).
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// runLintVerb resolves the store root from the current working directory,
// builds a lint.Context from git and CI environment signals, runs every
// VL-001..VL-018 rule, and prints one line per finding to stdout. Exit
// contract (CLAUDE.md): 0 clean, 1 findings present, 2 operational error
// (can't resolve the store root, can't build the snapshot). A
// SeverityDisclosure finding (VL-017's disclosed-unproven notice on a bare
// CI clone) is printed but does NOT flip the exit to 1 — disclosure is not
// failure (adjudicated at W2 wave close); a run whose only findings are
// disclosures still exits 0.
func runLintVerb(_ []string, stdout, stderr io.Writer) int {
	ctx := context.Background()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "verdi lint: %v\n", err)
		return 2
	}
	root, err := store.FindRoot(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "verdi lint: %v\n", err)
		return 2
	}

	// lint.BuildContext (lifted there from this file, verbatim behavior)
	// derives the git/CI context per I-14 — shared with the disclosures
	// view's enumeration so both run the same context-construction path.
	lctx := lint.BuildContext(ctx, root)

	findings, err := lint.NewEngine().Run(ctx, root, lctx, lint.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "verdi lint: %v\n", err)
		return 2
	}

	exit := 0
	for _, f := range findings {
		fmt.Fprintln(stdout, f.String())
		if f.Severity != lint.SeverityDisclosure {
			exit = 1
		}
	}
	return exit
}
