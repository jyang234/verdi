// Package contextreceipt defines the closed canonical receipt emitted by
// terminal sealed execution and review paths.
package contextreceipt

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// SchemaID is the sole accepted context-receipt schema.
const SchemaID = "verdi.context-receipt/v1"

const (
	// VerifyRequestSchemaID is the sole public receipt-verification request.
	VerifyRequestSchemaID = "verdi.context-receipt-verify-request/v1"
	// VerdictSchemaID is the sole public receipt-verification result.
	VerdictSchemaID = "verdi.context-receipt-verdict/v1"
	// RepositoryProofSchemaID is the exact offline Git-object proof bundle.
	RepositoryProofSchemaID = "verdi.context-repository-proof/v1"
)

// Role reuses the closed receipt-role vocabulary carried by receipt events.
type Role = contextevent.Role

const (
	RoleBuilder  = contextevent.RoleBuilder
	RoleReviewer = contextevent.RoleReviewer
)

// Authority reuses the closed event and receipt authority vocabulary.
type Authority = contextevent.Authority

const (
	AuthorityAuthoritative = contextevent.AuthorityAuthoritative
	AuthorityAdvisory      = contextevent.AuthorityAdvisory
)

// Expansion is one closed manifest expansion identity row.
type Expansion struct {
	RequestID            string `json:"request_id"`
	ParentRevision       uint64 `json:"parent_revision"`
	ParentManifestDigest string `json:"parent_manifest_digest"`
	ChildRevision        uint64 `json:"child_revision"`
	ChildManifestDigest  string `json:"child_manifest_digest"`
	ExpansionDigest      string `json:"expansion_digest"`
}

// Obligation is one closed accepted-obligation identity row.
type Obligation struct {
	Ref           string                `json:"ref"`
	Path          string                `json:"path"`
	AC            string                `json:"ac"`
	Kind          artifact.EvidenceKind `json:"kind"`
	ContentDigest string                `json:"content_digest"`
	Producer      string                `json:"producer"`
}

// Evidence is one closed evidence command/result row. Argv preserves command
// order while the rows themselves form a canonical set.
type Evidence struct {
	CommandID    string              `json:"command_id"`
	Argv         []string            `json:"argv"`
	ExitCode     int                 `json:"exit_code"`
	Verdict      countersign.Verdict `json:"verdict"`
	OutputDigest string              `json:"output_digest"`
}

// ReviewInput is one closed review packet identity row.
type ReviewInput struct {
	Kind          string `json:"kind"`
	ContentDigest string `json:"content_digest"`
}

// Receipt is the complete acyclic context receipt. The subsequently emitted
// receipt event and its acknowledgment are deliberately outside this record.
type Receipt struct {
	Schema                          string                                  `json:"schema"`
	Role                            Role                                    `json:"role"`
	Authority                       Authority                               `json:"authority"`
	ManifestDigest                  string                                  `json:"manifest_digest"`
	DispatchDigest                  string                                  `json:"dispatch_digest"`
	ATCRunway                       string                                  `json:"atc_runway"`
	ExecutionWorkspaceRequestDigest string                                  `json:"execution_workspace_request_digest"`
	ExecutionWorkspaceID            string                                  `json:"execution_workspace_id"`
	InputCommit                     string                                  `json:"input_commit"`
	InputTree                       string                                  `json:"input_tree"`
	OutputCommit                    string                                  `json:"output_commit"`
	OutputTree                      string                                  `json:"output_tree"`
	Clean                           bool                                    `json:"clean"`
	RevisionSegments                []contextevent.Revision                 `json:"revision_segments"`
	EventChainRoot                  string                                  `json:"event_chain_root"`
	TerminalManifestRevision        uint64                                  `json:"terminal_manifest_revision"`
	TerminalSourceSequence          uint64                                  `json:"terminal_source_sequence"`
	TerminalGlobalSequence          uint64                                  `json:"terminal_global_sequence"`
	Expansions                      []Expansion                             `json:"expansions"`
	Obligations                     []Obligation                            `json:"obligations"`
	Evidence                        []Evidence                              `json:"evidence"`
	RunnerPrincipalResolution       governanceprincipal.PrincipalResolution `json:"runner_principal_resolution"`
	Adapter                         contextevent.Adapter                    `json:"adapter"`
	AdapterVersion                  string                                  `json:"adapter_version"`
	ReviewInputs                    []ReviewInput                           `json:"review_inputs"`
	ReviewOf                        []string                                `json:"review_of,omitempty"`
	Digest                          string                                  `json:"digest"`
}

