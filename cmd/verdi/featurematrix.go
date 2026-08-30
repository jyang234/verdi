// This file retains feature discovery helpers shared by lifecycle consumers
// and the legacy feature text renderer. The matrix command now receives the
// fold and rendering detail from internal/matrixprojection, so this renderer
// formats the same typed record returned by matrix JSON and MCP.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// implementingStoryEdges is one implementing story's already-folded
// contribution, carried alongside its per-AC ImplementingStory views so
// the stub-reconciliation section can reuse the same discovery pass.
type implementingStoryEdges struct {
	SpecRef string
	ACIDs   []string // every feature AC this story implements, sorted
	Closed  bool
	// Slug is store.RefSlug(story.Title) — the same title-slug half of
	// R4-I-12's stub-match that binds a story to a stub (accept.go). Used by
	// the stub table's per-stub LIVE STORIES realization-candidate computation
	// (D-17).
	Slug string
}

// discoverImplementingStories finds every story spec with an `implements`
// edge into featureName's ACs (via the index's computed backlink
// inversion — 03 §The feature fold: "the authoritative AC->story mapping
// is computed ... the set of story specs whose implements edges name the
// feature AC"), story-folds each exactly once, and returns a flat per-story
// view (for stub reconciliation), an AC-grouped view (for the feature
// fold), and an AC-grouped view of excluded SUPERSEDED stories (for
// rendering only — see below).
//
// Round-5 amendment (D-16): a superseded story is excluded from the
// feature fold's AC->story mapping (the second return value) and from
// stub reconciliation's live-story set (the first) — it can never close,
// so folding it in would make "every implementing story closed or
// eligible" permanently unreachable for any feature that ever had a
// rung-3 event; its successor (the story that supersedes it) carries the
// same implements edges and remains in the mapping in its place.
//
// ac-2 (feature-supersession-state) amends the RENDERING half only: D-16's
// exclusion silently dropped a superseded story from the printed matrix
// entirely, the exact backlink-only blindness 03 §rung 3 exists to avoid.
// The third return value carries every excluded story back out, keyed by
// the feature AC ids it used to implement, purely so printFeatureMatrix can
// render it with a terminal marker instead of it vanishing — the fold and
// reconciliation inputs above are computed exactly as D-16 shipped them,
// unaffected by this third value's existence.
//
// Defect fix (disclosed, found while building feature closure, spec/close-
// verb's deferred half): this function used to resolve every implementing
// story via storyresolve.LoadActiveSpec, which reads specs/active/ only.
// The index (internal/index/walk.go) walks BOTH specs/active/ and
// specs/archive/, so a story's `implements` backlink is discovered
// regardless of which zone it lives in — but a story that has already
// CLOSED has moved to specs/archive/ (02 §Kind registry's
// "...→closed(archive)" transition), and LoadActiveSpec then errors
// "no such file or directory", surfaced as an operational failure of the
// whole matrix/close call. This is exactly the scenario feature closure
// most needs to handle (03 §The feature fold: "every implementing story is
// closed or eligible") and it was never exercised before: proven with a
// witness against this very repo — `verdi matrix spec/true-closure` (whose
// four implementing stories are already archived) fails today with
// "loading implementing story spec/close-verb: ... no such file or
// directory". Fixed by resolving through storyresolve.LoadSpec, which
// already checks active then archive (used elsewhere for exactly this
// "a supersedes target may legitimately live in archive" reason) —
// unchanged behavior for every existing test (none of which has a closed
// implementing story, by their own admission — see
// TestCmdMatrix_FeatureRef_Golden's doc comment), since LoadSpec checks
// active first.
func discoverImplementingStories(ctx context.Context, root, commit string, ix *index.Index, featureName string, spec *artifact.SpecFrontmatter, resolver specStateResolver) ([]implementingStoryEdges, map[string][]evidence.ImplementingStory, map[string][]string, error) {
	projected, byAC, supersededByAC, err := matrixprojection.DiscoverImplementingStories(ctx, root, commit, featureName, spec, ix, resolver)
	if err != nil {
		return nil, nil, nil, err
	}
	flat := make([]implementingStoryEdges, 0, len(projected))
	for _, story := range projected {
		flat = append(flat, implementingStoryEdges{SpecRef: story.SpecRef, ACIDs: story.ACIDs, Closed: story.Closed, Slug: story.Slug})
	}
	return flat, byAC, supersededByAC, nil
}

