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

	// claudeProcessSource is Amendment 002 §5's telemetry-gap source for
	// process-level (as opposed to stream-level) conditions.
	claudeProcessSource = "claude-process"
)

// ProcessResult is the explicit terminal process result. Amendment 002 §5
// makes nonempty stderr always operational, so the reaped child's stderr is a
// required operand of the terminal decision. The adapter only ever hashes and
// discards these bytes; they never reach a detail.
type ProcessResult struct {
	ExitCode int
	Stderr   []byte
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

// Adapter owns the pinned argv, typed stdin, and Claude stream-json decoder
// for exactly one command. Its scoped MCP configuration is supplied by the
// owner of the scoped server's lifecycle and is never derived from HOME or
// any other environment-root fact.
type Adapter struct {
	process   Process
	processor *sealedexec.DetailProcessor
	mcpConfig MCPConfig
}

// New constructs a per-command adapter over explicit process and
// detail-processor ports and the exact scoped MCP configuration that the
// command's server lifecycle owner already started.
func New(process Process, processor *sealedexec.DetailProcessor, mcpConfig MCPConfig) (*Adapter, error) {
	if process == nil {
		return nil, errors.New("sealedexec/claude: process port is nil")
	}
	if processor == nil {
		return nil, errors.New("sealedexec/claude: detail processor is nil")
	}
	if err := validateSuppliedMCPConfig(mcpConfig); err != nil {
		return nil, err
	}
	return &Adapter{process: process, processor: processor, mcpConfig: mcpConfig}, nil
}

// validateSuppliedMCPConfig proves the supplied configuration is the scoped
// one StartScopedMCP produced: Amendment 002 §4's exact file name at a clean
// absolute path, a transport URL, and the scoped capability bearer token.
func validateSuppliedMCPConfig(config MCPConfig) error {
	if !filepath.IsAbs(config.Path) || filepath.Clean(config.Path) != config.Path ||
		filepath.Base(config.Path) != claudeMCPConfigName {
		return fmt.Errorf("sealedexec/claude: scoped MCP config path must be a clean absolute %s", claudeMCPConfigName)
	}
	if config.URL == "" {
		return errors.New("sealedexec/claude: scoped MCP config has no transport URL")
	}
	token, ok := strings.CutPrefix(config.Authorization, "Bearer ")
	if !ok || !claudeMCPDigestRE.MatchString(token) {
		return errors.New("sealedexec/claude: scoped MCP config lacks the scoped capability authorization")
	}
	return nil
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
	args, err := startArgs(launch.Profile, a.mcpConfig.Path)
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
	args, err := resumeArgs(launch.Profile, a.mcpConfig.Path, sessionRef)
	if err != nil {
		return nil, err
	}
	return a.run(ctx, launch, args, sessionRef, false)
}

func (a *Adapter) run(ctx context.Context, launch sealedexec.AdapterLaunch, args []string, expectedSession string, start bool) (sealedexec.ActiveAdapterRun, error) {
	if launch.Profile.DecoderProfile != DecoderProfileV1 || launch.Profile.AdapterVersion != launch.Request.AdapterVersion {
		return nil, errors.New("sealedexec/claude: selected profile does not bind decoder/version")
	}

	workingDir := launch.Workspace.Path
	if !filepath.IsAbs(workingDir) || filepath.Clean(workingDir) != workingDir {
		return nil, errors.New("sealedexec/claude: launch workspace path must be a clean absolute path")
	}

	// §3: Before every start or resume, probe the version with the same
	// absolute executable, environment, and working directory as the launch.
	probeCmd, _, probeCancel, err := launch.Profile.Profile.Command(ctx, launch.Profile.Executable, "--version")
	if err != nil {
		return nil, fmt.Errorf("sealedexec/claude: construct version probe: %w", err)
	}
	probeCancel()
	probeCmd.Dir = workingDir
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
	command.Dir = workingDir
	if command.Path != probeCmd.Path || command.Dir != probeCmd.Dir || !equalStrings(command.Env, probeCmd.Env) {
		cancel()
		return nil, errors.New("sealedexec/claude: launch command contradicts the proven version probe")
	}
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
	return validateClaudeEnvironment(profile)
}

// requiredClaudeEnv is Amendment 002 §3's fixed-value half of the activated
// environment table. CLAUDE_CONFIG_DIR is checked separately because its
// required value is the resolved profile's own configuration directory, and
// ANTHROPIC_API_KEY is checked by validateClaudeProfile against the
// classified secret set.
var requiredClaudeEnv = map[string]string{
	"DISABLE_AUTOUPDATER":                      "1",
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
	"CLAUDE_CODE_AUTO_CONNECT_IDE":             "false",
}

// admittedClaudeEnv is the exact set of §3 Claude-specific names. Every other
// name in a forbidden class is refused; the deterministic process baseline
// (HOME, TMPDIR, PATH, XDG roots, and similar) is admitted because §3 adds the
// Claude table to that baseline rather than replacing it.
var admittedClaudeEnv = map[string]bool{
	"ANTHROPIC_API_KEY":                        true,
	"CLAUDE_CONFIG_DIR":                        true,
	"DISABLE_AUTOUPDATER":                      true,
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true,
	"CLAUDE_CODE_AUTO_CONNECT_IDE":             true,
}

// forbiddenClaudeEnvPrefixes and forbiddenClaudeEnvNames close §3's
// "no other ... variable is admitted" sentence over its named classes. Names
// are compared case-insensitively because the proxy controls are conventionally
// spelled in both cases and the OS environment is case-sensitive.
var forbiddenClaudeEnvPrefixes = map[string]string{
	"ANTHROPIC_": "provider",
	"CLAUDE_":    "provider",
	"AWS_":       "cloud-provider",
	"AMAZON_":    "cloud-provider",
	"AZURE_":     "cloud-provider",
	"BEDROCK_":   "cloud-provider",
	"CLOUDSDK_":  "cloud-provider",
	"GCLOUD_":    "cloud-provider",
	"GCP_":       "cloud-provider",
	"GOOGLE_":    "cloud-provider",
	"VERTEX_":    "cloud-provider",
	"IDEA_":      "ide",
	"JETBRAINS_": "ide",
	"VSCODE_":    "ide",
	"OTEL_":      "telemetry-export",
}

var forbiddenClaudeEnvNames = map[string]string{
	"ALL_PROXY":       "proxy",
	"BASHOPTS":        "shell-startup",
	"BASH_ENV":        "shell-startup",
	"CLOUD_ML_REGION": "cloud-provider",
	"ENV":             "shell-startup",
	"FTP_PROXY":       "proxy",
	"HTTPS_PROXY":     "proxy",
	"HTTP_PROXY":      "proxy",
	"IFS":             "shell-startup",
	"NO_PROXY":        "proxy",
	"PROMPT_COMMAND":  "shell-startup",
	"SHELL":           "shell-startup",
	"SHELLOPTS":       "shell-startup",
	"TERM_PROGRAM":    "ide",
	"ZDOTDIR":         "shell-startup",
}

// forbiddenClaudeEnvName reports the §3 forbidden class of name, or "" when
// the name is admitted.
func forbiddenClaudeEnvName(name string) string {
	if admittedClaudeEnv[name] {
		return ""
	}
	upper := strings.ToUpper(name)
	if class, ok := forbiddenClaudeEnvNames[upper]; ok {
		return class
	}
	for prefix, class := range forbiddenClaudeEnvPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return class
		}
	}
	if strings.Contains(upper, "PLUGIN") {
		return "plugin"
	}
	if strings.Contains(upper, "HOOK") {
		return "hook"
	}
	if strings.Contains(upper, "MODEL") {
		return "model-selection"
	}
	return ""
}

