// verdi align's diagram-sweep mode (spec/judged-sweep ac-1..4): the
// --diagram-sweep <diagram-ref> half of align.go's dispatch. Split into its
// own file mirroring align_design.go's own split rationale (this module's
// one-file-per-topic convention) — align.go's cmdAlign dispatches here on
// the --diagram-sweep flag, the smallest possible touch to the existing
// build-branch/design-branch dispatch.
//
// This mode is DELIBERATELY never read, invoked, or required by `verdi
// gate`, `verdi lint`, or any CI-run path (spec/judged-sweep ac-1/co-1): it
// takes its own diagram-ref argument rather than inferring anything from
// the checked-out branch, and gate.go/internal/lint's own source carries no
// reference to sweep-report.md or DiagramSweepFrontmatter anywhere — see
// aligndiagramsweepstatic_test.go for the static witness.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// runDiagramSweepAlign is the diagram-sweep mode's testable core: parses
// diagramRef, reads and decodes the target diagram file (read-only — the
// file is never reopened or written by anything downstream of this read;
// only its already-read bytes travel onward, spec/judged-sweep ac-4/dc-5),
// requires it to be a class: proposal diagram (the sweep's own stated
// subject — spec/judged-sweep's outcome/problem text), runs
// align.GenerateDiagramSweep, and writes the sibling sweep-report.md.
//
// DISCLOSURE (judged-diagram-sweep-ac1-gap, accepted-deviation): ac-1's
// report-path-first-line + --wait contract is ratified for align's build
// mode and close's freeze-align only, not this sweep — a disclosed residual.
func runDiagramSweepAlign(ctx context.Context, root, diagramRef string, deps alignDeps, stdout, stderr io.Writer) int {
	ref, err := artifact.ParseRef(diagramRef)
	if err != nil {
		fmt.Fprintf(stderr, "align: --diagram-sweep %q: %v\n", diagramRef, err)
		return 2
	}
	if ref.Kind != artifact.KindDiagram {
		fmt.Fprintf(stderr, "align: --diagram-sweep %q: not a diagram ref (kind %q)\n", diagramRef, ref.Kind)
		return 2
	}

	diagPath := filepath.Join(root, ".verdi", "diagrams", ref.Name+".mermaid")
	raw, err := os.ReadFile(diagPath)
	if err != nil {
		fmt.Fprintf(stderr, "align: %s: does not resolve to a diagram (%v)\n", ref.String(), err)
		return 2
	}
	fmBytes, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "align: %s: %v\n", diagPath, err)
		return 2
	}
	diag, err := artifact.DecodeDiagram(fmBytes)
	if err != nil {
		fmt.Fprintf(stderr, "align: %s: %v\n", diagPath, err)
		return 2
	}
	if diag.Class != artifact.DiagramClassProposal {
		fmt.Fprintf(stderr, "align: %s: is not a class: proposal diagram (the judged sweep reads future-state proposals only, spec/judged-sweep)\n", ref.String())
		return 1
	}

	covers, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}

	unpinnedRef := artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
	reportPath := filepath.Join(root, ".verdi", "diagrams", ref.Name+".sweep-report.md")

	existingReport, err := loadExistingDiagramSweepReport(reportPath)
	if err != nil {
		fmt.Fprintln(stderr, "align:", err)
		return 2
	}
	var existingFindings []artifact.ConflictFinding
	if existingReport != nil {
		existingFindings = existingReport.Findings
	}

	report, err := align.GenerateDiagramSweep(ctx, align.DiagramSweepInput{
		Root:             root,
		DiagramRef:       unpinnedRef,
		Body:             body,
		Diagram:          diag,
		Covers:           covers,
		JudgeCmd:         deps.JudgeCmd,
		JudgeRequired:    deps.JudgeRequired,
		ExistingFindings: existingFindings,
		ModelDigest:      deps.ModelDigest,
	})
	if err != nil {
		var reqAbsent *align.ErrDiagramJudgeRequiredAbsent
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
	fmt.Fprintln(stdout, "align: this sweep is advisory and non-exhaustive; it is never read by verdi gate, verdi lint, or any CI path")
	return 0
}

// loadExistingDiagramSweepReport reads and strict-decodes a prior
// sweep-report.md, if one exists — (nil, nil) for a first run. A file that
// exists but fails to decode is a real, surfaced error (CLAUDE.md: "silence
// is never a pass"), mirroring align.go's loadExistingReport exactly.
func loadExistingDiagramSweepReport(path string) (*artifact.DiagramSweepFrontmatter, error) {
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
	decoded, err := artifact.DecodeDiagramSweep(fm)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return decoded, nil
}
