// Package contextowner is the deliberately public v1 publication of Verdi's
// closed 22-operation sealed-controller owner wire.
//
// Verdi remains the sole owner of the private FD-3 controller payloads. This
// package publishes exactly the semantic contract those payloads already
// carry, so a separate repository can answer an owner call without importing a
// Verdi package, copying a private struct, or treating opaque echoed JSON as a
// successful owner decision.
//
// The publication rule is mechanical (VATC F12 controller-owner bridge
// correction §3.3, sibling invention-ledger row SI-176): take the accepted
// private request or result payload at the fixed Verdi controller-contract
// base, replace only its top-level schema literal with the corresponding
// verdi.context-owner/<operation>-{request,result}/v1 literal, and retain
// every other canonical JSON member name, value domain, nullability, closed-
// union rule, nested canonical representation, and cross-field validation
// unchanged. Nothing here decides, persists, launches, or supervises
// anything: a call and a reply are translation, never authority.
//
// Because the rule is a one-time publication rather than a permanent alias to
// Verdi's private Go types, this package deliberately restates the contract
// instead of importing internal/sealedexec. That also keeps the dependency
// direction one-way: the owning private codec imports this publication, never
// the reverse. Nested canonical Verdi documents that already cross the
// controller boundary are carried as their exact canonical bytes, and their
// interiors stay owned by the codec that writes them.
package contextowner

