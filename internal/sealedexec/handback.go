package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/gitx"
)

// HandbackPhase selects one closed I-78 terminal row.
type HandbackPhase string

const (
	HandbackPhaseCompleted                      HandbackPhase = "completed"
	HandbackPhaseExecutionIncompleteVerdict     HandbackPhase = "execution-incomplete-verdict"
	HandbackPhaseExecutionIncompleteOperational HandbackPhase = "execution-incomplete-operational"
	HandbackPhaseTerminalDurabilityFailed       HandbackPhase = "terminal-durability-failed"
	HandbackPhaseOutputWriteFailed              HandbackPhase = "output-write-failed"
	HandbackPhaseAbort                          HandbackPhase = "abort"
)

// RepositoryState is one fresh Git HEAD/tree/clean observation.
type RepositoryState struct {
	Path   string
	Commit string
	Tree   string
	Clean  bool
}

// HandbackRepository owns all fresh Git facts and the sole permitted runway
// mutation. Implementations must route FastForwardOnly to gitx.FastForwardOnly.
type HandbackRepository interface {
	Observe(context.Context, string) (RepositoryState, error)
	IsAncestor(context.Context, string, string, string) (bool, error)
	Diff(context.Context, string, string, string) ([]gitx.DiffEntry, error)
	FastForwardOnly(context.Context, string, string) (gitx.FastForwardResult, error)
}

// HandbackControlStore durably persists the private release-authorizing
// records. A release is forbidden until a matching handback or abort ack.
type HandbackControlStore interface {
	Usable() bool
	PersistHandback(context.Context, HandbackRecord) (ControlAck, error)
	PersistQuarantine(context.Context, QuarantineRecord, []byte) (ControlAck, error)
	PersistAbort(context.Context, AbortRecord) (ControlAck, error)
}

// WorkspaceReleasePort records the execution-workspace release fact.
type WorkspaceReleasePort interface {
	Release(context.Context, string) error
}

// HandbackPorts are the consumer-owned repository, control, and release seams.
type HandbackPorts struct {
	Repository HandbackRepository
	Control    HandbackControlStore
	Releaser   WorkspaceReleasePort
}

// HandbackRequest carries one completed, incomplete, failed-output, or abort
// phase. Preserved references must digest the exact carried bytes.
type HandbackRequest struct {
	Phase         HandbackPhase
	Request       ExecutionRequest
	Run           ExecutionRun
	Completion    *Completion
	PartialBytes  []byte
	Preserved     PreservedExecution
	Quarantine    *QuarantineRecord
	OwnerDecision LogicalRef
}

// HandbackOutcome retains inspectable result bytes and any durable control
// record even when a later Git, persistence, or release operation fails.
type HandbackOutcome struct {
	ExitCode    int
	ResultBytes []byte
	Handback    *HandbackRecord
	Quarantine  *QuarantineRecord
	Abort       *AbortRecord
	ControlAck  ControlAck
	Released    bool
}

// HandbackService owns the guarded fast-forward and release order.
type HandbackService struct {
	ports HandbackPorts
}

// NewHandbackService rejects an incomplete handback dependency set.
func NewHandbackService(ports HandbackPorts) (*HandbackService, error) {
	missing := make([]string, 0, 3)
	if ports.Repository == nil {
		missing = append(missing, "repository")
	}
	if ports.Control == nil {
		missing = append(missing, "control")
	}
	if ports.Releaser == nil {
		missing = append(missing, "releaser")
	}
	if len(missing) != 0 {
		return nil, fmt.Errorf("sealedexec: new handback service: missing ports %v", missing)
	}
	return &HandbackService{ports: ports}, nil
}

