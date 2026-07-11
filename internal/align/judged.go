package align

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/OWNER/verdi/internal/artifact"
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

// absentResult applies I-9's judge_required gate: required=true fails
// align outright (a non-nil error); required=false (v0's default) degrades
// to the synthetic absence finding, which — like any finding — must be
// dispositioned before the merge gate passes (03 §Alignment report:
// "skipping the judge is never free, always visible to the reviewer, and
// countable in audit").
func absentResult(required bool, failure *JudgeFailure) (*JudgedResult, error) {
	if required {
		return nil, fmt.Errorf("align: align.judge_required is true but no judge produced a judged section (stage=%s: %s)", failure.Stage, failure.Detail)
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
