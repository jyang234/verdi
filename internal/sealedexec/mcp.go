package sealedexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
)

const (
	ToolGetFlightPlan  = "get_flight_plan"
	ToolRequestContext = "request_context"
	inspectionSchemaID = "verdi.scoped-context-inspection/v1"
)

// ToolDefinition is one closed flight-scoped MCP tool.
type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

var scopedTools = []ToolDefinition{
	{Name: ToolGetFlightPlan, Description: "Inspect the current sealed flight plan.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{},"type":"object"}`)},
	{Name: ToolRequestContext, Description: "Request one declared context expansion for approval.", InputSchema: json.RawMessage(`{"additionalProperties":false,"properties":{"purpose":{"minLength":1,"type":"string"},"ref":{"minLength":1,"type":"string"}},"required":["ref","purpose"],"type":"object"}`)},
}

// InspectionKind closes the MCP response union.
type InspectionKind string

const (
	InspectionFlightPlan       InspectionKind = "flight-plan"
	InspectionContextApproved  InspectionKind = "context-approved"
	InspectionContextDenied    InspectionKind = "context-denied"
	InspectionEpochInvalidated InspectionKind = "epoch-invalidated"
)

// FlightPlanInspection is typed state, not provider instruction authority.
type FlightPlanInspection struct {
	Flight, Lane, Epoch, Session     string
	ManifestRevision                 uint64
	ManifestDigest, ProjectionDigest string
	ExpansionRoot                    string
	Invalidated                      bool
}

// ContextInspection is typed expansion inspection data.
type ContextInspection struct {
	RequestID, Ref, Purpose string
	Data                    contextcompile.DataItem
	ChildRevision           uint64
	ChildManifestDigest     string
	Witnesses               []string
}

// InspectionResult is the sole MCP result union. InstructionAuthority is
// deliberately never encoded and remains nil for every result.
type InspectionResult struct {
	Kind                 InspectionKind
	FlightPlan           FlightPlanInspection
	Context              ContextInspection
	InstructionAuthority *InstructionAuthority `json:"-"`
}

// FlightStateSnapshot is the serialized authority-transition state guarded
// by FlightState's mutex.
type FlightStateSnapshot struct {
	Request                          ExecutionRequest
	Key                              ExecutionKey
	WorkspaceID                      string
	CandidateCommit, CandidateTree   string
	Revision                         uint64
	ManifestDigest, ProjectionDigest string
	ExpansionRoot                    string
	NextSourceSequence               uint64
	PriorEventDigest                 string
	PriorRevision                    *contextevent.PriorRevision
	LastGlobalSequence               uint64
	Invalidated                      bool
}

// FlightState serializes event admission with expansion installation.
type FlightState struct {
	mu       sync.Mutex
	snapshot FlightStateSnapshot
}

// NewFlightState constructs state from the already-decoded request and any
// explicit continuation position.
func NewFlightState(snapshot FlightStateSnapshot) *FlightState {
	if snapshot.Key == (ExecutionKey{}) {
		snapshot.Key = executionKey(snapshot.Request)
	}
	if snapshot.Revision == 0 {
		snapshot.Revision = snapshot.Request.ManifestRevision
	}
	if snapshot.ManifestDigest == "" {
		snapshot.ManifestDigest = snapshot.Request.ManifestDigest
	}
	if snapshot.ProjectionDigest == "" {
		snapshot.ProjectionDigest = snapshot.Request.ProjectionDigest
	}
	if snapshot.WorkspaceID == "" {
		snapshot.WorkspaceID, _ = snapshot.Request.ExecutionWorkspaceRequest.WorkspaceID()
	}
	if snapshot.CandidateCommit == "" {
		snapshot.CandidateCommit = snapshot.Request.InputCommit
	}
	if snapshot.CandidateTree == "" {
		snapshot.CandidateTree = snapshot.Request.InputTree
	}
	if snapshot.NextSourceSequence == 0 {
		snapshot.NextSourceSequence = 1
	}
	return &FlightState{snapshot: snapshot}
}

// Snapshot returns an isolated copy of the current transition state.
func (s *FlightState) Snapshot() FlightStateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.snapshot
	if out.PriorRevision != nil {
		copy := *out.PriorRevision
		out.PriorRevision = &copy
	}
	return out
}

