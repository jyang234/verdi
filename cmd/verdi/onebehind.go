// SI-231 (docs/superpowers/invention-ledger.md; owner decision D4, recorded
// 2026-09-22): does HEAD itself carry a COMMITTED one-behind alignment
// report for a spec — the shape a CI close checkout produces, since a CI
// checkout carries only committed state and can never have the LIVING,
// uncommitted report closuregate.go's condition 4 (checkDispositionCompleteCondition)
// and align.go's runAlignForSpec freeze fork otherwise both require. 03
// §Gates already names this shape for the merge gate's own fresh report
// ("the committed record's `covers` names the content-final head it
// audited, one-behind by construction"); SI-231 extends closure's freeze
// and condition 4 to recognize it too.
//
// ONE predicate decides it (ledger contract item 1) so the rule can never
// drift between its two consumers: closuregate.go's checkDispositionCompleteCondition
// (both the story condition 4 and, by reuse, the feature condition 6) and
// align.go's runAlignForSpec freeze fork. Read-only, through internal/gitx
// alone — no new git primitive: RevParse alone answers the parent-count
// question (a second call's failure IS the "no second parent" answer, not an
// operational error — see soleParent), DiffNameStatus finds the changed
// paths, and Show reads the report's own committed content, decoupled from
// any incidental local disk state (a caller's clean checkout, or an
// operator's possibly-dirty working tree elsewhere, never matters to this
// answer).
package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// oneBehindOutcome is evaluateOneBehindReport's result.
type oneBehindOutcome struct {
	// Accepted is the predicate's yes/no verdict.
	Accepted bool
	// Reason names the one clause that failed, with its concrete evidence
	// (the ledger contract's "it returns which clause failed"); empty
	// exactly when Accepted is true.
	Reason string
	// Parent, Report, and Body are populated only when Accepted: HEAD's
	// sole parent (the value every accepting consumer keeps as the frozen
	// report's own `covers` — the content-final head it audited), the
	// decoded committed frontmatter, and its raw markdown body — so a
	// caller that accepts never needs a second git read to act on it.
	Parent string
	Report *artifact.DeviationFrontmatter
	Body   []byte
}

// evaluateOneBehindReport evaluates the five clauses in the order the
// ledger contract states them, stopping at (and naming) the first one that
// fails: (1) head has exactly one parent; (2) the parent..head diff changes
// exactly one path, spec's own deviation-report.md, added or modified; (3)
// the committed report is not frozen; (4) its `covers` equals head's
// parent; (5) every finding is dispositioned.
func evaluateOneBehindReport(ctx context.Context, root, specName, head string) (oneBehindOutcome, error) {
	parent, hasOneParent, err := soleParent(ctx, root, head)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	if !hasOneParent {
		return oneBehindOutcome{Reason: fmt.Sprintf("%s is not a single-parent commit (a root commit or a merge) — SI-231 requires exactly one parent", head)}, nil
	}

	reportRelPath := store.DeviationReportRelPath(store.ZoneActive, specName)
	entries, err := gitx.DiffNameStatus(ctx, root, parent, head)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	if len(entries) != 1 {
		return oneBehindOutcome{Reason: fmt.Sprintf("%s..%s changes %d path(s), not exactly one — SI-231 requires HEAD's sole change to be %s", parent, head, len(entries), reportRelPath)}, nil
	}
	entry := entries[0]
	if entry.Path != reportRelPath || (entry.Status != "A" && entry.Status != "M") {
		return oneBehindOutcome{Reason: fmt.Sprintf("%s..%s's sole changed path is %s (status %s), not an added/modified %s", parent, head, entry.Path, entry.Status, reportRelPath)}, nil
	}

	raw, err := gitx.Show(ctx, root, head, reportRelPath)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: reading %s at %s: %w", specName, reportRelPath, head, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %s at %s: %w", specName, reportRelPath, head, err)
	}
	report, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %s at %s failed to decode: %w", specName, reportRelPath, head, err)
	}

	if report.Frozen != nil {
		return oneBehindOutcome{Reason: fmt.Sprintf("the committed report at %s (%s) is already frozen (at %s, commit %s)", head, reportRelPath, report.Frozen.At, report.Frozen.Commit)}, nil
	}
	if report.Covers != parent {
		return oneBehindOutcome{Reason: fmt.Sprintf("the committed report at %s covers %s, not HEAD's parent %s", head, report.Covers, parent)}, nil
	}

	var undispositioned []string
	for _, f := range report.Findings {
		if !f.Dispositioned() {
			undispositioned = append(undispositioned, f.ID)
		}
	}
	if len(undispositioned) > 0 {
		sort.Strings(undispositioned)
		return oneBehindOutcome{Reason: fmt.Sprintf("the committed report at %s has undispositioned finding(s) %v", head, undispositioned)}, nil
	}

	return oneBehindOutcome{Accepted: true, Parent: parent, Report: report, Body: body}, nil
}

// soleParent reports head's one and only parent SHA, using RevParse alone
// (no new git primitive): "<head>^" resolves the first parent, and its
// failure means head has none (a root commit) — by the time this runs, root
// is already a validated repository and head an already-resolved commit (every
// caller derives it from gitx.RevParse/gitx.CurrentBranch upstream), so that
// failure is always a legitimate "no parent" answer here, never a swallowed
// operational error. "<head>^2" resolving successfully means a second parent
// exists (head is a merge commit), so hasOneParent is false in both the
// no-parent and multi-parent case — "exactly one parent" is the only state
// that returns true.
func soleParent(ctx context.Context, root, head string) (parent string, hasOneParent bool, err error) {
	parent, perr := gitx.RevParse(ctx, root, head+"^")
	if perr != nil {
		return "", false, nil
	}
	if _, serr := gitx.RevParse(ctx, root, head+"^2"); serr == nil {
		return "", false, nil
	}
	return parent, true, nil
}
