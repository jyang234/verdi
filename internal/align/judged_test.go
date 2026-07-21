package align

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// guide-claim: 11-configurable-alignment-judge
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

// TestArchivedDoubledJudgedID_DecodesAndRoundTripsUntouched is spec/ritual-
// traps ac-2's prospective-only guarantee: a fixture standing in for an
// already-archived deviation-report.md whose finding carries the OLD
// doubled "judged-judged-..." id form (the exact shape judgedFindingID's
// pre-fix minting bug at judge.go/decision_judge.go/diagram_judge.go could
// produce on a regeneration path that fed a prior finding's own id back
// through the judge) must still strict-decode through
// internal/artifact.DecodeDeviation and carry every field — the doubled id
// above all — through byte-for-byte unchanged. The fix here only ever
// touches an id at MINT time (judgedFindingID, called from the three
// judge-exec call sites); nothing on the decode path inspects or
// normalizes Finding.ID at all, so a real archived disposition that already
// references the doubled id exactly as originally minted is never silently
// renumbered on read.
//
// The test carries BOTH directions (finding judged-ac2-roundtrip-pin-has-no-
// encode-leg): the decode/stability legs alone compare two decodes of the
// same constant bytes and are structurally blind to the write direction —
// they would stay green even if the report renderer mangled the id on the way
// back to disk. The encode leg below re-encodes the decoded report through
// align's own report renderer (RenderMarkdown, the exact seam that writes
// deviation-report.md) and proves the doubled id, and every rendered byte of
// its finding, survive back onto disk untouched — which is what "round-trips"
// means and what actually protects an archived report the tooling later
// rewrites in place.
func TestArchivedDoubledJudgedID_DecodesAndRoundTripsUntouched(t *testing.T) {
	const doubledID = "judged-judged-retry-semantics-drift"
	const archived = `---
schema: verdi.deviation/v1
covers: 0123456789abcdef0123456789abcdef01234567
findings:
  - id: judged-judged-retry-semantics-drift
    kind: judged
    text: "retry semantics match spec intent (confidence 0.87)"
    disposition: accepted-deviation
    note: "pre-existing behavior, dispositioned before the id-doubling fix landed"
digest: sha256:1111111111111111111111111111111111111111111111111111111111111111
---
# Deviation report

Archived body text, preserved verbatim.
`
	fm, body, err := artifact.SplitFrontmatter([]byte(archived))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		t.Fatalf("DecodeDeviation(archived doubled-id fixture): %v", err)
	}
	if len(decoded.Findings) != 1 {
		t.Fatalf("Findings = %+v, want exactly 1", decoded.Findings)
	}
	f := decoded.Findings[0]
	if f.ID != doubledID {
		t.Fatalf("Findings[0].ID = %q, want the archived doubled id %q preserved exactly untouched", f.ID, doubledID)
	}
	if f.Kind != artifact.FindingJudged {
		t.Fatalf("Findings[0].Kind = %q, want %q", f.Kind, artifact.FindingJudged)
	}
	if f.Disposition != artifact.FindingAcceptedDeviation || f.Note == "" {
		t.Fatalf("Findings[0] disposition/note not preserved: %+v", f)
	}

	// Round-trip: decoding the exact same archived bytes again is stable —
	// decode is a pure function of its input; it never mutates or
	// renumbers an id it reads, doubled or not.
	fm2, body2, err := artifact.SplitFrontmatter([]byte(archived))
	if err != nil {
		t.Fatalf("SplitFrontmatter (second pass): %v", err)
	}
	if string(fm) != string(fm2) || string(body) != string(body2) {
		t.Fatal("SplitFrontmatter is not stable across repeated calls on the same archived bytes")
	}
	decoded2, err := artifact.DecodeDeviation(fm2)
	if err != nil {
		t.Fatalf("DecodeDeviation (second pass): %v", err)
	}
	if decoded2.Findings[0].ID != doubledID {
		t.Fatalf("second decode's id = %q, want the same doubled id %q — decode must not renumber on repeat reads", decoded2.Findings[0].ID, doubledID)
	}

	// --- Encode leg: the actual write-back round-trip the pin's name promises.
	// RenderMarkdown is align's report renderer — the exact seam that serializes
	// a DeviationFrontmatter back onto disk as deviation-report.md.
	reencoded := RenderMarkdown(decoded, string(body))
	if !bytes.Contains(reencoded, []byte(doubledID)) {
		t.Fatalf("re-encoded report does not contain the doubled id %q verbatim — the write path mangled it:\n%s", doubledID, reencoded)
	}

	fm3, body3, err := artifact.SplitFrontmatter(reencoded)
	if err != nil {
		t.Fatalf("SplitFrontmatter(re-encoded): %v", err)
	}
	decoded3, err := artifact.DecodeDeviation(fm3)
	if err != nil {
		t.Fatalf("DecodeDeviation(re-encoded): %v", err)
	}
	if len(decoded3.Findings) != 1 {
		t.Fatalf("re-encoded Findings = %+v, want exactly 1", decoded3.Findings)
	}
	rt := decoded3.Findings[0]
	if rt.ID != doubledID {
		t.Fatalf("round-tripped id = %q, want the doubled id %q to survive re-encoding onto disk untouched", rt.ID, doubledID)
	}
	if rt.Kind != f.Kind || rt.Text != f.Text || rt.Disposition != f.Disposition || rt.Note != f.Note {
		t.Fatalf("round-tripped finding = %+v, want every field preserved through the write path (was %+v)", rt, f)
	}

	// Fixed point: re-encoding the re-decoded report reproduces the exact same
	// bytes — the doubled id (and the whole finding) is stable across the write
	// path, not merely present once, which is what protects a report the tooling
	// rewrites in place across regenerations.
	if reencoded2 := RenderMarkdown(decoded3, string(body3)); !bytes.Equal(reencoded, reencoded2) {
		t.Fatalf("re-encode is not a fixed point — the renderer does not reproduce its own output byte-for-byte\nfirst:\n%s\nsecond:\n%s", reencoded, reencoded2)
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
