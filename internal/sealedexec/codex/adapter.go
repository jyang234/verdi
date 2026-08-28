// Package codex adapts the pinned non-interactive Codex JSONL protocol to
// sealedexec's harness-neutral process and observation ports.
package codex

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/sealedexec"
)

const (
	// DecoderProfileV1 is the I-71 witnessed foreign JSONL mapping.
	DecoderProfileV1      = "codex-jsonl-v1"
	providerInputSchemaID = "verdi.codex-provider-input/v1"
)

var canonicalDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ProcessResult is the explicit terminal process result.
type ProcessResult struct {
	ExitCode int
}

// ProcessObservation is one pull from the process boundary. ForeignJSON is
// one JSONL frame; Complete reports whether its newline framing completed.
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

// Process is the adapter's consumer-defined launch boundary. Tests never
// start Codex; a production provider may start the already-constructed Cmd.
type Process interface {
	Start(context.Context, *exec.Cmd, []byte) (ActiveProcess, error)
}

// Adapter owns the pinned argv, typed stdin, and version-selected decoder.
type Adapter struct{ process Process }

// New constructs an adapter over an explicit process port.
func New(process Process) (*Adapter, error) {
	if process == nil {
		return nil, errors.New("sealedexec/codex: process port is nil")
	}
	return &Adapter{process: process}, nil
}

// VerifyAdapter proves the selected executable/profile/version/decoder and
// constructs (but never starts) the exact profile-governed invocation.
func (a *Adapter) VerifyAdapter(ctx context.Context, check sealedexec.AdapterCheck) (sealedexec.AdapterFacts, error) {
	if ctx == nil {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/codex: verify adapter: nil context")
	}
	if check.Request.Adapter != contextevent.AdapterCodex || check.Profile.AdapterVersion != check.Request.AdapterVersion ||
		check.Profile.DecoderProfile != DecoderProfileV1 || check.Profile.Executable == "" || !filepath.IsAbs(check.Profile.Executable) ||
		check.Profile.Digest != check.Request.Profile.Digest || check.Profile.WorkspacePath != check.Workspace.Path {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/codex: adapter identity/profile/version mismatch")
	}
	args, err := verifyArgs(check.Request, check.Profile, check.Workspace)
	if err != nil {
		return sealedexec.AdapterFacts{}, err
	}
	command, _, cancel, err := check.Profile.Profile.Command(ctx, check.Profile.Executable, args...)
	if err != nil {
		return sealedexec.AdapterFacts{}, fmt.Errorf("sealedexec/codex: verify isolated command: %w", err)
	}
	cancel()
	command.Dir = check.Workspace.Path
	if command.Path != check.Profile.Executable || command.Dir != check.Profile.WorkspacePath || !equalStrings(command.Env, check.Profile.Profile.Env()) {
		return sealedexec.AdapterFacts{}, errors.New("sealedexec/codex: constructed command contradicts resolved profile")
	}
	return sealedexec.AdapterFacts{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven, Failure: sealedexec.FailureNone, Witnesses: []string{}},
		Adapter:      contextevent.AdapterCodex, AdapterVersion: check.Request.AdapterVersion,
		Executable: check.Profile.Executable, ProfileDigest: check.Profile.Digest,
		DecoderProfile: DecoderProfileV1,
	}, nil
}

