package constitutionapp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// SubmitPreparationRequest is SubmitPreparation's strict request: the same
// shape ImpactReview accepts, since preparation composes Validate and
// ImpactReview rather than deriving anything new. The checkout root is
// resolved by the adapter, never accepted as request content
// (InspectRequest's own doc comment explains why).
type SubmitPreparationRequest struct {
	Schema  string         `json:"schema"`
	Targets []ImpactTarget `json:"targets"`
}

func (r SubmitPreparationRequest) validate() error {
	return validateTargets(r.Targets)
}

// SubmitPreparationResult is the submission-readiness packet AC-6's
// "Prepare review" names: the validated proposal, its impact-review packet,
// and one closed ReadyForSubmission verdict. It persists nothing —
// preparation writes only the existing proposal artifacts Propose already
// committed (design §7.1); merge and approval remain the normal Git
// pull-request boundary, entirely outside this operation.
//
// UnprovenTargets names registered consumers whose canonical conflict result
// is blocked-unproven. It is always present; supplemental caller previews do
// not participate in readiness.
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
//   - canonical registered-consumer coverage is not proven, including a
//     missing inventory, empty changed universe, unavailable evaluator, or
//     unknown applicability/conflict posture;
//   - a canonical consumer's conflict evaluation reached
//     policyconflict.VerdictBlockedViolated — a mechanically PROVEN conflict
//     (AC-3: "A mechanically proven conflict cannot be dismissed as
//     no-conflict. It must be fixed, superseded, narrowed, or covered by an
//     authorized exemption");
//   - a canonical consumer's conflict evaluation reached
//     policyconflict.VerdictBlockedUnproven — SI-178's chosen semantics (c).
//     spec/context-integrity-v2 DC-6/DC-7 hold that unknown and unresolved
//     states "block authoritative progression," so this public, versioned
//     record must never read affirmatively clean over one. The target is
//     named in BlockingReasons with its own policyconflict reason codes
//     (e.g. judge-unavailable) and repeated in UnprovenTargets. This
//     invents no approval record and no conflict semantics: merge and
//     approval stay outside the operation, and a human still acts on the
//     packet's complete witnesses through the normal pull-request review; or
//   - a canonical consumer's report carries a verdict outside
//     policyconflict's closed three-state vocabulary (unknown values fail
//     closed).
//
// Caller targets are supplemental preview rows. They never satisfy coverage,
// remove a registered consumer, or participate in readiness.
//
// It refuses outright, rather than reporting, when the repository or the
// proposal itself moves mid-operation: one identity is pinned before the two
// sub-operations and re-observed after them, and the proposed constitution's
// own digests are compared across the two reads, so a packet can never
// staple one repository state's validation onto another's impact review.
func (s Service) SubmitPreparation(ctx context.Context, root string, req SubmitPreparationRequest) (*SubmitPreparationResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}

	// ONE repository observation, resolved once and threaded through both
	// sub-operations, then RE-OBSERVED and compared field by field once they
	// have returned. Threading alone would only make the packet's two halves
	// agree by construction while the underlying trees moved underneath them;
	// re-observing is what proves nothing moved. Identity is a comparable
	// struct, so the comparison covers every field it will ever gain —
	// branch, head, dirty, accepted_known, default_branch, accepted_head —
	// rather than the two a hand-written equality would remember to list.
	pinned, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}
	validation, typed := s.validateAt(root, pinned)
	if typed != nil {
		return nil, typed
	}
	review, typed := s.impactReviewAt(ctx, root, pinned, ImpactReviewRequest{Schema: ImpactReviewRequestSchema, Targets: req.Targets})
	if typed != nil {
		return nil, typed
	}
	observed, typed := s.resolveIdentity(ctx, root)
	if typed != nil {
		return nil, typed
	}
	// Operational, not verdict: the cause is an external mutation racing this
	// read, not a governance judgment about the proposal.
	if pinned != observed {
		return nil, operational("identity-shifted", fmt.Sprintf(
			"the repository moved during submission preparation: it was observed at %s before, and at %s after",
			describeIdentity(pinned), describeIdentity(observed)), nil)
	}
	// Git refs are not the whole identity: an UNCOMMITTED edit to the
	// proposed constitution moves no ref at all, so the two halves of the
	// packet can describe two different proposals while every Identity field
	// still agrees. The proposal's own resolved content is therefore pinned
	// too, through the digests policyauthority already computes.
	if !review.Proposed.unavailable {
		if difference := proposalDifference(validation.Snapshot, review.Proposed); difference != "" {
			return nil, operational("identity-shifted", "the proposed constitution changed during submission preparation: "+difference, nil)
		}
	}

	ready := review.Coverage.State == constitutionimpact.StateProven
	blocking := []string{}
	unproven := []string{}

	if !validation.Snapshot.Adopted {
		ready = false
		blocking = append(blocking, "the proposed constitution store has adopted no constitution: "+validation.Snapshot.Reason)
	}
	if review.Coverage.State != constitutionimpact.StateProven {
		blocking = append(blocking, "canonical impact coverage is "+string(review.Coverage.State)+": "+coverageReasons(review.Coverage.Reasons))
	}
	for _, evaluation := range review.Coverage.Evaluations {
		switch {
		case evaluation.Refusal != nil:
			ready = false
			blocking = append(blocking, "registered consumer "+evaluation.ConsumerIdentity+": conflict evaluation was refused")
		case evaluation.Report == nil:
			ready = false
			blocking = append(blocking, "registered consumer "+evaluation.ConsumerIdentity+": conflict evaluation produced no report")
		default:
			switch evaluation.Report.Verdict {
			case policyconflict.VerdictPass:
			case policyconflict.VerdictBlockedViolated:
				ready = false
				blocking = append(blocking, "registered consumer "+evaluation.ConsumerIdentity+": a mechanically proven conflict blocks submission")
			case policyconflict.VerdictBlockedUnproven:
				ready = false
				unproven = append(unproven, evaluation.ConsumerIdentity)
				blocking = append(blocking, "registered consumer "+evaluation.ConsumerIdentity+": conflict evaluation is unproven ("+unprovenReasons(evaluation.Report)+") and cannot be recorded as submission-clean")
			default:
				ready = false
				blocking = append(blocking, "registered consumer "+evaluation.ConsumerIdentity+": unrecognized conflict verdict "+string(evaluation.Report.Verdict))
			}
		}
	}
	sort.Strings(unproven)

	return &SubmitPreparationResult{
		Schema:             SubmitPreparationResultSchema,
		Identity:           pinned,
		Validation:         *validation,
		ImpactReview:       *review,
		ReadyForSubmission: ready,
		BlockingReasons:    blocking,
		UnprovenTargets:    unproven,
	}, nil
}

