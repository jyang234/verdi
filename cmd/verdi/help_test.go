// Built-binary end-to-end tests for ac-2 (spec/uat-round-1): "help",
// "--help", and "-h" at top level, and "verdi <verb> --help"/"-h"/"help"
// after any verb, print multi-line usage on stdout and exit 0 without
// ever executing the verb. Run as a real OS process (buildVerdiBinary/
// runVerdiBinary, the same convention model_test.go/
// obligationseam_e2e_test.go already established) since run() writes
// every verb's real output straight to the process's own os.Stdout —
// only a real subprocess lets a test capture and assert on it.
package main

import (
	"strings"
	"testing"
)

// TestHelp_TopLevel covers all three recognized top-level spellings
// (ac-2): multi-line usage on stdout, one line per verb, exit 0, nothing
// on stderr. Table-driven per CLAUDE.md's testing rules.
func TestHelp_TopLevel(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	for _, arg := range []string{"help", "--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, arg)
			if code != 0 {
				t.Fatalf("verdi %s exit = %d, want 0\nstdout: %s\nstderr: %s", arg, code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("verdi %s stderr = %q, want empty", arg, stderr)
			}
			if n := strings.Count(stdout, "\n"); n < 2 {
				t.Fatalf("verdi %s stdout = %q, want multi-line usage", arg, stdout)
			}
			if !strings.HasPrefix(stdout, "usage: verdi <verb>") {
				t.Fatalf("verdi %s stdout = %q, want it to start with the usage line", arg, stdout)
			}
			// One line per verb (ac-2): every real dispatchable verb name
			// must be discoverable in the listing.
			for _, verb := range []string{
				"lint", "design", "accept", "feature", "build", "align", "sync",
				"serve", "mcp", "matrix", "rollup", "close", "disposition",
				"waivers", "verify-artifact", "dex", "gc", "gate", "board",
				"audit", "attest", "model", "init", "obligation", "waive",
				"spec", "journey", "context", "experiment", "version", "help",
			} {
				if !strings.Contains(stdout, verb) {
					t.Errorf("verdi %s stdout missing verb %q:\n%s", arg, verb, stdout)
				}
			}
		})
	}
}

// TestHelp_PerVerb proves the central intercept fires for every one of
// the three per-verb spellings, for a representative sample of verbs
// spanning: a verb dispatched before the phase-map lookup (lint), a verb
// with a rich subcommand family (design, context), a verb whose CURRENT
// behavior the spec names as the two defects to fix (lint, spec), and an
// out-of-v0-scope verb (waivers). Every case: exit 0, stdout carries that
// verb's own usage, stderr empty.
func TestHelp_PerVerb(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	cases := []struct {
		verb       string
		wantSubstr string
	}{
		{"lint", "usage: verdi lint"},
		{"design", "usage: verdi design start"},
		{"context", "usage: verdi context compile"},
		{"spec", "usage: verdi spec state"},
		{"obligation", "usage: verdi obligation <author"},
		{"waivers", "verdi waivers"},
		{"disposition", "usage: verdi disposition"},
	}

	for _, tc := range cases {
		for _, help := range []string{"--help", "-h", "help"} {
			t.Run(tc.verb+"/"+help, func(t *testing.T) {
				stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, tc.verb, help)
				if code != 0 {
					t.Fatalf("verdi %s %s exit = %d, want 0\nstdout: %s\nstderr: %s", tc.verb, help, code, stdout, stderr)
				}
				if stderr != "" {
					t.Fatalf("verdi %s %s stderr = %q, want empty", tc.verb, help, stderr)
				}
				if !strings.Contains(stdout, tc.wantSubstr) {
					t.Fatalf("verdi %s %s stdout = %q, want substring %q", tc.verb, help, stdout, tc.wantSubstr)
				}
			})
		}
	}
}

// TestHelp_InitUsageMatchesConstant is review round 1, Minor 2's closure:
// help.go's verbUsage["init"] row now derives from init.go's own
// initUsageText constant ("usage: " + initUsageText, not a second
// hand-typed literal), so the two cannot drift apart structurally — this
// test pins the whole plumbing end to end at the built-binary level, on
// top of that compile-time guarantee: "verdi init --help"/"-h"/"help"
// must print EXACTLY "usage: " + initUsageText, the identical string
// cmdInit's own flag-parsing refusals cite.
func TestHelp_InitUsageMatchesConstant(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	want := "usage: " + initUsageText + "\n"
	for _, help := range []string{"--help", "-h", "help"} {
		t.Run(help, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "init", help)
			if code != 0 {
				t.Fatalf("verdi init %s exit = %d, want 0\nstdout: %s\nstderr: %s", help, code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("verdi init %s stderr = %q, want empty", help, stderr)
			}
			if stdout != want {
				t.Fatalf("verdi init %s stdout = %q, want exactly %q", help, stdout, want)
			}
		})
	}
}

