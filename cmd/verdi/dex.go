// Command verdi's `dex` verb: `verdi dex build -o <dir>` (PLAN.md Phase 12,
// 05 §Verdi-dex), wired against internal/dex's static-site builder. Kept in
// its own file per the same convention lint.go and sync.go already
// established, so dispatch.go's diff for wiring this verb in stays a
// one-line change.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/OWNER/verdi/internal/dex"
	"github.com/OWNER/verdi/internal/store"
)

// runDexVerb dispatches `verdi dex <subcommand>`. v0 has exactly one
// subcommand, `build` (05 §CLI / Phase 12); any other subcommand — or none
// — is a usage error, per CLAUDE.md's operational-error exit code 2.
func runDexVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "build" {
		fmt.Fprintln(stderr, "usage: verdi dex build -o <dir>")
		return 2
	}
	return runDexBuild(args[1:], stdout, stderr)
}

// runDexBuild resolves the store root from the current working directory
// and builds the site to the -o directory.
func runDexBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dex build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outDir := fs.String("o", "", "output directory for the built site (required)")
	commit := fs.String("commit", "", "commit to build (default: HEAD)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *outDir == "" {
		fmt.Fprintln(stderr, "dex build: -o <dir> is required")
		return 2
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "dex build:", err)
		return 2
	}
	root, err := store.FindRoot(cwd)
	if err != nil {
		fmt.Fprintln(stderr, "dex build:", err)
		return 2
	}

	// The story-page pending-supersession flag reads open MRs through the
	// forge (V1-P8; 03 §The amendment ladder) — wired best-effort exactly
	// like gate/serve/mcp (gate_threads.go): in the forge's own CI both
	// halves are present and the flag is computed; hermetically (no
	// credentials — every test) the forge is nil and the dex page
	// discloses the flag unproven rather than silently unflagged.
	ctx := context.Background()
	f := buildForgeBestEffort(ctx, root)
	if err := dex.Build(ctx, dex.Options{Root: root, OutDir: *outDir, Commit: *commit, Forge: f, DefaultBranch: resolveDefaultBranch(ctx, root)}); err != nil {
		fmt.Fprintln(stderr, "dex build:", err)
		return 2
	}
	fmt.Fprintf(stdout, "dex build: wrote site to %s\n", *outDir)
	return 0
}
