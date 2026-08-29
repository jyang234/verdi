package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
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
	// ErrInterrupted marks a run whose ordered adapter-stop was acknowledged.
	// It is always returned together with ErrVerdict.
	ErrInterrupted = errors.New("sealedexec: execution interrupted")
	// ErrInterruptNotActive marks a registered start/resume whose provider
	// stream has not yet been installed when signal normalization attempts a stop.
	// It is always returned together with ErrVerdict.
	ErrInterruptNotActive = errors.New("sealedexec: interrupt target is not active")
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
	PolicySecretValues                  [][]byte
	ClassificationComplete              bool
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
	ActiveRevision         *ActiveRevision
}

// ActiveRevision is the exact durable append position of one in-progress
// manifest revision. Complete checkpoint revisions exclude this tail.
type ActiveRevision struct {
	Revision           uint64
	ManifestDigest     string
	NextSourceSequence uint64
	PriorEventDigest   string
	PriorRevision      *contextevent.PriorRevision
	LastGlobalSequence uint64
	Invalidated        bool
	EventAcks          []contextevent.EventAck
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
	Review    *ReviewLaunch
}

// AdapterLaunch structurally separates immutable instructions from data.
type AdapterLaunch struct {
	Request   ExecutionRequest
	Profile   ResolvedProfile
	Workspace WorkspaceFacts
	Input     ProviderInput
	Review    *ReviewLaunch
}

const ReviewLaunchFactsSchemaID = "verdi.sealed-review-launch-facts/v1"

// ReviewPrior is the exact R2 lineage identity carried by launch facts.
type ReviewPrior struct {
	ReceiptDigest      string `json:"receipt_digest"`
	AdjudicationDigest string `json:"adjudication_digest"`
}

// ReviewLaunch carries the review-only provider launch operands. A nil value
// selects the byte-identical generic builder start path.
type ReviewLaunch struct {
	Round        string
	PacketDigest string
	PriorReview  *ReviewPrior
	Model        string
}

// ReviewLaunchFacts are the exact acknowledged I-97 review-start projection.
type ReviewLaunchFacts struct {
	Schema         string               `json:"schema"`
	Round          string               `json:"round"`
	PacketDigest   string               `json:"packet_digest"`
	PriorReview    *ReviewPrior         `json:"prior_review"`
	Lane           string               `json:"lane"`
	Adapter        contextevent.Adapter `json:"adapter"`
	AdapterVersion string               `json:"adapter_version"`
	Model          string               `json:"model"`
	ProfileID      string               `json:"profile_id"`
	ProfileDigest  string               `json:"profile_digest"`
	Session        string               `json:"session"`
	WorkspaceID    string               `json:"workspace_id"`
}

// EncodeReviewLaunchFacts validates and canonically encodes launch facts.
func EncodeReviewLaunchFacts(facts ReviewLaunchFacts) ([]byte, error) {
	if err := validateReviewLaunchFacts(facts); err != nil {
		return nil, err
	}
	encoded, err := canonjson.Marshal(facts)
	return bytes.TrimSuffix(encoded, []byte("\n")), err
}

// DecodeReviewLaunchFacts strictly decodes one canonical launch-facts document.
func DecodeReviewLaunchFacts(reader io.Reader) (ReviewLaunchFacts, error) {
	if reader == nil {
		return ReviewLaunchFacts{}, errors.New("sealedexec: decode review launch facts: nil reader")
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return ReviewLaunchFacts{}, fmt.Errorf("sealedexec: read review launch facts: %w", err)
	}
	var facts ReviewLaunchFacts
	if err := artifact.DecodeExactJSON(raw, &facts); err != nil {
		return ReviewLaunchFacts{}, fmt.Errorf("sealedexec: decode review launch facts: %w", err)
	}
	if err := validateReviewLaunchFacts(facts); err != nil {
		return ReviewLaunchFacts{}, err
	}
	canonical, err := canonjson.Marshal(facts)
	if err != nil {
		return ReviewLaunchFacts{}, err
	}
	canonical = bytes.TrimSuffix(canonical, []byte("\n"))
	if !bytes.Equal(raw, canonical) {
		return ReviewLaunchFacts{}, errors.New("sealedexec: review launch facts are not byte-canonical")
	}
	return cloneReviewLaunchFacts(facts), nil
}

