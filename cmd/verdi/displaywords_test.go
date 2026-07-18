package main

import "testing"

// TestDisplayAlternation pins the singular/plural alternation form: the
// no-rename pair reproduces today's hand-written "stor(y/ies)" byte-for-
// byte (the parity floor), a regular rename parenthesizes the extending
// tail, and degenerate pairs never emit an empty alternation.
func TestDisplayAlternation(t *testing.T) {
	tests := []struct{ singular, plural, want string }{
		{"story", "stories", "stor(y/ies)"},
		{"Change Request", "Change Requests", "Change Request(s)"},
		{"Workstream", "Workstreams", "Workstream(s)"},
		{"spike", "spikes", "spike(s)"},
		{"box", "boxes", "box(es)"},
		{"sheep", "sheep", "sheep"}, // identical forms: no alternation to render
		{"", "", ""},                // degenerate; display words are never empty in practice
	}
	for _, tt := range tests {
		if got := displayAlternation(tt.singular, tt.plural); got != tt.want {
			t.Fatalf("displayAlternation(%q, %q) = %q, want %q", tt.singular, tt.plural, got, tt.want)
		}
	}
}
