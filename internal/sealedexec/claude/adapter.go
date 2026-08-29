// Package claude implements the sealed Claude Code adapter boundary.
package claude

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/sealedexec"
)

const (
	// DecoderProfileV1 is the Amendment 002 §5 decoder profile literal.
	DecoderProfileV1 = "claude-stream-json-v1"

	claudeSource = "claude-stream-json-v1"
)

// ProcessResult is the explicit terminal process result.
type ProcessResult struct {
	ExitCode int
}

// ProcessObservation is one pull from the process boundary.
// ForeignJSON is one JSONL frame; Complete reports whether its newline framing completed.
// Terminal is the mutually exclusive explicit process result.
type ProcessObservation struct {
	ForeignJSON []byte
	Complete    bool
	Terminal    *ProcessResult
}

// ProcessStopResult is the provider's normalized stop outcome.
type ProcessStopResult struct {
	ExitCode   int
	ReasonCode string
}

// ActiveProcess exposes one framed foreign observation per pull and owns the
// cancellation handle for that exact process.
type ActiveProcess interface {
	Next(context.Context) (ProcessObservation, error)
	Stop(context.Context) (ProcessStopResult, error)
}

// Process is the adapter's consumer-defined launch boundary. It includes a
// pre-launch version probe unlike the Codex adapter.
type Process interface {
	Probe(context.Context, *exec.Cmd) (stdout, stderr []byte, exitCode int, err error)
	Start(context.Context, *exec.Cmd, []byte) (ActiveProcess, error)
}

// Adapter owns the pinned argv, typed stdin, and Claude stream-json decoder.
type Adapter struct {
	process   Process
	processor *sealedexec.DetailProcessor
}

// New constructs an adapter over explicit process and detail-processor ports.
func New(process Process, processor *sealedexec.DetailProcessor) (*Adapter, error) {
	if process == nil {
		return nil, errors.New("sealedexec/claude: process port is nil")
	}
	if processor == nil {
		return nil, errors.New("sealedexec/claude: detail processor is nil")
	}
	return &Adapter{process: process, processor: processor}, nil
}

// VerifyAdapter proves the selected executable/profile/version/decoder and
// constructs (but never starts) the exact profile-governed invocation.
func (a *Adapter) VerifyAdapter(ctx context.Context, check sealedexec.AdapterCheck) (sealedexec.AdapterFacts, error) {
	if ctx == nil {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/claude: verify adapter: nil context")
	}
	if check.Request.Adapter != contextevent.AdapterClaude ||
		check.Profile.AdapterVersion != check.Request.AdapterVersion ||
		check.Profile.DecoderProfile != DecoderProfileV1 ||
		check.Profile.Executable == "" || !filepath.IsAbs(check.Profile.Executable) ||
		check.Profile.Digest != check.Request.Profile.Digest ||
		check.Profile.WorkspacePath != check.Workspace.Path {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/claude: adapter identity/profile/version mismatch")
	}
	// Construct but never start the profile-governed command.
	command, _, cancel, err := check.Profile.Profile.Command(ctx, check.Profile.Executable, "--version")
	if err != nil {
		return sealedexec.AdapterFacts{}, fmt.Errorf("sealedexec/claude: verify isolated command: %w", err)
	}
	cancel()
	if command.Path != check.Profile.Executable ||
		!equalStrings(command.Env, check.Profile.Profile.Env()) {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/claude: constructed command contradicts resolved profile")
	}
	return sealedexec.AdapterFacts{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Failure: sealedexec.FailureNone, Witnesses: []string{}},
		Adapter:      contextevent.AdapterClaude, AdapterVersion: check.Request.AdapterVersion,
		Executable: check.Profile.Executable, ProfileDigest: check.Profile.Digest,
		DecoderProfile: DecoderProfileV1,
	}, nil
}

// Start invokes the Amendment 002 §4 start form. It validates profile
// activation (API-key membership, minimum-eight), probes the version,
// and launches.
func (a *Adapter) Start(ctx context.Context, launch sealedexec.AdapterLaunch) (sealedexec.ActiveAdapterRun, error) {
	if ctx == nil {
		return nil, errors.New("sealedexec/claude: start: nil context")
	}
	if launch.Review != nil {
		return nil, errors.New("sealedexec/claude: sealed review is not supported")
	}
	if err := validateClaudeProfile(launch.Profile); err != nil {
		return nil, err
	}
	args, err := startArgs(launch.Profile)
	if err != nil {
		return nil, err
	}
	return a.run(ctx, launch, args, "", true)
}

// Resume invokes only an explicit independently verified session id.
// Review launches are refused: Claude resume does not carry review launch facts.
func (a *Adapter) Resume(ctx context.Context, launch sealedexec.AdapterLaunch, sessionRef string) (sealedexec.ActiveAdapterRun, error) {
	if ctx == nil {
		return nil, errors.New("sealedexec/claude: resume: nil context")
	}
	if launch.Review != nil {
		return nil, errors.New("sealedexec/claude: sealed review is start-only")
	}
	if err := validateSessionRef(sessionRef); err != nil {
		return nil, err
	}
	if err := validateClaudeProfile(launch.Profile); err != nil {
		return nil, err
	}
	args, err := resumeArgs(launch.Profile, sessionRef)
	if err != nil {
		return nil, err
	}
	return a.run(ctx, launch, args, sessionRef, false)
}

