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
//
// spec/obligation-wall ac-1 adds the table's OBLIGATION column: per AC, for
// every evidence kind it declares, that kind's obligation title (read
// through internal/evidence.Obligations, keyed by the spec's OWN directory
// name — specDirName — never the story tracker slug above) or a disclosed
// "(no obligation)" marker when none exists yet (dc-2: disclosure, never a
// blocking error here). This is additive only: the fold itself is
// unchanged (evidence-obligations oq-1 — "no fold change, no record
// field") — obligationCellsFor and specDirName below compute the new
// column entirely outside evidence.Fold, and printMatrix stays a pure
// formatter over already-computed data.
package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
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

	// Only a round-four REAL feature spec renders through the feature fold;
	// everything else folds at the story level below. A round-four real
	// feature is exactly `class: feature` AND carrying problem/outcome
	// (VL-006 requires them on new-class specs). Both conjuncts are load-
	// bearing:
	//   - Class alone is not enough: a grandfathered v0 `class: feature`
	//     spec is story-grade (Problem == nil), and must fold at the story
	//     level, not through FoldFeature.
	//   - Problem alone is not enough: a round-four `class: story` spec
	//     ALSO carries problem/outcome, so a Problem-only discriminator
	//     misroutes it into FoldFeature, which fails closed ("not a feature
	//     spec") — the I-1 defect. Its Class is story, so the Class conjunct
	//     keeps it on the story path.
	// See featurematrix.go's doc comment for the grandfathering preserved.
	if spec.Class == artifact.ClassFeature && spec.Problem != nil {
		if err := cmdMatrixFeature(ctx, root, commit, spec, preview, stdout); err != nil {
			fmt.Fprintln(stderr, "matrix:", err)
			return 2
		}
		return 0
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

	// spec/obligation-wall ac-1: obligations are loaded by (spec-name,
	// ac-id) — the spec's OWN directory name, distinct from the story
	// tracker slug the fold's waiver/attestation lookups above use. specName
	// is not carried by evidence.Fold's own output (obligations do not
	// change the fold, feature evidence-obligations oq-1); it is derived
	// here, independently, from the resolved spec's canonical ref.
	specName, err := specDirName(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}
	obligationCells, err := obligationCellsFor(root, specName, spec.AcceptanceCriteria)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}

	printMatrix(stdout, result, spec.Status, preview, obligationCells)
	return 0
}

// specDirName returns the <name> segment of a spec ref "spec/<name>" — the
// same directory-basename convention .verdi/obligations/<name>/ is keyed
// by (spec/obligation-wall DC-1). For any spec resolved through
// storyresolve.Resolve this is exactly the directory storyresolve read it
// from: LoadActiveSpec keys specs/active/<name>/spec.md by this same name,
// and every consumer of a resolved spec already trusts spec.ID as that
// spec's own canonical ref (e.g. this file's own `spec: %s` matrix header
// line prints it directly).
func specDirName(specRef string) (string, error) {
	ref, err := artifact.ParseRef(specRef)
	if err != nil {
		return "", fmt.Errorf("resolved spec ref %q does not parse: %w", specRef, err)
	}
	return ref.Name, nil
}

// obligationCellsFor builds each AC's OBLIGATION column entry ahead of
// rendering (spec/obligation-wall ac-1, dc-1): for every evidence kind ac
// declares, in that AC's own declared order, that kind's obligation title —
// read through the one loader internal/evidence.Obligations backs (dc-1:
// "not two readers", shared with the board's own follow-on render) — or a
// disclosed "(no obligation)" marker when the kind has none yet (dc-2:
// disclosure, never a blocking error on this read surface). A file that
// exists but fails strict decode is a real operational error, not a
// disclosed marker — matrix already treats a decode error as operational
// (this file's own top doc comment: "a decode error" is one of the named
// exit-2 cases), and a broken obligation is not "no obligation."
func obligationCellsFor(root, specName string, acs []artifact.AcceptanceCriterion) (map[string]string, error) {
	cells := make(map[string]string, len(acs))
	for _, ac := range acs {
		obls, err := evidence.Obligations(root, specName, ac.ID)
		if err != nil {
			return nil, fmt.Errorf("loading obligations for %s: %w", ac.ID, err)
		}

		parts := make([]string, 0, len(ac.Evidence))
		for _, kind := range ac.Evidence {
			if o, ok := obls[kind]; ok {
				parts = append(parts, fmt.Sprintf("%s: %s", kind, o.Title))
			} else {
				parts = append(parts, fmt.Sprintf("%s: (no obligation)", kind))
			}
		}
		cells[ac.ID] = strings.Join(parts, "; ")
	}
	return cells, nil
}

// printMatrix renders result as a per-AC table plus the story eligibility
// line. status is the resolved spec's own frontmatter `status` (ac-2,
// feature-supersession-state): printed unconditionally so a superseded (or
// any other) terminal state is legible on this surface directly — 03
// §rung 3's "everywhere without consulting backlinks" property — rather
// than only inferable by opening the raw spec or chasing a
// `superseded-by` backlink. preview only controls the banner — Fold
// already decided what's in scope.
//
// obligationCells is spec/obligation-wall ac-1's addition: each AC's
// pre-rendered OBLIGATION column entry (obligationCellsFor), keyed by AC
// id — kept as a caller-supplied map, rather than looked up here, so this
// function stays a pure formatter over already-computed data (no disk I/O),
// exactly as it was before this story.
func printMatrix(w io.Writer, result evidence.StoryResult, status artifact.Status, preview bool, obligationCells map[string]string) {
	fmt.Fprintf(w, "story: %s\n", result.Story)
	fmt.Fprintf(w, "spec:  %s\n", result.SpecRef)
	fmt.Fprintf(w, "status: %s\n", status)
	if preview {
		fmt.Fprintln(w, "PREVIEW: advisory (source: local) evidence included alongside authoritative (source: ci)")
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "AC\tSTATUS\tEVIDENCE\tTEXT\tOBLIGATION")
	for _, r := range result.ACs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Status, r.Summary, r.Text, obligationCells[r.ID])
	}
	_ = tw.Flush() // tabwriter over stdout; flush error is unactionable CLI output

	fmt.Fprintln(w)
	fmt.Fprintf(w, "story.violated: %t\n", result.Violated)
	fmt.Fprintf(w, "story.eligible: %t\n", result.Eligible)
}
