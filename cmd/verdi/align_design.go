// verdi align's design-branch mode (03 §Decision-conflict gate; 05 §CLI's
// `align` design-branch mode row, R4-I-7): generates/refreshes
// decision-conflict-report.md for the current design branch's spec,
// inferred from the checked-out branch name (design/<name>, cut by `verdi
// design start`). Split from align.go (the build-branch mode's own file)
// rather than folded in, matching this module's one-file-per-topic
// convention — align.go's runAlign dispatches here on branch prefix, the
// smallest possible touch to the existing build-branch path.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/align"
	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// runDesignAlign is the design-branch mode's testable core, mirroring
// runAlign's shape exactly (align.go) but resolving via
// storyresolve.ResolveDesignSpec and writing
// decision-conflict-report.md instead of deviation-report.md.
func runDesignAlign(ctx context.Context, root string, freeze bool, deps alignDeps, stdout, stderr io.Writer) int {
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	spec, err := storyresolve.ResolveDesignSpec(root, branch)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	covers, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "align: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	reportPath := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "decision-conflict-report.md")

	existingReport, err := loadExistingDecisionReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	if existingReport != nil && existingReport.Frozen != nil {
		fmt.Fprintf(stderr, "align: %s is already frozen (at %s, commit %s); a frozen decision-conflict report is immutable\n", reportPath, existingReport.Frozen.At, existingReport.Frozen.Commit)
		return 1
	}
	var existingFindings []artifact.ConflictFinding
	if existingReport != nil {
		existingFindings = existingReport.Findings
	}

	in := align.DecisionConflictInput{
		Root:             root,
		Spec:             spec,
		Covers:           covers,
		JudgeCmd:         deps.JudgeCmd,
		JudgeRequired:    deps.JudgeRequired,
		ExistingFindings: existingFindings,
	}
	if freeze {
		at, err := gitx.CommitDate(ctx, root, covers)
		if err != nil {
			fmt.Fprintln(stderr, "align:", err)
			return 2
		}
		if len(at) < 10 {
			fmt.Fprintln(stderr, "align: internal error: commit date too short to derive frozen.at")
			return 2
		}
		in.Freeze = true
		in.FrozenAt = at[:10]
	}

	report, err := align.GenerateDecisionConflict(ctx, in)
	if err != nil {
		var reqAbsent *align.ErrDecisionJudgeRequiredAbsent
		if errors.As(err, &reqAbsent) {
			fmt.Fprintln(stderr, "align:", err)
			return 1
		}
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	if err := os.WriteFile(reportPath, report.Markdown, 0o644); err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	computedStatus, judgedStatus := align.DecisionGateStatuses(report.Frontmatter)
	fmt.Fprintf(stdout, "align: wrote %s (covers %s, %d findings)\n", reportPath, report.Frontmatter.Covers, len(report.Frontmatter.Findings))
	fmt.Fprintf(stdout, "align: computed status: %s\n", statusOrUnproven(computedStatus))
	fmt.Fprintf(stdout, "align: judged status: %s\n", statusOrUnproven(judgedStatus))
	if freeze {
		fmt.Fprintf(stdout, "align: frozen at %s\n", report.Frontmatter.Frozen.At)
	}
	return 0
}

// statusOrUnproven renders an empty align.GateStatus (03's "not yet one of
// the three honest labels" state, internal/align's own doc comment) as an
// explicit, never-blank string — this command never prints a bare status
// line that could be misread as a pass.
func statusOrUnproven(s align.GateStatus) string {
	if s == "" {
		return "(not yet proven — unresolved edges or undispositioned findings remain)"
	}
	return string(s)
}

// loadExistingDecisionReport reads and strict-decodes a prior
// decision-conflict-report.md, if one exists — mirrors align.go's
// loadExistingReport exactly, for the decision-conflict schema.
func loadExistingDecisionReport(path string) (*artifact.DecisionConflictFrontmatter, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading existing %s: %w", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	decoded, err := artifact.DecodeDecisionConflict(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, nil
}
