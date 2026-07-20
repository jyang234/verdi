package align

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunJudged_NotConfigured(t *testing.T) {
	t.Run("not required: absence finding", func(t *testing.T) {
		result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
		if err != nil {
			t.Fatalf("RunJudged: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
			t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
		}
		if result.Findings[0].Dispositioned() {
			t.Fatalf("absence finding must be undispositioned fresh")
		}
		if result.Integrity != "" || result.JudgeIntegrity != nil {
			t.Fatalf("not-configured path must carry no Integrity/JudgeIntegrity, got %+v", result)
		}
	})

	t.Run("required: align fails outright", func(t *testing.T) {
		_, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeRequired: true})
		if err == nil {
			t.Fatal("RunJudged(judge_required=true, no judge_cmd): want error, got nil")
		}
	})
}

func TestRunJudged_ExecutionFailure(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeNonZeroExitScript)

	t.Run("not required: absence finding names the failure", func(t *testing.T) {
		result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, Timeout: time.Second})
		if err != nil {
			t.Fatalf("RunJudged: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
			t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
		}
		if !strings.Contains(result.Findings[0].Text, "exit=3") {
			t.Fatalf("absence finding text = %q, want it to name the exit code", result.Findings[0].Text)
		}
	})

	t.Run("required: align fails outright", func(t *testing.T) {
		_, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, JudgeRequired: true, Timeout: time.Second})
		if err == nil {
			t.Fatal("RunJudged(judge_required=true, failing judge): want error, got nil")
		}
	})
}

func TestRunJudged_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeOKScript)
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, Timeout: time.Second, Prompt: []byte("prompt")})
	if err != nil {
		t.Fatalf("RunJudged: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "judged-j-1" {
		t.Fatalf("Findings = %+v, want one judged-j-1", result.Findings)
	}
	if result.Integrity == "" {
		t.Fatal("Integrity must be set on a successful judge exchange")
	}
	if result.JudgeIntegrity == nil || result.JudgeIntegrity.RawResult == "" || result.JudgeIntegrity.StdinB64 == "" {
		t.Fatalf("JudgeIntegrity = %+v, want both fields populated", result.JudgeIntegrity)
	}
	stdin, err := base64.StdEncoding.DecodeString(result.JudgeIntegrity.StdinB64)
	if err != nil {
		t.Fatalf("decoding StdinB64: %v", err)
	}
	if got := computeIntegrity(stdin, result.JudgeIntegrity.RawResult); got != result.Integrity {
		t.Fatalf("recomputed integrity %q != stored %q", got, result.Integrity)
	}
}

// TestRunJudged_WaitExpires proves spec/judge-ergonomics ac-2's expiry half
// at the engine layer: JudgedInput.Wait true plus a judge that does not
// complete within Timeout returns *ErrJudgeWaitExpired — never a nil error
// with a synthetic absence finding (today's non-wait degrade) and never
// *ErrJudgeRequiredAbsent — so cmd/verdi/align.go can map it to exit 2 ("an
// operational timeout, not a verdict", ac-2's own words) regardless of
// JudgeRequired.
func TestRunJudged_WaitExpires(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeTimeoutScript) // sleeps 5s
	start := time.Now()
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{
		JudgeCmd: []string{script}, Timeout: 100 * time.Millisecond, Wait: true,
	})
	elapsed := time.Since(start)

	if result != nil {
		t.Fatalf("RunJudged(Wait, expired) result = %+v, want nil — nothing to write on expiry", result)
	}
	var waitExpired *ErrJudgeWaitExpired
	if !errors.As(err, &waitExpired) {
		t.Fatalf("RunJudged(Wait, expired) error = %v (%T), want *ErrJudgeWaitExpired", err, err)
	}
	if waitExpired.Failure == nil || waitExpired.Failure.Stage != StageTimeout {
		t.Fatalf("ErrJudgeWaitExpired.Failure = %+v, want Stage=%s", waitExpired.Failure, StageTimeout)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("RunJudged took %s, want it to return promptly after the 100ms Timeout, not wait for the sleep 5", elapsed)
	}
}

