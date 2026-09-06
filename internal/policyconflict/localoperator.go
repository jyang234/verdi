package policyconflict

import "github.com/jyang234/verdi/internal/governanceprincipal"

// localOperatorDisclosures derives the report-level disclosure a
// local-operator principal resolution requires (2026-09-05 local-operator
// disposition design §2.2, ledger SI-178): every report whose actor
// resolutions include one claimed against a profile-declared
// local-operator trust source carries DisclosureLocalOperatorAsserted —
// regardless of whether that resolution actually succeeded. A violated or
// unproven local-operator attempt still means this evaluation consulted a
// self-asserted identity source, and that must never be hidden by the
// outcome: absence of the disclosure would otherwise be misreadable as "no
// local-operator path was tried," the same failure mode design §2.2 guards
// against for a favorable verdict.
//
// schema.go only defines and validates DisclosureLocalOperatorAsserted;
// this is the seam its own doc comment assigns the job to ("emitting it
// onto a real report is the conflict-service factory's job", design §2.2)
// — Evaluate calls this once, over the exact Actors the whole evaluation
// already resolved, so a profile that declares no local-operator source
// can never produce a report carrying it: byte-identical to a build with
// no local-operator support at all.
//
// The witness set is the sorted, deduplicated trust-source ids the
// disclosure fired for — never a subject or principal id, which would leak
// the self-asserted identity into a disclosure whose whole point is to
// flag the WEAKNESS of the evidence, not to re-publish it (the resolution
// witnesses and, when authenticated, the minted principal id already carry
// that where the design intends it: the resolution's own Witnesses and the
// disposition approval that names the principal).
func localOperatorDisclosures(profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) []Disclosure {
	localSources := make(map[string]bool, len(profile.IdentityTrustSources))
	for _, source := range profile.IdentityTrustSources {
		if source.Kind == governanceprincipal.TrustSourceLocalOperator {
			localSources[source.ID] = true
		}
	}
	if len(localSources) == 0 {
		return []Disclosure{}
	}

	witnesses := make(map[string]bool)
	for _, actor := range actors {
		if localSources[actor.Claim.TrustSource] {
			witnesses[actor.Claim.TrustSource] = true
		}
	}
	if len(witnesses) == 0 {
		return []Disclosure{}
	}
	return []Disclosure{{Code: DisclosureLocalOperatorAsserted, Witnesses: sortedKeysOf(witnesses)}}
}