func validateReviewLaunchFacts(facts ReviewLaunchFacts) error {
	if facts.Schema != ReviewLaunchFactsSchemaID {
		return fmt.Errorf("sealedexec: review launch facts schema must be %q", ReviewLaunchFactsSchemaID)
	}
	if facts.Round != "r0" && facts.Round != "r2" {
		return fmt.Errorf("sealedexec: review launch facts has unknown round %q", facts.Round)
	}
	for field, value := range map[string]string{
		"lane": facts.Lane, "adapter_version": facts.AdapterVersion, "model": facts.Model,
		"profile_id": facts.ProfileID, "session": facts.Session, "workspace_id": facts.WorkspaceID,
	} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := facts.Adapter.Validate(); err != nil {
		return fmt.Errorf("sealedexec: review launch facts: %w", err)
	}
	if err := validateDigest("packet_digest", facts.PacketDigest); err != nil {
		return err
	}
	if err := validateDigest("profile_digest", facts.ProfileDigest); err != nil {
		return err
	}
	if facts.Round == "r0" {
		if facts.PriorReview != nil {
			return errors.New("sealedexec: R0 review launch facts forbid prior_review")
		}
		return nil
	}
	if facts.PriorReview == nil {
		return errors.New("sealedexec: R2 review launch facts require prior_review")
	}
	if err := validateDigest("prior_review.receipt_digest", facts.PriorReview.ReceiptDigest); err != nil {
		return err
	}
	return validateDigest("prior_review.adjudication_digest", facts.PriorReview.AdjudicationDigest)
}

func cloneReviewLaunchFacts(facts ReviewLaunchFacts) ReviewLaunchFacts {
	if facts.PriorReview != nil {
		prior := *facts.PriorReview
		facts.PriorReview = &prior
	}
	return facts
}

// AdapterTerminalResult is the explicit terminal process result. A provider
// observation is never inferred to be terminal merely because Next blocks or
// returns no normalized events.
type AdapterTerminalResult struct {
	ExitCode int
}

// AdapterResult is one pull from an acknowledged active-run stream. One
// non-terminal result represents exactly one foreign provider observation;
// normalization may map that observation to more than one canonical event.
type AdapterResult struct {
	ObservedSessionRef string
	Observations       []NormalizedObservation
	OperationalFailure string
	Terminal           *AdapterTerminalResult
	Stopped            *AdapterStopResult
}

// ActiveAdapterRun is consumer-driven: the service does not request the next
// provider observation until every event from the current result is durably
// acknowledged.
type ActiveAdapterRun interface {
	Next(context.Context) (AdapterResult, error)
	Stop(context.Context) (AdapterStopResult, error)
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
	ReviewLaunchFacts *ReviewLaunchFacts
	ReviewLaunchEvent *contextevent.Event
	ReviewLaunchAck   *contextevent.EventAck
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
	Start(context.Context, AdapterLaunch) (ActiveAdapterRun, error)
	Resume(context.Context, AdapterLaunch, string) (ActiveAdapterRun, error)
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
	active map[ExecutionKey]*activeExecution
}

type activeExecution struct {
	mu               sync.Mutex
	operation        string
	stream           ActiveAdapterRun
	request          ExecutionRequest
	canonicalRequest []byte
	workspace        WorkspaceFacts
	profile          ResolvedProfile
	recorder         Recorder
	sequence         uint64
	priorDigest      string
	priorGlobal      uint64
	sessionRef       string
	review           *ReviewLaunch
	reviewFacts      *ReviewLaunchFacts
	reviewEvent      *contextevent.Event
	reviewAck        *contextevent.EventAck
	acks             []contextevent.EventAck
	state            activeExecutionState
	stopIssued       chan struct{}
	stopIssueOnce    sync.Once
	stopRequestErr   error
	terminalCause    error
	done             chan struct{}
	stopAck          contextevent.EventAck
	terminalErr      error
	resumeRechecked  bool
}

type activeExecutionState uint8