// State is the closed three-valued verification vocabulary.
type State string

const (
	StateProven   State = "proven"
	StateViolated State = "violated-with-witness"
	StateUnproven State = "unproven"
)

// Candidate binds the comparison base and exact candidate under review.
type Candidate struct {
	BaseCommit string `json:"base_commit"`
	BaseTree   string `json:"base_tree"`
	HeadCommit string `json:"head_commit"`
	HeadTree   string `json:"head_tree"`
}

// ProofBundle carries every exact content proof consumed offline.
type ProofBundle struct {
	ExecutionRequestBytes  []byte   `json:"execution_request_bytes"`
	RepositoryProofBytes   []byte   `json:"repository_proof_bytes"`
	ExecutionEventBytes    [][]byte `json:"execution_event_bytes"`
	ExecutionEventAckBytes [][]byte `json:"execution_event_ack_bytes"`
	ReceiptEventBytes      []byte   `json:"receipt_event_bytes"`
	ExpansionDataBytes     [][]byte `json:"expansion_data_bytes"`
	ObligationBytes        [][]byte `json:"obligation_bytes"`
	EvidenceResultBytes    [][]byte `json:"evidence_result_bytes"`
	ReviewPacketBytes      []byte   `json:"review_packet_bytes"`
}

// VerifyRequest is the self-digested public offline-verification request.
type VerifyRequest struct {
	Schema          string                       `json:"schema"`
	Receipt         Receipt                      `json:"receipt"`
	ReceiptEventAck contextevent.ReceiptEventAck `json:"receipt_event_ack"`
	Candidate       Candidate                    `json:"candidate"`
	Proofs          ProofBundle                  `json:"proofs"`
	Digest          string                       `json:"digest"`
}

// RepositoryObject carries one exact Git commit or tree object body.
type RepositoryObject struct {
	OID     string `json:"oid"`
	Type    string `json:"type"`
	Content []byte `json:"content"`
}

// ExecutionObservation is the acknowledged historical clean workspace fact.
type ExecutionObservation struct {
	WorkspaceID string `json:"workspace_id"`
	Commit      string `json:"commit"`
	Tree        string `json:"tree"`
	Clean       bool   `json:"clean"`
	EventDigest string `json:"event_digest"`
}

// RepositoryProof is the self-digested exact SHA-1 commit/tree closure.
type RepositoryProof struct {
	Schema               string               `json:"schema"`
	ObjectFormat         string               `json:"object_format"`
	Candidate            Candidate            `json:"candidate"`
	Objects              []RepositoryObject   `json:"objects"`
	ExecutionObservation ExecutionObservation `json:"execution_observation"`
	Digest               string               `json:"digest"`
}

// OperandKind identifies one of the fixed nineteen singleton evaluations.
type OperandKind string

var operandKinds = []OperandKind{
	"receipt", "candidate", "execution-request", "repository", "manifest",
	"dispatch", "events", "event-chain", "expansions", "obligations",
	"evidence", "receipt-event", "receipt-ack", "governance-profile",
	"runner", "isolation", "review-packet", "review-link", "freshness",
}

// OperandKinds returns the fixed operand order as a defensive copy.
func OperandKinds() []OperandKind { return append([]OperandKind(nil), operandKinds...) }

// Witness is the shared governance-principal witness shape.
type Witness = governanceprincipal.Witness

// Operand is one deterministic singleton evaluation.
type Operand struct {
	Kind           OperandKind `json:"kind"`
	ID             string      `json:"id"`
	State          State       `json:"state"`
	ExpectedDigest string      `json:"expected_digest"`
	ObservedDigest string      `json:"observed_digest"`
	Witnesses      []Witness   `json:"witnesses"`
}

// Finding is the single stable adverse result for an operand.
type Finding struct {
	Code        string      `json:"code"`
	OperandKind OperandKind `json:"operand_kind"`
	OperandID   string      `json:"operand_id"`
	State       State       `json:"state"`
}

// Verdict is the self-digested canonical three-valued verifier result.
type Verdict struct {
	Schema           string    `json:"schema"`
	RequestDigest    string    `json:"request_digest"`
	ReceiptDigest    string    `json:"receipt_digest"`
	ReceiptRole      Role      `json:"receipt_role"`
	ReceiptAuthority Authority `json:"receipt_authority"`
	State            State     `json:"state"`
	Operands         []Operand `json:"operands"`
	Findings         []Finding `json:"findings"`
	Witnesses        []Witness `json:"witnesses"`
	Digest           string    `json:"digest"`
}

