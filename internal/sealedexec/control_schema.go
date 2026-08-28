package sealedexec

import "github.com/jyang234/verdi/internal/contextevent"

const (
	ExecutionHandbackSchemaID     = "verdi.execution-handback/v1"
	ExecutionQuarantineSchemaID   = "verdi.execution-quarantine/v1"
	ExecutionAbortSchemaID        = "verdi.execution-abort/v1"
	PreservedExecutionRefSchemaID = "verdi.preserved-execution-ref/v1"
	ExecutionControlAckSchemaID   = "verdi.execution-control-ack/v1"
)

// ControlDisposition is the closed private execution-control outcome.
type ControlDisposition string

const (
	ControlDispositionFastForwarded ControlDisposition = "fast-forwarded"
	ControlDispositionQuarantined   ControlDisposition = "quarantined"
	ControlDispositionAbortPreserve ControlDisposition = "abort-preserve"
)

// GitIdentity binds one commit and its tree.
type GitIdentity struct {
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
}

// RunwayState is a required clean pre/post handback observation.
type RunwayState struct {
	Head  string `json:"head"`
	Tree  string `json:"tree"`
	Clean bool   `json:"clean"`
}

// DurableReceipt binds a receipt digest to its complete specialized ack.
type DurableReceipt struct {
	Digest   string                       `json:"digest"`
	EventAck contextevent.ReceiptEventAck `json:"event_ack"`
}

// HandbackRecord is the self-digested successful runway handback record.
type HandbackRecord struct {
	Schema      string             `json:"schema"`
	Flight      string             `json:"flight"`
	Lane        string             `json:"lane"`
	Epoch       string             `json:"epoch"`
	Session     string             `json:"session"`
	ATCRunway   string             `json:"atc_runway"`
	WorkspaceID string             `json:"workspace_id"`
	Receipt     DurableReceipt     `json:"receipt"`
	Input       GitIdentity        `json:"input"`
	Output      GitIdentity        `json:"output"`
	PreRunway   RunwayState        `json:"pre_runway"`
	PostRunway  RunwayState        `json:"post_runway"`
	Disposition ControlDisposition `json:"disposition"`
	Digest      string             `json:"digest"`
}

// QuarantineReason is the closed I-77 quarantine vocabulary.
type QuarantineReason string

const (
	QuarantineRunwayDirty                  QuarantineReason = "runway-dirty"
	QuarantineRunwayMoved                  QuarantineReason = "runway-moved"
	QuarantineChildDirty                   QuarantineReason = "child-dirty"
	QuarantineNonDescendant                QuarantineReason = "non-descendant"
	QuarantineProtectedSpecChange          QuarantineReason = "protected-spec-change"
	QuarantineFastForwardFailed            QuarantineReason = "fast-forward-failed"
	QuarantinePostVerificationMismatch     QuarantineReason = "post-verification-mismatch"
	QuarantineNonAuthoritative             QuarantineReason = "non-authoritative"
	QuarantineExecutionIncomplete          QuarantineReason = "execution-incomplete"
	QuarantineTerminalDurabilityFailed     QuarantineReason = "terminal-durability-failed"
	QuarantineOutputWriteFailed            QuarantineReason = "output-write-failed"
	QuarantineRepositoryVerificationFailed QuarantineReason = "repository-verification-failed"
	QuarantineChildOutputMismatch          QuarantineReason = "child-output-mismatch"
)

// QuarantineReceiptState closes the receipt union.
type QuarantineReceiptState string

const (
	QuarantineReceiptAbsent  QuarantineReceiptState = "absent"
	QuarantineReceiptDurable QuarantineReceiptState = "durable"
)

// QuarantineReceipt is exactly absent or durable.
type QuarantineReceipt struct {
	State    QuarantineReceiptState        `json:"state"`
	Digest   string                        `json:"digest,omitempty"`
	EventAck *contextevent.ReceiptEventAck `json:"event_ack,omitempty"`
}

// QuarantineOutputState closes the intended output union.
type QuarantineOutputState string

const (
	QuarantineOutputAbsent   QuarantineOutputState = "absent"
	QuarantineOutputObserved QuarantineOutputState = "observed"
)

// QuarantineOutput is exactly absent or an observed commit/tree.
type QuarantineOutput struct {
	State  QuarantineOutputState `json:"state"`
	Commit string                `json:"commit,omitempty"`
	Tree   string                `json:"tree,omitempty"`
}