// validateClaudeEnvironment closes Amendment 002 §3's activated environment
// table: every required row is present with its exact required value, and no
// name in a forbidden class is admitted. Only names ever appear in an error;
// values never do.
func validateClaudeEnvironment(profile sealedexec.ResolvedProfile) error {
	env := profile.Profile.Env()
	if profile.ClaudeConfigDir == "" || !filepath.IsAbs(profile.ClaudeConfigDir) ||
		filepath.Clean(profile.ClaudeConfigDir) != profile.ClaudeConfigDir {
		return errors.New("sealedexec/claude: resolved profile carries no clean absolute Claude configuration directory")
	}
	if envValueFromEnv(env, "CLAUDE_CONFIG_DIR") != profile.ClaudeConfigDir {
		return errors.New("sealedexec/claude: activated CLAUDE_CONFIG_DIR is not the resolved Claude configuration directory")
	}
	for _, name := range []string{"HOME", "TMPDIR"} {
		value := envValueFromEnv(env, name)
		if value == "" || !filepath.IsAbs(value) || filepath.Clean(value) != value {
			return fmt.Errorf("sealedexec/claude: activated %s must be a clean absolute path", name)
		}
	}
	path := envValueFromEnv(env, "PATH")
	if path == "" {
		return errors.New("sealedexec/claude: activated PATH is absent or empty")
	}
	for _, entry := range strings.Split(path, string(filepath.ListSeparator)) {
		if entry == "" || !filepath.IsAbs(entry) || filepath.Clean(entry) != entry {
			return errors.New("sealedexec/claude: activated PATH is not a deterministic list of clean absolute directories")
		}
	}
	for name, want := range requiredClaudeEnv {
		if envValueFromEnv(env, name) != want {
			return fmt.Errorf("sealedexec/claude: activated %s does not equal its required value", name)
		}
	}
	for _, row := range env {
		name, _, ok := strings.Cut(row, "=")
		if !ok {
			return errors.New("sealedexec/claude: activated environment row is not a NAME=VALUE entry")
		}
		if class := forbiddenClaudeEnvName(name); class != "" {
			return fmt.Errorf("sealedexec/claude: activated environment admits forbidden %s variable %s", class, name)
		}
	}
	return nil
}

