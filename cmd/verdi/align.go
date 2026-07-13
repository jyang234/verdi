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
	"strings"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// alignDeps is cmdAlign's injectable dependency set — the same seam
// feature.go's syncDeps establishes, so tests can supply an
// upstream.FakeRunner and a fake judge command instead of the real,
// network-needing toolchain/judge (CLAUDE.md: "no network in any test").
type alignDeps struct {
	Runner        upstream.Runner
	JudgeCmd      []string
	JudgeRequired bool
}

// cmdAlign is `verdi align`'s entry point, invoked by dispatch.go: resolves
// the store root and manifest, wires the real upstream.Runner and
// verdi.yaml's align: block, then delegates to runAlign.
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

	var deps alignDeps
	if manifest.Toolchain != nil {
		deps.Runner = upstream.RealRunner{Module: manifest.Toolchain.Module, Commit: manifest.Toolchain.Commit, Dir: root}
	}
	if manifest.Align != nil {
		deps.JudgeCmd = manifest.Align.JudgeCmd
		deps.JudgeRequired = manifest.Align.JudgeRequired
	}

	return runAlign(ctx, root, freeze, deps, stdout, stderr)
}

// runAlign is the testable core: given an already-resolved root and
// injected deps, resolve the build-head spec (from the current branch,
// feature/<name> — align takes no story/spec argument), run
// internal/align.Generate, and write deviation-report.md into the spec's
// directory.
func runAlign(ctx context.Context, root string, freeze bool, deps alignDeps, stdout, stderr io.Writer) int {
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	// Design-branch mode (03 §Decision-conflict gate; 05 §CLI: "on a
	// design branch, grows a decision-conflict-report mode") — the two
	// modes share this one command; see align_design.go.
	if strings.HasPrefix(branch, "design/") {
		return runDesignAlign(ctx, root, freeze, deps, stdout, stderr)
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
		Runner:           deps.Runner,
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
