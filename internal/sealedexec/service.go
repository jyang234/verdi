package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyconflict"
)

var (
	// ErrVerdict marks a proved refusal. The request and any partial output
	// remain inspectable, but no authoritative continuation is permitted.
	ErrVerdict = errors.New("sealedexec: verdict failure")
	// ErrOperational marks malformed input or an unavailable storage/process
	// dependency. It is distinct from an adverse authority verdict.
	ErrOperational = errors.New("sealedexec: operational failure")
	// ErrConcurrentDispatch refuses resume/replacement overlap for one epoch.
	ErrConcurrentDispatch = errors.New("sealedexec: concurrent dispatch")
)

// FailureCode is the closed reason category accompanying a non-proven fact.
type FailureCode string

const (
	FailureNone        FailureCode = ""
	FailureMismatch    FailureCode = "mismatch"
	FailureDirty       FailureCode = "dirty"
	FailureUnproven    FailureCode = "unproven"
	FailureUnavailable FailureCode = "unavailable"
	FailureStale       FailureCode = "stale"
	FailureRejected    FailureCode = "rejected"
	FailureOutOfScope  FailureCode = "out-of-scope"
)

// Verification is a structured three-valued result returned by prerequisite
// ports. Witnesses are evidence, never caller authority.
type Verification struct {
	State     contextcompile.Resolution
	Failure   FailureCode
	Witnesses []string
}

func (v Verification) validate(name string) error {
	switch v.State {
	case contextcompile.ResolutionProven:
		if v.Failure != FailureNone || len(v.Witnesses) != 0 {
			return fmt.Errorf("%s: proven fact carries adverse metadata", name)
		}
	case contextcompile.ResolutionViolatedWithWitness, contextcompile.ResolutionUnproven:
		if v.Failure == FailureNone {
			return fmt.Errorf("%s: non-proven fact has no failure code", name)
		}
	default:
		return fmt.Errorf("%s: unknown verification state %q", name, v.State)
	}
	return nil
}

// ExecutionKey identifies the one mutually exclusive flight/lane/epoch.
type ExecutionKey struct{ Flight, Lane, Epoch string }

func executionKey(request ExecutionRequest) ExecutionKey {
	return ExecutionKey{Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch}
}

// AuthorityFacts are freshly recomputed accepted authority and projection
// identities.
type AuthorityFacts struct {
	Verification
	ManifestRevision   uint64
	ManifestDigest     string
	ProjectionDigest   string
	AuthorityDigest    string
	AcceptedSpecCommit string
}

// RunwayFacts are a fresh repository observation of the ATC checkout.
type RunwayFacts struct {
	Verification
	Path, Commit, Tree string
	Clean              bool
}

// WorkspaceFacts prove the execution-workspace component's returned child,
// immutable sidecar, and fresh candidate state.
type WorkspaceFacts struct {
	Verification
	WorkspaceID, Path          string
	Request                    execworkspace.Identity
	RequestDigest              string
	CurrentCommit, CurrentTree string
	Clean                      bool
}

// ResolvedProfile is a freshly resolved logical project profile. Profile is
// the activated execution-workspace mechanism; no ambient environment is
// accepted by this type.
type ResolvedProfile struct {
	Verification
	Ref                                 LogicalRef
	Digest, Name, Executable, CodexHome string
	AdapterVersion, DecoderProfile      string
	WorkspacePath                       string
	Profile                             execworkspace.Profile
	Grants                              execworkspace.GrantSet
	Enforcement                         execworkspace.EnforcementReport
}

// ConflictFacts are the freshly verified policy-conflict report facts.
type ConflictFacts struct {
	Verification
	Report policyconflict.Report
}

// RecorderFacts prove that a logical endpoint resolves to the requested
// durable append/query binding.
type RecorderFacts struct {
	Verification
	Ref LogicalRef
}

// RecorderCheckpoint is a fresh durable query result.
type RecorderCheckpoint struct {
	Verification
	Digest                 string
	Revisions              []contextevent.Revision
	EventChainRoot         string
	TerminalSourceSequence uint64
	TerminalGlobalSequence uint64
}

// OpaqueIdentity is the only vendor-boundary information exposed to a port.
type OpaqueIdentity struct {
	ID, Kind, AdapterID, AdapterVersion string
}

// OpaqueBoundaryFacts prove identity-only handling of declared opaque rows.
type OpaqueBoundaryFacts struct {
	Verification
	Rows []OpaqueIdentity
}