func coverageReasons(reasons []constitutionimpact.Reason) string {
	parts := make([]string, len(reasons))
	for i, reason := range reasons {
		parts[i] = string(reason.Code) + " [" + strings.Join(reason.Witnesses, ", ") + "]"
	}
	if len(parts) == 0 {
		return "no reason disclosed"
	}
	return strings.Join(parts, "; ")
}

// describeIdentity renders one Identity for a refusal diagnostic: every
// field that names a repository observation, so a reader learns WHICH part
// moved rather than only that something did.
func describeIdentity(identity Identity) string {
	accepted := "accepted unresolved"
	if identity.AcceptedKnown {
		accepted = identity.DefaultBranch + "@" + identity.AcceptedHead
	}
	dirty := "clean"
	if identity.Dirty {
		dirty = "dirty"
	}
	return identity.Branch + "@" + identity.Head + " (" + dirty + ", accepted " + accepted + ")"
}

// proposalDifference reports the first way two reads of the SAME proposed
// constitution disagree, or "" when they are identical. It compares only
// what policyauthority itself already computed — adoption, the constitution
// and profile digests, and every source layer's (kind, id, digest) — never a
// digest this package invents over a projection of its own.
func proposalDifference(first, second Snapshot) string {
	switch {
	case first.Adopted != second.Adopted:
		return fmt.Sprintf("adoption changed (%t then %t)", first.Adopted, second.Adopted)
	case first.ConstitutionDigest != second.ConstitutionDigest:
		return fmt.Sprintf("constitution digest changed (%s then %s)", first.ConstitutionDigest, second.ConstitutionDigest)
	case first.ProfileDigest != second.ProfileDigest:
		return fmt.Sprintf("governance profile digest changed (%s then %s)", first.ProfileDigest, second.ProfileDigest)
	case len(first.Layers) != len(second.Layers):
		return fmt.Sprintf("source-layer count changed (%d then %d)", len(first.Layers), len(second.Layers))
	}
	for i, layer := range first.Layers {
		other := second.Layers[i]
		if layer.Kind != other.Kind || layer.ID != other.ID {
			return fmt.Sprintf("source layer %d changed identity (%s %s then %s %s)", i, layer.Kind, layer.ID, other.Kind, other.ID)
		}
		if layer.Digest != other.Digest {
			return fmt.Sprintf("source layer %s digest changed (%s then %s)", layer.ID, layer.Digest, other.Digest)
		}
	}
	return ""
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