import (
	"encoding/json"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// The two public envelope schemas. A call is what the owning codec emits from
// one private request payload; a reply is what an owner returns for it.
const (
	CallSchemaID  = "verdi.context-owner-call/v1"
	ReplySchemaID = "verdi.context-owner-reply/v1"
)

// requestSchemaPrefix and the two suffixes derive every operation arm schema.
// Derivation is the single source: listing 44 literals would create a second
// place for the wire to drift from the union that actually encodes it.
const (
	armSchemaPrefix  = "verdi.context-owner/"
	requestSchemaTag = "-request/v1"
	resultSchemaTag  = "-result/v1"
	// installRequestSchemaTag is SI-177's one ratified exception to the
	// publication base: the install-expansion request publishes the requested
	// ref, the request purpose, and the canonical installed item, so it
	// advances to v2 in lockstep with the private arm it mirrors.
	installRequestSchemaTag = "-request/v2"
)

// The schema literals of the nested canonical Verdi documents a published arm
// carries. Each is the exact literal the accepted private payload declares for
// that member, so a nested document that names another schema is refused
// before any owner sees it.
const (
	ExecutionRequestSchemaID     = "verdi.context-execution-request/v1"
	PolicyConflictReportSchemaID = "verdi.policy-conflict-report/v1"
	EventSchemaID                = "verdi.context-event/v1"
	ReceiptSchemaID              = "verdi.context-receipt/v1"
	DataItemSchemaID             = "verdi.context-data-item/v1"
	HandbackRecordSchemaID       = "verdi.execution-handback/v1"
	QuarantineRecordSchemaID     = "verdi.execution-quarantine/v1"
	AbortRecordSchemaID          = "verdi.execution-abort/v1"
	ControlAckSchemaID           = "verdi.execution-control-ack/v1"
)

// The schema literals of the published values that carry their own schema
// member, and the logical-reference schemas their arms are bound to.
const (
	RedactedSegmentSchemaID     = "verdi.context-redacted-segment/v1"
	StoredSegmentSchemaID       = "verdi.context-redacted-segment-stored/v1"
	ProfileRefSchemaID          = "verdi.sealed-project-profile-ref/v1"
	RecorderEndpointRefSchemaID = "verdi.context-recorder-endpoint-ref/v1"
)

// SegmentReferencePrefix binds a stored segment reference to its digest.
const SegmentReferencePrefix = "controller-segment/sha256/"

// The interior literals of nested documents this publication validates
// itself. They stay unexported: they are members of an already-published
// nested document, not arms of the public wire, so they do not widen the
// ratified public surface.
const (
	instructionProjectionSchemaID = "verdi.instruction-projection/v1"
	executionContinuitySchemaID   = "verdi.execution-continuity/v1"
	preservedExecutionRefSchemaID = "verdi.preserved-execution-ref/v1"
	executionPartialSchemaID      = "verdi.context-execution-partial/v1"
	executionResultSchemaID       = "verdi.context-execution-result/v1"
	preservedExecutionIDPrefix    = "controller-preserved/sha256/"
	// runwayNotCarried marks the records whose identity carries no runway.
	runwayNotCarried = "runway-not-carried"
)

// The closed interior unions of the control records. Each set is exactly the
// accepted private domain at the publication base; a value outside it fails
// closed before any owner sees the record.
const (
	dispositionFastForwarded = "fast-forwarded"
	dispositionQuarantined   = "quarantined"
	dispositionAbortPreserve = "abort-preserve"

	quarantineReceiptAbsent  = "absent"
	quarantineReceiptDurable = "durable"

	quarantineOutputAbsent   = "absent"
	quarantineOutputObserved = "observed"

	repositoryObserved = "observed"
	repositoryUnproven = "unproven"

	proofProven              = "proven"
	proofViolatedWithWitness = "violated-with-witness"
	proofUnproven            = "unproven"

	fastForwardNotAttempted = "not-attempted"
	fastForwardSucceeded    = "succeeded"
	fastForwardFailed       = "failed"

	preservedNone      = "none"
	preservedPartial   = "partial"
	preservedFinalized = "finalized"

	actionStart  = "start"
	actionResume = "resume"
)

// The fourteen published quarantine reasons, in accepted registry order.
const (
	quarantineRunwayDirty                  = "runway-dirty"
	quarantineRunwayMoved                  = "runway-moved"
	quarantineChildDirty                   = "child-dirty"
	quarantineNonDescendant                = "non-descendant"
	quarantineProtectedSpecChange          = "protected-spec-change"
	quarantineFastForwardFailed            = "fast-forward-failed"
	quarantinePostVerificationMismatch     = "post-verification-mismatch"
	quarantineNonAuthoritative             = "non-authoritative"
	quarantineExecutionIncomplete          = "execution-incomplete"
	quarantineTerminalDurabilityFailed     = "terminal-durability-failed"
	quarantineOutputWriteFailed            = "output-write-failed"
	quarantineRepositoryVerificationFailed = "repository-verification-failed"
	quarantineChildOutputMismatch          = "child-output-mismatch"
	quarantineHandbackDurabilityFailed     = "handback-durability-failed"
)

// The published mirrors of the nested documents internal/sealedexec owns.
// This publication cannot import that package — Task 2 makes it import this
// one — so the documents it owns are mirrored here exactly as the accepted
// contract declares them. Each mirror is strict: its decode rejects unknown
// members, its required-member list rejects absent or null ones, and its
// canonical re-encoding must equal the exact bytes it decoded, so a mirror
// that drifted from the accepted contract could not accept a document the
// owning codec produced.
type (
	// nestedGitIdentity binds one commit to its tree.
	nestedGitIdentity struct {
		Commit string `json:"commit"`
		Tree   string `json:"tree"`
	}
	// nestedRunwayState is a required clean pre/post handback observation.
	nestedRunwayState struct {
		Head  string `json:"head"`
		Tree  string `json:"tree"`
		Clean bool   `json:"clean"`
	}
	// nestedDurableReceipt binds a receipt digest to its complete ack.
	nestedDurableReceipt struct {
		Digest   string                       `json:"digest"`
		EventAck contextevent.ReceiptEventAck `json:"event_ack"`
	}
	// nestedHandbackRecord is the self-digested successful handback.
	nestedHandbackRecord struct {
		Schema      string               `json:"schema"`
		Flight      string               `json:"flight"`
		Lane        string               `json:"lane"`
		Epoch       string               `json:"epoch"`
		Session     string               `json:"session"`
		ATCRunway   string               `json:"atc_runway"`
		WorkspaceID string               `json:"workspace_id"`
		Receipt     nestedDurableReceipt `json:"receipt"`
		Input       nestedGitIdentity    `json:"input"`
		Output      nestedGitIdentity    `json:"output"`
		PreRunway   nestedRunwayState    `json:"pre_runway"`
		PostRunway  nestedRunwayState    `json:"post_runway"`
		Disposition string               `json:"disposition"`
		Digest      string               `json:"digest"`
	}
	// nestedQuarantineReceipt is exactly absent or durable.
	nestedQuarantineReceipt struct {
		State    string                        `json:"state"`
		Digest   string                        `json:"digest,omitempty"`
		EventAck *contextevent.ReceiptEventAck `json:"event_ack,omitempty"`
	}
	// nestedQuarantineOutput is exactly absent or an observed commit/tree.
	nestedQuarantineOutput struct {
		State  string `json:"state"`
		Commit string `json:"commit,omitempty"`
		Tree   string `json:"tree,omitempty"`
	}
	// nestedQuarantineRepository binds intended input and possible output.
	nestedQuarantineRepository struct {
		Input  nestedGitIdentity      `json:"input"`
		Output nestedQuarantineOutput `json:"output"`
	}
	// nestedRepoObservation is fresh Git facts or their explicit absence.
	nestedRepoObservation struct {
		State  string `json:"state"`
		Commit string `json:"commit"`
		Tree   string `json:"tree"`
		Clean  bool   `json:"clean"`
	}
	// nestedProof carries sorted witnesses for a three-valued control fact.
	nestedProof struct {
		State     string   `json:"state"`
		Witnesses []string `json:"witnesses"`
	}
	// nestedQuarantineObservations is the complete observed fact set.
	nestedQuarantineObservations struct {
		Runway         nestedRepoObservation `json:"runway"`
		Child          nestedRepoObservation `json:"child"`
		Descendant     nestedProof           `json:"descendant"`
		ProtectedPaths []string              `json:"protected_paths"`
		FastForward    string                `json:"fast_forward"`
		PostRunway     nestedRepoObservation `json:"post_runway"`
	}
	// nestedPreservedExecutionRef is the non-secret result reference.
	nestedPreservedExecutionRef struct {
		Schema string `json:"schema"`
		ID     string `json:"id"`
		Digest string `json:"digest"`
	}
	// nestedPreservedExecution is exactly none, partial, or finalized.
	nestedPreservedExecution struct {
		State string                       `json:"state"`
		Ref   *nestedPreservedExecutionRef `json:"ref,omitempty"`
	}
	// nestedQuarantineRecord is the self-digested owner-decision record.
	nestedQuarantineRecord struct {
		Schema      string                       `json:"schema"`
		Flight      string                       `json:"flight"`
		Lane        string                       `json:"lane"`
		Epoch       string                       `json:"epoch"`
		Session     string                       `json:"session"`
		ATCRunway   string                       `json:"atc_runway"`
		WorkspaceID string                       `json:"workspace_id"`
		Receipt     nestedQuarantineReceipt      `json:"receipt"`
		Repository  nestedQuarantineRepository   `json:"repository"`
		Observed    nestedQuarantineObservations `json:"observed"`
		Reason      string                       `json:"reason"`
		Preserved   nestedPreservedExecution     `json:"preserved"`
		Digest      string                       `json:"digest"`
	}
	// nestedAbortRecord is the self-digested abort-preserve disposition.
	nestedAbortRecord struct {
		Schema           string                      `json:"schema"`
		Flight           string                      `json:"flight"`
		Lane             string                      `json:"lane"`
		Epoch            string                      `json:"epoch"`
		Session          string                      `json:"session"`
		WorkspaceID      string                      `json:"workspace_id"`
		QuarantineDigest string                      `json:"quarantine_digest"`
		OwnerDecision    LogicalRef                  `json:"owner_decision"`
		Preserved        nestedPreservedExecutionRef `json:"preserved"`
		Disposition      string                      `json:"disposition"`
		Digest           string                      `json:"digest"`
	}
	// nestedControlAck is the self-digested durable acknowledgment.
	nestedControlAck struct {
		Schema                   string `json:"schema"`
		RecordSchema             string `json:"record_schema"`
		RecordDigest             string `json:"record_digest"`
		Flight                   string `json:"flight"`
		Lane                     string `json:"lane"`
		Epoch                    string `json:"epoch"`
		Session                  string `json:"session"`
		WorkspaceID              string `json:"workspace_id"`
		Disposition              string `json:"disposition"`
		ControllerGlobalSequence uint64 `json:"controller_global_sequence"`
		Digest                   string `json:"digest"`
	}
	// nestedInstructionFile is one exact UTF-8 projection file.
	nestedInstructionFile struct {
		Path          string `json:"path"`
		ContentDigest string `json:"content_digest"`
		Content       string `json:"content"`
	}
	// nestedInstructionProjection is the self-digested authority channel.
	nestedInstructionProjection struct {
		Schema string                  `json:"schema"`
		Files  []nestedInstructionFile `json:"files"`
		Digest string                  `json:"digest"`
	}
	// nestedStartArm fixes the first source sequence of a fresh execution.
	nestedStartArm struct {
		ExpectedSourceSequence uint64 `json:"expected_source_sequence"`
	}
	// nestedResumeArm carries the continuity checkpoint and its digest.
	nestedResumeArm struct {
		Continuity       json.RawMessage `json:"continuity"`
		ContinuityDigest string          `json:"continuity_digest"`
	}
	// nestedExecutionRequest is the sealed start-or-resume request. Its
	// sub-documents stay raw here and are decoded by their own owning
	// codecs, which is where their value rules already live.
	nestedExecutionRequest struct {
		Schema                    string          `json:"schema"`
		Action                    string          `json:"action"`
		Flight                    string          `json:"flight"`
		Lane                      string          `json:"lane"`
		Epoch                     string          `json:"epoch"`
		ManifestRevision          uint64          `json:"manifest_revision"`
		Session                   string          `json:"session"`
		ATCRunway                 string          `json:"atc_runway"`
		InputCommit               string          `json:"input_commit"`
		InputTree                 string          `json:"input_tree"`
		Manifest                  json.RawMessage `json:"manifest"`
		ManifestDigest            string          `json:"manifest_digest"`
		InstructionProjection     json.RawMessage `json:"instruction_projection"`
		ProjectionDigest          string          `json:"projection_digest"`
		ExecutionWorkspaceRequest json.RawMessage `json:"execution_workspace_request"`
		Adapter                   string          `json:"adapter"`
		AdapterVersion            string          `json:"adapter_version"`
		Profile                   LogicalRef      `json:"profile"`
		Grants                    json.RawMessage `json:"grants"`
		AuthorityVerdict          json.RawMessage `json:"authority_verdict"`
		RecorderEndpoint          LogicalRef      `json:"recorder_endpoint"`
		Start                     *nestedStartArm `json:"start,omitempty"`
		Resume                    json.RawMessage `json:"resume,omitempty"`
	}
	// nestedExecutionPartial is the inspectable request/run state a
	// quarantine preserves before a result is finalized.
	nestedExecutionPartial struct {
		Schema            string                  `json:"schema"`
		Flight            string                  `json:"flight"`
		Lane              string                  `json:"lane"`
		Epoch             string                  `json:"epoch"`
		Session           string                  `json:"session"`
		Action            string                  `json:"action"`
		ManifestRevision  uint64                  `json:"manifest_revision"`
		ManifestDigest    string                  `json:"manifest_digest"`
		Adapter           contextevent.Adapter    `json:"adapter"`
		AdapterVersion    string                  `json:"adapter_version"`
		WorkspaceID       string                  `json:"workspace_id"`
		AdapterSessionRef string                  `json:"adapter_session_ref"`
		Authority         contextevent.Authority  `json:"authority"`
		Witnesses         []string                `json:"witnesses"`
		EventAcks         []contextevent.EventAck `json:"event_acks"`
	}
	// nestedExecutionResult is the finalized result a quarantine preserves.
	nestedExecutionResult struct {
		Schema                   string                    `json:"schema"`
		Verdict                  contextcompile.Resolution `json:"verdict"`
		Authority                contextevent.Authority    `json:"authority"`
		Witnesses                []string                  `json:"witnesses"`
		Flight                   string                    `json:"flight"`
		Lane                     string                    `json:"lane"`
		Epoch                    string                    `json:"epoch"`
		Session                  string                    `json:"session"`
		ATCRunway                string                    `json:"atc_runway"`
		ExecutionWorkspaceID     string                    `json:"execution_workspace_id"`
		Adapter                  contextevent.Adapter      `json:"adapter"`
		AdapterVersion           string                    `json:"adapter_version"`
		InputCommit              string                    `json:"input_commit"`
		InputTree                string                    `json:"input_tree"`
		OutputCommit             string                    `json:"output_commit"`
		OutputTree               string                    `json:"output_tree"`
		Clean                    bool                      `json:"clean"`
		TerminalManifestDigest   string                    `json:"terminal_manifest_digest"`
		TerminalManifestRevision uint64                    `json:"terminal_manifest_revision"`
		TerminalSourceSequence   uint64                    `json:"terminal_source_sequence"`
		TerminalGlobalSequence   uint64                    `json:"terminal_global_sequence"`
		EventChainRoot           string                    `json:"event_chain_root"`
		Receipt                  json.RawMessage           `json:"receipt"`
		ReceiptEventAck          json.RawMessage           `json:"receipt_event_ack"`
	}
	// nestedExecutionContinuity is the self-digested resume checkpoint.
	nestedExecutionContinuity struct {
		Schema                          string                  `json:"schema"`
		Flight                          string                  `json:"flight"`
		Lane                            string                  `json:"lane"`
		Epoch                           string                  `json:"epoch"`
		Session                         string                  `json:"session"`
		Adapter                         contextevent.Adapter    `json:"adapter"`
		AdapterVersion                  string                  `json:"adapter_version"`
		ATCRunway                       string                  `json:"atc_runway"`
		InputCommit                     string                  `json:"input_commit"`
		InputTree                       string                  `json:"input_tree"`
		CurrentCommit                   string                  `json:"current_commit"`
		CurrentTree                     string                  `json:"current_tree"`
		ExecutionWorkspaceID            string                  `json:"execution_workspace_id"`
		ExecutionWorkspaceRequestDigest string                  `json:"execution_workspace_request_digest"`
		ProfileDigest                   string                  `json:"profile_digest"`
		GrantDigest                     string                  `json:"grant_digest"`
		AuthorityVerdictDigest          string                  `json:"authority_verdict_digest"`
		CurrentManifestRevision         uint64                  `json:"current_manifest_revision"`
		CurrentManifestDigest           string                  `json:"current_manifest_digest"`
		ProjectionDigest                string                  `json:"projection_digest"`
		RevisionSegments                []contextevent.Revision `json:"revision_segments"`
		EventChainRoot                  string                  `json:"event_chain_root"`
		ExpansionLedgerRoot             string                  `json:"expansion_ledger_root"`
		TerminalSourceSequence          uint64                  `json:"terminal_source_sequence"`
		TerminalGlobalSequence          uint64                  `json:"terminal_global_sequence"`
		RecorderCheckpointDigest        string                  `json:"recorder_checkpoint_digest"`
		AdapterSessionRef               string                  `json:"adapter_session_ref"`
		Digest                          string                  `json:"digest"`
	}
)

// Operation is the closed public owner registry: exactly the 22 Verdi-owned
// FD-3 operations. ATC-owned resolve-claim-mcp is operation 23 in the ATC
// controller and deliberately never enters this wire.
type Operation string

// The 22 published operations, in registry order.
const (
	OperationVerifyAuthority                     Operation = "verify-authority"
	OperationResolveProfile                      Operation = "resolve-profile"
	OperationVerifyConflict                      Operation = "verify-conflict"
	OperationResolveRecorder                     Operation = "resolve-recorder"
	OperationRecorderCheckpoint                  Operation = "recorder-checkpoint"
	OperationRecorderAppend                      Operation = "recorder-append"
	OperationStoreRedactedSegment                Operation = "store-redacted-segment"
	OperationResolveRedactedSegment              Operation = "resolve-redacted-segment"
	OperationVerifyOpaqueBoundary                Operation = "verify-opaque-boundary"
	OperationVerifyProviderSession               Operation = "verify-provider-session"
	OperationVerifyExpansion                     Operation = "verify-expansion"
	OperationStoreAdapterSession                 Operation = "store-adapter-session"
	OperationNextStamp                           Operation = "next-stamp"
	OperationResolveContext                      Operation = "resolve-context"
	OperationVerifyEpoch                         Operation = "verify-epoch"
	OperationInstallExpansion                    Operation = "install-expansion"
	OperationResolveReceiptInputs                Operation = "resolve-receipt-inputs"
	OperationAppendReceipt                       Operation = "append-receipt"
	OperationResolveReceiptVerificationAuthority Operation = "resolve-receipt-verification-authority"
	OperationPersistHandback                     Operation = "persist-handback"
	OperationPersistQuarantine                   Operation = "persist-quarantine"
	OperationPersistAbort                        Operation = "persist-abort"
)

// operations is the one closed registry shared by schema derivation and union
// validation.
var operations = []Operation{
	OperationVerifyAuthority,
	OperationResolveProfile,
	OperationVerifyConflict,
	OperationResolveRecorder,
	OperationRecorderCheckpoint,
	OperationRecorderAppend,
	OperationStoreRedactedSegment,
	OperationResolveRedactedSegment,
	OperationVerifyOpaqueBoundary,
	OperationVerifyProviderSession,
	OperationVerifyExpansion,
	OperationStoreAdapterSession,
	OperationNextStamp,
	OperationResolveContext,
	OperationVerifyEpoch,
	OperationInstallExpansion,
	OperationResolveReceiptInputs,
	OperationAppendReceipt,
	OperationResolveReceiptVerificationAuthority,
	OperationPersistHandback,
	OperationPersistQuarantine,
	OperationPersistAbort,
}

// Operations returns a copy of the closed public registry in wire order.
func Operations() []Operation {
	return append([]Operation(nil), operations...)
}

// RequestSchema derives the published request-arm schema for operation.
//
// Exactly one arm departs from the base tag: install-expansion's request is
// published at v2 (SI-177). Its v1 spelling is migration-only and is refused
// rather than served, and its result arm is unaffected.
func RequestSchema(operation Operation) string {
	if operation == OperationInstallExpansion {
		return armSchemaPrefix + string(operation) + installRequestSchemaTag
	}
	return armSchemaPrefix + string(operation) + requestSchemaTag
}

// ResultSchema derives the published result-arm schema for operation.
func ResultSchema(operation Operation) string {
	return armSchemaPrefix + string(operation) + resultSchemaTag
}

func validOperation(operation Operation) bool {
	for _, candidate := range operations {
		if candidate == operation {
			return true
		}
	}
	return false
}

// FailureCode is the published closed reason category accompanying a
// non-proven fact.
type FailureCode string

// The eight published failure codes. FailureNone is the empty string and is
// legal only on a proven fact.
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

// LogicalRef is a credential-free identity for a service-resolved operand.
type LogicalRef struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// ExecutionKey identifies the one mutually exclusive flight/lane/epoch.
type ExecutionKey struct {
	Flight string `json:"flight"`
	Lane   string `json:"lane"`
	Epoch  string `json:"epoch"`
}

// Verification is the published three-valued fact: witnesses are evidence,
// never caller authority.
type Verification struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
}

