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
// drift between its consumers: closuregate.go's checkDispositionCompleteCondition
// (both the story condition 4 and, by reuse, the feature condition 6),
// align.go's shared fork as the closure ritual enters it (close's freeze and
// --prepare's refresh, never bare `verdi align` — ruling R-W1-9), and
// closeprepare.go's own disclosure. Read-only, through internal/gitx
// alone — no new git primitive: RevParse alone answers the parent-count
// question (a second call's failure IS the "no second parent" answer, not an
// operational error — see soleParent), DiffNameStatus finds the changed
// paths, and Show reads the report's own committed content.
//
// The working tree DOES matter, in exactly one clause (the ledger row as
// amended at L3b review I-1, ruling R-W1-9): the spec's deviation-report.md
// on disk must be byte-identical to the report HEAD commits. Every
// accepting consumer acts on HEAD's committed bytes — close freezes them over
// the working-tree file — so without this clause an operator's uncommitted
// disposition change (a `verdi disposition --amend` after commit R, or a
// retraction) would be overwritten while close printed "dispositions
// preserved". A CI checkout never diverges, so the clause costs CI nothing.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
	// Acceptance also proves the working-tree file holds these exact bytes,
	// so a consumer that describes or freezes them describes the file.
	Parent string
	Report *artifact.DeviationFrontmatter
	Body   []byte
}

// evaluateOneBehindReport evaluates the six clauses in the order the
// ledger contract states them, stopping at (and naming) the first one that
// fails: (1) head has exactly one parent; (2) the parent..head diff changes
// exactly one path, spec's own deviation-report.md, added or modified; (3)
// the committed report is not frozen; (4) its `covers` equals head's
// parent; (5) every finding is dispositioned; (6) the working-tree report
// is byte-identical to the committed one.
func evaluateOneBehindReport(ctx context.Context, root, specName, head string) (oneBehindOutcome, error) {
	parent, parentRefusal, err := soleParent(ctx, root, head)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	if parentRefusal != "" {
		return oneBehindOutcome{Reason: parentRefusal}, nil
	}

	// DiffNameStatus and Show answer in REPOSITORY-root-relative paths, and
	// root is the STORE root, which sits below the git root whenever
	// store.FindRoot found a nested .verdi (L3b review m-4). Compare in git's
	// coordinates by re-basing the store-relative path through RepoPrefix,
	// as requireCleanIndex relates the two (close.go): without it a nested
	// store refuses its own report and accepts a root store's same-named
	// one. Under diff.relative git reports only this directory's paths, and
	// relative to it, so no entry can equal the re-based path: that
	// configuration refuses, never accepts.
	prefix, err := gitx.RepoPrefix(ctx, root)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	reportRepoPath := prefix + store.DeviationReportRelPath(store.ZoneActive, specName)
	entries, err := gitx.DiffNameStatus(ctx, root, parent, head)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	if len(entries) != 1 {
		return oneBehindOutcome{Reason: fmt.Sprintf("%s..%s changes %d path(s), not exactly one — SI-231 requires HEAD's sole change to be %s", parent, head, len(entries), reportRepoPath)}, nil
	}
	entry := entries[0]
	if entry.Path != reportRepoPath || (entry.Status != "A" && entry.Status != "M") {
		return oneBehindOutcome{Reason: fmt.Sprintf("%s..%s's sole changed path is %s (status %s), not an added/modified %s", parent, head, entry.Path, entry.Status, reportRepoPath)}, nil
	}

	raw, err := gitx.Show(ctx, root, head, reportRepoPath)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: reading %s at %s: %w", specName, reportRepoPath, head, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %s at %s: %w", specName, reportRepoPath, head, err)
	}
	report, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %s at %s failed to decode: %w", specName, reportRepoPath, head, err)
	}

	if report.Frozen != nil {
		return oneBehindOutcome{Reason: fmt.Sprintf("the committed report at %s (%s) is already frozen (at %s, commit %s)", head, reportRepoPath, report.Frozen.At, report.Frozen.Commit)}, nil
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

	workingPath := store.DeviationReportPath(root, store.ZoneActive, specName)
	divergence, err := workingTreeDivergence(workingPath, raw)
	if err != nil {
		return oneBehindOutcome{}, fmt.Errorf("align: evaluating SI-231 one-behind report for %s: %w", specName, err)
	}
	if divergence != "" {
		return oneBehindOutcome{Reason: fmt.Sprintf("the working-tree report differs from the committed report; commit or discard the change (%s %s the report HEAD %s commits) — to commit it, amend HEAD's own report commit, since a new commit on top would no longer cover its parent", workingPath, divergence, head)}, nil
	}

	return oneBehindOutcome{Accepted: true, Parent: parent, Report: report, Body: body}, nil
}

// workingTreeDivergence is clause (6): "" when the file at workingPath holds
// exactly the committed bytes, otherwise how it differs. An absent file
// differs (a working-tree deletion is a change like any other); any other
// read failure is operational, never a verdict.
func workingTreeDivergence(workingPath string, committed []byte) (string, error) {
	onDisk, err := os.ReadFile(workingPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "is absent, unlike", nil
	case err != nil:
		return "", fmt.Errorf("reading the working-tree report %s: %w", workingPath, err)
	case !bytes.Equal(onDisk, committed):
		return "is not byte-identical to", nil
	}
	return "", nil
}

// soleParent answers clause (1): head's one and only parent SHA, or the
// refusal naming why head has not exactly one, using RevParse alone (no new
// git primitive). "<head>^2" resolving successfully means a second parent
// exists (head is a merge commit). "<head>^" failing means git sees no
// parent at all: by the time this runs, root is already a validated
// repository and head an already-resolved commit (every caller derives it
// from gitx.RevParse/gitx.CurrentBranch upstream), so that is either a root
// commit or — in a shallow checkout — a parent beyond the shallow boundary,
// where the parent count is unknown and is named as such (L3b review m-5;
// close.yml fetches full history). A head whose first parent IS visible is
// not a boundary commit, so git reads its complete parent list and "<head>^2"
// answers truthfully even in a shallow checkout. "exactly one parent" is the
// only state that returns an empty refusal.
func soleParent(ctx context.Context, root, head string) (parent, refusal string, err error) {
	parent, perr := gitx.RevParse(ctx, root, head+"^")
	if perr != nil {
		shallow, serr := gitx.IsShallow(ctx, root)
		if serr != nil {
			return "", "", serr
		}
		if shallow {
			return "", fmt.Sprintf("%s's parent lies beyond this shallow checkout's boundary, so whether it has exactly one parent is unknown — SI-231 requires exactly one parent; fetch the full history (for example with fetch-depth: 0)", head), nil
		}
		return "", fmt.Sprintf(notSingleParentFormat, head), nil
	}
	if _, serr := gitx.RevParse(ctx, root, head+"^2"); serr == nil {
		return "", fmt.Sprintf(notSingleParentFormat, head), nil
	}
	return parent, "", nil
}

// notSingleParentFormat is clause (1)'s refusal for a head git sees in full.
// vocab:identity — non-vocabulary homograph: git's own merge commit (two parents), never the `merge` lifecycle transition word
const notSingleParentFormat = "%s is not a single-parent commit (a root commit or a merge) — SI-231 requires exactly one parent"
