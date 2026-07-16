// verdi align [--freeze] (05 §CLI, PLAN.md Phase 8): generates/refreshes
// deviation-report.md for the current build branch's spec, inferred from
// the checked-out branch name (feature/<name>, cut by `verdi feature
// start` — see internal/storyresolve.ResolveBuildSpec; align takes no
// story/spec argument, matching 05 §CLI's table). --freeze produces the
// closure edition (a Frozen stamp at the build head).
//
// verdi align --diagram-sweep <diagram-ref> (spec/judged-sweep ac-1, dc-1)
// is a THIRD, wholly on-demand mode: unlike the build-branch and
// design-branch modes, it takes its own diagram-ref argument, infers
// nothing from the checked-out branch, is never required by any gate, and
// writes a disposable sibling report — see aligndiagramsweep.go.
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
	"time"

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
	// JudgeTimeout mirrors verdi.yaml's align.judge_timeout_seconds (D6-21);
	// zero leaves internal/align's own DefaultJudgeTimeout fallback
	// unchanged (align.Input.JudgeTimeout's zero-value contract).
	JudgeTimeout time.Duration
}

// cmdAlign is `verdi align`'s entry point, invoked by dispatch.go: resolves
// the store root and manifest, wires the real upstream.Runner and
// verdi.yaml's align: block, then delegates to runAlign.
func cmdAlign(args []string, stdout, stderr io.Writer) int {
	freeze := false
	diagramRef := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--freeze":
			freeze = true
		case "--diagram-sweep":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "align: --diagram-sweep requires a <diagram-ref> argument")
				return 2
			}
			i++
			diagramRef = args[i]
		default:
			fmt.Fprintf(stderr, "align: unexpected argument %q; usage: verdi align [--freeze] | verdi align --diagram-sweep <diagram-ref>\n", a)
			return 2
		}
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
		if manifest.Align.JudgeTimeoutSeconds > 0 {
			deps.JudgeTimeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second
		}
	}

	// Diagram-sweep mode (spec/judged-sweep ac-1, dc-1): a THIRD, wholly
	// on-demand mode of this verb, dispatched before branch resolution since
	// it takes its own <diagram-ref> argument and never infers anything from
	// the checked-out branch — see aligndiagramsweep.go.
	if diagramRef != "" {
		if freeze {
			fmt.Fprintln(stderr, "align: --freeze and --diagram-sweep are mutually exclusive (a sweep report is never frozen, spec/judged-sweep dc-3)")
			return 2
		}
		return runDiagramSweepAlign(ctx, root, diagramRef, deps, stdout, stderr)
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
	return runAlignForSpec(ctx, root, spec, covers, freeze, deps, stdout, stderr)
}

// runAlignForSpec is runAlign's spec-taking core, factored out (round 6,
// spec/close-verb ac-1) so a caller that has ALREADY resolved its own spec
// by a means other than the feature/<name> build-branch convention —
// `verdi close` resolves the story via internal/storyresolve.Resolve, a
// story or spec-ref argument, never a branch name — can run the exact same
// generate-freeze-write logic runAlign uses for the frozen closure report,
// rather than duplicating it (CLAUDE.md: no copy-paste across call sites).
// runAlign itself is unchanged in behavior: it still resolves branch ->
// spec -> covers first, then delegates here.
func runAlignForSpec(ctx context.Context, root string, spec *artifact.SpecFrontmatter, covers string, freeze bool, deps alignDeps, stdout, stderr io.Writer) int {
	specRef, err := artifact.ParseRef(spec.ID)
	if err != nil {
		fmt.Fprintln(stderr, "align: internal error: resolved spec has an invalid id:", err)
		return 2
	}
	reportPath := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "deviation-report.md")

	existingReport, existingBody, err := loadExistingReport(reportPath)
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
		JudgeTimeout:     deps.JudgeTimeout,
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
		frozenAt := at[:10]

		// Freeze-in-place — the faithful freeze `verdi close` needs. When a
		// living report already covers this exact freeze commit and every
		// finding is dispositioned (the fresh, fully-dispositioned state the
		// merge gate required before merge, 03 §Gates condition 3), stamp it
		// VERBATIM (align.FreezeInPlace) rather than regenerating. Regenerating
		// re-runs the non-reproducible judge (03 §Alignment report), whose fresh
		// content-hash finding identities PreserveDispositions cannot match —
		// silently erasing every human disposition (the D6-21-exposed bug).
		// Any other state — no living report, stale covers, or an
		// undispositioned finding — falls through to the regenerate path below,
		// unchanged.
		if existingReport != nil && existingReport.Covers == covers && artifact.AllDispositioned(existingReport.Findings) {
			report, err := align.FreezeInPlace(existingReport, string(existingBody), frozenAt)
			if err != nil {
				fmt.Fprintln(stderr, "align:", err)
				return 2
			}
			if err := os.WriteFile(reportPath, report.Markdown, 0o644); err != nil {
				fmt.Fprintln(stderr, "align:", err)
				return 2
			}
			fmt.Fprintf(stdout, "align: froze %s in place (covers %s, %d findings, dispositions preserved)\n", reportPath, report.Frontmatter.Covers, len(report.Frontmatter.Findings))
			fmt.Fprintf(stdout, "align: frozen at %s\n", report.Frontmatter.Frozen.At)
			return 0
		}

		in.Freeze = true
		in.FrozenAt = frozenAt
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

// loadExistingReport reads and strict-decodes a prior deviation-report.md, if
// one exists — (nil, nil, nil) for a first run (no file yet). It returns the
// decoded frontmatter AND the raw body bytes: the freeze-in-place path
// (align.FreezeInPlace) reattaches the body verbatim, so a faithful freeze must
// keep it byte-for-byte rather than re-render it. A file that exists but fails
// to decode is a real, surfaced error (CLAUDE.md: "silence is never a pass" — a
// broken report on disk must never be treated as "no report").
func loadExistingReport(path string) (*artifact.DeviationFrontmatter, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading existing %s: %w", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, body, nil
}