// AuthorityFacts are freshly recomputed accepted authority and projection
// identities. The verification members are inlined exactly as the accepted
// private payload inlines them.
type AuthorityFacts struct {
	State              contextcompile.Resolution `json:"state"`
	Failure            FailureCode               `json:"failure"`
	Witnesses          []string                  `json:"witnesses"`
	ManifestRevision   uint64                    `json:"manifest_revision"`
	ManifestDigest     string                    `json:"manifest_digest"`
	ProjectionDigest   string                    `json:"projection_digest"`
	AuthorityDigest    string                    `json:"authority_digest"`
	AcceptedSpecCommit string                    `json:"accepted_spec_commit"`
}

// ProfileQuery carries no credentials and no activated profile handle. Grants
// is the exact canonical execution-grant-set document.
type ProfileQuery struct {
	Ref           LogicalRef      `json:"ref"`
	WorkspacePath string          `json:"workspace_path"`
	Grants        json.RawMessage `json:"grants"`
}

// ProfileMaterial is the credential-free profile material row. Exactly one of
// the Codex and Claude arms is selected.
type ProfileMaterial struct {
	Ref                LogicalRef `json:"ref"`
	Name               string     `json:"name"`
	AbsoluteExecutable string     `json:"absolute_executable"`
	AbsoluteEnvRoot    string     `json:"absolute_env_root"`
	AbsoluteCodexHome  string     `json:"absolute_codex_home,omitempty"`
	Model              string     `json:"model,omitempty"`
	ClaudeConfigDir    string     `json:"absolute_claude_config_dir,omitempty"`
	AdapterVersion     string     `json:"adapter_version"`
	DecoderProfile     string     `json:"decoder_profile"`
}

