package constitutionapp

import (
	"context"
	"os"
)

// ValidateRequest is Validate's strict request. Beyond its own envelope
// version it carries no field — the checkout root is resolved by the
// adapter, never accepted as request content (InspectRequest's own doc
// comment explains why).
type ValidateRequest struct {
	Schema string `json:"schema"`
}

func (r ValidateRequest) validate() error { return nil }

// ValidateResult is Validate's outcome over the proposed constitution
// store. It carries no separate "proven" flag: a returned ValidateResult
// exists only for a store Validate fully strict-decoded, cross-validated,
// and resolved, so such a flag could only ever be the constant true — a
// field that reads affirmatively while carrying no information, and whose
// consumers' "not proven" branches are unreachable by construction.
//
// The three-valued outcome is instead carried where it is actually decided,
// once: any decode/cross-validation/resolution failure returns a *Error
// (Classification=verdict, Code="corrupted-policy") and no result at all,
// while the one non-corrupt state that is still not a resolved constitution
// — not-adopted or incomplete adoption — is disclosed on Snapshot.Adopted
// with its own policyauthority Reason, since "there is no constitution here
// yet" is not a corruption (CO-1). SubmitPreparation reads exactly that
// field, so its blocking branch is live rather than dead.
type ValidateResult struct {
	Schema   string   `json:"schema"`
	Identity Identity `json:"identity"`
	Snapshot Snapshot `json:"snapshot"`
}

// Validate strict-decodes and cross-validates the proposed constitution
// store, exactly as internal/policyauthority.Load and .Resolve already do —
// this operation adds no second decode or validation pass of its own. A
// caller intending to submit a proposal calls this (or SubmitPreparation,
// which composes it) before ImpactReview, so a corrupted proposal is
// reported before any conflict evaluation runs against it.
func (s Service) Validate(ctx context.Context, root string, req ValidateRequest) (*ValidateResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}
	return s.validateAt(root, identity)
}

// validateAt is Validate's body over an ALREADY-RESOLVED identity, so a
// composing operation (SubmitPreparation) can pin ONE immutable identity
// snapshot and thread it through every sub-operation instead of letting each
// resolve its own. Two independent resolutions are two different repository
// observations, and a record carrying one beside a neighbour that used the
// other describes a repository state that never existed.
func (s Service) validateAt(root string, identity Identity) (*ValidateResult, *Error) {
	snapshot, typed := loadSnapshot(s.Authority, os.DirFS(root), identity.Head, "corrupted-policy")
	if typed != nil {
		return nil, typed
	}
	return &ValidateResult{Schema: ValidateResultSchema, Identity: identity, Snapshot: snapshot}, nil
}
