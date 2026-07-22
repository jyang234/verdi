package align

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode"

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
			ID:   judgedFindingID(jf.ID),
			Kind: artifact.FindingJudged,
			Text: decorateConfidence(normalizeJudgeText(jf.Text), jf.Confidence),
		})
	}
	return &JudgeSuccess{Findings: findings, Stdin: prompt, RawResult: rawResult}, nil
}

// reservedIDGuard is the sentinel segment judgedFindingID splices in to
// neutralize a raw judge slug whose minted id would otherwise land on a shape
// ReconcileJudged alone may MINT (artifact.IsCollisionMachineryID). It is a
// fixed literal, so the escape stays a pure, deterministic function of the raw
// slug.
const reservedIDGuard = "reserved"

// judgedFindingID normalizes a judge-supplied raw finding id into this
// package's own stable "judged-<slug>" finding id — the shared minting helper
// for every judge-consuming mode (this file's runJudgeOnce,
// decision_judge.go's RunDecisionSweep, diagram_judge.go's RunDiagramSweep).
// rawID is judge-supplied free text (S5's contract only requires {id, text,
// confidence}). It composes two independently-pinned guards, in order, at the
// single MINT seam.
//
// (1) PREFIX IDEMPOTENCE + DOUBLED-ECHO PRESERVATION (spec/ritual-traps ac-2,
// X-1's sibling trap). On certain regeneration/carry paths a caller re-presents
// a prior finding's own already-"judged-"-prefixed id back to the judge as
// context; if the judge echoes it verbatim, slugging that value and
// unconditionally re-prefixing produced "judged-judged-..." — a tool defect. So
// a raw id whose slug already carries the "judged-" prefix is used AS-IS rather
// than re-prefixed. When that echoed id is itself an ALREADY-DOUBLED archived id
// ("judged-judged-x") the short-circuit preserves it VERBATIM — deliberately NOT
// normalized to "judged-x" (adjudicated, finding
// judged-ac2-echoed-doubled-id-still-doubles): dispositions reference finding
// ids exactly as originally minted (the W4 precedent), so collapsing a doubled
// id would sever the id-join to any archived disposition recorded against the
// doubled form, orphaning it. "Exactly one judged- prefix" is thus a PROSPECTIVE
// mint-time property of fresh ids, never a retroactive renumbering of an id a
// prior report already minted. (TestJudgedFindingID,
// TestJudgedFindingID_EchoedDoubledID_PreservedVerbatim.)
//
// (2) RESERVED-SHAPE ESCAPE (spec/finding-identity's judged-reserved-id-shape
// guard, judged-reserved-id-shape-substring-match). Whatever id the prefix rule
// yields, a judge must never end up minting one that IsCollisionMachineryID — a
// shape ReconcileJudged ALONE may mint (the numeric-tail collision member
// "<slug><CollisionInfix><n>" or the ContractViolationIDPrefix) — or the two id
// consumers (ReconcileJudged's candidate path vs. the disposition verb's live
// path) would disagree about the same id (the L-N13 consumers-agree property: a
// forged shape gets a rendered ac-1 Candidate yet has its live-path
// resolve+stamp withheld). The escape splices reservedIDGuard so the reserved
// shape breaks — the guard right after "judged-" (so the id no longer begins
// with ContractViolationIDPrefix) and/or as a trailing segment (so it no longer
// ends in "<CollisionInfix><digits>"). Running the escape AFTER the prefix rule
// is deliberate: it closes the forge in BOTH directions — a fresh raw slug
// shaped exactly like a minted id, AND a judge trying to smuggle a reserved
// shape through the verbatim echo path above (e.g. a raw
// "judged-contract-violation-x"), which the prefix rule alone would preserve
// intact. (TestJudgedFindingID_ReservesMintedShapes.)
//
// The two guards compose without conflict: the escape is a literal transform
// whose result is ITSELF never a reserved shape (its prefix no longer starts
// with the CV literal; its tail no longer ends in digits), so re-minting from an
// escaped id — whether slug-stripped or echoed back "judged-"-prefixed — is a
// no-op (the guard is idempotent, never compounds), and an echoed doubled id
// that is not a reserved shape passes through both guards untouched. Fixed
// prospectively only: this touches an id solely at MINT time; nothing rewrites
// an id already read back off an archived report (internal/artifact decodes
// Finding.ID with no transform).
//
// RESIDUAL (disclosed, self-healing): the escape is not injective, so a raw
// judge slug that already mints to an escaped form could in principle collide
// with an escaped one. Both require the judge to emit a very specifically
// reserved-shaped slug, and a same-run collision is caught and disclosed by
// ReconcileJudged's own within-run collision machinery (every member suffixed +
// a contract-violation finding), never a silent merge.
func judgedFindingID(rawID string) string {
	// (1) Prefix idempotence + doubled-echo preservation: a slug already
	// carrying "judged-" is used as-is (a re-presented prior id is never
	// re-prefixed; an archived doubled id survives verbatim).
	slug := store.RefSlug(rawID)
	id := slug
	if !strings.HasPrefix(slug, "judged-") {
		id = "judged-" + slug
	}
	// (2) Reserved-shape escape, applied to whatever (1) yielded — including a
	// verbatim echo — so a judge can never FORGE a shape ReconcileJudged alone
	// may mint.
	if strings.HasPrefix(id, artifact.ContractViolationIDPrefix) {
		// Break the reserved PREFIX: splice the guard right after "judged-".
		id = "judged-" + reservedIDGuard + "-" + strings.TrimPrefix(id, "judged-")
	}
	if artifact.IsCollisionMachineryID(id) {
		// Only the numeric-tail arm can still be true here (the CV prefix, if it
		// was present, was just broken above). Append the guard so the id no
		// longer ends in "<CollisionInfix><digits>".
		id = id + "-" + reservedIDGuard
	}
	return id
}