// RedactedSegment is one canonical JSON detail stored through the controller.
// Digest authenticates Bytes and ByteCount counts them.
type RedactedSegment struct {
	Schema           string `json:"schema"`
	MediaType        string `json:"media_type"`
	RedactionProfile string `json:"redaction_profile"`
	ByteCount        uint64 `json:"byte_count"`
	Digest           string `json:"digest"`
	Bytes            []byte `json:"bytes"`
}

// StoredSegment is the controller-owned content-addressed segment identity.
type StoredSegment struct {
	Schema           string `json:"schema"`
	Reference        string `json:"reference"`
	MediaType        string `json:"media_type"`
	RedactionProfile string `json:"redaction_profile"`
	ByteCount        uint64 `json:"byte_count"`
	Digest           string `json:"digest"`
}

// ConflictFacts carry the exact canonical policy-conflict report.
type ConflictFacts struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Report    json.RawMessage           `json:"report"`
}

// RecorderFacts resolve the durable recorder sink.
type RecorderFacts struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Ref       LogicalRef                `json:"ref"`
}

// RecorderCheckpoint is a fresh durable query result. ActiveRevision is
// required and explicitly null when no revision is in progress.
type RecorderCheckpoint struct {
	State                  contextcompile.Resolution `json:"state"`
	Failure                FailureCode               `json:"failure"`
	Witnesses              []string                  `json:"witnesses"`
	Digest                 string                    `json:"digest"`
	Revisions              []contextevent.Revision   `json:"revisions"`
	EventChainRoot         string                    `json:"event_chain_root"`
	TerminalSourceSequence uint64                    `json:"terminal_source_sequence"`
	TerminalGlobalSequence uint64                    `json:"terminal_global_sequence"`
	ActiveRevision         *ActiveRevision           `json:"active_revision"`
}

