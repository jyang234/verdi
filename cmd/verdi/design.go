// verdi design start <story-ref> --name <name> (05 §CLI, PLAN.md Phase 7):
// cuts the design branch, scaffolds specs/active/<name>/ as a draft
// feature spec, resolves the story's title via the provider registry
// (degrading to the raw ref on any resolution failure — 04 §Semantics),
// commits the scaffold, and best-effort regenerates the impacted-service
// baseline (baseline.go). Kept in its own file per the lint.go/sync.go/
// matrix.go/dex.go convention, so dispatch.go's diff for wiring this verb
// in stays a one-line change.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/provider"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/upstream"
)

// runDesignVerb dispatches `verdi design <subcommand>`. v0 has exactly one
// subcommand, `start` (05 §CLI); anything else is a usage error.
func runDesignVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "start" {
		fmt.Fprintln(stderr, "usage: verdi design start <story-ref> --name <name>")
		return 2
	}
	return cmdDesignStart(args[1:], stdout, stderr)
}

// designDeps bundles design start's injectable dependencies (mirroring
// syncDeps) so runDesignStart can be driven hermetically in tests
// (CLAUDE.md: no network, no exec in any test); cmdDesignStart wires the
// real ones. Runner is nil when verdi.yaml carries no toolchain: block —
// baseline.go's regenerateBaseline reads that as "skip gracefully",
// never as an error.
type designDeps struct {
	Provider provider.StoryProvider
	Runner   upstream.Runner
	GoTest   goTestRunner
}

