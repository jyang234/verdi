package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

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

// TestShellQuoteWord_QuotesEqualsForZshEqualsExpansion pins the one shell
// this word's safe set previously did not cover. Every character of "=ls"
// is POSIX-literal, so the pre-fix safe set emitted it BARE — and zsh (the
// macOS default shell, where an operator pastes these templates) applies
// EQUALS expansion to a word starting with '=', turning the bare word into
// the ls binary's path. A finding id is arbitrary artifact text, so a bare
// "=ls" would disposition an id the operator never saw printed.
func TestShellQuoteWord_QuotesEqualsForZshEqualsExpansion(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "equals-prefixed, otherwise all-safe", input: "=ls", want: `'=ls'`},
		{name: "equals-prefixed path shape", input: "=/bin/ls", want: `'=/bin/ls'`},
		{name: "interior equals", input: "kind=computed", want: `'kind=computed'`},
		{name: "trailing equals", input: "id=", want: `'id='`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuoteWord(tc.input); got != tc.want {
				t.Fatalf("shellQuoteWord(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestShellQuoteWord_RoundTripsThroughRealShells is the differential
// witness behind the safe set: every emitted word must come back
// byte-identical after a real shell has parsed it, in each shell an
// operator plausibly pastes into. /bin/sh is present everywhere; /bin/zsh
// is the macOS default login shell and the one that motivated dropping '='
// from the safe set. A shell that is absent is DISCLOSED (t.Skip names it)
// rather than silently reducing what this test proves.
func TestShellQuoteWord_RoundTripsThroughRealShells(t *testing.T) {
	words := []string{
		"=ls",
		"=/bin/ls",
		"kind=computed",
		"finding-1",
		"spec/close-fixture",
		"spec/close-fixture#ac-1",
		"--amend",
		"two words",
		`judge's finding`,
		`$HOME; $(touch nope) | <bad>`,
		"~root",
		"a{b,c}",
		"tilde~inside",
	}

	// prelude is per-shell, because the hazard depends on options an
	// ordinary user sets: zsh's extended_glob (widely enabled, including by
	// popular zsh frameworks) makes '#' a pattern operator, so a bare
	// fragment ref dies with "no matches found" before the command runs.
	shells := []struct{ path, prelude string }{
		{path: "/bin/sh"},
		{path: "/bin/zsh", prelude: "setopt extended_glob; "},
		{path: "/bin/bash"},
	}

	for _, shell := range shells {
		t.Run(shell.path, func(t *testing.T) {
			if _, err := os.Stat(shell.path); err != nil {
				t.Skipf("%s is absent on this machine: its round-trip is disclosed-unproven here (%v)", shell.path, err)
			}
			for _, word := range words {
				// printf %s writes the parsed word with no added newline, so
				// stdout is exactly what the shell made of the emitted word.
				script := shell.prelude + "set -- " + shellQuoteWord(word) + `; printf %s "$1"`
				cmd := exec.Command(shell.path, "-c", script)
				var stdout, stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("%s -c %q: %v; stderr=%s", shell.path, script, err, stderr.String())
				}
				if stdout.String() != word {
					t.Fatalf("%s parsed %s as %q, want the original word %q", shell.path, shellQuoteWord(word), stdout.String(), word)
				}
			}
		})
	}
}

// TestIsSafeShellWordRune tests the safe-set predicate directly: a bare
// word is emitted only when EVERY rune passes, so this table is the actual
// boundary shellQuoteWord's callers depend on.
func TestIsSafeShellWordRune(t *testing.T) {
	tests := []struct {
		name string
		in   rune
		want bool
	}{
		{name: "lowercase letter", in: 'a', want: true},
		{name: "lowercase boundary z", in: 'z', want: true},
		{name: "uppercase letter", in: 'A', want: true},
		{name: "uppercase boundary Z", in: 'Z', want: true},
		{name: "digit", in: '0', want: true},
		{name: "digit boundary 9", in: '9', want: true},
		{name: "underscore", in: '_', want: true},
		{name: "at", in: '@', want: true},
		{name: "percent", in: '%', want: true},
		{name: "plus", in: '+', want: true},
		{name: "colon", in: ':', want: true},
		{name: "comma", in: ',', want: true},
		{name: "dot", in: '.', want: true},
		{name: "slash", in: '/', want: true},
		{name: "hyphen", in: '-', want: true},
		{name: "equals is unsafe: zsh EQUALS expansion", in: '=', want: false},
		{name: "space", in: ' ', want: false},
		{name: "tab", in: '\t', want: false},
		{name: "newline", in: '\n', want: false},
		{name: "dollar", in: '$', want: false},
		{name: "backtick", in: '`', want: false},
		{name: "single quote", in: '\'', want: false},
		{name: "double quote", in: '"', want: false},
		{name: "backslash", in: '\\', want: false},
		{name: "semicolon", in: ';', want: false},
		{name: "pipe", in: '|', want: false},
		{name: "ampersand", in: '&', want: false},
		{name: "star", in: '*', want: false},
		{name: "question mark", in: '?', want: false},
		{name: "open bracket", in: '[', want: false},
		{name: "open brace", in: '{', want: false},
		{name: "tilde", in: '~', want: false},
		{name: "hash", in: '#', want: false},
		{name: "bang", in: '!', want: false},
		{name: "open paren", in: '(', want: false},
		{name: "less than", in: '<', want: false},
		{name: "greater than", in: '>', want: false},
		{name: "caret", in: '^', want: false},
		{name: "non-ascii letter", in: 'é', want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSafeShellWordRune(tc.in); got != tc.want {
				t.Fatalf("isSafeShellWordRune(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
