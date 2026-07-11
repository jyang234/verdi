// verdi feature start <story-ref | spec-ref> (05 §CLI, PLAN.md Phase 7):
// the post-acceptance build ritual — locates the story's spec (I-30
// strict forms, reusing internal/storyresolve), REFUSES (exit 1) unless
// its status is accepted-pending-build (03 §Gates condition 1's local
// half), cuts the build branch feature/<name>, and best-effort refreshes
// the baseline (baseline.go). Kept in its own file per the lint.go/sync.go/
// matrix.go/dex.go convention.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/storyresolve"
	"github.com/OWNER/verdi/internal/upstream"
)

// runFeatureVerb dispatches `verdi feature <subcommand>`. v0 has exactly
// one subcommand, `start` (05 §CLI); anything else is a usage error.
func runFeatureVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		fmt.Fprintln(stderr, "usage: verdi feature start <story-ref | spec-ref>")
		return 2
	}
	return cmdFeatureStart(args[1:], stdout, stderr)
}

// cmdFeatureStart is `verdi feature start`'s real entry point: it parses
// the single positional argument, resolves the store root and manifest,
// and wires the real runner before delegating to runFeatureStart.
func cmdFeatureStart(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "feature start: usage: verdi feature start <story-ref | spec-ref>")
		return 2
	}
	storyArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
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

// runFeatureStart is the testable core: given an already-resolved root and
// injected deps, run the whole feature-start ritual and return the exit
// code. It refuses (exit 1, a verdict failure per CLAUDE.md's 0/1/2
// contract — a business precondition, not an operational problem) before
// touching git at all when the resolved spec is not
// accepted-pending-build, so a refused feature start leaves the repo
// exactly as it found it.
func runFeatureStart(ctx context.Context, root, storyArg string, deps syncDeps, stdout, stderr io.Writer) int {
	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}
	if spec.Status != "accepted-pending-build" {
		fmt.Fprintf(stderr, "feature start: %s status is %q, not accepted-pending-build; a build may only reference an accepted spec (03 §Gates)\n", spec.ID, spec.Status)
		return 1
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "feature start: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	branch := "feature/" + specRef.Name

	commit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		fmt.Fprintln(stderr, "feature start:", err)
		return 2
	}

	regenerateBaseline(ctx, root, branch, commit, spec, deps, "feature start", stderr)

	fmt.Fprintf(stdout, "feature start: created branch %s from %s (status: accepted-pending-build)\n", branch, spec.ID)
	return 0
}
