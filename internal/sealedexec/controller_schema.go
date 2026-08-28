package sealedexec

import (
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
)

const (
	ControllerCallSchemaID   = "verdi.context-controller-call/v1"
	ControllerResultSchemaID = "verdi.context-controller-result/v1"
	ControllerErrorSchemaID  = "verdi.context-controller-error/v1"
)

// ControllerOperation is the exact closed FD-3 operation registry.
type ControllerOperation string

const (
	ControllerOperationVerifyAuthority       ControllerOperation = "verify-authority"
	ControllerOperationResolveProfile        ControllerOperation = "resolve-profile"
	ControllerOperationVerifyConflict        ControllerOperation = "verify-conflict"
	ControllerOperationResolveRecorder       ControllerOperation = "resolve-recorder"
	ControllerOperationRecorderCheckpoint    ControllerOperation = "recorder-checkpoint"
	ControllerOperationRecorderAppend        ControllerOperation = "recorder-append"
	ControllerOperationVerifyOpaqueBoundary  ControllerOperation = "verify-opaque-boundary"
	ControllerOperationVerifyProviderSession ControllerOperation = "verify-provider-session"
	ControllerOperationVerifyExpansion       ControllerOperation = "verify-expansion"
	ControllerOperationStoreAdapterSession   ControllerOperation = "store-adapter-session"
	ControllerOperationNextStamp             ControllerOperation = "next-stamp"
	ControllerOperationResolveContext        ControllerOperation = "resolve-context"
	ControllerOperationVerifyEpoch           ControllerOperation = "verify-epoch"
	ControllerOperationInstallExpansion      ControllerOperation = "install-expansion"
	ControllerOperationResolveReceiptInputs  ControllerOperation = "resolve-receipt-inputs"
	ControllerOperationAppendReceipt         ControllerOperation = "append-receipt"
	ControllerOperationPersistHandback       ControllerOperation = "persist-handback"
	ControllerOperationPersistQuarantine     ControllerOperation = "persist-quarantine"
	ControllerOperationPersistAbort          ControllerOperation = "persist-abort"
)

var controllerOperations = []ControllerOperation{
	ControllerOperationVerifyAuthority,
	ControllerOperationResolveProfile,
	ControllerOperationVerifyConflict,
	ControllerOperationResolveRecorder,
	ControllerOperationRecorderCheckpoint,
	ControllerOperationRecorderAppend,
	ControllerOperationVerifyOpaqueBoundary,
	ControllerOperationVerifyProviderSession,
	ControllerOperationVerifyExpansion,
	ControllerOperationStoreAdapterSession,
	ControllerOperationNextStamp,
	ControllerOperationResolveContext,
	ControllerOperationVerifyEpoch,
	ControllerOperationInstallExpansion,
	ControllerOperationResolveReceiptInputs,
	ControllerOperationAppendReceipt,
	ControllerOperationPersistHandback,
	ControllerOperationPersistQuarantine,
	ControllerOperationPersistAbort,
}

func controllerRequestSchema(operation ControllerOperation) string {
	return "verdi.context-controller/" + string(operation) + "-request/v1"
}

func controllerResultSchema(operation ControllerOperation) string {
	return "verdi.context-controller/" + string(operation) + "-result/v1"
}

// ControllerErrorClass is closed to operational inability.
type ControllerErrorClass string

const ControllerErrorClassOperational ControllerErrorClass = "operational"

// ControllerErrorCode is the closed controller inability vocabulary.
type ControllerErrorCode string

const (
	ControllerErrorUnavailable       ControllerErrorCode = "unavailable"
	ControllerErrorMalformedRequest  ControllerErrorCode = "malformed-request"
	ControllerErrorIdentityMismatch  ControllerErrorCode = "identity-mismatch"
	ControllerErrorSequenceMismatch  ControllerErrorCode = "sequence-mismatch"
	ControllerErrorOperationMismatch ControllerErrorCode = "operation-mismatch"
	ControllerErrorPersistenceFailed ControllerErrorCode = "persistence-failed"
	ControllerErrorConflictingReplay ControllerErrorCode = "conflicting-replay"
	ControllerErrorInternal          ControllerErrorCode = "internal"
)

