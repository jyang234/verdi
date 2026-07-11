package align

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeJudge writes a tiny shell script honoring S5's `claude -p
// --output-format json` envelope shape (or one of its failure modes) and
// returns its path. A real script executed via os/exec — not a mocked
// interface — so ExecJudgeRunner itself (the production exec path) is what
// gets exercised; only the far end (the "claude" binary) is fake, matching
// PLAN.md Phase 8's "fake judge binary in tests keeps the phase hermetic".
func writeFakeJudge(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("writing fake judge script: %v", err)
	}
	return path
}

const fakeJudgeOKScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-1\",\"text\":\"retry semantics match spec intent\",\"confidence\":0.87}]}"}
EOF
`

// fakeJudgeOKFencedScript wraps the result string's JSON in a markdown code
// fence — S5: "trim/strip fences defensively".
const fakeJudgeFencedScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"` + "```json\\n{\\\"findings\\\":[{\\\"id\\\":\\\"j-2\\\",\\\"text\\\":\\\"fenced ok\\\",\\\"confidence\\\":0.5}]}\\n```" + `"}
EOF
`

const fakeJudgeNonZeroExitScript = `echo "simulated crash" >&2
exit 3
`

const fakeJudgeGarbageStdoutScript = `echo "not json at all"
`

const fakeJudgeIsErrorScript = `cat <<'EOF'
{"is_error":true,"subtype":"error_during_execution","result":""}
EOF
`

const fakeJudgeInnerGarbageScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"this is not findings json"}
EOF
`

const fakeJudgeTimeoutScript = `sleep 5
echo "should never get here"
`

// judgeTestBudget bounds every runJudgeOnce call in this file that is NOT
// itself testing the timeout stage (TestRunJudgeOnce_Timeout, below, keeps
// its own short, deliberately tight timeout — that IS the behavior under
// test there).
//
// This is a fix for a proven flake, not a guess: a flat 1-second injected
// timeout on these calls was observed failing StageTimeout under
// full-module `go test -race` load three times across this build (always
// passing standalone). Reproduced on demand by running this package's
// tests concurrently with the rest of the module under -race (see the
// phase report for the captured run) — every failure showed
// Stage:timeout, Detail:"no result within 1s", never a wrong finding or a
// wrong OTHER stage. The fake judge scripts here do near-zero real work
// (a `cat <<EOF` heredoc or an `echo`); the time that blows through 1s
// under load is the OS scheduling this test's own os/exec fork+exec of
// /bin/sh, competing with the rest of the module's `-race`-instrumented
// goroutines for CPU — not the judge logic being slow. Sizing the budget
// as a `context.WithTimeout` deadline handed to runJudgeOnce as ctx (with
// the `timeout` parameter itself left at its zero-value default, so
// runJudgeOnce's OWN timeout-selection logic — "timeout<=0 uses
// DefaultJudgeTimeout" — stays exercised exactly as production calls it)
// is deadline-from-context, not a blind widening of the injected
// duration: the deadline these tests actually race against is expressed
// once, here, reasoned about explicitly, and stays two orders of
// magnitude below DefaultJudgeTimeout's 120s ceiling — a genuine
// timeout-logic regression (e.g. the real timeout silently not firing at
// all) still fails this test suite within judgeTestBudget, not by hanging
// for the full 120s or forever.
const judgeTestBudget = 10 * time.Second

// judgeTestContext returns a context bounded by judgeTestBudget, cleaned
// up automatically at the end of the calling test.
func judgeTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), judgeTestBudget)
	t.Cleanup(cancel)
	return ctx
}

func TestRunJudgeOnce_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeOKScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt bytes"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", success.Findings)
	}
	f := success.Findings[0]
	if f.ID != "judged-j-1" || f.Kind != "judged" {
		t.Fatalf("finding = %+v, want id judged-j-1 kind judged", f)
	}
	if !strings.Contains(f.Text, "retry semantics match spec intent") || !strings.Contains(f.Text, "0.87") {
		t.Fatalf("finding text = %q, want it to carry the judge's text and confidence", f.Text)
	}
	if string(success.Stdin) != "prompt bytes" {
		t.Fatalf("Stdin = %q, want the exact prompt bytes", success.Stdin)
	}
	if !strings.Contains(success.RawResult, "j-1") {
		t.Fatalf("RawResult = %q, want the raw result string preserved", success.RawResult)
	}
}

func TestRunJudgeOnce_FencedResult(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeFencedScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("p"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 || success.Findings[0].ID != "judged-j-2" {
		t.Fatalf("Findings = %+v, want one judged-j-2", success.Findings)
	}
}

func TestRunJudgeOnce_FailureModes(t *testing.T) {
	cases := []struct {
		name   string
		script string
		stage  JudgeFailureStage
	}{
		{"missing binary", "", StageExec}, // handled specially below
		{"non-zero exit", fakeJudgeNonZeroExitScript, StageExit},
		{"garbage stdout (outer-parse)", fakeJudgeGarbageStdoutScript, StageOuterParse},
		{"is_error true (outer-parse)", fakeJudgeIsErrorScript, StageOuterParse},
		{"inner garbage (inner-parse)", fakeJudgeInnerGarbageScript, StageInnerParse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var argv []string
			if tc.name == "missing binary" {
				argv = []string{filepath.Join(t.TempDir(), "does-not-exist")}
			} else {
				argv = []string{writeFakeJudge(t, tc.script)}
			}
			success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, argv, 0, []byte("p"))
			if success != nil {
				t.Fatalf("runJudgeOnce(%s): unexpected success %+v", tc.name, success)
			}
			if failure == nil {
				t.Fatalf("runJudgeOnce(%s): want a JudgeFailure, got nil", tc.name)
			}
			if failure.Stage != tc.stage {
				t.Fatalf("runJudgeOnce(%s): Stage = %q, want %q (failure: %+v)", tc.name, failure.Stage, tc.stage, failure)
			}
			if failure.CmdTemplate == "" {
				t.Fatalf("runJudgeOnce(%s): CmdTemplate must be recorded", tc.name)
			}
		})
	}

	t.Run("non-zero exit records exit code and stderr", func(t *testing.T) {
		script := writeFakeJudge(t, fakeJudgeNonZeroExitScript)
		_, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("p"))
		if failure.ExitCode != 3 {
			t.Fatalf("ExitCode = %d, want 3", failure.ExitCode)
		}
		if !strings.Contains(failure.StderrSnippet, "simulated crash") {
			t.Fatalf("StderrSnippet = %q, want it to carry the child's stderr", failure.StderrSnippet)
		}
	})
}

// TestRunJudgeOnce_Timeout exercises the timeout stage with a short
// injected Timeout, never S5's real ~120s ceiling. Unlike every other test
// in this file, this one is DELIBERATELY tight (100ms) — it is the
// regression detector for the timeout mechanism itself, so it must stay
// sensitive rather than getting the judgeTestBudget treatment above. It is
// not part of the proven flake (judgeTestBudget's doc comment) and does
// not need to be: fakeJudgeTimeoutScript always sleeps 5s regardless of
// scheduling load, an interval no plausible fork/exec scheduling jitter
// approaches, so a 100ms timeout fires deterministically either way.
func TestRunJudgeOnce_Timeout(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeTimeoutScript)
	start := time.Now()
	success, failure := runJudgeOnce(context.Background(), ExecJudgeRunner{}, []string{script}, 100*time.Millisecond, []byte("p"))
	elapsed := time.Since(start)
	if success != nil {
		t.Fatalf("unexpected success %+v", success)
	}
	if failure == nil || failure.Stage != StageTimeout {
		t.Fatalf("failure = %+v, want stage timeout", failure)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("runJudgeOnce took %s, want it to return promptly after the injected 100ms timeout, not wait for the sleep 5", elapsed)
	}
}
