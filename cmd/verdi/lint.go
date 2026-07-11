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

	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/lint"
	"github.com/OWNER/verdi/internal/store"
)

// runLintVerb resolves the store root from the current working directory,
// builds a lint.Context from git and CI environment signals, runs every
// VL-001..VL-014 rule, and prints one "VL-xxx path: message" line per
// finding to stdout. Exit contract (CLAUDE.md): 0 clean, 1 findings
// present, 2 operational error (can't resolve the store root, can't build
// the snapshot).
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

	lctx := buildLintContext(ctx, root)

	findings, err := lint.NewEngine().Run(ctx, root, lctx, lint.Options{})
	if err != nil {
		fmt.Fprintf(stderr, "verdi lint: %v\n", err)
		return 2
	}

	for _, f := range findings {
		fmt.Fprintln(stdout, f.String())
	}
	if len(findings) > 0 {
		return 1
	}
	return 0
}

// buildLintContext derives lint.Context from git (CurrentBranch via
// symbolic-ref; DefaultBranch via the configured remote's HEAD symbolic
// ref, or a CI-declared default branch when running in CI; DiffBase via
// merge-base(HEAD, DefaultBranch) when DefaultBranch is known) per I-14.
// Every git/CI lookup failure degrades to "unknown" rather than aborting
// the lint run — the git-aware rules (VL-004, VL-010) already treat an
// unknown Context field as "can't prove it, don't enforce" (three-valued
// honesty, constitution 2), which is the correct behavior for a checkout
// this function cannot fully introspect (shallow clone, detached HEAD, no
// configured remote).
func buildLintContext(ctx context.Context, root string) lint.Context {
	env := lint.ReadCIEnv()

	var lctx lint.Context
	lctx.InCI = env.InCI
	lctx.TargetBranch = env.TargetBranch

	if branch, err := gitx.CurrentBranch(ctx, root); err == nil {
		lctx.CurrentBranch = branch
	}

	lctx.DefaultBranch = env.DefaultBranch
	if lctx.DefaultBranch == "" {
		if branch, err := gitx.DefaultBranch(ctx, root); err == nil {
			lctx.DefaultBranch = branch
		}
	}

	if lctx.DefaultBranch != "" {
		if base, err := gitx.MergeBase(ctx, root, "HEAD", lctx.DefaultBranch); err == nil {
			lctx.DiffBase = base
		}
	}

	return lctx
}