// ActiveRevision is the exact durable append position of one in-progress
// manifest revision. Complete checkpoint revisions exclude this tail.
type ActiveRevision struct {
	Revision           uint64                      `json:"revision"`
	ManifestDigest     string                      `json:"manifest_digest"`
	NextSourceSequence uint64                      `json:"next_source_sequence"`
	PriorEventDigest   string                      `json:"prior_event_digest"`
	PriorRevision      *contextevent.PriorRevision `json:"prior_revision"`
	LastGlobalSequence uint64                      `json:"last_global_sequence"`
	Invalidated        bool                        `json:"invalidated"`
	EventAcks          []contextevent.EventAck     `json:"event_acks"`
}

// OpaqueIdentity is the only vendor-boundary information a result exposes.
type OpaqueIdentity struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	AdapterID      string `json:"adapter_id"`
	AdapterVersion string `json:"adapter_version"`
}

// OpaqueBoundaryFacts prove identity-only handling of declared opaque rows.
type OpaqueBoundaryFacts struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Rows      []OpaqueIdentity          `json:"rows"`
}

// ProviderSessionCheck names the exact state a session verifier must prove.
type ProviderSessionCheck struct {
	SessionRef     string `json:"session_ref"`
	AdapterVersion string `json:"adapter_version"`
	ProfileDigest  string `json:"profile_digest"`
	WorkspaceID    string `json:"workspace_id"`
}

// ProviderSessionFacts are a fresh isolated-profile session-state proof.
type ProviderSessionFacts struct {
	State          contextcompile.Resolution `json:"state"`
	Failure        FailureCode               `json:"failure"`
	Witnesses      []string                  `json:"witnesses"`
	SessionRef     string                    `json:"session_ref"`
	AdapterVersion string                    `json:"adapter_version"`
	ProfileDigest  string                    `json:"profile_digest"`
	WorkspaceID    string                    `json:"workspace_id"`
}

// ExpansionFacts are the current atomically installed expansion ledger facts.
// Only a proven fact may carry an installed root.
type ExpansionFacts struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Root      string                    `json:"root"`
}

// SessionRecord is persisted only after adapter-start is durably acknowledged.
type SessionRecord struct {
	Key            ExecutionKey          `json:"key"`
	SessionRef     string                `json:"session_ref"`
	AdapterVersion string                `json:"adapter_version"`
	ProfileDigest  string                `json:"profile_digest"`
	WorkspaceID    string                `json:"workspace_id"`
	LifecycleAck   contextevent.EventAck `json:"lifecycle_ack"`
}

