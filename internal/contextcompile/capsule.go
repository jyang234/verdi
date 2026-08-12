package contextcompile

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// BoundObligation is one exact obligation payload bound to a target
// acceptance-criterion/evidence-kind pair.
type BoundObligation struct {
	Ref           string
	Path          string
	TargetRef     string
	AC            string
	Kind          artifact.EvidenceKind
	ContentDigest string
	Content       []byte
}

// PolicyOperand is one applicable constitution policy, overlay or exemption
// operand included in a phase capsule.
type PolicyOperand struct {
	Kind   string
	ID     string
	Path   string
	Digest string
	Scope  policyartifact.Scope
}

// reviewRequiredInputs returns Compile's fixed, sorted five-row review
// required_inputs set and its matching three-code disclosure union
// (authority design §6/§8.2): accepted-spec and review-policy proven with a
// digest witness, and builder-receipt/evidence-bundle/result-diff unproven
// with their fixed disclosure codes — v1 has no port that could resolve
// them. assembleManifest calls this directly for PhaseReview only; design
// and build carry explicit empty RequiredInputs/Disclosures instead.
func reviewRequiredInputs(acceptedDigest, policyDigest string) ([]RequiredInput, []DisclosureCode) {
	accepted := acceptedDigest
	policy := policyDigest
	inputs := []RequiredInput{
		{Kind: RequiredInputAcceptedSpec, Resolution: ResolutionProven, Digest: &accepted, Witnesses: []string{}},
		{Kind: RequiredInputBuilderReceipt, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewBuilderReceiptUnproven)}},
		{Kind: RequiredInputEvidenceBundle, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewEvidenceBundleUnproven)}},
		{Kind: RequiredInputResultDiff, Resolution: ResolutionUnproven, Witnesses: []string{string(DisclosureReviewResultDiffUnproven)}},
		{Kind: RequiredInputReviewPolicy, Resolution: ResolutionProven, Digest: &policy, Witnesses: []string{}},
	}
	disclosures := []DisclosureCode{
		DisclosureReviewBuilderReceiptUnproven,
		DisclosureReviewEvidenceBundleUnproven,
		DisclosureReviewResultDiffUnproven,
	}
	return inputs, disclosures
}

// cloneFeatureFragments returns a deep copy of in: a fresh Targets slice
// per fragment (each with its own fresh Evidence slice), and fresh
// Constraints/Decisions slices, so a caller holding the clone can never
// observe or cause a mutation through the original slice.
func cloneFeatureFragments(in []FeatureFragment) []FeatureFragment {
	out := make([]FeatureFragment, len(in))
	for i, fragment := range in {
		out[i] = fragment
		out[i].Targets = make([]FragmentTarget, len(fragment.Targets))
		for j, target := range fragment.Targets {
			out[i].Targets[j] = target
			out[i].Targets[j].Evidence = append([]artifact.EvidenceKind(nil), target.Evidence...)
		}
		out[i].Constraints = cloneConstraints(fragment.Constraints)
		out[i].Decisions = cloneDecisions(fragment.Decisions)
	}
	return out
}

// cloneStrings returns a fresh copy of in, preserving a nil input as nil
// (never collapsing it to an explicit empty slice) so a caller's own
// nil-vs-[] distinction survives the clone.
func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}

// cloneGrantSet returns a deep copy of in: fresh Paths/Argv0s slices and a
// fresh Ceilings map per grant, so a caller holding the clone can never
// observe or cause a mutation through the original GrantSet.
func cloneGrantSet(in execworkspace.GrantSet) execworkspace.GrantSet {
	if in.Grants == nil {
		return execworkspace.GrantSet{}
	}
	out := execworkspace.GrantSet{Grants: make([]execworkspace.Grant, len(in.Grants))}
	for i, grant := range in.Grants {
		out.Grants[i] = grant
		out.Grants[i].Paths = append([]string(nil), grant.Paths...)
		out.Grants[i].Argv0s = append([]string(nil), grant.Argv0s...)
		if grant.Ceilings != nil {
			out.Grants[i].Ceilings = make(map[string]int, len(grant.Ceilings))
			for name, ceiling := range grant.Ceilings {
				out.Grants[i].Ceilings[name] = ceiling
			}
		}
	}
	return out
}