const (
	activePreparing activeExecutionState = iota
	activeRunning
	activeStopRequested
	activeTerminal
)

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
	return &Service{ports: ports, active: make(map[ExecutionKey]*activeExecution)}, nil
}

// Execute verifies every prerequisite and launches last.
func (s *Service) Execute(ctx context.Context, request ExecutionRequest, data []contextcompile.DataItem) (ExecutionRun, error) {
	return s.execute(ctx, request, data, nil)
}

// ExecuteReview verifies and launches one fresh start-only sealed review.
func (s *Service) ExecuteReview(ctx context.Context, request ExecutionRequest, data []contextcompile.DataItem, review ReviewLaunch) (ExecutionRun, error) {
	if request.Action != ActionStart || request.Resume != nil {
		return ExecutionRun{}, verdict("sealed review is start-only")
	}
	if err := validateReviewLaunch(review); err != nil {
		return ExecutionRun{}, operational("validate review launch", err)
	}
	return s.execute(ctx, request, data, &review)
}

func (s *Service) execute(ctx context.Context, request ExecutionRequest, data []contextcompile.DataItem, review *ReviewLaunch) (ExecutionRun, error) {
	if ctx == nil {
		return ExecutionRun{}, operational("execute", errors.New("nil context"))
	}
	active, release, err := s.acquire(executionKey(request), string(request.Action))
	if err != nil {
		return ExecutionRun{}, err
	}
	defer release()
	canonicalRequest, err := EncodeExecutionRequest(request)
	if err != nil {
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
	if request.Action == ActionStart && (checkpoint.ActiveRevision != nil || len(checkpoint.Revisions) != 0 || checkpoint.EventChainRoot != "" || checkpoint.TerminalSourceSequence != 0 || checkpoint.TerminalGlobalSequence != 0) {
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
	adapterFacts, err := s.ports.Adapter.VerifyAdapter(ctx, AdapterCheck{Request: request, Profile: profile, Workspace: workspace, Review: cloneReviewLaunch(review)})
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

	launch := AdapterLaunch{Request: request, Profile: profile, Workspace: workspace, Input: input, Review: cloneReviewLaunch(review)}
	partial := ExecutionRun{
		Authority: authorityMode, Witnesses: append([]string(nil), witnesses...),
		Workspace: workspace, Profile: profile,
	}
	var stream ActiveAdapterRun
	if request.Action == ActionStart {
		stream, err = s.ports.Adapter.Start(ctx, launch)
	} else {
		stream, err = s.ports.Adapter.Resume(ctx, launch, sessionFacts.SessionRef)
	}
	if err != nil {
		return partial, operational("provider process", err)
	}
	if stream == nil {
		return partial, operational("provider process", errors.New("adapter returned a nil active run"))
	}
	sequence, priorDigest, priorGlobal := uint64(1), "", uint64(0)
	if request.Action == ActionResume {
		sequence = checkpoint.TerminalSourceSequence + 1
		priorDigest = terminalRoot(checkpoint)
		priorGlobal = checkpoint.TerminalGlobalSequence
	}
	s.activate(active, stream, request, canonicalRequest, workspace, profile, recorder, sequence, priorDigest, priorGlobal, sessionFacts.SessionRef, review)
	return s.consumeActive(ctx, active, authorityMode, witnesses)
}

// Interrupt requests the adapter's normalized stop path. The sole stream
// consumer records the stop after acknowledging any already-observed frame;
// Interrupt waits for and returns that exact acknowledgment.
func (s *Service) Interrupt(ctx context.Context, request InterruptRequest) (contextevent.EventAck, error) {
	if ctx == nil {
		return contextevent.EventAck{}, operational("interrupt", errors.New("nil context"))
	}
	canonicalRequest, err := EncodeExecutionRequest(request.Request)
	if err != nil {
		return contextevent.EventAck{}, operational("interrupt request", err)
	}
	active := s.activeRun(executionKey(request.Request))
	if active == nil {
		return contextevent.EventAck{}, verdict("interrupt has no matching active run")
	}
	active.mu.Lock()
	if active.state != activeRunning {
		active.mu.Unlock()
		return contextevent.EventAck{}, verdict("interrupt has no stoppable active run")
	}
	if !bytes.Equal(canonicalRequest, active.canonicalRequest) || !reflect.DeepEqual(request.Workspace, active.workspace) || !interruptSessionMatches(active, request.AdapterSessionRef) {
		active.mu.Unlock()
		return contextevent.EventAck{}, verdict("interrupt identity does not match the verified active run")
	}
	active.state = activeStopRequested
	active.terminalCause = interruptedVerdict()
	done := active.done
	active.mu.Unlock()

	s.issueStop(context.WithoutCancel(ctx), active)
	select {
	case <-done:
	case <-ctx.Done():
		// A simultaneously published terminal result wins over caller
		// cancellation; otherwise the accepted run-bound stop continues.
		select {
		case <-done:
		default:
			return contextevent.EventAck{}, operational("interrupt wait", ctx.Err())
		}
	}
	active.mu.Lock()
	defer active.mu.Unlock()
	if active.stopAck != (contextevent.EventAck{}) {
		return active.stopAck, nil
	}
	if active.terminalErr != nil {
		return contextevent.EventAck{}, active.terminalErr
	}
	return contextevent.EventAck{}, operational("interrupt", errors.New("adapter-stop was not acknowledged"))
}

// InterruptRegistered snapshots the already-verified workspace and session
// facts of the matching active run and delegates to the strict interrupt
// identity check. It does not construct an identity when no provider is active.
func (s *Service) InterruptRegistered(ctx context.Context, request ExecutionRequest) (contextevent.EventAck, error) {
	s.mu.Lock()
	active := s.active[executionKey(request)]
	if active == nil || (active.operation != string(ActionStart) && active.operation != string(ActionResume)) {
		s.mu.Unlock()
		return contextevent.EventAck{}, verdict("interrupt has no matching active run")
	}
	active.mu.Lock()
	s.mu.Unlock()
	if active.stream == nil {
		active.mu.Unlock()
		return contextevent.EventAck{}, fmt.Errorf("%w: %w", ErrVerdict, ErrInterruptNotActive)
	}
	workspace := active.workspace
	sessionRef := active.sessionRef
	active.mu.Unlock()
	return s.Interrupt(ctx, InterruptRequest{Request: request, Workspace: workspace, AdapterSessionRef: sessionRef})
}

func interruptSessionMatches(active *activeExecution, operand string) bool {
	if active.request.Action == ActionStart && active.sessionRef == "" {
		return operand == ""
	}
	return operand != "" && operand == active.sessionRef
}

// BeginReplacement reserves the same exclusion key used by start/resume.
func (s *Service) BeginReplacement(key ExecutionKey) (func(), error) {
	_, release, err := s.acquire(key, "replacement")
	return release, err
}

func (s *Service) acquire(key ExecutionKey, operation string) (*activeExecution, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current, ok := s.active[key]; ok {
		return nil, nil, fmt.Errorf("%w: %s cannot coexist with %s for %v", ErrConcurrentDispatch, operation, current.operation, key)
	}
	active := &activeExecution{operation: operation}
	s.active[key] = active
	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			if s.active[key] == active {
				delete(s.active, key)
			}
			s.mu.Unlock()
		})
	}
	return active, release, nil
}