// Apply executes exactly one I-78 terminal row. A quarantine never releases.
func (s *HandbackService) Apply(ctx context.Context, input HandbackRequest) (HandbackOutcome, error) {
	if ctx == nil {
		return HandbackOutcome{ExitCode: 2}, operational("apply execution handback", errors.New("nil context"))
	}
	if _, err := EncodeExecutionRequest(input.Request); err != nil {
		return HandbackOutcome{ExitCode: 2}, operational("apply execution handback request", err)
	}
	allowEmptyAcks := input.Phase == HandbackPhaseExecutionIncompleteVerdict || input.Phase == HandbackPhaseExecutionIncompleteOperational ||
		input.Phase == HandbackPhaseTerminalDurabilityFailed || input.Phase == HandbackPhaseAbort
	if err := validateHandbackRunIdentity(input.Request, input.Run, allowEmptyAcks); err != nil {
		return HandbackOutcome{ExitCode: handbackErrorExit(err)}, err
	}
	if input.Phase == HandbackPhaseAbort {
		return s.applyAbort(ctx, input)
	}

	resultBytes, canonicalResult, err := validatePreservedHandbackInput(input)
	if err != nil {
		return HandbackOutcome{ExitCode: handbackErrorExit(err)}, err
	}
	base := HandbackOutcome{ResultBytes: resultBytes}
	switch input.Phase {
	case HandbackPhaseCompleted:
		if input.Completion == nil {
			return HandbackOutcome{ExitCode: 2}, operational("apply completed handback", errors.New("missing durable completion"))
		}
		if canonicalResult.Authority == contextevent.AuthorityAdvisory {
			return s.persistNoHandbackQuarantine(ctx, input, base, QuarantineNonAuthoritative, ErrVerdict)
		}
		if canonicalResult.Authority != contextevent.AuthorityAuthoritative {
			return HandbackOutcome{ExitCode: 2, ResultBytes: resultBytes}, operational("apply completed handback", fmt.Errorf("unknown authority %q", canonicalResult.Authority))
		}
		return s.applyAuthoritative(ctx, input, base)
	case HandbackPhaseExecutionIncompleteVerdict:
		return s.persistIncompleteQuarantine(ctx, input, base, QuarantineExecutionIncomplete, ErrVerdict)
	case HandbackPhaseExecutionIncompleteOperational:
		return s.persistIncompleteQuarantine(ctx, input, base, QuarantineExecutionIncomplete, ErrOperational)
	case HandbackPhaseTerminalDurabilityFailed:
		return s.persistIncompleteQuarantine(ctx, input, base, QuarantineTerminalDurabilityFailed, ErrOperational)
	case HandbackPhaseOutputWriteFailed:
		if input.Completion == nil {
			return HandbackOutcome{ExitCode: 2, ResultBytes: resultBytes}, operational("record output-write failure", errors.New("missing durable completion"))
		}
		return s.persistNoHandbackQuarantine(ctx, input, base, QuarantineOutputWriteFailed, ErrOperational)
	default:
		return HandbackOutcome{ExitCode: 2, ResultBytes: resultBytes}, operational("apply execution handback", fmt.Errorf("unknown handback phase %q", input.Phase))
	}
}