// ContextResolution is the resolver's identity-bound data result.
type ContextResolution struct {
	Verification
	Ref  string
	Data contextcompile.DataItem
}

// EpochCheck carries the exact state that must remain unchanged.
type EpochCheck struct {
	Snapshot   FlightStateSnapshot
	Resolution ContextResolution
}

// ChildCompileRequest asks the compiler for a deterministic child manifest.
type ChildCompileRequest struct {
	RequestID string
	Ref       string
	Purpose   string
	Data      contextcompile.DataItem
	Snapshot  FlightStateSnapshot
}

// ChildManifest is the compiler's digest-bound approved transition.
type ChildManifest struct {
	Verification
	RequestID, ParentManifestDigest, ChildManifestDigest string
	ParentRevision, ChildRevision                        uint64
	ExpansionDigest, ExpansionRoot                       string
}

// ExpansionInstall is atomically persisted immediately after child ack.
type ExpansionInstall struct {
	Key                  ExecutionKey
	RequestID            string
	ParentRevision       uint64
	ParentManifestDigest string
	ChildRevision        uint64
	ChildManifestDigest  string
	ExpansionDigest      string
	ExpansionRoot        string
	TerminalAck          contextevent.EventAck
}

type ContextResolver interface {
	ResolveContext(context.Context, string) (ContextResolution, error)
}
type ChildCompiler interface {
	CompileChild(context.Context, ChildCompileRequest) (ChildManifest, error)
}
type EpochVerifier interface {
	VerifyEpoch(context.Context, EpochCheck) (Verification, error)
}
type ExpansionStore interface {
	InstallExpansion(context.Context, ExpansionInstall) error
}

// ScopedMCPPorts are the only capabilities reachable from the two tools.
type ScopedMCPPorts struct {
	Resolver ContextResolver
	Compiler ChildCompiler
	Verifier EpochVerifier
	Recorder interface {
		Append(context.Context, contextevent.Event) (contextevent.EventAck, error)
	}
	Store  ExpansionStore
	Stamps StampSource
}

// ScopedMCP exposes exactly the two ratified tools.
type ScopedMCP struct {
	ports ScopedMCPPorts
	state *FlightState
}

// NewScopedMCP refuses every missing capability.
func NewScopedMCP(ports ScopedMCPPorts, state *FlightState) (*ScopedMCP, error) {
	if state == nil || ports.Resolver == nil || ports.Compiler == nil || ports.Verifier == nil || ports.Recorder == nil || ports.Store == nil || ports.Stamps == nil {
		return nil, fmt.Errorf("sealedexec: scoped MCP requires state, resolver, compiler, verifier, recorder, store, and stamps")
	}
	return &ScopedMCP{ports: ports, state: state}, nil
}

// Tools returns the exact closed registry.
func (m *ScopedMCP) Tools() []ToolDefinition {
	out := make([]ToolDefinition, len(scopedTools))
	for i, tool := range scopedTools {
		out[i] = tool
		out[i].InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
	}
	return out
}

