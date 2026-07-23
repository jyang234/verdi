package main

import "testing"

func TestShellQuoteWord(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple finding id", input: "finding-1", want: "finding-1"},
		{name: "canonical spec ref", input: "spec/close-fixture", want: "spec/close-fixture"},
		{name: "empty", input: "", want: `''`},
		{name: "spaces", input: "two words", want: `'two words'`},
		{name: "shell metacharacters", input: `$HOME; $(touch nope) | <bad>`, want: `'$HOME; $(touch nope) | <bad>'`},
		{name: "single quote", input: `judge's finding`, want: `'judge'"'"'s finding'`},
		{name: "newline", input: "line\nbreak", want: "'line\nbreak'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuoteWord(tc.input); got != tc.want {
				t.Fatalf("shellQuoteWord(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