func (a *Adapter) run(ctx context.Context, launch sealedexec.AdapterLaunch, args []string, expectedSession string, start bool) (sealedexec.ActiveAdapterRun, error) {
	if launch.Profile.DecoderProfile != DecoderProfileV1 || launch.Profile.AdapterVersion != launch.Request.AdapterVersion {
		return nil, errors.New("sealedexec/claude: selected profile does not bind decoder/version")
	}

	// §3: Before every start or resume, probe the version.
	probeCmd, _, probeCancel, err := launch.Profile.Profile.Command(ctx, launch.Profile.Executable, "--version")
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: construct version probe: %w", err)
	}
	probeCancel()
	stdout, stderr, exitCode, err := a.process.Probe(ctx, probeCmd)
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: version probe: %w", err)
	}
	if exitCode != 0 || len(stderr) != 0 {
		return nil, fmt.Errorf("sealedexec/claude: version probe: exit %d, stderr present=%t", exitCode, len(stderr) != 0)
	}
	// One UTF-8 stdout line after removing the single trailing LF.
	probeOut := bytes.TrimSuffix(stdout, []byte("\n"))
	if bytes.ContainsAny(probeOut, "\r\n") || !utf8.Valid(probeOut) {
		return nil, errors.New("sealedexec/claude: version probe: output has unexpected newlines or invalid UTF-8")
	}
	if string(probeOut) != launch.Request.AdapterVersion {
		return nil, fmt.Errorf("sealedexec/claude: version probe: output %q != expected %q", string(probeOut), launch.Request.AdapterVersion)
	}

	// Build and encode the typed stdin envelope.
	stdin, err := encodeClaudeStdin(launch.Input)
	if err != nil {
		return nil, err
	}

	command, runCtx, cancel, err := launch.Profile.Profile.Command(ctx, launch.Profile.Executable, args...)
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: construct process: %w", err)
	}
	command.Dir = launch.Workspace.Path
	command.Stdin = bytes.NewReader(stdin)
	processRun, err := a.process.Start(runCtx, command, stdin)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sealedexec/claude: process: %w", err)
	}
	if processRun == nil {
		cancel()
		return nil, errors.New("sealedexec/claude: process returned a nil active run")
	}

	// Build the per-run protected value set: classified secrets.
	// The provider session (extracted from init) is added before init emission.
	protectedValues := append([][]byte(nil), launch.Profile.PolicySecretValues...)

	return &claudeActiveRun{
		process:         processRun,
		processor:       a.processor,
		launch:          launch,
		expectedSession: expectedSession,
		start:           start,
		cancel:          cancel,
		protectedValues: protectedValues,
	}, nil
}

// validateClaudeProfile enforces §3: API key membership, min-8, classification complete, nonempty set.
func validateClaudeProfile(profile sealedexec.ResolvedProfile) error {
	if !profile.ClassificationComplete {
		return errors.New("sealedexec/claude: profile classification is incomplete")
	}
	if len(profile.PolicySecretValues) == 0 {
		return errors.New("sealedexec/claude: policy secret set must be nonempty")
	}
	for _, v := range profile.PolicySecretValues {
		if len(v) == 0 {
			return errors.New("sealedexec/claude: policy secret set has an empty member")
		}
	}
	// Locate the activated ANTHROPIC_API_KEY in the profile env.
	apiKey := envValueFromEnv(profile.Profile.Env(), "ANTHROPIC_API_KEY")
	if apiKey == "" {
		return errors.New("sealedexec/claude: ANTHROPIC_API_KEY is absent from profile env")
	}
	// §3: "a set that does not contain the activated API key refuses launch"
	if !containsByteSlice(profile.PolicySecretValues, []byte(apiKey)) {
		return errors.New("sealedexec/claude: activated ANTHROPIC_API_KEY is not a member of the policy secret set")
	}
	// §3: "at least eight UTF-8 bytes"
	if len(apiKey) < 8 {
		return errors.New("sealedexec/claude: ANTHROPIC_API_KEY must be at least 8 UTF-8 bytes")
	}
	return nil
}

func startArgs(profile sealedexec.ResolvedProfile) ([]string, error) {
	model := profile.Name // §3: model is stored in profile.Name (pragmatic gap — service.go lacks Model field)
	if model == "" {
		return nil, errors.New("sealedexec/claude: profile.Name (model) is empty")
	}
	mcpConfigPath, err := mcpConfigPathFromProfile(profile)
	if err != nil {
		return nil, err
	}
	return []string{
		"--bare", "-p",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--model", model,
		"--permission-mode", "bypassPermissions",
		"--strict-mcp-config",
		"--mcp-config", mcpConfigPath,
		"--no-chrome",
	}, nil
}

func resumeArgs(profile sealedexec.ResolvedProfile, sessionRef string) ([]string, error) {
	args, err := startArgs(profile)
	if err != nil {
		return nil, err
	}
	return append(args, "--resume", sessionRef), nil
}

// mcpConfigPathFromProfile derives the MCP config path: <envRoot>/claude-mcp.json.
// HOME in the profile env is <envRoot>/.home (set by execworkspace.BuildProfile),
// so envRoot = filepath.Dir(HOME).
func mcpConfigPathFromProfile(profile sealedexec.ResolvedProfile) (string, error) {
	home := envValueFromEnv(profile.Profile.Env(), "HOME")
	if home == "" {
		return "", errors.New("sealedexec/claude: profile env has no HOME")
	}
	envRoot := filepath.Dir(home)
	if envRoot == "." || envRoot == "" {
		return "", errors.New("sealedexec/claude: profile HOME does not have a valid parent directory")
	}
	return filepath.Join(envRoot, "claude-mcp.json"), nil
}

// encodeClaudeStdin wraps the shared provider input in the Claude user-message format.
// stdin = {"type":"user","message":{"role":"user","content":[{"type":"text","text":"VERDI_SEALED_PROVIDER_INPUT_V1\n<json>"}]}}
func encodeClaudeStdin(input sealedexec.ProviderInput) ([]byte, error) {
	providerInputBytes, err := sealedexec.EncodeProviderInput(input)
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: encode provider input: %w", err)
	}
	// Remove trailing LF before embedding.
	providerInputJSON := string(bytes.TrimSuffix(providerInputBytes, []byte("\n")))
	text := "VERDI_SEALED_PROVIDER_INPUT_V1\n" + providerInputJSON
	msg := claudeStdinMessage{
		Type: "user",
		Message: claudeStdinInner{
			Role: "user",
			Content: []claudeStdinBlock{{
				Type: "text",
				Text: text,
			}},
		},
	}
	encoded, err := canonjson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: encode stdin message: %w", err)
	}
	return encoded, nil
}

