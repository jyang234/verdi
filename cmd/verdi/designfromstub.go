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
//
// Fix round 1, Finding 1: the feature's Git-derived EFFECTIVE status
// (internal/specstate), never the raw persisted status: field, is what
// gates stub-instantiate here — mirroring internal/workbench/
// boardspecapi.go's actionStubInstantiate, which passes proj.Status (the
// board's own specstate-resolved projection field, boardspec.go's
// loadBoard: `string(st.ArtifactStatus())`), never the board's raw
// decoded frontmatter either. Before this fix, this file passed
// `string(spec.Status)` — straight off artifact.DecodeSpec — so a
// statusless feature merge-signaled accepted (Task 4's compatibility
// reading: exact bytes already on the default branch) was ALLOWED by the
// board and REFUSED here: a parity break on a state-changing verb. The
// seam (specStateResolver, Resolve/ResolveMany) is buildstart.go's own
// package-level interface, reused verbatim (04 §port pattern: one
// definition per package, not copy-pasted per consumer).
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
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

	return runDesignStartFromStub(ctx, root, featureName, slug, specstate.NewProjector(), cfg.Model, stdout, stderr)
}

// runDesignStartFromStub is the testable core: given an already-resolved
// root, model, and specStateResolver (production: specstate.NewProjector(),
// the buildstart.go/gate.go convention; tests: the same real, git-backed
// projector against a fixturegit repo, or a fake for shapes real git
// cannot practically reconstruct), it loads featureName's own spec
// (.verdi/specs/active/<featureName>/spec.md — a bare spec name), resolves
// its Git-derived EFFECTIVE status (Finding 1 fix: never the raw
// persisted status: field), and calls the identical
// internal/stubinstantiate.Instantiate core the board's own
// stub-instantiate action calls, with the feature's own declared class,
// EFFECTIVE status, and stubs — never a second, CLI-side reimplementation
// that could drift from the board's (the parity proof,
// TestDesignStartFromStub_ParityWithBoardAction, and Finding 1's own new
// TestRunDesignStartFromStub_StatuslessExactDefaultBranch_Starts). Every
// refusal from Instantiate (unknown slug, wrong class/status, an
// already-existing branch) is operational (exit 2), matching design
// start's own established local convention: every OTHER refusal in this
// verb (an invalid name, an already-existing spec dir, a malformed story
// ref) is exit 2 too, so this path stays internally consistent with it
// rather than introducing this verb's first exit-1 business verdict. A
// resolver error (including an unproven effective state) is likewise
// operational (exit 2): this verb cannot honestly decide the precondition
// without a proven state, so it must not guess either way — the same
// posture runBuildStart's own resolver.Resolve call takes.
func runDesignStartFromStub(ctx context.Context, root, featureName, slug string, resolver specStateResolver, mdl *model.Model, stdout, stderr io.Writer) int {
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

	// Finding 1: the feature's EFFECTIVE status, resolved through the
	// shared specstate projector exactly like the board path
	// (boardspec.go's loadBoard: `string(st.ArtifactStatus())`), never
	// spec.Status straight off DecodeSpec — a statusless feature whose
	// exact bytes already landed on the default branch resolves
	// accepted-pending-build here exactly as it does for the board, and a
	// merely locally-edited/unmerged claim of acceptance does not.
	relPath := store.ActiveSpecRelPath(featureName)
	state, err := resolver.Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: raw})
	if err != nil {
		fmt.Fprintln(stderr, "design start --from-stub:", err)
		return 2
	}

	result, err := stubinstantiate.Instantiate(ctx, root, featureName, spec.Class, string(state.ArtifactStatus()), spec.Stubs, slug, mdl)
	if err != nil {
		fmt.Fprintln(stderr, "design start --from-stub:", err)
		return 2
	}

	fmt.Fprintf(stdout, "design start: created branch %s\n", result.Branch)
	fmt.Fprintf(stdout, "design start: scaffolded spec/%s from stub %q of spec/%s\n", slug, slug, featureName)
	return 0
}
