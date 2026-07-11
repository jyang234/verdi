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
// Story resolution (v0 convention, disclosed — no spec pins this down):
// the argument may be a spec ref ("spec/<name>") or a story key. A story
// key is matched against every feature spec under specs/active/ by
// comparing the trailing digit run of each side's RefSlug — e.g.
// "STORY-1482" matches a spec whose `story:` field is "jira:LOAN-1482"
// because both end in "1482". This is necessarily a heuristic: nothing in
// 02/03/04 defines a mechanical relationship between a spec's tracker-
// scoped `story:` field (jira:LOAN-1482) and the free-standing key used to
// name its waivers/attestations/board files elsewhere in the corpus
// (waivers/story-1482/, mutable/boards/STORY-1482.json) — see this
// package's matrix_test.go and the phase-6 report for the full trail.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
		fmt.Fprintln(stderr, "matrix: usage: verdi matrix <story-or-spec-ref> [--preview]")
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

	slug := resolveStorySlug(root, storyArg, spec.Story)
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

// resolveSpec resolves storyArg to a feature spec under specs/active/ —
// 03 §The fold's "Scope: the fold is evaluated only for specs under
// specs/active/". storyArg is either a spec ref ("spec/<name>") or a
// story key matched against every feature spec's `story:` field via
// storyKeyMatches.
func resolveSpec(root, storyArg string) (*artifact.SpecFrontmatter, error) {
	if ref, err := artifact.ParseRef(storyArg); err == nil && ref.Kind == artifact.KindSpec {
		spec, loadErr := loadActiveSpec(root, ref.Name)
		if loadErr != nil {
			return nil, loadErr
		}
		if spec.Class != artifact.ClassFeature {
			return nil, fmt.Errorf("spec %q is a component spec (no story, no acceptance criteria); matrix only folds feature specs", storyArg)
		}
		return spec, nil
	}

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
		if spec.Story == storyArg || storyKeyMatches(storyArg, spec.Story) {
			matches = append(matches, spec)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no feature spec under specs/active matches story/spec ref %q", storyArg)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.ID
		}
		return nil, fmt.Errorf("story/spec ref %q matches more than one spec under specs/active: %s", storyArg, strings.Join(names, ", "))
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

var trailingDigitsRe = regexp.MustCompile(`[0-9]+$`)

// storyKeyMatches reports whether arg and specStory name the same story
// key, comparing the trailing run of digits in each side's store.RefSlug
// form — e.g. RefSlug("STORY-1482") = "story-1482" and
// RefSlug("jira:LOAN-1482") = "jira-loan-1482" both end in "1482". Two
// empty (digit-less) suffixes never match — that would make every
// digit-less argument match every digit-less story, which is worse than
// refusing the match.
func storyKeyMatches(arg, specStory string) bool {
	a := trailingDigitsRe.FindString(store.RefSlug(arg))
	b := trailingDigitsRe.FindString(store.RefSlug(specStory))
	return a != "" && a == b
}

// resolveStorySlug picks the waivers/<slug>/ and attestations/<slug>/
// directory name to consult: whichever of the raw argument's own slug or
// the resolved spec's story-field slug actually has a waivers/ or
// attestations/ directory on disk wins; the argument's own slug is the
// default when neither exists yet (a story with no waivers/attestations
// at all is the common case, not an error).
func resolveStorySlug(root, storyArg, specStory string) string {
	candidates := []string{store.RefSlug(storyArg)}
	if slug := store.RefSlug(specStory); slug != candidates[0] {
		candidates = append(candidates, slug)
	}
	for _, c := range candidates {
		if dirExists(filepath.Join(root, ".verdi", "waivers", c)) || dirExists(filepath.Join(root, ".verdi", "attestations", c)) {
			return c
		}
	}
	return candidates[0]
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
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