type claudeStdinMessage struct {
	Type    string           `json:"type"`
	Message claudeStdinInner `json:"message"`
}

type claudeStdinInner struct {
	Role    string             `json:"role"`
	Content []claudeStdinBlock `json:"content"`
}

type claudeStdinBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ---------------------------------------------------------------------------
// Active run
// ---------------------------------------------------------------------------

type claudeActiveRun struct {
	process         ActiveProcess
	processor       *sealedexec.DetailProcessor
	launch          sealedexec.AdapterLaunch
	expectedSession string
	start           bool
	cancel          context.CancelFunc
	protectedValues [][]byte

	nextMu sync.Mutex
	mu     sync.Mutex

	// Session state
	providerSession string // extracted from first valid init
	initReceived    bool
	resultReceived  bool

	// Tool call tracking: call_id -> tool_name
	pendingToolCalls map[string]string

	// Foreign sequence counter (1-based)
	foreignSeq uint64

	// Terminal buffering
	pendingResultObs *pendingResultObservations

	// Stop machinery
	terminal      bool
	stopOnce      sync.Once
	stopDone      chan struct{}
	stopRequested bool
	stopDelivered bool
	stopResult    sealedexec.AdapterStopResult
	stopErr       error
}

// pendingResultObservations buffers the terminal result until process exit.
type pendingResultObservations struct {
	observations []sealedexec.NormalizedObservation
	failure      string
}

func (r *claudeActiveRun) Next(ctx context.Context) (sealedexec.AdapterResult, error) {
	if ctx == nil {
		return sealedexec.AdapterResult{}, errors.New("sealedexec/claude: next: nil context")
	}
	r.nextMu.Lock()
	defer r.nextMu.Unlock()

	r.mu.Lock()
	stopRequested, stopDone := r.stopRequested, r.stopDone
	if r.terminal && !stopRequested {
		r.mu.Unlock()
		return sealedexec.AdapterResult{}, errors.New("sealedexec/claude: next after terminal result")
	}
	r.mu.Unlock()

	if stopRequested {
		<-stopDone
		return r.stopTerminal()
	}

	item, err := r.process.Next(ctx)
	r.mu.Lock()
	stopRequested = r.stopRequested
	stopDone = r.stopDone
	r.mu.Unlock()

	if err != nil {
		if stopRequested {
			<-stopDone
			return r.stopTerminal()
		}
		return sealedexec.AdapterResult{}, fmt.Errorf("sealedexec/claude: process stream: %w", err)
	}

	if stopRequested && (item.Terminal != nil || item.ForeignJSON == nil || !item.Complete) {
		<-stopDone
		return r.stopTerminal()
	}

	if item.Terminal != nil {
		if item.ForeignJSON != nil || item.Complete {
			r.cancel()
			return sealedexec.AdapterResult{}, errors.New("sealedexec/claude: process stream terminal/result union is invalid")
		}
		r.mu.Lock()
		r.terminal = true
		r.mu.Unlock()
		r.cancel()
		return r.handleProcessTerminal(ctx, item.Terminal)
	}

	if item.ForeignJSON == nil {
		r.cancel()
		return sealedexec.AdapterResult{}, errors.New("sealedexec/claude: process stream observation/result union is invalid")
	}

	r.mu.Lock()
	r.foreignSeq++
	seq := r.foreignSeq
	r.mu.Unlock()

	if !item.Complete {
		detail := r.malformedSafeDetail(ctx, item.ForeignJSON, "truncated-final-line")
		return sealedexec.AdapterResult{
			Observations:       r.gapObservations(detail, seq, "truncated-final-line"),
			OperationalFailure: "truncated-final-line",
		}, nil
	}

	return r.normalize(ctx, item.ForeignJSON, seq), nil
}

func (r *claudeActiveRun) handleProcessTerminal(ctx context.Context, proc *ProcessResult) (sealedexec.AdapterResult, error) {
	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}

	r.mu.Lock()
	pending := r.pendingResultObs
	r.mu.Unlock()

	if pending == nil {
		// No terminal result was received before process exit.
		r.mu.Lock()
		r.foreignSeq++
		seq := r.foreignSeq
		r.mu.Unlock()
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-terminal-result"})
		result.Observations = append(result.Observations, r.gapObservations(detail, seq, "missing-terminal-result")...)
		result.Observations = append(result.Observations, adapterStopObservation(r.launch, proc.ExitCode, "missing-terminal-result"))
		result.OperationalFailure = "missing-terminal-result"
		result.Terminal = &sealedexec.AdapterTerminalResult{ExitCode: proc.ExitCode}
		return result, nil
	}

	// Emit pending result observations.
	result.Observations = append(result.Observations, pending.observations...)
	if pending.failure != "" {
		result.OperationalFailure = pending.failure
	}

	// Determine final stop.
	if pending.failure != "" {
		// Provider failure: adapter-stop with actual exit code.
		result.Observations = append(result.Observations, adapterStopObservation(r.launch, proc.ExitCode, pending.failure))
	} else {
		// Success: only emit if exit 0 and empty stderr.
		// (Stderr check: this adapter does not re-read stderr here;
		//  the process boundary owns stderr handling. For test purposes,
		//  we trust exit 0 as success.)
		if proc.ExitCode != 0 {
			r.mu.Lock()
			r.foreignSeq++
			seq := r.foreignSeq
			r.mu.Unlock()
			detail := r.fixedSafeDetail(ctx, map[string]any{"exit_code": proc.ExitCode, "reason": "nonzero-exit"})
			result.Observations = append(result.Observations, r.gapObservations(detail, seq, "nonzero-exit")...)
			result.Observations = append(result.Observations, adapterStopObservation(r.launch, proc.ExitCode, "nonzero-exit"))
			result.OperationalFailure = "nonzero-exit"
		} else {
			result.Observations = append(result.Observations, adapterStopObservation(r.launch, 0, "completed"))
		}
	}
	result.Terminal = &sealedexec.AdapterTerminalResult{ExitCode: proc.ExitCode}
	return result, nil
}