// ContextQuery binds one logical expansion read to a flight epoch.
type ContextQuery struct {
	Key ExecutionKey `json:"key"`
	Ref string       `json:"ref"`
}

// ContextResolution is the resolver's identity-bound data result. Data is the
// exact canonical context data item.
type ContextResolution struct {
	State     contextcompile.Resolution `json:"state"`
	Failure   FailureCode               `json:"failure"`
	Witnesses []string                  `json:"witnesses"`
	Ref       string                    `json:"ref"`
	Data      json.RawMessage           `json:"data"`
}

// FlightStateSnapshot carries the exact state an epoch check must find
// unchanged. Request is the exact canonical sealed execution request.
type FlightStateSnapshot struct {
	Request            json.RawMessage             `json:"request"`
	Key                ExecutionKey                `json:"key"`
	WorkspaceID        string                      `json:"workspace_id"`
	CandidateCommit    string                      `json:"candidate_commit"`
	CandidateTree      string                      `json:"candidate_tree"`
	Revision           uint64                      `json:"revision"`
	ManifestDigest     string                      `json:"manifest_digest"`
	ProjectionDigest   string                      `json:"projection_digest"`
	ExpansionRoot      string                      `json:"expansion_root"`
	NextSourceSequence uint64                      `json:"next_source_sequence"`
	PriorEventDigest   string                      `json:"prior_event_digest"`
	PriorRevision      *contextevent.PriorRevision `json:"prior_revision,omitempty"`
	LastGlobalSequence uint64                      `json:"last_global_sequence"`
	Invalidated        bool                        `json:"invalidated"`
}

// EpochCheck carries the exact state that must remain unchanged.
type EpochCheck struct {
	Snapshot   FlightStateSnapshot `json:"snapshot"`
	Resolution ContextResolution   `json:"resolution"`
}

// ExpansionInstall is atomically persisted immediately after a child ack.
//
// Ref, Purpose, and Data publish SI-177's restart-reconstructible facts
// mechanically: they are the accepted private members with no projection
// choice. Data is the nested canonical data-item document, and an item that
// carries its own optional ref must carry this row's ref.
type ExpansionInstall struct {
	Key                  ExecutionKey          `json:"key"`
	RequestID            string                `json:"request_id"`
	ParentRevision       uint64                `json:"parent_revision"`
	ParentManifestDigest string                `json:"parent_manifest_digest"`
	ChildRevision        uint64                `json:"child_revision"`
	ChildManifestDigest  string                `json:"child_manifest_digest"`
	ExpansionDigest      string                `json:"expansion_digest"`
	ExpansionRoot        string                `json:"expansion_root"`
	TerminalAck          contextevent.EventAck `json:"terminal_ack"`
	Ref                  string                `json:"ref"`
	Purpose              string                `json:"purpose"`
	Data                 json.RawMessage       `json:"data"`
}

// ReceiptInputsQuery binds terminal receipt inputs to exact finalized facts.
type ReceiptInputsQuery struct {
	Request                json.RawMessage `json:"request"`
	WorkspaceID            string          `json:"workspace_id"`
	DispatchDigest         string          `json:"dispatch_digest"`
	TerminalRevision       uint64          `json:"terminal_revision"`
	TerminalSourceSequence uint64          `json:"terminal_source_sequence"`
	TerminalGlobalSequence uint64          `json:"terminal_global_sequence"`
	EventChainRoot         string          `json:"event_chain_root"`
	ResultFactsDigest      string          `json:"result_facts_digest"`
}

// ReceiptInputs are the controller-owned canonical builder receipt operands.
// Every array is non-null, sorted, and deduplicated.
type ReceiptInputs struct {
	Expansions      []contextreceipt.Expansion   `json:"expansions"`
	Obligations     []contextreceipt.Obligation  `json:"obligations"`
	Evidence        []contextreceipt.Evidence    `json:"evidence"`
	ReviewInputs    []contextreceipt.ReviewInput `json:"review_inputs"`
	RunnerPrincipal gp.PrincipalResolution       `json:"runner_principal"`
}

// ReceiptAppend is the atomic receipt bytes/event persistence request. Both
// members are exact canonical documents.
type ReceiptAppend struct {
	Receipt json.RawMessage `json:"receipt"`
	Event   json.RawMessage `json:"event"`
}

// ReceiptVerificationTrustFact publishes the trust fact with the explicit
// member names the accepted private payload declares for it.
type ReceiptVerificationTrustFact struct {
	SourceID       string             `json:"source_id"`
	SourceKind     gp.TrustSourceKind `json:"source_kind"`
	Subjects       []string           `json:"subjects"`
	EvidenceDigest string             `json:"evidence_digest"`
	Available      bool               `json:"available"`
	Valid          bool               `json:"valid"`
	Reason         string             `json:"reason"`
}

// ReceiptVerificationAuthority is the exact read-only receipt verification
// authority result.
type ReceiptVerificationAuthority struct {
	Profile     contextreceipt.ProfileAuthority     `json:"profile"`
	TrustFact   ReceiptVerificationTrustFact        `json:"trust_fact"`
	Isolation   contextreceipt.IsolationAuthority   `json:"isolation"`
	Persistence contextreceipt.PersistenceAuthority `json:"persistence"`
}

