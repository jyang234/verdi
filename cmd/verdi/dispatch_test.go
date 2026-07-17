package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestRun_KnownVerbs is the happy path: every spec-named verb still
// stubbed at this phase parses and exits 2 with a one-line "not
// implemented" message on stderr. design/accept/feature graduated to real
// implementations in Phase 7, align/gate in Phase 8, close/gc in round 6 —
// see TestRun_DesignDispatchesToRealVerb (design_test.go),
// TestRun_AcceptDispatchesToRealVerb (accept_test.go),
// TestRun_FeatureDispatchesToRealVerb (feature_test.go),
// TestRun_AlignDispatchesToRealVerb/TestRun_GateDispatchesToRealVerb/
// TestRun_CloseDispatchesToRealVerb/TestRun_GcDispatchesToRealVerb below
// for their dispatch coverage, matching the lint/dex pattern. Table-driven
// per CLAUDE.md's testing rules.
func TestRun_KnownVerbs(t *testing.T) {
	cases := []struct {
		verb       string
		wantSubstr string
	}{
		{"waivers", "not implemented (out of v0 scope)"},
		{"verify-artifact", "not implemented (out of v0 scope)"},
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
// verb do not change dispatch (verb-only parsing at phase 1). `lint`,
// `design`/`accept`/`feature`, `align`/`gate`, and `close`/`gc` are now
// implemented (phases 4, 7, 8, and round 6), so this uses a still-stubbed
// verb.
func TestRun_KnownVerbs_ExtraArgs(t *testing.T) {
	var stderr bytes.Buffer
	got := run([]string{"waivers", "--some-flag", "extra"}, &stderr)
	if got != 2 {
		t.Fatalf("run with extra args exit = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "not implemented (out of v0 scope)") {
		t.Fatalf("stderr = %q, want the out-of-v0-scope message", stderr.String())
	}
}

// TestRun_AlignDispatchesToRealVerb proves `run` routes "align" to the real
// implementation (align.go, PLAN.md Phase 8) rather than the generic
// phase-stub path: outside any store root it must fail with align's own
// store-root error, never the generic "not implemented" message.
func TestRun_AlignDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"align"}, &stderr)
	if got != 2 {
		t.Fatalf("run([align]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRun_GateDispatchesToRealVerb is align's own analogue for "gate" (I-7).
func TestRun_GateDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"gate"}, &stderr)
	if got != 2 {
		t.Fatalf("run([gate]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRun_CloseDispatchesToRealVerb proves `run` routes "close" to the real
// implementation (close.go, round 6/spec/close-verb) rather than the
// generic phase-stub path (I-23's old "not implemented (out of v0 scope)"):
// a bare "close" (no story/spec argument) must produce close's own usage
// message, never the generic stub message.
func TestRun_CloseDispatchesToRealVerb(t *testing.T) {
	var stderr bytes.Buffer
	got := run([]string{"close"}, &stderr)
	if got != 2 {
		t.Fatalf("run([close]) = %d, want 2 (usage error, no story/spec argument given)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want close's own usage message, not the generic stub message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi close") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi close'", stderr.String())
	}
}

// TestRun_GcDispatchesToRealVerb proves `run` routes "gc" to the real
// implementation (gc.go, round 6/spec/worktree-manager) rather than the
// generic phase-stub path (I-23's old "not implemented (out of v0
// scope)"): outside any store root it must fail with gc's own store-root
// error, never the generic "not implemented" message.
func TestRun_GcDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"gc"}, &stderr)
	if got != 2 {
		t.Fatalf("run([gc]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
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

// TestRun_ServeDispatchesToRealVerb proves `run` routes "serve" to the
// real implementation (serve.go, PLAN.md Phase 9) rather than the generic
// phase-stub path: outside any store root it must fail with serve's own
// store-root error, never the generic "not implemented" message.
func TestRun_ServeDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"serve"}, &stderr)
	if got != 2 {
		t.Fatalf("run([serve]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRun_McpDispatchesToRealVerb proves `run` routes "mcp" to the real
// implementation (mcp.go, PLAN.md Phase 9) the same way.
func TestRun_McpDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())

	var stderr bytes.Buffer
	got := run([]string{"mcp"}, &stderr)
	if got != 2 {
		t.Fatalf("run([mcp]) outside a store = %d, want 2 (operational)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// TestRun_DispositionDispatchesToRealVerb proves `run` routes "disposition"
// to the real implementation (disposition.go, spec/disposition-verb) rather
// than the generic phase-stub path: a bare "disposition" (no arguments at
// all) must produce disposition's own usage message, never the generic
// "not implemented" stub message — mirroring TestRun_CloseDispatchesToRealVerb,
// since both are mutating verbs whose bare invocation fails on argument
// parsing before touching any file.
func TestRun_DispositionDispatchesToRealVerb(t *testing.T) {
	var stderr bytes.Buffer
	got := run([]string{"disposition"}, &stderr)
	if got != 2 {
		t.Fatalf("run([disposition]) = %d, want 2 (usage error, no arguments given)", got)
	}
	if strings.Contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want disposition's own usage message, not the generic stub message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "usage: verdi disposition") {
		t.Fatalf("stderr = %q, want it to mention 'usage: verdi disposition'", stderr.String())
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
