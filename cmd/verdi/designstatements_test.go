// Tests for design start's statement-sourcing modes (spec/cli-creation
// ac-1/ac-2, ledger L-N7): --problem/--outcome, --defer-statements, the
// flagless TTY interview, and their refusals. The real, built-binary TTY
// interview path is proven via the disclosed VERDI_DESIGN_ASSUME_TTY=1
// stdin-script harness (mirroring init_test.go's own VERDI_INIT_ASSUME_TTY
// convention, an independent env var so the two verbs' test-injection
// surfaces never interfere with each other in their shared test binary).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/store"
)

// TestRunDesignStart_ProblemOutcomeFlags_TODOFree proves --problem/
// --outcome given together render the scaffold's statement attributes
// with the real supplied text — never the Default* TODO placeholders.
func TestRunDesignStart_ProblemOutcomeFlags_TODOFree(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		Problem: "the real problem", Outcome: "the real outcome"}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "real-statements", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, raw := readSpec(t, repo.Dir, "real-statements")
	if spec.Problem == nil || spec.Problem.Text != "the real problem" {
		t.Fatalf("Problem = %+v, want the supplied text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "the real outcome" {
		t.Fatalf("Outcome = %+v, want the supplied text", spec.Outcome)
	}
	if strings.Contains(string(raw), designscaffold.DefaultProblem) || strings.Contains(string(raw), designscaffold.DefaultOutcome) {
		t.Fatalf("scaffold still contains a Default placeholder:\n%s", raw)
	}
}

// TestRunDesignStart_DeferStatements_DisclosesAndKeepsPlaceholders proves
// --defer-statements commits the old TODO placeholders deliberately, with
// an explicit disclosure line on stdout naming the deferral.
func TestRunDesignStart_DeferStatements_DisclosesAndKeepsPlaceholders(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}, DeferStatements: true}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "deferred-statements", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "deferred-statements")
	if spec.Problem == nil || spec.Problem.Text != designscaffold.DefaultProblem {
		t.Fatalf("Problem = %+v, want the Default placeholder (deferred)", spec.Problem)
	}
	if !contains(stdout.String(), "defer") {
		t.Fatalf("stdout = %q, want an explicit deferral disclosure line", stdout.String())
	}
}

// TestRunDesignStart_StatementFlags_Negative covers every refusal shape:
// a lone --problem/--outcome, --defer-statements combined with either,
// and a flagless invocation with no attached terminal.
func TestRunDesignStart_StatementFlags_Negative(t *testing.T) {
	manifest := phase7Manifest(t)
	ctx := context.Background()

	cases := []struct {
		name string
		deps designDeps
	}{
		{"problem alone", designDeps{Provider: seedFakeProvider(t), GoTest: fakeGoTest{}, Problem: "p only"}},
		{"outcome alone", designDeps{Provider: seedFakeProvider(t), GoTest: fakeGoTest{}, Outcome: "o only"}},
		{"defer plus problem", designDeps{Provider: seedFakeProvider(t), GoTest: fakeGoTest{}, DeferStatements: true, Problem: "p"}},
		{"defer plus outcome", designDeps{Provider: seedFakeProvider(t), GoTest: fakeGoTest{}, DeferStatements: true, Outcome: "o"}},
		{"no flags, no TTY", designDeps{Provider: seedFakeProvider(t), GoTest: fakeGoTest{}, IsTTY: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildPhase7Repo(t)
			var stdout, stderr bytes.Buffer
			got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "refused-"+strings.ReplaceAll(tc.name, " ", "-"), manifest, phase7Model(t), tc.deps, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("runDesignStart(%s) = %d, want 2; stderr=%s", tc.name, got, stderr.String())
			}
			if stderr.Len() == 0 {
				t.Fatal("expected an explanatory stderr message")
			}
		})
	}
}

