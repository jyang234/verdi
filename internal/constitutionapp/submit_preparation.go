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
//     three-state vocabulary (unknown values fail closed);
//   - the proposal changes at least one constitution source layer and NO
//     governed target was evaluated at all. Readiness starts true and only
//     per-target findings lower it, so an empty declared set over a real
//     policy delta would otherwise report zero coverage as zero impact. This
//     operation still derives no affected set of its own — no ratified
//     reverse-applicability seam exists in this repository and inventing one
//     is the Task 3 stop gate — it refuses to read clean over the gap, which
//     the caller closes by declaring its targets.
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
	if difference := proposalDifference(validation.Snapshot, review.Proposed); difference != "" {
		return nil, operational("identity-shifted", "the proposed constitution changed during submission preparation: "+difference, nil)
	}

	ready := true
	blocking := []string{}
	unproven := []string{}

	if !validation.Snapshot.Adopted {
		ready = false
		blocking = append(blocking, "the proposed constitution store has adopted no constitution: "+validation.Snapshot.Reason)
	}
	// A non-empty policy delta with NO evaluated coverage at all cannot read
	// submission-ready: ready starts true and only per-target findings lower
	// it, so a caller that declares no target would otherwise receive an
	// affirmatively clean packet over a changed constitution whose impact
	// nothing examined — zero coverage reported as zero impact. This
	// operation does not (and must not) derive the affected set itself: no
	// ratified reverse-applicability seam exists in this repository, and
	// inventing one is the Task 3 stop gate. It refuses to read clean over
	// the gap instead, exactly as SI-178(c) refuses to read clean over an
	// unproven conflict verdict. Declaring the targets closes it.
	if len(review.Layers) > 0 && len(review.AffectedConsumers) == 0 {
		ready = false
		blocking = append(blocking, fmt.Sprintf(
			"the proposal changes %d constitution source layer(s) but no governed target was evaluated: submission readiness cannot be asserted with no impact coverage at all", len(review.Layers)))
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
		Identity:           pinned,
		Validation:         *validation,
		ImpactReview:       *review,
		ReadyForSubmission: ready,
		BlockingReasons:    blocking,
		UnprovenTargets:    unproven,
	}, nil
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
