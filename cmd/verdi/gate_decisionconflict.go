// verdi gate's declared-decision-conflict spec-MR condition (03 §Decision-
// conflict gate: "All declared conflicts resolved and all judged findings
// dispositioned is a merge-blocking condition ... on the spec MR"; 05
// §CLI's gate row: "on spec MRs additionally blocks on unresolved declared
// decision conflicts").
//
// WIRING NOTE (read before deleting this comment): as of this phase
// (V1-P5), cmd/verdi/gate.go's runGate ALWAYS resolves the build-branch
// spec (storyresolve.ResolveBuildSpec, feature/<name> only) and evaluates
// exactly three build-branch conditions — it has no spec-MR / design-
// branch code path AT ALL yet for this condition to join. That branch-kind
// dispatch is V1-P4's job (lifecycle verbs, run concurrently in a sibling
// worktree this same wave) — V1-P4's own brief deliberately left this
// condition unwired because the capability below (internal/align's
// design-branch decision-conflict machinery) did not exist until this
// phase. checkDeclaredDecisionConflicts is written to be a drop-in
// addition to gate.go's existing gateCondition-returning check function
// family (checkAcceptedOnDefaultBranch / checkNoACViolated /
// checkFreshFullyDispositioned) the moment a spec-MR branch path exists to
// call it from — ONE call site, no gate.go edits needed beyond that call.
// If gate.go has already grown a spec-MR path by merge-prep, wire this
// function in there directly instead of re-deriving the condition; if its
// shape has moved in some other way, leave the wiring to the merge
// reviewer per this phase's own instructions.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/align"
	"github.com/OWNER/verdi/internal/artifact"
)

// checkDeclaredDecisionConflicts is the spec-MR analogue of gate.go's
// checkFreshFullyDispositioned: present, `covers` == head, and every
// finding (computed — declared-edge completeness — and judged) is
// dispositioned (align.DecisionReviewReady, decision_report.go) — 03's
// merge-blocking condition on the spec MR. A missing report fails the
// condition by name rather than erroring, mirroring
// checkFreshFullyDispositioned's own "no report at all" case exactly.
func checkDeclaredDecisionConflicts(root, specName, head string) (gateCondition, error) {
	name := "4. spec-MR: declared decision conflicts resolved and judged findings dispositioned"
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