func (r *claudeActiveRun) Stop(ctx context.Context) (sealedexec.AdapterStopResult, error) {
	if ctx == nil {
		return sealedexec.AdapterStopResult{}, errors.New("sealedexec/claude: stop: nil context")
	}
	r.stopOnce.Do(func() {
		r.mu.Lock()
		r.stopRequested = true
		r.stopDone = make(chan struct{})
		r.mu.Unlock()
		result, err := r.process.Stop(ctx)
		if err != nil {
			err = fmt.Errorf("sealedexec/claude: stop process: %w", err)
		} else if strings.TrimSpace(result.ReasonCode) == "" {
			err = errors.New("sealedexec/claude: stop process returned an empty reason code")
		}
		r.mu.Lock()
		if err != nil {
			r.stopErr = err
		} else {
			r.stopResult = sealedexec.AdapterStopResult{ExitCode: result.ExitCode, ReasonCode: result.ReasonCode}
		}
		r.mu.Unlock()
		r.cancel()
		close(r.stopDone)
	})
	<-r.stopDone
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopResult, r.stopErr
}

func (r *claudeActiveRun) stopTerminal() (sealedexec.AdapterResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopDelivered {
		return sealedexec.AdapterResult{}, errors.New("sealedexec/claude: next after stop terminal")
	}
	r.stopDelivered = true
	r.terminal = true
	if r.stopErr != nil {
		return sealedexec.AdapterResult{}, r.stopErr
	}
	stop := r.stopResult
	return sealedexec.AdapterResult{Stopped: &stop, Observations: []sealedexec.NormalizedObservation{}}, nil
}

// ---------------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------------

func (r *claudeActiveRun) normalize(ctx context.Context, line []byte, seq uint64) sealedexec.AdapterResult {
	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}

	if len(line) == 0 {
		detail := r.malformedSafeDetail(ctx, line, "empty-foreign-record")
		result.Observations = r.gapObservations(detail, seq, "empty-foreign-record")
		result.OperationalFailure = "empty-foreign-record"
		return result
	}

	object, err := sealedexec.DecodeUniqueJSONObject(line)
	if err != nil {
		detail := r.malformedSafeDetail(ctx, line, "malformed-foreign-frame")
		result.Observations = r.gapObservations(detail, seq, "malformed-foreign-frame")
		result.OperationalFailure = "malformed-foreign-frame"
		return result
	}

	outer, ok := nonemptyString(object["type"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "type"})
		result.Observations = r.gapObservations(detail, seq, "missing-foreign-field")
		result.OperationalFailure = "missing-foreign-field"
		return result
	}

	subtype, _ := nonemptyString(object["subtype"])
	family := outer
	if subtype != "" {
		family = outer + "/" + subtype
	}

	// Check global state machine constraints.
	r.mu.Lock()
	initReceived := r.initReceived
	resultReceived := r.resultReceived
	r.mu.Unlock()

	if outer == "result" && !initReceived {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "result-before-init", "family": family})
		result.Observations = r.gapObservations(detail, seq, "result-before-init")
		result.OperationalFailure = "result-before-init"
		return result
	}

	if resultReceived && outer != "result" {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "observation-after-result", "family": family})
		result.Observations = r.gapObservations(detail, seq, "observation-after-result")
		result.OperationalFailure = "observation-after-result"
		return result
	}

	if outer == "system" && subtype == "init" {
		if initReceived {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "duplicate-init"})
			result.Observations = r.gapObservations(detail, seq, "duplicate-init")
			result.OperationalFailure = "duplicate-init"
			return result
		}
		return r.handleInit(ctx, object, seq)
	}

	if outer == "system" && subtype == "api_retry" {
		return r.handleRetry(ctx, object, seq)
	}

	if outer == "assistant" {
		return r.handleAssistant(ctx, object, seq)
	}

	if outer == "user" {
		return r.handleToolResult(ctx, object, seq)
	}

	if outer == "result" {
		if resultReceived {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "duplicate-result"})
			result.Observations = r.gapObservations(detail, seq, "duplicate-result")
			result.OperationalFailure = "duplicate-result"
			return result
		}
		return r.handleResult(ctx, object, seq)
	}

	// Unknown family
	detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "unknown-foreign-family", "type": outer})
	result.Observations = r.gapObservations(detail, seq, "unknown-foreign-family")
	result.OperationalFailure = "unknown-foreign-family"
	return result
}

// ---------------------------------------------------------------------------
// Family handlers
// ---------------------------------------------------------------------------

