// verdi gate's declared-decision-conflict spec-MR condition (03 §Decision-
// conflict gate: "All declared conflicts resolved and all judged findings
// dispositioned is a merge-blocking condition ... on the spec MR"; 05
// §CLI's gate row: "on spec MRs additionally blocks on unresolved declared
// decision conflicts").
//
// WIRING (W3 merge reconciliation): this condition is now wired. gate.go's
// cmdGate dispatches on the "design/" branch prefix (mirroring align.go's
// own runAlign→runDesignAlign split) into runSpecMRGate (below), which
// resolves the design branch's spec via storyresolve.ResolveDesignSpec and
// evaluates this condition — the spec-MR analogue of the build-branch merge
// conditions, which never run on a design branch and are the only ones that
// run on a build branch. checkDeclaredDecisionConflicts stays a drop-in
// sibling of gate.go's build-branch check family
// (checkAcceptedOnDefaultBranch / checkNoACViolated /
// checkFreshFullyDispositioned): ONE call site, reached only on the spec-MR
// path.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// runSpecMRGate is the spec-MR (design-branch) gate path, wired from
// gate.go's cmdGate on the "design/" branch prefix (WIRING NOTE above, now
// resolved at merge). It mirrors gate.go's runGate but resolves the design
// branch's OWN spec (storyresolve.ResolveDesignSpec — feature or story class,
// the same resolver `verdi align`'s design-branch mode uses) and evaluates
// the spec-MR condition set: declared-decision-conflict
// (checkDeclaredDecisionConflicts) and, as of V1-P7, review-thread
// resolution (checkReviewThreadsCondition, gate_threads.go — "V1-P7's
// review-thread condition joins this same set later," now resolved at this
// phase). The build-branch merge conditions never run here, and neither of
// these conditions ever runs on a build branch (03 §Decision-conflict
// gate: "the design-branch analogue of the build-branch merge gate's
// fresh-report requirement"). head is the design branch head the report
// must cover. f is the forge to query for checkReviewThreadsCondition —
// injected, mirroring sync.go's cmdSync/runSync split (CLAUDE.md: no
// network in any test), so tests drive this core directly with nil or a
// hermetic fake/httptest forge; only cmdGate (gate.go) ever builds a live
// one (buildForgeBestEffort, gate_threads.go). defaultBranchRef is also
// injected (mirroring runGate's own parameter, and closuregate.go's
// runClosureGate) rather than resolved internally, so tests control it
// directly instead of depending on a fixturegit repo's actual git
// default-branch detection.
func runSpecMRGate(ctx context.Context, root, branch string, f forge.Forge, defaultBranchRef string, stdout, stderr io.Writer) int {
	return runSpecMRGateWithConflict(ctx, root, branch, f, defaultBranchRef, "", localLifecycleConflictProvider{root: root}, stdout, stderr)
}

func runSpecMRGateWithConflict(ctx context.Context, root, branch string, f forge.Forge, defaultBranchRef, requestPath string, provider policyconflict.VerdictProvider, stdout, stderr io.Writer) int {
	spec, err := storyresolve.ResolveDesignSpec(root, branch)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "gate: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	conflict, err := runConflictGate(ctx, root, conflictGateInput{
		RequestPath: requestPath,
		Phase:       contextcompile.PhaseDesign,
		Spec:        spec.ID,
		Candidate:   true,
		Branch:      branch,
		Head:        head,
	}, provider)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	cond1, err := checkDeclaredDecisionConflicts(ctx, root, specRef.Name, head)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	cond2, err := checkReviewThreadsCondition(ctx, f, defaultBranchRef, branch)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}

	conds := []gateCondition{cond1, cond2}
	if conflict.Adopted {
		conds = append(conds, conflictCondition(conflict.Result))
	}
	numberSpecMRConditions(conds)
	return reportGateConditions(stdout, conds)
}

// numberSpecMRConditions prefixes each spec-MR condition's Name with its
// 1-based position on this path, so the rendered ordinals start at "1."
// rather than carrying a build-branch merge-condition number: the spec-MR
// set is disjoint from runGate's build-branch conditions (03 §Decision-
// conflict gate) and numbers independently. checkDeclaredDecisionConflicts
// (and V1-P7's review-thread condition, when it joins this set) leaves its
// Name unnumbered; the ordinal is derived here from actual position.
func numberSpecMRConditions(conds []gateCondition) {
	for i := range conds {
		conds[i].Name = fmt.Sprintf("%d. %s", i+1, conds[i].Name)
	}
}