func (s *HandbackService) applyAuthoritative(ctx context.Context, input HandbackRequest, base HandbackOutcome) (HandbackOutcome, error) {
	completion := *input.Completion
	observed := emptyQuarantineObservations()
	runway, err := s.ports.Repository.Observe(ctx, input.Request.ATCRunway)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if err := validateRepositoryState("runway", runway, input.Request.ATCRunway); err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	observed.Runway = repoObservation(runway)
	if !runway.Clean {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineRunwayDirty, observed, ErrVerdict)
	}
	if runway.Commit != input.Request.InputCommit || runway.Tree != input.Request.InputTree {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineRunwayMoved, observed, ErrVerdict)
	}

	child, err := s.ports.Repository.Observe(ctx, input.Run.Workspace.Path)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if err := validateRepositoryState("child", child, input.Run.Workspace.Path); err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	observed.Child = repoObservation(child)
	if !child.Clean {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineChildDirty, observed, ErrVerdict)
	}
	if child.Commit != completion.Result.OutputCommit || child.Tree != completion.Result.OutputTree {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineChildOutputMismatch, observed, ErrVerdict)
	}

	descendant, err := s.ports.Repository.IsAncestor(ctx, input.Run.Workspace.Path, input.Request.InputCommit, completion.Result.OutputCommit)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if !descendant {
		observed.Descendant = Proof{State: ProofViolatedWithWitness, Witnesses: []string{"output is not a descendant of input"}}
		return s.persistFactualQuarantine(ctx, input, base, QuarantineNonDescendant, observed, ErrVerdict)
	}
	observed.Descendant = Proof{State: ProofProven, Witnesses: []string{}}

	diff, err := s.ports.Repository.Diff(ctx, input.Run.Workspace.Path, input.Request.InputCommit, completion.Result.OutputCommit)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	observed.ProtectedPaths = protectedSpecPaths(diff)
	if len(observed.ProtectedPaths) != 0 {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineProtectedSpecChange, observed, ErrVerdict)
	}

	fastForward, err := s.ports.Repository.FastForwardOnly(ctx, input.Request.ATCRunway, completion.Result.OutputCommit)
	if err != nil || fastForward.Category != gitx.FastForwardSucceeded || !fastForward.Attempted {
		if err == nil {
			err = errors.New("guarded fast-forward returned a contradictory success result")
		}
		return s.resolveFastForwardFailure(ctx, input, base, observed, fastForward, err)
	}
	observed.FastForward = FastForwardSucceeded
	post, err := s.ports.Repository.Observe(ctx, input.Request.ATCRunway)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if err := validateRepositoryState("post-runway", post, input.Request.ATCRunway); err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	observed.PostRunway = repoObservation(post)
	if !post.Clean || post.Commit != completion.Result.OutputCommit || post.Tree != completion.Result.OutputTree {
		return s.persistFactualQuarantine(ctx, input, base, QuarantinePostVerificationMismatch, observed, ErrVerdict)
	}

	record := HandbackRecord{
		Schema: ExecutionHandbackSchemaID, Flight: input.Request.Flight, Lane: input.Request.Lane,
		Epoch: input.Request.Epoch, Session: input.Request.Session, ATCRunway: input.Request.ATCRunway,
		WorkspaceID: input.Run.Workspace.WorkspaceID,
		Receipt:     DurableReceipt{Digest: completion.Receipt.Digest, EventAck: completion.ReceiptEventAck},
		Input:       GitIdentity{Commit: input.Request.InputCommit, Tree: input.Request.InputTree},
		Output:      GitIdentity{Commit: completion.Result.OutputCommit, Tree: completion.Result.OutputTree},
		PreRunway:   RunwayState{Head: runway.Commit, Tree: runway.Tree, Clean: runway.Clean},
		PostRunway:  RunwayState{Head: post.Commit, Tree: post.Tree, Clean: post.Clean},
		Disposition: ControlDispositionFastForwarded,
	}
	canonical, err := canonicalHandbackValue(record)
	if err != nil {
		base.ExitCode = 2
		return base, operational("encode successful handback record", err)
	}
	base.Handback = &canonical
	ack, err := s.ports.Control.PersistHandback(ctx, canonical)
	if err != nil {
		base.ExitCode = 2
		return s.recoverFailedHandback(ctx, input, base, observed, operational("persist successful handback", err))
	}
	if err := ValidateHandbackAck(canonical, ack); err != nil {
		base.ExitCode = 2
		return s.recoverFailedHandback(ctx, input, base, observed, operational("acknowledge successful handback", err))
	}
	base.ControlAck = ack
	if err := s.ports.Releaser.Release(ctx, input.Run.Workspace.WorkspaceID); err != nil {
		base.ExitCode = 2
		return base, operational("release execution workspace after handback", err)
	}
	base.ExitCode = 0
	base.Released = true
	return base, nil
}

func (s *HandbackService) recoverFailedHandback(ctx context.Context, input HandbackRequest, base HandbackOutcome, observed QuarantineObservations, failure error) (HandbackOutcome, error) {
	if !s.ports.Control.Usable() {
		return base, failure
	}
	record := finalizedQuarantineRecord(input, QuarantineHandbackDurabilityFailed)
	record.Observed = observed
	return s.persistQuarantine(ctx, base, record, ErrOperational)
}