func (s *Service) activate(active *activeExecution, stream ActiveAdapterRun, request ExecutionRequest, canonicalRequest []byte, workspace WorkspaceFacts, profile ResolvedProfile, recorder Recorder, sequence uint64, priorDigest string, priorGlobal uint64, sessionRef string, review *ReviewLaunch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	active.mu.Lock()
	defer active.mu.Unlock()
	active.stream = stream
	active.request = request
	active.canonicalRequest = append([]byte(nil), canonicalRequest...)
	active.workspace = workspace
	active.profile = profile
	active.recorder = recorder
	active.sequence = sequence
	active.priorDigest = priorDigest
	active.priorGlobal = priorGlobal
	active.sessionRef = sessionRef
	active.review = cloneReviewLaunch(review)
	active.acks = []contextevent.EventAck{}
	active.state = activeRunning
	active.stopIssued = make(chan struct{})
	active.done = make(chan struct{})
}

func (s *Service) activeRun(key ExecutionKey) *activeExecution {
	s.mu.Lock()
	defer s.mu.Unlock()
	active := s.active[key]
	if active == nil || active.stream == nil {
		return nil
	}
	return active
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

func (s *Service) consumeActive(ctx context.Context, active *activeExecution, authority contextevent.Authority, witnesses []string) (ExecutionRun, error) {
	for {
		active.mu.Lock()
		state, stopIssued := active.state, active.stopIssued
		active.mu.Unlock()
		if state == activeStopRequested {
			<-stopIssued
			active.mu.Lock()
			stopRequestErr := active.stopRequestErr
			active.mu.Unlock()
			if stopRequestErr != nil {
				return s.finishWithoutStopAck(active, authority, witnesses, operational("request adapter stop", stopRequestErr))
			}
		}

		result, nextErr := active.stream.Next(ctx)
		if nextErr != nil {
			failure := operational("provider stream", nextErr)
			active.mu.Lock()
			alreadyStopping := active.state == activeStopRequested
			if alreadyStopping {
				active.terminalCause = errors.Join(active.terminalCause, failure)
			}
			active.mu.Unlock()
			if alreadyStopping {
				continue
			}
			s.requestFailureStop(context.WithoutCancel(ctx), active, failure)
			continue
		}
		if result.Stopped != nil {
			if result.Terminal != nil || result.ObservedSessionRef != "" || len(result.Observations) != 0 || result.OperationalFailure != "" {
				failure := operational("provider stream", errors.New("stop terminal/result union is invalid"))
				s.requestFailureStop(context.WithoutCancel(ctx), active, failure)
				return s.finishWithoutStopAck(active, authority, witnesses, failure)
			}
			active.mu.Lock()
			stopping := active.state == activeStopRequested
			active.mu.Unlock()
			if !stopping {
				failure := operational("provider stream", errors.New("unsolicited stop terminal"))
				s.requestFailureStop(context.WithoutCancel(ctx), active, failure)
				return s.finishWithoutStopAck(active, authority, witnesses, failure)
			}
			ack, err := s.recordStop(context.WithoutCancel(ctx), active, *result.Stopped)
			if err != nil {
				return s.finishWithoutStopAck(active, authority, witnesses, err)
			}
			active.mu.Lock()
			cause := active.terminalCause
			active.mu.Unlock()
			return s.finishActive(active, authority, witnesses, ack, cause)
		}
		if result.Terminal == nil && len(result.Observations) == 0 {
			failure := operational("provider stream", errors.New("non-terminal result has no normalized observations"))
			s.requestFailureStop(context.WithoutCancel(ctx), active, failure)
			continue
		}
		blocked, recorderUsable, recordErr := s.recordResult(ctx, active, result)
		if recordErr != nil {
			s.requestFailureStop(context.WithoutCancel(ctx), active, recordErr)
			if !recorderUsable {
				return s.finishWithoutStopAck(active, authority, witnesses, errors.New("recorder rejected activity; adapter-stop was not acknowledged"))
			}
			continue
		}
		var failure error
		if active.request.Action == ActionResume && !active.resumeRechecked {
			if _, err := s.verifySession(ctx, active.request, active.profile, active.workspace); err != nil {
				failure = err
			}
			active.resumeRechecked = true
		}
		if result.OperationalFailure != "" && failure == nil {
			failure = operational("provider observation", errors.New(result.OperationalFailure))
		}
		active.mu.Lock()
		sessionRef := active.sessionRef
		active.mu.Unlock()
		if active.request.Action == ActionResume && result.ObservedSessionRef != "" && result.ObservedSessionRef != sessionRef && failure == nil {
			failure = verdict("resumed stream session identity mismatch")
		}
		if blocked != "" && failure == nil {
			failure = verdict(blocked)
		}
		if result.Terminal != nil && result.Terminal.ExitCode != 0 && failure == nil {
			failure = operational("provider process", fmt.Errorf("unreported nonzero exit %d", result.Terminal.ExitCode))
		}
		if result.Terminal != nil && active.request.Action == ActionStart && sessionRef == "" && failure == nil {
			failure = verdict("start stream did not establish an acknowledged adapter session identity")
		}
		if failure != nil {
			s.requestFailureStop(context.WithoutCancel(ctx), active, failure)
			continue
		}
		active.mu.Lock()
		stopping := active.state == activeStopRequested
		active.mu.Unlock()
		if stopping {
			if result.Terminal != nil {
				return s.finishWithoutStopAck(active, authority, witnesses, operational("provider stream", errors.New("natural terminal replaced requested stop terminal")))
			}
			continue
		}
		if result.Terminal != nil {
			if run, err, finished := s.tryFinishNormally(active, authority, witnesses); finished {
				return run, err
			}
		}
	}
}

func (s *Service) recordResult(ctx context.Context, active *activeExecution, result AdapterResult) (string, bool, error) {
	blocked := ""
	for _, observation := range result.Observations {
		var reviewFacts *ReviewLaunchFacts
		if observation.Kind == contextevent.KindAdapterStart {
			facts, err := reviewFactsFromObservation(active, observation)
			if err != nil {
				return "", true, err
			}
			reviewFacts = facts
		}
		lockSessionBoundary := active.request.Action == ActionStart && observation.Kind == contextevent.KindAdapterStart
		if lockSessionBoundary {
			active.mu.Lock()
		}
		stamp, err := s.ports.Stamps.NextStamp(ctx)
		if err != nil {
			if lockSessionBoundary {
				active.mu.Unlock()
			}
			return "", true, operational("observation stamp", err)
		}
		event, err := buildEvent(active.request, active.workspace, active.sequence, active.priorDigest, nil, stamp, observation.Kind, observation.Payload)
		if err != nil {
			if lockSessionBoundary {
				active.mu.Unlock()
			}
			return "", true, operational("normalize adapter observation", err)
		}
		ack, err := active.recorder.Append(ctx, event)
		if err != nil {
			if lockSessionBoundary {
				active.mu.Unlock()
			}
			return "", false, operational("append adapter observation", err)
		}
		if err := validateAck(event, ack, active.priorGlobal); err != nil {
			if lockSessionBoundary {
				active.mu.Unlock()
			}
			return "", false, operational("acknowledge adapter observation", err)
		}
		active.acks = append(active.acks, ack)
		active.sequence++
		active.priorDigest, active.priorGlobal = event.EventDigest, ack.GlobalSequence
		if active.request.Action == ActionStart && observation.Kind == contextevent.KindAdapterStart {
			if result.ObservedSessionRef == "" || (active.sessionRef != "" && active.sessionRef != result.ObservedSessionRef) {
				active.mu.Unlock()
				return "", true, verdict("adapter-start lacks one stable session identity")
			}
			record := SessionRecord{Key: executionKey(active.request), SessionRef: result.ObservedSessionRef, AdapterVersion: active.request.AdapterVersion, ProfileDigest: active.profile.Digest, WorkspaceID: active.workspace.WorkspaceID, LifecycleAck: ack}
			if err := s.ports.SessionStore.StoreAdapterSession(ctx, record); err != nil {
				active.mu.Unlock()
				return "", true, operational("store acknowledged adapter session", err)
			}
			active.sessionRef = result.ObservedSessionRef
			active.reviewFacts = reviewFacts
			if reviewFacts != nil {
				eventCopy, ackCopy := event, ack
				active.reviewEvent, active.reviewAck = &eventCopy, &ackCopy
			}
			active.mu.Unlock()
		}
		if observation.BlocksAuthority && blocked == "" {
			blocked = observation.Witness
			if blocked == "" {
				blocked = "adapter observation blocks authoritative continuation"
			}
		}
	}
	return blocked, true, nil
}

func (s *Service) requestFailureStop(ctx context.Context, active *activeExecution, failure error) {
	active.mu.Lock()
	owner := active.state == activeRunning
	if owner {
		active.state = activeStopRequested
		active.terminalCause = failure
	} else if active.state == activeStopRequested {
		active.terminalCause = errors.Join(active.terminalCause, failure)
	}
	active.mu.Unlock()
	if owner {
		s.issueStop(ctx, active)
	}
}

func (s *Service) issueStop(ctx context.Context, active *activeExecution) {
	active.stopIssueOnce.Do(func() {
		_, err := active.stream.Stop(ctx)
		active.mu.Lock()
		active.stopRequestErr = err
		active.mu.Unlock()
		close(active.stopIssued)
	})
}

func (s *Service) recordStop(ctx context.Context, active *activeExecution, stop AdapterStopResult) (contextevent.EventAck, error) {
	schema, _ := contextevent.PayloadSchema(contextevent.KindAdapterStop)
	payload := &contextevent.AdapterStopPayload{Schema: schema, Adapter: active.request.Adapter, AdapterVersion: active.request.AdapterVersion, Session: active.request.Session, ExitCode: stop.ExitCode, ReasonCode: stop.ReasonCode}
	stamp, err := s.ports.Stamps.NextStamp(ctx)
	if err != nil {
		return contextevent.EventAck{}, operational("stop stamp", err)
	}
	event, err := buildEvent(active.request, active.workspace, active.sequence, active.priorDigest, nil, stamp, contextevent.KindAdapterStop, payload)
	if err != nil {
		return contextevent.EventAck{}, operational("encode stop event", err)
	}
	ack, err := active.recorder.Append(ctx, event)
	if err != nil {
		return contextevent.EventAck{}, operational("append stop event", err)
	}
	if err := validateAck(event, ack, active.priorGlobal); err != nil {
		return contextevent.EventAck{}, operational("acknowledge stop event", err)
	}
	active.acks = append(active.acks, ack)
	active.sequence++
	active.priorDigest, active.priorGlobal = event.EventDigest, ack.GlobalSequence
	return ack, nil
}

func (s *Service) finishWithoutStopAck(active *activeExecution, authority contextevent.Authority, witnesses []string, failure error) (ExecutionRun, error) {
	active.mu.Lock()
	state, stopIssued := active.state, active.stopIssued
	active.mu.Unlock()
	if state == activeStopRequested && stopIssued != nil {
		<-stopIssued
	}
	active.mu.Lock()
	cause := errors.Join(active.terminalCause, failure, operational("unacknowledged stop", errors.New("adapter-stop was not acknowledged")))
	active.mu.Unlock()
	return s.finishActive(active, authority, witnesses, contextevent.EventAck{}, cause)
}

func (s *Service) finishActive(active *activeExecution, authority contextevent.Authority, witnesses []string, stopAck contextevent.EventAck, terminalErr error) (ExecutionRun, error) {
	active.mu.Lock()
	if terminalErr != nil {
		authority = contextevent.AuthorityAdvisory
		if errors.Is(terminalErr, ErrInterrupted) {
			witnesses = appendUnique(witnesses, "execution interrupted")
		} else {
			witnesses = appendUnique(witnesses, "execution did not reach an authoritative terminal")
		}
	}
	active.state = activeTerminal
	active.stopAck = stopAck
	active.terminalErr = terminalErr
	run := activeRunResult(active, authority, witnesses)
	close(active.done)
	active.mu.Unlock()
	return run, terminalErr
}

func (s *Service) tryFinishNormally(active *activeExecution, authority contextevent.Authority, witnesses []string) (ExecutionRun, error, bool) {
	active.mu.Lock()
	defer active.mu.Unlock()
	if active.state != activeRunning {
		return ExecutionRun{}, nil, false
	}
	active.state = activeTerminal
	active.stopIssueOnce.Do(func() { close(active.stopIssued) })
	run := activeRunResult(active, authority, witnesses)
	close(active.done)
	return run, nil, true
}

func activeRunResult(active *activeExecution, authority contextevent.Authority, witnesses []string) ExecutionRun {
	var reviewFacts *ReviewLaunchFacts
	var reviewEvent *contextevent.Event
	var reviewAck *contextevent.EventAck
	if active.reviewFacts != nil {
		facts := cloneReviewLaunchFacts(*active.reviewFacts)
		reviewFacts = &facts
	}
	if active.reviewEvent != nil {
		event := *active.reviewEvent
		reviewEvent = &event
	}
	if active.reviewAck != nil {
		ack := *active.reviewAck
		reviewAck = &ack
	}
	return ExecutionRun{
		Authority: authority, Witnesses: append([]string(nil), witnesses...),
		Workspace: active.workspace, Profile: active.profile,
		AdapterSessionRef: active.sessionRef, ReviewLaunchFacts: reviewFacts,
		ReviewLaunchEvent: reviewEvent, ReviewLaunchAck: reviewAck,
		Acks: append([]contextevent.EventAck(nil), active.acks...),
	}
}

func validateReviewLaunch(review ReviewLaunch) error {
	if review.Round != "r0" && review.Round != "r2" {
		return fmt.Errorf("unknown review round %q", review.Round)
	}
	if err := validateDigest("review packet_digest", review.PacketDigest); err != nil {
		return err
	}
	if err := requireText("review model", review.Model); err != nil {
		return err
	}
	if review.Round == "r0" {
		if review.PriorReview != nil {
			return errors.New("R0 review launch forbids prior review")
		}
		return nil
	}
	if review.PriorReview == nil {
		return errors.New("R2 review launch requires prior review")
	}
	if err := validateDigest("review prior receipt", review.PriorReview.ReceiptDigest); err != nil {
		return err
	}
	return validateDigest("review prior adjudication", review.PriorReview.AdjudicationDigest)
}

func cloneReviewLaunch(review *ReviewLaunch) *ReviewLaunch {
	if review == nil {
		return nil
	}
	cloned := *review
	if review.PriorReview != nil {
		prior := *review.PriorReview
		cloned.PriorReview = &prior
	}
	return &cloned
}

func reviewFactsFromObservation(active *activeExecution, observation NormalizedObservation) (*ReviewLaunchFacts, error) {
	payload, ok := observation.Payload.(*contextevent.AdapterStartPayload)
	if !ok {
		return nil, operational("review adapter-start facts", fmt.Errorf("unexpected payload %T", observation.Payload))
	}
	if active.review == nil {
		if payload.Detail != nil {
			return nil, verdict("builder adapter-start carries review-only launch facts")
		}
		return nil, nil
	}
	if payload.Detail == nil {
		return nil, operational("review adapter-start facts", errors.New("acknowledged launch facts are missing"))
	}
	facts, err := DecodeReviewLaunchFacts(bytes.NewReader(payload.Detail.RedactedJSON))
	if err != nil {
		return nil, operational("review adapter-start facts", err)
	}
	want := ReviewLaunchFacts{
		Schema: ReviewLaunchFactsSchemaID, Round: active.review.Round, PacketDigest: active.review.PacketDigest,
		PriorReview: active.review.PriorReview, Lane: active.request.Lane, Adapter: active.request.Adapter,
		AdapterVersion: active.request.AdapterVersion, Model: active.review.Model, ProfileID: active.request.Profile.ID,
		ProfileDigest: active.profile.Digest, Session: active.request.Session, WorkspaceID: active.workspace.WorkspaceID,
	}
	wantBytes, err := EncodeReviewLaunchFacts(want)
	if err != nil {
		return nil, operational("review expected launch facts", err)
	}
	gotBytes, err := EncodeReviewLaunchFacts(facts)
	if err != nil {
		return nil, operational("review observed launch facts", err)
	}
	if !bytes.Equal(wantBytes, gotBytes) {
		return nil, verdict("review launch facts contradict verified request and workspace")
	}
	canonical := cloneReviewLaunchFacts(facts)
	return &canonical, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return append([]string(nil), values...)
		}
	}
	return append(append([]string(nil), values...), value)
}

func interruptedVerdict() error {
	return fmt.Errorf("%w: %w", ErrVerdict, ErrInterrupted)
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
	if !filepath.IsAbs(profile.CodexHome) || filepath.Clean(profile.CodexHome) != profile.CodexHome ||
		envValueFromProfile(profile.Profile.Env(), "CODEX_HOME") != profile.CodexHome {
		return verdict("resolved profile CODEX_HOME identity is not isolated and exact")
	}
	if !profile.ClassificationComplete || profile.PolicySecretValues == nil {
		return verdict("resolved profile secret classification is incomplete")
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
	if checkpoint.ActiveRevision != nil || runway.Commit != c.InputCommit || runway.Tree != c.InputTree || !runway.Clean ||
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
