package align

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// DefaultJudgeTimeout is S5's observed-safe ceiling: "Timeout ~120s via exec
// context (observed calls 16-24s)". Tests inject a far shorter Timeout to
// exercise the timeout stage without actually waiting two minutes.
const DefaultJudgeTimeout = 120 * time.Second

// JudgeExecResult is one judge invocation's raw outcome — the exec-level
// analogue of upstream.Result, kept as its own type since the judge is a
// foreign command (S5), not one of internal/upstream's pinned CLIs.
type JudgeExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// ErrJudgeTimeout is JudgeRunner's sentinel for a context-deadline kill,
// distinct from every other exec-level failure (missing binary, ...).
var ErrJudgeTimeout = errors.New("align: judge command timed out")

// JudgeRunner execs one judge invocation per spike S5's binding design:
// argv[0] is the binary, argv[1:] its arguments (an array — never a shell
// string, see internal/store's AlignConfig doc), and the prompt goes to the
// child's stdin ONLY (S5: "argv risks E2BIG + shell-escaping; a ~100KB
// stdin prompt round-tripped cleanly"). A non-nil error is only ever an
// exec-level failure (binary not found) or ErrJudgeTimeout; a clean exec
// with a non-zero exit is reported via JudgeExecResult.ExitCode, mirroring
// internal/upstream.Runner's own convention.
type JudgeRunner interface {
	RunJudge(ctx context.Context, argv []string, stdin []byte) (JudgeExecResult, error)
}

// ExecJudgeRunner execs the real judge command via os/exec. Never exercised
// by this module's own tests (CLAUDE.md: "no network in any test" — the
// real `claude -p` path needs live auth); spike S5 proved it manually.
// Tests use a fake judge binary (a tiny script) with ExecJudgeRunner itself
// — ExecJudgeRunner has no dependency on the real `claude` binary, only on
// os/exec, so it is exercised against fakes hermetically.
type ExecJudgeRunner struct{}

// RunJudge implements JudgeRunner.
func (ExecJudgeRunner) RunJudge(ctx context.Context, argv []string, stdin []byte) (JudgeExecResult, error) {
	if len(argv) == 0 {
		return JudgeExecResult{}, fmt.Errorf("align: RunJudge: argv must not be empty")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// When ctx expires, CommandContext SIGKILLs the judge, but cmd.Run() can
	// still block until the process's grandchildren (e.g. a shell's `sleep`)
	// release the inherited stdout/stderr pipes — the classic WaitDelay gotcha,
	// env-dependent (prompt locally, up to the child's lifetime in CI). Bound
	// it so a timed-out judge returns promptly.
	cmd.WaitDelay = 2 * time.Second
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	res := JudgeExecResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}

	if runErr == nil {
		res.ExitCode = 0
		return res, nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return res, ErrJudgeTimeout
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		return res, nil
	}
	return res, fmt.Errorf("align: exec %s: %w", argv[0], runErr)
}

// JudgeFailureStage names which of S5's five stages a judge invocation
// failed at, plus align's own sixth, prior stage for a wholly unconfigured
// judge (align.judge_cmd absent — see judged.go).
type JudgeFailureStage string

const (
	StageNotConfigured JudgeFailureStage = "not-configured"
	StageExec          JudgeFailureStage = "exec"
	StageTimeout       JudgeFailureStage = "timeout"
	StageExit          JudgeFailureStage = "exit"
	StageOuterParse    JudgeFailureStage = "outer-parse"
	StageInnerParse    JudgeFailureStage = "inner-parse"
)

// stderrSnippetLimit matches S5's observed absence-finding shape ("~500B
// stderr snippet").
const stderrSnippetLimit = 500

// JudgeFailure is a judge invocation's disclosed failure, the synthetic
// "judged coverage absent" finding's source material (S5: "records failure
// stage ..., exit code, ~500B stderr snippet, and the attempted cmd
// template").
type JudgeFailure struct {
	Stage         JudgeFailureStage
	ExitCode      int
	StderrSnippet string
	CmdTemplate   string
	Detail        string
}

// judgeOuterEnvelope is `claude -p --output-format json`'s outer shape
// (S5): a foreign tool's own output contract, decoded loosely (unknown
// fields ignored) since verdi does not own this schema — only is_error,
// subtype, and result are read.
type judgeOuterEnvelope struct {
	IsError bool   `json:"is_error"`
	Subtype string `json:"subtype"`
	Result  string `json:"result"`
}

// judgeInnerResult is the judge's own findings contract (S5: "strict-decode
// {\"findings\":[{id,text,confidence}]}") — a shape verdi DOES own (it is
// what the prompt asks the judge to emit), so this layer decodes strictly.
type judgeInnerResult struct {
	Findings []judgeInnerFinding `json:"findings"`
}

type judgeInnerFinding struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
}

// JudgeSuccess is one successful judge exchange's parsed content plus the
// exact bytes an integrity hash needs (S5: "hash of the exact stdin bytes +
// the raw result string").
type JudgeSuccess struct {
	Findings  []artifact.Finding
	Stdin     []byte
	RawResult string
}

