// Decision-conflict report rendering — decision_report.go's render.go
// analogue.
package align

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// RenderDecisionBody renders decision-conflict-report.md's markdown body:
// computed and judged sections, each finding labeled with its disposition
// (or UNDISPOSITIONED) and, when present, its CODEOWNERS routing —
// mirroring render.go's RenderBody shape for the build-branch report.
func RenderDecisionBody(findings []artifact.ConflictFinding) string {
	var b strings.Builder
	b.WriteString("# Decision-conflict report\n\n")

	b.WriteString("## Computed (declared-edge completeness)\n\n")
	renderConflictFindings(&b, findingsOfConflictKind(findings, artifact.FindingComputed))

	b.WriteString("\n## Judged (undeclared-conflict sweep)\n\n")
	renderConflictFindings(&b, findingsOfConflictKind(findings, artifact.FindingJudged))

	return b.String()
}

func findingsOfConflictKind(findings []artifact.ConflictFinding, kind artifact.FindingKind) []artifact.ConflictFinding {
	var out []artifact.ConflictFinding
	for _, f := range findings {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func renderConflictFindings(b *strings.Builder, findings []artifact.ConflictFinding) {
	if len(findings) == 0 {
		b.WriteString("(none)\n")
		return
	}
	for _, f := range findings {
		disposition := "UNDISPOSITIONED"
		if f.Dispositioned() {
			disposition = string(f.Disposition)
		}
		fmt.Fprintf(b, "- **%s** [%s]: %s", f.ID, disposition, f.Text)
		if f.Note != "" {
			fmt.Fprintf(b, " — %s", f.Note)
		}
		if len(f.RoutedOwners) > 0 {
			fmt.Fprintf(b, " (routed to: %s)", strings.Join(f.RoutedOwners, ", "))
		}
		b.WriteString("\n")
	}
}

// RenderDecisionMarkdown assembles fm's frontmatter and body into the full
// decision-conflict-report.md file content, hand-rendered into the same
// restricted-dialect flow-mapping style render.go's RenderMarkdown uses —
// deterministic across runs (PLAN.md Phase 8's byte-identical golden
// convention, reapplied here).
func RenderDecisionMarkdown(fm *artifact.DecisionConflictFrontmatter, body string) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	renderDecisionFrontmatter(&b, fm)
	b.WriteString("---\n")
	b.WriteString(body)
	return []byte(b.String())
}

func renderDecisionFrontmatter(b *strings.Builder, fm *artifact.DecisionConflictFrontmatter) {
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
		b.WriteString(" }\n")
	}
}