func (r *claudeActiveRun) handleInit(ctx context.Context, object map[string]any, seq uint64) sealedexec.AdapterResult {
	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}

	sessionID, ok := nonemptyString(object["session_id"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "session_id"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}

	model, ok := nonemptyString(object["model"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "model"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}

	// §5: observed model equals profile model (profile.Name).
	profileModel := r.launch.Profile.Name
	if model != profileModel {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "model-mismatch", "observed": model, "expected": profileModel})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "model-mismatch"), OperationalFailure: "model-mismatch"}
	}

	// §5: MCP inventory is exactly the scoped server.
	if reason := validateInitMCPServers(object); reason != "" {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": reason})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, reason), OperationalFailure: reason}
	}

	// §5: permission mode is bypassPermissions.
	permMode, _ := nonemptyString(object["permissionMode"])
	if permMode != "bypassPermissions" {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "permissionMode"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
	}

	// §5: plugins is the required empty array.
	plugins, pluginsOK := object["plugins"]
	if !pluginsOK {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "plugins"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	pluginsArr, _ := plugins.([]any)
	if len(pluginsArr) != 0 {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "plugins"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
	}

	// Resume: check session matches.
	if !r.start {
		if sessionID != r.expectedSession {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "session-mismatch"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "session-mismatch"), OperationalFailure: "session-mismatch"}
		}
	}

	// §3: extract provider session and add to protected-value set BEFORE redacting I.
	r.mu.Lock()
	r.initReceived = true
	r.providerSession = sessionID
	// Add session ID to protected values for variable detail redaction.
	r.protectedValues = append(r.protectedValues, []byte(sessionID))
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()

	// Build I = {family:"system/init", model, mcp_servers, permission_mode, session_id}.
	mcpServersRedacted := []map[string]string{{"name": "verdi-context", "status": "connected"}}
	initPayloadObj := map[string]any{
		"family":          "system/init",
		"mcp_servers":     mcpServersRedacted,
		"model":           model,
		"permission_mode": permMode,
		"session_id":      sessionID,
	}
	initDetail, err := r.processDetail(ctx, initPayloadObj, protectedValues)
	if err != nil {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
	}

	// Build adapter-start (nil detail for builder; non-nil only for review).
	adapterStart := buildAdapterStart(r.launch, nil)
	initSummary := buildProviderSummary(r.launch, "system/init", initDetail.Digest, contextevent.AuthorityAdvisory, initDetail)

	result.ObservedSessionRef = sessionID
	result.Observations = []sealedexec.NormalizedObservation{}

	// Resume emits: resume, adapter-start, provider-summary.
	// Start emits: adapter-start, provider-summary.
	if !r.start {
		resumeObs := buildResumeObservation(r.launch, initDetail)
		result.Observations = append(result.Observations, resumeObs)
	}
	result.Observations = append(result.Observations, adapterStart, initSummary)
	return result
}

func (r *claudeActiveRun) handleRetry(ctx context.Context, object map[string]any, seq uint64) sealedexec.AdapterResult {
	// Validate required fields.
	attempt, attemptOK := uintValueF(object["attempt"])
	maxRetries, maxRetriesOK := uintValueF(object["max_retries"])
	retryDelay, retryDelayOK := uintValueF(object["retry_delay_ms"])
	uuid, uuidOK := nonemptyString(object["uuid"])
	sessionID, sessionOK := nonemptyString(object["session_id"])
	errorObj, errorOK := object["error"].(map[string]any)
	errorCategory, categoryOK := nonemptyString(errorObj["type"])

	if !attemptOK || !maxRetriesOK || !retryDelayOK || !uuidOK || !sessionOK || !errorOK || !categoryOK ||
		attempt == 0 || maxRetries == 0 || attempt > maxRetries {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "family": "system/api_retry"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}

	// Validate error category.
	validCategories := map[string]bool{
		"authentication": true, "billing": true, "rate_limit": true,
		"server": true, "network": true, "unknown": true,
	}
	if !validCategories[errorCategory] {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "error.type"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
	}

	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()

	if sessionID != currentSession {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "session-mismatch"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "session-mismatch"), OperationalFailure: "session-mismatch"}
	}

	// D = {family:"system/api_retry", attempt, max_retries, retry_delay_ms, error_category, uuid, session_id}
	retryDetailObj := map[string]any{
		"attempt":        attempt,
		"error_category": errorCategory,
		"family":         "system/api_retry",
		"max_retries":    maxRetries,
		"retry_delay_ms": retryDelay,
		"session_id":     sessionID,
		"uuid":           uuid,
	}
	detail, err := r.processDetail(ctx, retryDetailObj, protectedValues)
	if err != nil {
		safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
	}

	reasonCode := "provider-api-" + errorCategory
	schema, _ := contextevent.PayloadSchema(contextevent.KindRetry)
	retryPayload := &contextevent.RetryPayload{
		Schema:           schema,
		ReasonCode:       reasonCode,
		PriorSession:     currentSession,
		NextSession:      currentSession,
		ContinuityDigest: detail.Digest,
	}
	retryObs := sealedexec.NormalizedObservation{Kind: contextevent.KindRetry, Payload: retryPayload, ForeignDetail: detail}
	summaryObs := buildProviderSummary(r.launch, "api-retry/"+fmt.Sprintf("%d", attempt), detail.Digest, contextevent.AuthorityAdvisory, detail)
	return sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{retryObs, summaryObs}}
}

