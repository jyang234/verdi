package disclosure

import "testing"

func TestNew(t *testing.T) {
	cases := []struct {
		name                string
		source, scope, text string
		wantID              string
	}{
		{"with scope", "lint:VL-017", "spec/disclosure-legibility", "mutable zone absent", "lint:VL-017/spec/disclosure-legibility"},
		{"no scope (checkout-wide)", "mcp:review-feed", "", "forge unreachable", "mcp:review-feed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := New(tc.source, tc.scope, tc.text)
			if d.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", d.ID, tc.wantID)
			}
			if d.Source != tc.source || d.Scope != tc.scope || d.Text != tc.text {
				t.Errorf("New(%q,%q,%q) = %+v, fields not preserved", tc.source, tc.scope, tc.text, d)
			}
			if d.Severity != SeverityDisclosedUnproven {
				t.Errorf("Severity = %q, want %q (v1's only value)", d.Severity, SeverityDisclosedUnproven)
			}
		})
	}
}

func TestNew_Deterministic(t *testing.T) {
	// Same inputs must always re-derive the same ID (no wall-clock or
	// randomness, CLAUDE.md) — a caller can diff two enumerations without
	// persisting anything.
	a := New("lint:VL-017", "spec/x", "text")
	b := New("lint:VL-017", "spec/x", "text")
	if a.ID != b.ID {
		t.Fatalf("New is not deterministic: %q != %q", a.ID, b.ID)
	}
	if a != b {
		t.Fatalf("New(same inputs) produced different Disclosure values: %+v != %+v", a, b)
	}
}

func TestRender(t *testing.T) {
	cases := []struct {
		name string
		d    Disclosure
		want string
	}{
		{
			name: "with scope",
			d:    New("lint:VL-017", "spec/disclosure-legibility", "mutable zone absent"),
			want: "disclosed-unproven [lint:VL-017] spec/disclosure-legibility: mutable zone absent",
		},
		{
			name: "no scope",
			d:    New("mcp:review-feed", "", "forge unreachable"),
			want: "disclosed-unproven [mcp:review-feed]: forge unreachable",
		},
		{
			name: "empty text is not an error, just an empty explanation",
			d:    New("gate:example", "", ""),
			want: "disclosed-unproven [gate:example]: ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Render(tc.d); got != tc.want {
				t.Errorf("Render(%+v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// TestRender_EqualDisclosuresRenderIdentically is ac-2's exerciser at the
// package level: two independently-constructed but equal Disclosure
// values must render byte-identical text — the property every migrated
// call site (lint, gate, mcp/workbench) depends on.
func TestRender_EqualDisclosuresRenderIdentically(t *testing.T) {
	a := New("lint:VL-999", "spec/example", "example input is absent")
	b := New("lint:VL-999", "spec/example", "example input is absent")
	if Render(a) != Render(b) {
		t.Fatalf("Render(a) = %q, Render(b) = %q; equal Disclosures must render identically", Render(a), Render(b))
	}
}

// TestIsRendered is the recognizer's own table: every string Render can
// produce is recognized, and the near-misses a consumer would otherwise
// hand-match with a prefix test are not.
//
// The negative rows are the point. A consumer that reconstructs the render
// format locally (cmd/verdi's closure-gate reporting loop did, with
// strings.HasPrefix(SeverityDisclosedUnproven+" [")) both accepts prose that
// merely opens with the severity word and silently zeroes its count the day
// Render's format changes — with no compile error and no failing test outside
// this package.
func TestIsRendered(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"rendered with a scope", Render(New("gate:evidence-quarantine", "ac-1", "a record was excluded")), true},
		{"rendered without a scope", Render(New("mcp:review-feed", "", "forge unreachable")), true},
		{"rendered with an empty text", Render(New("gate:example", "", "")), true},
		{"rendered with a scope carrying its own colon", Render(New("gate:x", "spec/a: b", "text")), true},
		{"rendered with a source carrying its own bracket", Render(New("a]b", "", "text")), true},
		{"rendered line indented for a nested report", "  " + Render(New("gate:x", "", "text")), false},
		{"prose that merely opens with the severity word", SeverityDisclosedUnproven + " is what this line reports", false},
		{"prose naming the severity mid-sentence", "the check is disclosed-unproven [gate:x]: text", false},
		// An empty source is a real Render output — New accepts it, Render emits
		// `disclosed-unproven []: text`, so the recognizer must accept it too:
		// the guarantee is EXACTLY Render's image, and rejecting this row was one
		// of the forward-direction misses (New("","","text")) the property test
		// now guards against.
		{"the empty-source form New/Render actually produce", SeverityDisclosedUnproven + " []: text", true},
		{"an unterminated source bracket", SeverityDisclosedUnproven + " [gate:x", false},
		{"a source with no text separator at all", SeverityDisclosedUnproven + " [gate:x] just more words", false},
		{"an informational tally line the feature gate also carries", "       [union over the feature's own report + 2 closed implementing story archive(s): accepted-deviation count 1]", false},
		{"an ordinary gate PASS line", "[PASS] closure(feature): 1. every feature AC evidenced", false},
		{"the empty string", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsRendered(tc.s); got != tc.want {
				t.Fatalf("IsRendered(%q) = %v, want %v", tc.s, got, tc.want)
			}
		})
	}
}

// TestIsRendered_RecognizesEveryRender closes the drift loop the predicate
// exists for: whatever Render emits, IsRendered accepts. A future edit to
// Render's format that this package does not also teach IsRendered fails
// HERE, in the package that owns the format, rather than silently zeroing a
// disclosure count in cmd/verdi.
//
// It is generated over the CROSS PRODUCT of adversarial sources, scopes and
// texts rather than a handful of realistic literals, because the realistic
// literals were exactly the problem: every source in the tree today is a
// package constant with no bracket in it, so a fixed list proves the invariant
// only for producers that already exist. The generated rows include the ones
// that broke it — a source carrying "]", an empty source, a whitespace scope,
// a scope carrying the ": " separator — so a future producer that reaches any
// of them is caught here and not by a silently short disclosure count.
func TestIsRendered_RecognizesEveryRender(t *testing.T) {
	sources := []string{
		"lint:VL-017",
		"gate:pending-supersession",
		"close:uncommitted-fold-record",
		"",          // New accepts it; Render emits it; the recognizer must too.
		" ",         // whitespace-only
		"a]b",       // a bracket inside the source: the first "]" is not the end
		"]",         // nothing but a bracket
		"a]b]c",     // several
		"[x]",       // brackets in both directions
		"gate:x: y", // the text separator inside the source
		"disclosed-unproven [z]",
	}
	scopes := []string{
		"",
		"spec/x",
		".verdi/waivers/jira-close-1/ac-1.md",
		" ",         // whitespace-only: Render still takes the scoped branch
		"a]b",       // a bracket inside the scope
		"spec/a: b", // the text separator inside the scope
		"]: ",
		"  ",
	}
	texts := []string{"", "text", ": ", "]", "a: b", "\n"}

	for _, source := range sources {
		for _, scope := range scopes {
			for _, text := range texts {
				d := New(source, scope, text)
				if got := Render(d); !IsRendered(got) {
					t.Fatalf("IsRendered(Render(%+v)) = false on %q; every rendered disclosure must be recognizable, or a consumer's disclosure count is silently short", d, got)
				}
			}
		}
	}
}
