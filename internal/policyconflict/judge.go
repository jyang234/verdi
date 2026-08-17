// judge.go is the pure judge process transport (authority design §6, ledger
// SI-96): one primary or challenger invocation, given the exact canonical
// prompt and normalized-input bytes independently, producing a
// self-consistent JudgmentExchange or a typed operational error. It never
// caches (that is cache.go's own, separately lock-scoped responsibility —
// authority design §7: "The evaluator is pure; the CLI process adapter owns
// immutable machine records") and it never cross-checks a result's finding
// witnesses against a typed SemanticInput (that is ValidateJudgeResult's
// job, in semantic.go, which needs the typed input this two-[]byte-argument
// port cannot carry). It reuses `align.judge_cmd` solely as configured argv
// transport — JudgeAdapter never imports internal/align or reuses its
// prompts, finding IDs, or wrappers.
package policyconflict

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
)

// ErrJudgeOperational is the one sentinel every judge transport/validation
// failure wraps (authority design §6: "typed operational error"; §12:
// configured judge start/exit/timeout/cancellation/malformed-output
// failures are Exit 2, never a favorable fallback). errors.Is(err,
// ErrJudgeOperational) is the stable check a later CLI/report layer uses to
// classify the failure without string-matching.
var ErrJudgeOperational = errors.New("policyconflict: judge operational failure")

// Judge is the consumer-owned port one primary or challenger invocation
// satisfies: the exact canonical prompt bytes, the exact canonical
// normalized-input bytes, and the resulting exchange or error.
type Judge interface {
	Judge(ctx context.Context, prompt []byte, input []byte) (JudgmentExchange, error)
}

// JudgeRunner execs one judge command: argv[0] is the binary, argv[1:] its
// arguments (never a shell string), and stdin carries the invocation
// payload — mirroring the existing align.judge_cmd transport shape (S5:
// argv risk of E2BIG/shell-escaping, so the payload is stdin-only). A
// non-nil error is an exec-level failure (binary not found, context
// cancellation/timeout); a clean exec with a non-zero exit is reported via
// exitCode, never err.
type JudgeRunner interface {
	Run(ctx context.Context, argv []string, stdin []byte) (stdout []byte, exitCode int, err error)
}

// JudgeAdapter is the concrete Judge over one configured process transport
// (authority design §6): Role selects primary or challenger identity,
// Adapter/Model are the adapter-declared transport/model posture (never
// taken from judge output — "Judge output cannot override adapter-declared
// model identity"), Argv is the exact configured command line (the local
// CLI's only source is align.judge_cmd; this package invents no second
// project-config command), Timeout bounds one invocation (<=0 means no
// adapter-imposed deadline beyond ctx's own), and Root is the checkout
// root a later cache-aware caller resolves D4 paths under (JudgeAdapter's
// own Judge method does no path resolution itself — see cache.go).
type JudgeAdapter struct {
	Role    string
	Adapter contextcompile.AdapterRef
	Model   string
	Argv    []string
	Timeout time.Duration
	Root    string
	Runner  JudgeRunner
}

// judgeStdinEnvelope is the process-transport wire this package invents
// from scratch for its OWN judge protocol (distinct from — and never
// reusing — legacy align's judge stdin shape, per authority design §6:
// "it does not reuse legacy align prompts... or permissive wrappers").
// Nothing outside a hermetic test process or the real configured judge
// binary ever decodes this value, so it carries plain, undecorated json
// tags rather than this package's strict/canonical wire conventions, which
// are reserved for persisted or cross-process-compared artifacts.
type judgeStdinEnvelope struct {
	Prompt string          `json:"prompt"`
	Input  json.RawMessage `json:"input"`
}

// Judge implements the Judge port: builds the stdin envelope from prompt
// and input, execs Argv via Runner (bounded by Timeout when positive),
// classifies every transport failure as ErrJudgeOperational, strict-decodes
// and self-validates stdout as a JudgeResult (DecodeJudgeResult already
// enforces exact canonical bytes, closed enums, and finding-cardinality
// rules — authority design §6), and returns the complete JudgmentExchange.
// It does NOT cross-check finding witnesses against a typed SemanticInput
// (see ValidateJudgeResult) and does NOT cache.
func (a JudgeAdapter) Judge(ctx context.Context, prompt, input []byte) (JudgmentExchange, error) {
	if a.Runner == nil {
		return JudgmentExchange{}, fmt.Errorf("%w: adapter runner is nil", ErrJudgeOperational)
	}
	if err := validateJudgeArgv(a.Argv); err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: %v", ErrJudgeOperational, err)
	}
	role := JudgeRole(a.Role)
	if err := role.Validate(); err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: adapter role: %v", ErrJudgeOperational, err)
	}
	if err := validateNonEmpty("adapter.id", a.Adapter.ID); err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: %v", ErrJudgeOperational, err)
	}
	if err := validateNonEmpty("adapter.version", a.Adapter.Version); err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: %v", ErrJudgeOperational, err)
	}
	if err := validateNonEmpty("adapter.model", a.Model); err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: %v", ErrJudgeOperational, err)
	}

	stdin, err := json.Marshal(judgeStdinEnvelope{Prompt: string(prompt), Input: json.RawMessage(input)})
	if err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: building stdin envelope: %v", ErrJudgeOperational, err)
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if a.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, a.Timeout)
		defer cancel()
	}

	stdout, exitCode, runErr := a.Runner.Run(runCtx, a.Argv, stdin)
	if runErr != nil {
		switch {
		case errors.Is(runCtx.Err(), context.DeadlineExceeded):
			return JudgmentExchange{}, fmt.Errorf("%w: judge command timed out: %v", ErrJudgeOperational, runErr)
		case errors.Is(ctx.Err(), context.Canceled):
			return JudgmentExchange{}, fmt.Errorf("%w: judge command canceled: %v", ErrJudgeOperational, runErr)
		default:
			return JudgmentExchange{}, fmt.Errorf("%w: starting judge command: %v", ErrJudgeOperational, runErr)
		}
	}
	if exitCode != 0 {
		return JudgmentExchange{}, fmt.Errorf("%w: judge command exited %d", ErrJudgeOperational, exitCode)
	}

	result, err := DecodeJudgeResult(stdout)
	if err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: decoding judge result: %v", ErrJudgeOperational, err)
	}

	argvDigest := rawContentDigest([]byte(strings.Join(a.Argv, "\x00")))
	return JudgmentExchange{
		Role:          role,
		Adapter:       a.Adapter,
		Model:         a.Model,
		CommandDigest: argvDigest,
		PromptDigest:  rawContentDigest(prompt),
		InputDigest:   rawContentDigest(input),
		RawResult:     string(stdout),
		RawDigest:     rawContentDigest(stdout),
		Result:        result,
	}, nil
}

// validateJudgeArgv rejects only transport-impossible argv shapes. Empty
// later argument values remain legal; an empty vector or argv[0] has no
// executable, and a NUL byte cannot cross the operating-system exec boundary.
func validateJudgeArgv(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("adapter argv is empty")
	}
	if argv[0] == "" {
		return fmt.Errorf("adapter argv[0] executable is empty")
	}
	for i, arg := range argv {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("adapter argv[%d] contains NUL", i)
		}
	}
	return nil
}