func (s *HandbackService) resolveFastForwardFailure(ctx context.Context, input HandbackRequest, base HandbackOutcome, observed QuarantineObservations, result gitx.FastForwardResult, cause error) (HandbackOutcome, error) {
	validFailure := false
	switch result.Category {
	case gitx.FastForwardRunwayDirty, gitx.FastForwardStatusFailed:
		validFailure = !result.Attempted
		observed.FastForward = FastForwardNotAttempted
	case gitx.FastForwardMergeFailed:
		validFailure = result.Attempted
		observed.FastForward = FastForwardFailed
	default:
		if result.Attempted {
			observed.FastForward = FastForwardFailed
		}
	}
	post, err := s.ports.Repository.Observe(ctx, input.Request.ATCRunway)
	if err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if err := validateRepositoryState("post-runway", post, input.Request.ATCRunway); err != nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	if !validFailure || cause == nil {
		return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
	}
	observed.PostRunway = repoObservation(post)
	if !post.Clean {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineRunwayDirty, observed, ErrVerdict)
	}
	postIsInput := post.Commit == input.Request.InputCommit && post.Tree == input.Request.InputTree
	postIsOutput := post.Commit == input.Completion.Result.OutputCommit && post.Tree == input.Completion.Result.OutputTree
	if result.Attempted {
		if !postIsInput && !postIsOutput {
			return s.persistFactualQuarantine(ctx, input, base, QuarantineRunwayMoved, observed, ErrVerdict)
		}
		return s.persistFactualQuarantine(ctx, input, base, QuarantineFastForwardFailed, observed, ErrOperational)
	}
	if !postIsInput {
		return s.persistFactualQuarantine(ctx, input, base, QuarantineRunwayMoved, observed, ErrVerdict)
	}
	// A status refusal or failure followed by a clean exact input observation
	// proves no factual runway mismatch; the repository operation was unable
	// to establish the guarded mutation prerequisite.
	observed.PostRunway = RepoObservation{State: RepositoryUnproven}
	return s.persistRepositoryVerificationFailure(ctx, input, base, observed)
}

func (s *HandbackService) persistRepositoryVerificationFailure(ctx context.Context, input HandbackRequest, base HandbackOutcome, observed QuarantineObservations) (HandbackOutcome, error) {
	return s.persistFactualQuarantine(ctx, input, base, QuarantineRepositoryVerificationFailed, observed, ErrOperational)
}

func (s *HandbackService) persistFactualQuarantine(ctx context.Context, input HandbackRequest, base HandbackOutcome, reason QuarantineReason, observed QuarantineObservations, class error) (HandbackOutcome, error) {
	record := finalizedQuarantineRecord(input, reason)
	record.Observed = observed
	return s.persistQuarantine(ctx, base, record, class)
}

func (s *HandbackService) persistNoHandbackQuarantine(ctx context.Context, input HandbackRequest, base HandbackOutcome, reason QuarantineReason, class error) (HandbackOutcome, error) {
	record := finalizedQuarantineRecord(input, reason)
	return s.persistQuarantine(ctx, base, record, class)
}

func (s *HandbackService) persistIncompleteQuarantine(ctx context.Context, input HandbackRequest, base HandbackOutcome, reason QuarantineReason, class error) (HandbackOutcome, error) {
	record := QuarantineRecord{
		Schema: ExecutionQuarantineSchemaID, Flight: input.Request.Flight, Lane: input.Request.Lane,
		Epoch: input.Request.Epoch, Session: input.Request.Session, ATCRunway: input.Request.ATCRunway,
		WorkspaceID: input.Run.Workspace.WorkspaceID,
		Receipt:     QuarantineReceipt{State: QuarantineReceiptAbsent},
		Repository: QuarantineRepository{
			Input:  GitIdentity{Commit: input.Request.InputCommit, Tree: input.Request.InputTree},
			Output: QuarantineOutput{State: QuarantineOutputAbsent},
		},
		Observed: emptyQuarantineObservations(), Reason: reason, Preserved: input.Preserved,
	}
	return s.persistQuarantine(ctx, base, record, class)
}

