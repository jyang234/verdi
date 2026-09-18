package specdoc

import (
	"fmt"
	"strings"
)

// RenderMarkdown writes the canonical text form. The layout is the
// cross-consumer contract (spec/spec-documents ac-6): CLI, board, docs
// site, and MCP compare on these bytes.
func RenderMarkdown(doc Document) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# %s\n\n", doc.Title)
	if doc.Stamp.Proposed {
		// vocab:identity — "proposed"/"accepted" here name the merge event of the spec MR, not a lifecycle state label
		w("> Proposed, not accepted: rendered from design-branch bytes at `%s`.\n\n", doc.Stamp.Commit)
	}

	for _, s := range doc.Sections {
		switch s {
		case SectionIdentity:
			w("## Identity\n\n| Field | Value |\n|---|---|\n")
			for _, kv := range doc.Identity {
				w("| %s | %s |\n", kv.Label, kv.Value)
			}
			w("\n")
		case SectionProblem:
			w("## Problem\n\n")
			if doc.ProblemKnown {
				w("%s\n\n", doc.ProblemText)
			} else {
				w("No problem statement is declared on this spec.\n\n")
			}
		case SectionOutcome:
			w("## Outcome\n\n")
			if doc.OutcomeKnown {
				w("%s\n\n", doc.OutcomeText)
			} else {
				w("No outcome statement is declared on this spec.\n\n")
			}
		case SectionDecisions:
			w("## Decisions\n\n")
			if len(doc.Decisions) == 0 {
				w("No decisions are declared.\n\n")
			}
			for _, d := range doc.Decisions {
				w("### %s\n\n%s\n\n", d.ID, d.Text)
				if d.Detail != "" {
					w("%s\n\n", d.Detail)
				}
			}
		case SectionConstraints:
			w("## Constraints\n\n")
			if len(doc.Constraints) == 0 {
				w("No constraints are declared.\n\n")
			}
			for _, c := range doc.Constraints {
				w("- **%s** %s\n", c.ID, c.Text)
				if c.Detail != "" {
					w("  %s\n", strings.ReplaceAll(c.Detail, "\n", "\n  "))
				}
			}
			if len(doc.Constraints) > 0 {
				w("\n")
			}
		case SectionCriteria:
			w("## Acceptance criteria\n\n")
			for i, c := range doc.Criteria {
				w("%d. **%s** %s <a id=\"%s\"></a>\n", i+1, c.ID, c.Text, c.ID)
				w("   Evidence: %s.\n", joinOr(c.Evidence, "none declared"))
				w("   Coverage: %s\n", coverageLine(c, doc.Words))
				if c.Detail != "" {
					w("\n   %s\n", strings.ReplaceAll(c.Detail, "\n", "\n   "))
				}
				w("\n")
			}
		case SectionQuestions:
			w("## Open questions\n\n")
			if len(doc.Questions) == 0 {
				w("No open questions are declared.\n\n")
			}
			for _, q := range doc.Questions {
				w("- **%s** %s — %s\n", q.ID, q.Text, claimsLine(q, doc.Words))
			}
			if len(doc.Questions) > 0 {
				w("\n")
			}
		case SectionPlan:
			w("## Plan\n\n")
			if len(doc.Plan) == 0 {
				w("No planned %s or %s are declared.\n\n", pluralWord(doc.Words.Story), pluralWord(doc.Words.Spike))
			}
			for i, p := range doc.Plan {
				if p.Spike {
					w("%d. Research %s `%s` answers %s.\n", i+1, doc.Words.Spike, p.Slug, joinOr(p.Resolves, "no declared question"))
				} else {
					w("%d. Planned %s `%s` covers %s.\n", i+1, doc.Words.Story, p.Slug, joinOr(p.Criteria, "no declared criterion"))
				}
			}
			if len(doc.Plan) > 0 {
				w("\n")
			}
		case SectionEvidence:
			w("## Evidence\n\n")
			if !doc.EvidenceKnown {
				w("Evidence was not supplied for this render.\n\n")
				break
			}
			w("Source: %s.\n\n| Criterion | State | Summary | Detail |\n|---|---|---|---|\n", doc.EvidenceSource)
			for _, e := range doc.Evidence {
				w("| %s | %s | %s | %s |\n", e.ID, orDash(e.Status), orDash(e.Summary), evidenceDetail(e, doc.Words))
			}
			w("\n")
		}
	}

	w("---\n\n")
	// vocab:identity — the stamp names the artifact's objects, not a lifecycle state label
	w("Derived from the spec's objects; not authority. Ref `%s` · commit `%s` · kind `%s` · engine `%s`\n", doc.Stamp.Ref, doc.Stamp.Commit, doc.Kind, doc.Stamp.Engine)
	return b.String()
}

func coverageLine(c Criterion, words Words) string {
	if !c.CoverageKnown {
		return "not computed for this render."
	}
	if len(c.Coverage) == 0 {
		return "not yet planned."
	}
	return fmt.Sprintf("planned in %s %s.", pluralIf(words.Story, len(c.Coverage)), joinCode(c.Coverage))
}

func claimsLine(q Question, words Words) string {
	if !q.ClaimsKnown {
		return "Claims: not computed for this render."
	}
	if len(q.Claims) == 0 {
		return "unclaimed; blocks acceptance until a research " + words.Spike + " claims it or a decision answers it."
	}
	return fmt.Sprintf("claimed by research %s %s; answered after acceptance.", pluralIf(words.Spike, len(q.Claims)), joinCode(q.Claims))
}

func evidenceDetail(e EvidenceRow, words Words) string {
	if len(e.Stories) > 0 {
		return fmt.Sprintf("implementing %s: %s", pluralIf(words.Story, len(e.Stories)), joinCode(e.Stories))
	}
	if len(e.Kinds) > 0 {
		parts := make([]string, 0, len(e.Kinds))
		for _, k := range e.Kinds {
			state := "unsatisfied"
			if k.Satisfied {
				state = "satisfied"
			}
			parts = append(parts, k.Kind+" "+state)
		}
		return strings.Join(parts, ", ")
	}
	return "—"
}

func joinOr(items []string, empty string) string {
	if len(items) == 0 {
		return empty
	}
	return strings.Join(items, ", ")
}

func joinCode(items []string) string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, "`"+it+"`")
	}
	return strings.Join(out, ", ")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// pluralWord appends "s" unless the word already ends in "s"; the display
// chain's own plural (DisplayClassPlural) is model-aware but needs the
// model, which Build has already consumed into Words, so the renderer
// applies the same rule the chain applies for plain words.
func pluralWord(word string) string {
	if strings.HasSuffix(word, "s") {
		return word
	}
	return word + "s"
}

func pluralIf(word string, n int) string {
	if n == 1 {
		return word
	}
	return pluralWord(word)
}