// startArgs is Amendment 002 §4's normative start order. The model is the
// resolved profile's exact full identifier: the logical profile name is never
// a model substitute, and a moving alias is refused upstream by the
// controller material.
func startArgs(profile sealedexec.ResolvedProfile, mcpConfigPath string) ([]string, error) {
	model := profile.Model
	if model == "" {
		return nil, errors.New("sealedexec/claude: resolved profile carries no Claude model")
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

func resumeArgs(profile sealedexec.ResolvedProfile, mcpConfigPath, sessionRef string) ([]string, error) {
	args, err := startArgs(profile, mcpConfigPath)
	if err != nil {
		return nil, err
	}
	return append(args, "--resume", sessionRef), nil
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

	// Stream-identity uniqueness within the active provider session.
	seenMessageIDs   map[string]bool
	pendingToolCalls map[string]string // call_id -> tool_name, still unmatched
	closedToolCalls  map[string]bool   // call_id already answered exactly once

	// Foreign sequence counter (1-based)
	foreignSeq uint64

	// Terminal buffering
	pendingResult *pendingTerminal

	// Stop machinery
	terminal      bool
	stopOnce      sync.Once
	stopDone      chan struct{}
	stopRequested bool
	stopDelivered bool
	stopResult    sealedexec.AdapterStopResult
	stopErr       error
}

// pendingTerminal buffers the exact terminal result until the child is reaped.
// reason is empty for the success family and the closed provider-result reason
// for the provider-failure family.
type pendingTerminal struct {
	seq          uint64
	observations []sealedexec.NormalizedObservation
	reason       string
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

	// A record whose LF framing never completed is not a decodable frame; §5's
	// closed reason set reduces it to malformed-foreign-frame.
	if !item.Complete {
		return r.rawFailure(ctx, seq, "malformed-foreign-frame", item.ForeignJSON)
	}

	return r.normalize(ctx, item.ForeignJSON, seq)
}

// handleProcessTerminal applies Amendment 002 §5's terminal precedence over the
// reaped child. A success result is admitted only by exit zero, empty stderr,
// and a fully matched tool-call set.
func (r *claudeActiveRun) handleProcessTerminal(ctx context.Context, proc *ProcessResult) (sealedexec.AdapterResult, error) {
	r.mu.Lock()
	pending := r.pendingResult
	incomplete := len(r.pendingToolCalls) != 0
	// §5: process-level gaps range over the terminal result's sequence, or the
	// next foreign sequence when no result exists (EOF uses the next line).
	gapSeq := uint64(0)
	if pending != nil {
		gapSeq = pending.seq
	} else {
		r.foreignSeq++
		gapSeq = r.foreignSeq
	}
	r.mu.Unlock()

	result := sealedexec.AdapterResult{
		Observations: []sealedexec.NormalizedObservation{},
		Terminal:     &sealedexec.AdapterTerminalResult{ExitCode: proc.ExitCode},
	}

	// §5 terminal precedence. No lower-priority terminal event is also emitted.
	switch {
	case len(proc.Stderr) != 0:
		// Stderr is hashed while read and discarded; only the fixed digest
		// detail may be emitted, never the bytes.
		detail, err := r.rawSafeDetail(ctx, proc.Stderr, "provider-stderr")
		if err != nil {
			return sealedexec.AdapterResult{}, err
		}
		return r.terminalFailure(result, detail, gapSeq, "provider-stderr", proc.ExitCode), nil

	case incomplete:
		detail, err := r.fixedSafeDetail(ctx, map[string]any{"reason": "incomplete-tool-call"})
		if err != nil {
			return sealedexec.AdapterResult{}, err
		}
		return r.terminalFailure(result, detail, gapSeq, "incomplete-tool-call", proc.ExitCode), nil

	case pending == nil:
		detail, err := r.fixedSafeDetail(ctx, map[string]any{"reason": "missing-terminal-result"})
		if err != nil {
			return sealedexec.AdapterResult{}, err
		}
		return r.terminalFailure(result, detail, gapSeq, "missing-terminal-result", proc.ExitCode), nil

	case pending.reason != "":
		// Provider-declared failure: adapter-error over the exact result
		// detail, then adapter-stop with the actual exit. No gap is defined.
		result.Observations = append(result.Observations, pending.observations...)
		result.Observations = append(result.Observations, adapterStopObservation(r.launch, proc.ExitCode, pending.reason))
		result.OperationalFailure = pending.reason
		return result, nil

	case proc.ExitCode != 0:
		detail, err := r.fixedSafeDetail(ctx, map[string]any{"exit_code": proc.ExitCode, "reason": "provider-exit-nonzero"})
		if err != nil {
			return sealedexec.AdapterResult{}, err
		}
		return r.terminalFailure(result, detail, gapSeq, "provider-exit-nonzero", proc.ExitCode), nil
	}

	result.Observations = append(result.Observations, pending.observations...)
	result.Observations = append(result.Observations, adapterStopObservation(r.launch, 0, "completed"))
	return result, nil
}

// terminalFailure emits the fixed process gap, adapter-error, and adapter-stop
// for one closed terminal reason and discards every lower-priority event.
func (r *claudeActiveRun) terminalFailure(result sealedexec.AdapterResult, detail contextevent.Detail, seq uint64, reason string, exitCode int) sealedexec.AdapterResult {
	result.Observations = append(result.Observations, r.gapObservations(detail, seq, reason, claudeProcessSource)...)
	result.Observations = append(result.Observations, adapterStopObservation(r.launch, exitCode, reason))
	result.OperationalFailure = reason
	return result
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
// Closed stream grammar
// ---------------------------------------------------------------------------

// The exact accepted-key sets of Amendment 002 §5. Every family is decoded
// with DisallowUnknownFields into these types; required keys are pointers so an
// absent key is distinguishable from a present zero value. No family is ever
// accepted through a map-shaped read.

type claudeMCPRow struct {
	Name   *string `json:"name"`
	Status *string `json:"status"`
}

type claudeInitFrame struct {
	Type              *string            `json:"type"`
	Subtype           *string            `json:"subtype"`
	SessionID         *string            `json:"session_id"`
	Model             *string            `json:"model"`
	MCPServers        *[]claudeMCPRow    `json:"mcp_servers"`
	CWD               *string            `json:"cwd"`
	Tools             *[]string          `json:"tools"`
	PermissionMode    *string            `json:"permissionMode"`
	APIKeySource      *string            `json:"apiKeySource"`
	ClaudeCodeVersion *string            `json:"claude_code_version"`
	SlashCommands     *[]string          `json:"slash_commands"`
	OutputStyle       *string            `json:"output_style"`
	Agents            *[]string          `json:"agents"`
	Skills            *[]string          `json:"skills"`
	Plugins           *[]json.RawMessage `json:"plugins"`
	UUID              *string            `json:"uuid"`
}

type claudeRetryError struct {
	Type    *string `json:"type"`
	Message *string `json:"message"`
}

type claudeRetryFrame struct {
	Type         *string           `json:"type"`
	Subtype      *string           `json:"subtype"`
	Attempt      *uint64           `json:"attempt"`
	MaxRetries   *uint64           `json:"max_retries"`
	RetryDelayMS *uint64           `json:"retry_delay_ms"`
	Error        *claudeRetryError `json:"error"`
	UUID         *string           `json:"uuid"`
	SessionID    *string           `json:"session_id"`
}

type claudeUsage struct {
	InputTokens              *uint64 `json:"input_tokens"`
	CacheCreationInputTokens *uint64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *uint64 `json:"cache_read_input_tokens"`
	OutputTokens             *uint64 `json:"output_tokens"`
	ServiceTier              *string `json:"service_tier"`
}

type claudeAssistantMessage struct {
	ID           *string            `json:"id"`
	Type         *string            `json:"type"`
	Role         *string            `json:"role"`
	Model        *string            `json:"model"`
	Content      *[]json.RawMessage `json:"content"`
	Usage        *json.RawMessage   `json:"usage"`
	StopReason   *string            `json:"stop_reason"`
	StopSequence *string            `json:"stop_sequence"`
}

type claudeAssistantFrame struct {
	Type            *string                 `json:"type"`
	SessionID       *string                 `json:"session_id"`
	UUID            *string                 `json:"uuid"`
	Message         *claudeAssistantMessage `json:"message"`
	ParentToolUseID *string                 `json:"parent_tool_use_id"`
}

type claudeUserMessage struct {
	Role    *string            `json:"role"`
	Content *[]json.RawMessage `json:"content"`
}

type claudeUserFrame struct {
	Type            *string            `json:"type"`
	SessionID       *string            `json:"session_id"`
	UUID            *string            `json:"uuid"`
	Message         *claudeUserMessage `json:"message"`
	ParentToolUseID *string            `json:"parent_tool_use_id"`
	ToolUseResult   *json.RawMessage   `json:"tool_use_result"`
}

type claudeResultFrame struct {
	Type              *string          `json:"type"`
	Subtype           *string          `json:"subtype"`
	IsError           *bool            `json:"is_error"`
	Result            *string          `json:"result"`
	SessionID         *string          `json:"session_id"`
	UUID              *string          `json:"uuid"`
	DurationMS        *uint64          `json:"duration_ms"`
	DurationAPIMS     *uint64          `json:"duration_api_ms"`
	NumTurns          *uint64          `json:"num_turns"`
	TotalCostUSD      *json.RawMessage `json:"total_cost_usd"`
	Usage             *json.RawMessage `json:"usage"`
	PermissionDenials *json.RawMessage `json:"permission_denials"`
	ModelUsage        *json.RawMessage `json:"modelUsage"`
}

type claudePermissionDenial struct {
	ToolName  *string          `json:"tool_name"`
	ToolUseID *string          `json:"tool_use_id"`
	ToolInput *json.RawMessage `json:"tool_input"`
}

type claudeBlockDiscriminator struct {
	Type *string `json:"type"`
}

type claudeTextBlock struct {
	Type *string `json:"type"`
	Text *string `json:"text"`
}

type claudeToolUseBlock struct {
	Type  *string          `json:"type"`
	ID    *string          `json:"id"`
	Name  *string          `json:"name"`
	Input *json.RawMessage `json:"input"`
}

type claudeThinkingBlock struct {
	Type      *string `json:"type"`
	Thinking  *string `json:"thinking"`
	Signature *string `json:"signature"`
}

type claudeRedactedThinkingBlock struct {
	Type *string `json:"type"`
	Data *string `json:"data"`
}

type claudeToolResultBlock struct {
	Type      *string          `json:"type"`
	ToolUseID *string          `json:"tool_use_id"`
	Content   *json.RawMessage `json:"content"`
	IsError   *bool            `json:"is_error"`
}

// strictDecodeReason decodes raw into target with unknown-field rejection and
// trailing-data rejection. It returns "" on success or the closed §5 decode
// reason that describes the refusal.
func strictDecodeReason(raw []byte, target any) string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return "unknown-foreign-field"
		}
		return "invalid-foreign-field"
	}
	if decoder.More() {
		return "malformed-foreign-frame"
	}
	return ""
}

