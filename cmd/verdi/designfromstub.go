// verdi design start --from-stub <feature> <stub> (05 §CLI candidate,
// spec/cli-creation ac-3, ledger L-N7): creates a story (or spike) spec
// from a feature's declared stub from the CLI for the first time, calling
// the identical internal/stubinstantiate.Instantiate core the board's own
// stub-instantiate action calls — the ADJ-65 asymmetry closed at the
// mechanism, not merely at the surface, and proven by an output-equality
// parity assertion (designfromstub_test.go).
//
// Kept in its own file per the accept.go/acceptobligation.go convention
// (a related but distinct entry point for the same verb, not tangled
// into design.go's --kind/--name flag flow): <feature> is a bare spec
// name (matching design start's own --name convention, and the board's
// own bare {name} path segment), never a spec/-prefixed ref.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/stubinstantiate"
)

// cmdDesignStartFromStub is `verdi design start --from-stub`'s real entry
// point: parses the two required positional arguments (feature, stub),
// resolves the store root and its already-resolved operating model, and
// delegates to runDesignStartFromStub.
func cmdDesignStartFromStub(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: verdi design start --from-stub <feature> <stub>")
		return 2
	}
	featureName, slug := args[0], args[1]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "design start --from-stub:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "design start --from-stub:", err)
		return 2
	}

	return runDesignStartFromStub(ctx, root, featureName, slug, cfg.Model, stdout, stderr)
}

// runDesignStartFromStub is the testable core: given an already-resolved
// root and model, it loads featureName's own spec
// (.verdi/specs/active/<featureName>/spec.md — a bare spec name) and
// calls the identical internal/stubinstantiate.Instantiate core the
// board's own stub-instantiate action calls, with the feature's own
// declared class, status, and stubs — never a second, CLI-side
// reimplementation that could drift from the board's (the parity
// proof, TestDesignStartFromStub_ParityWithBoardAction). Every refusal
// from Instantiate (unknown slug, wrong class/status, an
// already-existing branch) is operational (exit 2), matching design
// start's own established local convention: every OTHER refusal in this
// verb (an invalid name, an already-existing spec dir, a malformed story
// ref) is exit 2 too, so this path stays internally consistent with it
// rather than introducing this verb's first exit-1 business verdict.
func runDesignStartFromStub(ctx context.Context, root, featureName, slug string, mdl *model.Model, stdout, stderr io.Writer) int {
	specPath := store.ActiveSpecPath(root, featureName)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(stderr, "design start --from-stub: reading %s: %v\n", specPath, err)
		return 2
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "design start --from-stub: %s: %v\n", specPath, err)
		return 2
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		fmt.Fprintf(stderr, "design start --from-stub: %s: %v\n", specPath, err)
		return 2
	}

	result, err := stubinstantiate.Instantiate(ctx, root, featureName, spec.Class, string(spec.Status), spec.Stubs, slug, mdl)
	if err != nil {
		fmt.Fprintln(stderr, "design start --from-stub:", err)
		return 2
	}

	fmt.Fprintf(stdout, "design start: created branch %s\n", result.Branch)
	fmt.Fprintf(stdout, "design start: scaffolded spec/%s from stub %q of spec/%s\n", slug, slug, featureName)
	return 0
}
