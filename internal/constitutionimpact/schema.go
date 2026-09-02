// Package constitutionimpact owns the strict registered-consumer inventory
// and the completeness witness for constitution impact review. It does not
// discover consumers or evaluate policy conflicts.
package constitutionimpact

import (
	"io/fs"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyconflict"
)

const (
	InventorySchema = "verdi.constitution-consumer-inventory/v1"
	CoverageSchema  = "verdi.constitution-impact-coverage/v1"
	InventoryPath   = ".verdi/constitution/consumers.json"

	constitutionPath = ".verdi/policy/constitution.md"
)

// State is the closed proof state of a constitution impact coverage witness.
type State string

const (
	StateProven              State = "proven"
	StateViolatedWithWitness State = "violated-with-witness"
	StateDisclosedUnproven   State = "disclosed-as-unproven"
)

// Presence is the exact-tree availability state of an inventory.
type Presence string

const (
	PresencePresent     Presence = "present"
	PresenceMissing     Presence = "missing"
	PresenceUnavailable Presence = "unavailable"
)

// ReasonCode is one closed reason explaining a non-proven coverage state.
type ReasonCode string

const (
	ReasonAcceptedInventoryMissing   ReasonCode = "accepted-inventory-missing"
	ReasonProposedInventoryMissing   ReasonCode = "proposed-inventory-missing"
	ReasonAcceptedTreeUnavailable    ReasonCode = "accepted-tree-unavailable"
	ReasonProposedTreeUnavailable    ReasonCode = "proposed-tree-unavailable"
	ReasonAcceptedCatalogUnavailable ReasonCode = "accepted-catalog-unavailable"
	ReasonProposedCatalogUnavailable ReasonCode = "proposed-catalog-unavailable"
	ReasonConsumerUniverseEmpty      ReasonCode = "consumer-universe-empty"
	ReasonInventoryDuplicate         ReasonCode = "inventory-duplicate"
	ReasonEvaluationOmitted          ReasonCode = "evaluation-omitted"
	ReasonEvaluationExtra            ReasonCode = "evaluation-extra"
	ReasonEvaluationDuplicate        ReasonCode = "evaluation-duplicate"
	ReasonEvaluationIdentityMismatch ReasonCode = "evaluation-identity-mismatch"
	ReasonEvaluationResultInvalid    ReasonCode = "evaluation-result-invalid"
	ReasonEvaluationOperandMismatch  ReasonCode = "evaluation-operand-mismatch"
	ReasonEvaluationUnresolved       ReasonCode = "evaluation-unresolved"
)

// Reason carries the sorted witnesses for one coverage failure.
type Reason struct {
	Code      ReasonCode `json:"code"`
	Witnesses []string   `json:"witnesses"`
}

// Consumer is one registered context and its declared evaluation operands.
// Identity binds all three fields together through their canonical forms.
type Consumer struct {
	Request            contextcompile.Request
	Environment        string
	GovernedOperations []string
}

// Inventory is the decoded registered-consumer document.
type Inventory struct {
	Schema    string
	Consumers []Consumer
}

// ExactTree couples immutable Git identities to a filesystem view of that
// same tree. The caller owns construction of the view; coverage records both
// identities so evaluation results can be checked against the operation.
type ExactTree struct {
	Commit string
	Tree   string
	FS     fs.FS
}

// LayerChange is one canonical accepted/proposed constitution-layer delta.
type LayerChange struct {
	Kind           string `json:"kind"`
	ID             string `json:"id"`
	Change         string `json:"change"`
	AcceptedDigest string `json:"accepted_digest"`
	ProposedDigest string `json:"proposed_digest"`
}

// InventoryEvidence binds one exact tree to the inventory and constitution
// catalog used for its consumer validation.
type InventoryEvidence struct {
	Commit             string   `json:"commit"`
	Tree               string   `json:"tree"`
	Presence           Presence `json:"presence"`
	InventoryDigest    string   `json:"inventory_digest"`
	ConstitutionDigest string   `json:"constitution_digest"`
}

// EvaluationRefusal is an explicit unknown evaluation. Its witnesses are
// disclosures, never substitutes for a conflict report.
type EvaluationRefusal struct {
	Code      ReasonCode `json:"code"`
	Witnesses []string   `json:"witnesses"`
}

// Evaluation is a claimed canonical row. ConsumerIdentity is checked against
// Consumer, and exactly one of Result or Refusal must be present.
type Evaluation struct {
	ConsumerIdentity string
	Consumer         Consumer
	Result           *policyconflict.Result
	Refusal          *EvaluationRefusal
}

// SupplementalTarget is caller-provided preview input. It is retained in the
// witness but never participates in union membership or completeness.
type SupplementalTarget struct {
	Consumer Consumer
	Result   *policyconflict.Result
	Refusal  *EvaluationRefusal
}

// EvaluationCoverage is one registered union member's closed row.
type EvaluationCoverage struct {
	ConsumerIdentity string
	Consumer         Consumer
	Report           *policyconflict.Report
	Refusal          *EvaluationRefusal
}

// Coverage is the canonical complete witness for one plan.
type Coverage struct {
	Schema              string
	Accepted            InventoryEvidence
	Proposed            InventoryEvidence
	Layers              []LayerChange
	Consumers           []Consumer
	Evaluations         []EvaluationCoverage
	SupplementalTargets []SupplementalTarget
	State               State
	Reasons             []Reason
}

type plannedConsumer struct {
	identity  string
	consumer  Consumer
	canonical []byte
}

// Plan is the immutable accepted/proposed registered-consumer union.
type Plan struct {
	accepted       InventoryEvidence
	proposed       InventoryEvidence
	layers         []LayerChange
	layerChanged   bool
	consumers      []plannedConsumer
	initialReasons []Reason
}