// validUniqueStrings proves a non-null array of unique nonempty UTF-8 strings.
func validUniqueStrings(values *[]string) bool {
	if values == nil {
		return false
	}
	seen := make(map[string]struct{}, len(*values))
	for _, value := range *values {
		if value == "" || !utf8.ValidString(value) {
			return false
		}
		if _, duplicate := seen[value]; duplicate {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

// validateUsage proves §5's exact nonnegative-integer usage object.
func validateUsage(raw *json.RawMessage) string {
	if raw == nil {
		return "missing-foreign-field"
	}
	var usage claudeUsage
	if reason := strictDecodeReason(*raw, &usage); reason != "" {
		return reason
	}
	if usage.InputTokens == nil || usage.CacheCreationInputTokens == nil ||
		usage.CacheReadInputTokens == nil || usage.OutputTokens == nil {
		return "missing-foreign-field"
	}
	if usage.ServiceTier != nil {
		switch *usage.ServiceTier {
		case "standard", "priority", "batch":
		default:
			return "invalid-foreign-field"
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Normalization
// ---------------------------------------------------------------------------

func (r *claudeActiveRun) normalize(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	if len(line) == 0 {
		return r.rawFailure(ctx, seq, "empty-foreign-record", line)
	}

	// Unique-key strict JSON is the only admissible frame shape. The decoded
	// object is used solely to read the type/subtype discriminators; every
	// acceptance below is a closed struct decode of the same bytes.
	object, err := sealedexec.DecodeUniqueJSONObject(line)
	if err != nil {
		return r.rawFailure(ctx, seq, "malformed-foreign-frame", line)
	}

	outer, ok := nonemptyString(object["type"])
	if !ok {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "type"})
	}
	subtype, _ := nonemptyString(object["subtype"])

	r.mu.Lock()
	initReceived := r.initReceived
	resultReceived := r.resultReceived
	r.mu.Unlock()

	if outer == "result" && !initReceived {
		return r.decodeFailure(ctx, seq, "result-before-init", nil)
	}
	if resultReceived {
		return r.decodeFailure(ctx, seq, "observation-after-result", nil)
	}

	switch {
	case outer == "system" && subtype == "init":
		if initReceived {
			return r.decodeFailure(ctx, seq, "duplicate-init", nil)
		}
		return r.handleInit(ctx, line, seq)
	case outer == "system" && subtype == "api_retry":
		return r.handleRetry(ctx, line, seq)
	case outer == "assistant" && subtype == "":
		return r.handleAssistant(ctx, line, seq)
	case outer == "user" && subtype == "":
		return r.handleToolResult(ctx, line, seq)
	case outer == "result":
		return r.handleResult(ctx, line, seq)
	}
	return r.decodeFailure(ctx, seq, "unknown-foreign-family", nil)
}

// ---------------------------------------------------------------------------
// Family handlers
// ---------------------------------------------------------------------------

func (r *claudeActiveRun) handleInit(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	var frame claudeInitFrame
	if reason := strictDecodeReason(line, &frame); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"family": "system/init"})
	}
	if frame.Type == nil || frame.Subtype == nil || frame.SessionID == nil || frame.Model == nil ||
		frame.MCPServers == nil || frame.CWD == nil || frame.Tools == nil || frame.PermissionMode == nil ||
		frame.APIKeySource == nil || frame.ClaudeCodeVersion == nil || frame.SlashCommands == nil ||
		frame.OutputStyle == nil || frame.Agents == nil || frame.Skills == nil || frame.Plugins == nil ||
		frame.UUID == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "system/init"})
	}
	if *frame.Type != "system" || *frame.Subtype != "init" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "type"})
	}

	sessionID := *frame.SessionID
	if !nonemptyStringValue(sessionID) || !nonemptyStringValue(*frame.UUID) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "session_id"})
	}
	if *frame.Model != r.launch.Profile.Model {
		return r.decodeFailure(ctx, seq, "model-mismatch", nil)
	}
	if reason := validateInitMCPServers(*frame.MCPServers); reason != "" {
		return r.decodeFailure(ctx, seq, reason, nil)
	}
	if *frame.CWD != r.launch.Workspace.Path {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "cwd"})
	}
	if *frame.ClaudeCodeVersion != r.launch.Request.AdapterVersion {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "claude_code_version"})
	}
	if *frame.PermissionMode != "bypassPermissions" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "permissionMode"})
	}
	if *frame.APIKeySource != "ANTHROPIC_API_KEY" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "apiKeySource"})
	}
	if *frame.OutputStyle != "default" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "output_style"})
	}
	if !validUniqueStrings(frame.Tools) || !validUniqueStrings(frame.SlashCommands) ||
		!validUniqueStrings(frame.Agents) || !validUniqueStrings(frame.Skills) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "tools"})
	}
	if len(*frame.Plugins) != 0 {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "plugins"})
	}
	if !r.start && sessionID != r.expectedSession {
		return r.decodeFailure(ctx, seq, "session-mismatch", nil)
	}

	// §5: extract the provider session and protect it before I is redacted.
	r.mu.Lock()
	r.initReceived = true
	r.providerSession = sessionID
	r.protectedValues = append(r.protectedValues, []byte(sessionID))
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()

	initDetailSource := map[string]any{
		"family":          "system/init",
		"mcp_servers":     []map[string]string{{"name": "verdi-context", "status": "connected"}},
		"model":           *frame.Model,
		"permission_mode": *frame.PermissionMode,
		"session_id":      sessionID,
	}
	detail, err := r.processDetail(ctx, initDetailSource, protectedValues)
	if err != nil {
		return r.decodeFailure(ctx, seq, "redaction-failed", nil)
	}

	// Amendment 002 §7 and the Codex ruling reserve the resume observation for
	// the acknowledged-prefix owner. The adapter never invents one.
	return sealedexec.AdapterResult{
		ObservedSessionRef: sessionID,
		Observations: []sealedexec.NormalizedObservation{
			buildAdapterStart(r.launch, nil),
			buildProviderSummary(r.launch, "system/init", detail.Digest, contextevent.AuthorityAdvisory, detail),
		},
	}, nil
}

