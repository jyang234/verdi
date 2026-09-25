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
// shallow clone) the recompute reads "acceptance unproven", so the gate
// never passes on an unproven acceptance. The judged section is judge
// output and is not recomputed.
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
		return gateCondition{Name: name, Reason: fmt.Sprintf("the report's computed section differs from the records at %s: %s (run `verdi align` again)", head, strings.Join(diffs, "; "))}, nil
	}

	ok, undispositioned := align.DecisionReviewReady(decoded)
	if !ok {
		return gateCondition{Name: name, Reason: fmt.Sprintf("undispositioned/unresolved finding(s): %v", undispositioned)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}

// diffComputedFindings names every difference between a report's computed
// findings and the recomputed ones, matched by id (and, for a repeated id,
// by position among that id's findings): a missing or extra finding, and
// any field that differs — text, disposition, note, target ref, or routed
// owners. Judged findings are not compared. An empty result means the
// sections are equal.
func diffComputedFindings(reported, recomputed []artifact.ConflictFinding) []string {
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
	var diffs []string
	for _, id := range order {
		g, w := got[id], want[id]
		for i := 0; i < len(g) || i < len(w); i++ {
			switch {
			case i >= len(g):
				diffs = append(diffs, fmt.Sprintf("missing computed finding %s (the records compute %q)", id, w[i].Text))
			case i >= len(w):
				diffs = append(diffs, fmt.Sprintf("extra computed finding %s (the records do not compute it)", id))
			default:
				for _, fd := range []struct{ field, got, want string }{
					{"text", g[i].Text, w[i].Text},
					{"disposition", string(g[i].Disposition), string(w[i].Disposition)},
					{"note", g[i].Note, w[i].Note},
					{"target_ref", g[i].TargetRef, w[i].TargetRef},
					{"routed_owners", strings.Join(g[i].RoutedOwners, ", "), strings.Join(w[i].RoutedOwners, ", ")},
				} {
					if fd.got != fd.want {
						diffs = append(diffs, fmt.Sprintf("%s: %s is %s, the records compute %s", id, fd.field, quotedOrNone(fd.got), quotedOrNone(fd.want)))
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
