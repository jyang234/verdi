// The FEATURE closure gate (05 §CLI's close row; 03 §The feature fold,
// §Stub reconciliation, §Closure ritual): completes spec/close-verb's
// deferred feature half. Mirrors closuregate.go's story closure gate in
// shape (a small ordered list of gateCondition checks, each printed
// PASS/FAIL) but with feature-specific conditions:
//
//  1. every feature AC folds evidenced (evidence.FoldFeature, including
//     the mandatory outcome floor — 03 §The feature fold's precedence
//     table already treats a still-no-signal AC as a hard blocker, not a
//     yellow, so this condition is exactly "every AC.Status ==
//     evidenced", nothing looser);
//  2. stub reconciliation is not blocked (evidence.ReconcileStubs; 03
//     §Stub reconciliation: "a stub in neither state blocks closure");
//  3. every implementing story is actually CLOSED — 03 states feature
//     closure as a three-way AND ("every feature AC evidenced ... + stub
//     reconciliation passing ... + all implementing stories closed", §The
//     feature fold; echoed again in §Closure ritual's framing, "once every
//     implementing story has closed") — and this third conjunct is NOT
//     implied by the first two: FoldFeature's own "evidenced" reads
//     "closed OR ELIGIBLE" (an implementing story can be merely eligible,
//     not yet actually closed, and still count), and ReconcileStubs only
//     ever inspects CLOSED stories when computing stub coverage — so an
//     eligible-but-still-open straggler story can be invisible to both of
//     the first two conditions. Disclosed here as a deliberate addition
//     beyond the task brief's literal 2-condition list (which mirrors 05
//     §CLI's own shorter framing), backed directly by 03's fuller text;
//     4 & 5. the same spec-stale / pending-supersession posture the story
//     gate checks (closuregate.go's checkSpecStaleCondition /
//     checkPendingSupersessionCondition), reused UNCHANGED against the
//     feature spec itself rather than reimplemented — see each call site
//     below for why that reuse is honest rather than a silently-vacuous
//     no-op;
//  6. disposition-completeness (X-13/X-16/X-17, closuregate.go's
//     checkDispositionCompleteCondition — see that file's top doc comment
//     for the full mechanism), reused UNCHANGED against the feature spec's
//     OWN deviation-report.md: runCloseFeature (closefeature.go) freezes
//     that exact report via the same runAlignForSpec freeze step the story
//     path uses, so it is subject to the identical trap and needs the
//     identical guard.
package main

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// runFeatureClosureGate evaluates the feature-closure gate and prints each
// condition PASS/FAIL (or, for a disclosed condition, the shared
// disclosure rendering) exactly as runClosureGate (closuregate.go) does
// for a story, under its own "closure(feature):" label so the two ritual's
// printed output is never ambiguous about which spec class is closing.
// head is the feature spec's build head — needed by condition 6
// (disposition-completeness) to check the feature's own deviation report
// covers it, mirroring runClosureGate's own head parameter exactly.
func runFeatureClosureGate(ctx context.Context, root string, spec *artifact.SpecFrontmatter, fold evidence.FeatureResult, reconciliation evidence.StubReconciliation, stories []implementingStoryEdges, f forge.Forge, defaultBranchRef string, manifest *store.Manifest, mdl *model.Model, head string, stdout io.Writer) (bool, error) {
	cond1 := checkFeatureFoldEligible(fold, mdl)
	cond2 := checkStubReconciliationCondition(reconciliation)
	cond3 := checkAllImplementingStoriesClosed(stories, mdl)

	// Condition 4 (spec-stale, spec/finding-identity ac-4): the feature-close
	// budget is a UNION over every CLOSED implementing story's ARCHIVED
	// report (findings: + not-resurfaced:) plus the feature's own report
	// (findings: + not-resurfaced:) — the actual cross-report X-18 fix (the
	// within-report "unique identities" framing alone was proven a no-op,
	// ledger L-N2). See checkFeatureSpecStaleCondition's own doc comment.
	cond4raw, err := checkFeatureSpecStaleCondition(root, spec, manifest, stories, mdl)
	if err != nil {
		return false, err
	}
	cond4 := renumbered(cond4raw, "4. no unresolved spec-stale flag")

	// Condition 5 (pending-supersession): reused verbatim from the story
	// gate, called with the feature spec instead of a story spec.
	// checkPendingSupersessionCondition keys off
	// evidence.ImplementsByFeature(spec.Links) — the set of features THIS
	// spec's own `implements` edges name. A feature spec never carries an
	// `implements` edge (02 §Object model: a feature is the AC-providing
	// side of that edge, never the implementing side — no story or feature
	// fixture in this codebase ever gives a feature spec one), so this is
	// always empty for a feature, and the condition passes outright via
	// its own "nothing to prove" branch — never a fabricated "is this
	// feature itself about to be superseded" check, which 03 does not
	// describe and which is already transitively covered: a feature cannot
	// close until every implementing story has closed (condition 3 above),
	// and each of those stories already had to clear ITS OWN
	// pending-supersession condition against this exact feature before it
	// was allowed to close — so no live open supersession against this
	// feature's objects could have slipped through by the time every story
	// is closed. Disclosed as the invention-ledger candidate for this
	// condition rather than silently inventing new machinery.
	cond5raw, err := checkPendingSupersessionCondition(ctx, f, defaultBranchRef, spec)
	if err != nil {
		return false, err
	}
	cond5 := renumbered(cond5raw, "5. no unresolved pending-supersession flag")

	// Condition 6 (disposition-completeness, X-13/X-16/X-17): reused
	// unchanged from the story gate, called with the feature spec and its
	// own build head — see this file's top doc comment for why that reuse
	// is honest (runCloseFeature freezes THIS exact report via the same
	// freeze step the story path uses).
	cond6raw, err := checkDispositionCompleteCondition(root, spec, head)
	if err != nil {
		return false, err
	}
	cond6 := renumbered(cond6raw, "6. deviation report ready to freeze (no undispositioned findings)")

	// The label's parenthetical exists to tell a HUMAN which spec class is
	// closing (this function's doc comment: "never ambiguous about which
	// spec class") — display prose, resolved through the model (L-M13(1));
	// the disclosure Source producer ids it wraps stay identity.
	label := "closure(" + mdl.DisplayClass("feature") + "): "
	allOK := true
	for _, c := range []gateCondition{cond1, cond2, cond3, cond4, cond5, cond6} {
		switch {
		case c.Disclosed:
			// Three-valued honesty (constitution 2/10), rendered through the
			// shared internal/disclosure seam exactly as the story gate does.
			fmt.Fprint(stdout, label)
			fmt.Fprintln(stdout, disclosure.Render(disclosure.New(c.Source, "", c.Reason)))
		case c.OK:
			fmt.Fprintf(stdout, "[PASS] %s%s\n", label, c.Name)
		default:
			allOK = false
			fmt.Fprintf(stdout, "[FAIL] %s%s\n", label, c.Name)
			fmt.Fprintf(stdout, "       %s\n", c.Reason)
		}
	}
	return allOK, nil
}