// TestRunJudged_WaitCompletes proves ac-2's completing half: a judge that
// finishes within Timeout behaves identically whether Wait is set or not —
// real findings, no error, Integrity populated.
func TestRunJudged_WaitCompletes(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeOKScript)
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{
		JudgeCmd: []string{script}, Timeout: time.Second, Wait: true, Prompt: []byte("prompt"),
	})
	if err != nil {
		t.Fatalf("RunJudged(Wait, completing): %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "judged-j-1" {
		t.Fatalf("Findings = %+v, want one judged-j-1", result.Findings)
	}
	if result.Integrity == "" || result.JudgeIntegrity == nil {
		t.Fatalf("result = %+v, want Integrity/JudgeIntegrity populated on a genuine completion", result)
	}
}

// TestRunJudged_WaitFalse_TimeoutStillDegrades is the regression pin for
// today's DEFAULT (Wait's zero value, false): a timeout must still degrade
// gracefully to the synthetic absence finding exactly as before this story
// — ac-2's bounded-exit-2 behavior is opt-in, never a change to the
// unflagged default.
func TestRunJudged_WaitFalse_TimeoutStillDegrades(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeTimeoutScript)
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{
		JudgeCmd: []string{script}, Timeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("RunJudged(Wait=false, timeout): %v, want nil (graceful degrade unchanged)", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
		t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
	}
}

// TestRunJudged_WaitTrue_NonTimeoutFailureStillDegrades proves Wait's new
// exit path is scoped to StageTimeout only — ac-2's "does not complete
// within the bound" — not to every judge failure. A judge that exits
// immediately (non-zero exit) has completed, just unsuccessfully; that is
// today's ordinary absent-judge case, unaffected by Wait.
func TestRunJudged_WaitTrue_NonTimeoutFailureStillDegrades(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeNonZeroExitScript)
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{
		JudgeCmd: []string{script}, Timeout: time.Second, Wait: true,
	})
	if err != nil {
		t.Fatalf("RunJudged(Wait=true, non-timeout failure): %v, want nil (still a graceful degrade)", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
		t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
	}
}

// TestRunJudged_WaitTrue_JudgeRequiredTrue_TimeoutIsWaitExpiredNotRequiredAbsent
// pins the precedence between Wait and JudgeRequired on a timeout: ac-2 says
// expiry is "exit 2 — not 1, since this is an operational timeout, not a
// verdict" unconditionally, so ErrJudgeWaitExpired must win even when
// JudgeRequired is also true — never ErrJudgeRequiredAbsent (which
// cmd/verdi/align.go maps to exit 1).
func TestRunJudged_WaitTrue_JudgeRequiredTrue_TimeoutIsWaitExpiredNotRequiredAbsent(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeTimeoutScript)
	_, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{
		JudgeCmd: []string{script}, Timeout: 100 * time.Millisecond, Wait: true, JudgeRequired: true,
	})
	var waitExpired *ErrJudgeWaitExpired
	if !errors.As(err, &waitExpired) {
		t.Fatalf("RunJudged(Wait, JudgeRequired, expired) error = %v (%T), want *ErrJudgeWaitExpired even with JudgeRequired=true", err, err)
	}
	var reqAbsent *ErrJudgeRequiredAbsent
	if errors.As(err, &reqAbsent) {
		t.Fatal("error must not ALSO satisfy errors.As for *ErrJudgeRequiredAbsent — that would wrongly let cmd/verdi/align.go pick exit 1 over ac-2's exit 2")
	}
}

func TestRunJudged_DeterministicAbsenceText(t *testing.T) {
	// Same not-configured input twice -> byte-identical absence finding
	// text, the property identity.go's disposition preservation across
	// regeneration depends on for the no-judge path.
	r1, _ := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
	r2, _ := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
	if r1.Findings[0].Text != r2.Findings[0].Text {
		t.Fatalf("absence text not deterministic: %q vs %q", r1.Findings[0].Text, r2.Findings[0].Text)
	}
}