// TestRunDesignStart_TTYInterview_CollectsStatements proves the flagless,
// on-a-TTY path runs the interview (internal/designinterview) over the
// injected Stdin and lands the collected answers as the scaffold's real
// problem/outcome text.
func TestRunDesignStart_TTYInterview_CollectsStatements(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{
		Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		IsTTY: true, Stdin: strings.NewReader("interviewed problem\ninterviewed outcome\n"),
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassStory, "jira:LOAN-1482", "interviewed-story", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	spec, _ := readSpec(t, repo.Dir, "interviewed-story")
	if spec.Problem == nil || spec.Problem.Text != "interviewed problem" {
		t.Fatalf("Problem = %+v, want the interviewed text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "interviewed outcome" {
		t.Fatalf("Outcome = %+v, want the interviewed text", spec.Outcome)
	}
	if !contains(stdout.String(), "Problem") {
		t.Fatalf("stdout = %q, want the interview's own prompts", stdout.String())
	}
}

// TestRunDesignStart_TTYInterview_Aborted proves a short-stdin abort mid
// interview refuses (exit 2) rather than silently landing a partial or
// empty statement.
func TestRunDesignStart_TTYInterview_Aborted(t *testing.T) {
	repo := buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{
		Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{},
		IsTTY: true, Stdin: strings.NewReader("only the problem\n"),
	}

	var stdout, stderr bytes.Buffer
	got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "aborted-interview", manifest, phase7Model(t), deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runDesignStart(aborted interview) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(store.ActiveSpecDir(repo.Dir, "aborted-interview")); err == nil {
		t.Fatal("aborted interview left a spec.md on disk; want nothing written")
	}
}

// TestCmdDesignStart_StatementFlags_ParseAndRoundTrip proves --problem/
// --outcome/--defer-statements parse correctly through the REAL flag
// grammar (extractFlags) in every position, mirroring
// TestCmdDesignStart_NameFlagOrdering's own style.
func TestCmdDesignStart_StatementFlags_ParseAndRoundTrip(t *testing.T) {
	t.Run("--problem/--outcome round trip", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind", "feature", "--name", "flag-parsed",
			"--problem", "flagged problem", "--outcome", "flagged outcome"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
		spec, _ := readSpec(t, repo.Dir, "flag-parsed")
		if spec.Problem == nil || spec.Problem.Text != "flagged problem" {
			t.Fatalf("Problem = %+v, want the flagged text", spec.Problem)
		}
	})

	t.Run("--problem= form", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind=feature", "--name=flag-eq-parsed",
			"--problem=flagged problem", "--outcome=flagged outcome"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
	})

	t.Run("--defer-statements bare flag", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"jira:LOAN-1482", "--kind", "feature", "--name", "flag-deferred", "--defer-statements"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 0 {
			t.Fatalf("cmdDesignStart(%v) = %d, want 0; stderr=%s", args, got, stderr.String())
		}
	})

	t.Run("--problem given twice refuses", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		args := []string{"--kind", "feature", "--name", "x", "--problem", "a", "--problem", "b"}
		got := cmdDesignStart(args, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(--problem twice) = %d, want 2", got)
		}
	})
}

