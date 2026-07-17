// Diagram sweep report rendering — decision_render.go's sibling for
// sweep-report.md (spec/judged-sweep ac-4, dc-5).
package align

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// DiagramSweepDisclosureLine is the sweep report's fixed, non-empty
// advisory/non-exhaustive disclosure line (spec/judged-sweep ac-4/dc-5:
// "states the sweep is advisory and non-exhaustive verbatim") — rendered
// unconditionally, in a report with findings and in a clean one alike, so a
// clean sweep is never read as "nothing to find here" (dc-5's own framing,
// mirroring how RunJudged's absence finding already discloses failure
// rather than implying success).
const DiagramSweepDisclosureLine = "This sweep is advisory and non-exhaustive — never a completeness guarantee. A human disposes every finding; the AI never edits the diagram in response to its own finding."

// RenderDiagramSweepBody renders sweep-report.md's markdown body: the fixed
// disclosure line first (unconditionally, before any finding), then the
// findings themselves labeled with their disposition (or UNDISPOSITIONED)
// and, when present, their CODEOWNERS routing — mirroring
// RenderDecisionBody's shape for the design-branch report.
func RenderDiagramSweepBody(diagramRef string, findings []artifact.ConflictFinding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Diagram sweep report for %s\n\n", diagramRef)
	b.WriteString(DiagramSweepDisclosureLine)
	b.WriteString("\n\n")

	b.WriteString("## Findings\n\n")
	if len(findings) == 0 {
		b.WriteString("(none)\n")
	}
	for _, f := range findings {
		disposition := "UNDISPOSITIONED"
		if f.Dispositioned() {
			disposition = string(f.Disposition)
		}
		fmt.Fprintf(&b, "- **%s** [%s]: %s", f.ID, disposition, f.Text)
		if f.Note != "" {
			fmt.Fprintf(&b, " — %s", f.Note)
		}
		if len(f.RoutedOwners) > 0 {
			fmt.Fprintf(&b, " (routed to: %s)", strings.Join(f.RoutedOwners, ", "))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderDiagramSweepMarkdown assembles fm's frontmatter and body into the
// full sweep-report.md file content, hand-rendered in the same restricted
// flow-mapping style RenderDecisionMarkdown uses — deterministic across
// runs given identical inputs.
func RenderDiagramSweepMarkdown(fm *artifact.DiagramSweepFrontmatter, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	renderDiagramSweepFrontmatter(&b, fm)
	b.WriteString("---\n")
	b.WriteString(body)
	return []byte(b.String())
}

func renderDiagramSweepFrontmatter(b *strings.Builder, fm *artifact.DiagramSweepFrontmatter) {
	fmt.Fprintf(b, "schema: %s\n", fm.Schema)
	fmt.Fprintf(b, "covers: %s\n", fm.Covers)

	if len(fm.Findings) == 0 {
		b.WriteString("findings: []\n")
	} else {
		b.WriteString("findings:\n")
		for _, f := range fm.Findings {
			fmt.Fprintf(b, "  - { id: %s, kind: %s, text: %s", f.ID, f.Kind, artifact.YAMLDoubleQuote(f.Text))
			if f.Disposition != "" {
				fmt.Fprintf(b, ", disposition: %s", f.Disposition)
			}
			if f.Note != "" {
				fmt.Fprintf(b, ", note: %s", artifact.YAMLDoubleQuote(f.Note))
			}
			if f.TargetRef != "" {
				fmt.Fprintf(b, ", target_ref: %s", f.TargetRef)
			}
			if len(f.RoutedOwners) > 0 {
				fmt.Fprintf(b, ", routed_owners: [%s]", strings.Join(f.RoutedOwners, ", "))
			}
			b.WriteString(" }\n")
		}
	}

	if fm.SweepProvenance != nil {
		fmt.Fprintf(b, "sweep_provenance: { adr_corpus_digest: %s, decisions_scanned: [%s] }\n",
			fm.SweepProvenance.ADRCorpusDigest, strings.Join(fm.SweepProvenance.DecisionsScanned, ", "))
	}

	if fm.Integrity != "" {
		fmt.Fprintf(b, "integrity: %s\n", fm.Integrity)
	}
	if fm.JudgeIntegrity != nil {
		fmt.Fprintf(b, "judge_integrity: { stdin_b64: %s, raw_result: %s }\n", fm.JudgeIntegrity.StdinB64, artifact.YAMLDoubleQuote(fm.JudgeIntegrity.RawResult))
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
