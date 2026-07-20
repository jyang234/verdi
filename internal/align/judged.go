package align

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// AbsenceFindingID is the synthetic "judged coverage absent" finding's
// stable id (I-9; PLAN.md Phase 8). It never changes across regenerations
// with the SAME failure content — see identity.go's content-hash identity
// rule for why a materially different failure (a different stage, exit
// code, or stderr) still resets its disposition.
const AbsenceFindingID = "judged-coverage-absent"

// JudgedInput is RunJudged's input.
type JudgedInput struct {
	// JudgeCmd is verdi.yaml's align.judge_cmd argv array. Empty/nil means
	// no judge is configured at all (the common v0 default).
	JudgeCmd []string
	// JudgeRequired is verdi.yaml's align.judge_required (I-9).
	JudgeRequired bool
	// Timeout overrides DefaultJudgeTimeout (tests only; zero uses the
	// default).
	Timeout time.Duration
	// Prompt is the exact stdin bytes to send the judge, already rendered
	// from pinned inputs by the caller (report.go) — this package does not
	// decide prompt content, only the exec/parse/absence contract.
	Prompt []byte
	// Wait requests spec/judge-ergonomics ac-2's bounded-wait contract: a
	// judge invocation that does not complete within Timeout (StageTimeout)
	// returns *ErrJudgeWaitExpired instead of degrading to the synthetic
	// absence finding — an operational timeout the caller (cmd/verdi/
	// align.go, via --wait) maps to exit 2, never a silent hang and never
	// exit 0 with a placeholder finding standing in for a judge that simply
	// never got to run. False (the zero value) is today's unchanged
	// default: every failure stage, timeout included, degrades gracefully.
	// Scoped to StageTimeout only — a judge that exits fast (even
	// unsuccessfully) has completed, which is the ordinary absent-judge case
	// ac-2 leaves untouched.
	Wait bool
}

// JudgedResult is RunJudged's output: either the judge's real findings (a
// non-empty Integrity plus the persisted exchange for self-verification),
// or exactly one synthetic absence finding (no Integrity at all — see
// internal/artifact's JudgeIntegrity doc comment on why).
type JudgedResult struct {
	Findings       []artifact.Finding
	Integrity      string
	JudgeIntegrity *artifact.JudgeIntegrity
}

// RunJudged produces the judged section per I-9 / spike S5. It returns a
// non-nil error ONLY when JudgeRequired is true and no working judged
// section could be produced (align must exit non-zero in that case,
// PLAN.md Phase 8's exit criteria) — every other failure mode (not
// configured, or any of S5's five exec/parse stages) degrades to the
// synthetic absence finding instead of an error.
func RunJudged(ctx context.Context, runner JudgeRunner, in JudgedInput) (*JudgedResult, error) {
	if len(in.JudgeCmd) == 0 {
		return absentResult(in.JudgeRequired, &JudgeFailure{
			Stage:  StageNotConfigured,
			Detail: "no align.judge_cmd configured in verdi.yaml (align: { judge_cmd: [...] })",
		})
	}

	success, failure := runJudgeOnce(ctx, runner, in.JudgeCmd, in.Timeout, in.Prompt)
	if failure != nil {
		// spec/judge-ergonomics ac-2: under Wait, a timeout is reported as an
		// operational expiry, unconditionally — even when JudgeRequired is
		// also true ("exit 2 — not 1, since this is an operational timeout,
		// not a verdict"). Checked BEFORE absentResult's JudgeRequired branch
		// so it always wins over ErrJudgeRequiredAbsent on this one failure
		// stage; every other stage (not configured, exec, exit, parse) is
		// unaffected and keeps its existing absentResult handling regardless
		// of Wait.
		if in.Wait && failure.Stage == StageTimeout {
			return nil, &ErrJudgeWaitExpired{Failure: failure}
		}
		return absentResult(in.JudgeRequired, failure)
	}

	return &JudgedResult{
		Findings:  success.Findings,
		Integrity: computeIntegrity(success.Stdin, success.RawResult),
		JudgeIntegrity: &artifact.JudgeIntegrity{
			StdinB64:  base64.StdEncoding.EncodeToString(success.Stdin),
			RawResult: success.RawResult,
		},
	}, nil
}