func (r *claudeActiveRun) handleRetry(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	var frame claudeRetryFrame
	if reason := strictDecodeReason(line, &frame); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"family": "system/api_retry"})
	}
	if frame.Type == nil || frame.Subtype == nil || frame.Attempt == nil || frame.MaxRetries == nil ||
		frame.RetryDelayMS == nil || frame.Error == nil || frame.UUID == nil || frame.SessionID == nil ||
		frame.Error.Type == nil || frame.Error.Message == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "system/api_retry"})
	}
	if *frame.Type != "system" || *frame.Subtype != "api_retry" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "type"})
	}
	if *frame.Attempt == 0 || *frame.MaxRetries == 0 || *frame.Attempt > *frame.MaxRetries ||
		!nonemptyStringValue(*frame.UUID) || !utf8.ValidString(*frame.Error.Message) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"family": "system/api_retry"})
	}
	switch *frame.Error.Type {
	case "authentication", "billing", "rate_limit", "server", "network", "unknown":
	default:
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "error.type"})
	}

	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()
	if *frame.SessionID != currentSession {
		return r.decodeFailure(ctx, seq, "session-mismatch", nil)
	}

	retryDetailSource := map[string]any{
		"attempt":        *frame.Attempt,
		"error_category": *frame.Error.Type,
		"family":         "system/api_retry",
		"max_retries":    *frame.MaxRetries,
		"retry_delay_ms": *frame.RetryDelayMS,
		"session_id":     *frame.SessionID,
		"uuid":           *frame.UUID,
	}
	detail, err := r.processDetail(ctx, retryDetailSource, protectedValues)
	if err != nil {
		return r.decodeFailure(ctx, seq, "redaction-failed", nil)
	}

	schema, _ := contextevent.PayloadSchema(contextevent.KindRetry)
	retryObs := sealedexec.NormalizedObservation{
		Kind: contextevent.KindRetry,
		Payload: &contextevent.RetryPayload{
			Schema:           schema,
			ReasonCode:       "provider-api-" + *frame.Error.Type,
			PriorSession:     currentSession,
			NextSession:      currentSession,
			ContinuityDigest: detail.Digest,
		},
		ForeignDetail: detail,
	}
	summaryObs := buildProviderSummary(r.launch, fmt.Sprintf("api-retry/%d", *frame.Attempt), detail.Digest, contextevent.AuthorityAdvisory, detail)
	return sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{retryObs, summaryObs}}, nil
}