func (s *HandbackService) persistQuarantine(ctx context.Context, base HandbackOutcome, record QuarantineRecord, class error) (HandbackOutcome, error) {
	canonical, err := canonicalQuarantineValue(record)
	if err != nil {
		base.ExitCode = 2
		return base, operational("encode execution quarantine", err)
	}
	base.Quarantine = &canonical
	ack, err := s.ports.Control.PersistQuarantine(ctx, canonical, base.ResultBytes)
	if err != nil {
		base.ExitCode = 2
		return base, operational("persist execution quarantine", err)
	}
	if err := ValidateQuarantineAck(canonical, ack); err != nil {
		base.ExitCode = 2
		return base, operational("acknowledge execution quarantine", err)
	}
	base.ControlAck = ack
	base.ExitCode = handbackErrorExit(class)
	return base, classifiedHandbackError(class, string(record.Reason))
}

func (s *HandbackService) applyAbort(ctx context.Context, input HandbackRequest) (HandbackOutcome, error) {
	base := HandbackOutcome{ExitCode: 2}
	if input.Quarantine == nil {
		return base, operational("apply abort", errors.New("missing durable quarantine"))
	}
	quarantine, err := canonicalQuarantineValue(*input.Quarantine)
	if err != nil {
		return base, operational("apply abort quarantine", fmt.Errorf("invalid canonical quarantine: %w", err))
	}
	if quarantine.Digest != input.Quarantine.Digest {
		return base, operational("apply abort quarantine", errors.New("quarantine digest was not already durable"))
	}
	base.Quarantine = &quarantine
	resultBytes, err := resultBytesForPreserved(input.Completion, input.PartialBytes, quarantine.Preserved)
	if err != nil {
		return base, err
	}
	base.ResultBytes = resultBytes
	if quarantine.Flight != input.Request.Flight || quarantine.Lane != input.Request.Lane || quarantine.Epoch != input.Request.Epoch ||
		quarantine.WorkspaceID != input.Run.Workspace.WorkspaceID {
		base.ExitCode = 1
		return base, verdict("abort quarantine identity contradicts execution")
	}
	if quarantine.Preserved.Ref == nil {
		return base, operational("apply abort", errors.New("quarantine has no preserved execution reference"))
	}
	record := AbortRecord{
		Schema: ExecutionAbortSchemaID, Flight: quarantine.Flight, Lane: quarantine.Lane,
		Epoch: quarantine.Epoch, Session: quarantine.Session, WorkspaceID: quarantine.WorkspaceID,
		QuarantineDigest: quarantine.Digest, OwnerDecision: input.OwnerDecision,
		Preserved: *quarantine.Preserved.Ref, Disposition: ControlDispositionAbortPreserve,
	}
	canonical, err := canonicalAbortValue(record)
	if err != nil {
		return base, operational("encode abort-preserve record", err)
	}
	if err := ValidateAbortAgainstQuarantine(canonical, quarantine); err != nil {
		return base, operational("bind abort to quarantine", err)
	}
	base.Abort = &canonical
	ack, err := s.ports.Control.PersistAbort(ctx, canonical)
	if err != nil {
		return base, operational("persist abort-preserve", err)
	}
	if err := ValidateAbortAck(canonical, ack); err != nil {
		return base, operational("acknowledge abort-preserve", err)
	}
	base.ControlAck = ack
	if err := s.ports.Releaser.Release(ctx, input.Run.Workspace.WorkspaceID); err != nil {
		return base, operational("release execution workspace after abort", err)
	}
	base.ExitCode = 1
	base.Released = true
	return base, verdict("execution aborted with inspectable output preserved")
}

func validateHandbackRunIdentity(request ExecutionRequest, run ExecutionRun, allowEmptyAcks bool) error {
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return operational("handback workspace id", err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return operational("handback workspace request", err)
	}
	if run.Workspace.WorkspaceID != workspaceID || run.Workspace.Path == "" ||
		!run.Workspace.Request.Equal(request.ExecutionWorkspaceRequest) || run.Workspace.RequestDigest != workspaceDigest {
		return verdict("handback execution workspace identity contradicts request")
	}
	_, err = validateRunAcknowledgments(request, run.Acks, allowEmptyAcks)
	return err
}

