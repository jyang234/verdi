package dex

import (
	"strings"
	"testing"
)

// TestTokenRules (SI-326): the ac-4 test's check of "new rules use tokens
// only, except the pushpin highlights and shadows", on workbench-only
// blocks — one allowed and one refused case per rule.
func TestTokenRules(t *testing.T) {
	block := func(rules string) string {
		return "a { color: red; }\n" + testWorkbenchOnlyBegin + "\n" + rules + "\n" + testWorkbenchOnlyEnd + "\nb { background: #fff; }\n"
	}
	for _, tc := range []struct {
		name, css string
		// refused is the property a violation must name; "" means the
		// block passes.
		refused string
	}{
		// A colour literal outside a custom-property definition.
		{name: "a token defined with a literal", css: block(":root { --wall-edge: #d6cdb6; --scrim: rgba(35,41,32,.28); }")},
		{name: "a hex colour in a rule", css: block(".wall { border: 1px solid #d6cdb6; }"), refused: "border"},
		{name: "a colour read through tokens", css: block(".wall { border: 1px solid var(--wall-edge); color: var(--ink); }")},
		{name: "an rgba() colour in a rule", css: block(".drawer-scrim { background: rgba(0, 0, 0, .5); }"), refused: "background"},
		{name: "the keywords that are no colour literal", css: block(".x { background: transparent; color: currentColor; border-color: inherit; outline-color: unset; }")},
		{name: "a named colour in a rule", css: block(".x { outline: 2px solid Tomato; }"), refused: "outline"},
		{name: "a named colour only inside a token's name or a string", css: block(`.x { color: var(--note-red); content: "red"; }`)},
		{name: "an hsl() colour in a nested at-rule", css: block("@media (prefers-color-scheme: dark) { .x { color: hsl(0 0% 50%); } }"), refused: "color"},
		// Shadows keep their rgba()s.
		{name: "a box-shadow literal", css: block(".card { box-shadow: 0 2px 3px rgba(0,0,0,.35); }")},
		{name: "a text-shadow literal", css: block(".chip { text-shadow: 0 1px 0 #ffffff; }")},
		{name: "a shadow literal smuggled into another property", css: block(".card { filter: drop-shadow(0 2px 3px rgba(0,0,0,.35)); }"), refused: "filter"},
		// The pushpin keeps its gradient highlights.
		{name: "the pushpin's gradient highlights", css: block(".yarn-handle { background: radial-gradient(circle at 33% 28%, #dd9482, var(--yarn) 55%, #6f2418); }")},
		{name: "a rule targeting the pushpin and something else", css: block(".yarn-handle, .card { background: #dd9482; }"), refused: "background"},
		// A var() fallback is scanned: only the token's name is set aside
		// (F1A-A5).
		{name: "a token with no fallback", css: block(".x { background: var(--paper); }")},
		{name: "a hex fallback", css: block(".x { color: var(--wal-edge, #d6cdb6); }"), refused: "color"},
		{name: "a named-colour fallback", css: block(".x { color: var(--ink, red); }"), refused: "color"},
		{name: "a fallback nested in a fallback", css: block(".x { border: 1px solid var(--a, var(--b, #fff)); }"), refused: "border"},
		{name: "an rgba() fallback", css: block(".x { color: var(--x, rgba(0,0,0,.5)); }"), refused: "color"},
		{name: "a token fallback to another token", css: block(".x { color: var(--ink, var(--muted)); }")},
		// A named colour only as a whole token, never a function's name.
		{name: "the tan() math function", css: block(".x { color: tan(45deg); }")},
		{name: "a hyphenated identifier holding a colour word", css: block(".x { transition: dark-red 1s; }")},
		{name: "a named colour as a whole token", css: block(".x { color: tan; }"), refused: "color"},
		// The pushpin is the rule's subject (F1A-A7).
		{name: "the pushpin's own highlights", css: block(".yarn-handle { background: radial-gradient(#fff, #c33); }")},
		{name: "the pushpin's hover state inside a card", css: block(".card .yarn-handle:hover { background: #dd9482; }")},
		{name: "a rule whose subject has the pushpin", css: block(".board:has(.yarn-handle) { background: #fff; }"), refused: "background"},
		{name: "a rule whose subject follows the pushpin", css: block(".yarn-handle ~ .card { color: #123456; }"), refused: "color"},
		{name: "a rule whose subject is the pushpin's parent", css: block(".yarn-handle-row > .card { color: #123456; }"), refused: "color"},
		{name: "the old card pseudo-element is no pushpin", css: block(".card::before { background: radial-gradient(#dd9482, #6f2418); }"), refused: "background"},
		// The font shorthand's family is a font token (F1A-A6).
		{name: "a font shorthand of a CSS-wide keyword", css: block(".x { font-family: var(--mono); font: inherit; }")},
		{name: "a font shorthand with a token family", css: block(".chip { font: 700 10px/1 var(--mono); }")},
		{name: "a font shorthand with a literal family", css: block(".sticky-body { font: italic 14px Georgia, serif; }"), refused: "font"},
		{name: "a font shorthand with a generic family list", css: block(".chip { font: 700 10px/1 ui-monospace, monospace; }"), refused: "font"},
		{name: "a font shorthand with a token and a literal fallback family", css: block(".chip { font: 700 10px/1 var(--mono), monospace; }"), refused: "font"},
		// Fonts come from font tokens.
		{name: "a font token", css: block(".chip { font-family: var(--mono); }")},
		{name: "a literal font family", css: block(".chip { font-family: Georgia, serif; }"), refused: "font-family"},
		{name: "a font token with a literal fallback", css: block(".chip { font-family: var(--mono), monospace; }"), refused: "font-family"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenRuleViolations(scanDeclarations(t, tc.css))
			if tc.refused == "" {
				if len(got) != 0 {
					t.Fatalf("violations %q, want none", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tc.refused+":") {
				t.Fatalf("violations %q, want exactly one naming %q", got, tc.refused)
			}
		})
	}
}