// ErrJudgeRequiredAbsent is RunJudged's (and Generate's) error when
// align.judge_required is true and no judge produced a judged section.
// cmd/verdi/align.go type-switches on it (errors.As) to choose exit 1
// ("judge-required-and-absent", PLAN.md Phase 8's exit criteria) rather
// than exit 2's generic operational failure.
type ErrJudgeRequiredAbsent struct{ Failure *JudgeFailure }

func (e *ErrJudgeRequiredAbsent) Error() string {
	return fmt.Sprintf("align: align.judge_required is true but no judge produced a judged section (stage=%s: %s)", e.Failure.Stage, e.Failure.Detail)
}

// ErrJudgeWaitExpired is RunJudged's (and Generate's) error when Wait is
// true and the judge did not complete within Timeout (spec/judge-ergonomics
// ac-2). cmd/verdi/align.go type-switches on it (errors.As) to choose exit 2
// — an operational timeout, never verdict exit 1 (ErrJudgeRequiredAbsent's
// code) and never exit 0 (the non-Wait default's graceful degrade). Unlike
// ErrJudgeRequiredAbsent, this is not conditioned on JudgeRequired at all:
// it fires whenever Wait asked for a bounded wait and the bound elapsed,
// regardless of whether the judge was also required.
type ErrJudgeWaitExpired struct{ Failure *JudgeFailure }

func (e *ErrJudgeWaitExpired) Error() string {
	return fmt.Sprintf("align: --wait expired before the judge completed (stage=%s: %s)", e.Failure.Stage, e.Failure.Detail)
}

// absentResult applies I-9's judge_required gate: required=true fails
// align outright (a non-nil error); required=false (v0's default) degrades
// to the synthetic absence finding, which — like any finding — must be
// dispositioned before the merge gate passes (03 §Alignment report:
// "skipping the judge is never free, always visible to the reviewer, and
// countable in audit").
func absentResult(required bool, failure *JudgeFailure) (*JudgedResult, error) {
	if required {
		return nil, &ErrJudgeRequiredAbsent{Failure: failure}
	}
	return &JudgedResult{Findings: []artifact.Finding{absenceFinding(failure)}}, nil
}

// absenceFinding renders JudgeFailure into the synthetic "judged coverage
// absent" finding's text, deterministically: identical failure content
// (same stage/exit/cmd/stderr) renders identical text, which is what lets
// identity.go's content-hash identity preserve a human's disposition
// across an unchanged repeat failure.
func absenceFinding(f *JudgeFailure) artifact.Finding {
	text := fmt.Sprintf("judged coverage absent: %s", f.Detail)
	if f.Stage != StageNotConfigured {
		text += fmt.Sprintf(" (stage=%s, exit=%d, cmd=%q", f.Stage, f.ExitCode, f.CmdTemplate)
		if f.StderrSnippet != "" {
			text += fmt.Sprintf(", stderr=%q", f.StderrSnippet)
		}
		text += ")"
	}
	return artifact.Finding{ID: AbsenceFindingID, Kind: artifact.FindingJudged, Text: text}
}

// computeIntegrity implements spike S5's binding formula: "hash of the
// exact stdin bytes + the raw result string". A NUL separator between the
// two is a disclosed strengthening beyond S5's literal wording (plain
// concatenation is ambiguous — some stdin/result byte pairs could collide
// across a different split), not a silent invention: it changes no
// observable behavior S5 specified (still one sha256 over "the stdin bytes
// and the raw result string", just unambiguously delimited).
func computeIntegrity(stdin []byte, rawResult string) string {
	h := sha256.New()
	h.Write(stdin)
	h.Write([]byte{0})
	h.Write([]byte(rawResult))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