// The 22 published request arms. Each carries only the operands an owner must
// validate or act on: no private Go representation, hidden provider state,
// credential, or arbitrary opaque payload.
type (
	// VerifyAuthorityRequest asks an owner to verify accepted authority.
	VerifyAuthorityRequest struct {
		Schema  string          `json:"schema"`
		Request json.RawMessage `json:"request"`
	}
	// ResolveProfileRequest asks for credential-free profile material.
	ResolveProfileRequest struct {
		Schema string       `json:"schema"`
		Query  ProfileQuery `json:"query"`
	}
	// VerifyConflictRequest submits one policy-conflict report.
	VerifyConflictRequest struct {
		Schema string          `json:"schema"`
		Report json.RawMessage `json:"report"`
	}
	// ResolveRecorderRequest names the logical recorder endpoint.
	ResolveRecorderRequest struct {
		Schema string     `json:"schema"`
		Ref    LogicalRef `json:"ref"`
	}
	// RecorderCheckpointRequest reads one flight's durable position.
	RecorderCheckpointRequest struct {
		Schema string       `json:"schema"`
		Key    ExecutionKey `json:"key"`
	}
	// RecorderAppendRequest appends one canonical execution event.
	RecorderAppendRequest struct {
		Schema string          `json:"schema"`
		Event  json.RawMessage `json:"event"`
	}
	// StoreRedactedSegmentRequest stores one redacted detail segment.
	StoreRedactedSegmentRequest struct {
		Schema  string          `json:"schema"`
		Segment RedactedSegment `json:"segment"`
	}
	// ResolveRedactedSegmentRequest reads one stored segment by reference.
	ResolveRedactedSegmentRequest struct {
		Schema    string `json:"schema"`
		Reference string `json:"reference"`
	}
	// VerifyOpaqueBoundaryRequest submits the declared opaque ledger rows.
	VerifyOpaqueBoundaryRequest struct {
		Schema string                       `json:"schema"`
		Rows   []contextcompile.OpaqueEntry `json:"rows"`
	}
	// VerifyProviderSessionRequest names the session state to prove.
	VerifyProviderSessionRequest struct {
		Schema string               `json:"schema"`
		Check  ProviderSessionCheck `json:"check"`
	}
	// VerifyExpansionRequest reads the installed expansion ledger facts.
	VerifyExpansionRequest struct {
		Schema string       `json:"schema"`
		Key    ExecutionKey `json:"key"`
	}
	// StoreAdapterSessionRequest persists one acknowledged adapter session.
	StoreAdapterSessionRequest struct {
		Schema string        `json:"schema"`
		Record SessionRecord `json:"record"`
	}
	// NextStampRequest asks for the next controller-owned stamp.
	NextStampRequest struct {
		Schema string `json:"schema"`
	}
	// ResolveContextRequest reads one logical expansion ref.
	ResolveContextRequest struct {
		Schema string       `json:"schema"`
		Query  ContextQuery `json:"query"`
	}
	// VerifyEpochRequest submits the state that must be unchanged.
	VerifyEpochRequest struct {
		Schema string     `json:"schema"`
		Check  EpochCheck `json:"check"`
	}
	// InstallExpansionRequest atomically installs one child expansion.
	InstallExpansionRequest struct {
		Schema  string           `json:"schema"`
		Install ExpansionInstall `json:"install"`
	}
	// ResolveReceiptInputsRequest binds receipt inputs to finalized facts.
	ResolveReceiptInputsRequest struct {
		Schema string             `json:"schema"`
		Query  ReceiptInputsQuery `json:"query"`
	}
	// AppendReceiptRequest atomically persists a receipt and its event.
	AppendReceiptRequest struct {
		Schema string        `json:"schema"`
		Append ReceiptAppend `json:"append"`
	}
	// ResolveReceiptVerificationAuthorityRequest binds one verifier lookup.
	ResolveReceiptVerificationAuthorityRequest struct {
		Schema string                        `json:"schema"`
		Query  contextreceipt.AuthorityQuery `json:"query"`
	}
	// PersistHandbackRequest persists one handback control record.
	PersistHandbackRequest struct {
		Schema string          `json:"schema"`
		Record json.RawMessage `json:"record"`
	}
	// PersistQuarantineRequest persists one quarantine record and its exact
	// preserved bytes.
	PersistQuarantineRequest struct {
		Schema         string          `json:"schema"`
		Record         json.RawMessage `json:"record"`
		PreservedBytes []byte          `json:"preserved_bytes"`
	}
	// PersistAbortRequest persists one abort control record.
	PersistAbortRequest struct {
		Schema string          `json:"schema"`
		Record json.RawMessage `json:"record"`
	}
)