// Call strict-decodes one tool call and returns typed inspection data only.
func (m *ScopedMCP) Call(ctx context.Context, name string, arguments []byte) (InspectionResult, error) {
	if ctx == nil {
		return InspectionResult{}, operational("MCP call", errors.New("nil context"))
	}
	switch name {
	case ToolGetFlightPlan:
		var args struct{}
		if err := decodeMCPArguments(arguments, &args); err != nil {
			return InspectionResult{}, operational("decode get_flight_plan arguments", err)
		}
		s := m.state.Snapshot()
		return InspectionResult{Kind: InspectionFlightPlan, FlightPlan: FlightPlanInspection{
			Flight: s.Key.Flight, Lane: s.Key.Lane, Epoch: s.Key.Epoch, Session: s.Request.Session,
			ManifestRevision: s.Revision, ManifestDigest: s.ManifestDigest,
			ProjectionDigest: s.ProjectionDigest, ExpansionRoot: s.ExpansionRoot, Invalidated: s.Invalidated,
		}}, nil
	case ToolRequestContext:
		var args struct {
			Ref     string `json:"ref"`
			Purpose string `json:"purpose"`
		}
		if err := decodeMCPArguments(arguments, &args); err != nil {
			return InspectionResult{}, operational("decode request_context arguments", err)
		}
		if strings.TrimSpace(args.Ref) == "" || args.Ref != strings.TrimSpace(args.Ref) || strings.TrimSpace(args.Purpose) == "" || args.Purpose != strings.TrimSpace(args.Purpose) {
			return InspectionResult{}, operational("decode request_context arguments", errors.New("ref and purpose must be nonempty UTF-8 without surrounding whitespace"))
		}
		return m.requestContext(ctx, args.Ref, args.Purpose)
	default:
		return InspectionResult{}, operational("MCP tool", fmt.Errorf("unknown scoped tool %q", name))
	}
}

func (m *ScopedMCP) requestContext(ctx context.Context, ref, purpose string) (InspectionResult, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	snapshot := &m.state.snapshot
	requestID, err := contextRequestID(*snapshot, ref, purpose)
	if err != nil {
		return InspectionResult{}, operational("digest context request", err)
	}
	if snapshot.Invalidated {
		return InspectionResult{Kind: InspectionEpochInvalidated, Context: ContextInspection{RequestID: requestID, Ref: ref, Purpose: purpose, Witnesses: []string{"epoch already invalidated"}}}, nil
	}
	resolution, err := m.ports.Resolver.ResolveContext(ctx, ref)
	if err != nil {
		return InspectionResult{}, operational("resolve declared context", err)
	}
	if err := resolution.validate("context resolution"); err != nil {
		return InspectionResult{}, verdict(err.Error())
	}
	if resolution.Ref != ref {
		return InspectionResult{}, verdict("context resolver returned a different ref")
	}
	if err := m.appendContextRequest(ctx, snapshot, requestID, ref, purpose); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, err
	}

	if resolution.State != contextcompile.ResolutionProven {
		witnesses := nonNilSorted(resolution.Witnesses)
		decision := countersign.VerdictViolated
		if resolution.State == contextcompile.ResolutionUnproven {
			decision = countersign.VerdictUnproven
		}
		if err := m.appendContextDecision(ctx, snapshot, requestID, decision, "context-denied", "", witnesses); err != nil {
			snapshot.Invalidated = true
			return InspectionResult{}, err
		}
		return InspectionResult{Kind: InspectionContextDenied, Context: ContextInspection{RequestID: requestID, Ref: ref, Purpose: purpose, Witnesses: witnesses}}, nil
	}
	if _, err := contextcompile.EncodeDataItem(resolution.Data); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, operational("validate resolved context data", err)
	}

	epochVerification, err := m.ports.Verifier.VerifyEpoch(ctx, EpochCheck{Snapshot: *snapshot, Resolution: resolution})
	if err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, operational("reverify expansion epoch", err)
	}
	if err := epochVerification.validate("expansion epoch"); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, verdict(err.Error())
	}
	if epochVerification.State != contextcompile.ResolutionProven {
		witnesses := nonNilSorted(epochVerification.Witnesses)
		decision := countersign.VerdictViolated
		if epochVerification.State == contextcompile.ResolutionUnproven {
			decision = countersign.VerdictUnproven
		}
		if err := m.appendContextDecision(ctx, snapshot, requestID, decision, "epoch-invalidated", "", witnesses); err != nil {
			snapshot.Invalidated = true
			return InspectionResult{}, err
		}
		snapshot.Invalidated = true
		return InspectionResult{Kind: InspectionEpochInvalidated, Context: ContextInspection{RequestID: requestID, Ref: ref, Purpose: purpose, Witnesses: witnesses}}, nil
	}

	child, err := m.ports.Compiler.CompileChild(ctx, ChildCompileRequest{RequestID: requestID, Ref: ref, Purpose: purpose, Data: resolution.Data, Snapshot: *snapshot})
	if err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, operational("compile child manifest", err)
	}
	if err := requireProven("child manifest", child.Verification); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, err
	}
	if child.RequestID != requestID || child.ParentRevision != snapshot.Revision || child.ParentManifestDigest != snapshot.ManifestDigest ||
		child.ChildRevision != snapshot.Revision+1 || child.ChildManifestDigest == "" || child.ExpansionDigest == "" || child.ExpansionRoot == "" {
		snapshot.Invalidated = true
		return InspectionResult{}, verdict("child manifest transition identity mismatch")
	}
	if err := m.appendContextDecision(ctx, snapshot, requestID, countersign.VerdictProven, "approved", child.ChildManifestDigest, []string{}); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, err
	}
	ack, event, err := m.appendChildManifest(ctx, snapshot, child)
	if err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, err
	}
	install := ExpansionInstall{
		Key: snapshot.Key, RequestID: requestID, ParentRevision: child.ParentRevision,
		ParentManifestDigest: child.ParentManifestDigest, ChildRevision: child.ChildRevision,
		ChildManifestDigest: child.ChildManifestDigest, ExpansionDigest: child.ExpansionDigest,
		ExpansionRoot: child.ExpansionRoot, TerminalAck: ack,
	}
	if err := m.ports.Store.InstallExpansion(ctx, install); err != nil {
		snapshot.Invalidated = true
		return InspectionResult{}, operational("install acknowledged expansion", err)
	}
	snapshot.Revision = child.ChildRevision
	snapshot.ManifestDigest = child.ChildManifestDigest
	snapshot.ExpansionRoot = child.ExpansionRoot
	snapshot.NextSourceSequence = 1
	snapshot.PriorEventDigest = ""
	snapshot.PriorRevision = &contextevent.PriorRevision{
		ManifestRevision: child.ParentRevision, ManifestDigest: child.ParentManifestDigest,
		EventRoot: event.EventDigest, TerminalSourceSequence: event.SourceSequence,
		TerminalGlobalSequence: ack.GlobalSequence,
	}
	return InspectionResult{Kind: InspectionContextApproved, Context: ContextInspection{
		RequestID: requestID, Ref: ref, Purpose: purpose, Data: resolution.Data,
		ChildRevision: child.ChildRevision, ChildManifestDigest: child.ChildManifestDigest, Witnesses: []string{},
	}}, nil
}