// cmdDesignStart is `verdi design start`'s real entry point: it parses
// --name, resolves the store root and manifest, wires the real provider
// registry (empty in v0 — the Jira adapter is Phase 11's deliverable, so
// every scheme currently degrades to the raw ref, which is the honest,
// disclosed v0 behavior per 04 §Semantics's own failure table) and runner,
// and delegates to runDesignStart.
func cmdDesignStart(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("design start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "the spec directory name (required, I-10: no magic)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *name == "" {
		fmt.Fprintln(stderr, "design start: --name is required (I-10: no magic, no tracker-derived naming)")
		return 2
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "design start: usage: verdi design start <story-ref> --name <name>")
		return 2
	}
	storyRef := rest[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	// No real story-provider adapter ships in v0 (Phase 11 builds the Jira
	// adapter); an empty registry makes every Resolve call fail with
	// ErrUnknownScheme, which runDesignStart's degrade path already
	// handles honestly per 04 §Semantics.
	reg := provider.NewRegistry(map[string]provider.StoryProvider{})

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	deps := designDeps{Provider: reg, Runner: runner, GoTest: realGoTestRunner{}}

	return runDesignStart(ctx, root, storyRef, *name, manifest, deps, stdout, stderr)
}

// runDesignStart is the testable core: given an already-resolved root and
// injected deps, run the whole design-start ritual and return the exit
// code. It never partially applies the ritual on failure: a validation
// failure before the branch is cut leaves the repo untouched; baseline
// regeneration failures after the scaffold is committed are disclosed but
// non-fatal (baseline.go), since the baseline is advisory, not the point
// of this verb.
func runDesignStart(ctx context.Context, root, storyRef, name string, manifest *store.Manifest, deps designDeps, stdout, stderr io.Writer) int {
	specRef, err := artifact.ParseRef("spec/" + name)
	if err != nil {
		fmt.Fprintf(stderr, "design start: --name %q is not a valid spec name: %v\n", name, err)
		return 2
	}

	scheme, _, err := provider.ParseStoryRef(provider.StoryRef(storyRef))
	if err != nil {
		fmt.Fprintf(stderr, "design start: story ref %q: %v\n", storyRef, err)
		return 2
	}
	if schemes := manifest.ConfiguredStorySchemes(); !schemes[scheme] {
		fmt.Fprintf(stderr, "design start: story ref %q uses scheme %q, which verdi.yaml's providers: block does not configure\n", storyRef, scheme)
		return 2
	}

	specDir := filepath.Join(root, ".verdi", "specs", "active", name)
	if _, statErr := os.Stat(specDir); statErr == nil {
		fmt.Fprintf(stderr, "design start: %s already exists\n", specDir)
		return 2
	}

	branch := "design/" + name
	if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	title := resolveStoryTitle(ctx, deps.Provider, storyRef, stderr)

	content := scaffoldDraftSpec(specRef.String(), storyRef, title)
	// Self-validate before writing anything to disk (CLAUDE.md: "never
	// fake success") — a scaffold that cannot round-trip through the same
	// strict decode/validate every other verb uses is an internal bug,
	// not a user-facing state.
	fm, _, splitErr := artifact.SplitFrontmatter([]byte(content))
	if splitErr != nil {
		fmt.Fprintln(stderr, "design start: internal error: scaffold failed self-validation:", splitErr)
		return 2
	}
	spec, decodeErr := artifact.DecodeSpec(fm)
	if decodeErr != nil {
		fmt.Fprintln(stderr, "design start: internal error: scaffold failed self-validation:", decodeErr)
		return 2
	}

	if err := os.MkdirAll(specDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}
	headCommit, err := gitx.CreateCommit(ctx, root, fmt.Sprintf("design start: scaffold %s (story %s)", specRef.String(), storyRef))
	if err != nil {
		fmt.Fprintln(stderr, "design start:", err)
		return 2
	}

	regenerateBaseline(ctx, root, branch, headCommit, spec, syncDeps{Runner: deps.Runner, GoTest: deps.GoTest, Stdout: stdout, Stderr: stderr}, "design start", stderr)

	fmt.Fprintf(stdout, "design start: created branch %s\n", branch)
	fmt.Fprintf(stdout, "design start: scaffolded %s (status: draft)\n", specRef.String())
	fmt.Fprintf(stdout, "design start: board: http://localhost:5173/design/%s (workbench UI lands in phase 10; verdi serve lands in phase 9)\n", name)
	return 0
}

// resolveStoryTitle resolves storyRef's title through prov, degrading to
// the raw ref on any failure (04 §Semantics: "On failure, degrade to
// displaying the raw ref; never block rendering") — NotFound, Unavailable,
// or (v0's common case, no real adapter registered yet) ErrUnknownScheme
// all take the same honest, disclosed path.
func resolveStoryTitle(ctx context.Context, prov provider.StoryProvider, storyRef string, stderr io.Writer) string {
	if prov == nil {
		return storyRef
	}
	story, err := prov.Resolve(ctx, provider.StoryRef(storyRef))
	if err != nil {
		fmt.Fprintf(stderr, "design start: story title resolution degraded to the raw ref %q: %v\n", storyRef, err)
		return storyRef
	}
	if story.Title == "" {
		return storyRef
	}
	return story.Title
}

// scaffoldDraftSpec renders a draft feature spec's markdown content:
// frontmatter plus a minimal body. The scaffold carries one placeholder
// acceptance criterion — artifact.SpecFrontmatter.Validate requires at
// least one (02 §feature-spec frontmatter additions), and nothing in 05
// §CLI or PLAN.md's I-10 supplies real ACs at scaffold time; the design
// branch's own edits (the "edit" step of the design → accept ritual) are
// where a human or agent replaces this placeholder before `verdi accept`.
// Owners is likewise a disclosed placeholder ("unassigned"): 02 documents
// owners as "team or CODEOWNERS-resolvable handles", and v0 has no
// manifest field naming a default owning team — inventing one silently
// would be exactly the "no magic" I-10 rejects for naming.
func scaffoldDraftSpec(specRef, storyRef, title string) string {
	return fmt.Sprintf(`---
id: %s
kind: spec
title: %q
owners: [unassigned]
class: feature
status: draft
story: %s
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static] }
---
# %s

TODO: design notes.
`, specRef, title, storyRef, title)
}