// confidenceSuffixRE matches ONE trailing " (confidence N.NN)" decoration of
// exactly the shape decorateConfidence itself mints: a single leading space,
// the literal "(confidence ", a %.2f-shaped number (optional sign, one-or-more
// integer digits, a dot, exactly two fractional digits), and a closing paren —
// anchored to end-of-string so only a genuine TAIL decoration is stripped,
// never a mint-shaped fragment the finding text legitimately quotes mid-sentence
// (e.g. this very defect's own report quotes a doubled "(confidence 0.30)
// (confidence 0.30)" as its witness, which must survive intact). A one-digit
// fractional part or a non-numeric parenthetical is not this mint shape and is
// deliberately left alone.
var confidenceSuffixRE = regexp.MustCompile(` \(confidence -?[0-9]+\.[0-9]{2}\)$`)

// decorateConfidence appends the CURRENT run's confidence to a judge-emitted
// finding text exactly once, idempotently — the text-half analogue of
// judgedFindingID's "judged-" prefix idempotence, at the same ingestion seam
// (spec/ritual-traps: the confidence suffix is the sibling trap of the prefix
// the id half already guards).
//
// runJudgeOnce decorates every judged finding's text with " (confidence N.NN)".
// On certain regeneration/carry paths (spec/finding-identity) a prior report's
// already-decorated text is re-presented to the judge as context; if the judge
// echoes it back verbatim, a naive append lands a SECOND suffix, and each carry
// round adds another (the render-echo-redecorate loop that produced
// "judged-judged-" for the id half). Before appending, strip every trailing
// suffix of the shape this function itself mints — REPEATEDLY, so an
// already-doubled archived echo collapses back to bare text — then append this
// run's confidence once. The result carries exactly one suffix reflecting THIS
// run, regardless of how many the echoed text arrived with. (A judge whose
// genuine free-text happens to END with a mint-shaped "(confidence N.NN)" is
// indistinguishable from an echo and is treated as one — the same benign
// tradeoff judgedFindingID makes for a raw id that genuinely begins "judged-";
// only the exact tail is at risk, never mid-text content, per the $ anchor.)
//
// Fixed prospectively only (the judgedFindingID precedent): this runs solely at
// MINT time on a judge exchange being ingested now; nothing rewrites text
// already read back off an archived report (internal/artifact decodes
// Finding.Text with no transform), so reports already on disk are untouched.
// Unlike the id half, this collapse carries no orphaning risk: nothing joins on
// Finding.Text. It feeds align.Identity's content hash (identity.go), but that
// hash is a same-process disposition-carry match that already fails closed on
// ANY text change and is never a persisted join key — persisted dispositions
// reference a finding by its ID (cmd/verdi/disposition.go matches f.ID, never
// the text hash). A finding minted under the old doubling bug therefore simply
// fails closed to undispositioned on its first post-fix regeneration (a human
// re-looks once), after which the now-stable text lets its disposition carry
// forward normally — a strict improvement, since the growing doubled text broke
// carry every round.
func decorateConfidence(text string, confidence float64) string {
	for confidenceSuffixRE.MatchString(text) {
		text = confidenceSuffixRE.ReplaceAllString(text, "")
	}
	return fmt.Sprintf("%s (confidence %.2f)", text, confidence)
}

// normalizeJudgeText replaces every maximal run of Unicode control
// characters (newlines, carriage returns, tabs, and other C0/C1 controls)
// in judge-emitted text with a single space, then trims the result — input
// hygiene at the ONE seam every judge-emitted Finding/ConflictFinding Text
// passes through (this function backs both this file's build-branch
// ingestion and decision_judge.go's design-branch sweep, which shares the
// identical Text-construction shape). ADJ-53's j-4 fix: a judge is
// free-text and nothing in S5's own contract constrains it to a single
// line, but every downstream consumer assumes one —
// align.RenderFindingLine's single rendered bullet, the disposition verb's
// whole-line matcher (cmd/verdi/disposition.go), and Identity's
// content-hash finding equality (identity.go, hashing Text as part of a
// finding's stable identity). Establishing the invariant here, once, for
// every finding this package ever produces, is the fix; teaching each of
// those consumers to tolerate a multi-line "line" would be accommodating
// corrupt-shaped input rather than preventing it.
func normalizeJudgeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range s {
		if unicode.IsControl(r) {
			pendingSpace = true
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
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

// decodeInnerResult decodes the judge's findings JSON out of its free-text
// result, tolerating a natural-language preamble/postamble and/or a markdown
// code fence around the object (innerparse.go: decodeJudgeInnerJSON) before
// strict-decoding it. The strict-decode contract is preserved on the extracted
// object; only the surrounding prose is widened. This is the build-branch
// binding of the one shared inner-parse seam every judge-consuming mode uses.
func decodeInnerResult(raw string) (*judgeInnerResult, error) {
	return decodeJudgeInnerJSON[judgeInnerResult](raw)
}

func snippet(b []byte) string {
	if len(b) > stderrSnippetLimit {
		b = b[:stderrSnippetLimit]
	}
	return string(b)
}
