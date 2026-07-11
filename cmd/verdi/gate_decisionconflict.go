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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/align"
	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// runSpecMRGate is the spec-MR (design-branch) gate path, wired from
// gate.go's cmdGate on the "design/" branch prefix (WIRING NOTE above, now
// resolved at merge). It mirrors gate.go's runGate but resolves the design
// branch's OWN spec (storyresolve.ResolveDesignSpec — feature or story class,
// the same resolver `verdi align`'s design-branch mode uses) and evaluates
// the spec-MR condition set: for this phase, the single
// declared-decision-conflict condition (checkDeclaredDecisionConflicts).
// V1-P7's review-thread condition joins this same set later; the build-branch
// merge conditions never run here, and this condition never runs on a build
// branch (03 §Decision-conflict gate: "the design-branch analogue of the
// build-branch merge gate's fresh-report requirement"). head is the design
// branch head the report must cover.
func runSpecMRGate(ctx context.Context, root, branch string, stdout, stderr io.Writer) int {
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
	cond, err := checkDeclaredDecisionConflicts(root, specRef.Name, head)
	if err != nil {
		fmt.Fprintln(stderr, "gate:", err)
		return 2
	}
	conds := []gateCondition{cond}
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
// checkFreshFullyDispositioned: present, `covers` == head, and every
// finding (computed — declared-edge completeness — and judged) is
// dispositioned (align.DecisionReviewReady, decision_report.go) — 03's
// merge-blocking condition on the spec MR. A missing report fails the
// condition by name rather than erroring, mirroring
// checkFreshFullyDispositioned's own "no report at all" case exactly.
func checkDeclaredDecisionConflicts(root, specName, head string) (gateCondition, error) {
	name := "spec-MR: declared decision conflicts resolved and judged findings dispositioned"
	path := filepath.Join(root, ".verdi", "specs", "active", specName, "decision-conflict-report.md")

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

	ok, undispositioned := align.DecisionReviewReady(decoded)
	if !ok {
		return gateCondition{Name: name, Reason: fmt.Sprintf("undispositioned/unresolved finding(s): %v", undispositioned)}, nil
	}
	return gateCondition{Name: name, OK: true}, nil
}
