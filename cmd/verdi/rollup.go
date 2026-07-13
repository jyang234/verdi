// verdi rollup <story> --publish (05 §CLI, PLAN.md Phase 11): computes a
// story's fold from AUTHORITATIVE evidence only (03 §The fold, Preview:
// false — CI provenance only, never advisory) and publishes it through the
// story-provider registry (04 §Semantics: "PublishRollup runs in CI
// only"). Kept in its own file per PLAN.md's instruction (matrix.go,
// sync.go) so dispatch.go's diff for wiring this verb in stays a one-line
// handler change.
//
// Story/spec resolution reuses matrix.go's shared I-30 helper
// (internal/storyresolve): a positional argument accepting exactly a
// scheme-prefixed story ref or a spec ref. 05 §CLI's terse table entry
// ("verdi rollup --publish") omits the argument the same way its "verdi
// lint" row omits lint's path arguments; PLAN.md's own Phase 11 goal is
// explicit that rollup "resolves the story's spec (strict I-30 forms)",
// which requires an identifier as input — there is no other mechanism in
// v0 to name "the current story" from CI context alone (branch/spec
// directory names carry no mechanical relationship to a story ref, I-10).
//
// --publish is required, not optional: 05 §CLI names no other form of this
// verb, and PLAN.md's exit-criteria text is explicit that rollup only ever
// publishes ("Exit 0 published / 1 (reserved: none here...) / 2
// operational") — there is no local, read-only rollup preview in v0;
// `verdi matrix --preview` already owns that job.
//
// CI-only enforcement (04 §Semantics: "PublishRollup runs in CI only"):
// cmdRollup refuses to run outside a detected CI environment
// (internal/lint.ReadCIEnv) with a legible exit-2 message, unless
// --force-local is passed. --force-local exists purely for local testing
// of this verb; it is not a way to make a local publish authoritative —
// every use prints a disclosed, non-authoritative warning to stderr before
// proceeding.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider"
	"github.com/jyang234/verdi/internal/provider/jira"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// rollupDeps bundles rollup's injectable dependencies so runRollup can be
// driven hermetically in tests (CLAUDE.md: no network in any test) —
// mirrors sync.go's syncDeps pattern. cmdRollup wires the real registry;
// tests inject a fake provider or an httptest-backed jira.Adapter
// directly.
type rollupDeps struct {
	Registry       provider.StoryProvider
	Stdout, Stderr io.Writer
}

// cmdRollup is `verdi rollup`'s real entry point, invoked by dispatch.go.
func cmdRollup(args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()

	publish := false
	forceLocal := false
	var storyArg string
	for _, a := range args {
		switch a {
		case "--publish":
			publish = true
		case "--force-local":
			forceLocal = true
		default:
			if storyArg != "" {
				fmt.Fprintf(stderr, "rollup: unexpected extra argument %q\n", a)
				return 2
			}
			storyArg = a
		}
	}
	if storyArg == "" {
		fmt.Fprintln(stderr, "rollup: usage: verdi rollup <jira:STORY-KEY | spec/name> --publish [--force-local]")
		return 2
	}
	if !publish {
		fmt.Fprintln(stderr, "rollup: --publish is required (05 §CLI names no other form; `verdi matrix --preview` is the local, read-only fold report)")
		return 2
	}

	inCI := lint.ReadCIEnv().InCI
	if !inCI && !forceLocal {
		fmt.Fprintln(stderr, "rollup: refusing to publish outside CI (04 §Semantics: \"PublishRollup runs in CI only\"); pass --force-local to publish anyway for local testing only")
		return 2
	}
	if !inCI {
		fmt.Fprintln(stderr, "rollup: --force-local: publishing outside CI; this escape hatch exists for local testing only and is NON-AUTHORITATIVE (04 §Semantics: PublishRollup runs in CI only)")
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "rollup:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(stderr, "rollup:", err)
		return 2
	}

	deps := rollupDeps{Registry: buildProviderRegistry(manifest), Stdout: stdout, Stderr: stderr}
	return runRollup(ctx, root, storyArg, deps)
}

// runRollup is the testable core: given an already-resolved store root, a
// story/spec argument, and injected deps, fold the story's authoritative
// evidence and publish it.
func runRollup(ctx context.Context, root, storyArg string, deps rollupDeps) int {
	ref, commit, err := resolveRefCommit(ctx, root)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "rollup:", err)
		return 2
	}

	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "rollup:", err)
		return 2
	}

	derivedRoot := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		fmt.Fprintln(deps.Stderr, "rollup:", err)
		return 2
	}

	// Rollups publish AUTHORITATIVE evidence only (03 §The fold: "over
	// authoritative records only") — Preview stays false, unlike matrix's
	// --preview escape hatch, which has no rollup equivalent.
	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{
		Spec:      spec,
		Records:   records,
		Preview:   false,
		StoreRoot: root,
		StorySlug: slug,
	})
	if err != nil {
		fmt.Fprintln(deps.Stderr, "rollup:", err)
		return 2
	}

	roll := provider.Rollup{
		Story:    provider.StoryRef(spec.Story),
		Ref:      ref,
		Commit:   commit,
		Criteria: mapCriteria(result.ACs),
		Eligible: result.Eligible,
	}
	if err := deps.Registry.PublishRollup(ctx, roll); err != nil {
		fmt.Fprintln(deps.Stderr, "rollup:", err)
		return 2
	}

	fmt.Fprintf(deps.Stdout, "rollup: published %s at %s (eligible=%t, violated=%t)\n", spec.Story, commit, result.Eligible, result.Violated)
	return 0
}

// mapCriteria maps the fold's per-AC results onto the story-provider
// port's CriterionStatus (04 §The port): Status values are already the
// same strings on both sides (evidence.Status's constants spell out
// evidenced/violated/pending/no-signal/waived verbatim, matching 04's
// CriterionStatus.Status comment).
func mapCriteria(acs []evidence.ACResult) []provider.CriterionStatus {
	out := make([]provider.CriterionStatus, len(acs))
	for i, ac := range acs {
		out[i] = provider.CriterionStatus{
			ID:      ac.ID,
			Text:    ac.Text,
			Status:  string(ac.Status),
			Summary: ac.Summary,
		}
	}
	return out
}

// buildProviderRegistry builds the real story-provider registry from
// verdi.yaml's providers: map (04 §Reference scheme). Only ids and URLs
// come from the manifest; the Jira token comes from VERDI_JIRA_TOKEN (04
// §Jira adapter: "Secrets ... never committed"), never verdi.yaml.
func buildProviderRegistry(m *store.Manifest) *provider.Registry {
	providers := map[string]provider.StoryProvider{}
	if m.Providers != nil && m.Providers.Jira != nil {
		providers["jira"] = jira.New(jira.Config{
			BaseURL:     m.Providers.Jira.BaseURL,
			RollupField: m.Providers.Jira.RollupField,
			Token:       os.Getenv("VERDI_JIRA_TOKEN"),
		})
	}
	return provider.NewRegistry(providers)
}