func validatePreservedHandbackInput(input HandbackRequest) ([]byte, ExecutionResult, error) {
	var canonicalResult ExecutionResult
	if input.Phase == HandbackPhaseCompleted || input.Phase == HandbackPhaseOutputWriteFailed {
		if input.Completion == nil {
			return nil, ExecutionResult{}, operational("validate durable completion", errors.New("missing completion"))
		}
		var err error
		canonicalResult, err = validateCompletionForHandback(input.Request, input.Run, *input.Completion)
		if err != nil {
			return nil, ExecutionResult{}, err
		}
	}
	resultBytes, err := resultBytesForPreserved(input.Completion, input.PartialBytes, input.Preserved)
	if err != nil {
		return nil, ExecutionResult{}, err
	}
	if input.Preserved.State == PreservedPartial {
		want, err := EncodeExecutionPartial(input.Request, input.Run)
		if err != nil {
			return nil, ExecutionResult{}, operational("validate partial execution preservation", err)
		}
		if !bytes.Equal(resultBytes, want) {
			return nil, ExecutionResult{}, operational("validate partial execution preservation", errors.New("partial bytes contradict actual request/run state"))
		}
	}
	return resultBytes, canonicalResult, nil
}

func validateCompletionForHandback(request ExecutionRequest, run ExecutionRun, completion Completion) (ExecutionResult, error) {
	if len(completion.ResultBytes) == 0 {
		return ExecutionResult{}, operational("validate durable completion", errors.New("missing canonical result bytes"))
	}
	decoded, err := DecodeExecutionResult(bytes.NewReader(completion.ResultBytes))
	if err != nil {
		return ExecutionResult{}, operational("validate durable completion", err)
	}
	if !reflect.DeepEqual(decoded, completion.Result) || !reflect.DeepEqual(decoded.Receipt, completion.Receipt) ||
		decoded.ReceiptEventAck != completion.ReceiptEventAck || decoded.Verdict != completion.Verdict ||
		decoded.Authority != completion.Authority || decoded.EventChainRoot != completion.EventChainRoot ||
		!reflect.DeepEqual(decoded.Receipt.RevisionSegments, completion.Revisions) ||
		completion.Output.WorkspaceID != decoded.ExecutionWorkspaceID || completion.Output.Path != run.Workspace.Path ||
		!completion.Output.Request.Equal(request.ExecutionWorkspaceRequest) || completion.Output.RequestDigest != run.Workspace.RequestDigest ||
		completion.Output.CurrentCommit != decoded.OutputCommit || completion.Output.CurrentTree != decoded.OutputTree || completion.Output.Clean != decoded.Clean {
		return ExecutionResult{}, operational("validate durable completion", errors.New("completion values contradict canonical result bytes"))
	}
	if decoded.Flight != request.Flight || decoded.Lane != request.Lane || decoded.Epoch != request.Epoch || decoded.Session != request.Session ||
		decoded.ATCRunway != request.ATCRunway || decoded.ExecutionWorkspaceID != run.Workspace.WorkspaceID ||
		decoded.Adapter != request.Adapter || decoded.AdapterVersion != request.AdapterVersion ||
		decoded.InputCommit != request.InputCommit || decoded.InputTree != request.InputTree ||
		decoded.Authority != run.Authority || decoded.EventChainRoot != completion.EventChainRoot {
		return ExecutionResult{}, verdict("durable completion identity contradicts execution request or run")
	}
	return decoded, nil
}

func resultBytesForPreserved(completion *Completion, partial []byte, preserved PreservedExecution) ([]byte, error) {
	result := []byte{}
	if completion != nil {
		result = append(result, completion.ResultBytes...)
	} else {
		result = append(result, partial...)
	}
	switch preserved.State {
	case PreservedNone:
		if preserved.Ref != nil || len(result) != 0 {
			return nil, operational("validate preserved execution", errors.New("preserved none carries a reference or bytes"))
		}
	case PreservedPartial, PreservedFinalized:
		if preserved.Ref == nil || len(result) == 0 {
			return nil, operational("validate preserved execution", errors.New("preserved execution lacks reference or bytes"))
		}
		want, err := PreservedExecutionForBytes(preserved.State, result)
		if err != nil || !preservedExecutionEqual(preserved, want) {
			return nil, operational("validate preserved execution", errors.New("preserved reference does not match exact controller locator and bytes"))
		}
		if completion != nil && preserved.State != PreservedFinalized {
			return nil, operational("validate preserved execution", errors.New("durable completion is not preserved as finalized"))
		}
	default:
		return nil, operational("validate preserved execution", fmt.Errorf("unknown preserved state %q", preserved.State))
	}
	return result, nil
}