// TestHelp_LintNeverExecutes is the exact regression the spec names:
// "today `verdi lint --help` runs a full lint" (ac-2). Run from a
// directory with NO .verdi/ ancestor at all: if lint actually ran, it
// would fail operationally trying to resolve a store root (exit 2, a
// store-root error on stderr) rather than printing usage on stdout with
// exit 0 — so this single assertion set distinguishes "printed usage" from
// "tried to run".
func TestHelp_LintNeverExecutes(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // deliberately not a verdi store

	for _, help := range []string{"--help", "-h", "help"} {
		t.Run(help, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "lint", help)
			if code != 0 {
				t.Fatalf("verdi lint %s exit = %d, want 0 (lint must not have run)\nstdout: %s\nstderr: %s", help, code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("verdi lint %s stderr = %q, want empty", help, stderr)
			}
			if !strings.Contains(stdout, "usage: verdi lint") {
				t.Fatalf("verdi lint %s stdout = %q, want usage text, not lint findings", help, stdout)
			}
		})
	}
}

// TestHelp_SpecShowsEveryForm is the other exact regression the spec
// names: "`verdi spec --help` prints only the `spec state` form" — today
// via the usage-error path (stderr, exit 2). After the fix it must be a
// clean exit (stdout, exit 0). `spec` now has two real forms (`state`
// and `doc`, Task 7); --help must show both. It must also be
// byte-identical to what a bare `verdi spec` invocation (no subcommand)
// prints to stderr (fix round 1, F5): both read the SAME specVerbUsage
// constant — help.go's verbUsage["spec"] entry is that constant, not a
// hand-duplicated literal — so the two call sites can never drift apart.
func TestHelp_SpecShowsEveryForm(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "spec", "--help")
	if code != 0 {
		t.Fatalf("verdi spec --help exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("verdi spec --help stderr = %q, want empty (not the usage-error path)", stderr)
	}
	if !strings.Contains(stdout, "usage: verdi spec state <spec-ref>") {
		t.Fatalf("stdout = %q, want the spec state form", stdout)
	}
	if !strings.Contains(stdout, "verdi spec doc <spec-ref>") {
		t.Fatalf("stdout = %q, want the spec doc form", stdout)
	}

	bareStdout, bareStderr, bareCode := runVerdiBinary(t, bin, dir, nil, "spec")
	if bareCode != 2 {
		t.Fatalf("bare verdi spec exit = %d, want 2", bareCode)
	}
	if bareStdout != "" {
		t.Fatalf("bare verdi spec stdout = %q, want empty", bareStdout)
	}
	if bareStderr != stdout {
		t.Fatalf("bare `verdi spec` stderr and `verdi spec --help` stdout must be byte-identical (fix round 1, F5):\nbare stderr:   %q\n--help stdout: %q", bareStderr, stdout)
	}
}

// TestHelp_UnknownVerbAndNoArgsUnchanged pins co-2's explicit carve-out:
// "Unknown verb and no-args behavior stay as they are (usage to stderr,
// exit 2)" — the central help intercept must never swallow these two
// existing paths.
func TestHelp_UnknownVerbAndNoArgsUnchanged(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	cases := []struct {
		name string
		args []string
	}{
		{"unknown verb", []string{"frobnicate"}},
		{"no args", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, tc.args...)
			if code != 2 {
				t.Fatalf("verdi %v exit = %d, want 2\nstdout: %s\nstderr: %s", tc.args, code, stdout, stderr)
			}
			if stdout != "" {
				t.Fatalf("verdi %v stdout = %q, want empty (usage stays on stderr)", tc.args, stdout)
			}
			if !strings.Contains(stderr, "usage: verdi <verb>") {
				t.Fatalf("verdi %v stderr = %q, want the usage banner", tc.args, stderr)
			}
		})
	}
}

// TestHelp_DoesNotInterceptRealSubcommands proves the per-verb intercept
// only fires on the exact tokens "--help"/"-h"/"help" immediately after a
// verb — a real subcommand (e.g. design's "board") must dispatch exactly
// as before: its own usage error, on stderr, exit 2 (never help.go's
// registry, never stdout).
func TestHelp_DoesNotInterceptRealSubcommands(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "design", "board")
	if code != 2 {
		t.Fatalf("verdi design board (no spec-ref) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty (this is design board's own usage error, not help)", stdout)
	}
	if !strings.Contains(stderr, "usage: verdi design board <spec-ref>") {
		t.Fatalf("stderr = %q, want design board's own usage message", stderr)
	}
}