// Start invokes the exact non-interactive start form.
func (a *Adapter) Start(ctx context.Context, launch sealedexec.AdapterLaunch) (sealedexec.ActiveAdapterRun, error) {
	args := []string{"exec", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", "--profile", launch.Profile.Name, "--sandbox", "workspace-write", "--cd", launch.Workspace.Path, "-"}
	return a.run(ctx, launch, args, "", true)
}

// Resume invokes only an explicit independently verified session id.
func (a *Adapter) Resume(ctx context.Context, launch sealedexec.AdapterLaunch, sessionRef string) (sealedexec.ActiveAdapterRun, error) {
	if err := validateSessionRef(sessionRef); err != nil {
		return nil, err
	}
	args := []string{"exec", "resume", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", sessionRef, "-"}
	return a.run(ctx, launch, args, sessionRef, false)
}

func (a *Adapter) run(ctx context.Context, launch sealedexec.AdapterLaunch, args []string, expectedSession string, start bool) (sealedexec.ActiveAdapterRun, error) {
	if ctx == nil {
		return nil, errors.New("sealedexec/codex: run: nil context")
	}
	if launch.Profile.DecoderProfile != DecoderProfileV1 || launch.Profile.AdapterVersion != launch.Request.AdapterVersion {
		return nil, errors.New("sealedexec/codex: selected profile does not bind decoder/version")
	}
	stdin, err := EncodeProviderInput(launch.Input)
	if err != nil {
		return nil, err
	}
	command, runCtx, cancel, err := launch.Profile.Profile.Command(ctx, launch.Profile.Executable, args...)
	if err != nil {
		return nil, fmt.Errorf("sealedexec/codex: construct process: %w", err)
	}
	command.Dir = launch.Workspace.Path
	command.Stdin = bytes.NewReader(stdin)
	processRun, err := a.process.Start(runCtx, command, stdin)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sealedexec/codex: process: %w", err)
	}
	if processRun == nil {
		cancel()
		return nil, errors.New("sealedexec/codex: process returned a nil active run")
	}
	return &activeRun{process: processRun, launch: launch, expectedSession: expectedSession, start: start, cancel: cancel}, nil
}

type activeRun struct {
	process         ActiveProcess
	launch          sealedexec.AdapterLaunch
	expectedSession string
	start           bool
	cancel          context.CancelFunc
	nextMu          sync.Mutex
	mu              sync.Mutex
	line            uint64
	threadCount     int
	terminal        bool
	stopOnce        sync.Once
	stopResult      sealedexec.AdapterStopResult
	stopErr         error
}

func (r *activeRun) Next(ctx context.Context) (sealedexec.AdapterResult, error) {
	if ctx == nil {
		return sealedexec.AdapterResult{}, errors.New("sealedexec/codex: next: nil context")
	}
	r.nextMu.Lock()
	defer r.nextMu.Unlock()
	r.mu.Lock()
	if r.terminal {
		r.mu.Unlock()
		return sealedexec.AdapterResult{}, errors.New("sealedexec/codex: next after terminal result")
	}
	r.mu.Unlock()
	item, err := r.process.Next(ctx)
	if err != nil {
		r.cancel()
		return sealedexec.AdapterResult{}, fmt.Errorf("sealedexec/codex: process stream: %w", err)
	}
	if item.Terminal != nil {
		if item.ForeignJSON != nil || item.Complete {
			r.cancel()
			return sealedexec.AdapterResult{}, errors.New("sealedexec/codex: process stream terminal/result union is invalid")
		}
		r.mu.Lock()
		r.terminal = true
		threadCount := r.threadCount
		line := r.line + 1
		r.mu.Unlock()
		r.cancel()
		result := sealedexec.AdapterResult{Terminal: &sealedexec.AdapterTerminalResult{ExitCode: item.Terminal.ExitCode}, Observations: []sealedexec.NormalizedObservation{}}
		if r.start && threadCount != 1 {
			detail := malformedDetail(nil, "missing-thread-started")
			result.Observations = append(result.Observations, gapObservations(r.launch, detail, line, "missing-thread-started")...)
			result.OperationalFailure = "missing-thread-started"
		}
		if item.Terminal.ExitCode != 0 {
			detail, detailErr := detailFor(map[string]any{"exit_code": item.Terminal.ExitCode, "type": "process.exit"})
			if detailErr != nil {
				return sealedexec.AdapterResult{}, detailErr
			}
			result.Observations = append(result.Observations, adapterErrorObservation(r.launch, detail, "process-exit", "nonzero-exit", true))
			if result.OperationalFailure == "" {
				result.OperationalFailure = "nonzero-exit"
			}
		}
		return result, nil
	}
	if item.ForeignJSON == nil {
		r.cancel()
		return sealedexec.AdapterResult{}, errors.New("sealedexec/codex: process stream observation/result union is invalid")
	}
	r.mu.Lock()
	r.line++
	line := r.line
	r.mu.Unlock()
	if !item.Complete {
		detail := malformedDetail(item.ForeignJSON, "truncated-final-line")
		return sealedexec.AdapterResult{Observations: gapObservations(r.launch, detail, line, "truncated-final-line"), OperationalFailure: "truncated-final-line"}, nil
	}
	return r.normalize(item.ForeignJSON, line), nil
}

func (r *activeRun) Stop(ctx context.Context) (sealedexec.AdapterStopResult, error) {
	if ctx == nil {
		return sealedexec.AdapterStopResult{}, errors.New("sealedexec/codex: stop: nil context")
	}
	r.stopOnce.Do(func() {
		r.cancel()
		result, err := r.process.Stop(ctx)
		if err != nil {
			r.stopErr = fmt.Errorf("sealedexec/codex: stop process: %w", err)
			return
		}
		if strings.TrimSpace(result.ReasonCode) == "" {
			r.stopErr = errors.New("sealedexec/codex: stop process returned an empty reason code")
			return
		}
		r.stopResult = sealedexec.AdapterStopResult{ExitCode: result.ExitCode, ReasonCode: result.ReasonCode}
	})
	return r.stopResult, r.stopErr
}

func verifyArgs(request sealedexec.ExecutionRequest, profile sealedexec.ResolvedProfile, workspace sealedexec.WorkspaceFacts) ([]string, error) {
	switch request.Action {
	case sealedexec.ActionStart:
		return []string{"exec", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", "--profile", profile.Name, "--sandbox", "workspace-write", "--cd", workspace.Path, "-"}, nil
	case sealedexec.ActionResume:
		if request.Resume == nil {
			return nil, errors.New("sealedexec/codex: resume request has no continuity arm")
		}
		if err := validateSessionRef(request.Resume.Continuity.AdapterSessionRef); err != nil {
			return nil, err
		}
		return []string{"exec", "resume", "--json", "--strict-config", "--ignore-user-config", "--ignore-rules", request.Resume.Continuity.AdapterSessionRef, "-"}, nil
	default:
		return nil, fmt.Errorf("sealedexec/codex: unsupported action %q", request.Action)
	}
}

type providerInputWire struct {
	Schema       string                   `json:"schema"`
	Instructions providerInstructionsWire `json:"instructions"`
	Data         []json.RawMessage        `json:"data"`
}

type providerInstructionsWire struct {
	InstructionProjection json.RawMessage `json:"instruction_projection"`
}

// EncodeProviderInput returns the one canonical typed stdin envelope.
func EncodeProviderInput(input sealedexec.ProviderInput) ([]byte, error) {
	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("sealedexec/codex: provider input: %w", err)
	}
	projection, err := sealedexec.EncodeInstructionProjection(input.Instructions.Projection)
	if err != nil {
		return nil, err
	}
	data := make([]json.RawMessage, len(input.Data))
	for i, item := range input.Data {
		encoded, err := contextcompile.EncodeDataItem(item)
		if err != nil {
			return nil, fmt.Errorf("sealedexec/codex: provider input data[%d]: %w", i, err)
		}
		data[i] = json.RawMessage(bytes.TrimSuffix(encoded, []byte("\n")))
	}
	return canonjson.Marshal(providerInputWire{
		Schema:       providerInputSchemaID,
		Instructions: providerInstructionsWire{InstructionProjection: json.RawMessage(bytes.TrimSuffix(projection, []byte("\n")))},
		Data:         data,
	})
}

// DecodeProviderInput strict-decodes the adapter-owned stdin envelope.
func DecodeProviderInput(reader io.Reader) (sealedexec.ProviderInput, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return sealedexec.ProviderInput{}, err
	}
	if _, err := sealedexec.DecodeUniqueJSONObject(raw); err != nil {
		return sealedexec.ProviderInput{}, fmt.Errorf("sealedexec/codex: decode provider input: %w", err)
	}
	var wire providerInputWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return sealedexec.ProviderInput{}, fmt.Errorf("sealedexec/codex: decode provider input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return sealedexec.ProviderInput{}, errors.New("sealedexec/codex: decode provider input: trailing data")
	}
	if wire.Schema != providerInputSchemaID || wire.Instructions.InstructionProjection == nil || wire.Data == nil {
		return sealedexec.ProviderInput{}, errors.New("sealedexec/codex: decode provider input: missing or wrong mandatory field")
	}
	projectionBytes, err := canonjson.Marshal(wire.Instructions.InstructionProjection)
	if err != nil {
		return sealedexec.ProviderInput{}, err
	}
	projection, err := sealedexec.DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		return sealedexec.ProviderInput{}, err
	}
	items := make([]contextcompile.DataItem, len(wire.Data))
	for i, encoded := range wire.Data {
		itemBytes, err := canonjson.Marshal(encoded)
		if err != nil {
			return sealedexec.ProviderInput{}, err
		}
		items[i], err = contextcompile.DecodeDataItem(itemBytes)
		if err != nil {
			return sealedexec.ProviderInput{}, fmt.Errorf("sealedexec/codex: decode provider input data[%d]: %w", i, err)
		}
	}
	input := sealedexec.ProviderInput{Instructions: sealedexec.InstructionAuthority{Projection: projection}, Data: items}
	canonical, err := EncodeProviderInput(input)
	if err != nil {
		return sealedexec.ProviderInput{}, err
	}
	if !bytes.Equal(canonical, raw) {
		return sealedexec.ProviderInput{}, errors.New("sealedexec/codex: provider input is not canonical")
	}
	return input, nil
}