// ControllerError is the sole operational error arm.
type ControllerError struct {
	Schema    string
	Class     ControllerErrorClass
	Code      ControllerErrorCode
	Witnesses []string
}

// ProfileQuery carries no credentials or activated profile handle.
type ProfileQuery struct {
	Ref           LogicalRef
	WorkspacePath string
	Grants        execworkspace.GrantSet
}

// ProfileMaterial is the credential-free profile material row returned over
// FD 3 and activated locally by Verdi.
type ProfileMaterial struct {
	Ref                LogicalRef
	Name               string
	AbsoluteExecutable string
	AbsoluteEnvRoot    string
	AbsoluteCodexHome  string
	AdapterVersion     string
	DecoderProfile     string
}

// ContextQuery binds one logical expansion read to a flight epoch.
type ContextQuery struct {
	Key ExecutionKey
	Ref string
}

// ReceiptInputsQuery binds terminal receipt inputs to exact finalized facts.
type ReceiptInputsQuery struct {
	Request                ExecutionRequest
	WorkspaceID            string
	DispatchDigest         string
	TerminalRevision       uint64
	TerminalSourceSequence uint64
	TerminalGlobalSequence uint64
	EventChainRoot         string
	ResultFactsDigest      string
}

// ReceiptInputs are the controller-owned canonical builder receipt operands.
type ReceiptInputs struct {
	Expansions      []contextreceipt.Expansion
	Obligations     []contextreceipt.Obligation
	Evidence        []contextreceipt.Evidence
	ReviewInputs    []contextreceipt.ReviewInput
	RunnerPrincipal gp.PrincipalResolution
}

// ReceiptAppend is the atomic receipt bytes/event persistence request.
type ReceiptAppend struct {
	Receipt contextreceipt.Receipt
	Event   contextevent.Event
}

type ControllerVerifyAuthorityRequest struct {
	Schema  string
	Request ExecutionRequest
}
type ControllerVerifyAuthorityResult struct {
	Schema string
	Facts  AuthorityFacts
}
type ControllerResolveProfileRequest struct {
	Schema string
	Query  ProfileQuery
}
type ControllerResolveProfileResult struct {
	Schema   string
	Material ProfileMaterial
}
type ControllerVerifyConflictRequest struct {
	Schema string
	Report policyconflict.Report
}
type ControllerVerifyConflictResult struct {
	Schema string
	Facts  ConflictFacts
}
type ControllerResolveRecorderRequest struct {
	Schema string
	Ref    LogicalRef
}
type ControllerResolveRecorderResult struct {
	Schema string
	Facts  RecorderFacts
}
type ControllerRecorderCheckpointRequest struct {
	Schema string
	Key    ExecutionKey
}
type ControllerRecorderCheckpointResult struct {
	Schema     string
	Checkpoint RecorderCheckpoint
}
type ControllerRecorderAppendRequest struct {
	Schema string
	Event  contextevent.Event
}
type ControllerRecorderAppendResult struct {
	Schema string
	Ack    contextevent.EventAck
}
type ControllerVerifyOpaqueBoundaryRequest struct {
	Schema string
	Rows   []contextcompile.OpaqueEntry
}
type ControllerVerifyOpaqueBoundaryResult struct {
	Schema string
	Facts  OpaqueBoundaryFacts
}
type ControllerVerifyProviderSessionRequest struct {
	Schema string
	Check  ProviderSessionCheck
}
type ControllerVerifyProviderSessionResult struct {
	Schema string
	Facts  ProviderSessionFacts
}
type ControllerVerifyExpansionRequest struct {
	Schema string
	Key    ExecutionKey
}
type ControllerVerifyExpansionResult struct {
	Schema string
	Facts  ExpansionFacts
}
type ControllerStoreAdapterSessionRequest struct {
	Schema string
	Record SessionRecord
}
type ControllerStoreAdapterSessionResult struct{ Schema string }
type ControllerNextStampRequest struct{ Schema string }
type ControllerNextStampResult struct {
	Schema string
	Stamp  string
}
type ControllerResolveContextRequest struct {
	Schema string
	Query  ContextQuery
}
type ControllerResolveContextResult struct {
	Schema     string
	Resolution ContextResolution
}
type ControllerVerifyEpochRequest struct {
	Schema string
	Check  EpochCheck
}
type ControllerVerifyEpochResult struct {
	Schema       string
	Verification Verification
}
type ControllerInstallExpansionRequest struct {
	Schema  string
	Install ExpansionInstall
}
type ControllerInstallExpansionResult struct{ Schema string }
type ControllerResolveReceiptInputsRequest struct {
	Schema string
	Query  ReceiptInputsQuery
}
type ControllerResolveReceiptInputsResult struct {
	Schema string
	Inputs ReceiptInputs
}
type ControllerAppendReceiptRequest struct {
	Schema string
	Append ReceiptAppend
}
type ControllerAppendReceiptResult struct {
	Schema string
	Ack    contextevent.ReceiptEventAck
}
type ControllerPersistHandbackRequest struct {
	Schema string
	Record HandbackRecord
}
type ControllerPersistHandbackResult struct {
	Schema string
	Ack    ControlAck
}
type ControllerPersistQuarantineRequest struct {
	Schema string
	Record QuarantineRecord
}
type ControllerPersistQuarantineResult struct {
	Schema string
	Ack    ControlAck
}
type ControllerPersistAbortRequest struct {
	Schema string
	Record AbortRecord
}
type ControllerPersistAbortResult struct {
	Schema string
	Ack    ControlAck
}