// topLevelUsageHasRow reports whether verb has its own row in
// topLevelUsage — a line that, after its leading indent, starts with
// verb followed by a space (the column-formatted "  <verb><padding>
// <description>" shape every real row uses) or is exactly verb alone.
// A plain strings.Contains(topLevelUsage, verb) would false-positive on
// e.g. "spec" (a substring of design's own "specifications" in its
// description) and so could not actually catch a deleted row — this
// checks the LEFT COLUMN specifically.
func topLevelUsageHasRow(verb string) bool {
	for _, line := range strings.Split(topLevelUsage, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == verb {
			return true
		}
		if rest, ok := strings.CutPrefix(trimmed, verb+" "); ok && rest != "" {
			return true
		}
	}
	return false
}

// TestVerbUsageRegistry_CoversEveryVerb is a drift guard (Opus L1 review,
// ACCEPT-WITH-MINOR finding 1): every verbPhase key, plus "lint"
// (dispatched before the phase map is ever consulted, dispatch.go), must
// have its own verbUsage entry AND its own row in topLevelUsage. Without
// this test, a future verb added to verbPhase with no matching help.go
// update would silently print verbUsageOrFallback's content-free "usage:
// verdi <verb>" line (no real form information) and/or be undiscoverable
// from "verdi help" — both content-free failures a human skimming test
// output would not necessarily notice, exactly what this guard is for.
func TestVerbUsageRegistry_CoversEveryVerb(t *testing.T) {
	t.Parallel()
	verbs := make([]string, 0, len(verbPhase)+1)
	for v := range verbPhase {
		verbs = append(verbs, v)
	}
	verbs = append(verbs, "lint")

	for _, verb := range verbs {
		t.Run(verb, func(t *testing.T) {
			usage, ok := verbUsage[verb]
			if !ok {
				t.Fatalf("verbUsage has no entry for %q — dispatch.go recognizes this verb (verbPhase or the lint special case) but help.go's registry does not, so \"verdi %s --help\" would silently fall back to the content-free \"usage: verdi %s\" line", verb, verb, verb)
			}
			if strings.TrimSpace(usage) == "" {
				t.Fatalf("verbUsage[%q] is present but empty", verb)
			}
			if !topLevelUsageHasRow(verb) {
				t.Fatalf("topLevelUsage has no row for %q — \"verdi help\" would not list it", verb)
			}
		})
	}
}

// TestHelp_VersionRowHelpShowsItsOwnUsage is ACCEPT-WITH-MINOR finding 2:
// "verdi version --help"/"-h"/"help" (and the same three suffixes on
// "--version") must print version's OWN verbUsage row, not silently run
// the version report itself — otherwise topLevelUsage's closing sentence
// ("run \"verdi <verb> help\" ... for that verb's own usage") would be
// false for the "version" row specifically.
func TestHelp_VersionRowHelpShowsItsOwnUsage(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	for _, verbForm := range []string{"version", "--version"} {
		for _, help := range []string{"--help", "-h", "help"} {
			t.Run(verbForm+"/"+help, func(t *testing.T) {
				stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, verbForm, help)
				if code != 0 {
					t.Fatalf("verdi %s %s exit = %d, want 0\nstdout: %s\nstderr: %s", verbForm, help, code, stdout, stderr)
				}
				if stderr != "" {
					t.Fatalf("verdi %s %s stderr = %q, want empty", verbForm, help, stderr)
				}
				if !strings.Contains(stdout, "usage: verdi version") {
					t.Fatalf("verdi %s %s stdout = %q, want version's own usage, not the version line itself", verbForm, help, stdout)
				}
				if strings.Contains(stdout, "verdi (devel)") || strings.HasPrefix(stdout, "verdi v") {
					t.Fatalf("verdi %s %s stdout = %q, looks like the version report ran instead of showing usage", verbForm, help, stdout)
				}
			})
		}
	}
}

// TestHelp_HelpRowHelpShowsItsOwnUsage is the same finding 2 case for the
// "help" row: "verdi help help"/"--help"/"-h" (and the same three
// suffixes on "--help"/"-h" as the opening token) must print help's OWN
// verbUsage row, not a second dump of topLevelUsage.
func TestHelp_HelpRowHelpShowsItsOwnUsage(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	for _, opener := range []string{"help", "--help", "-h"} {
		for _, help := range []string{"--help", "-h", "help"} {
			t.Run(opener+"/"+help, func(t *testing.T) {
				stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, opener, help)
				if code != 0 {
					t.Fatalf("verdi %s %s exit = %d, want 0\nstdout: %s\nstderr: %s", opener, help, code, stdout, stderr)
				}
				if stderr != "" {
					t.Fatalf("verdi %s %s stderr = %q, want empty", opener, help, stderr)
				}
				if !strings.Contains(stdout, "usage: verdi help") {
					t.Fatalf("verdi %s %s stdout = %q, want help's own usage row", opener, help, stdout)
				}
				if strings.Contains(stdout, "verbs:") {
					t.Fatalf("verdi %s %s stdout = %q, looks like topLevelUsage printed again instead of help's own row", opener, help, stdout)
				}
			})
		}
	}
}
