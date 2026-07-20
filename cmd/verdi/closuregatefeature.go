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

	// Condition 4 (spec-stale): reused verbatim from the story gate, called
	// with the feature spec instead of a story spec. checkSpecStaleCondition
	// reads specs/active/<name>/deviation-report.md and, absent, reports
	// "trivially unflagged" (its own doc comment: "a story with no build
	// activity yet cannot be spec-stale") — the ordinary case for a
	// round-four feature, which is never built directly (03 §Lifecycle:
	// stories are the unit of build; a feature is downward-blind). The
	// condition still fires honestly on the rare case a feature spec's own
	// directory does carry a deviation-report.md (e.g. a grandfathered v0
	// feature that WAS built directly, A8).
	cond4raw, err := checkSpecStaleCondition(root, spec, manifest)
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