func finalizedQuarantineRecord(input HandbackRequest, reason QuarantineReason) QuarantineRecord {
	completion := *input.Completion
	ack := completion.ReceiptEventAck
	return QuarantineRecord{
		Schema: ExecutionQuarantineSchemaID, Flight: input.Request.Flight, Lane: input.Request.Lane,
		Epoch: input.Request.Epoch, Session: input.Request.Session, ATCRunway: input.Request.ATCRunway,
		WorkspaceID: input.Run.Workspace.WorkspaceID,
		Receipt:     QuarantineReceipt{State: QuarantineReceiptDurable, Digest: completion.Receipt.Digest, EventAck: &ack},
		Repository: QuarantineRepository{
			Input:  GitIdentity{Commit: input.Request.InputCommit, Tree: input.Request.InputTree},
			Output: QuarantineOutput{State: QuarantineOutputObserved, Commit: completion.Result.OutputCommit, Tree: completion.Result.OutputTree},
		},
		Observed: emptyQuarantineObservations(), Reason: reason, Preserved: input.Preserved,
	}
}

func emptyQuarantineObservations() QuarantineObservations {
	return QuarantineObservations{
		Runway:         RepoObservation{State: RepositoryUnproven},
		Child:          RepoObservation{State: RepositoryUnproven},
		Descendant:     Proof{State: ProofUnproven, Witnesses: []string{"not observed"}},
		ProtectedPaths: []string{}, FastForward: FastForwardNotAttempted,
		PostRunway: RepoObservation{State: RepositoryUnproven},
	}
}

func repoObservation(state RepositoryState) RepoObservation {
	return RepoObservation{State: RepositoryObserved, Commit: state.Commit, Tree: state.Tree, Clean: state.Clean}
}

func validateRepositoryState(name string, state RepositoryState, wantPath string) error {
	if state.Path != wantPath {
		return fmt.Errorf("%s path %q does not match %q", name, state.Path, wantPath)
	}
	if err := validateGitOID(name+" commit", state.Commit, false); err != nil {
		return err
	}
	return validateGitOID(name+" tree", state.Tree, false)
}

func protectedSpecPaths(entries []gitx.DiffEntry) []string {
	set := make(map[string]struct{})
	for _, entry := range entries {
		if protectedSpecPath(entry.Path) {
			set[entry.Path] = struct{}{}
		}
		if entry.OldPath != "" && protectedSpecPath(entry.OldPath) {
			set[entry.OldPath] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func protectedSpecPath(value string) bool {
	if value == "" || strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value {
		return false
	}
	parts := strings.Split(value, "/")
	return len(parts) >= 4 && parts[0] == ".verdi" && parts[1] == "specs" && parts[len(parts)-1] == "spec.md"
}

func canonicalHandbackValue(record HandbackRecord) (HandbackRecord, error) {
	encoded, err := EncodeHandbackRecord(record)
	if err != nil {
		return HandbackRecord{}, err
	}
	return DecodeHandbackRecord(bytes.NewReader(encoded))
}

func canonicalQuarantineValue(record QuarantineRecord) (QuarantineRecord, error) {
	encoded, err := EncodeQuarantineRecord(record)
	if err != nil {
		return QuarantineRecord{}, err
	}
	return DecodeQuarantineRecord(bytes.NewReader(encoded))
}

func canonicalAbortValue(record AbortRecord) (AbortRecord, error) {
	encoded, err := EncodeAbortRecord(record)
	if err != nil {
		return AbortRecord{}, err
	}
	return DecodeAbortRecord(bytes.NewReader(encoded))
}

func handbackErrorExit(err error) int {
	if errors.Is(err, ErrVerdict) {
		return 1
	}
	return 2
}

func classifiedHandbackError(class error, reason string) error {
	if errors.Is(class, ErrVerdict) {
		return verdict(reason)
	}
	return operational("execution handback", errors.New(reason))
}
