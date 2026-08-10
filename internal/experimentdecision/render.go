package experimentdecision

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
)

// RenderResult validates res and returns its canonical JSON encoding —
// this IS the content a result.json file must hold. Two calls on an equal
// res always return byte-identical output (canonjson.Marshal's own
// determinism), and Evaluate's own numeric-formatting discipline
// (formatFloat) is what keeps that byte-identity holding across
// independent Evaluate runs over the same evidence, not anything this
// function does itself.
func RenderResult(res experiment.Result) ([]byte, error) {
	if err := res.Validate(); err != nil {
		return nil, errfWrap("rendering result", err)
	}
	data, err := canonjson.Marshal(res)
	if err != nil {
		return nil, errfWrap("rendering result", err)
	}
	return data, nil
}

// formatJSONNumber parses n and re-renders it with
// strconv.FormatFloat(v, 'f', -1, 64) — the one fixed numeric formatting
// every rendered value in this package uses, independent of how the
// source document originally spelled the same value.
func formatJSONNumber(n interface{ Float64() (float64, error) }) (string, error) {
	v, err := n.Float64()
	if err != nil {
		return "", fmt.Errorf("formatting number %v: %w", n, err)
	}
	return strconv.FormatFloat(v, 'f', -1, 64), nil
}

// RenderRecommendation renders a deterministic recommendation.md
// explanation: a pure function of (locked def, validated res) with no
// wall-clock reads, environment reads, or map-iteration-ordered content —
// every list below is either already in registered order or explicitly
// sorted. It errors when def is not locked, res is invalid, or
// res.DefinitionDigest does not match def's own computed
// DefinitionDigest — a recommendation can never be rendered against a
// result it does not actually belong to.
//
// The rendered content appears in this FIXED order: a title naming the
// experiment id; the verdict; when proven, the winner and CO-5's verbatim
// boundary sentence plus an explicit disclaimer against universal
// superiority; when violated or unproven, every reason with its detail,
// candidate, guard, and witness; a candidates table (id, baseline marker,
// primary aggregate and unit, eligibility, violated guards with
// witnesses); and finally the definition digest, result digest, run id,
// and algorithm version.
func RenderRecommendation(def experiment.Definition, res experiment.Result) ([]byte, error) {
	locked, err := experiment.Locked(def)
	if err != nil {
		return nil, errfWrap("rendering recommendation", err)
	}
	if !locked {
		return nil, errf("rendering recommendation: definition %q is not locked", def.ID)
	}
	if err := res.Validate(); err != nil {
		return nil, errfWrap("rendering recommendation", err)
	}
	defDigest, err := experiment.DefinitionDigest(def)
	if err != nil {
		return nil, errfWrap("rendering recommendation", err)
	}
	if res.DefinitionDigest != defDigest {
		return nil, errf("rendering recommendation: result definition_digest %q does not match the locked definition digest %q", res.DefinitionDigest, defDigest)
	}
	resultDigest, err := experiment.ResultDigest(res)
	if err != nil {
		return nil, errfWrap("rendering recommendation", err)
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Experiment %s\n\n", res.Experiment)
	fmt.Fprintf(&b, "**Verdict:** %s\n\n", res.Verdict)

	if res.Verdict == experiment.VerdictProvenWinner {
		fmt.Fprintf(&b, "**Winner:** %s\n\n", res.Winner)
		fmt.Fprintf(&b, "Candidate %s is the best demonstrated path among the registered candidates for this desired outcome, workload, environment, and comparison revision.\n\n", res.Winner)
		b.WriteString("This is not a claim of universal superiority over unregistered designs or unrepresented production conditions.\n\n")
	} else {
		b.WriteString("## Reasons\n\n")
		for _, r := range res.Reasons {
			if err := writeReason(&b, r); err != nil {
				return nil, errfWrap("rendering recommendation", err)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Candidates\n\n")
	b.WriteString("| id | baseline | primary | eligible | violated guards |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, c := range res.Candidates {
		if err := writeCandidateRow(&b, c); err != nil {
			return nil, errfWrap("rendering recommendation", err)
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "Definition digest: `%s`\n\n", res.DefinitionDigest)
	fmt.Fprintf(&b, "Result digest: `%s`\n\n", resultDigest)
	fmt.Fprintf(&b, "Run: `%s`\n\n", res.Run)
	fmt.Fprintf(&b, "Algorithm: `%s`\n", res.Algorithm)

	return []byte(b.String()), nil
}

// writeReason appends one reason line: code, then whichever of
// candidate/guard/detail/witness are present, in that fixed field order.
func writeReason(b *strings.Builder, r experiment.Reason) error {
	fmt.Fprintf(b, "- `%s`", r.Code)
	if r.Candidate != "" {
		fmt.Fprintf(b, " candidate=%s", r.Candidate)
	}
	if r.Guard != "" {
		fmt.Fprintf(b, " guard=%s", r.Guard)
	}
	if r.Detail != "" {
		fmt.Fprintf(b, " detail=%q", r.Detail)
	}
	if r.Witness != nil {
		fmt.Fprintf(b, " witness=%q", *r.Witness)
	}
	b.WriteString("\n")
	return nil
}

// writeCandidateRow appends one row of the candidates table: id, a "yes"
// baseline marker or empty, the formatted primary aggregate and unit (or
// "n/a" when absent), eligibility, and every violated guard with its
// round and witness, semicolon-joined in the candidate's own registered
// violation order.
func writeCandidateRow(b *strings.Builder, c experiment.CandidateResult) error {
	baselineMark := ""
	if c.Baseline {
		baselineMark = "yes"
	}

	primaryCell := "n/a"
	if c.Primary != nil {
		formatted, err := formatJSONNumber(c.Primary.Value)
		if err != nil {
			return err
		}
		primaryCell = fmt.Sprintf("%s %s", formatted, c.Primary.Unit)
	}

	violated := make([]string, 0, len(c.Violations))
	for _, v := range c.Violations {
		violated = append(violated, fmt.Sprintf("%s (round %d: %s)", v.Guard, v.Round, v.Witness))
	}

	fmt.Fprintf(b, "| %s | %s | %s | %t | %s |\n", c.ID, baselineMark, primaryCell, c.Eligible, strings.Join(violated, "; "))
	return nil
}
