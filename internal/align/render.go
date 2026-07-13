package align

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// RenderBody renders deviation-report.md's markdown body: computed and
// judged sections, each rendered form labeling its findings' disposition
// (02 §Generated artifacts and digests: "its rendered form labels each
// section computed or judged"), plus the acceptance-baseline boundary diff
// as supporting, undispositioned context (computed.go's doc comment on why
// it is not itself a finding). Deterministic given deterministic inputs —
// Compute and PreserveDispositions both guarantee that.
func RenderBody(findings []artifact.Finding, baselineDiffs []ServiceBoundaryDiff) string {
	var b strings.Builder
	b.WriteString("# Alignment report\n\n")

	b.WriteString("## Computed\n\n")
	renderFindings(&b, findingsOfKind(findings, artifact.FindingComputed))

	b.WriteString("\n### Boundary diff vs acceptance baseline\n\n")
	renderBaselineDiffs(&b, baselineDiffs)

	b.WriteString("\n## Judged\n\n")
	renderFindings(&b, findingsOfKind(findings, artifact.FindingJudged))

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
		disposition := "UNDISPOSITIONED"
		if f.Dispositioned() {
			disposition = string(f.Disposition)
		}
		fmt.Fprintf(b, "- **%s** [%s]: %s", f.ID, disposition, f.Text)
		if f.Note != "" {
			fmt.Fprintf(b, " — %s", f.Note)
		}
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

func renderFrontmatter(b *strings.Builder, fm *artifact.DeviationFrontmatter) {
	fmt.Fprintf(b, "schema: %s\n", fm.Schema)
	fmt.Fprintf(b, "covers: %s\n", fm.Covers)

	if len(fm.Findings) == 0 {
		b.WriteString("findings: []\n")
	} else {
		b.WriteString("findings:\n")
		for _, f := range fm.Findings {
			fmt.Fprintf(b, "  - { id: %s, kind: %s, text: %s", f.ID, f.Kind, yamlDQ(f.Text))
			if f.Disposition != "" {
				fmt.Fprintf(b, ", disposition: %s", f.Disposition)
			}
			if f.Note != "" {
				fmt.Fprintf(b, ", note: %s", yamlDQ(f.Note))
			}
			b.WriteString(" }\n")
		}
	}

	fmt.Fprintf(b, "digest: %s\n", fm.Digest)
	if fm.Integrity != "" {
		fmt.Fprintf(b, "integrity: %s\n", fm.Integrity)
	}
	if fm.JudgeIntegrity != nil {
		fmt.Fprintf(b, "judge_integrity: { stdin_b64: %s, raw_result: %s }\n", fm.JudgeIntegrity.StdinB64, yamlDQ(fm.JudgeIntegrity.RawResult))
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

// yamlDQ renders s as a YAML double-quoted scalar. encoding/json's string
// escaping (\", \\, \n, \t, \r, control chars via \u00XX) is a valid subset
// of YAML double-quoted scalar escaping, so json.Marshal on a plain string
// is a safe, well-tested way to quote arbitrary finding/judge text (which
// may itself contain quotes, colons, or newlines) into this hand-rendered
// frontmatter without a second, hand-rolled escaper.
func yamlDQ(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal on a string cannot fail for well-formed UTF-8/any Go
		// string (invalid UTF-8 is replaced, not rejected); this exists only
		// to satisfy err-checking discipline, never to be reached.
		return `""`
	}
	return string(b)
}
