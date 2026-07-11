// Open-MR listing (V1-P3): the forge port's second I-22 extension. v0's
// port (PLAN.md §7 I-22) has no MR-query surface at all — it only pulls a
// CI evidence bundle by (ref, commit). The rung-4 cascade fold's
// `pending-supersession` flag needs one: "the fold's input set includes
// open supersession MRs" (03 §The amendment ladder), so a story's edges
// can be checked against a feature-supersession manifest that has not
// merged yet.
//
// The port surface stays exactly as tiny as I-22's own precedent
// ("adapters absorb the forge-native shape"): enough to (1) discover which
// MRs/PRs are open against a target branch, and (2) read one file's
// content from an open MR's source branch — enough to fetch a pending
// supersession manifest's spec file (PLAN-V1.md §3: "list open MRs
// targeting the default branch, enough to fetch a pending manifest's spec
// content"). It does not enumerate an MR's changed files or diff — that
// remains out of scope here; the caller (internal/evidence) already knows
// which candidate spec path to probe (R4-I-14's `<name>-v2` supersession
// naming convention). This method is distinct from V1-P7's later
// comment-thread round-trip methods (05 §Review stickies and forge
// round-trip) and creates no dependency on that later wave.
package forge

// OpenMR is one open (unmerged) merge/pull request targeting a branch,
// through whichever forge hosts the store.
type OpenMR struct {
	// ID is the forge-native MR/PR identifier (GitLab IID, GitHub PR
	// number) rendered as a string — the two forges' numbering spaces are
	// unrelated, so this is never compared across forges, only used for
	// disclosure/logging and to key a fetched candidate back to the MR
	// that produced it.
	ID string
	// SourceBranch is the MR's source (head) branch — the ref
	// FetchFileAtRef reads content from.
	SourceBranch string
	// Title is the MR's title, carried through for disclosure only.
	Title string
}