func (r *claudeActiveRun) handleAssistant(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	var frame claudeAssistantFrame
	if reason := strictDecodeReason(line, &frame); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"family": "assistant"})
	}
	if frame.Type == nil || frame.SessionID == nil || frame.UUID == nil || frame.Message == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "assistant"})
	}
	message := frame.Message
	if message.ID == nil || message.Type == nil || message.Role == nil || message.Model == nil ||
		message.Content == nil || message.Usage == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "assistant"})
	}
	if *frame.Type != "assistant" || *message.Type != "message" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "type"})
	}
	if *message.Role != "assistant" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "message.role"})
	}
	if !nonemptyStringValue(*message.ID) || !nonemptyStringValue(*frame.UUID) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "message.id"})
	}
	if *message.Model != r.launch.Profile.Model {
		return r.decodeFailure(ctx, seq, "model-mismatch", nil)
	}
	if reason := validateUsage(message.Usage); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "message.usage"})
	}

	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	duplicateMessage := r.seenMessageIDs[*message.ID]
	if !duplicateMessage {
		if r.seenMessageIDs == nil {
			r.seenMessageIDs = make(map[string]bool)
		}
		r.seenMessageIDs[*message.ID] = true
	}
	r.mu.Unlock()

	if *frame.SessionID != currentSession {
		return r.decodeFailure(ctx, seq, "session-mismatch", nil)
	}
	// §5: a repeated provider message id is a stream contradiction.
	if duplicateMessage {
		return r.decodeFailure(ctx, seq, "duplicate-message-id", nil)
	}

	messageID := *message.ID
	observations := []sealedexec.NormalizedObservation{}
	for blockIndex, rawBlock := range *message.Content {
		var discriminator claudeBlockDiscriminator
		if err := json.Unmarshal(rawBlock, &discriminator); err != nil || discriminator.Type == nil {
			return r.decodeFailure(ctx, seq, "unknown-content-block", nil)
		}
		switch *discriminator.Type {
		case "text":
			var block claudeTextBlock
			if reason := strictDecodeReason(rawBlock, &block); reason != "" {
				return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "content.text"})
			}
			if block.Text == nil {
				return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "content.text"})
			}
			if *block.Text == "" || !utf8.ValidString(*block.Text) {
				return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "content.text"})
			}
			detail, err := r.processDetail(ctx, map[string]any{
				"block_index": float64(blockIndex),
				"family":      "assistant/text",
				"message_id":  messageID,
				"text":        *block.Text,
			}, protectedValues)
			if err != nil {
				return r.decodeFailure(ctx, seq, "redaction-failed", nil)
			}
			schema, _ := contextevent.PayloadSchema(contextevent.KindProviderMessage)
			observations = append(observations, sealedexec.NormalizedObservation{
				Kind:          contextevent.KindProviderMessage,
				ForeignDetail: detail,
				Payload: &contextevent.ProviderMessagePayload{
					Schema:        schema,
					MessageID:     fmt.Sprintf("%s:%d", messageID, blockIndex),
					Role:          "assistant",
					MessageDigest: detail.Digest,
					Detail:        detail,
				},
			})

		case "tool_use":
			var block claudeToolUseBlock
			if reason := strictDecodeReason(rawBlock, &block); reason != "" {
				return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "content.tool_use"})
			}
			if block.ID == nil || block.Name == nil || block.Input == nil {
				return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "content.tool_use"})
			}
			if !nonemptyStringValue(*block.ID) || !nonemptyStringValue(*block.Name) {
				return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "content.tool_use"})
			}
			inputBytes, err := canonjsonValue(json.RawMessage(*block.Input))
			if err != nil {
				return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "content.tool_use.input"})
			}
			redactedInput, err := redactBytes(inputBytes, protectedValues)
			if err != nil {
				return r.decodeFailure(ctx, seq, "redaction-failed", nil)
			}
			detail, err := r.processDetail(ctx, map[string]any{
				"block_index": float64(blockIndex),
				"call_id":     *block.ID,
				"family":      "assistant/tool_use",
				"input":       json.RawMessage(redactedInput),
				"message_id":  messageID,
				"tool_name":   *block.Name,
			}, protectedValues)
			if err != nil {
				return r.decodeFailure(ctx, seq, "redaction-failed", nil)
			}
			r.mu.Lock()
			if r.pendingToolCalls == nil {
				r.pendingToolCalls = make(map[string]string)
			}
			_, openCall := r.pendingToolCalls[*block.ID]
			duplicateCall := openCall || r.closedToolCalls[*block.ID]
			if !duplicateCall {
				r.pendingToolCalls[*block.ID] = *block.Name
			}
			r.mu.Unlock()
			if duplicateCall {
				return r.decodeFailure(ctx, seq, "duplicate-call-id", nil)
			}
			schema, _ := contextevent.PayloadSchema(contextevent.KindToolCall)
			observations = append(observations, sealedexec.NormalizedObservation{
				Kind:          contextevent.KindToolCall,
				ForeignDetail: detail,
				Payload: &contextevent.ToolCallPayload{
					Schema:          schema,
					CallID:          *block.ID,
					ToolName:        *block.Name,
					ArgumentsDigest: digestBytes(redactedInput),
					Detail:          detail,
				},
			})

		case "thinking":
			var block claudeThinkingBlock
			if reason := strictDecodeReason(rawBlock, &block); reason != "" {
				return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "content.thinking"})
			}
			if block.Thinking == nil || block.Signature == nil {
				return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "content.thinking"})
			}
			observation, err := r.omissionSummary(ctx, "thinking", messageID, blockIndex, protectedValues)
			if err != nil {
				return r.decodeFailure(ctx, seq, "redaction-failed", nil)
			}
			observations = append(observations, observation)

		case "redacted_thinking":
			var block claudeRedactedThinkingBlock
			if reason := strictDecodeReason(rawBlock, &block); reason != "" {
				return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "content.redacted_thinking"})
			}
			if block.Data == nil {
				return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "content.redacted_thinking"})
			}
			observation, err := r.omissionSummary(ctx, "redacted_thinking", messageID, blockIndex, protectedValues)
			if err != nil {
				return r.decodeFailure(ctx, seq, "redaction-failed", nil)
			}
			observations = append(observations, observation)

		default:
			return r.decodeFailure(ctx, seq, "unknown-content-block", nil)
		}
	}
	return sealedexec.AdapterResult{Observations: observations}, nil
}

// omissionSummary builds the fixed hidden-content omission summary. Hidden
// bytes are never inputs to redaction or the digest.
func (r *claudeActiveRun) omissionSummary(ctx context.Context, contentType, messageID string, blockIndex int, protectedValues [][]byte) (sealedexec.NormalizedObservation, error) {
	detail, err := r.processDetail(ctx, map[string]any{
		"content_type": contentType,
		"omitted":      true,
	}, protectedValues)
	if err != nil {
		return sealedexec.NormalizedObservation{}, err
	}
	return buildProviderSummary(r.launch, fmt.Sprintf("%s:%d", messageID, blockIndex), detail.Digest, contextevent.AuthorityAdvisory, detail), nil
}

