package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_KnownVerbs is the happy path: every spec-named verb parses and
// exits 2 with a one-line "not implemented" message on stderr (I-7 for
// `gate`; the rest per 05 §CLI). Table-driven per CLAUDE.md's testing rules.
func TestRun_KnownVerbs(t *testing.T) {
	cases := []struct {
		verb       string
		wantSubstr string
	}{
		{"design", "not implemented (phase 7)"},
		{"accept", "not implemented (phase 7)"},
		{"feature", "not implemented (phase 7)"},
		{"align", "not implemented (phase 8)"},
		{"serve", "not implemented (phase 9)"},
		{"mcp", "not implemented (phase 9)"},
		{"rollup", "not implemented (phase 11)"},
		{"close", "not implemented (out of v0 scope)"},
		{"waivers", "not implemented (out of v0 scope)"},
		{"verify-artifact", "not implemented (out of v0 scope)"},
		{"gc", "not implemented (out of v0 scope)"},
		{"gate", "not implemented (phase 8)"},
	}

	for _, tc := range cases {
		t.Run(tc.verb, func(t *testing.T) {
			var stderr bytes.Buffer
			got := run([]string{tc.verb}, &stderr)
			if got != 2 {
				t.Fatalf("run(%q) exit = %d, want 2", tc.verb, got)
			}
			if !strings.Contains(stderr.String(), tc.wantSubstr) {
				t.Fatalf("run(%q) stderr = %q, want substring %q", tc.verb, stderr.String(), tc.wantSubstr)
			}
			// Every spec-named verb's message is exactly one line.
			if n := strings.Count(strings.TrimRight(stderr.String(), "\n"), "\n"); n != 0 {
				t.Fatalf("run(%q) stderr not one line: %q", tc.verb, stderr.String())
			}
		})
	}
}

// TestRun_KnownVerbs_ExtraArgs asserts that trailing arguments after a known
// verb do not change dispatch (verb-only parsing at phase 1). `lint` is now
// implemented (phase 4 — see lint.go/lint_test.go), so this uses a
// still-stubbed verb.
func TestRun_KnownVerbs_ExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	got := run([]string{"design", "--some-flag", "extra"}, &stderr)
	if got != 2 {
		t.Fatalf("run with extra args exit = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "not implemented (phase 7)") {
		t.Fatalf("stderr = %q, want phase 7 message", stderr.String())
	}
}

// TestRun_LintDispatchesToRealVerb proves `run` routes "lint" to the real
// implementation (lint.go) rather than the generic phase-stub path: run
// from a directory with no .verdi/ ancestor, it must fail operationally
// with a store-root error, never the generic "usage" or "not implemented"
// messages other stubbed verbs still produce.
func TestRun_LintDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"lint"}, &stderr)
	if got != 2 {
		t.Fatalf("run([lint]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "usage") || strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRun_DexDispatchesToRealVerb proves `run` routes "dex" to the real
// implementation (dex.go, PLAN.md Phase 12) rather than the generic
// phase-stub path: a bare "dex" (no "build" subcommand) must produce dex's
// own usage message, never the generic "not implemented (phase 12)" other
// still-stubbed verbs produce.
func TestRun_DexDispatchesToRealVerb(t *testing.T) {
	var stderr bytes.Buffer
	got := run([]string{"dex"}, &stderr)
	if got != 2 {
		t.Fatalf("run([dex]) = %d, want 2 (usage error, no subcommand given)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want dex's own usage message, not the generic stub message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "verdi dex build") {
		t.Fatalf("stderr = %q, want it to mention 'verdi dex build'", stderr.String())
	}
}

// TestRun_NegativePaths covers the unknown-verb and no-args cases: both
// exit 2 with usage, never silently succeeding (constitution 2).
func TestRun_NegativePaths(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"unknown verb", []string{"frobnicate"}},
		{"no args", []string{}},
		{"empty string verb", []string{""}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got := run(tc.args, &stderr)
			if got != 2 {
				t.Fatalf("run(%v) exit = %d, want 2", tc.args, got)
			}
			if !strings.Contains(stderr.String(), "usage") {
				t.Fatalf("run(%v) stderr = %q, want usage message", tc.args, stderr.String())
			}
		})
	}
}