// renumbered returns c with Name replaced by name, preserving every other
// field — used to re-present a reused story-gate condition (whose own
// Name string embeds ITS position in the 3-condition story gate, e.g.
// "2. no unresolved spec-stale flag") under this gate's own 5-condition
// numbering, without touching the shared, unexported checkXxxCondition
// functions themselves (closuregate.go) that the story gate also calls
// unchanged.
func renumbered(c gateCondition, name string) gateCondition {
	c.Name = name
	return c
}

// checkFeatureFoldEligible is the feature-closure gate's condition 1:
// every feature AC folds to evidenced (evidence.FoldFeature already
// enforces the mandatory outcome floor per-AC, 03 §The feature fold) — a
// still-no-signal, still-pending, or violated AC all block closure alike
// (03: "A feature AC still no-signal at closure time is a hard blocker,
// not a yellow").
func checkFeatureFoldEligible(fold evidence.FeatureResult, mdl *model.Model) gateCondition {
	// The spoken class word resolves (L-M13(1)); the "(03 §The feature
	// fold …)" SPEC CITATION quotes the spec's own section title —
	// identity, kept verbatim.
	featureWord := mdl.DisplayClass("feature")
	name := "1. every " + featureWord + " AC evidenced (03 §The feature fold, including the outcome floor)"
	var notEvidenced []string
	for _, ac := range fold.ACs {
		if ac.Status != evidence.StatusEvidenced {
			notEvidenced = append(notEvidenced, fmt.Sprintf("%s=%s", ac.ID, ac.Status))
		}
	}
	if len(notEvidenced) == 0 {
		return gateCondition{Name: name, OK: true}
	}
	sort.Strings(notEvidenced)
	return gateCondition{Name: name, Reason: fmt.Sprintf("not every %s AC is evidenced: %v", featureWord, notEvidenced)}
}

