// verdi feature start <story-spec | story-ref> (R4-I-6): the deprecation
// alias for `verdi build start`, kept one release per 03 §Lifecycle: the
// feature-first cascade ("feature start is kept one release as a
// deprecation alias: it prints the new form and proceeds"). Every bit of
// real logic now lives in buildstart.go — this file is a thin,
// notice-printing forward.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// runFeatureVerb dispatches `verdi feature <subcommand>`. There is exactly
// one subcommand, `start`; anything else is a usage error.
func runFeatureVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		// vocab:identity — CLI verb name / usage grammar (identity)
		fmt.Fprintln(stderr, "usage: verdi feature start <story-spec | story-ref>")
		return 2
	}
	return cmdFeatureStart(args[1:], stdout, stderr)
}

// cmdFeatureStart is `verdi feature start`'s real entry point: it prints
// the deprecation notice, then delegates to the exact same wiring
// cmdBuildStart uses.
func cmdFeatureStart(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		// vocab:identity — CLI verb name / usage grammar (identity)
		fmt.Fprintln(stderr, "feature start: usage: verdi feature start <story-spec | story-ref>")
		return 2
	}
	storyArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		// vocab:identity — CLI verb name / usage grammar (identity)
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
		// vocab:identity — CLI verb name / usage grammar (identity)
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	deps := syncDeps{Runner: runner, GoTest: realGoTestRunner{}, Stdout: stdout, Stderr: stderr}

	return runFeatureStart(ctx, root, storyArg, deps, stdout, stderr)
}

// runFeatureStart is the testable core: prints R4-I-6's deprecation
// notice ("prints the new form and proceeds rather than erroring") and
// delegates to runBuildStart unchanged — the deprecation alias shares
// every precondition (accepted-pending-build, the rung-4 cascade check)
// and every side effect (branch cut, baseline regeneration) with `verdi
// build start`; it differs only in the printed notice.
func runFeatureStart(ctx context.Context, root, storyArg string, deps syncDeps, stdout, stderr io.Writer) int {
	// vocab:identity — CLI verb name / usage grammar (identity)
	fmt.Fprintln(stderr, "feature start: deprecated (R4-I-6) — use `verdi build start <story-spec | story-ref>` instead; proceeding")
	return runBuildStart(ctx, root, storyArg, deps, stdout, stderr)
}