// checkDeclaredDecisionConflicts is the spec-MR analogue of gate.go's
// checkFreshFullyDispositioned: present, `covers` == head, a computed
// section equal to the one the records at head compute, and every finding
// (computed — declared-edge completeness — and judged) dispositioned
// (align.DecisionReviewReady, decision_report.go) — 03's merge-blocking
// condition on the spec MR. A missing report fails the condition by name
// rather than erroring, mirroring checkFreshFullyDispositioned's own "no
// report at all" case exactly.
//
// The gate recomputes (03 §Decision-conflict gate; design §5, SI-262): the
// computed section is recomputed from the records of the head commit — never
// the working tree — by align.ComputeDecisionEdges, the one computation
// `verdi align` also runs, and any difference from the report's computed
// findings fails the condition, naming each. A disposition, note, or any
// other text written onto a computed finding therefore cannot supply a
// missing target, conflict, or successor. A carried replacement needs the
// default branch's first-parent history (SI-270): where it is missing (a
// shallow clone, or a default branch that cannot be resolved) the recompute
// reads "acceptance unproven", so the gate never passes on an unproven
// acceptance, and a difference it causes names the missing history rather
// than advising a re-align that cannot restore it (recomputeHint). The
// judged section is judge output and is not recomputed.
func checkDeclaredDecisionConflicts(ctx context.Context, root, specName, head string) (gateCondition, error) {
	name := "spec-MR: declared decision conflicts resolved and judged findings dispositioned"
	path := store.DecisionConflictReportPath(root, store.ZoneActive, specName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return gateCondition{Name: name, Reason: fmt.Sprintf("no decision-conflict-report.md found at %s (run `verdi align`)", path)}, nil
		}
		return gateCondition{}, fmt.Errorf("reading %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("%s: %v", path, err)}, nil
	}
	decoded, err := artifact.DecodeDecisionConflict(fm)
	if err != nil {
		return gateCondition{Name: name, Reason: fmt.Sprintf("%s failed to decode: %v", path, err)}, nil
	}
	if decoded.Covers != head {
		return gateCondition{Name: name, Reason: fmt.Sprintf("stale: covers %s, head is %s (run `verdi align` again)", decoded.Covers, head)}, nil
	}

	recomputed, err := align.ComputeDecisionEdges(ctx, objsupersede.CommitTree{Root: root, Commit: head}, specName, objsupersede.NewHistory(ctx, root))
	if errors.Is(err, align.ErrSpecNotInTree) {
		return gateCondition{Name: name, Reason: fmt.Sprintf("the computed section cannot be recomputed from the records at %s: %v", head, err)}, nil
	}
	if err != nil {
		return gateCondition{}, fmt.Errorf("recomputing the decision-conflict report's computed section at %s: %w", head, err)
	}
	if diffs := diffComputedFindings(decoded.Findings, recomputed); len(diffs) > 0 {
		msgs := make([]string, len(diffs))
		for i, d := range diffs {
			msgs[i] = d.msg
		}
		return gateCondition{Name: name, Reason: fmt.Sprintf("the report's computed section differs from the records at %s: %s %s", head, strings.Join(msgs, "; "), recomputeHint(diffs, recomputed))}, nil
	}

	ok, undispositioned := align.DecisionReviewReady(decoded)
	if !ok {
		return gateCondition{Name: name, Reason: fmt.Sprintf("undispositioned/unresolved finding(s): %v", undispositioned)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}

// computedDiff is one difference diffComputedFindings names: the id of the
// finding it concerns, and the message naming it.
type computedDiff struct{ id, msg string }

// recomputeHint is the remedy a recomputed difference's reason ends with:
// re-run `verdi align`, unless the records at the head leave an acceptance
// unproven for a differing finding — history this checkout lacks (a
// shallow clone, an unresolvable default branch; SI-270), which re-running
// align here cannot restore. The hint then names that missing history, each
// differing finding with its witness, and advises a re-align only for any
// other difference.
func recomputeHint(diffs []computedDiff, recomputed []artifact.ConflictFinding) string {
	witness := map[string]string{}
	for _, f := range recomputed {
		if w, ok := acceptanceUnprovenWitness(f); ok {
			if _, seen := witness[f.ID]; !seen {
				witness[f.ID] = w
			}
		}
	}
	var missing []string
	named, other := map[string]bool{}, false
	for _, d := range diffs {
		w, ok := witness[d.id]
		switch {
		case !ok:
			other = true
		case !named[d.id]:
			named[d.id] = true
			missing = append(missing, d.id+": "+w)
		}
	}
	if len(missing) == 0 {
		return "(run `verdi align` again)"
	}
	hint := "(this checkout cannot prove an acceptance, and re-running `verdi align` cannot restore the missing history: " + strings.Join(missing, "; ")
	if other {
		hint += "; run `verdi align` again for the other differences"
	}
	return hint + ")"
}

// acceptanceUnprovenWitness returns the witness of a recomputed computed
// finding whose records leave an establishing successor's acceptance
// unproven (objsupersede.ReasonAcceptanceUnproven: the text the core
// renders, "acceptance unproven: <witness>", its prefix derived from the
// core rather than restated), and false for every other finding.
func acceptanceUnprovenWitness(f artifact.ConflictFinding) (string, bool) {
	const marker = "\x00"
	text, err := objsupersede.Result{Outcome: objsupersede.Unresolved, Reason: objsupersede.ReasonAcceptanceUnproven, Detail: marker}.Text()
	prefix, ok := strings.CutSuffix(text, marker)
	if err != nil || !ok || f.Kind != artifact.FindingComputed || f.Dispositioned() {
		return "", false
	}
	return strings.CutPrefix(f.Text, prefix)
}

// diffComputedFindings names every difference between a report's computed
// findings and the recomputed ones, matched by id (and, for a repeated id,
// by position among that id's findings): a missing or extra finding, and
// any field that differs — text, disposition, note, target ref, or routed
// owners (element by element, never by a joined rendering). Judged findings
// are not compared. An empty result means the sections are equal.
// TestDiffComputedFindings_ComparesEveryField fails when
// artifact.ConflictFinding gains a field this does not compare.
func diffComputedFindings(reported, recomputed []artifact.ConflictFinding) []computedDiff {
	byID := func(fs []artifact.ConflictFinding) (map[string][]artifact.ConflictFinding, []string) {
		m := map[string][]artifact.ConflictFinding{}
		var order []string
		for _, f := range fs {
			if f.Kind != artifact.FindingComputed {
				continue
			}
			if _, seen := m[f.ID]; !seen {
				order = append(order, f.ID)
			}
			m[f.ID] = append(m[f.ID], f)
		}
		return m, order
	}
	got, gotOrder := byID(reported)
	want, order := byID(recomputed)
	for _, id := range gotOrder {
		if _, ok := want[id]; !ok {
			order = append(order, id)
		}
	}
	var diffs []computedDiff
	for _, id := range order {
		g, w := got[id], want[id]
		for i := 0; i < len(g) || i < len(w); i++ {
			switch {
			case i >= len(g):
				diffs = append(diffs, computedDiff{id, fmt.Sprintf("missing computed finding %s (the records compute %q)", id, w[i].Text)})
			case i >= len(w):
				diffs = append(diffs, computedDiff{id, fmt.Sprintf("extra computed finding %s (the records do not compute it)", id)})
			default:
				g, w := g[i], w[i]
				for _, fd := range []struct {
					field, got, want string
					equal            bool
				}{
					{"text", quotedOrNone(g.Text), quotedOrNone(w.Text), g.Text == w.Text},
					{"disposition", quotedOrNone(string(g.Disposition)), quotedOrNone(string(w.Disposition)), g.Disposition == w.Disposition},
					{"note", quotedOrNone(g.Note), quotedOrNone(w.Note), g.Note == w.Note},
					{"target_ref", quotedOrNone(g.TargetRef), quotedOrNone(w.TargetRef), g.TargetRef == w.TargetRef},
					{"routed_owners", listOrNone(g.RoutedOwners), listOrNone(w.RoutedOwners), slices.Equal(g.RoutedOwners, w.RoutedOwners)},
				} {
					if !fd.equal {
						diffs = append(diffs, computedDiff{id, fmt.Sprintf("%s: %s is %s, the records compute %s", id, fd.field, fd.got, fd.want)})
					}
				}
			}
		}
	}
	return diffs
}

// quotedOrNone renders a compared field for a difference: quoted, or
// "none" when empty.
func quotedOrNone(s string) string {
	if s == "" {
		return "none"
	}
	return fmt.Sprintf("%q", s)
}

// listOrNone renders a compared list for a difference, each element
// quoted, so lists that join alike (["a, b"] and ["a" "b"]) read apart; or
// "none" when empty.
func listOrNone(l []string) string {
	if len(l) == 0 {
		return "none"
	}
	return fmt.Sprintf("%q", l)
}