func (r *claudeActiveRun) handleToolResult(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	var frame claudeUserFrame
	if reason := strictDecodeReason(line, &frame); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"family": "user"})
	}
	if frame.Type == nil || frame.SessionID == nil || frame.UUID == nil || frame.Message == nil ||
		frame.Message.Role == nil || frame.Message.Content == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "user"})
	}
	if *frame.Type != "user" || *frame.Message.Role != "user" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "message.role"})
	}
	if !nonemptyStringValue(*frame.UUID) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "uuid"})
	}

	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()
	if *frame.SessionID != currentSession {
		return r.decodeFailure(ctx, seq, "session-mismatch", nil)
	}

	observations := []sealedexec.NormalizedObservation{}
	for _, rawBlock := range *frame.Message.Content {
		var discriminator claudeBlockDiscriminator
		if err := json.Unmarshal(rawBlock, &discriminator); err != nil || discriminator.Type == nil {
			return r.decodeFailure(ctx, seq, "unknown-content-block", nil)
		}
		// §5's closed user grammar admits tool_result blocks only.
		if *discriminator.Type != "tool_result" {
			return r.decodeFailure(ctx, seq, "unknown-content-block", nil)
		}
		var block claudeToolResultBlock
		if reason := strictDecodeReason(rawBlock, &block); reason != "" {
			return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "content.tool_result"})
		}
		if block.ToolUseID == nil || block.Content == nil {
			return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "content.tool_result"})
		}
		callID := *block.ToolUseID
		if !nonemptyStringValue(callID) {
			return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "content.tool_result"})
		}

		r.mu.Lock()
		toolName, open := r.pendingToolCalls[callID]
		alreadyClosed := r.closedToolCalls[callID]
		if open {
			delete(r.pendingToolCalls, callID)
			if r.closedToolCalls == nil {
				r.closedToolCalls = make(map[string]bool)
			}
			r.closedToolCalls[callID] = true
		}
		r.mu.Unlock()

		if !open {
			if alreadyClosed {
				return r.decodeFailure(ctx, seq, "duplicate-tool-result", nil)
			}
			return r.decodeFailure(ctx, seq, "unmatched-tool-result", nil)
		}

		contentBytes, err := canonjsonValue(json.RawMessage(*block.Content))
		if err != nil {
			return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "content.tool_result.content"})
		}
		redactedContent, err := redactBytes(contentBytes, protectedValues)
		if err != nil {
			return r.decodeFailure(ctx, seq, "redaction-failed", nil)
		}
		status := "success"
		if block.IsError != nil && *block.IsError {
			status = "error"
		}
		detail, err := r.processDetail(ctx, map[string]any{
			"call_id": callID,
			"content": json.RawMessage(redactedContent),
			"family":  "user/tool_result",
			"status":  status,
		}, protectedValues)
		if err != nil {
			return r.decodeFailure(ctx, seq, "redaction-failed", nil)
		}
		schema, _ := contextevent.PayloadSchema(contextevent.KindToolResult)
		observations = append(observations, sealedexec.NormalizedObservation{
			Kind:          contextevent.KindToolResult,
			ForeignDetail: detail,
			Payload: &contextevent.ToolResultPayload{
				Schema:       schema,
				CallID:       callID,
				ToolName:     toolName,
				Status:       status,
				OutputDigest: digestBytes(redactedContent),
				Detail:       detail,
			},
		})
	}
	return sealedexec.AdapterResult{Observations: observations}, nil
}

func (r *claudeActiveRun) handleResult(ctx context.Context, line []byte, seq uint64) (sealedexec.AdapterResult, error) {
	var frame claudeResultFrame
	if reason := strictDecodeReason(line, &frame); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"family": "result"})
	}
	if frame.Type == nil || frame.Subtype == nil || frame.IsError == nil || frame.Result == nil ||
		frame.SessionID == nil || frame.UUID == nil || frame.DurationMS == nil || frame.DurationAPIMS == nil ||
		frame.NumTurns == nil || frame.TotalCostUSD == nil || frame.Usage == nil || frame.PermissionDenials == nil {
		return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"family": "result"})
	}
	if *frame.Type != "result" {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "type"})
	}
	switch *frame.Subtype {
	case "success", "error_max_turns", "error_during_execution", "error_max_budget_usd", "error_max_structured_output_retries":
	default:
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "subtype"})
	}
	if !nonemptyStringValue(*frame.UUID) || !utf8.ValidString(*frame.Result) {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "uuid"})
	}
	var totalCost float64
	if err := json.Unmarshal(*frame.TotalCostUSD, &totalCost); err != nil || totalCost < 0 {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "total_cost_usd"})
	}
	if reason := validateUsage(frame.Usage); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "usage"})
	}
	var denials []claudePermissionDenial
	if reason := strictDecodeReason(*frame.PermissionDenials, &denials); reason != "" {
		return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "permission_denials"})
	}
	if denials == nil {
		return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "permission_denials"})
	}
	for _, denial := range denials {
		if denial.ToolName == nil || denial.ToolUseID == nil || denial.ToolInput == nil {
			return r.decodeFailure(ctx, seq, "missing-foreign-field", map[string]any{"field": "permission_denials"})
		}
	}
	if frame.ModelUsage != nil {
		var modelUsage map[string]json.RawMessage
		if reason := strictDecodeReason(*frame.ModelUsage, &modelUsage); reason != "" {
			return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "modelUsage"})
		}
		usage, sole := modelUsage[r.launch.Profile.Model]
		if len(modelUsage) != 1 || !sole {
			return r.decodeFailure(ctx, seq, "invalid-foreign-field", map[string]any{"field": "modelUsage"})
		}
		if reason := validateUsage(&usage); reason != "" {
			return r.decodeFailure(ctx, seq, reason, map[string]any{"field": "modelUsage"})
		}
	}

	r.mu.Lock()
	currentSession := r.providerSession
	protectedValues := append([][]byte(nil), r.protectedValues...)
	r.mu.Unlock()
	if *frame.SessionID != currentSession {
		return r.decodeFailure(ctx, seq, "session-mismatch", nil)
	}

	projection := map[string]any{
		"duration_api_ms":    *frame.DurationAPIMS,
		"duration_ms":        *frame.DurationMS,
		"family":             "result",
		"is_error":           *frame.IsError,
		"num_turns":          *frame.NumTurns,
		"permission_denials": json.RawMessage(*frame.PermissionDenials),
		"result":             *frame.Result,
		"subtype":            *frame.Subtype,
		"total_cost_usd":     json.RawMessage(*frame.TotalCostUSD),
		"usage":              json.RawMessage(*frame.Usage),
	}
	if frame.ModelUsage != nil {
		projection["modelUsage"] = json.RawMessage(*frame.ModelUsage)
	}
	detail, err := r.processDetail(ctx, projection, protectedValues)
	if err != nil {
		return r.decodeFailure(ctx, seq, "redaction-failed", nil)
	}

	r.mu.Lock()
	r.resultReceived = true
	r.mu.Unlock()

	if *frame.Subtype == "success" && !*frame.IsError {
		r.mu.Lock()
		r.pendingResult = &pendingTerminal{
			seq: seq,
			observations: []sealedexec.NormalizedObservation{
				buildProviderSummary(r.launch, "terminal-result", detail.Digest, contextevent.AuthorityAdvisory, detail),
			},
		}
		r.mu.Unlock()
		return sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}, nil
	}

	reasonCode := "provider-result-" + *frame.Subtype
	if *frame.IsError && *frame.Subtype == "success" {
		reasonCode = "provider-result-error"
	}
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterError)
	errorObs := sealedexec.NormalizedObservation{
		Kind:            contextevent.KindAdapterError,
		ForeignDetail:   detail,
		BlocksAuthority: true,
		Witness:         reasonCode,
		Payload: &contextevent.AdapterErrorPayload{
			Schema:         schema,
			Adapter:        contextevent.AdapterClaude,
			AdapterVersion: r.launch.Request.AdapterVersion,
			Session:        r.launch.Request.Session,
			Operation:      "process",
			ReasonCode:     reasonCode,
			ErrorDigest:    detail.Digest,
			Detail:         detail,
		},
	}
	r.mu.Lock()
	r.pendingResult = &pendingTerminal{seq: seq, observations: []sealedexec.NormalizedObservation{errorObs}, reason: reasonCode}
	r.mu.Unlock()
	return sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}, nil
}

