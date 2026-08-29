// Package contextevent defines the closed canonical wire contract for sealed
// execution events and their durable VATC acknowledgments.
package contextevent

const (
	// EventSchemaID is the sole context-event envelope schema.
	EventSchemaID = "verdi.context-event/v1"
	// RevisionSchemaID is the sole revision-segment schema.
	RevisionSchemaID = "verdi.context-event-revision/v1"
	// AckSchemaID is the general durable event acknowledgment schema.
	AckSchemaID = "verdi.context-event-ack/v1"
	// ReceiptAckSchemaID is the specialized receipt-event acknowledgment schema.
	ReceiptAckSchemaID = "verdi.receipt-event-ack/v1"
	// EventPrefixSchemaID is the durable execution-prefix anchor schema.
	EventPrefixSchemaID = "verdi.context-event-prefix/v1"
)

// EventPrefix is a durable anchor summarizing the terminal state of a completed
// or in-progress execution for replay identification. All digest fields carry
// sha256: prefixes. Epoch is a monotonic counter, not a timestamp string.
type EventPrefix struct {
	Schema                  string `json:"schema"`
	Flight                  string `json:"flight"`
	Lane                    string `json:"lane"`
	Session                 string `json:"session"`
	Epoch                   uint64 `json:"epoch"`
	ManifestRevision        uint64 `json:"manifest_revision"`
	ManifestDigest          string `json:"manifest_digest"`
	TerminalSourceSequence  uint64 `json:"terminal_source_sequence"`
	TerminalGlobalSequence  uint64 `json:"terminal_global_sequence"`
	TerminalEventDigest     string `json:"terminal_event_digest"`
	CompletedEventChainRoot string `json:"completed_event_chain_root"`
}

// Adapter is the closed sealed-execution adapter vocabulary.
type Adapter string

const (
	AdapterCodex  Adapter = "codex"
	AdapterClaude Adapter = "claude"
)

// Validate rejects adapters outside the sealed vocabulary.
func (a Adapter) Validate() error { return validateAdapter(a) }

// Authority is the closed event and receipt authority vocabulary.
type Authority string

const (
	AuthorityAuthoritative Authority = "authoritative"
	AuthorityAdvisory      Authority = "advisory"
)

// Role is the closed receipt-role vocabulary shared with receipt events.
type Role string

const (
	RoleBuilder  Role = "builder"
	RoleReviewer Role = "reviewer"
)

// Kind is the closed context-event kind vocabulary.
type Kind string

const (
	KindFlightPlan            Kind = "flight-plan"
	KindInstructionProjection Kind = "instruction-projection"
	KindChildManifest         Kind = "child-manifest"
	KindPrompt                Kind = "prompt"
	KindProviderMessage       Kind = "provider-message"
	KindProviderSummary       Kind = "provider-summary"
	KindToolCall              Kind = "tool-call"
	KindToolResult            Kind = "tool-result"
	KindRead                  Kind = "read"
	KindWrite                 Kind = "write"
	KindEditDenied            Kind = "edit-denied"
	KindContextRequest        Kind = "context-request"
	KindContextDecision       Kind = "context-decision"
	KindClaimRequest          Kind = "claim-request"
	KindClaimDecision         Kind = "claim-decision"
	KindClaimWait             Kind = "claim-wait"
	KindClaimRelease          Kind = "claim-release"
	KindCommand               Kind = "command"
	KindTest                  Kind = "test"
	KindResource              Kind = "resource"
	KindTimeout               Kind = "timeout"
	KindGitStatus             Kind = "git-status"
	KindGitDiff               Kind = "git-diff"
	KindGitCommit             Kind = "git-commit"
	KindForgeChange           Kind = "forge-change"
	KindGateInput             Kind = "gate-input"
	KindGateVerdict           Kind = "gate-verdict"
	KindWitness               Kind = "witness"
	KindFlightPlanDeviation   Kind = "flight-plan-deviation"
	KindAdjudication          Kind = "adjudication"
	KindExecutionResult       Kind = "execution-result"
	KindReceipt               Kind = "receipt"
	KindRetry                 Kind = "retry"
	KindResume                Kind = "resume"
	KindSuspension            Kind = "suspension"
	KindTelemetryGap          Kind = "telemetry-gap"
	KindAdapterStart          Kind = "adapter-start"
	KindAdapterStop           Kind = "adapter-stop"
	KindAdapterError          Kind = "adapter-error"
)

