package specdoc

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// RenderMarkdown writes the canonical text form. The layout is the
// cross-consumer contract (spec/spec-documents ac-6): CLI, board, docs
// site, and MCP compare on these bytes.
func RenderMarkdown(doc Document) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	// criterionText/questionText let the Plan and Evidence sections read
	// standalone (fix round 1, A3): a plan line or an evidence row names
	// an id, but a reader of just that section should not have to jump to
	// the Acceptance criteria/Open questions sections to know what the id
	// means. Build always fills doc.Criteria/doc.Questions regardless of
	// which sections the current kind renders (build.go), so both maps
	// are complete here even for a kind whose own Sections() omits
	// SectionCriteria/SectionQuestions (e.g. the plan kind).
	criterionText := make(map[string]string, len(doc.Criteria))
	for _, c := range doc.Criteria {
		criterionText[c.ID] = c.Text
	}
	questionText := make(map[string]string, len(doc.Questions))
	for _, q := range doc.Questions {
		questionText[q.ID] = q.Text
	}

	w("# %s\n\n", doc.Title)
	if doc.Stamp.Proposed {
		// vocab:identity — "proposed"/"accepted" here name the merge event of the spec MR, not a lifecycle state label
		w("> **Proposed, not accepted.** These are the working tree's bytes at %s, not the accepted bytes on the default branch; nothing here is accepted authority yet.\n\n", doc.Stamp.Commit)
	}

	for _, s := range doc.Sections {
		switch s {
		case SectionIdentity:
			w("## Identity\n\n| Field | Value |\n|---|---|\n")
			for _, kv := range doc.Identity {
				w("| %s | %s |\n", kv.Label, escapeCell(kv.Value))
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
				w("### %s — %s <a id=\"%s\"></a>\n\n", d.ID, d.Text, d.ID)
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
				w("- **%s** %s <a id=\"%s\"></a>\n", c.ID, c.Text, c.ID)
				if c.Detail != "" {
					w("\n%s\n", indentLines(c.Detail, "  "))
				}
			}
			if len(doc.Constraints) > 0 {
				w("\n")
			}
		case SectionCriteria:
			w("## Acceptance criteria\n\n")
			if len(doc.Criteria) == 0 {
				w("No acceptance criteria are declared.\n\n")
			}
			for i, c := range doc.Criteria {
				indent := itemIndent(i + 1)
				w("%d. **%s** %s <a id=\"%s\"></a>\n", i+1, c.ID, c.Text, c.ID)
				w("%s- Evidence: %s.\n", indent, joinOr(c.Evidence, "none declared"))
				w("%s- Coverage: %s\n", indent, coverageLine(c, doc.Words))
				if c.Detail != "" {
					w("\n%s\n", indentLines(c.Detail, indent))
				}
				w("\n")
			}
		case SectionQuestions:
			w("## Open questions\n\n")
			if len(doc.Questions) == 0 {
				w("No open questions are declared.\n\n")
			}
			for _, q := range doc.Questions {
				w("- **%s** %s <a id=\"%s\"></a> — Claims: %s\n", q.ID, q.Text, q.ID, claimsLine(q, doc.Words))
				if q.Detail != "" {
					w("\n%s\n", indentLines(q.Detail, "  "))
				}
			}
			if len(doc.Questions) > 0 {
				w("\n")
			}
		case SectionPlan:
			w("## Plan\n\n")
			if len(doc.Plan) == 0 {
				w("This spec declares no %s and no %s.\n\n", doc.Words.StoryPlural, doc.Words.SpikePlural)
			} else {
				// `spec/<slug>` is wrapped as a code span (fix round 1
				// golden review: bare "spec/<slug>" parses as an inline
				// raw-HTML tag under goldmark's WithUnsafe() passthrough,
				// so the placeholder text never reaches a browser's DOM —
				// a code span's contents are never parsed as HTML). The
				// lead sentence itself only prints when there is a plan
				// to lead (fix round 2, N1) — an empty plan has nothing
				// for "in this plan" to refer to.
				w("Each %s in this plan becomes `spec/<slug>` when it is instantiated.\n\n", doc.Words.Story)
			}
			for i, p := range doc.Plan {
				// No trailing period after the closing parenthesis (fix
				// round 2, N3): the covered criterion's/resolved
				// question's own declared text already ends the
				// sentence, e.g. "covers ac-1 (A key opens one box.)" —
				// idsWithText's "not declared"/empty fallbacks below
				// carry their own terminal period instead.
				if p.Spike {
					w("%d. %s `%s` answers %s\n", i+1, capitalize(doc.Words.Spike), p.Slug, idsWithText(p.Resolves, questionText, "no declared question."))
				} else {
					w("%d. %s `%s` covers %s\n", i+1, capitalize(doc.Words.Story), p.Slug, idsWithText(p.Criteria, criterionText, "no declared criterion."))
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
				w("| %s | %s | %s | %s |\n", escapeCell(criterionCell(e.ID, criterionText)), escapeCell(orDash(e.Status)), escapeCell(orDash(e.Summary)), escapeCell(evidenceDetail(e, doc.Words)))
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
	return fmt.Sprintf("covered by %s %s.", pluralIf(words.Story, words.StoryPlural, len(c.Coverage)), joinCode(c.Coverage))
}

// claimsLine returns only the predicate; RenderMarkdown writes the
// "Claims: " prefix itself so the unknown and known cases share one
// prefix instead of only the unknown case carrying it (fix round 1, A9).
func claimsLine(q Question, words Words) string {
	if !q.ClaimsKnown {
		return "not computed for this render."
	}
	if len(q.Claims) == 0 {
		return "unclaimed; blocks acceptance until a " + words.Spike + " claims it or a decision answers it."
	}
	return fmt.Sprintf("claimed by %s %s; answered after acceptance.", pluralIf(words.Spike, words.SpikePlural, len(q.Claims)), joinCode(q.Claims))
}

func evidenceDetail(e EvidenceRow, words Words) string {
	if len(e.Stories) > 0 {
		return fmt.Sprintf("implementing %s: %s", pluralIf(words.Story, words.StoryPlural, len(e.Stories)), joinCode(e.Stories))
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

// notDeclaredFallback is what criterionCell/idsWithText show for an id
// that names no entry in the id->text map (fix round 2, A3 minor): a
// dangling reference (a stub or evidence row naming an id the spec never
// declared) must read as an explicit, honest gap, never as a bare
// trailing separator or empty parentheses.
const notDeclaredFallback = "not declared on this spec"

// criterionCell is the Evidence table's Criterion column (fix round 1,
// A3): the id plus its declared text, so the Evidence section reads
// standalone instead of forcing a reader back to Acceptance criteria.
func criterionCell(id string, textByID map[string]string) string {
	text, ok := textByID[id]
	if !ok {
		text = "(" + notDeclaredFallback + ")"
	}
	return fmt.Sprintf("%s — %s", id, text)
}

// idsWithText renders a Plan line's covered-criteria/resolved-questions
// list as "id (text)" pairs (fix round 1, A3), or empty when ids is empty.
func idsWithText(ids []string, textByID map[string]string, empty string) string {
	if len(ids) == 0 {
		return empty
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		text, ok := textByID[id]
		if !ok {
			text = notDeclaredFallback
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", id, text))
	}
	return strings.Join(parts, ", ")
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

// escapeCell escapes a literal "|" so a declared value can never fracture
// a Markdown table row (fix round 1, F8).
func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// itemIndent is the continuation indent for numbered-list item n: the
// width of "<n>. " itself, so a continuation or nested-bullet line lines
// up under the item's own text at every item number, not just 1-9 (fix
// round 1, F3 — the old hard-coded three spaces broke at item 10, whose
// "10. " marker is four columns wide).
func itemIndent(n int) string {
	return strings.Repeat(" ", len(strconv.Itoa(n))+2)
}

// indentLines prefixes every non-empty line of s with indent, leaving
// blank lines (paragraph breaks inside a multi-paragraph Detail) truly
// empty rather than whitespace-only (fix round 1, A1/F4/F5: "non-empty
// lines only, never emit whitespace-only lines") — a blind
// strings.ReplaceAll("\n", "\n"+indent) would pad blank separator lines
// too.
func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = indent + l
		}
	}
	return strings.Join(lines, "\n")
}

// capitalize upper-cases only the first rune, leaving the rest of a
// (possibly renamed, possibly multi-word) class word untouched.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// pluralIf returns plural when n != 1, singular otherwise. Both forms
// are supplied by the caller (Words.Story/StoryPlural,
// Words.Spike/SpikePlural) — the model's own DisplayClassPlural, resolved
// once at Build time — so the renderer never re-derives a plural from a
// bare word itself (fix round 1, F1: the deleted pluralWord's "always
// append s" rule rendered "storys").
func pluralIf(singular, plural string, n int) string {
	if n != 1 {
		return plural
	}
	return singular
}
