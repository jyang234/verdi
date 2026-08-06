package humanartifact

import "sort"

// init pre-registers a no-extensions Contract for every artifact kind
// kernelFieldTable (kernel.go) recognizes — the ten kinds AC-1 names:
// feature, story, adr, attestation, waiver, reaffirmation, obligation,
// policy, policy-overlay, policy-exemption. Deriving the kind list from
// kernelFieldTable's own keys, rather than a second hand-maintained
// list, means the two tables can never drift apart. The contract's mere
// existence and its kernel-collision validation are the deliverable
// here; AI-assisted-spec-design later maps model descriptors onto real
// Extensions (CX-16/R-10).
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