func (r *activeRun) normalize(line []byte, lineNumber uint64) sealedexec.AdapterResult {
	result := sealedexec.AdapterResult{Observations: []sealedexec.NormalizedObservation{}}
	if len(line) == 0 {
		detail := malformedDetail(line, "empty-line")
		result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "empty-line")...)
		result.OperationalFailure = "empty-line"
		return result
	}
	object, err := sealedexec.DecodeUniqueJSONObject(line)
	if err != nil {
		detail := malformedDetail(line, "malformed-json")
		result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "malformed-json")...)
		result.OperationalFailure = "malformed-json"
		return result
	}
	detail, err := detailFor(object)
	if err != nil {
		detail = malformedDetail(line, "canonicalization-failed")
		result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "canonicalization-failed")...)
		result.OperationalFailure = "canonicalization-failed"
		return result
	}
	outer, ok := nonemptyString(object["type"])
	if !ok {
		result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "missing-outer-type")...)
		result.OperationalFailure = "missing-outer-type"
		return result
	}
	if outer == "thread.started" {
		r.mu.Lock()
		r.threadCount++
		threadCount := r.threadCount
		r.mu.Unlock()
		threadID, ok := nonemptyString(object["thread_id"])
		if !ok || threadCount != 1 {
			result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "session-identity-mismatch")...)
			return result
		}
		result.ObservedSessionRef = threadID
		if !r.start && threadID != r.expectedSession {
			result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, "session-identity-mismatch")...)
			return result
		}
	}
	observations, reason := mapForeign(r.launch, outer, object, detail, lineNumber)
	if reason != "" {
		result.Observations = append(result.Observations, gapObservations(r.launch, detail, lineNumber, reason)...)
		result.OperationalFailure = reason
		return result
	}
	result.Observations = append(result.Observations, observations...)
	if outer == "turn.failed" || outer == "error" {
		result.OperationalFailure = "provider-error"
	}
	return result
}