// ---------------------------------------------------------------------------
// Detail helpers
// ---------------------------------------------------------------------------

// processDetail marshals the exact detail source to canonical JSON and runs it
// through the shared processor.
func (r *claudeActiveRun) processDetail(ctx context.Context, source map[string]any, protectedValues [][]byte) (contextevent.Detail, error) {
	encoded, err := canonjson.Marshal(source)
	if err != nil {
		return contextevent.Detail{}, err
	}
	return r.processor.Process(ctx, bytes.TrimSuffix(encoded, []byte("\n")), protectedValues)
}

// fixedSafeDetail builds a fixed safe detail from classified secrets only. A
// failure to represent it is a lower-layer operational failure: §5 forbids
// inventing a later event, so no replacement detail is ever fabricated.
func (r *claudeActiveRun) fixedSafeDetail(ctx context.Context, source map[string]any) (contextevent.Detail, error) {
	encoded, err := canonjson.Marshal(source)
	if err != nil {
		return contextevent.Detail{}, fmt.Errorf("sealedexec/claude: encode fixed safe detail: %w", err)
	}
	detail, err := r.processor.Process(ctx, bytes.TrimSuffix(encoded, []byte("\n")), r.classifiedOnly())
	if err != nil {
		return contextevent.Detail{}, fmt.Errorf("sealedexec/claude: fixed safe detail is unrepresentable: %w", err)
	}
	return detail, nil
}

// rawSafeDetail reduces discarded foreign bytes to §5's sole digest-and-reason
// object. The bytes themselves are hashed and never converted or embedded.
func (r *claudeActiveRun) rawSafeDetail(ctx context.Context, raw []byte, reason string) (contextevent.Detail, error) {
	encoded, err := contextevent.SafeRawDetail(raw, reason)
	if err != nil {
		return contextevent.Detail{}, fmt.Errorf("sealedexec/claude: reduce foreign bytes: %w", err)
	}
	detail, err := r.processor.Process(ctx, encoded, r.classifiedOnly())
	if err != nil {
		return contextevent.Detail{}, fmt.Errorf("sealedexec/claude: safe raw detail is unrepresentable: %w", err)
	}
	return detail, nil
}

func (r *claudeActiveRun) classifiedOnly() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]byte(nil), r.launch.Profile.PolicySecretValues...)
}

// decodeFailure emits the fixed telemetry-gap/adapter-error pair for one closed
// stream reason.
func (r *claudeActiveRun) decodeFailure(ctx context.Context, seq uint64, reason string, fields map[string]any) (sealedexec.AdapterResult, error) {
	source := map[string]any{"reason": reason}
	for key, value := range fields {
		source[key] = value
	}
	detail, err := r.fixedSafeDetail(ctx, source)
	if err != nil {
		return sealedexec.AdapterResult{}, err
	}
	return sealedexec.AdapterResult{
		Observations:       r.gapObservations(detail, seq, reason, claudeSource),
		OperationalFailure: reason,
	}, nil
}

// rawFailure emits the same pair for bytes that never became a frame.
func (r *claudeActiveRun) rawFailure(ctx context.Context, seq uint64, reason string, raw []byte) (sealedexec.AdapterResult, error) {
	detail, err := r.rawSafeDetail(ctx, raw, reason)
	if err != nil {
		return sealedexec.AdapterResult{}, err
	}
	return sealedexec.AdapterResult{
		Observations:       r.gapObservations(detail, seq, reason, claudeSource),
		OperationalFailure: reason,
	}, nil
}

// claudeErrorOperation is Amendment 002 §5's closed reason-to-operation map.
func claudeErrorOperation(reason string) string {
	switch reason {
	case "secret-classification-unavailable", "redaction-failed", "protected-fixed-field":
		return "redaction"
	case "provider-stderr", "provider-exit-nonzero":
		return "process"
	case "segment-store-failed", "segment-resolve-failed", "segment-mismatch":
		return "segment"
	case "recorder-append-failed", "recorder-ack-invalid", "recorder-replay-conflict", "unconfirmed-initial-session":
		return "recorder"
	}
	if strings.HasPrefix(reason, "provider-result-") {
		return "process"
	}
	return "decode"
}

// gapObservations produces the telemetry-gap and adapter-error pair for one
// closed reason over the named gap source.
func (r *claudeActiveRun) gapObservations(detail contextevent.Detail, seq uint64, reason, source string) []sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindTelemetryGap)
	gap := sealedexec.NormalizedObservation{
		Kind:            contextevent.KindTelemetryGap,
		ForeignDetail:   detail,
		BlocksAuthority: true,
		Witness:         reason,
		Payload: &contextevent.TelemetryGapPayload{
			Schema:       schema,
			Source:       source,
			FromSequence: seq,
			ToSequence:   seq,
			ReasonCode:   reason,
			Availability: "unavailable",
		},
	}
	errorSchema, _ := contextevent.PayloadSchema(contextevent.KindAdapterError)
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
			Operation:      claudeErrorOperation(reason),
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

// validateInitMCPServers proves the observed inventory is exactly the one
// scoped, connected verdi-context row.
func validateInitMCPServers(servers []claudeMCPRow) string {
	if len(servers) != 1 {
		return "mcp-mismatch"
	}
	row := servers[0]
	if row.Name == nil || row.Status == nil || *row.Name != "verdi-context" || *row.Status != "connected" {
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
	return text, ok && nonemptyStringValue(text)
}

// nonemptyStringValue is the closed shape of every §5 identity string.
func nonemptyStringValue(text string) bool {
	return text != "" && text == strings.TrimSpace(text) && utf8.ValidString(text)
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