// QuarantineRepository binds the intended input and possible output.
type QuarantineRepository struct {
	Input  GitIdentity      `json:"input"`
	Output QuarantineOutput `json:"output"`
}

// RepositoryObservationState closes fresh repository observation state.
type RepositoryObservationState string

const (
	RepositoryObserved RepositoryObservationState = "observed"
	RepositoryUnproven RepositoryObservationState = "unproven"
)

// RepoObservation records either fresh Git facts or their explicit absence.
type RepoObservation struct {
	State  RepositoryObservationState `json:"state"`
	Commit string                     `json:"commit"`
	Tree   string                     `json:"tree"`
	Clean  bool                       `json:"clean"`
}

// ProofState is the closed private proof state.
type ProofState string

const (
	ProofProven              ProofState = "proven"
	ProofViolatedWithWitness ProofState = "violated-with-witness"
	ProofUnproven            ProofState = "unproven"
)

// Proof carries sorted witnesses for a three-valued control fact.
type Proof struct {
	State     ProofState `json:"state"`
	Witnesses []string   `json:"witnesses"`
}

// FastForwardState records the only permitted runway mutation attempt.
type FastForwardState string

const (
	FastForwardNotAttempted FastForwardState = "not-attempted"
	FastForwardSucceeded    FastForwardState = "succeeded"
	FastForwardFailed       FastForwardState = "failed"
)

// QuarantineObservations records the complete I-77 observed fact set.
type QuarantineObservations struct {
	Runway         RepoObservation  `json:"runway"`
	Child          RepoObservation  `json:"child"`
	Descendant     Proof            `json:"descendant"`
	ProtectedPaths []string         `json:"protected_paths"`
	FastForward    FastForwardState `json:"fast_forward"`
	PostRunway     RepoObservation  `json:"post_runway"`
}

// PreservedState closes the inspectable execution-result union.
type PreservedState string

const (
	PreservedNone      PreservedState = "none"
	PreservedPartial   PreservedState = "partial"
	PreservedFinalized PreservedState = "finalized"
)

// PreservedExecutionRef is a controller-owned non-secret result reference.
type PreservedExecutionRef struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// PreservedExecution is exactly none, partial, or finalized.
type PreservedExecution struct {
	State PreservedState         `json:"state"`
	Ref   *PreservedExecutionRef `json:"ref,omitempty"`
}

// QuarantineRecord is the self-digested owner-decision record.
type QuarantineRecord struct {
	Schema      string                 `json:"schema"`
	Flight      string                 `json:"flight"`
	Lane        string                 `json:"lane"`
	Epoch       string                 `json:"epoch"`
	Session     string                 `json:"session"`
	ATCRunway   string                 `json:"atc_runway"`
	WorkspaceID string                 `json:"workspace_id"`
	Receipt     QuarantineReceipt      `json:"receipt"`
	Repository  QuarantineRepository   `json:"repository"`
	Observed    QuarantineObservations `json:"observed"`
	Reason      QuarantineReason       `json:"reason"`
	Preserved   PreservedExecution     `json:"preserved"`
	Digest      string                 `json:"digest"`
}

// AbortRecord is the self-digested explicit abort-preserve disposition.
type AbortRecord struct {
	Schema           string                `json:"schema"`
	Flight           string                `json:"flight"`
	Lane             string                `json:"lane"`
	Epoch            string                `json:"epoch"`
	Session          string                `json:"session"`
	WorkspaceID      string                `json:"workspace_id"`
	QuarantineDigest string                `json:"quarantine_digest"`
	OwnerDecision    LogicalRef            `json:"owner_decision"`
	Preserved        PreservedExecutionRef `json:"preserved"`
	Disposition      ControlDisposition    `json:"disposition"`
	Digest           string                `json:"digest"`
}

// ControlAck is the self-digested durable controller acknowledgment.
type ControlAck struct {
	Schema                   string             `json:"schema"`
	RecordSchema             string             `json:"record_schema"`
	RecordDigest             string             `json:"record_digest"`
	Flight                   string             `json:"flight"`
	Lane                     string             `json:"lane"`
	Epoch                    string             `json:"epoch"`
	Session                  string             `json:"session"`
	WorkspaceID              string             `json:"workspace_id"`
	Disposition              ControlDisposition `json:"disposition"`
	ControllerGlobalSequence uint64             `json:"controller_global_sequence"`
	Digest                   string             `json:"digest"`
}