// checkStubReconciliationCondition is the feature-closure gate's condition
// 2: no acceptance-time stub is left unreconciled (03 §Stub reconciliation:
// "A stub in neither [realized-by nor withdrawn-with-note] state blocks
// closure").
func checkStubReconciliationCondition(r evidence.StubReconciliation) gateCondition {
	name := "2. stub reconciliation not blocked (03 §Stub reconciliation)"
	if !r.Blocked {
		return gateCondition{Name: name, OK: true}
	}
	var unreconciled []string
	for _, s := range r.Stubs {
		if s.Bucket == evidence.StubUnreconciled {
			unreconciled = append(unreconciled, s.Slug)
		}
	}
	sort.Strings(unreconciled)
	return gateCondition{Name: name, Reason: fmt.Sprintf("unreconciled stub(s): %v", unreconciled)}
}

// checkAllImplementingStoriesClosed is the feature-closure gate's
// condition 3 — see this file's top doc comment for why it is a real,
// separate check rather than being implied by conditions 1-2. stories is
// discoverImplementingStories' flat view, which already excludes
// superseded stories (D-16) — a superseded story is neither open nor
// closed in any sense this condition needs to police; its successor
// carries the same implements edges and is the one that must close.
func checkAllImplementingStoriesClosed(stories []implementingStoryEdges, mdl *model.Model) gateCondition {
	// Display resolution (L-M13(1)): the class word, the closed state
	// word, and the stor(y/ies) alternation (displayAlternation) resolve;
	// the still-open REFS and the spec citations stay identity.
	storyWord := mdl.DisplayClass("story")
	closedWord := mdl.DisplayState("story", "closed")
	name := "3. every implementing " + storyWord + " " + closedWord + " (03 §The feature fold / §Closure ritual)"
	var open []string
	for _, s := range stories {
		if !s.Closed {
			open = append(open, s.SpecRef)
		}
	}
	if len(open) == 0 {
		return gateCondition{Name: name, OK: true}
	}
	sort.Strings(open)
	return gateCondition{Name: name, Reason: fmt.Sprintf("implementing %s not yet %s: %v",
		displayAlternation(storyWord, mdl.DisplayClassPlural("story")), closedWord, open)}
}