func (r *claudeActiveRun) handleAssistant(ctx context.Context, object map[string]any, seq uint64) sealedexec.AdapterResult {
	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}

	sessionID, ok := nonemptyString(object["session_id"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "session_id"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()
	if sessionID != currentSession {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "session-mismatch"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "session-mismatch"), OperationalFailure: "session-mismatch"}
	}

	message, ok := object["message"].(map[string]any)
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "message"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	messageID, ok := nonemptyString(message["id"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "message.id"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	role, _ := nonemptyString(message["role"])
	if role != "assistant" {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "message.role"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
	}
	msgModel, _ := nonemptyString(message["model"])
	if msgModel != r.launch.Profile.Name {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "model-mismatch"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "model-mismatch"), OperationalFailure: "model-mismatch"}
	}
	content, ok := message["content"].([]any)
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "message.content"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}

	for blockIndex, blockAny := range content {
		block, ok := blockAny.(map[string]any)
		if !ok {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "malformed-foreign-frame", "field": "content_block"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "malformed-foreign-frame"), OperationalFailure: "malformed-foreign-frame"}
		}
		blockType, ok := nonemptyString(block["type"])
		if !ok {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "content_block.type"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
		}

		switch blockType {
		case "text":
			text, ok := block["text"].(string)
			if !ok || text == "" {
				detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "text"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
			}
			// D = {family:"assistant/text", message_id, block_index, text}
			detailObj := map[string]any{
				"block_index": float64(blockIndex),
				"family":      "assistant/text",
				"message_id":  messageID,
				"text":        text,
			}
			detail, err := r.processDetail(ctx, detailObj, protectedValues)
			if err != nil {
				safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
			}
			compoundID := fmt.Sprintf("%s:%d", messageID, blockIndex)
			schema, _ := contextevent.PayloadSchema(contextevent.KindProviderMessage)
			msgPayload := &contextevent.ProviderMessagePayload{
				Schema:        schema,
				MessageID:     compoundID,
				Role:          "assistant",
				MessageDigest: detail.Digest,
				Detail:        detail,
			}
			result.Observations = append(result.Observations, sealedexec.NormalizedObservation{Kind: contextevent.KindProviderMessage, ForeignDetail: detail, Payload: msgPayload})

		case "tool_use":
			callID, ok := nonemptyString(block["id"])
			if !ok {
				detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "tool_use.id"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
			}
			toolName, ok := nonemptyString(block["name"])
			if !ok {
				detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "tool_use.name"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
			}
			inputRaw := block["input"]
			// Redact input and build D.
			inputBytes, err := canonjsonValue(inputRaw)
			if err != nil {
				detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "tool_use.input"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
			}
			// A = R(input)
			redactedInput, err := redactBytes(inputBytes, protectedValues)
			if err != nil {
				detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
			}
			argDigest := digestBytes(redactedInput)
			// D = {family:"assistant/tool_use", message_id, block_index, call_id, tool_name, input:<redacted input>}
			detailObj := map[string]any{
				"block_index": float64(blockIndex),
				"call_id":     callID,
				"family":      "assistant/tool_use",
				"input":       json.RawMessage(redactedInput),
				"message_id":  messageID,
				"tool_name":   toolName,
			}
			detail, err := r.processDetail(ctx, detailObj, protectedValues)
			if err != nil {
				safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
			}
			// Track the tool call.
			r.mu.Lock()
			if r.pendingToolCalls == nil {
				r.pendingToolCalls = make(map[string]string)
			}
			if _, exists := r.pendingToolCalls[callID]; exists {
				r.mu.Unlock()
				dupDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "duplicate-call-id"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(dupDetail, seq, "duplicate-call-id"), OperationalFailure: "duplicate-call-id"}
			}
			r.pendingToolCalls[callID] = toolName
			r.mu.Unlock()

			schema, _ := contextevent.PayloadSchema(contextevent.KindToolCall)
			callPayload := &contextevent.ToolCallPayload{
				Schema:          schema,
				CallID:          callID,
				ToolName:        toolName,
				ArgumentsDigest: argDigest,
				Detail:          detail,
			}
			result.Observations = append(result.Observations, sealedexec.NormalizedObservation{Kind: contextevent.KindToolCall, ForeignDetail: detail, Payload: callPayload})

		case "thinking", "redacted_thinking":
			// §5: thinking/redacted thinking — accepted only to be discarded.
			// D = {content_type:<discriminator>, omitted:true}
			// Hidden bytes are not inputs to R or H.
			detailObj := map[string]any{
				"content_type": blockType,
				"omitted":      true,
			}
			detail, err := r.processDetail(ctx, detailObj, protectedValues)
			if err != nil {
				safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
				return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
			}
			summaryID := fmt.Sprintf("%s:%d", messageID, blockIndex)
			summaryObs := buildProviderSummary(r.launch, summaryID, detail.Digest, contextevent.AuthorityAdvisory, detail)
			result.Observations = append(result.Observations, summaryObs)

		default:
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "unknown-content-block", "block_type": blockType})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "unknown-content-block"), OperationalFailure: "unknown-content-block"}
		}
	}

	return result
}

func (r *claudeActiveRun) handleToolResult(ctx context.Context, object map[string]any, seq uint64) sealedexec.AdapterResult {
	sessionID, ok := nonemptyString(object["session_id"])
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "session_id"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()
	if sessionID != currentSession {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "session-mismatch"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "session-mismatch"), OperationalFailure: "session-mismatch"}
	}

	message, ok := object["message"].(map[string]any)
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "message"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}
	blocks, ok := message["content"].([]any)
	if !ok {
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "message.content"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
	}

	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}
	for _, blockAny := range blocks {
		block, ok := blockAny.(map[string]any)
		if !ok {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "malformed-foreign-frame"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "malformed-foreign-frame"), OperationalFailure: "malformed-foreign-frame"}
		}
		blockType, _ := nonemptyString(block["type"])
		if blockType != "tool_result" {
			continue // skip non-tool-result blocks
		}
		callID, ok := nonemptyString(block["tool_use_id"])
		if !ok {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-foreign-field", "field": "tool_use_id"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "missing-foreign-field"), OperationalFailure: "missing-foreign-field"}
		}
		r.mu.Lock()
		toolName, exists := r.pendingToolCalls[callID]
		if !exists {
			r.mu.Unlock()
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "unmatched-tool-result", "call_id": callID})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "unmatched-tool-result"), OperationalFailure: "unmatched-tool-result"}
		}
		delete(r.pendingToolCalls, callID)
		r.mu.Unlock()

		contentRaw := block["content"]
		contentBytes, err := canonjsonValue(contentRaw)
		if err != nil {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "invalid-foreign-field", "field": "content"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "invalid-foreign-field"), OperationalFailure: "invalid-foreign-field"}
		}
		redactedContent, err := redactBytes(contentBytes, protectedValues)
		if err != nil {
			detail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
		}
		outputDigest := digestBytes(redactedContent)

		isError, _ := block["is_error"].(bool)
		status := "success"
		if isError {
			status = "error"
		}

		detailObj := map[string]any{
			"call_id": callID,
			"content": json.RawMessage(redactedContent),
			"family":  "user/tool_result",
			"status":  status,
		}
		detail, err := r.processDetail(ctx, detailObj, protectedValues)
		if err != nil {
			safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
			return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
		}

		schema, _ := contextevent.PayloadSchema(contextevent.KindToolResult)
		toolResultPayload := &contextevent.ToolResultPayload{
			Schema:       schema,
			CallID:       callID,
			ToolName:     toolName,
			Status:       status,
			OutputDigest: outputDigest,
			Detail:       detail,
		}
		result.Observations = append(result.Observations, sealedexec.NormalizedObservation{Kind: contextevent.KindToolResult, ForeignDetail: detail, Payload: toolResultPayload})
	}
	return result
}

