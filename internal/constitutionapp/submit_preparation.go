package constitutionapp

import (
	"context"
	"fmt"
	"sort"
	"strings"

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
//
// UnprovenTargets is the packet-level disclosure SI-178 fixes: every
// declared target whose conflict evaluation reached
// policyconflict.VerdictBlockedUnproven, named on the record itself rather
// than discoverable only by walking the nested conflict reports. It is
// always present (never omitted when empty), so a reader can distinguish
// "no unproven targets" from "this record does not disclose them."
type SubmitPreparationResult struct {
	Schema             string             `json:"schema"`
	Identity           Identity           `json:"identity"`
	Validation         ValidateResult     `json:"validation"`
	ImpactReview       ImpactReviewResult `json:"impact_review"`
	ReadyForSubmission bool               `json:"ready_for_submission"`
	BlockingReasons    []string           `json:"blocking_reasons,omitempty"`
	UnprovenTargets    []string           `json:"unproven_targets"`
}

// SubmitPreparation composes Validate and ImpactReview into one
// submission-readiness packet without merging, approving, or writing
// anything. ReadyForSubmission is false whenever:
//
//   - the proposed store has adopted no constitution at all, or adopted one
//     incompletely (there is nothing to submit; the disclosed
//     policyauthority reason travels on Validation.Snapshot.Reason);
//   - a declared target could not be evaluated at all (its per-target
//     Refusal);
//   - a declared target's conflict evaluation reached
//     policyconflict.VerdictBlockedViolated — a mechanically PROVEN conflict
//     (AC-3: "A mechanically proven conflict cannot be dismissed as
//     no-conflict. It must be fixed, superseded, narrowed, or covered by an
//     authorized exemption");
//   - a declared target's conflict evaluation reached
//     policyconflict.VerdictBlockedUnproven — SI-178's chosen semantics (c).
//     spec/context-integrity-v2 DC-6/DC-7 hold that unknown and unresolved
//     states "block authoritative progression," so this public, versioned
//     record must never read affirmatively clean over one. The target is
//     named in BlockingReasons with its own policyconflict reason codes
//     (e.g. judge-unavailable) and repeated in UnprovenTargets. This
//     invents no approval record and no conflict semantics: merge and
//     approval stay outside the operation, and a human still acts on the
//     packet's complete witnesses through the normal pull-request review;
//   - a target's report carries a verdict outside policyconflict's closed
//     three-state vocabulary (unknown values fail closed).
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
	// Validate and ImpactReview each resolve the checkout's identity
	// independently, so a checkout that moves between them would compose one
	// commit's validation with another commit's impact review into a single
	// packet that describes no repository state that ever existed. Refuse
	// instead — operationally, since the cause is an external mutation
	// racing this read, not a governance verdict about the proposal.
	if validation.Identity.Head != review.Identity.Head || validation.Identity.Branch != review.Identity.Branch {
		return nil, operational("identity-shifted", fmt.Sprintf(
			"the checkout moved during submission preparation: validation observed %s@%s, impact review observed %s@%s",
			validation.Identity.Branch, validation.Identity.Head, review.Identity.Branch, review.Identity.Head), nil)
	}

	ready := true
	blocking := []string{}
	unproven := []string{}

	if !validation.Snapshot.Adopted {
		ready = false
		blocking = append(blocking, "the proposed constitution store has adopted no constitution: "+validation.Snapshot.Reason)
	}
	for _, tc := range review.Conflicts {
		switch {
		case tc.Refusal != "":
			ready = false
			blocking = append(blocking, "target "+tc.Target.Spec+": "+tc.Refusal)
		case tc.Report == nil:
			ready = false
			blocking = append(blocking, "target "+tc.Target.Spec+": conflict evaluation produced no report")
		default:
			switch tc.Report.Verdict {
			case policyconflict.VerdictPass:
			case policyconflict.VerdictBlockedViolated:
				ready = false
				blocking = append(blocking, "target "+tc.Target.Spec+": a mechanically proven conflict blocks submission")
			case policyconflict.VerdictBlockedUnproven:
				ready = false
				unproven = append(unproven, tc.Target.Spec)
				blocking = append(blocking, "target "+tc.Target.Spec+": conflict evaluation is unproven ("+unprovenReasons(tc.Report)+") and cannot be recorded as submission-clean")
			default:
				ready = false
				blocking = append(blocking, "target "+tc.Target.Spec+": unrecognized conflict verdict "+string(tc.Report.Verdict))
			}
		}
	}

	return &SubmitPreparationResult{
		Schema:             SubmitPreparationResultSchema,
		Identity:           review.Identity,
		Validation:         *validation,
		ImpactReview:       *review,
		ReadyForSubmission: ready,
		BlockingReasons:    blocking,
		UnprovenTargets:    unproven,
	}, nil
}

// unprovenReasons renders the sorted, deduplicated policyconflict reason
// codes carried by report's own UNPROVEN rows, so a blocking reason names
// why the target is unproven (judge-unavailable, disposition-required, ...)
// in the kernel's own vocabulary rather than in a phrase this package
// invents. A report whose verdict is blocked-unproven always has at least
// one unproven row; the empty fallback exists so the message can never be
// silently truncated to an empty parenthesis if that ever stops holding.
func unprovenReasons(report *policyconflict.Report) string {
	seen := map[string]bool{}
	collect := func(state policyconflict.ProofState, reasons []policyconflict.ReasonCode) {
		if state != policyconflict.ProofUnproven {
			return
		}
		for _, r := range reasons {
			seen[string(r)] = true
		}
	}
	for _, row := range report.Mechanical {
		collect(row.State, row.Reasons)
	}
	for _, row := range report.Semantic {
		collect(row.State, row.Reasons)
	}
	if len(seen) == 0 {
		return "no reason code disclosed"
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return strings.Join(codes, ", ")
}