func (m *ScopedMCP) appendContextRequest(ctx context.Context, state *FlightStateSnapshot, requestID, ref, purpose string) error {
	schema, _ := contextevent.PayloadSchema(contextevent.KindContextRequest)
	payload := &contextevent.ContextRequestPayload{Schema: schema, RequestID: requestID, Ref: ref, Purpose: purpose}
	_, _, err := m.appendEvent(ctx, state, contextevent.KindContextRequest, payload)
	return err
}

func (m *ScopedMCP) appendContextDecision(ctx context.Context, state *FlightStateSnapshot, requestID string, decision countersign.Verdict, reason, childDigest string, witnesses []string) error {
	schema, _ := contextevent.PayloadSchema(contextevent.KindContextDecision)
	if childDigest == "" {
		// A denial or invalidation installs no child; equality records the exact
		// no-transition manifest identity without inventing a digest.
		childDigest = state.ManifestDigest
	}
	payload := &contextevent.ContextDecisionPayload{Schema: schema, RequestID: requestID, Verdict: decision, ReasonCode: reason, ParentManifestDigest: state.ManifestDigest, ChildManifestDigest: childDigest, Witnesses: nonNilSorted(witnesses)}
	_, _, err := m.appendEvent(ctx, state, contextevent.KindContextDecision, payload)
	return err
}