// The 22 published result arms. Each is a real owner decision or fact; none is
// an opaque success payload.
type (
	// VerifyAuthorityResult carries freshly recomputed authority facts.
	VerifyAuthorityResult struct {
		Schema string         `json:"schema"`
		Facts  AuthorityFacts `json:"facts"`
	}
	// ResolveProfileResult carries credential-free profile material.
	ResolveProfileResult struct {
		Schema   string          `json:"schema"`
		Material ProfileMaterial `json:"material"`
	}
	// VerifyConflictResult carries the adjudicated conflict facts.
	VerifyConflictResult struct {
		Schema string        `json:"schema"`
		Facts  ConflictFacts `json:"facts"`
	}
	// ResolveRecorderResult carries the resolved recorder facts.
	ResolveRecorderResult struct {
		Schema string        `json:"schema"`
		Facts  RecorderFacts `json:"facts"`
	}
	// RecorderCheckpointResult carries the durable checkpoint.
	RecorderCheckpointResult struct {
		Schema     string             `json:"schema"`
		Checkpoint RecorderCheckpoint `json:"checkpoint"`
	}
	// RecorderAppendResult carries the durable event acknowledgment.
	RecorderAppendResult struct {
		Schema string                `json:"schema"`
		Ack    contextevent.EventAck `json:"ack"`
	}
	// StoreRedactedSegmentResult carries the stored segment identity.
	StoreRedactedSegmentResult struct {
		Schema string        `json:"schema"`
		Stored StoredSegment `json:"stored"`
	}
	// ResolveRedactedSegmentResult carries the resolved segment.
	ResolveRedactedSegmentResult struct {
		Schema  string          `json:"schema"`
		Segment RedactedSegment `json:"segment"`
	}
	// VerifyOpaqueBoundaryResult carries identity-only opaque facts.
	VerifyOpaqueBoundaryResult struct {
		Schema string              `json:"schema"`
		Facts  OpaqueBoundaryFacts `json:"facts"`
	}
	// VerifyProviderSessionResult carries the session-state proof.
	VerifyProviderSessionResult struct {
		Schema string               `json:"schema"`
		Facts  ProviderSessionFacts `json:"facts"`
	}
	// VerifyExpansionResult carries the installed expansion facts.
	VerifyExpansionResult struct {
		Schema string         `json:"schema"`
		Facts  ExpansionFacts `json:"facts"`
	}
	// StoreAdapterSessionResult acknowledges the durable session write.
	StoreAdapterSessionResult struct {
		Schema string `json:"schema"`
	}
	// NextStampResult carries one normalized UTC RFC3339Nano stamp.
	NextStampResult struct {
		Schema string `json:"schema"`
		Stamp  string `json:"stamp"`
	}
	// ResolveContextResult carries the identity-bound data resolution.
	ResolveContextResult struct {
		Schema     string            `json:"schema"`
		Resolution ContextResolution `json:"resolution"`
	}
	// VerifyEpochResult carries the epoch verification.
	VerifyEpochResult struct {
		Schema       string       `json:"schema"`
		Verification Verification `json:"verification"`
	}
	// InstallExpansionResult acknowledges the atomic install.
	InstallExpansionResult struct {
		Schema string `json:"schema"`
	}
	// ResolveReceiptInputsResult carries the canonical receipt operands.
	ResolveReceiptInputsResult struct {
		Schema string        `json:"schema"`
		Inputs ReceiptInputs `json:"inputs"`
	}
	// AppendReceiptResult carries the receipt event acknowledgment.
	AppendReceiptResult struct {
		Schema string                       `json:"schema"`
		Ack    contextevent.ReceiptEventAck `json:"ack"`
	}
	// ResolveReceiptVerificationAuthorityResult carries verifier authority.
	ResolveReceiptVerificationAuthorityResult struct {
		Schema    string                       `json:"schema"`
		Authority ReceiptVerificationAuthority `json:"authority"`
	}
	// PersistHandbackResult carries the durable control acknowledgment.
	PersistHandbackResult struct {
		Schema string          `json:"schema"`
		Ack    json.RawMessage `json:"ack"`
	}
	// PersistQuarantineResult carries the durable control acknowledgment.
	PersistQuarantineResult struct {
		Schema string          `json:"schema"`
		Ack    json.RawMessage `json:"ack"`
	}
	// PersistAbortResult carries the durable control acknowledgment.
	PersistAbortResult struct {
		Schema string          `json:"schema"`
		Ack    json.RawMessage `json:"ack"`
	}
)

// Call is the public owner call: the closed typed request union the owning
// codec emits from one private request payload. Operation selects exactly one
// non-zero request arm.
//
// ControllerRequestDigest is SHA-256 over the exact standalone canonical
// private request-payload bytes, including their trailing LF. It is a
// cross-process identity binding, not an authority fact.
type Call struct {
	Schema                  string
	Operation               Operation
	ControllerRequestDigest string

	VerifyAuthority                     VerifyAuthorityRequest
	ResolveProfile                      ResolveProfileRequest
	VerifyConflict                      VerifyConflictRequest
	ResolveRecorder                     ResolveRecorderRequest
	RecorderCheckpoint                  RecorderCheckpointRequest
	RecorderAppend                      RecorderAppendRequest
	StoreRedactedSegment                StoreRedactedSegmentRequest
	ResolveRedactedSegment              ResolveRedactedSegmentRequest
	VerifyOpaqueBoundary                VerifyOpaqueBoundaryRequest
	VerifyProviderSession               VerifyProviderSessionRequest
	VerifyExpansion                     VerifyExpansionRequest
	StoreAdapterSession                 StoreAdapterSessionRequest
	NextStamp                           NextStampRequest
	ResolveContext                      ResolveContextRequest
	VerifyEpoch                         VerifyEpochRequest
	InstallExpansion                    InstallExpansionRequest
	ResolveReceiptInputs                ResolveReceiptInputsRequest
	AppendReceipt                       AppendReceiptRequest
	ResolveReceiptVerificationAuthority ResolveReceiptVerificationAuthorityRequest
	PersistHandback                     PersistHandbackRequest
	PersistQuarantine                   PersistQuarantineRequest
	PersistAbort                        PersistAbortRequest
}

// Reply is the public owner reply: the byte-exact canonical call the owner
// answered, plus the closed typed result union. Call.Operation selects exactly
// one non-zero result arm, so a result can never be accepted for another call
// merely because both operations have the same name.
type Reply struct {
	Schema string
	Call   Call

	VerifyAuthority                     VerifyAuthorityResult
	ResolveProfile                      ResolveProfileResult
	VerifyConflict                      VerifyConflictResult
	ResolveRecorder                     ResolveRecorderResult
	RecorderCheckpoint                  RecorderCheckpointResult
	RecorderAppend                      RecorderAppendResult
	StoreRedactedSegment                StoreRedactedSegmentResult
	ResolveRedactedSegment              ResolveRedactedSegmentResult
	VerifyOpaqueBoundary                VerifyOpaqueBoundaryResult
	VerifyProviderSession               VerifyProviderSessionResult
	VerifyExpansion                     VerifyExpansionResult
	StoreAdapterSession                 StoreAdapterSessionResult
	NextStamp                           NextStampResult
	ResolveContext                      ResolveContextResult
	VerifyEpoch                         VerifyEpochResult
	InstallExpansion                    InstallExpansionResult
	ResolveReceiptInputs                ResolveReceiptInputsResult
	AppendReceipt                       AppendReceiptResult
	ResolveReceiptVerificationAuthority ResolveReceiptVerificationAuthorityResult
	PersistHandback                     PersistHandbackResult
	PersistQuarantine                   PersistQuarantineResult
	PersistAbort                        PersistAbortResult
}
