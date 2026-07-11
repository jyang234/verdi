// verdi align [--freeze] (05 §CLI, PLAN.md Phase 8): generates/refreshes
// deviation-report.md for the current build branch's spec, inferred from
// the checked-out branch name (feature/<name>, cut by `verdi feature
// start` — see internal/storyresolve.ResolveBuildSpec; align takes no
// story/spec argument, matching 05 §CLI's table). --freeze produces the
// closure edition (a Frozen stamp at the build head).
//
// Exit contract (CLAUDE.md 0/1/2, PLAN.md Phase 8's exit criteria):
// 0 report written; 1 align.judge_required is true and no judge produced a
// judged section; 2 every other operational failure.
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
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/storyresolve"
	"github.com/OWNER/verdi/internal/upstream"
)

// cmdAlign is `verdi align`'s entry point, invoked by dispatch.go.
func cmdAlign(args []string, stdout, stderr io.Writer) int {
	freeze := false
	for _, a := range args {
		if a == "--freeze" {
			freeze = true
			continue
		}
		fmt.Fprintf(stderr, "align: unexpected argument %q; usage: verdi align [--freeze]\n", a)
		return 2
	}

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	manifest, err := loadManifest(root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	spec, err := storyresolve.ResolveBuildSpec(root, branch)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	covers, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	var runner upstream.Runner
	if manifest.Toolchain != nil {
		runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	var judgeCmd []string
	judgeRequired := false
	if manifest.Align != nil {
		judgeCmd = manifest.Align.JudgeCmd
		judgeRequired = manifest.Align.JudgeRequired
	}

	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "align: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	reportPath := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "deviation-report.md")

	existingReport, err := loadExistingReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	if existingReport != nil && existingReport.Frozen != nil {
		fmt.Fprintf(stderr, "align: %s is already frozen (at %s, commit %s); a frozen alignment report is immutable\n", reportPath, existingReport.Frozen.At, existingReport.Frozen.Commit)
		return 1
	}
	var existingFindings []artifact.Finding
	if existingReport != nil {
		existingFindings = existingReport.Findings
	}

	in := align.Input{
		Root:             root,
		Runner:           runner,
		Spec:             spec,
		Covers:           covers,
		JudgeCmd:         judgeCmd,
		JudgeRequired:    judgeRequired,
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

	report, err := align.Generate(ctx, in)
	if err != nil {
		var reqAbsent *align.ErrJudgeRequiredAbsent
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

	fmt.Fprintf(stdout, "align: wrote %s (covers %s, %d findings)\n", reportPath, report.Frontmatter.Covers, len(report.Frontmatter.Findings))
	if freeze {
		fmt.Fprintf(stdout, "align: frozen at %s\n", report.Frontmatter.Frozen.At)
	}
	return 0
}

// loadExistingReport reads and strict-decodes a prior deviation-report.md,
// if one exists — (nil, nil) for a first run (no file yet). A file that
// exists but fails to decode is a real, surfaced error (CLAUDE.md: "silence
// is never a pass" — a broken report on disk must never be treated as "no
// report").
func loadExistingReport(path string) (*artifact.DeviationFrontmatter, error) {
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
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, nil
}
