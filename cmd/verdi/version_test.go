// Built-binary end-to-end tests for `verdi version`/`verdi --version`
// (spec/uat-round-1 ac-1, CLI part; closes UAT-003): both forms must
// print one line identifying the build and exit 0, on stdout, from
// runtime/debug.ReadBuildInfo via internal/buildinfo — never a fabricated
// version. Run as a real OS process (mirroring model_test.go/
// obligationseam_e2e_test.go's buildVerdiBinary/runVerdiBinary
// convention) rather than calling run() in-process, since run() writes
// every verb's real stdout straight to the process's own os.Stdout with
// no injectable writer — only a real subprocess lets a test capture it.
package main

import (
	"strings"
	"testing"
)

// TestVersion_PrintsIdentificationLine is the happy path for both
// recognized spellings (ac-1: "`verdi version` and `verdi --version`
// print one line identifying the build"): exit 0, exactly one line, on
// stdout, nothing on stderr, starting with "verdi " (never empty, never
// a bare fabricated string).
func TestVersion_PrintsIdentificationLine(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, args...)
			if code != 0 {
				t.Fatalf("verdi %v exit = %d, want 0\nstdout: %s\nstderr: %s", args, code, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("verdi %v stderr = %q, want empty", args, stderr)
			}
			if !strings.HasPrefix(stdout, "verdi ") {
				t.Fatalf("verdi %v stdout = %q, want it to start with %q", args, stdout, "verdi ")
			}
			if n := strings.Count(strings.TrimRight(stdout, "\n"), "\n"); n != 0 {
				t.Fatalf("verdi %v stdout = %q, want exactly one line", args, stdout)
			}
		})
	}
}

// TestVersion_BothFormsAgree proves "version" and "--version" report the
// SAME build identically (ac-1's whole point: a result must always trace
// back to the exact build that produced it, so the two spellings of "ask
// the build who it is" can never disagree).
func TestVersion_BothFormsAgree(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdoutVerb, _, codeVerb := runVerdiBinary(t, bin, dir, nil, "version")
	stdoutFlag, _, codeFlag := runVerdiBinary(t, bin, dir, nil, "--version")
	if codeVerb != 0 || codeFlag != 0 {
		t.Fatalf("exit codes = %d, %d, want both 0", codeVerb, codeFlag)
	}
	if stdoutVerb != stdoutFlag {
		t.Fatalf("verdi version = %q, verdi --version = %q, want identical", stdoutVerb, stdoutFlag)
	}
}

// TestVersion_IgnoresExtraArguments is the negative/edge path: trailing
// arguments never turn "version" into an operational error (co-2:
// "Help and version are clean exits") — it stays a clean, unconditional
// report, the same posture several already-stubbed verbs take toward
// trailing arguments (dispatch_test.go's TestRun_KnownVerbs_ExtraArgs).
func TestVersion_IgnoresExtraArguments(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir()

	stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "version", "--some-flag", "extra")
	if code != 0 {
		t.Fatalf("verdi version with extra args exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "verdi ") {
		t.Fatalf("stdout = %q, want it to start with %q", stdout, "verdi ")
	}
}

// TestVersion_RunsOutsideAnyStore proves version never resolves a store
// root (unlike almost every real verb) — it must succeed identically run
// from a directory with no .verdi/ ancestor at all.
func TestVersion_RunsOutsideAnyStore(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // deliberately not a verdi store

	stdout, stderr, code := runVerdiBinary(t, bin, dir, nil, "version")
	if code != 0 {
		t.Fatalf("verdi version outside a store exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "verdi ") {
		t.Fatalf("stdout = %q, want it to start with %q", stdout, "verdi ")
	}
}
