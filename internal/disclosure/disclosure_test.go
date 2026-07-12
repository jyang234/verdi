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
