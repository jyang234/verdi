// Prose-tolerant inner-parse of a judge result. The judge is a free-text
// command (S5: `claude -p`; runJudgeOnce's own doc concedes "the judge is
// free-text"), and it intermittently wraps the findings object verdi's prompt
// asked for in a natural-language PREAMBLE ("I've now reviewed all five
// acceptance criteria...") and/or POSTAMBLE, and/or a markdown code fence.
// The historical inner-parse — TrimSpace, strip a leading ```json/``` fence,
// strip a trailing ```, then strict-decode — could not tolerate prose around
// the object: a leading 'I' failed strict decode at stage=inner-parse, which
// D6-24-preserved the prior report and blocked the round (witnessed 5x in the
// obligation-seam round-4 block). This is the ONE shared extraction seam every
// judge-consuming mode uses — judge.go's build-branch decodeInnerResult,
// decision_judge.go's design-branch sweep, and diagram_judge.go's diagram
// sweep — so the fix lands once, never copy-pasted (../CLAUDE.md).
//
// What is widened and what is NOT: only the prose SURROUNDING the object is
// tolerated. Strict decode (artifact.DecodeStrictJSON: DisallowUnknownFields +
// trailing-data rejection) is preserved UNCHANGED on the extracted object — an
// unknown field inside the object still fails closed, and a result with no
// decodable findings object at all (a refusal, an empty result, garbage) still
// fails cleanly at StageInnerParse rather than being swallowed as a false
// "0 findings" DRY verdict. The strictness contract is a property of the
// object; only its surroundings are relaxed.
package align

import (
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// decodeJudgeInnerJSON decodes the judge's findings JSON (shape T) out of its
// free-text result, tolerating a natural-language preamble/postamble and/or a
// markdown code fence around the object. It is generic over the mode-specific
// result shape (judgeInnerResult / decisionInnerResult / diagramInnerResult),
// so all three inner-parse sites share this one extractor with no copy-paste
// and each attempt decodes into a FRESH zero value (never a partially-populated
// carryover from a rejected candidate).
//
// Two paths, strict decode on both:
//
//   - Fast path — the historical defensive fence-strip, then a single strict
//     decode. Covers the common bare-object and ```json-fenced-object cases
//     with no scanning at all.
//   - Slow path — a string/escape-aware balanced-brace scan of the raw result,
//     trying each successive TOP-LEVEL '{' candidate and returning the first
//     that strict-decodes to T. This is what tolerates surrounding prose.
//
// When nothing decodes it returns the fast path's strict error as the
// representative inner-parse failure (never nil/empty findings), so the caller
// reports StageInnerParse exactly as before.
//
// RESIDUAL (disclosed): if the surrounding prose itself contains a balanced
// JSON object that strict-decodes to T (e.g. a decoy `{}` a preamble happened
// to print) BEFORE the intended object, the scan takes the first match. This
// requires the judge to emit its own findings-shaped JSON inside its prose,
// which no observed response does; the far likelier shapes (prose with no
// braces, prose with non-JSON braces like "the set {a,b}") are handled
// correctly because those candidates fail to decode and the scan advances.
func decodeJudgeInnerJSON[T any](raw string) (*T, error) {
	// Fast path: bare object or a whole-string ```json fence.
	var fast T
	fastErr := artifact.DecodeStrictJSON([]byte(stripJudgeFence(raw)), &fast)
	if fastErr == nil {
		return &fast, nil
	}

	// Slow path: scan for a balanced top-level object embedded in prose and
	// strict-decode the first candidate that matches the shape. Strictness is
	// enforced per candidate; only the surrounding prose is tolerated.
	for _, obj := range balancedObjects(raw) {
		var v T
		if artifact.DecodeStrictJSON([]byte(obj), &v) == nil {
			return &v, nil
		}
	}

	// No candidate decoded: a genuine judge failure. Surface the fast-path
	// strict error so the caller reports StageInnerParse — never a false
	// "0 findings".
	return nil, fastErr
}

// stripJudgeFence trims whitespace and a defensive markdown code fence (S5:
// "trim/strip fences defensively") — the historical fast-path shape, unchanged.
func stripJudgeFence(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// balancedObjects returns, in order, the substrings of s that are TOP-LEVEL
// balanced brace regions — each a candidate JSON object embedded in
// surrounding prose. It only ever yields top-level regions: after a balanced
// region is found it resumes scanning PAST that region's closing brace, never
// descending into the region's own nested braces. That is deliberate and
// load-bearing for strictness — it means a shape-violating outer object (one
// with an unknown field) can never be "rescued" by a clean nested sub-object
// that happens to match the shape; the outer object is the candidate, it fails
// strict decode, and the scan moves on. A stray unbalanced '{' in prose (one
// that never closes) is skipped a byte at a time so a genuinely malformed
// prose brace does not blind the scan to a later, well-formed object.
func balancedObjects(s string) []string {
	var objs []string
	for i := 0; i < len(s); {
		if s[i] != '{' {
			i++
			continue
		}
		end, ok := scanBalancedObject(s, i)
		if !ok {
			// No balanced object starts at this '{' (it never closes before
			// the end of s). Skip this byte and keep looking.
			i++
			continue
		}
		objs = append(objs, s[i:end])
		i = end // resume past the whole region; never descend into it
	}
	return objs
}

// scanBalancedObject scans s starting at start (which must index a '{') for
// the matching '}' that returns brace depth to zero, treating double-quoted
// JSON string literals — and their backslash escapes — as opaque, so a brace
// or quote INSIDE a string literal never affects depth. It returns the index
// just past the closing '}' (so s[start:end] is the balanced object) and true,
// or (0, false) if the braces never balance before the end of s (an unbalanced
// or unterminated fragment). It scans byte-by-byte, which is UTF-8 safe: every
// structural byte it tests ({ } " \) is ASCII, and a UTF-8 multibyte rune
// never contains an ASCII byte, so a '{' or '"' inside a multibyte rune is
// impossible and slicing at start/end lands on rune boundaries.
func scanBalancedObject(s string, start int) (end int, ok bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false // this char is the escaped one; consume it
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}
