package constitutionapp

import (
	"context"
	"os"
)

// ValidateRequest is Validate's strict request. It carries no field — the
// checkout root is resolved by the adapter, never accepted as request
// content (InspectRequest's own doc comment explains why).
type ValidateRequest struct{}

func (r ValidateRequest) validate() error { return nil }

// ValidateResult is Validate's three-valued proof outcome over the proposed
// constitution store: Proven when the whole store strict-decodes,
// cross-validates, and resolves; not-adopted/incomplete-adoption are
// disclosed via Snapshot.Adopted/Reason on a clean (proven) result, since
// "there is no constitution here yet" is not itself a corruption. Any other
// decode/cross-validation/resolution failure returns a *Error with
// Classification=verdict and Code="corrupted-policy" instead — Validate
// never returns a proven result over a store it could not fully resolve.
type ValidateResult struct {
	Schema   string   `json:"schema"`
	Identity Identity `json:"identity"`
	Proven   bool     `json:"proven"`
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
	snapshot, typed := loadSnapshot(s.Authority, os.DirFS(root), identity.Head, "corrupted-policy")
	if typed != nil {
		return nil, typed
	}
	return &ValidateResult{Schema: ValidateResultSchema, Identity: identity, Proven: true, Snapshot: snapshot}, nil
}
