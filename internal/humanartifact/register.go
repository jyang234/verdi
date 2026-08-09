package humanartifact

import "sort"

// init pre-registers a no-extensions Contract for every artifact kind
// kernelFieldTable (kernel.go) recognizes: feature, story, adr,
// attestation, waiver, reaffirmation, obligation, policy, policy-
// overlay, policy-exemption. Deriving the kind list from
// kernelFieldTable's own keys, rather than a second hand-maintained
// list, means the two tables can never drift apart. The contract's mere
// existence and its kernel-collision validation are the deliverable
// here; AI-assisted-spec-design later maps model descriptors onto real
// Extensions (CX-16/R-10).
//
// AC-1 additionally names "component specs" among the human-authored
// kinds an operating model resolves a scaffold for. This package does
// NOT yet carry a kernel-field table or Contract for the component
// class: internal/model's own canonical default model registers only
// feature and story (internal/model/canonical.go — "2 classes, 4
// transitions"), so there is no shipped scaffold consumer for it today.
// Requesting one anyway fails closed rather than silently admitting a
// contract with an unproven kernel boundary: KernelFields("component")
// returns ok=false, so Contract.Validate rejects it as an unrecognized
// artifact family, and RegisterContract would panic rather than
// register it. A future component-spec kernel table is later, narrowly-
// scoped work — not implied by this comment.
func init() {
	kinds := make([]string, 0, len(kernelFieldTable))
	for k := range kernelFieldTable {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		RegisterContract(Contract{Kind: k})
	}
}