func (m *ScopedMCP) appendChildManifest(ctx context.Context, state *FlightStateSnapshot, child ChildManifest) (contextevent.EventAck, contextevent.Event, error) {
	schema, _ := contextevent.PayloadSchema(contextevent.KindChildManifest)
	payload := &contextevent.ChildManifestPayload{Schema: schema, RequestID: child.RequestID, ParentRevision: child.ParentRevision, ParentManifestDigest: child.ParentManifestDigest, ChildRevision: child.ChildRevision, ChildManifestDigest: child.ChildManifestDigest, ExpansionDigest: child.ExpansionDigest}
	return m.appendEvent(ctx, state, contextevent.KindChildManifest, payload)
}

func (m *ScopedMCP) appendEvent(ctx context.Context, state *FlightStateSnapshot, kind contextevent.Kind, payload any) (contextevent.EventAck, contextevent.Event, error) {
	stamp, err := m.ports.Stamps.NextStamp(ctx)
	if err != nil {
		return contextevent.EventAck{}, contextevent.Event{}, operational("MCP event stamp", err)
	}
	request := state.Request
	request.ManifestRevision, request.ManifestDigest = state.Revision, state.ManifestDigest
	workspace := WorkspaceFacts{WorkspaceID: state.WorkspaceID, CurrentCommit: state.CandidateCommit, CurrentTree: state.CandidateTree}
	var priorRevision *contextevent.PriorRevision
	if state.NextSourceSequence == 1 {
		priorRevision = state.PriorRevision
	}
	event, err := buildEvent(request, workspace, state.NextSourceSequence, state.PriorEventDigest, priorRevision, stamp, kind, payload)
	if err != nil {
		return contextevent.EventAck{}, contextevent.Event{}, operational("encode MCP event", err)
	}
	ack, err := m.ports.Recorder.Append(ctx, event)
	if err != nil {
		return contextevent.EventAck{}, event, operational("append MCP event", err)
	}
	if err := validateAck(event, ack, state.LastGlobalSequence); err != nil {
		return contextevent.EventAck{}, event, operational("acknowledge MCP event", err)
	}
	if state.NextSourceSequence == 1 {
		state.PriorRevision = nil
	}
	state.NextSourceSequence++
	state.PriorEventDigest = event.EventDigest
	state.LastGlobalSequence = ack.GlobalSequence
	return ack, event, nil
}

func contextRequestID(state FlightStateSnapshot, ref, purpose string) (string, error) {
	digest, err := canonjson.Digest(struct {
		Flight         string `json:"flight"`
		Lane           string `json:"lane"`
		Epoch          string `json:"epoch"`
		Revision       uint64 `json:"revision"`
		ManifestDigest string `json:"manifest_digest"`
		Ref            string `json:"ref"`
		Purpose        string `json:"purpose"`
	}{Flight: state.Key.Flight, Lane: state.Key.Lane, Epoch: state.Key.Epoch, Revision: state.Revision, ManifestDigest: state.ManifestDigest, Ref: ref, Purpose: purpose})
	if err != nil {
		return "", err
	}
	return "context-request:" + strings.TrimPrefix(digest, "sha256:"), nil
}

func nonNilSorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	write := 0
	for _, value := range out {
		if write == 0 || value != out[write-1] {
			out[write] = value
			write++
		}
	}
	out = out[:write]
	if out == nil {
		out = []string{}
	}
	return out
}

type inspectionResultWire struct {
	Schema     *string                   `json:"schema"`
	Kind       *string                   `json:"kind"`
	FlightPlan *flightPlanInspectionWire `json:"flight_plan,omitempty"`
	Context    *contextInspectionWire    `json:"context,omitempty"`
}

type flightPlanInspectionWire struct {
	Flight           string `json:"flight"`
	Lane             string `json:"lane"`
	Epoch            string `json:"epoch"`
	Session          string `json:"session"`
	ManifestRevision uint64 `json:"manifest_revision"`
	ManifestDigest   string `json:"manifest_digest"`
	ProjectionDigest string `json:"projection_digest"`
	ExpansionRoot    string `json:"expansion_root"`
	Invalidated      bool   `json:"invalidated"`
}

type contextInspectionWire struct {
	RequestID           string          `json:"request_id"`
	Ref                 string          `json:"ref"`
	Purpose             string          `json:"purpose"`
	Data                json.RawMessage `json:"data,omitempty"`
	ChildRevision       uint64          `json:"child_revision,omitempty"`
	ChildManifestDigest string          `json:"child_manifest_digest,omitempty"`
	Witnesses           []string        `json:"witnesses"`
}

