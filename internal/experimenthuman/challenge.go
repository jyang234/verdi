// Package experimenthuman verifies offline, action-bound human proofs for
// experiment operations without reading ambient identity or credentials.
package experimenthuman

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// ChallengeSchema is the only accepted offline-human challenge schema.
const ChallengeSchema = "verdi.experiment-human-challenge/v1"

// Operation is the closed human-only experiment operation vocabulary.
type Operation string

const (
	OperationReconcileDraft      Operation = "reconcile-draft"
	OperationProposeRegistration Operation = "propose-registration"
	OperationProposeRatification Operation = "propose-ratification"
)

// Validate rejects operations outside the closed challenge vocabulary.
func (o Operation) Validate() error {
	switch o {
	case OperationReconcileDraft, OperationProposeRegistration, OperationProposeRatification:
		return nil
	default:
		return fmt.Errorf("experimenthuman: unknown operation %q", o)
	}
}

// ChallengeFacts are the already-derived, current facts bound by a challenge.
type ChallengeFacts struct {
	Operation      Operation
	Spike          string
	ExperimentID   string
	AcceptedHEAD   string
	ProposalHEAD   string
	TrustSource    string
	InputDigest    string
	ProposalDigest string
}

// Challenge is the canonical action- and repository-state-bound signed value.
type Challenge struct {
	Schema         string    `json:"schema"`
	Operation      Operation `json:"operation"`
	Spike          string    `json:"spike"`
	ExperimentID   string    `json:"experiment_id"`
	AcceptedHEAD   string    `json:"accepted_head"`
	ProposalHEAD   string    `json:"proposal_head"`
	TrustSource    string    `json:"trust_source"`
	InputDigest    string    `json:"input_digest"`
	ProposalDigest string    `json:"proposal_digest"`
}

var spikeRe = regexp.MustCompile(`^spec/[a-z0-9]+(-[a-z0-9]+)*$`)

func (f ChallengeFacts) validate() error {
	if err := f.Operation.Validate(); err != nil {
		return err
	}
	if !spikeRe.MatchString(f.Spike) {
		// vocab:identity — "spike" names the challenge's fixed spec/<id> artifact-ref field grammar, not renameable display vocabulary.
		return fmt.Errorf("experimenthuman: spike %q does not match ^spec/<id>$", f.Spike)
	}
	if err := experiment.ValidateID(f.ExperimentID); err != nil {
		return fmt.Errorf("experimenthuman: experiment id: %w", err)
	}
	if err := experiment.ValidateCommit(f.AcceptedHEAD); err != nil {
		return fmt.Errorf("experimenthuman: accepted HEAD: %w", err)
	}
	if err := experiment.ValidateCommit(f.ProposalHEAD); err != nil {
		return fmt.Errorf("experimenthuman: proposal HEAD: %w", err)
	}
	if err := governanceprincipal.ValidateID(f.TrustSource); err != nil {
		return fmt.Errorf("experimenthuman: trust source: %w", err)
	}
	if err := experiment.ValidateDigest(f.InputDigest); err != nil {
		return fmt.Errorf("experimenthuman: input digest: %w", err)
	}
	if err := experiment.ValidateDigest(f.ProposalDigest); err != nil {
		return fmt.Errorf("experimenthuman: proposal digest: %w", err)
	}
	return nil
}

// NewChallenge validates current facts and constructs their challenge value.
func NewChallenge(facts ChallengeFacts) (Challenge, error) {
	if err := facts.validate(); err != nil {
		return Challenge{}, err
	}
	return Challenge{
		Schema: ChallengeSchema, Operation: facts.Operation, Spike: facts.Spike,
		ExperimentID: facts.ExperimentID, AcceptedHEAD: facts.AcceptedHEAD,
		ProposalHEAD: facts.ProposalHEAD, TrustSource: facts.TrustSource,
		InputDigest: facts.InputDigest, ProposalDigest: facts.ProposalDigest,
	}, nil
}

func (c Challenge) validate() error {
	if c.Schema != ChallengeSchema {
		return fmt.Errorf("experimenthuman: challenge schema %q, want %q", c.Schema, ChallengeSchema)
	}
	return (ChallengeFacts{
		Operation: c.Operation, Spike: c.Spike, ExperimentID: c.ExperimentID,
		AcceptedHEAD: c.AcceptedHEAD, ProposalHEAD: c.ProposalHEAD,
		TrustSource: c.TrustSource, InputDigest: c.InputDigest,
		ProposalDigest: c.ProposalDigest,
	}).validate()
}

// Canonical returns the exact bytes a human signs outside Verdi.
func (c Challenge) Canonical() ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(c)
}

// DecodeChallenge strictly decodes and requires byte-canonical challenge JSON.
func DecodeChallenge(data []byte) (Challenge, error) {
	var challenge Challenge
	if err := artifact.DecodeExactJSON(data, &challenge); err != nil {
		return Challenge{}, fmt.Errorf("experimenthuman: decoding challenge: %w", err)
	}
	canonical, err := challenge.Canonical()
	if err != nil {
		return Challenge{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Challenge{}, fmt.Errorf("experimenthuman: challenge is not canonical JSON")
	}
	return challenge, nil
}