// AdapterFacts bind the selected decoder and exact executable/profile.
type AdapterFacts struct {
	Verification
	Adapter        contextevent.Adapter
	AdapterVersion string
	Executable     string
	ProfileDigest  string
	DecoderProfile string
}

// ProviderSessionFacts are a fresh isolated-profile session-state proof.
type ProviderSessionFacts struct {
	Verification
	SessionRef, AdapterVersion, ProfileDigest, WorkspaceID string
}

// ExpansionFacts are the current atomically installed expansion ledger facts.
type ExpansionFacts struct {
	Verification
	Root string
}

// NormalizedObservation is one adapter-owned foreign observation mapped onto
// U4a's closed registry. Payload is the registered pointer type for Kind.
type NormalizedObservation struct {
	Kind            contextevent.Kind
	Payload         any
	ForeignDetail   contextevent.Detail
	BlocksAuthority bool
	Witness         string
}

// AdapterCheck carries only already-verified launch identities.
type AdapterCheck struct {
	Request   ExecutionRequest
	Profile   ResolvedProfile
	Workspace WorkspaceFacts
}

// AdapterLaunch structurally separates immutable instructions from data.
type AdapterLaunch struct {
	Request   ExecutionRequest
	Profile   ResolvedProfile
	Workspace WorkspaceFacts
	Input     ProviderInput
}

// AdapterResult is a normalized, already-redacted provider stream.
type AdapterResult struct {
	ObservedSessionRef string
	Observations       []NormalizedObservation
	ExitCode           int
	OperationalFailure string
}

// AdapterStopRequest selects the normalized stop path; it is not a public
// command.
type AdapterStopRequest struct {
	Request           ExecutionRequest
	Workspace         WorkspaceFacts
	AdapterSessionRef string
}

// AdapterStopResult is the normalized process stop outcome.
type AdapterStopResult struct {
	ExitCode   int
	ReasonCode string
}

// ProviderSessionCheck names the exact state a session verifier must prove.
type ProviderSessionCheck struct {
	SessionRef, AdapterVersion, ProfileDigest, WorkspaceID string
}

// SessionRecord is persisted only after adapter-start is durably acknowledged.
type SessionRecord struct {
	Key            ExecutionKey
	SessionRef     string
	AdapterVersion string
	ProfileDigest  string
	WorkspaceID    string
	LifecycleAck   contextevent.EventAck
}

// ExecutionRun is U4c's harness-neutral partial execution outcome. U4d owns
// public result/receipt completion and handback.
type ExecutionRun struct {
	Authority         contextevent.Authority
	Witnesses         []string
	Workspace         WorkspaceFacts
	Profile           ResolvedProfile
	AdapterSessionRef string
	Acks              []contextevent.EventAck
}

// InterruptRequest carries the already-decoded execution identity needed to
// record a normalized stop.
type InterruptRequest struct {
	Request           ExecutionRequest
	Workspace         WorkspaceFacts
	AdapterSessionRef string
}

// Consumer-defined ports keep providers from manufacturing proof.
type AuthorityVerifier interface {
	VerifyAuthority(context.Context, ExecutionRequest) (AuthorityFacts, error)
}
type RunwayVerifier interface {
	VerifyRunway(context.Context, string) (RunwayFacts, error)
}
type WorkspaceMaterializer interface {
	Materialize(context.Context, execworkspace.Request) (execworkspace.Result, error)
}
type WorkspaceVerifier interface {
	VerifyWorkspace(context.Context, string, execworkspace.Identity) (WorkspaceFacts, error)
}
type ProfileResolver interface {
	ResolveProfile(context.Context, LogicalRef, string, execworkspace.GrantSet) (ResolvedProfile, error)
}
type ConflictVerifier interface {
	VerifyConflict(context.Context, policyconflict.Report) (ConflictFacts, error)
}
type RecorderResolver interface {
	ResolveRecorder(context.Context, LogicalRef) (RecorderFacts, Recorder, error)
}
type Recorder interface {
	Append(context.Context, contextevent.Event) (contextevent.EventAck, error)
	Checkpoint(context.Context, ExecutionKey) (RecorderCheckpoint, error)
}
type OpaqueBoundaryVerifier interface {
	VerifyOpaqueBoundary(context.Context, []contextcompile.OpaqueEntry) (OpaqueBoundaryFacts, error)
}
type ExecutionAdapter interface {
	VerifyAdapter(context.Context, AdapterCheck) (AdapterFacts, error)
	Start(context.Context, AdapterLaunch) (AdapterResult, error)
	Resume(context.Context, AdapterLaunch, string) (AdapterResult, error)
	Stop(context.Context, AdapterStopRequest) (AdapterStopResult, error)
}
type ProviderSessionVerifier interface {
	VerifyProviderSession(context.Context, ProviderSessionCheck) (ProviderSessionFacts, error)
}
type ExpansionVerifier interface {
	VerifyExpansion(context.Context, ExecutionKey) (ExpansionFacts, error)
}
type AdapterSessionStore interface {
	StoreAdapterSession(context.Context, SessionRecord) error
}
type StampSource interface {
	NextStamp(context.Context) (string, error)
}