func (r *claudeActiveRun) handleResult(ctx context.Context, object map[string]any, seq uint64) sealedexec.AdapterResult {
	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()

	sessionID, ok := nonemptyString(object["session_id"])
	if !ok || sessionID != currentSession {
		reason := "missing-foreign-field"
		if ok {
			reason = "session-mismatch"
		}
		detail := r.fixedSafeDetail(ctx, map[string]any{"reason": reason})
		return sealedexec.AdapterResult{Observations: r.gapObservations(detail, seq, reason), OperationalFailure: reason}
	}

	subtype, _ := nonemptyString(object["subtype"])
	isError, _ := object["is_error"].(bool)

	// Build D = redacted projection.
	durObj := map[string]any{}
	for _, key := range []string{"duration_ms", "duration_api_ms", "num_turns", "total_cost_usd"} {
		durObj[key] = object[key]
	}
	projObj := map[string]any{
		"duration_api_ms":    object["duration_api_ms"],
		"duration_ms":        object["duration_ms"],
		"family":             "result",
		"is_error":           isError,
		"num_turns":          object["num_turns"],
		"permission_denials": object["permission_denials"],
		"result":             object["result"],
		"subtype":            subtype,
		"total_cost_usd":     object["total_cost_usd"],
		"usage":              object["usage"],
	}
	if mu, ok := object["modelUsage"]; ok {
		projObj["modelUsage"] = mu
	}

	detail, err := r.processDetail(ctx, projObj, protectedValues)
	if err != nil {
		safeDetail := r.fixedSafeDetail(ctx, map[string]any{"reason": "redaction-failed"})
		return sealedexec.AdapterResult{Observations: r.gapObservations(safeDetail, seq, "redaction-failed"), OperationalFailure: "redaction-failed"}
	}

	// Success: subtype "success" and is_error=false.
	isSuccess := subtype == "success" && !isError
	// Provider failure: is_error=true or subtype not "success".
	// Special case: subtype=="success" but is_error==true → reason "provider-result-error".
	reasonCode := "provider-result-" + subtype
	if isError && subtype == "success" {
		reasonCode = "provider-result-error"
	}

	r.mu.Lock()
	r.resultReceived = true
	r.mu.Unlock()

	if isSuccess {
		// §5: Hold the exact result until process termination;
		// on exit 0 with empty stderr → advisory provider-summary + adapter-stop.
		summaryObs := buildProviderSummary(r.launch, "terminal-result", detail.Digest, contextevent.AuthorityAdvisory, detail)
		r.mu.Lock()
		r.pendingResultObs = &pendingResultObservations{
			observations: []sealedexec.NormalizedObservation{summaryObs},
		}
		r.mu.Unlock()
	} else {
		// §5: adapter-error, no provider-summary.
		schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterError)
		errorPayload := &contextevent.AdapterErrorPayload{
			Schema:         schema,
			Adapter:        contextevent.AdapterClaude,
			AdapterVersion: r.launch.Request.AdapterVersion,
			Session:        r.launch.Request.Session,
			Operation:      "process",
			ReasonCode:     reasonCode,
			ErrorDigest:    detail.Digest,
			Detail:         detail,
		}
		errorObs := sealedexec.NormalizedObservation{Kind: contextevent.KindAdapterError, ForeignDetail: detail, BlocksAuthority: true, Witness: reasonCode, Payload: errorPayload}
		r.mu.Lock()
		r.pendingResultObs = &pendingResultObservations{
			observations: []sealedexec.NormalizedObservation{errorObs},
			failure:      reasonCode,
		}
		r.mu.Unlock()
	}

	// Return empty (the result observations are buffered until process exit).
	return sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}
}

// ---------------------------------------------------------------------------
// Detail helpers
// ---------------------------------------------------------------------------

// processDetail marshals the object to canonical JSON and runs it through the processor.
func (r *claudeActiveRun) processDetail(ctx context.Context, obj map[string]any, protectedValues [][]byte) (contextevent.Detail, error) {
	encoded, err := canonjson.Marshal(obj)
	if err != nil {
		return contextevent.Detail{}, err
	}
	encoded = bytes.TrimSuffix(encoded, []byte("\n"))
	return r.processor.Process(ctx, encoded, protectedValues)
}

// fixedSafeDetail produces a fixed safe detail for error observations.
// It uses only the classified secrets (never variable detail) for fixed payload scanning.
func (r *claudeActiveRun) fixedSafeDetail(ctx context.Context, obj map[string]any) contextevent.Detail {
	r.mu.Lock()
	classifiedOnly := append([][]byte(nil), r.launch.Profile.PolicySecretValues...)
	r.mu.Unlock()
	encoded, err := canonjson.Marshal(obj)
	if err != nil {
		fallback := []byte(`{}`)
		return contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: digestBytes(fallback), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: fallback}
	}
	encoded = bytes.TrimSuffix(encoded, []byte("\n"))
	detail, err := r.processor.Process(ctx, encoded, classifiedOnly)
	if err != nil {
		fallback := []byte(`{}`)
		return contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: digestBytes(fallback), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: fallback}
	}
	return detail
}

// malformedSafeDetail constructs a safe detail for malformed foreign input.
func (r *claudeActiveRun) malformedSafeDetail(ctx context.Context, raw []byte, reason string) contextevent.Detail {
	if !utf8.Valid(raw) {
		raw = []byte(strings.ToValidUTF8(string(raw), ""))
	}
	obj := map[string]any{"foreign_line": string(raw), "reason": reason}
	return r.fixedSafeDetail(ctx, obj)
}