// TestDesignGo_NoOwnersFlag is spec/cli-creation ac-4's own negative pin:
// --owners deliberately stays out of design start's flag surface
// (I-10/X-4 ratified posture), disclosed rather than silently
// reconsidered now that this story adds --problem/--outcome/
// --defer-statements/--from-stub. Two proofs, deliberately distinct:
//
//  1. Behavioral — extractFlags' own switch recognizes no "--owners"
//     case, so cmdDesignStart given --owners on the command line treats
//     it as a stray POSITIONAL argument (never a value-carrying flag),
//     which the existing positional-count grammar then refuses.
//  2. Static, on the printed usage text ONLY (the two literal "usage:"
//     lines runDesignVerb/cmdDesignStart print) — never the whole file's
//     source, which legitimately documents the --owners absence in prose
//     (a doc comment mentioning the word is not the flag existing).
func TestDesignGo_NoOwnersFlag(t *testing.T) {
	t.Run("behavioral: --owners is a stray positional, not a flag", func(t *testing.T) {
		repo := buildPhase7Repo(t)
		t.Chdir(repo.Dir)
		var stdout, stderr bytes.Buffer
		// A feature takes at most one positional ref; "--owners" and
		// "alice" both land in rest (extractFlags recognizes neither),
		// giving two positionals — the existing "too many positionals"
		// refusal fires, proof --owners was never consumed as a flag.
		got := cmdDesignStart([]string{"--kind", "feature", "--name", "x", "--owners", "alice"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdDesignStart(--owners alice) = %d, want 2 (stray positional refusal)", got)
		}
	})

	t.Run("static: the printed usage text never mentions --owners", func(t *testing.T) {
		// Each call below is chosen to hit an actual "usage: ..." print
		// site (verified against design.go/designfromstub.go's source):
		// runDesignVerb's subcommand mismatch, cmdDesignStart's too-many-
		// positionals default arm, and cmdDesignStartFromStub's arg-count
		// guard.
		var stderr1 bytes.Buffer
		runDesignVerb([]string{"bogus"}, &bytes.Buffer{}, &stderr1)
		var stderr2 bytes.Buffer
		cmdDesignStart([]string{"--kind", "feature", "--name", "x", "one", "two"}, &bytes.Buffer{}, &stderr2)
		var stderr3 bytes.Buffer
		cmdDesignStartFromStub(nil, &bytes.Buffer{}, &stderr3)
		for i, s := range []string{stderr1.String(), stderr2.String(), stderr3.String()} {
			if !contains(s, "usage") {
				t.Fatalf("case #%d did not hit a usage print at all (test setup drifted from design.go's refusal shape): %q", i+1, s)
			}
			if strings.Contains(s, "owners") {
				t.Errorf("usage text #%d mentions owners: %q", i+1, s)
			}
		}
	})
}

// TestRun_DesignStart_TTYInterview_BuiltBinary is the real, built-binary
// proof of the flagless TTY interview end to end: VERDI_DESIGN_ASSUME_TTY=1
// stands in for the TTY predicate (this story's own disclosed stdin-script
// harness, mirroring init_test.go's VERDI_INIT_ASSUME_TTY), a scripted
// stdin answers the interview's two prompts, and the committed scaffold
// carries the real answers.
func TestRun_DesignStart_TTYInterview_BuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildPhase7Repo(t)

	cmd := exec.Command(bin, "design", "start", "jira:LOAN-1482", "--kind", "feature", "--name", "built-binary-interview")
	cmd.Dir = repo.Dir
	cmd.Env = append(os.Environ(), "VERDI_DESIGN_ASSUME_TTY=1")
	cmd.Stdin = strings.NewReader("built-binary interviewed problem\nbuilt-binary interviewed outcome\n")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("verdi design start (TTY interview): %v\nstdout:\n%s\nstderr:\n%s", err, outBuf.String(), errBuf.String())
	}

	spec, _ := readSpec(t, repo.Dir, "built-binary-interview")
	if spec.Problem == nil || spec.Problem.Text != "built-binary interviewed problem" {
		t.Fatalf("Problem = %+v, want the interviewed text", spec.Problem)
	}
	if spec.Outcome == nil || spec.Outcome.Text != "built-binary interviewed outcome" {
		t.Fatalf("Outcome = %+v, want the interviewed text", spec.Outcome)
	}
}

// TestRun_DesignStart_NoTTY_NoFlags_Refuses_BuiltBinary is the built-binary
// twin of the no-TTY refusal: without VERDI_DESIGN_ASSUME_TTY, a real
// subprocess genuinely has no attached terminal, and design start refuses
// by name rather than silently emitting the old TODO placeholders.
func TestRun_DesignStart_NoTTY_NoFlags_Refuses_BuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := buildPhase7Repo(t)

	cmd := exec.Command(bin, "design", "start", "jira:LOAN-1482", "--kind", "feature", "--name", "no-tty-no-flags")
	cmd.Dir = repo.Dir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	ee, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("verdi design start (no TTY, no flags): want an ExitError, got %v\nstdout:\n%s", err, outBuf.String())
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("exit code = %d, want 2\nstderr:\n%s", ee.ExitCode(), errBuf.String())
	}
	if !contains(errBuf.String(), "cannot interview") {
		t.Fatalf("stderr = %q, want the cannot-interview refusal", errBuf.String())
	}
}