// ServicePorts is the complete harness-neutral dependency set.
type ServicePorts struct {
	Authority    AuthorityVerifier
	Runway       RunwayVerifier
	Materializer WorkspaceMaterializer
	Workspace    WorkspaceVerifier
	Profiles     ProfileResolver
	Conflicts    ConflictVerifier
	Recorders    RecorderResolver
	Opaque       OpaqueBoundaryVerifier
	Adapter      ExecutionAdapter
	Sessions     ProviderSessionVerifier
	Expansions   ExpansionVerifier
	SessionStore AdapterSessionStore
	Stamps       StampSource
}

// Service owns prerequisite ordering and per-epoch dispatch exclusivity.
type Service struct {
	ports  ServicePorts
	mu     sync.Mutex
	active map[ExecutionKey]string
}

// NewService refuses a missing proof or execution dependency.
func NewService(ports ServicePorts) (*Service, error) {
	missing := []string{}
	for name, port := range map[string]any{
		"authority": ports.Authority, "runway": ports.Runway, "materializer": ports.Materializer,
		"workspace": ports.Workspace, "profiles": ports.Profiles, "conflicts": ports.Conflicts,
		"recorders": ports.Recorders, "opaque": ports.Opaque, "adapter": ports.Adapter,
		"sessions": ports.Sessions, "expansions": ports.Expansions,
		"session_store": ports.SessionStore, "stamps": ports.Stamps,
	} {
		if port == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("sealedexec: new service: missing ports %v", missing)
	}
	return &Service{ports: ports, active: make(map[ExecutionKey]string)}, nil
}