// checkFeatureSpecStaleCondition is the feature-closure gate's condition 4
// (spec/finding-identity ac-4, ledger L-N2): the spec-stale budget is a
// UNION over every CLOSED implementing story's ARCHIVED deviation report
// (findings: + not-resurfaced:, store.ZoneArchive — the closure ritual's
// active→archive move, store.ArchiveMove, carries a story's frozen report
// along with its spec.md) plus the feature's own report (its own active-zone
// findings: + not-resurfaced:) — the actual cross-report X-18 fix: an
// accepted deviation recorded in one story's archived report counts EXACTLY
// ONCE toward the feature-close budget, never zero (silently dropped because
// the feature's own report never independently reproduced it) and never
// twice (double-counted across the story and feature reports
// independently). evidence.SpecStale's own AdditionalSets union (unique
// content identity, align.Identity) is the mechanism; this function is only
// responsible for GATHERING the right sets.
//
// A story not yet closed contributes nothing (s.Closed false skips it,
// never an error) — it has no archived report to read yet; condition 3
// already separately blocks the feature from closing while any implementing
// story remains open, but every condition here is computed unconditionally
// (this gate never short-circuits) so this must degrade gracefully rather
// than operationally fail.
//
// Trigger (a)'s own-text join uses ONLY the feature's own declared AC ids
// against the feature's own report — both its findings: (Findings) and its own
// not-resurfaced: (OwnNotResurfaced), which share the feature's AC-id
// namespace — never AdditionalSets (see that field's own doc comment): a
// story's archived finding id colliding with a feature AC id (both commonly
// short forms like "ac-1") must never be misread as the feature's own text
// having been targeted (judged-spec-stale-own-text-not-resurfaced).
func checkFeatureSpecStaleCondition(root string, spec *artifact.SpecFrontmatter, manifest *store.Manifest, stories []implementingStoryEdges, mdl *model.Model) (gateCondition, error) {
	name := "4. no unresolved spec-stale flag"

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
		return gateCondition{}, fmt.Errorf("closure gate: internal error: resolved spec has an invalid id: %w", err)
	}
	own, err := loadDeviationReportIfExists(store.DeviationReportPath(root, store.ZoneActive, specRef.Name))
	if err != nil {
		return gateCondition{}, err
	}

	var ownFindings, ownNotResurfaced []artifact.Finding
	additional := make([][]artifact.Finding, 0, 2*len(stories))
	if own != nil {
		ownFindings = own.Findings
		// The feature's OWN not-resurfaced: shares the feature's AC-id
		// namespace, so it feeds trigger (a) too (OwnNotResurfaced) — never
		// AdditionalSets, which is for cross-report story archives only
		// (judged-spec-stale-own-text-not-resurfaced).
		ownNotResurfaced = own.NotResurfaced
	}

	storiesUnioned := 0
	for _, s := range stories {
		if !s.Closed {
			continue
		}
		storyRef, err := artifact.ParseRef(s.SpecRef)
		if err != nil {
			// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
			return gateCondition{}, fmt.Errorf("closure gate: internal error: implementing story has an invalid id: %w", err)
		}
		archived, err := loadDeviationReportIfExists(store.DeviationReportPath(root, store.ZoneArchive, storyRef.Name))
		if err != nil {
			return gateCondition{}, err
		}
		if archived == nil {
			continue
		}
		additional = append(additional, archived.Findings, archived.NotResurfaced)
		storiesUnioned++
	}

	featureACIDs := make(map[string]bool, len(spec.AcceptanceCriteria))
	for _, ac := range spec.AcceptanceCriteria {
		featureACIDs[ac.ID] = true
	}
	threshold := 0
	if manifest != nil && manifest.Audit != nil {
		threshold = manifest.Audit.DeviationsStaleThreshold
	}

	result := evidence.SpecStale(evidence.SpecStaleInput{
		Findings:         ownFindings,
		OwnNotResurfaced: ownNotResurfaced,
		AdditionalSets:   additional,
		StoryACIDs:       featureACIDs,
		Threshold:        threshold,
	})
	if !result.Flagged {
		return gateCondition{Name: name, OK: true}, nil
	}
	// Display resolution (L-M13(1)): the class words and the closed state
	// word resolve; the finding ids/counts stay identity.
	featureWord := mdl.DisplayClass("feature")
	storyWord := mdl.DisplayClass("story")
	closedWord := mdl.DisplayState("story", "closed")
	return gateCondition{Name: name, Reason: fmt.Sprintf(
		"spec-stale: own-text finding(s) %v, accepted-deviation count %d (threshold %d) [union over the %s's own report + %d %s implementing %s archive(s)]",
		result.OwnTextFindingIDs, result.AcceptedDeviationCount, threshold, featureWord, storiesUnioned, closedWord, storyWord)}, nil
}
