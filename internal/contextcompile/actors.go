// Actor projection (authority design §4): the manifest's `actors` section
// is a canonical projection of already-sealed
// governanceprincipal.PrincipalResolution values, never a reconstitutable
// one. This file owns the one seam that turns an injected ActorResolver
// into that projection; it never mints, forges, or reconstructs a kernel
// seal itself.
package contextcompile

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// claimIdentity is a resolution's duplicate-detection key: the
// (claim.trust_source, claim.subject) pair authority design §4/§8.2 sorts
// and de-duplicates resolutions by.
type claimIdentity struct {
	trustSource string
	subject     string
}

// projectActors resolves the manifest's `actors` section.
//
// A nil resolver is the v1 explicit-absence path (authority design §4):
// posture is unproven, resolutions is the explicit empty set (never a nil
// slice that would serialize differently), and disclosure
// actor-resolution-unproven is mandatory. No error is possible on this
// path.
//
// A non-nil resolver supplies zero or more already-sealed
// governanceprincipal.PrincipalResolution values through its Resolutions
// port method:
//
//   - a port error is wrapped and returned as an operational error;
//   - every resolution is verified sealed and unmodified through
//     governanceprincipal.AttributionFromResolution — the kernel's public
//     seal checker (this package never inspects or reconstructs the
//     private seal itself); a forged or mutated resolution is an
//     operational error;
//   - duplicate claim identity across the returned set is an operational
//     error (fails closed rather than silently picking one);
//   - the accepted resolutions are deep-copied (including their witness
//     slices) so the returned section never aliases the port's backing
//     memory, then sorted by (claim.trust_source, claim.subject) for a
//     deterministic manifest.
//
// posture is then computed by the closed algebra: any violated resolution
// makes the whole section violated-with-witness; otherwise any unproven
// resolution, or an empty accepted set, makes it unproven; otherwise (every
// resolution authenticated) it is proven. Disclosure
// actor-resolution-unproven is present if and only if posture is unproven;
// otherwise Disclosures is the explicit empty set.
func projectActors(ctx context.Context, resolver ActorResolver) (ActorsSection, error) {
	if resolver == nil {
		return ActorsSection{
			Posture:     ResolutionUnproven,
			Resolutions: []governanceprincipal.PrincipalResolution{},
			Disclosures: []DisclosureCode{DisclosureActorResolutionUnproven},
		}, nil
	}

	resolutions, err := resolver.Resolutions(ctx)
	if err != nil {
		return ActorsSection{}, fmt.Errorf("contextcompile: resolving actors: %w", err)
	}

	out := make([]governanceprincipal.PrincipalResolution, 0, len(resolutions))
	seen := make(map[claimIdentity]bool, len(resolutions))
	for _, res := range resolutions {
		if _, err := governanceprincipal.AttributionFromResolution(res); err != nil {
			return ActorsSection{}, fmt.Errorf("contextcompile: actor resolution failed kernel seal verification: %w", err)
		}

		id := claimIdentity{trustSource: res.Claim.TrustSource, subject: res.Claim.Subject}
		if seen[id] {
			return ActorsSection{}, fmt.Errorf("contextcompile: actor resolutions carry duplicate claim identity %q/%q", id.trustSource, id.subject)
		}
		seen[id] = true

		cp := res
		cp.Witnesses = append([]governanceprincipal.Witness(nil), res.Witnesses...)
		out = append(out, cp)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Claim.TrustSource != out[j].Claim.TrustSource {
			return out[i].Claim.TrustSource < out[j].Claim.TrustSource
		}
		return out[i].Claim.Subject < out[j].Claim.Subject
	})

	posture := actorPosture(out)
	disclosures := []DisclosureCode{}
	if posture == ResolutionUnproven {
		disclosures = []DisclosureCode{DisclosureActorResolutionUnproven}
	}

	return ActorsSection{
		Posture:     posture,
		Resolutions: out,
		Disclosures: disclosures,
	}, nil
}

// actorPosture computes the manifest's actors.posture algebra: violated
// wins outright over unproven; an empty accepted set is unproven, never
// vacuously proven; only an entirely authenticated nonempty set is proven.
func actorPosture(resolutions []governanceprincipal.PrincipalResolution) Resolution {
	if len(resolutions) == 0 {
		return ResolutionUnproven
	}
	sawUnproven := false
	for _, res := range resolutions {
		switch res.State {
		case governanceprincipal.ResolutionViolated:
			return ResolutionViolatedWithWitness
		case governanceprincipal.ResolutionUnproven:
			sawUnproven = true
		}
	}
	if sawUnproven {
		return ResolutionUnproven
	}
	return ResolutionProven
}