// Execute verifies every prerequisite and launches last.
func (s *Service) Execute(ctx context.Context, request ExecutionRequest, data []contextcompile.DataItem) (ExecutionRun, error) {
	if ctx == nil {
		return ExecutionRun{}, operational("execute", errors.New("nil context"))
	}
	release, err := s.acquire(executionKey(request), string(request.Action))
	if err != nil {
		return ExecutionRun{}, err
	}
	defer release()
	if _, err := EncodeExecutionRequest(request); err != nil {
		return ExecutionRun{}, operational("validate canonical request", err)
	}
	input := ProviderInput{Instructions: InstructionAuthority{Projection: request.InstructionProjection}, Data: data}
	if err := input.Validate(); err != nil {
		return ExecutionRun{}, operational("validate provider input", err)
	}

	authority, err := s.ports.Authority.VerifyAuthority(ctx, request)
	if err != nil {
		return ExecutionRun{}, operational("verify authority", err)
	}
	if err := requireProven("authority", authority.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if authority.ManifestRevision != request.ManifestRevision || authority.ManifestDigest != request.ManifestDigest ||
		authority.ProjectionDigest != request.ProjectionDigest || authority.AuthorityDigest != request.AuthorityVerdict.Digest ||
		authority.AcceptedSpecCommit != request.Manifest.AcceptedSpec.Commit {
		return ExecutionRun{}, verdict("fresh authority identity mismatch")
	}

	runway, err := s.ports.Runway.VerifyRunway(ctx, request.ATCRunway)
	if err != nil {
		return ExecutionRun{}, operational("verify runway", err)
	}
	if err := requireProven("runway", runway.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if runway.Path != request.ATCRunway || runway.Commit != request.InputCommit || runway.Tree != request.InputTree || !runway.Clean {
		return ExecutionRun{}, verdict("ATC runway is dirty or not exactly at input commit/tree")
	}

	materialized, err := s.ports.Materializer.Materialize(ctx, execworkspace.Request{Identity: request.ExecutionWorkspaceRequest})
	if err != nil {
		return ExecutionRun{}, operational("materialize execution workspace", err)
	}
	wantWorkspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return ExecutionRun{}, operational("derive workspace id", err)
	}
	if materialized.WorkspaceID != wantWorkspaceID || !executionWorkspacePath(materialized.Path, wantWorkspaceID) ||
		(materialized.Outcome != execworkspace.OutcomeMaterialized && materialized.Outcome != execworkspace.OutcomeReused) {
		return ExecutionRun{}, verdict("materializer returned an identity or path outside data/execution")
	}
	workspace, err := s.ports.Workspace.VerifyWorkspace(ctx, materialized.Path, request.ExecutionWorkspaceRequest)
	if err != nil {
		return ExecutionRun{}, operational("verify workspace", err)
	}
	if err := requireProven("workspace", workspace.Verification); err != nil {
		return ExecutionRun{}, err
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return ExecutionRun{}, operational("digest workspace request", err)
	}
	if workspace.WorkspaceID != wantWorkspaceID || workspace.Path != materialized.Path ||
		!workspace.Request.Equal(request.ExecutionWorkspaceRequest) || workspace.RequestDigest != workspaceDigest || !workspace.Clean {
		return ExecutionRun{}, verdict("execution workspace identity, sidecar, or clean state mismatch")
	}
	expectedCurrentCommit, expectedCurrentTree := request.InputCommit, request.InputTree
	if request.Action == ActionResume {
		expectedCurrentCommit, expectedCurrentTree = request.Resume.Continuity.CurrentCommit, request.Resume.Continuity.CurrentTree
	}
	if workspace.CurrentCommit != expectedCurrentCommit || workspace.CurrentTree != expectedCurrentTree {
		return ExecutionRun{}, verdict("execution workspace candidate mismatch")
	}

	profile, err := s.ports.Profiles.ResolveProfile(ctx, request.Profile, workspace.Path, request.Grants)
	if err != nil {
		return ExecutionRun{}, operational("resolve profile", err)
	}
	if err := validateProfile(request, workspace, profile); err != nil {
		return ExecutionRun{}, err
	}

	conflict, err := s.ports.Conflicts.VerifyConflict(ctx, request.AuthorityVerdict)
	if err != nil {
		return ExecutionRun{}, operational("verify policy conflict", err)
	}
	authorityMode, witnesses, err := reduceConflict(request, conflict)
	if err != nil {
		return ExecutionRun{}, err
	}

	recorderFacts, recorder, err := s.ports.Recorders.ResolveRecorder(ctx, request.RecorderEndpoint)
	if err != nil {
		return ExecutionRun{}, operational("resolve recorder", err)
	}
	if err := requireProven("recorder", recorderFacts.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if recorder == nil || recorderFacts.Ref != request.RecorderEndpoint {
		return ExecutionRun{}, verdict("recorder binding mismatch")
	}
	checkpoint, err := recorder.Checkpoint(ctx, executionKey(request))
	if err != nil {
		return ExecutionRun{}, operational("query recorder checkpoint", err)
	}
	if err := requireProven("recorder checkpoint", checkpoint.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if request.Action == ActionStart && (len(checkpoint.Revisions) != 0 || checkpoint.EventChainRoot != "" || checkpoint.TerminalSourceSequence != 0 || checkpoint.TerminalGlobalSequence != 0) {
		return ExecutionRun{}, verdict("start recorder checkpoint is not empty")
	}

	var sessionFacts ProviderSessionFacts
	if request.Action == ActionResume {
		expansion, err := s.ports.Expansions.VerifyExpansion(ctx, executionKey(request))
		if err != nil {
			return ExecutionRun{}, operational("verify expansion ledger", err)
		}
		if err := requireProven("expansion ledger", expansion.Verification); err != nil {
			return ExecutionRun{}, err
		}
		if err := validateResumeFacts(request, runway, workspace, profile, authority, checkpoint, expansion); err != nil {
			return ExecutionRun{}, err
		}
		sessionFacts, err = s.verifySession(ctx, request, profile, workspace)
		if err != nil {
			return ExecutionRun{}, err
		}
	}

	opaque, err := s.ports.Opaque.VerifyOpaqueBoundary(ctx, request.Manifest.Opaque)
	if err != nil {
		return ExecutionRun{}, operational("verify opaque boundary", err)
	}
	if err := requireProven("opaque boundary", opaque.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if !reflect.DeepEqual(opaque.Rows, opaqueIdentities(request.Manifest.Opaque)) {
		return ExecutionRun{}, verdict("opaque vendor identities mismatch")
	}
	adapterFacts, err := s.ports.Adapter.VerifyAdapter(ctx, AdapterCheck{Request: request, Profile: profile, Workspace: workspace})
	if err != nil {
		return ExecutionRun{}, operational("verify adapter", err)
	}
	if err := requireProven("adapter", adapterFacts.Verification); err != nil {
		return ExecutionRun{}, err
	}
	if adapterFacts.Adapter != request.Adapter || adapterFacts.AdapterVersion != request.AdapterVersion ||
		adapterFacts.Executable != profile.Executable || adapterFacts.ProfileDigest != profile.Digest ||
		adapterFacts.DecoderProfile != profile.DecoderProfile {
		return ExecutionRun{}, verdict("adapter identity/version/profile/executable mismatch")
	}

	launch := AdapterLaunch{Request: request, Profile: profile, Workspace: workspace, Input: input}
	var adapterResult AdapterResult
	if request.Action == ActionStart {
		adapterResult, err = s.ports.Adapter.Start(ctx, launch)
	} else {
		adapterResult, err = s.ports.Adapter.Resume(ctx, launch, sessionFacts.SessionRef)
	}
	if err != nil {
		return ExecutionRun{}, operational("provider process", err)
	}
	acks, blocked, err := s.recordObservations(ctx, recorder, request, workspace, profile, checkpoint, adapterResult)
	if err != nil {
		return ExecutionRun{}, err
	}
	if request.Action == ActionResume {
		if _, err := s.verifySession(ctx, request, profile, workspace); err != nil {
			return ExecutionRun{}, err
		}
	}
	if adapterResult.OperationalFailure != "" {
		return ExecutionRun{}, operational("provider observation", errors.New(adapterResult.OperationalFailure))
	}
	if request.Action == ActionStart && strings.TrimSpace(adapterResult.ObservedSessionRef) == "" {
		return ExecutionRun{}, verdict("start stream did not establish an adapter session identity")
	}
	if request.Action == ActionResume && adapterResult.ObservedSessionRef != "" && adapterResult.ObservedSessionRef != sessionFacts.SessionRef {
		return ExecutionRun{}, verdict("resumed stream session identity mismatch")
	}
	if blocked != "" {
		return ExecutionRun{}, verdict(blocked)
	}
	sessionRef := adapterResult.ObservedSessionRef
	if request.Action == ActionResume {
		sessionRef = sessionFacts.SessionRef
	}
	return ExecutionRun{Authority: authorityMode, Witnesses: witnesses, Workspace: workspace, Profile: profile, AdapterSessionRef: sessionRef, Acks: acks}, nil
}

// Interrupt requests the adapter's normalized stop path and records its
// acknowledged outcome.
func (s *Service) Interrupt(ctx context.Context, request InterruptRequest) (contextevent.EventAck, error) {
	if ctx == nil {
		return contextevent.EventAck{}, operational("interrupt", errors.New("nil context"))
	}
	if _, err := EncodeExecutionRequest(request.Request); err != nil {
		return contextevent.EventAck{}, operational("interrupt request", err)
	}
	release, err := s.acquire(executionKey(request.Request), "interrupt")
	if err != nil {
		return contextevent.EventAck{}, err
	}
	defer release()
	facts, recorder, err := s.ports.Recorders.ResolveRecorder(ctx, request.Request.RecorderEndpoint)
	if err != nil {
		return contextevent.EventAck{}, operational("interrupt recorder", err)
	}
	if err := requireProven("interrupt recorder", facts.Verification); err != nil {
		return contextevent.EventAck{}, err
	}
	checkpoint, err := recorder.Checkpoint(ctx, executionKey(request.Request))
	if err != nil {
		return contextevent.EventAck{}, operational("interrupt checkpoint", err)
	}
	stop, err := s.ports.Adapter.Stop(ctx, AdapterStopRequest(request))
	if err != nil {
		return contextevent.EventAck{}, operational("adapter stop", err)
	}
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterStop)
	payload := &contextevent.AdapterStopPayload{Schema: schema, Adapter: request.Request.Adapter, AdapterVersion: request.Request.AdapterVersion, Session: request.Request.Session, ExitCode: stop.ExitCode, ReasonCode: stop.ReasonCode}
	stamp, err := s.ports.Stamps.NextStamp(ctx)
	if err != nil {
		return contextevent.EventAck{}, operational("interrupt stamp", err)
	}
	event, err := buildEvent(request.Request, request.Workspace, checkpoint.TerminalSourceSequence+1, terminalRoot(checkpoint), nil, stamp, contextevent.KindAdapterStop, payload)
	if err != nil {
		return contextevent.EventAck{}, operational("encode stop event", err)
	}
	ack, err := recorder.Append(ctx, event)
	if err != nil {
		return contextevent.EventAck{}, operational("append stop event", err)
	}
	if err := validateAck(event, ack, checkpoint.TerminalGlobalSequence); err != nil {
		return contextevent.EventAck{}, operational("acknowledge stop event", err)
	}
	return ack, nil
}

// BeginReplacement reserves the same exclusion key used by start/resume.
func (s *Service) BeginReplacement(key ExecutionKey) (func(), error) {
	return s.acquire(key, "replacement")
}

func (s *Service) acquire(key ExecutionKey, operation string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.active[key]; ok {
		return nil, fmt.Errorf("%w: %s cannot coexist with %s for %v", ErrConcurrentDispatch, operation, current, key)
	}
	s.active[key] = operation
	var once sync.Once
	return func() { once.Do(func() { s.mu.Lock(); delete(s.active, key); s.mu.Unlock() }) }, nil
}

func (s *Service) verifySession(ctx context.Context, request ExecutionRequest, profile ResolvedProfile, workspace WorkspaceFacts) (ProviderSessionFacts, error) {
	want := ProviderSessionCheck{SessionRef: request.Resume.Continuity.AdapterSessionRef, AdapterVersion: request.AdapterVersion, ProfileDigest: profile.Digest, WorkspaceID: workspace.WorkspaceID}
	facts, err := s.ports.Sessions.VerifyProviderSession(ctx, want)
	if err != nil {
		return ProviderSessionFacts{}, operational("verify provider session", err)
	}
	if err := requireProven("provider session", facts.Verification); err != nil {
		return ProviderSessionFacts{}, err
	}
	if facts.SessionRef != want.SessionRef || facts.AdapterVersion != want.AdapterVersion || facts.ProfileDigest != want.ProfileDigest || facts.WorkspaceID != want.WorkspaceID {
		return ProviderSessionFacts{}, verdict("provider session identity/state mismatch")
	}
	return facts, nil
}

func (s *Service) recordObservations(ctx context.Context, recorder Recorder, request ExecutionRequest, workspace WorkspaceFacts, profile ResolvedProfile, checkpoint RecorderCheckpoint, result AdapterResult) ([]contextevent.EventAck, string, error) {
	sequence := uint64(1)
	prior := ""
	priorGlobal := uint64(0)
	if request.Action == ActionResume {
		sequence = checkpoint.TerminalSourceSequence + 1
		prior = terminalRoot(checkpoint)
		priorGlobal = checkpoint.TerminalGlobalSequence
	}
	acks := make([]contextevent.EventAck, 0, len(result.Observations))
	blocked := ""
	for _, observation := range result.Observations {
		stamp, err := s.ports.Stamps.NextStamp(ctx)
		if err != nil {
			return nil, "", operational("observation stamp", err)
		}
		event, err := buildEvent(request, workspace, sequence, prior, nil, stamp, observation.Kind, observation.Payload)
		if err != nil {
			return nil, "", operational("normalize adapter observation", err)
		}
		ack, err := recorder.Append(ctx, event)
		if err != nil {
			return nil, "", operational("append adapter observation", err)
		}
		if err := validateAck(event, ack, priorGlobal); err != nil {
			return nil, "", operational("acknowledge adapter observation", err)
		}
		acks = append(acks, ack)
		if request.Action == ActionStart && observation.Kind == contextevent.KindAdapterStart {
			if result.ObservedSessionRef == "" {
				return nil, "", verdict("adapter-start lacks session identity")
			}
			record := SessionRecord{Key: executionKey(request), SessionRef: result.ObservedSessionRef, AdapterVersion: request.AdapterVersion, ProfileDigest: profile.Digest, WorkspaceID: workspace.WorkspaceID, LifecycleAck: ack}
			if err := s.ports.SessionStore.StoreAdapterSession(ctx, record); err != nil {
				return nil, "", operational("store acknowledged adapter session", err)
			}
		}
		if observation.BlocksAuthority {
			blocked = observation.Witness
			if blocked == "" {
				blocked = "adapter observation blocks authoritative continuation"
			}
		}
		sequence++
		prior, priorGlobal = event.EventDigest, ack.GlobalSequence
	}
	if request.Action == ActionStart && result.OperationalFailure == "" {
		stored := false
		for _, observation := range result.Observations {
			stored = stored || observation.Kind == contextevent.KindAdapterStart
		}
		if !stored {
			return nil, "", verdict("start lifecycle event was not durably recorded")
		}
	}
	return acks, blocked, nil
}

func buildEvent(request ExecutionRequest, workspace WorkspaceFacts, sequence uint64, prior string, priorRevision *contextevent.PriorRevision, stamp string, kind contextevent.Kind, payload any) (contextevent.Event, error) {
	schema, err := contextevent.PayloadSchema(kind)
	if err != nil {
		return contextevent.Event{}, err
	}
	event := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: sequence,
		Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch,
		ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
		Session: request.Session, ATCRunway: request.ATCRunway,
		ExecutionWorkspaceID: workspace.WorkspaceID, CandidateCommit: workspace.CurrentCommit,
		CandidateTree: workspace.CurrentTree, Adapter: request.Adapter, AdapterVersion: request.AdapterVersion,
		OccurredAt: stamp, Kind: kind, PayloadSchema: schema, Payload: payload,
		PriorEventDigest: prior, PriorRevision: priorRevision,
	}
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		return contextevent.Event{}, err
	}
	return contextevent.DecodeEvent(bytes.NewReader(encoded))
}

func validateAck(event contextevent.Event, ack contextevent.EventAck, priorGlobal uint64) error {
	encoded, err := contextevent.EncodeEventAck(ack)
	if err != nil {
		return err
	}
	canonical, err := contextevent.DecodeEventAck(bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	if canonical.Flight != event.Flight || canonical.Lane != event.Lane || canonical.Epoch != event.Epoch || canonical.Session != event.Session ||
		canonical.ManifestRevision != event.ManifestRevision || canonical.Kind != event.Kind || canonical.SourceSequence != event.SourceSequence ||
		canonical.EventDigest != event.EventDigest || canonical.GlobalSequence <= priorGlobal {
		return errors.New("recorder acknowledgment does not bind event identity and monotonic order")
	}
	return nil
}

func validateProfile(request ExecutionRequest, workspace WorkspaceFacts, profile ResolvedProfile) error {
	if err := requireProven("profile", profile.Verification); err != nil {
		return err
	}
	if profile.Ref != request.Profile || profile.Digest != request.Profile.Digest || profile.AdapterVersion != request.AdapterVersion ||
		profile.WorkspacePath != workspace.Path || profile.Name == "" || profile.DecoderProfile == "" ||
		!filepath.IsAbs(profile.Executable) || profile.CodexHome == "" {
		return verdict("resolved profile identity/version/workspace is mismatched")
	}
	if envValueFromProfile(profile.Profile.Env(), "CODEX_HOME") != profile.CodexHome {
		return verdict("resolved profile CODEX_HOME identity is not isolated and exact")
	}
	requested, err := execworkspace.EncodeGrantSet(request.Grants)
	if err != nil {
		return operational("encode requested grants", err)
	}
	resolved, err := execworkspace.EncodeGrantSet(profile.Grants)
	if err != nil {
		return operational("encode resolved grants", err)
	}
	if !bytes.Equal(requested, resolved) || !contains(profile.Profile.AllowedArgv0s, profile.Executable) {
		return verdict("resolved profile grants or executable membership mismatch")
	}
	expectedKinds := make([]execworkspace.GrantKind, len(request.Grants.Grants))
	networkMode := execworkspace.NetworkDeny
	for i, grant := range request.Grants.Grants {
		expectedKinds[i] = grant.Kind
		if grant.Kind == execworkspace.GrantNetwork {
			networkMode = execworkspace.NetworkAllow
		}
	}
	sort.Slice(expectedKinds, func(i, j int) bool { return expectedKinds[i] < expectedKinds[j] })
	if len(profile.Enforcement.Rows) != len(expectedKinds) || !profile.Enforcement.Network.Configured || profile.Enforcement.Network.Mode != networkMode ||
		strings.TrimSpace(profile.Enforcement.Network.Reason) == "" || profile.Enforcement.Network.Reason != strings.TrimSpace(profile.Enforcement.Network.Reason) {
		return verdict("grant enforcement facts are incomplete")
	}
	for i, row := range profile.Enforcement.Rows {
		if row.Kind != expectedKinds[i] || !row.Applied || strings.TrimSpace(row.Reason) == "" || row.Reason != strings.TrimSpace(row.Reason) {
			return verdict("a requested grant was not applied")
		}
	}
	return nil
}

func reduceConflict(request ExecutionRequest, facts ConflictFacts) (contextevent.Authority, []string, error) {
	if err := facts.validate("policy conflict"); err != nil {
		return "", nil, verdict(err.Error())
	}
	want, err := policyconflict.EncodeReport(request.AuthorityVerdict)
	if err != nil {
		return "", nil, operational("encode request conflict report", err)
	}
	got, err := policyconflict.EncodeReport(facts.Report)
	if err != nil {
		return "", nil, operational("encode fresh conflict report", err)
	}
	if !bytes.Equal(want, got) || facts.Report.Digest != request.AuthorityVerdict.Digest {
		return "", nil, verdict("fresh conflict report mismatch")
	}
	switch facts.State {
	case contextcompile.ResolutionProven:
		if facts.Report.Verdict != policyconflict.VerdictPass {
			return "", nil, verdict("policy conflict report is not pass")
		}
		return contextevent.AuthorityAuthoritative, []string{}, nil
	case contextcompile.ResolutionUnproven:
		if len(facts.Witnesses) == 0 {
			return "", nil, verdict("advisory conflict proof lacks explicit witnesses")
		}
		return contextevent.AuthorityAdvisory, append([]string(nil), facts.Witnesses...), nil
	default:
		return "", nil, verdict("policy conflict is violated")
	}
}

func validateResumeFacts(request ExecutionRequest, runway RunwayFacts, workspace WorkspaceFacts, profile ResolvedProfile, authority AuthorityFacts, checkpoint RecorderCheckpoint, expansion ExpansionFacts) error {
	c := request.Resume.Continuity
	grantBytes, err := execworkspace.EncodeGrantSet(profile.Grants)
	if err != nil {
		return operational("resume grant digest", err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(workspace.Request)
	if err != nil {
		return operational("resume workspace digest", err)
	}
	root, err := contextevent.EventChainRoot(checkpoint.Revisions)
	if err != nil {
		return verdict("fresh recorder revision chain is incomplete or invalid")
	}
	if runway.Commit != c.InputCommit || runway.Tree != c.InputTree || !runway.Clean ||
		workspace.WorkspaceID != c.ExecutionWorkspaceID || workspaceDigest != c.ExecutionWorkspaceRequestDigest ||
		workspace.CurrentCommit != c.CurrentCommit || workspace.CurrentTree != c.CurrentTree || !workspace.Clean ||
		profile.Digest != c.ProfileDigest || digestBytes(grantBytes) != c.GrantDigest || authority.AuthorityDigest != c.AuthorityVerdictDigest ||
		authority.ManifestRevision != c.CurrentManifestRevision || authority.ManifestDigest != c.CurrentManifestDigest ||
		authority.ProjectionDigest != c.ProjectionDigest || expansion.Root != c.ExpansionLedgerRoot ||
		checkpoint.Digest != c.RecorderCheckpointDigest || !reflect.DeepEqual(checkpoint.Revisions, c.RevisionSegments) ||
		root != c.EventChainRoot || checkpoint.EventChainRoot != c.EventChainRoot ||
		checkpoint.TerminalSourceSequence != c.TerminalSourceSequence || checkpoint.TerminalGlobalSequence != c.TerminalGlobalSequence {
		return verdict("fresh resume continuity operand mismatch")
	}
	return nil
}

func requireProven(name string, verification Verification) error {
	if err := verification.validate(name); err != nil {
		return verdict(err.Error())
	}
	if verification.State != contextcompile.ResolutionProven {
		return verdict(name + " is not proven")
	}
	return nil
}

func verdict(witness string) error { return fmt.Errorf("%w: %s", ErrVerdict, witness) }
func operational(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrOperational, operation, err)
}

func executionWorkspacePath(path, workspaceID string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != workspaceID {
		return false
	}
	execution := filepath.Dir(path)
	return filepath.Base(execution) == "execution" && filepath.Base(filepath.Dir(execution)) == "data"
}

func opaqueIdentities(rows []contextcompile.OpaqueEntry) []OpaqueIdentity {
	out := make([]OpaqueIdentity, len(rows))
	for i, row := range rows {
		out[i] = OpaqueIdentity{ID: row.ID, Kind: row.Kind, AdapterID: row.Adapter.ID, AdapterVersion: row.Adapter.Version}
	}
	return out
}

func envValueFromProfile(env []string, name string) string {
	prefix := name + "="
	for _, row := range env {
		if strings.HasPrefix(row, prefix) {
			return strings.TrimPrefix(row, prefix)
		}
	}
	return ""
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func terminalRoot(checkpoint RecorderCheckpoint) string {
	if len(checkpoint.Revisions) == 0 {
		return ""
	}
	return checkpoint.Revisions[len(checkpoint.Revisions)-1].EventRoot
}
