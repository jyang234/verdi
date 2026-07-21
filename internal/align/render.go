package align

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// RenderBody renders deviation-report.md's markdown body: computed and
// judged sections, each rendered form labeling its findings' disposition
// (02 §Generated artifacts and digests: "its rendered form labels each
// section computed or judged"), plus the acceptance-baseline boundary diff
// and the diagram-alignment section as supporting, undispositioned context
// (computed.go's doc comment on why neither is itself a finding — the
// diagram Findings that DO ride the dispositionable path are already among
// findings above, rendered by the renderFindings call), plus two
// spec/finding-identity sections: candidates awaiting reaffirmation (ac-1's
// pre-fill — old ruling and old text rendered beside each pending
// candidate's new text) and not-resurfaced findings (ac-3's persisted
// archive). Deterministic given deterministic inputs — Compute,
// PreserveDispositions, and ReconcileJudged all guarantee that.
//
// candidates/notResurfaced may be nil (every caller outside this package's
// own tests: FreezeInPlace never calls this — it reattaches an existing body
// verbatim; disposition.go never calls this — it surgically patches one
// rendered line in place, see its own doc comment on why. Only Generate,
// which always has both, calls this in production).
func RenderBody(findings []artifact.Finding, candidates map[string]JudgedCandidate, notResurfaced []artifact.Finding, baselineDiffs []ServiceBoundaryDiff, diagramProposals []DiagramAlignmentEntry, illustrativeDiagrams []IllustrativeFigure) string {
	var b strings.Builder
	b.WriteString("# Alignment report\n\n")

	b.WriteString("## Computed\n\n")
	renderFindings(&b, findingsOfKind(findings, artifact.FindingComputed))

	b.WriteString("\n### Boundary diff vs acceptance baseline\n\n")
	renderBaselineDiffs(&b, baselineDiffs)

	b.WriteString("\n### Diagram alignment\n\n")
	renderDiagramAlignment(&b, diagramProposals, illustrativeDiagrams)

	b.WriteString("\n## Judged\n\n")
	renderFindings(&b, findingsOfKind(findings, artifact.FindingJudged))

	b.WriteString("\n### Candidates awaiting reaffirmation\n\n")
	renderCandidates(&b, findings, candidates)

	b.WriteString("\n## Not resurfaced\n\n")
	renderNotResurfaced(&b, notResurfaced)

	return b.String()
}