func mapForeign(launch sealedexec.AdapterLaunch, outer string, object map[string]any, detail contextevent.Detail, line uint64) ([]sealedexec.NormalizedObservation, string) {
	switch outer {
	case "thread.started":
		schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterStart)
		primary := sealedexec.NormalizedObservation{Kind: contextevent.KindAdapterStart, ForeignDetail: detail, Payload: &contextevent.AdapterStartPayload{
			Schema: schema, Adapter: contextevent.AdapterCodex, AdapterVersion: launch.Request.AdapterVersion,
			Session: launch.Request.Session, ProfileDigest: launch.Profile.Digest,
			WorkspaceRequestDigest: launch.Workspace.RequestDigest,
		}}
		return []sealedexec.NormalizedObservation{primary, summaryObservation(detail, "thread-started")}, ""
	case "turn.started", "turn.completed":
		return []sealedexec.NormalizedObservation{summaryObservation(detail, outer)}, ""
	case "turn.failed", "error":
		return []sealedexec.NormalizedObservation{adapterErrorObservation(launch, detail, outer, "provider-error", true)}, ""
	case "item.started", "item.updated", "item.completed":
		item, ok := object["item"].(map[string]any)
		if !ok {
			return nil, "missing-item"
		}
		return mapItem(launch, outer, item, detail, line)
	default:
		return nil, "unknown-outer-type"
	}
}

