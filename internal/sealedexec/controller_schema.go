package sealedexec

import (
	"github.com/jyang234/verdi/internal/canonjson"
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

// ControllerContractSchemaID is the read-only projection of this build's
// sealed-controller wire: the three envelope schemas above and the closed
// operation registry below.
//
// It exists so a caller can learn which controller this build speaks without
// starting a sealed execution. Publishing it is not a capability: nothing here
// serves an operation, and the document is a pure function of constants.
const ControllerContractSchemaID = "verdi.context-controller-contract/v1"

// controllerContract is the exact published shape.
//
// Per-operation request and result schemas are deliberately absent. They are
// derived from the operation name by controllerRequestSchema and
// controllerResultSchema, so listing them would create a second place for the
// wire to drift from the derivation that actually encodes it; comparing the
// registry compares them all.
type controllerContract struct {
	Schema                 string                `json:"schema"`
	ControllerCallSchema   string                `json:"controller_call_schema"`
	ControllerResultSchema string                `json:"controller_result_schema"`
	ControllerErrorSchema  string                `json:"controller_error_schema"`
	Operations             []ControllerOperation `json:"operations"`
}

// EncodeControllerContract renders this build's controller contract as one
// canonical document.
//
// It reads no file, opens no store, and takes no operand: the answer is the
// same for one build wherever it runs, which is what lets a caller address the
// document by digest and compare two builds by their bytes.
func EncodeControllerContract() ([]byte, error) {
	return canonjson.Marshal(controllerContract{
		Schema:                 ControllerContractSchemaID,
		ControllerCallSchema:   ControllerCallSchemaID,
		ControllerResultSchema: ControllerResultSchemaID,
		ControllerErrorSchema:  ControllerErrorSchemaID,
		Operations:             ControllerOperations(),
	})
}

// ControllerOperation is the exact closed FD-3 operation registry.
type ControllerOperation string

const (
	ControllerOperationVerifyAuthority                     ControllerOperation = "verify-authority"
	ControllerOperationResolveProfile                      ControllerOperation = "resolve-profile"
	ControllerOperationVerifyConflict                      ControllerOperation = "verify-conflict"
	ControllerOperationResolveRecorder                     ControllerOperation = "resolve-recorder"
	ControllerOperationRecorderCheckpoint                  ControllerOperation = "recorder-checkpoint"
	ControllerOperationRecorderAppend                      ControllerOperation = "recorder-append"
	ControllerOperationStoreRedactedSegment                ControllerOperation = "store-redacted-segment"
	ControllerOperationResolveRedactedSegment              ControllerOperation = "resolve-redacted-segment"
	ControllerOperationVerifyOpaqueBoundary                ControllerOperation = "verify-opaque-boundary"
	ControllerOperationVerifyProviderSession               ControllerOperation = "verify-provider-session"
	ControllerOperationVerifyExpansion                     ControllerOperation = "verify-expansion"
	ControllerOperationStoreAdapterSession                 ControllerOperation = "store-adapter-session"
	ControllerOperationNextStamp                           ControllerOperation = "next-stamp"
	ControllerOperationResolveContext                      ControllerOperation = "resolve-context"
	ControllerOperationVerifyEpoch                         ControllerOperation = "verify-epoch"
	ControllerOperationInstallExpansion                    ControllerOperation = "install-expansion"
	ControllerOperationResolveReceiptInputs                ControllerOperation = "resolve-receipt-inputs"
	ControllerOperationAppendReceipt                       ControllerOperation = "append-receipt"
	ControllerOperationResolveReceiptVerificationAuthority ControllerOperation = "resolve-receipt-verification-authority"
	ControllerOperationPersistHandback                     ControllerOperation = "persist-handback"
	ControllerOperationPersistQuarantine                   ControllerOperation = "persist-quarantine"
	ControllerOperationPersistAbort                        ControllerOperation = "persist-abort"
	ControllerOperationResolveClaimMCP                     ControllerOperation = "resolve-claim-mcp"
)

var controllerOperations = []ControllerOperation{
	ControllerOperationVerifyAuthority,
	ControllerOperationResolveProfile,
	ControllerOperationVerifyConflict,
	ControllerOperationResolveRecorder,
	ControllerOperationRecorderCheckpoint,
	ControllerOperationRecorderAppend,
	ControllerOperationStoreRedactedSegment,
	ControllerOperationResolveRedactedSegment,
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
	ControllerOperationResolveReceiptVerificationAuthority,
	ControllerOperationPersistHandback,
	ControllerOperationPersistQuarantine,
	ControllerOperationPersistAbort,
	ControllerOperationResolveClaimMCP,
}

// ControllerOperations returns the exact closed FD-3 operation registry.
func ControllerOperations() []ControllerOperation {
	return append([]ControllerOperation(nil), controllerOperations...)
}

// controllerRequestSchema derives the request-payload schema for operation.
//
// SI-177 adds exactly one exception to the single derivation: the
// install-expansion request carries the requested ref, the request purpose,
// and the canonical installed item that make an installed expansion
// reconstructible after a restart, so it advances to v2. Its v1 spelling is
// migration-only and is never served, because a v1 document cannot carry
// those operands. Every other request arm, and every result arm including
// install-expansion's, stays at the accepted publication base.
func controllerRequestSchema(operation ControllerOperation) string {
	if operation == ControllerOperationInstallExpansion {
		return "verdi.context-controller/" + string(operation) + "-request/v2"
	}
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
	Model              string
	ClaudeConfigDir    string
	AdapterVersion     string
	DecoderProfile     string
}

// RedactedSegment is one canonical JSON detail stored through the controller.
type RedactedSegment struct {
	Schema           string
	MediaType        string
	RedactionProfile string
	Digest           string
	ByteCount        uint64
	Bytes            []byte
}

// StoredSegment is the controller-owned content-addressed segment identity.
type StoredSegment struct {
	Schema           string
	Reference        string
	MediaType        string
	RedactionProfile string
	Digest           string
	ByteCount        uint64
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
type ControllerStoreRedactedSegmentRequest struct {
	Schema  string
	Segment RedactedSegment
}
type ControllerStoreRedactedSegmentResult struct {
	Schema string
	Stored StoredSegment
}
type ControllerResolveRedactedSegmentRequest struct {
	Schema    string
	Reference string
}
type ControllerResolveRedactedSegmentResult struct {
	Schema  string
	Segment RedactedSegment
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
type ControllerResolveReceiptVerificationAuthorityRequest struct {
	Schema string
	Query  contextreceipt.AuthorityQuery
}
type ControllerResolveReceiptVerificationAuthorityResult struct {
	Schema    string
	Authority contextreceipt.AuthorityFacts
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
	Schema         string
	Record         QuarantineRecord
	PreservedBytes []byte
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

// ClaimMCPQuery binds one claim-registration lookup to this invocation's exact
// canonical request digest. It carries no credential and no provider state.
type ClaimMCPQuery struct {
	RequestDigest string
}

// ClaimMCPRegistration is the controller-owned ATC registration row. It carries
// no bearer, credential, provider state, plan content, claim decision, or any
// identity beyond the request digest it cross-matches.
type ClaimMCPRegistration struct {
	Name          string
	Type          string
	URL           string
	Tools         []string
	RequestDigest string
}

type ControllerResolveClaimMCPRequest struct {
	Schema string
	Query  ClaimMCPQuery
}
type ControllerResolveClaimMCPResult struct {
	Schema       string
	Registration ClaimMCPRegistration
}

// ControllerCall is a closed typed request union. Operation selects exactly
// one operation-specific value; the wire codec emits only that payload.
type ControllerCall struct {
	Schema       string
	CallSequence uint64
	Operation    ControllerOperation

	VerifyAuthority                     ControllerVerifyAuthorityRequest
	ResolveProfile                      ControllerResolveProfileRequest
	VerifyConflict                      ControllerVerifyConflictRequest
	ResolveRecorder                     ControllerResolveRecorderRequest
	RecorderCheckpoint                  ControllerRecorderCheckpointRequest
	RecorderAppend                      ControllerRecorderAppendRequest
	StoreRedactedSegment                ControllerStoreRedactedSegmentRequest
	ResolveRedactedSegment              ControllerResolveRedactedSegmentRequest
	VerifyOpaqueBoundary                ControllerVerifyOpaqueBoundaryRequest
	VerifyProviderSession               ControllerVerifyProviderSessionRequest
	VerifyExpansion                     ControllerVerifyExpansionRequest
	StoreAdapterSession                 ControllerStoreAdapterSessionRequest
	NextStamp                           ControllerNextStampRequest
	ResolveContext                      ControllerResolveContextRequest
	VerifyEpoch                         ControllerVerifyEpochRequest
	InstallExpansion                    ControllerInstallExpansionRequest
	ResolveReceiptInputs                ControllerResolveReceiptInputsRequest
	AppendReceipt                       ControllerAppendReceiptRequest
	ResolveReceiptVerificationAuthority ControllerResolveReceiptVerificationAuthorityRequest
	PersistHandback                     ControllerPersistHandbackRequest
	PersistQuarantine                   ControllerPersistQuarantineRequest
	PersistAbort                        ControllerPersistAbortRequest
	ResolveClaimMCP                     ControllerResolveClaimMCPRequest
}

// ControllerResult is a closed typed result/error union. A valid reply has
// either the operation-selected result value or Error, never both/neither.
type ControllerResult struct {
	Schema       string
	CallSequence uint64
	Operation    ControllerOperation
	Error        *ControllerError

	VerifyAuthority                     ControllerVerifyAuthorityResult
	ResolveProfile                      ControllerResolveProfileResult
	VerifyConflict                      ControllerVerifyConflictResult
	ResolveRecorder                     ControllerResolveRecorderResult
	RecorderCheckpoint                  ControllerRecorderCheckpointResult
	RecorderAppend                      ControllerRecorderAppendResult
	StoreRedactedSegment                ControllerStoreRedactedSegmentResult
	ResolveRedactedSegment              ControllerResolveRedactedSegmentResult
	VerifyOpaqueBoundary                ControllerVerifyOpaqueBoundaryResult
	VerifyProviderSession               ControllerVerifyProviderSessionResult
	VerifyExpansion                     ControllerVerifyExpansionResult
	StoreAdapterSession                 ControllerStoreAdapterSessionResult
	NextStamp                           ControllerNextStampResult
	ResolveContext                      ControllerResolveContextResult
	VerifyEpoch                         ControllerVerifyEpochResult
	InstallExpansion                    ControllerInstallExpansionResult
	ResolveReceiptInputs                ControllerResolveReceiptInputsResult
	AppendReceipt                       ControllerAppendReceiptResult
	ResolveReceiptVerificationAuthority ControllerResolveReceiptVerificationAuthorityResult
	PersistHandback                     ControllerPersistHandbackResult
	PersistQuarantine                   ControllerPersistQuarantineResult
	PersistAbort                        ControllerPersistAbortResult
	ResolveClaimMCP                     ControllerResolveClaimMCPResult
}
