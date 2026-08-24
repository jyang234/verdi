package contextcompile

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// PayloadSelectionInput is the exact operation target evaluated against each
// effective policy layer through EvaluateApplicability. Request is the
// caller-declared scope; the remaining fields are the concrete target facts.
type PayloadSelectionInput struct {
	Request       policyartifact.Scope
	CandidatePath string
	CandidateRef  string
	Phase         Phase
	Environment   string
}

// ApplicablePayloadLayer is one applicable policy owner and its typed payload.
// Layers returns fresh instances, so callers cannot mutate selection custody.
type ApplicablePayloadLayer struct {
	PolicyID     string                 `json:"policy_id"`
	PolicyDigest string                 `json:"policy_digest"`
	Payload      policyartifact.Payload `json:"payload"`
}

// ApplicablePayloadSelection is the sealed, canonically sorted set of every
// applicable layer for one registered layered payload kind. Its operands are
// private; Layers returns strict deep copies after rechecking the seal.
type ApplicablePayloadSelection struct {
	kind            string
	effectiveDigest string
	layers          []ApplicablePayloadLayer
	seal            string
}

type applicablePayloadSelectionContent struct {
	Kind            string                   `json:"kind"`
	EffectiveDigest string                   `json:"effective_digest"`
	Layers          []ApplicablePayloadLayer `json:"layers"`
}

// SelectApplicablePayloads selects every applicable layer of kind from one
// already-sealed effective policy. Unknown applicability is a blocking error;
// proven inapplicability is omitted; no feature precedence is introduced.
func SelectApplicablePayloads(effective *policyauthority.EffectivePolicy, kind string, input PayloadSelectionInput) (*ApplicablePayloadSelection, error) {
	if effective == nil {
		return nil, fmt.Errorf("contextcompile: select applicable payloads: effective policy is nil")
	}
	cardinality, ok := policyartifact.RegisteredPayloadCardinality(kind)
	if !ok {
		return nil, fmt.Errorf("contextcompile: select applicable payloads: payload kind %q is not registered", kind)
	}
	if cardinality != policyartifact.PayloadLayered {
		return nil, fmt.Errorf("contextcompile: select applicable payloads: payload kind %q has cardinality %q, want %q", kind, cardinality, policyartifact.PayloadLayered)
	}
	effectiveDigest, err := effective.Digest()
	if err != nil {
		return nil, fmt.Errorf("contextcompile: select applicable payloads: effective policy: %w", err)
	}

	layers := make([]ApplicablePayloadLayer, 0)
	for _, entry := range effective.Policies {
		payload, present := entry.Payloads[kind]
		if !present {
			continue
		}
		applicability, err := EvaluateApplicability(ApplicabilityInput{
			Policy:        entry.Scope,
			Request:       input.Request,
			CandidatePath: input.CandidatePath,
			CandidateRef:  input.CandidateRef,
			Phase:         input.Phase,
			Environment:   input.Environment,
		})
		if err != nil {
			return nil, fmt.Errorf("contextcompile: select applicable payloads: policy %s: %w", entry.PolicyID, err)
		}
		switch applicability.State {
		case ApplicabilityInapplicable:
			continue
		case ApplicabilityUnknown:
			return nil, fmt.Errorf("contextcompile: select applicable payloads: policy %s payload %q applicability is unknown", entry.PolicyID, kind)
		case ApplicabilityApplicable:
		default:
			return nil, fmt.Errorf("contextcompile: select applicable payloads: policy %s returned unknown applicability value %q", entry.PolicyID, applicability.State)
		}
		cloned, err := policyartifact.ClonePayload(kind, payload)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: select applicable payloads: policy %s: %w", entry.PolicyID, err)
		}
		layers = append(layers, ApplicablePayloadLayer{PolicyID: entry.PolicyID, PolicyDigest: entry.PolicyDigest, Payload: cloned})
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i].PolicyID < layers[j].PolicyID })
	for i := 1; i < len(layers); i++ {
		if layers[i-1].PolicyID == layers[i].PolicyID {
			return nil, fmt.Errorf("contextcompile: select applicable payloads: duplicate policy id %q", layers[i].PolicyID)
		}
	}

	selection := &ApplicablePayloadSelection{kind: kind, effectiveDigest: effectiveDigest, layers: layers}
	seal, err := selection.contentDigest()
	if err != nil {
		return nil, fmt.Errorf("contextcompile: select applicable payloads: seal: %w", err)
	}
	selection.seal = seal
	return selection, nil
}

// Kind returns the selection's registered payload kind after seal proof.
func (s *ApplicablePayloadSelection) Kind() (string, error) {
	if err := s.checkSeal(); err != nil {
		return "", err
	}
	return s.kind, nil
}

// EffectiveDigest returns the exact effective-policy identity that was the
// source of the selection.
func (s *ApplicablePayloadSelection) EffectiveDigest() (string, error) {
	if err := s.checkSeal(); err != nil {
		return "", err
	}
	return s.effectiveDigest, nil
}

// Layers returns a mutation-safe copy of every selected layer.
func (s *ApplicablePayloadSelection) Layers() ([]ApplicablePayloadLayer, error) {
	if err := s.checkSeal(); err != nil {
		return nil, err
	}
	out := make([]ApplicablePayloadLayer, len(s.layers))
	for i, layer := range s.layers {
		cloned, err := policyartifact.ClonePayload(s.kind, layer.Payload)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: selected payload layer %s: %w", layer.PolicyID, err)
		}
		out[i] = ApplicablePayloadLayer{PolicyID: layer.PolicyID, PolicyDigest: layer.PolicyDigest, Payload: cloned}
	}
	return out, nil
}

// Digest returns the selection's canonical sealed-content digest.
func (s *ApplicablePayloadSelection) Digest() (string, error) {
	if err := s.checkSeal(); err != nil {
		return "", err
	}
	return s.seal, nil
}

func (s *ApplicablePayloadSelection) checkSeal() error {
	if s == nil || s.seal == "" {
		return fmt.Errorf("contextcompile: applicable payload selection was not produced by SelectApplicablePayloads")
	}
	digest, err := s.contentDigest()
	if err != nil {
		return err
	}
	if digest != s.seal {
		return fmt.Errorf("contextcompile: applicable payload selection was modified after selection")
	}
	return nil
}

func (s *ApplicablePayloadSelection) contentDigest() (string, error) {
	return canonjson.Digest(applicablePayloadSelectionContent{
		Kind: s.kind, EffectiveDigest: s.effectiveDigest, Layers: s.layers,
	})
}