// EncodeInspectionResult validates and canonically encodes the closed MCP
// inspection union. There is deliberately no instruction-authority field.
func EncodeInspectionResult(result InspectionResult) ([]byte, error) {
	if result.InstructionAuthority != nil {
		return nil, errors.New("sealedexec: MCP inspection result cannot carry instruction authority")
	}
	schema, kind := inspectionSchemaID, string(result.Kind)
	wire := inspectionResultWire{Schema: &schema, Kind: &kind}
	switch result.Kind {
	case InspectionFlightPlan:
		if !reflect.DeepEqual(result.Context, ContextInspection{}) {
			return nil, errors.New("sealedexec: flight-plan inspection cannot carry a context arm")
		}
		if result.FlightPlan.Flight == "" || result.FlightPlan.Lane == "" || result.FlightPlan.Epoch == "" || result.FlightPlan.Session == "" ||
			result.FlightPlan.ManifestDigest == "" || result.FlightPlan.ProjectionDigest == "" {
			return nil, errors.New("sealedexec: incomplete flight-plan inspection")
		}
		if result.FlightPlan.ExpansionRoot != "" {
			if err := validateDigest("flight-plan expansion root", result.FlightPlan.ExpansionRoot); err != nil {
				return nil, err
			}
		}
		wire.FlightPlan = &flightPlanInspectionWire{
			Flight: result.FlightPlan.Flight, Lane: result.FlightPlan.Lane, Epoch: result.FlightPlan.Epoch,
			Session: result.FlightPlan.Session, ManifestRevision: result.FlightPlan.ManifestRevision,
			ManifestDigest: result.FlightPlan.ManifestDigest, ProjectionDigest: result.FlightPlan.ProjectionDigest,
			ExpansionRoot: result.FlightPlan.ExpansionRoot, Invalidated: result.FlightPlan.Invalidated,
		}
	case InspectionContextApproved, InspectionContextDenied, InspectionEpochInvalidated:
		if !reflect.DeepEqual(result.FlightPlan, FlightPlanInspection{}) {
			return nil, errors.New("sealedexec: context inspection cannot carry a flight-plan arm")
		}
		if result.Context.RequestID == "" || result.Context.Ref == "" || result.Context.Purpose == "" || result.Context.Witnesses == nil {
			return nil, errors.New("sealedexec: incomplete context inspection")
		}
		contextWire := &contextInspectionWire{
			RequestID: result.Context.RequestID, Ref: result.Context.Ref, Purpose: result.Context.Purpose,
			ChildRevision: result.Context.ChildRevision, ChildManifestDigest: result.Context.ChildManifestDigest,
			Witnesses: nonNilSorted(result.Context.Witnesses),
		}
		if result.Kind == InspectionContextApproved {
			if result.Context.ChildRevision == 0 || result.Context.ChildManifestDigest == "" {
				return nil, errors.New("sealedexec: approved context inspection lacks child identity")
			}
			data, err := contextcompile.EncodeDataItem(result.Context.Data)
			if err != nil {
				return nil, fmt.Errorf("sealedexec: encode inspection data: %w", err)
			}
			contextWire.Data = json.RawMessage(bytes.TrimSuffix(data, []byte("\n")))
		} else if !reflect.DeepEqual(result.Context.Data, contextcompile.DataItem{}) || result.Context.ChildRevision != 0 || result.Context.ChildManifestDigest != "" {
			return nil, errors.New("sealedexec: denied/invalidated inspection cannot carry data or a child identity")
		}
		wire.Context = contextWire
	default:
		return nil, fmt.Errorf("sealedexec: unknown inspection kind %q", result.Kind)
	}
	return canonjson.Marshal(wire)
}

