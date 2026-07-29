package disclosure

// The pending-supersession disclosure family lives here, in the seam
// package itself, because ONE underlying state — a story implements a
// feature, but open supersession MRs cannot be enumerated, so the
// pending-supersession fold is unprovable — has producers in TWO
// different trees: cmd/verdi's closure gate
// (closuregate.go's checkPendingSupersessionCondition, which discloses it
// with its own cause text at its decision point) and internal/wallbadge's
// ladder badge (ladder.go's PendingSupersessionBadge). CLAUDE.md's rule
// is that anything used by two or more packages lives in one shared
// internal/ package, and cmd/verdi cannot be that home (nothing under
// internal/ may import a main package) — so the source id and the wall
// badge's cause constructors sit next to New/Render: one home, no copy to
// drift (the reviewfeed.go placement reasoning).
//
// The migration this file completes: the wall badge hand-authored its
// case-file lines ("pending-supersession is disclosed-unproven: ...") —
// the seam grammar's word order inverted — so disclosure.IsRendered
// rejected them and a reader who learned the gate's rendering of the SAME
// state did not recognize the wall's (judged-ac-1-vocabulary-coverage;
// spec/disclosure-legibility#ac-1 binds every surface to one vocabulary).

// SourcePendingSupersession is the source id every pending-supersession
// disclosure carries, whichever surface renders it: the closure gate and
// the wall badge disclose the SAME underlying condition (open
// supersession MRs cannot be enumerated), so they share the gate's own
// existing id rather than minting a per-surface synonym — one state = one
// source everywhere. Their difference lives in the text, which names each
// site's observed cause.
const SourcePendingSupersession = "gate:pending-supersession"

// PendingSupersessionNoForge is the wall badge's no-forge cause: the
// story implements a feature, but no forge is configured at all (a nil
// candidate loader), so open supersession MRs cannot be enumerated —
// disclosed-unproven rather than silently "not flagged" (badge-computes
// ac-3). The scope is empty: the condition is the checkout's forge
// wiring, not any one artifact (the checkout-wide form).
func PendingSupersessionNoForge() Disclosure {
	return New(SourcePendingSupersession, "", "no forge is configured to enumerate open MRs")
}

// PendingSupersessionNoDefaultBranch is the wall badge's
// configured-but-unable cause: a forge is wired, but candidate loading
// reported it could not enumerate (no default branch resolved) — the same
// disclosed-unproven contract as the no-forge cause, same source, cause
// named in the text.
func PendingSupersessionNoDefaultBranch() Disclosure {
	return New(SourcePendingSupersession, "", "open MRs could not be enumerated (no default branch resolved)")
}
