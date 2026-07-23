// Package disclosure implements the shared disclosed-unproven rendering
// seam spec/disclosure-legibility#ac-1 requires and docs/spikes/v1/
// disclosure-enumeration-spike.md specifies: every disclosure-producing
// call site already computes the same three-valued judgment (proven /
// violated / disclosed-unproven, CLAUDE.md constitution 2) on demand; what
// was missing was a shared shape and one render function so that judgment
// reads in one consistent vocabulary wherever it appears, rather than as
// N independently hand-authored strings that can silently drift apart on
// the next edit (spec/disclosure-seam's own rung-3 finding — see
// conflict/disclosure-seam-rename-insufficient).
//
// No producer's underlying decision logic changes here: internal/lint's
// VL-017, cmd/verdi gate's disclosed conditions, and
// internal/mcpserve/internal/workbench's review_unavailable rendering all
// keep deciding disclosedness exactly as before — they are migrated only
// to construct a Disclosure at their existing decision point and render
// it through Render, instead of formatting their own ad hoc string.
package disclosure

import (
	"fmt"
	"strings"
)

// SeverityDisclosedUnproven is the one severity value the system
// currently produces: constitution 2's three-valued honesty has exactly
// one non-terminal state besides proven/violated, and violated is never a
// disclosure — it is a verdict failure, reported through a different
// channel entirely (the spike's own reasoning for why Severity is a
// closed one-value field rather than an open enum).
const SeverityDisclosedUnproven = "disclosed-unproven"

// Disclosure is the one shape every disclosure-producing call site emits
// (docs/spikes/v1/disclosure-enumeration-spike.md, verbatim).
type Disclosure struct {
	// ID is a deterministic, content-derived identifier — never a ULID or
	// wall-clock stamp (CLAUDE.md: "no wall-clock or randomness in
	// generated artifacts"). Computed as Source, or Source+"/"+Scope when
	// Scope is non-empty, so the same disclosure re-derives the same ID on
	// every call.
	ID string `json:"id"`
	// Source names the producing rule/verb/condition (e.g. "lint:VL-017",
	// "gate:pending-supersession", "mcp:review-feed") — reusing each
	// producer's own existing id rather than inventing a new taxonomy.
	Source string `json:"source"`
	// Scope is the artifact or ref this disclosure is about, when there is
	// one; omitted for checkout-wide disclosures that name no single
	// artifact.
	Scope string `json:"scope,omitempty"`
	// Text is the human-readable explanation — exactly the message the
	// producer already computed, never re-derived.
	Text string `json:"text"`
	// Severity is always SeverityDisclosedUnproven in v1 (see the constant's
	// doc comment for why the field exists rather than being omitted).
	Severity string `json:"severity"`
}

// New constructs a Disclosure from a producer's existing source id, an
// optional scope, and its already-computed text, with the deterministic ID
// and the fixed v1 severity filled in.
func New(source, scope, text string) Disclosure {
	id := source
	if scope != "" {
		id = source + "/" + scope
	}
	return Disclosure{ID: id, Source: source, Scope: scope, Text: text, Severity: SeverityDisclosedUnproven}
}

// Render renders d in the one shared vocabulary ac-1 requires:
// "disclosed-unproven [<source>]<scope suffix>: <text>". Every disclosure
// call site produces its printed/returned text through this function —
// never a local fmt.Sprintf — so two calls given an equal Disclosure
// always produce byte-identical output (ac-2), and a reader who recognizes
// one disclosure recognizes all of them (ac-1).
func Render(d Disclosure) string {
	scopeSuffix := ""
	if d.Scope != "" {
		scopeSuffix = " " + d.Scope
	}
	return fmt.Sprintf("%s [%s]%s: %s", d.Severity, d.Source, scopeSuffix, d.Text)
}

// IsRendered reports whether s is a line Render produced — the recognizer
// half of ac-1's "a reader who recognizes one disclosure recognizes all of
// them", for the CONSUMERS that must recognize one too.
//
// It exists because the format has to live in exactly one package. A
// consumer that reconstructs it locally (cmd/verdi's closure-gate reporting
// loop counted disclosures with strings.HasPrefix(SeverityDisclosedUnproven
// +" [")) is a second, silent copy of Render's grammar: it accepts prose
// that merely opens with the severity word, and the day Render's format
// changes it starts matching nothing at all — no compile error, no failing
// test outside that consumer. Both halves now change together, here, under
// TestIsRendered_RecognizesEveryRender.
//
// The recognized language is EXACTLY Render's image: IsRendered(Render(d)) is
// true for every Disclosure d whatsoever, and every accepted string is one
// some Disclosure renders to. Anything weaker is a silent miscount — a
// consumer counting disclosures drops a line that was printed — and
// constitution 2/10 has no room for that; anything stronger claims lines the
// format never produced. The predicate therefore polices no producer inputs of
// its own: an empty source renders, so it is recognized, and whether a source
// may be empty is New's question, not the recognizer's.
//
// The search over bracket positions is what makes the first half true. Render
// interpolates the source between "[" and "]" without escaping it, so the
// source's own "]" (if any) is not the closing one; a parse that cut at the
// FIRST "]" rejected `disclosed-unproven [a]b]: t`, which Render emits for the
// source "a]b". Trying every "]" accepts iff SOME decomposition is a valid
// render, which is the definition being implemented.
//
// A line still has to be in that exact shape — the severity, the bracket, then
// either ": " (no scope) or " <scope>: " with a non-empty scope — so an
// indented, hand-built, or merely severity-quoting line is NOT a disclosure.
// That is the intended reading: a producer that wants its line counted routes
// it through New/Render, exactly as ac-1 already requires, rather than
// approximating the vocabulary by hand.
func IsRendered(s string) bool {
	rest, ok := strings.CutPrefix(s, SeverityDisclosedUnproven+" [")
	if !ok {
		return false
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] != ']' {
			continue
		}
		after := rest[i+1:]
		// The unscoped form: "…[source]: text".
		if strings.HasPrefix(after, ": ") {
			return true
		}
		// The scoped form: "…[source] <scope>: text". The separator is looked
		// for from index 2 on, never index 1: at index 1 the scope would be
		// empty, and Render emits the unscoped form for an empty scope — so
		// accepting it there would accept a string no Disclosure renders to.
		// Anywhere past that, the scope is whatever precedes it (its own ": "
		// and "]" included) and the rest is simply text.
		if len(after) >= 2 && after[0] == ' ' && strings.Contains(after[2:], ": ") {
			return true
		}
	}
	return false
}