// ControllerCall is a closed typed request union. Operation selects exactly
// one operation-specific value; the wire codec emits only that payload.
type ControllerCall struct {
	Schema       string
	CallSequence uint64
	Operation    ControllerOperation

	VerifyAuthority       ControllerVerifyAuthorityRequest
	ResolveProfile        ControllerResolveProfileRequest
	VerifyConflict        ControllerVerifyConflictRequest
	ResolveRecorder       ControllerResolveRecorderRequest
	RecorderCheckpoint    ControllerRecorderCheckpointRequest
	RecorderAppend        ControllerRecorderAppendRequest
	VerifyOpaqueBoundary  ControllerVerifyOpaqueBoundaryRequest
	VerifyProviderSession ControllerVerifyProviderSessionRequest
	VerifyExpansion       ControllerVerifyExpansionRequest
	StoreAdapterSession   ControllerStoreAdapterSessionRequest
	NextStamp             ControllerNextStampRequest
	ResolveContext        ControllerResolveContextRequest
	VerifyEpoch           ControllerVerifyEpochRequest
	InstallExpansion      ControllerInstallExpansionRequest
	ResolveReceiptInputs  ControllerResolveReceiptInputsRequest
	AppendReceipt         ControllerAppendReceiptRequest
	PersistHandback       ControllerPersistHandbackRequest
	PersistQuarantine     ControllerPersistQuarantineRequest
	PersistAbort          ControllerPersistAbortRequest
}

// ControllerResult is a closed typed result/error union. A valid reply has
// either the operation-selected result value or Error, never both/neither.
type ControllerResult struct {
	Schema       string
	CallSequence uint64
	Operation    ControllerOperation
	Error        *ControllerError

	VerifyAuthority       ControllerVerifyAuthorityResult
	ResolveProfile        ControllerResolveProfileResult
	VerifyConflict        ControllerVerifyConflictResult
	ResolveRecorder       ControllerResolveRecorderResult
	RecorderCheckpoint    ControllerRecorderCheckpointResult
	RecorderAppend        ControllerRecorderAppendResult
	VerifyOpaqueBoundary  ControllerVerifyOpaqueBoundaryResult
	VerifyProviderSession ControllerVerifyProviderSessionResult
	VerifyExpansion       ControllerVerifyExpansionResult
	StoreAdapterSession   ControllerStoreAdapterSessionResult
	NextStamp             ControllerNextStampResult
	ResolveContext        ControllerResolveContextResult
	VerifyEpoch           ControllerVerifyEpochResult
	InstallExpansion      ControllerInstallExpansionResult
	ResolveReceiptInputs  ControllerResolveReceiptInputsResult
	AppendReceipt         ControllerAppendReceiptResult
	PersistHandback       ControllerPersistHandbackResult
	PersistQuarantine     ControllerPersistQuarantineResult
	PersistAbort          ControllerPersistAbortResult
}