// loadSpecBytesWithZone reads and strict-decodes name's spec.md from
// specs/active/, then specs/archive/ (active preferred — the same zone
// order storyresolve.LoadSpec uses, generalized here to also return the
// zone-relative path and raw bytes a specstate.Candidate needs), returning
// (nil, "", nil, nil) when neither zone has it.
func loadSpecBytesWithZone(root, name string) (spec *artifact.SpecFrontmatter, relPath string, content []byte, err error) {
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		path := store.SpecPath(root, zone, name)
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				continue
			}
			return nil, "", nil, fmt.Errorf("reading %s: %w", path, rerr)
		}
		fm, _, serr := artifact.SplitFrontmatter(data)
		if serr != nil {
			return nil, "", nil, fmt.Errorf("%s: %w", path, serr)
		}
		decoded, derr := artifact.DecodeSpec(fm)
		if derr != nil {
			return nil, "", nil, fmt.Errorf("%s: %w", path, derr)
		}
		return decoded, store.SpecRelPath(zone, name), data, nil
	}
	return nil, "", nil, nil
}

// supersededStoryRefs flattens discoverImplementingStories' AC-keyed superseded
// view (supersededByAC) into the deduplicated set of superseded implementing
// story refs the feature-close spec-stale condition needs (L-N12): a story that
// implements more than one feature AC appears under each, but must be
// disclosed-and-excluded exactly once. The output is sorted here at the source
// (the M9 determinism note): supersededByAC is a map, so the flatten's first-
// seen order is Go-map-iteration-random — sorting once here makes this function
// deterministic on its own rather than leaving each caller to re-sort. The
// spec-stale condition (closuregatefeature.go) still sorts its own defensive
// copy; that stays correct, merely redundant, on an already-sorted slice.
func supersededStoryRefs(supersededByAC map[string][]string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, group := range supersededByAC {
		for _, ref := range group {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return refs
}

// printFeatureMatrix renders the feature fold, mirroring printMatrix's
// shape (matrix.go) at the feature level, plus the stub × computed-live-
// mapping section 05 §Lenses requires: "story stubs always rendered
// paired with the computed live implements mapping under an explicit
// 'acceptance-time plan; current mapping computed below' banner (never the
// frozen stubs alone)".
//
// supersededByAC (ac-2, feature-supersession-state) is discoverImplementing-
// Stories' third return value: every superseded implementing story,
// excluded from result/reconciliation's own fold inputs exactly as D-16
// shipped, rendered here with a terminal `[superseded]` marker appended to
// its ref instead of silently vanishing from the row it used to occupy —
// legible without consulting a `superseded-by` backlink (03 §rung 3), with
// no change to the eligibility math computed above.
func printFeatureMatrix(w io.Writer, spec *artifact.SpecFrontmatter, effectiveStatus artifact.Status, record matrixprojection.Record, reconciliation evidence.StubReconciliation, stories []matrixprojection.ImplementingStory, supersededByAC map[string][]string, mdl *model.Model) {
	result := record.Feature
	// L-M13(1) classification: the "feature:"/"status:" line KEYS and the
	// trailing feature.violated/stub_reconciliation.blocked lines are
	// verdict/field KEYS — identity, bare. State/class words spoken as
	// VALUES or table prose below resolve through mdl (nil-safe).
	fmt.Fprintf(w, "feature: %s\n", record.Target.SpecRef)
	// ac-2 (feature-supersession-state), as amended by final fix wave I2:
	// the feature's EFFECTIVE lifecycle state (the caller's one
	// effectiveMatrixStatus resolution — the raw persisted field is
	// legitimately BLANK on a statusless, merge-accepted feature), printed
	// unconditionally so a superseded FEATURE's terminal state is legible
	// on this surface directly — the feature-rung mirror of printMatrix's
	// own status line, satisfying ac-2's "every surface ... at both the
	// story and feature rungs" (03 §rung 3, "without consulting
	// backlinks") for a feature you point `verdi matrix` at, not only for
	// a superseded story rendered inside a feature's fold.
	fmt.Fprintf(w, "status: %s\n", mdl.DisplayState(string(spec.Class), string(effectiveStatus)))
	if record.Preview {
		// The SAME constructor the story rung and the workbench matrix page
		// use (disclosure.AdvisoryPreview), rendered through the same seam —
		// one state, one vocabulary, on every surface. This line was a
		// hand-authored duplicate of matrix.go's own banner string before
		// spec/disclosure-legibility#ac-1's migration.
		fmt.Fprintln(w, disclosure.Render(disclosure.AdvisoryPreview()))
	}
	fmt.Fprintln(w)

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	// Column headers speak the class plural as display (upper-cased,
	// L-M13(1)); the [superseded] marker is the state word as display.
	// Story REFS in the cells stay identity.
	storiesHeader := strings.ToUpper(mdl.DisplayClassPlural("story"))
	fmt.Fprintln(tw, "AC\tSTATUS\tEVIDENCE\tIMPLEMENTING "+storiesHeader+"\tTEXT")
	for _, ac := range result.ACs {
		entries := append([]string(nil), ac.ImplementingStories...)
		for _, s := range supersededByAC[ac.ID] {
			entries = append(entries, s+" ["+mdl.DisplayState("story", "superseded")+"]")
		}
		stories := "-"
		if len(entries) > 0 {
			stories = joinComma(entries)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", ac.ID, ac.Status, ac.Summary, stories, ac.Text)
	}
	_ = tw.Flush()

	fmt.Fprintln(w)
	fmt.Fprintln(w, "stubs: acceptance-time plan; current mapping computed below")
	if len(spec.Stubs) == 0 {
		fmt.Fprintln(w, "(none declared)")
	}
	byStubSlug := make(map[string]evidence.StubResult, len(reconciliation.Stubs))
	for _, r := range reconciliation.Stubs {
		byStubSlug[r.Slug] = r
	}
	stw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(stw, "STUB\tDECLARED ACS\tLIVE "+storiesHeader+"\tRECONCILIATION")
	for _, stub := range spec.Stubs {
		live := realizationCandidates(stories, stub)
		liveStr := "-"
		if len(live) > 0 {
			liveStr = joinComma(live)
		}
		bucket := byStubSlug[stub.Slug].Bucket
		fmt.Fprintf(stw, "%s\t%s\t%s\t%s\n", stub.Slug, joinComma(stub.AcceptanceCriteria), liveStr, bucket)
	}
	_ = stw.Flush()

	if len(reconciliation.Unplanned) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "unplanned additions (%s %s tracing to no stub):\n",
			mdl.DisplayState("story", "closed"), mdl.DisplayClassPlural("story"))
		for _, u := range reconciliation.Unplanned {
			fmt.Fprintf(w, "  %s (%s)\n", u.SpecRef, joinComma(u.ACIDs))
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "feature.violated: %t\n", record.Violated)
	fmt.Fprintf(w, "stub_reconciliation.blocked: %t\n", reconciliation.Blocked)
}

// realizationCandidates returns the per-stub LIVE STORIES a stub's row shows
// (D-17): the stories that plausibly REALIZE this specific stub, so an
// operator can read which story maps to which stub — not every story merely
// touching one of the stub's ACs (the old union, which listed the same story
// under every stub sharing an AC and made the column illegible). A story is a
// candidate iff it matches either half of R4-I-12's stub-match binding: its
// title slug equals the stub's slug, OR its implements-AC set equals the
// stub's declared AC set. "Unreconciled" reconciliation semantics are
// unchanged — this only sharpens the projection of them.
func realizationCandidates(stories []matrixprojection.ImplementingStory, stub artifact.Stub) []string {
	want := sortedSet(stub.AcceptanceCriteria)
	var out []string
	for _, s := range stories {
		if s.Slug == stub.Slug || equalSortedSets(sortedSet(s.ACIDs), want) {
			out = append(out, s.SpecRef)
		}
	}
	sort.Strings(out)
	return out
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
