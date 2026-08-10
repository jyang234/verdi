package main

import "sort"

// SpecTransitionVerbs names the rituals that advance a spec through its
// lifecycle — the "ritual verbs for spec classes" internal/model's
// embedded canonical model (Task 6, spec/model-schema ac-2) must agree
// with exactly.
//
// These are NOT all CLI verbs. Two different inventories overlap here and
// must not be conflated:
//
//   - `merge` is a FORGE transition. Specification acceptance is
//     merge-signaled: merging the reviewed specification pull request into
//     the configured default branch accepts that exact landed revision
//     (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-
//     design.md — "the successful merge is the state transition"), and
//     internal/specstate derives the state from Git. `verdi accept` is
//     retired: accept.go prints a compatibility notice and flips nothing,
//     so it is no longer a status-flip site and no longer a catalog verb —
//     even though it remains a recognized CLI verb (dispatch.go's
//     verbPhase) for that compatibility window. SpecTransitionForgeVerbs
//     below names this half.
//   - `close` is a live Verdi CLI verb and a real status-flip site
//     (grep-verified: close.go's closeAcceptedStatusLineRe,
//     accepted-pending-build -> closed).
//
// `build start` (buildstart.go) cuts a branch without touching status at
// all, so it is not one of these. The accepted-pending-build ->
// superseded flip a PREDECESSOR spec undergoes when its successor is
// accepted (supersede.go's supersedePredecessors, on a DIFFERENT spec
// object) is a side effect of acceptance, never its own verb-transition —
// matching the reference guide's own framing (docs/design/concepts/
// 2026-07-17-integration-startup-guide.md §8.3: "accepting v2 flips v1's
// status to superseded").
//
// package main cannot itself be imported by internal/model (Go forbids
// importing package main), so this is exported for documentation
// clarity and any future same-repo consumer, not for cross-package
// linkage: internal/model/canonical.go's own transition verbs are
// compared against this set from a test IN this package instead
// (modelparity_test.go's TestCanonicalModel_VerbsMatchDispatch).
func SpecTransitionVerbs() []string {
	verbs := []string{"merge", "close"}
	sort.Strings(verbs)
	return verbs
}

// SpecTransitionForgeVerbs names the subset of SpecTransitionVerbs that
// are FORGE transitions rather than Verdi CLI verbs — the ones dispatch.go
// deliberately does not recognize, because no Verdi command performs them.
// GLG v3 dc-3 admits both kinds as safe actions ("registered existing
// Verdi verbs or forge transitions"), so a projection naming one of these
// is naming a real transition, not an unknown command.
func SpecTransitionForgeVerbs() []string {
	verbs := []string{"merge"}
	sort.Strings(verbs)
	return verbs
}