// DecodeInspectionResult strict-decodes one canonical closed MCP result.
func DecodeInspectionResult(reader io.Reader) (InspectionResult, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return InspectionResult{}, err
	}
	if _, err := DecodeUniqueJSONObject(raw); err != nil {
		return InspectionResult{}, fmt.Errorf("sealedexec: decode inspection result: %w", err)
	}
	var wire inspectionResultWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return InspectionResult{}, fmt.Errorf("sealedexec: decode inspection result: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return InspectionResult{}, fmt.Errorf("sealedexec: decode inspection result: %w", err)
	}
	if wire.Schema == nil || *wire.Schema != inspectionSchemaID || wire.Kind == nil {
		return InspectionResult{}, errors.New("sealedexec: decode inspection result: missing or wrong schema/kind")
	}
	result := InspectionResult{Kind: InspectionKind(*wire.Kind)}
	switch result.Kind {
	case InspectionFlightPlan:
		if wire.FlightPlan == nil || wire.Context != nil {
			return InspectionResult{}, errors.New("sealedexec: decode inspection result: wrong flight-plan union arm")
		}
		p := wire.FlightPlan
		result.FlightPlan = FlightPlanInspection{Flight: p.Flight, Lane: p.Lane, Epoch: p.Epoch, Session: p.Session, ManifestRevision: p.ManifestRevision, ManifestDigest: p.ManifestDigest, ProjectionDigest: p.ProjectionDigest, ExpansionRoot: p.ExpansionRoot, Invalidated: p.Invalidated}
	case InspectionContextApproved, InspectionContextDenied, InspectionEpochInvalidated:
		if wire.Context == nil || wire.FlightPlan != nil || wire.Context.Witnesses == nil {
			return InspectionResult{}, errors.New("sealedexec: decode inspection result: wrong context union arm")
		}
		c := wire.Context
		result.Context = ContextInspection{RequestID: c.RequestID, Ref: c.Ref, Purpose: c.Purpose, ChildRevision: c.ChildRevision, ChildManifestDigest: c.ChildManifestDigest, Witnesses: c.Witnesses}
		if c.Data != nil {
			data, err := canonjson.Marshal(c.Data)
			if err != nil {
				return InspectionResult{}, err
			}
			result.Context.Data, err = contextcompile.DecodeDataItem(data)
			if err != nil {
				return InspectionResult{}, fmt.Errorf("sealedexec: decode inspection data: %w", err)
			}
		}
	default:
		return InspectionResult{}, fmt.Errorf("sealedexec: decode inspection result: unknown kind %q", result.Kind)
	}
	canonical, err := EncodeInspectionResult(result)
	if err != nil {
		return InspectionResult{}, err
	}
	if !bytes.Equal(canonical, raw) {
		return InspectionResult{}, errors.New("sealedexec: inspection result is not canonical")
	}
	return result, nil
}

func decodeMCPArguments(raw []byte, target any) error {
	if _, err := DecodeUniqueJSONObject(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

// DecodeUniqueJSONObject decodes one UTF-8 JSON object, rejecting duplicate
// keys at every depth and trailing data while retaining foreign fields.
func DecodeUniqueJSONObject(raw []byte) (map[string]any, error) {
	if !utf8.Valid(raw) {
		return nil, errors.New("JSON is not UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errors.New("JSON value must be an object")
	}
	return object, nil
}

func decodeUniqueJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return token, nil
	}
	switch delim {
	case '{':
		object := map[string]any{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, errors.New("JSON object key is not a string")
			}
			if _, exists := object[key]; exists {
				return nil, fmt.Errorf("duplicate JSON key %q", key)
			}
			value, err := decodeUniqueJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			object[key] = value
		}
		closeToken, err := decoder.Token()
		if err != nil || closeToken != json.Delim('}') {
			return nil, errors.New("unterminated JSON object")
		}
		return object, nil
	case '[':
		array := []any{}
		for decoder.More() {
			value, err := decodeUniqueJSONValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closeToken, err := decoder.Token()
		if err != nil || closeToken != json.Delim(']') {
			return nil, errors.New("unterminated JSON array")
		}
		return array, nil
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return errors.New("trailing JSON data")
	}
	return fmt.Errorf("trailing JSON data: %w", err)
}