func findingsOfKind(findings []artifact.Finding, kind artifact.FindingKind) []artifact.Finding {
	var out []artifact.Finding
	for _, f := range findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func renderFindings(b *strings.Builder, findings []artifact.Finding) {
	if len(findings) == 0 {
		b.WriteString("(none)\n")
		return
	}
	for _, f := range findings {
		b.WriteString(RenderFindingLine(f))
		b.WriteString("\n")
	}
}

// RenderFindingLine renders a single finding's markdown body bullet line —
// the exact line renderFindings emits for f, factored out and exported
// (spec/disposition-verb dc-2) so the `verdi disposition` verb can compute
// both a finding's OLD line (to locate, byte-for-byte, within an existing
// report's body) and its NEW line (to substitute in place) from the SAME one
// formatting rule renderFindings itself uses, rather than a second, drifting
// copy of the format living in cmd/verdi. TestRenderFindingLine_MatchesRenderBody
// pins that this is not a second copy: RenderBody's own output contains
// exactly this line for every finding it renders.
func RenderFindingLine(f artifact.Finding) string {
	disposition := "UNDISPOSITIONED"
	if f.Dispositioned() {
		disposition = string(f.Disposition)
	}
	line := fmt.Sprintf("- **%s** [%s]: %s", f.ID, disposition, f.Text)
	if f.Note != "" {
		line += fmt.Sprintf(" — %s", f.Note)
	}
	return line
}

// renderCandidates renders spec/finding-identity ac-1's pre-fill block: one
// entry per finding in findings that has a paired candidates entry, showing
// the fresh (new) finding's own bullet line beside its old ruling and old
// text — so a human reviewing the report sees exactly what a same-slug
// regeneration changed before running `verdi disposition`. Order follows
// findings' own order (deterministic given ReconcileJudged's own
// determinism). This section is regenerated fresh by every `verdi align`
// run and is NOT itself surgically maintained by the disposition verb (which
// only ever patches one finding's own bullet line, per its own doc
// comment) — a confirmed candidate's entry here becomes stale prose until
// the next align run, though the frontmatter (the actual source of truth
// AllDispositioned/the gates/the spec-stale budget all read) is correct
// immediately.
func renderCandidates(b *strings.Builder, findings []artifact.Finding, candidates map[string]JudgedCandidate) {
	var rendered int
	for _, f := range findings {
		cand, ok := candidates[f.ID]
		if !ok {
			continue
		}
		rendered++
		fmt.Fprintf(b, "- **%s** CANDIDATE — new text: %q\n", f.ID, f.Text)
		fmt.Fprintf(b, "  - prior ruling [%s]: %q", cand.OldDisposition, cand.OldText)
		if cand.OldNote != "" {
			fmt.Fprintf(b, " — %s", cand.OldNote)
		}
		b.WriteString("\n")
		fmt.Fprintf(b, "  - not a disposition: confirm via `verdi disposition <spec-ref> %s <fixed|accepted-deviation> --rationale <text>`\n", f.ID)
	}
	if rendered == 0 {
		b.WriteString("(none)\n")
	}
}

// RenderNotResurfacedLine renders a single not-resurfaced entry's markdown
// body bullet line — mirrors RenderFindingLine's shape with a distinguishing
// prefix (never confusable, byte-for-byte, with a live finding's own bullet:
// disposition.go's confirm path locates and removes this exact line by
// content, symmetric to how it locates a live finding's line).
func RenderNotResurfacedLine(f artifact.Finding) string {
	line := fmt.Sprintf("- **%s** [not-resurfaced, was %s]: %s", f.ID, f.Disposition, f.Text)
	if f.Note != "" {
		line += fmt.Sprintf(" — %s", f.Note)
	}
	return line
}

func renderNotResurfaced(b *strings.Builder, notResurfaced []artifact.Finding) {
	if len(notResurfaced) == 0 {
		b.WriteString("(none)\n")
		return
	}
	for _, f := range notResurfaced {
		b.WriteString(RenderNotResurfacedLine(f))
		b.WriteString("\n")
	}
}

func renderBaselineDiffs(b *strings.Builder, diffs []ServiceBoundaryDiff) {
	if len(diffs) == 0 {
		b.WriteString("(no impacted services)\n")
		return
	}
	for _, d := range diffs {
		if d.Skipped {
			fmt.Fprintf(b, "- %s: skipped (%s)\n", d.Service, d.SkipReason)
			continue
		}
		if len(d.Entries) == 0 {
			fmt.Fprintf(b, "- %s: no drift since acceptance (%s)\n", d.Service, d.BaselineCommit)
			continue
		}
		fmt.Fprintf(b, "- %s (since %s):\n", d.Service, d.BaselineCommit)
		for _, e := range d.Entries {
			breaking := ""
			if e.Breaking {
				breaking = " (BREAKING)"
			}
			fmt.Fprintf(b, "  - %s %s %s%s\n", e.Op, e.Surface, e.Name, breaking)
		}
	}
}

// renderDiagramAlignment renders the "### Diagram alignment" subsection
// (spec/alignment-section ac-3), mirroring renderBaselineDiffs' shape
// exactly: one line per accepted proposal, one line per illustrative
// diagram, each an explicit placeholder line when its own set is empty
// (never an omitted heading — CLAUDE.md: "silence is never a pass").
// Every proposal line's text is diagramFindingText's own output (the SAME
// substance the proposal's Finding carries, diagram_computed.go) so the
// coverage tier and any deltas are never dropped or restated differently
// between the two renderings.
func renderDiagramAlignment(b *strings.Builder, proposals []DiagramAlignmentEntry, illustrative []IllustrativeFigure) {
	if len(proposals) == 0 {
		b.WriteString("- (no accepted proposals)\n")
	} else {
		for _, p := range proposals {
			fmt.Fprintf(b, "- %s: %s\n", p.Name, diagramFindingText(p))
		}
	}
	if len(illustrative) == 0 {
		b.WriteString("- (no illustrative diagrams in this spec's body)\n")
	} else {
		for _, f := range illustrative {
			fmt.Fprintf(b, "- %s: unverifiable (illustrative — no truth generator)\n", f.Name)
		}
	}
}

// RenderMarkdown assembles fm's frontmatter and body into the full
// deviation-report.md file content. Frontmatter is hand-rendered (not a
// generic YAML marshal) into the same restricted-dialect flow-mapping style
// deviation_test.go's own fixtures use — matching cmd/verdi/design.go's
// scaffoldDraftSpec precedent for generated frontmatter — so field order
// and formatting are exactly deterministic across runs, a requirement for
// the byte-identical golden comparison (PLAN.md Phase 8's exit criteria).
func RenderMarkdown(fm *artifact.DeviationFrontmatter, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	renderFrontmatter(&b, fm)
	b.WriteString("---\n")
	b.WriteString(body)
	return []byte(b.String())
}

// renderFindingFlowMap renders one Finding as a YAML flow-mapping entry
// (without a leading list-item indent or trailing newline — callers add
// both), the one shared rule both findings: and not-resurfaced: render with
// (CLAUDE.md: no copy-paste across call sites) — so carried-from: rendering
// can never drift between the two sections.
func renderFindingFlowMap(b *strings.Builder, f artifact.Finding) {
	fmt.Fprintf(b, "- { id: %s, kind: %s, text: %s", f.ID, f.Kind, artifact.YAMLDoubleQuote(f.Text))
	if f.Disposition != "" {
		fmt.Fprintf(b, ", disposition: %s", f.Disposition)
	}
	if f.Note != "" {
		fmt.Fprintf(b, ", note: %s", artifact.YAMLDoubleQuote(f.Note))
	}
	if f.CarriedFrom != "" {
		fmt.Fprintf(b, ", carried-from: %s", f.CarriedFrom)
	}
	b.WriteString(" }")
}

func renderFrontmatter(b *strings.Builder, fm *artifact.DeviationFrontmatter) {
	fmt.Fprintf(b, "schema: %s\n", fm.Schema)
	fmt.Fprintf(b, "covers: %s\n", fm.Covers)

	if len(fm.Findings) == 0 {
		b.WriteString("findings: []\n")
	} else {
		b.WriteString("findings:\n")
		for _, f := range fm.Findings {
			b.WriteString("  ")
			renderFindingFlowMap(b, f)
			b.WriteString("\n")
		}
	}

	// not-resurfaced: (spec/finding-identity ac-3) — schema-additive and
	// omitempty, so an old-shaped report with none renders exactly as
	// before this story: the key is omitted entirely, never printed as an
	// empty list.
	if len(fm.NotResurfaced) > 0 {
		b.WriteString("not-resurfaced:\n")
		for _, f := range fm.NotResurfaced {
			b.WriteString("  ")
			renderFindingFlowMap(b, f)
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(b, "digest: %s\n", fm.Digest)
	if fm.Integrity != "" {
		fmt.Fprintf(b, "integrity: %s\n", fm.Integrity)
	}
	if fm.JudgeIntegrity != nil {
		fmt.Fprintf(b, "judge_integrity: { stdin_b64: %s, raw_result: %s }\n", fm.JudgeIntegrity.StdinB64, artifact.YAMLDoubleQuote(fm.JudgeIntegrity.RawResult))
	}
	if fm.Frozen != nil {
		fmt.Fprintf(b, "frozen: { at: %s, commit: %s }\n", fm.Frozen.At, fm.Frozen.Commit)
	}
	if fm.Provenance != nil {
		fmt.Fprintf(b, "provenance: { generator: %s, version: %s, inputs: [%s]", fm.Provenance.Generator, fm.Provenance.Version, strings.Join(fm.Provenance.Inputs, ", "))
		if fm.Provenance.Digest != "" {
			fmt.Fprintf(b, ", digest: %s", fm.Provenance.Digest)
		}
		if fm.Provenance.Integrity != "" {
			fmt.Fprintf(b, ", integrity: %s", fm.Provenance.Integrity)
		}
		if fm.Provenance.Model != "" {
			fmt.Fprintf(b, ", model: %s", fm.Provenance.Model)
		}
		b.WriteString(" }\n")
	}
}
