package constitutionapp

import (
	"context"

	"github.com/jyang234/verdi/internal/policyconflict"
)

// SubmitPreparationRequest is SubmitPreparation's strict request: the same
// shape ImpactReview accepts, since preparation composes Validate and
// ImpactReview rather than deriving anything new. The checkout root is
// resolved by the adapter, never accepted as request content
// (InspectRequest's own doc comment explains why).
type SubmitPreparationRequest struct {
	Targets []ImpactTarget `json:"targets"`
}

func (r SubmitPreparationRequest) validate() error {
	return ImpactReviewRequest(r).validate()
}

// SubmitPreparationResult is the submission-readiness packet AC-6's
// "Prepare review" names: the validated proposal, its impact-review packet,
// and one closed ReadyForSubmission verdict. It persists nothing —
// preparation writes only the existing proposal artifacts Propose already
// committed (design §7.1); merge and approval remain the normal Git
// pull-request boundary, entirely outside this operation.
type SubmitPreparationResult struct {
	Schema             string             `json:"schema"`
	Identity           Identity           `json:"identity"`
	Validation         ValidateResult     `json:"validation"`
	ImpactReview       ImpactReviewResult `json:"impact_review"`
	ReadyForSubmission bool               `json:"ready_for_submission"`
	BlockingReasons    []string           `json:"blocking_reasons,omitempty"`
}

// SubmitPreparation composes Validate and ImpactReview into one
// submission-readiness packet without merging, approving, or writing
// anything: ReadyForSubmission is false whenever the proposal itself fails
// to validate, or any declared target's conflict evaluation reaches
// policyconflict.VerdictBlockedViolated (a mechanically PROVEN conflict —
// AC-3: "A mechanically proven conflict cannot be dismissed as no-conflict.
// It must be fixed, superseded, narrowed, or covered by an authorized
// exemption"). VerdictBlockedUnproven still permits submission — AC-3 calls
// that state "requires human disposition," which is exactly the normal
// pull-request review this operation defers to, never a second approval
// gate of its own.
func (s Service) SubmitPreparation(ctx context.Context, root string, req SubmitPreparationRequest) (*SubmitPreparationResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}

	validation, typed := s.Validate(ctx, root, ValidateRequest{})
	if typed != nil {
		return nil, typed
	}
	review, typed := s.ImpactReview(ctx, root, ImpactReviewRequest(req))
	if typed != nil {
		return nil, typed
	}

	ready := validation.Proven
	blocking := []string{}
	if !validation.Proven {
		blocking = append(blocking, "the proposed constitution store did not validate")
	}
	for _, tc := range review.Conflicts {
		if tc.Refusal != "" {
			ready = false
			blocking = append(blocking, "target "+tc.Target.Spec+": "+tc.Refusal)
			continue
		}
		if tc.Report != nil && tc.Report.Verdict == policyconflict.VerdictBlockedViolated {
			ready = false
			blocking = append(blocking, "target "+tc.Target.Spec+": a mechanically proven conflict blocks submission")
		}
	}

	return &SubmitPreparationResult{
		Schema:             SubmitPreparationResultSchema,
		Identity:           review.Identity,
		Validation:         *validation,
		ImpactReview:       *review,
		ReadyForSubmission: ready,
		BlockingReasons:    blocking,
	}, nil
}
