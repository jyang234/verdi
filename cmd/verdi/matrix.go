// verdi matrix <story> (05 §CLI, PLAN.md Phase 6): folds a story's
// acceptance-criteria evidence (internal/evidence) and prints the per-AC
// status table plus story eligibility. Kept in its own file per PLAN.md's
// instruction so dispatch.go's diff for wiring this verb in stays a
// one-line handler change.
//
// matrix REPORTS; it never GATES (PLAN.md Phase 8 owns `verdi gate`) — so
// it exits 0 whenever the fold computed successfully, even when the story
// has violated or ineligible ACs, and 2 only for an operational failure
// (no store root, no spec found, a dangling binding, a decode error, ...).
// This is deliberate, not an oversight: a report that refused to print
// because the news was bad would be worse than useless in CI logs.
//
// Story/spec resolution (05 §CLI, I-30) is shared with rollup.go (PLAN.md
// Phase 11) and lives in storyresolve.go: matrix accepts EXACTLY the two ref
// forms documented there — a scheme-prefixed story ref or a spec ref — and
// nothing else.
//
// The waivers/<slug>/ and attestations/<slug>/ directories the fold consults
// are keyed by the story's own ref slug — store.RefSlug of the resolved
// spec's `story:` field, e.g. store.RefSlug("jira:LOAN-1482") = "jira-loan-1482"
// (I-31's canonical <story> path segment, which the corpus fixture now names
// its waiver/attestation dirs by). A corpus that instead named them by some
// other free-standing key (a bare tracker key like waivers/story-1482/) would
// not be bridged here; bridging two unrelated keys was exactly the rejected
// heuristic's job. The board file (mutable/boards/STORY-1482.json) is board
// state owned by a different subsystem — keyed by the tracker's own board key,
// not by RefSlug — and is never an input to matrix's resolution.
package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/OWNER/verdi/internal/evidence"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// cmdMatrix is `verdi matrix`'s real entry point, invoked by dispatch.go.
func cmdMatrix(args []string, stdout, stderr io.Writer) int {
	ctx := context.Background()

	preview := false
	var storyArg string
	for _, a := range args {
		if a == "--preview" {
			preview = true
			continue
		}
		if storyArg != "" {
			fmt.Fprintf(stderr, "matrix: unexpected extra argument %q\n", a)
			return 2
		}
		storyArg = a
	}
	if storyArg == "" {
		fmt.Fprintln(stderr, "matrix: usage: verdi matrix <jira:STORY-KEY | spec/name> [--preview]")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}
	commit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}

	spec, err := storyresolve.Resolve(root, storyArg)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}

	derivedRoot := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}

	// The fold's waiver/attestation directories are keyed by the story's
	// own ref slug (I-30): store.RefSlug of the resolved spec's story: field.
	slug := store.RefSlug(spec.Story)
	result, err := evidence.Fold(evidence.Input{
		Spec:      spec,
		Records:   records,
		Preview:   preview,
		StoreRoot: root,
		StorySlug: slug,
	})
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}

	printMatrix(stdout, result, preview)
	return 0
}

// printMatrix renders result as a per-AC table plus the story eligibility
// line. preview only controls the banner — Fold already decided what's in
// scope.
func printMatrix(w io.Writer, result evidence.StoryResult, preview bool) {
	fmt.Fprintf(w, "story: %s\n", result.Story)
	fmt.Fprintf(w, "spec:  %s\n", result.SpecRef)
	if preview {
		fmt.Fprintln(w, "PREVIEW: advisory (source: local) evidence included alongside authoritative (source: ci)")
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "AC\tSTATUS\tEVIDENCE\tTEXT")
	for _, r := range result.ACs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.ID, r.Status, r.Summary, r.Text)
	}
	tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "story.violated: %t\n", result.Violated)
	fmt.Fprintf(w, "story.eligible: %t\n", result.Eligible)
}