// ProfileRef is the credential-free selected governance-profile identity.
type ProfileRef struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// AuthorityQuery binds the controller lookup to one exact verifier request.
type AuthorityQuery struct {
	RequestDigest   string                             `json:"request_digest"`
	ReceiptDigest   string                             `json:"receipt_digest"`
	CandidateCommit string                             `json:"candidate_commit"`
	CandidateTree   string                             `json:"candidate_tree"`
	ProfileRef      ProfileRef                         `json:"profile_ref"`
	RunnerClaim     governanceprincipal.PrincipalClaim `json:"runner_claim"`
}

// ProfileAuthority carries selected canonical profile bytes or an adverse fact.
type ProfileAuthority struct {
	State        State     `json:"state"`
	ProfileBytes []byte    `json:"profile_bytes"`
	Witnesses    []Witness `json:"witnesses"`
}

// IsolationAuthority binds the managed runner to its isolated execution.
type IsolationAuthority struct {
	State         State     `json:"state"`
	ProfileID     string    `json:"profile_id"`
	ProfileDigest string    `json:"profile_digest"`
	Session       string    `json:"session"`
	WorkspaceID   string    `json:"workspace_id"`
	Witnesses     []Witness `json:"witnesses"`
}

// PersistenceAuthority proves controller-owned receipt/event/ack durability.
type PersistenceAuthority struct {
	State              State     `json:"state"`
	ReceiptDigest      string    `json:"receipt_digest"`
	ReceiptEventDigest string    `json:"receipt_event_digest"`
	ReceiptAckDigest   string    `json:"receipt_ack_digest"`
	Witnesses          []Witness `json:"witnesses"`
}

// AuthorityFacts are the exact read-only I-91 controller result.
type AuthorityFacts struct {
	Profile     ProfileAuthority              `json:"profile"`
	TrustFact   governanceprincipal.TrustFact `json:"-"`
	Isolation   IsolationAuthority            `json:"isolation"`
	Persistence PersistenceAuthority          `json:"persistence"`
}

// ExecutionProjection is the cycle-free, contextreceipt-owned view of a
// strictly decoded canonical execution request. The command adapter obtains it
// only through sealedexec's component codec.
type ExecutionProjection struct {
	Flight                          string
	Lane                            string
	Epoch                           string
	Session                         string
	ATCRunway                       string
	ManifestRevision                uint64
	ManifestDigest                  string
	InputCommit                     string
	InputTree                       string
	ExecutionWorkspaceRequestDigest string
	Adapter                         contextevent.Adapter
	AdapterVersion                  string
	ProfileRef                      ProfileRef
}

// ExecutionProofDecoder strictly decodes the component-owned execution wire
// and returns only the receipt verifier's closed projection.
type ExecutionProofDecoder interface {
	DecodeExecutionProof([]byte) (ExecutionProjection, error)
}

// ExpansionProofProjection is the closed component-owned recomputation of one
// canonical expansion DataItem and its I-84 expansion preimage.
type ExpansionProofProjection struct {
	DataItemDigest  string
	DataDigest      string
	ExpansionDigest string
}

// ExpansionProofVerifier keeps the expansion digest algorithm in its owning
// sealedexec component while leaving contextreceipt cycle-free.
type ExpansionProofVerifier interface {
	VerifyExpansionProof([]byte, Expansion) (ExpansionProofProjection, error)
}

// ReviewOperandProjection is one closed I-92 proof result. The review
// component owns packet decoding and freshness semantics; contextreceipt owns
// only their reduction into the fixed verifier operands.
type ReviewOperandProjection struct {
	State          State
	ExpectedDigest string
	ObservedDigest string
}

// ReviewProofProjection carries the three reviewer-only proof results without
// copying the component-owned review-packet wire schema.
type ReviewProofProjection struct {
	Packet    ReviewOperandProjection
	Link      ReviewOperandProjection
	Freshness ReviewOperandProjection
}

// ReviewLaunchProof is the optional acknowledged adapter-start observation
// selected from the already strict-decoded parallel execution event/ack
// streams. Present is false when no review launch facts were recorded.
type ReviewLaunchProof struct {
	Present   bool
	Execution ExecutionProjection
	Event     contextevent.Event
	Ack       contextevent.EventAck
}

// ReviewProofVerifier strictly verifies the component-owned I-92 packet and
// returns only contextreceipt's closed three-operand projection.
type ReviewProofVerifier interface {
	VerifyReviewProof([]byte, Receipt, Candidate, ReviewLaunchProof) (ReviewProofProjection, error)
}
