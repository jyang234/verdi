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
// Story/spec resolution (05 §CLI, I-30): matrix accepts EXACTLY two ref
// forms and nothing else —
//
//	(a) a scheme-prefixed story ref ("jira:LOAN-1482"), matched against
//	    every active feature spec's `story:` field; and
//	(b) a spec ref ("spec/stale-decline").
//
// Any other argument — including a bare tracker key like "STORY-1482" — is
// an operational error (exit 2) whose message names both accepted forms.
// I-30 rejected the earlier trailing-digit-run heuristic (a bare key matched
// against a spec by comparing the numeric suffixes of the two sides' ref
// slugs): it collided silently between stories that share a digit suffix and
// cut against VL-005's scheme discipline.
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
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/evidence"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
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

	spec, err := resolveSpec(root, storyArg)
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

// resolveSpec resolves arg to a feature spec under specs/active/ — 03 §The
// fold's "Scope: the fold is evaluated only for specs under specs/active/".
// Per I-30, arg is EXACTLY one of two forms: a spec ref ("spec/<name>"),
// loaded directly; or a scheme-prefixed story ref ("jira:LOAN-1482"),
// matched against every active feature spec's `story:` field. Any other
// argument is an operational error naming both accepted forms.
func resolveSpec(root, arg string) (*artifact.SpecFrontmatter, error) {
	// (b) A spec ref: load it directly.
	if ref, err := artifact.ParseRef(arg); err == nil && ref.Kind == artifact.KindSpec {
		spec, loadErr := loadActiveSpec(root, ref.Name)
		if loadErr != nil {
			return nil, loadErr
		}
		if spec.Class != artifact.ClassFeature {
			return nil, fmt.Errorf("spec %q is a component spec (no story, no acceptance criteria); matrix only folds feature specs", arg)
		}
		return spec, nil
	}

	// (a) A scheme-prefixed story ref: match it against every active feature
	// spec's story: field. The scheme (the part before ":") need not be a
	// configured provider — an unmatched story ref simply names no spec.
	if scheme, key, ok := strings.Cut(arg, ":"); ok && scheme != "" && key != "" {
		return matchStoryRef(root, arg)
	}

	return nil, fmt.Errorf("%q is neither a scheme-prefixed story ref (e.g. jira:LOAN-1482) nor a spec ref (e.g. spec/stale-decline); matrix accepts exactly those two forms", arg)
}

// matchStoryRef returns the single active feature spec whose story: field
// equals storyRef, erroring if none — or more than one — does.
func matchStoryRef(root, storyRef string) (*artifact.SpecFrontmatter, error) {
	dir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	var matches []*artifact.SpecFrontmatter
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		spec, err := loadActiveSpec(root, e.Name())
		if err != nil {
			return nil, err
		}
		if spec.Class != artifact.ClassFeature {
			continue
		}
		if spec.Story == storyRef {
			matches = append(matches, spec)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no active feature spec has story: %s", storyRef)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.ID
		}
		return nil, fmt.Errorf("story ref %q matches more than one active feature spec: %s", storyRef, strings.Join(names, ", "))
	}
}

// loadActiveSpec reads and strict-decodes specs/active/<name>/spec.md.
func loadActiveSpec(root, name string) (*artifact.SpecFrontmatter, error) {
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return spec, nil
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