func mapItem(launch sealedexec.AdapterLaunch, outer string, item map[string]any, detail contextevent.Detail, line uint64) ([]sealedexec.NormalizedObservation, string) {
	id, ok := nonemptyString(item["id"])
	if !ok {
		return nil, "missing-item-id"
	}
	kind, ok := nonemptyString(item["type"])
	if !ok {
		return nil, "missing-item-type"
	}
	switch kind {
	case "agent_message":
		text, ok := nonemptyString(item["text"])
		if !ok {
			return nil, "missing-agent-message-text"
		}
		schema, _ := contextevent.PayloadSchema(contextevent.KindProviderMessage)
		return []sealedexec.NormalizedObservation{{Kind: contextevent.KindProviderMessage, ForeignDetail: detail, Payload: &contextevent.ProviderMessagePayload{Schema: schema, MessageID: id, Role: "assistant", MessageDigest: digestBytes([]byte(text)), Detail: detail}}}, ""
	case "reasoning", "plan_update":
		return []sealedexec.NormalizedObservation{summaryObservation(detail, id)}, ""
	case "command_execution":
		if outer == "item.started" {
			command, ok := nonemptyString(item["command"])
			if !ok {
				return nil, "missing-command"
			}
			schema, _ := contextevent.PayloadSchema(contextevent.KindCommand)
			payload := &contextevent.CommandPayload{Schema: schema, CommandID: id, Argv: []string{command}, WorkingDirectory: launch.Workspace.Path, DeclaredEnvironmentNames: environmentNames(launch.Profile.Profile.Env()), TimeoutMilliseconds: uint64(launch.Profile.Profile.Timeout.Milliseconds())}
			return []sealedexec.NormalizedObservation{{Kind: contextevent.KindCommand, Payload: payload, ForeignDetail: detail}, summaryObservation(detail, id)}, ""
		}
		status, ok := nonemptyString(item["status"])
		if !ok {
			return nil, "missing-command-status"
		}
		schema, _ := contextevent.PayloadSchema(contextevent.KindToolResult)
		return []sealedexec.NormalizedObservation{{Kind: contextevent.KindToolResult, ForeignDetail: detail, Payload: &contextevent.ToolResultPayload{Schema: schema, CallID: id, ToolName: "codex.command_execution", Status: status, OutputDigest: detail.Digest, Detail: detail}}}, ""
	case "file_change":
		path, pathOK := nonemptyString(item["path"])
		before, beforeOK := nonemptyString(item["before_digest"])
		after, afterOK := nonemptyString(item["after_digest"])
		count, countOK := uintValue(item["byte_count"])
		if !pathOK || !beforeOK || !afterOK || !countOK || count == 0 || !canonicalDigest.MatchString(before) || !canonicalDigest.MatchString(after) || !workspaceRelativePath(path) {
			return nil, "incomplete-file-change"
		}
		schema, _ := contextevent.PayloadSchema(contextevent.KindWrite)
		payload := &contextevent.WritePayload{Schema: schema, Path: path, ClaimID: id, BeforeDigest: before, AfterDigest: after, ByteCount: count}
		return []sealedexec.NormalizedObservation{{Kind: contextevent.KindWrite, ForeignDetail: detail, Payload: payload}, summaryObservation(detail, id)}, ""
	case "mcp_tool_call":
		tool, ok := nonemptyString(item["tool"])
		if !ok || (tool != sealedexec.ToolGetFlightPlan && tool != sealedexec.ToolRequestContext) {
			return nil, "unscoped-mcp-tool"
		}
		if outer == "item.started" {
			arguments, ok := item["arguments"]
			if !ok {
				return nil, "missing-mcp-arguments"
			}
			argumentDigest, err := canonjson.Digest(arguments)
			if err != nil {
				return nil, "invalid-mcp-arguments"
			}
			schema, _ := contextevent.PayloadSchema(contextevent.KindToolCall)
			return []sealedexec.NormalizedObservation{{Kind: contextevent.KindToolCall, ForeignDetail: detail, Payload: &contextevent.ToolCallPayload{Schema: schema, CallID: id, ToolName: tool, ArgumentsDigest: argumentDigest, Detail: detail}}}, ""
		}
		status, ok := nonemptyString(item["status"])
		if !ok {
			return nil, "missing-mcp-status"
		}
		schema, _ := contextevent.PayloadSchema(contextevent.KindToolResult)
		return []sealedexec.NormalizedObservation{{Kind: contextevent.KindToolResult, ForeignDetail: detail, Payload: &contextevent.ToolResultPayload{Schema: schema, CallID: id, ToolName: tool, Status: status, OutputDigest: detail.Digest, Detail: detail}}}, ""
	case "web_search":
		schema, _ := contextevent.PayloadSchema(contextevent.KindToolCall)
		call := sealedexec.NormalizedObservation{Kind: contextevent.KindToolCall, ForeignDetail: detail, BlocksAuthority: true, Witness: "web search invalidates sealed epoch", Payload: &contextevent.ToolCallPayload{Schema: schema, CallID: id, ToolName: "web_search", ArgumentsDigest: detail.Digest, Detail: detail}}
		rows := []sealedexec.NormalizedObservation{call}
		rows = append(rows, gapObservations(launch, detail, line, "web-search-forbidden")...)
		return rows, ""
	default:
		return nil, "unknown-item-type"
	}
}

