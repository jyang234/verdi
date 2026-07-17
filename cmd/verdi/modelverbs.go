package main

import "sort"

// SpecTransitionVerbs names the subset of dispatch.go's verbPhase verbs
// that actually flip a spec's status field — the "ritual verbs for spec
// classes" internal/model's embedded canonical model (Task 6,
// spec/model-schema ac-2) must agree with exactly.
//
// Grep-verified against this package's own status-flip sites: accept.go
// (draftStatusLineRe: draft -> accepted-pending-build) and close.go
// (closeAcceptedStatusLineRe: accepted-pending-build -> closed) are the
// ONLY two. `build start` (buildstart.go) cuts a branch without
// touching status at all — no verdi status line is ever flipped by it,
// so it is not one of these verbs. The accepted-pending-build ->
// superseded flip a PREDECESSOR spec undergoes when its successor is
// accepted (supersede.go's supersedePredecessors, invoked from within
// the accept ritual on a DIFFERENT spec object) is a side effect of
// `accept`, never its own verb-transition — matching the reference
// guide's own framing (docs/design/concepts/2026-07-17-integration-
// startup-guide.md §8.3: "accepting v2 flips v1's status to
// superseded").
//
// package main cannot itself be imported by internal/model (Go forbids
// importing package main), so this is exported for documentation
// clarity and any future same-repo consumer, not for cross-package
// linkage: internal/model/canonical.go's own transition verbs are
// compared against this set from a test IN this package instead
// (modelparity_test.go's TestCanonicalModel_VerbsMatchDispatch).
func SpecTransitionVerbs() []string {
	verbs := []string{"accept", "close"}
	sort.Strings(verbs)
	return verbs
}