// PriorRevision is the optional whole bridge from sequence one of a new
// manifest revision to the acknowledged terminal event of its predecessor.
type PriorRevision struct {
	ManifestRevision       uint64 `json:"manifest_revision"`
	ManifestDigest         string `json:"manifest_digest"`
	EventRoot              string `json:"event_root"`
	TerminalSourceSequence uint64 `json:"terminal_source_sequence"`
	TerminalGlobalSequence uint64 `json:"terminal_global_sequence"`
}

// Event is the complete sealed execution event envelope. VATC global order is
// deliberately absent and is supplied only by a later acknowledgment. Payload
// must be a pointer to the registered payload struct for Kind; values and
// arbitrary implementations fail closed.
type Event struct {
	Schema               string         `json:"schema"`
	SourceSequence       uint64         `json:"source_sequence"`
	Flight               string         `json:"flight"`
	Lane                 string         `json:"lane"`
	Epoch                string         `json:"epoch"`
	ManifestRevision     uint64         `json:"manifest_revision"`
	ManifestDigest       string         `json:"manifest_digest"`
	Session              string         `json:"session"`
	ATCRunway            string         `json:"atc_runway"`
	ExecutionWorkspaceID string         `json:"execution_workspace_id"`
	CandidateCommit      string         `json:"candidate_commit"`
	CandidateTree        string         `json:"candidate_tree"`
	Adapter              Adapter        `json:"adapter"`
	AdapterVersion       string         `json:"adapter_version"`
	OccurredAt           string         `json:"occurred_at"`
	Kind                 Kind           `json:"kind"`
	PayloadSchema        string         `json:"payload_schema"`
	Payload              any            `json:"payload"`
	PriorEventDigest     string         `json:"prior_event_digest"`
	PriorRevision        *PriorRevision `json:"prior_revision,omitempty"`
	EventDigest          string         `json:"event_digest"`
}

// Revision is one acknowledged manifest-revision terminal segment.
type Revision struct {
	Schema                 string `json:"schema"`
	ManifestRevision       uint64 `json:"manifest_revision"`
	ManifestDigest         string `json:"manifest_digest"`
	FirstGlobalSequence    uint64 `json:"first_global_sequence"`
	TerminalGlobalSequence uint64 `json:"terminal_global_sequence"`
	TerminalSourceSequence uint64 `json:"terminal_source_sequence"`
	TerminalKind           Kind   `json:"terminal_kind"`
	EventRoot              string `json:"event_root"`
}

// EventAck binds a source event to VATC's separately allocated global order.
type EventAck struct {
	Schema           string `json:"schema"`
	Flight           string `json:"flight"`
	Lane             string `json:"lane"`
	Epoch            string `json:"epoch"`
	Session          string `json:"session"`
	ManifestRevision uint64 `json:"manifest_revision"`
	Kind             Kind   `json:"kind"`
	SourceSequence   uint64 `json:"source_sequence"`
	EventDigest      string `json:"event_digest"`
	GlobalSequence   uint64 `json:"global_sequence"`
}

// ReceiptEventAck specializes EventAck with the finalized receipt digest.
type ReceiptEventAck struct {
	Schema           string `json:"schema"`
	Flight           string `json:"flight"`
	Lane             string `json:"lane"`
	Epoch            string `json:"epoch"`
	Session          string `json:"session"`
	ManifestRevision uint64 `json:"manifest_revision"`
	Kind             Kind   `json:"kind"`
	SourceSequence   uint64 `json:"source_sequence"`
	EventDigest      string `json:"event_digest"`
	GlobalSequence   uint64 `json:"global_sequence"`
	ReceiptDigest    string `json:"receipt_digest"`
}
