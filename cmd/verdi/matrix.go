// verdi matrix [--preview] --json <story-or-feature-ref> emits the canonical
// matrix record; the legacy no-JSON forms render that same typed projection as
// the established text tables. internal/matrixprojection owns target
// resolution, folding, and record assembly for both adapters and MCP.
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
	"strings"
	"text/tabwriter"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// cmdMatrix is `verdi matrix`'s real entry point, invoked by dispatch.go.
func cmdMatrix(args []string, stdout, stderr io.Writer) int {
	preview, jsonMode, target, ok := parseMatrixArgs(args)
	if !ok {
		fmt.Fprintln(stderr, "matrix: usage: verdi matrix [--preview] --json <story-or-feature-ref> | verdi matrix <story-or-feature-ref> [--preview]")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}
	projection, err := matrixprojection.Project(context.Background(), root, target, preview, cfg.Model)
	if err != nil {
		fmt.Fprintln(stderr, "matrix:", err)
		return 2
	}
	if jsonMode {
		data, err := matrixprojection.Marshal(projection.Record)
		if err != nil {
			fmt.Fprintln(stderr, "matrix:", err)
			return 2
		}
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintln(stderr, "matrix:", err)
			return 2
		}
		return 0
	}

	if projection.Record.Story != nil {
		specName, err := specDirName(projection.Spec.ID)
		if err != nil {
			fmt.Fprintln(stderr, "matrix:", err)
			return 2
		}
		obligationCells, err := obligationCellsFor(root, specName, projection.Spec.AcceptanceCriteria)
		if err != nil {
			fmt.Fprintln(stderr, "matrix:", err)
			return 2
		}
		printMatrix(stdout, projection.Record, projection.EffectiveStatus, projection.Model, obligationCells)
		return 0
	}
	if projection.Feature == nil {
		fmt.Fprintln(stderr, "matrix: feature projection detail is missing")
		return 2
	}
	printFeatureMatrix(stdout, projection.Spec, projection.EffectiveStatus, projection.Record, projection.Feature.Reconciliation, projection.Feature.Stories, projection.Feature.SupersededByAC, projection.Model)
	return 0
}

func parseMatrixArgs(args []string) (preview, jsonMode bool, target string, ok bool) {
	switch len(args) {
	case 1:
		if strings.HasPrefix(args[0], "-") {
			return false, false, "", false
		}
		return false, false, args[0], true
	case 2:
		switch {
		case args[0] == "--json" && !strings.HasPrefix(args[1], "-"):
			return false, true, args[1], true
		case args[0] == "--preview" && !strings.HasPrefix(args[1], "-"):
			return true, false, args[1], true
		case args[1] == "--preview" && !strings.HasPrefix(args[0], "-"):
			return true, false, args[0], true
		default:
			return false, false, "", false
		}
	case 3:
		if args[0] == "--preview" && args[1] == "--json" && !strings.HasPrefix(args[2], "-") {
			return true, true, args[2], true
		}
		return false, false, "", false
	default:
		return false, false, "", false
	}
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

// The `--preview` advisory-fold disclosure is disclosure.AdvisoryPreview —
// hoisted into the seam package itself (disclosure/advisorypreview.go) when
// internal/workbench's matrix page became its second producing PACKAGE,
// exactly the threshold disclosure/reviewfeed.go's placement note cites. It
// briefly lived here, beside the two same-package rungs (printMatrix and
// featurematrix.go's printFeatureMatrix), the way sync.go's
// toolPinCarrierAbsentDisclosure still does for its genuinely single-package
// state.

// printMatrix renders result as a per-AC table plus the story eligibility
// line. status is the resolved spec's EFFECTIVE lifecycle state (final fix
// wave I2: the caller's one effectiveMatrixStatus resolution, never the
// raw persisted field — which is legitimately BLANK on a statusless,
// merge-accepted spec): printed unconditionally so a superseded (or any
// other) terminal state is legible on this surface directly — 03
// §rung 3's "everywhere without consulting backlinks" property — rather
// than only inferable by opening the raw spec or chasing a
// `superseded-by` backlink. preview only controls whether
// disclosure.AdvisoryPreview is rendered through the shared
// internal/disclosure seam — Fold already decided what's in scope.
//
// obligationCells is spec/obligation-wall ac-1's addition: each AC's
// pre-rendered OBLIGATION column entry (obligationCellsFor), keyed by AC
// id — kept as a caller-supplied map, rather than looked up here, so this
// function stays a pure formatter over already-computed data (no disk I/O),
// exactly as it was before this story.
func printMatrix(w io.Writer, record matrixprojection.Record, status artifact.Status, mdl *model.Model, obligationCells map[string]string) {
	result := record.Story
	// L-M13(1) classification: the "story:"/"spec:"/"status:" line KEYS
	// mirror frontmatter field names, and the trailing
	// story.violated/story.eligible lines are the fold's verdict KEYS —
	// identity, bare. The status VALUE is a state word — display, resolved
	// (spec/vocabulary-surfaces ac-1; nil-safe bare-id fallback).
	fmt.Fprintf(w, "story: %s\n", result.StoryRef)
	fmt.Fprintf(w, "spec:  %s\n", record.Target.SpecRef)
	fmt.Fprintf(w, "status: %s\n", mdl.DisplayState("story", string(status)))
	if record.Preview {
		// Bare Render output, unindented and unprefixed, so
		// disclosure.IsRendered recognizes the line and a disclosure consumer
		// can count it (ac-1's recognizer half).
		fmt.Fprintln(w, disclosure.Render(disclosure.AdvisoryPreview()))
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "AC\tSTATUS\tEVIDENCE\tTEXT\tOBLIGATION")
	for _, r := range result.ACs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Status, r.Summary, r.Text, obligationCells[r.ID])
	}
	_ = tw.Flush() // tabwriter over stdout; flush error is unactionable CLI output

	fmt.Fprintln(w)
	fmt.Fprintf(w, "story.violated: %t\n", record.Violated)
	fmt.Fprintf(w, "story.eligible: %t\n", result.Eligible)
}