// runJudgeOnce execs argv once with prompt on stdin (S5) and two-layer
// parses the result: outer envelope -> trim/strip fences defensively ->
// strict inner decode. Returns exactly one of (*JudgeSuccess, nil) or
// (nil, *JudgeFailure) — never both, and never a bare Go error, since every
// failure mode here is disclosed content for the synthetic absence finding,
// not an operational error align propagates.
func runJudgeOnce(ctx context.Context, runner JudgeRunner, argv []string, timeout time.Duration, prompt []byte) (*JudgeSuccess, *JudgeFailure) {
	rawResult, err := execJudgeEnvelope(ctx, runner, argv, timeout, prompt)
	if err != nil {
		return nil, err
	}

	inner, decErr := decodeInnerResult(rawResult)
	if decErr != nil {
		return nil, &JudgeFailure{
			Stage:       StageInnerParse,
			CmdTemplate: strings.Join(argv, " "),
			Detail:      fmt.Sprintf("decoding inner findings JSON: %v", decErr),
		}
	}

	findings := make([]artifact.Finding, 0, len(inner.Findings))
	for _, jf := range inner.Findings {
		findings = append(findings, artifact.Finding{
			ID:   "judged-" + store.RefSlug(jf.ID),
			Kind: artifact.FindingJudged,
			Text: fmt.Sprintf("%s (confidence %.2f)", jf.Text, jf.Confidence),
		})
	}
	return &JudgeSuccess{Findings: findings, Stdin: prompt, RawResult: rawResult}, nil
}

// execJudgeEnvelope execs argv once with prompt on stdin and runs S5's
// exec/timeout/exit/outer-envelope stages only (StageExec, StageTimeout,
// StageExit, StageOuterParse) — the layer shared by every judge-consuming
// mode this package has (build-branch deviation findings, decision_judge.go's
// design-branch sweep): a foreign command is spawned, timed out, and its
// `claude -p --output-format json`-shaped outer envelope decoded the same
// way regardless of what the judge was ASKED to produce. S5's final stage,
// inner-JSON parsing, is mode-specific (a different findings shape per
// mode) and is deliberately left to each caller — see runJudgeOnce and
// decision_judge.go's own inner-parse callers.
//
// A *JudgeFailure's ExitCode/StderrSnippet are populated whenever a
// response (even a non-zero exit) was actually received; the CmdTemplate on
// a StageOuterParse failure is filled in by the caller since this function
// does not itself retain argv past the exec call — callers that need it in
// the returned failure re-join argv themselves (both current callers do).
func execJudgeEnvelope(ctx context.Context, runner JudgeRunner, argv []string, timeout time.Duration, prompt []byte) (string, *JudgeFailure) {
	if timeout <= 0 {
		timeout = DefaultJudgeTimeout
	}
	cmdTemplate := strings.Join(argv, " ")

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := runner.RunJudge(runCtx, argv, prompt)
	if err != nil {
		if errors.Is(err, ErrJudgeTimeout) {
			return "", &JudgeFailure{Stage: StageTimeout, CmdTemplate: cmdTemplate, Detail: fmt.Sprintf("no result within %s", timeout)}
		}
		return "", &JudgeFailure{Stage: StageExec, CmdTemplate: cmdTemplate, Detail: err.Error()}
	}
	if res.ExitCode != 0 {
		return "", &JudgeFailure{
			Stage:         StageExit,
			ExitCode:      res.ExitCode,
			StderrSnippet: snippet(res.Stderr),
			CmdTemplate:   cmdTemplate,
			Detail:        fmt.Sprintf("judge command exited %d", res.ExitCode),
		}
	}

	var outer judgeOuterEnvelope
	if err := json.Unmarshal(res.Stdout, &outer); err != nil {
		return "", &JudgeFailure{
			Stage:         StageOuterParse,
			ExitCode:      res.ExitCode,
			StderrSnippet: snippet(res.Stderr),
			CmdTemplate:   cmdTemplate,
			Detail:        fmt.Sprintf("decoding outer envelope: %v", err),
		}
	}
	if outer.IsError {
		return "", &JudgeFailure{
			Stage:         StageOuterParse,
			ExitCode:      res.ExitCode,
			StderrSnippet: snippet(res.Stderr),
			CmdTemplate:   cmdTemplate,
			Detail:        fmt.Sprintf("judge reported is_error=true (subtype %q)", outer.Subtype),
		}
	}
	return outer.Result, nil
}

// decodeInnerResult trims whitespace and strips a defensive markdown code
// fence (S5: "trim/strip fences defensively") before strict-decoding the
// findings JSON verdi's own prompt asked the judge to emit.
func decodeInnerResult(raw string) (*judgeInnerResult, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var inner judgeInnerResult
	if err := artifact.DecodeStrictJSON([]byte(s), &inner); err != nil {
		return nil, err
	}
	return &inner, nil
}

func snippet(b []byte) string {
	if len(b) > stderrSnippetLimit {
		b = b[:stderrSnippetLimit]
	}
	return string(b)
}