func summaryObservation(detail contextevent.Detail, identity string) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindProviderSummary)
	return sealedexec.NormalizedObservation{Kind: contextevent.KindProviderSummary, ForeignDetail: detail, Payload: &contextevent.ProviderSummaryPayload{Schema: schema, SummaryID: "codex:" + identity + ":" + strings.TrimPrefix(detail.Digest, "sha256:")[:12], SummaryDigest: detail.Digest, Authority: contextevent.AuthorityAdvisory, Detail: detail}}
}

func adapterErrorObservation(launch sealedexec.AdapterLaunch, detail contextevent.Detail, operation, reason string, blocks bool) sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterError)
	return sealedexec.NormalizedObservation{Kind: contextevent.KindAdapterError, ForeignDetail: detail, BlocksAuthority: blocks, Witness: reason, Payload: &contextevent.AdapterErrorPayload{Schema: schema, Adapter: contextevent.AdapterCodex, AdapterVersion: launch.Request.AdapterVersion, Session: launch.Request.Session, Operation: operation, ReasonCode: reason, ErrorDigest: detail.Digest, Detail: detail}}
}

func gapObservations(launch sealedexec.AdapterLaunch, detail contextevent.Detail, line uint64, reason string) []sealedexec.NormalizedObservation {
	schema, _ := contextevent.PayloadSchema(contextevent.KindTelemetryGap)
	gap := sealedexec.NormalizedObservation{Kind: contextevent.KindTelemetryGap, ForeignDetail: detail, BlocksAuthority: true, Witness: reason, Payload: &contextevent.TelemetryGapPayload{Schema: schema, Source: "codex-jsonl", FromSequence: line, ToSequence: line, ReasonCode: reason, Availability: "unavailable"}}
	return []sealedexec.NormalizedObservation{gap, adapterErrorObservation(launch, detail, "decode-jsonl", reason, true)}
}

func detailFor(object map[string]any) (contextevent.Detail, error) {
	encoded, err := canonjson.Marshal(object)
	if err != nil {
		return contextevent.Detail{}, err
	}
	encoded = bytes.TrimSuffix(encoded, []byte("\n"))
	digest, err := canonjson.Digest(object)
	if err != nil {
		return contextevent.Detail{}, err
	}
	detail := contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: digest, RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: json.RawMessage(encoded)}
	if err := detail.Validate(); err != nil {
		return contextevent.Detail{}, err
	}
	return detail, nil
}

func malformedDetail(raw []byte, reason string) contextevent.Detail {
	if !utf8.Valid(raw) {
		raw = []byte(strings.ToValidUTF8(string(raw), "�"))
	}
	detail, err := detailFor(map[string]any{"foreign_line": string(raw), "reason": reason})
	if err != nil {
		fallback := json.RawMessage(`{}`)
		return contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: digestBytes(fallback), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: fallback}
	}
	return detail
}

func workspaceRelativePath(path string) bool {
	clean := filepath.Clean(path)
	return path == clean && path != "." && !filepath.IsAbs(path) && path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func validateSessionRef(value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.HasPrefix(value, "-") || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n\x00") {
		return errors.New("sealedexec/codex: adapter session ref must be nonempty explicit identity, never an option selector")
	}
	return nil
}

func environmentNames(env []string) []string {
	names := make([]string, 0, len(env))
	for _, row := range env {
		name, _, ok := strings.Cut(row, "=")
		if ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func nonemptyString(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok && text != "" && text == strings.TrimSpace(text) && utf8.ValidString(text)
}

func uintValue(value any) (uint64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseUint(string(typed), 10, 64)
		return parsed, err == nil
	case float64:
		if typed < 0 || typed != float64(uint64(typed)) {
			return 0, false
		}
		return uint64(typed), true
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