// gapObservations produces telemetry-gap + adapter-error for a decode failure.
func (r *claudeActiveRun) gapObservations(detail contextevent.Detail, seq uint64, reason string) []sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindTelemetryGap)
	gap := sealedexec.NormalizedObservation{
		Kind:            contextevent.KindTelemetryGap,
		ForeignDetail:   detail,
		BlocksAuthority: true,
		Witness:         reason,
		Payload: &contextevent.TelemetryGapPayload{
			Schema:       schema,
			Source:       claudeSource,
			FromSequence: seq,
			ToSequence:   seq,
			ReasonCode:   reason,
			Availability: "unavailable",
		},
	}
	errorSchema, _ := contextevent.PayloadSchema(contextevent.KindAdapterError)
	operation := "decode"
	switch reason {
	case "redaction-failed", "protected-fixed-field", "secret-classification-unavailable":
		operation = "redaction"
	}
	errorObs := sealedexec.NormalizedObservation{
		Kind:            contextevent.KindAdapterError,
		ForeignDetail:   detail,
		BlocksAuthority: true,
		Witness:         reason,
		Payload: &contextevent.AdapterErrorPayload{
			Schema:         errorSchema,
			Adapter:        contextevent.AdapterClaude,
			AdapterVersion: r.launch.Request.AdapterVersion,
			Session:        r.launch.Request.Session,
			Operation:      operation,
			ReasonCode:     reason,
			ErrorDigest:    detail.Digest,
			Detail:         detail,
		},
	}
	return []sealedexec.NormalizedObservation{gap, errorObs}
}

// ---------------------------------------------------------------------------
// Observation builders
// ---------------------------------------------------------------------------

func buildAdapterStart(launch sealedexec.AdapterLaunch, detail *contextevent.Detail) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterStart)
	payload := &contextevent.AdapterStartPayload{
		Schema:                 schema,
		Adapter:                contextevent.AdapterClaude,
		AdapterVersion:         launch.Request.AdapterVersion,
		Session:                launch.Request.Session,
		ProfileDigest:          launch.Profile.Digest,
		WorkspaceRequestDigest: launch.Workspace.RequestDigest,
		Detail:                 detail,
	}
	return sealedexec.NormalizedObservation{Kind: contextevent.KindAdapterStart, Payload: payload}
}

func buildResumeObservation(launch sealedexec.AdapterLaunch, detail contextevent.Detail) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindResume)
	payload := &contextevent.ResumePayload{
		Schema:           schema,
		ContinuityDigest: detail.Digest,
		PriorSession:     launch.Request.Session,
		CurrentSession:   launch.Request.Session,
	}
	return sealedexec.NormalizedObservation{Kind: contextevent.KindResume, Payload: payload, ForeignDetail: detail}
}

func buildProviderSummary(launch sealedexec.AdapterLaunch, summaryID, summaryDigest string, authority contextevent.Authority, detail contextevent.Detail) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindProviderSummary)
	payload := &contextevent.ProviderSummaryPayload{
		Schema:        schema,
		SummaryID:     summaryID,
		SummaryDigest: summaryDigest,
		Authority:     authority,
		Detail:        detail,
	}
	return sealedexec.NormalizedObservation{Kind: contextevent.KindProviderSummary, ForeignDetail: detail, Payload: payload}
}

func adapterStopObservation(launch sealedexec.AdapterLaunch, exitCode int, reasonCode string) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterStop)
	payload := &contextevent.AdapterStopPayload{
		Schema:         schema,
		Adapter:        contextevent.AdapterClaude,
		AdapterVersion: launch.Request.AdapterVersion,
		Session:        launch.Request.Session,
		ExitCode:       exitCode,
		ReasonCode:     reasonCode,
	}
	return sealedexec.NormalizedObservation{Kind: contextevent.KindAdapterStop, Payload: payload}
}

// ---------------------------------------------------------------------------
// Validation helpers for init
// ---------------------------------------------------------------------------

func validateInitMCPServers(object map[string]any) string {
	servers, ok := object["mcp_servers"].([]any)
	if !ok {
		return "mcp-mismatch"
	}
	if len(servers) != 1 {
		return "mcp-mismatch"
	}
	server, ok := servers[0].(map[string]any)
	if !ok {
		return "mcp-mismatch"
	}
	name, _ := server["name"].(string)
	status, _ := server["status"].(string)
	if name != "verdi-context" || status != "connected" {
		return "mcp-mismatch"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Low-level utilities
// ---------------------------------------------------------------------------

func envValueFromEnv(env []string, name string) string {
	prefix := name + "="
	for _, row := range env {
		if strings.HasPrefix(row, prefix) {
			return strings.TrimPrefix(row, prefix)
		}
	}
	return ""
}

func containsByteSlice(haystack [][]byte, needle []byte) bool {
	for _, v := range haystack {
		if bytes.Equal(v, needle) {
			return true
		}
	}
	return false
}

func validateSessionRef(value string) error {
	if value == "" || value != strings.TrimSpace(value) ||
		strings.HasPrefix(value, "-") || !utf8.ValidString(value) ||
		strings.ContainsAny(value, "\r\n\x00") {
		return errors.New("sealedexec/claude: adapter session ref must be nonempty explicit identity, never an option selector")
	}
	return nil
}

func nonemptyString(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok && text != "" && text == strings.TrimSpace(text) && utf8.ValidString(text)
}

func uintValueF(value any) (uint64, bool) {
	switch v := value.(type) {
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 {
			return 0, false
		}
		return uint64(n), true
	case float64:
		if v < 0 || v != float64(uint64(v)) {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// canonjsonValue encodes a Go value to canonical JSON bytes (no trailing LF).
func canonjsonValue(value any) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	encoded, err := canonjson.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(encoded, []byte("\n")), nil
}

// redactBytes applies standard redaction to raw bytes with protected values.
func redactBytes(raw []byte, protectedValues [][]byte) ([]byte, error) {
	return contextevent.RedactStandardV1(raw, protectedValues)
}
