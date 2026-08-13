// exemption.go applies structural exemption departure to one already-proven
// mechanical row (authority design §5.5, ledger SI-95). Task 8 owns how an
// ExemptionResolution's five authority-resolution states (match, freshness,
// scope coverage, bound, authorization) are derived; this file only accepts
// a resolution whose states are ALL already proven, removes exactly the
// exact-current claim witnesses it names, and reruns the identical domain
// solver over the remainder. It never interprets dates or principals, and
// it never edits row in place — every path returns a fresh value built from
// row's own fields.
package policyconflict

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// allProven reports whether every one of r's five authority-resolution
// states is proven (authority design §5.5: "Accept ONLY a resolution whose
// match, freshness, scope-coverage, bound, and authorization states are ALL
// proven").
func allProven(r AuthorityResolution) bool {
	return r.Match == ProofProven &&
		r.Freshness == ProofProven &&
		r.Scope == ProofProven &&
		r.Bound == ProofProven &&
		r.Authorization == ProofProven
}

// addReason returns reasons with r appended, deduplicated, and re-sorted —
// never mutating the input slice's backing array.
func addReason(reasons []ReasonCode, r ReasonCode) []ReasonCode {
	for _, x := range reasons {
		if x == r {
			return reasons
		}
	}
	out := make([]ReasonCode, 0, len(reasons)+1)
	out = append(out, reasons...)
	out = append(out, r)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ApplyEffectiveExemptions recomputes row's post-exemption proof from
// resolutions (authority design §5.5). row is never mutated: every return
// path builds and returns a new value. row.Before, row.Claims, row.Scope,
// row.Domain, and every other structural field carry over unchanged — only
// Exemptions, After, State, and Reasons can differ from row's own.
//
// A resolution not all-proven (allProven) is rejected: it is silently
// excluded from application (never applied, never treated as an operational
// caller defect — an authority shortfall is routine three-valued-honesty
// territory, not a malformed call), and its exclusion is disclosed via
// ReasonExemptionIneffective on the returned row whenever no OTHER
// resolution ends up covering the conflict either. This package's own
// choice between "operational error" and "untouched row" for this case (the
// brief leaves it open) is: untouched row, silently-excluded resolution,
// disclosed reason code — see the task report for the full rationale.
//
// An accepted resolution's RemovedClaims must each name a claim digest
// exactly present among row.Claims (its "match" state already being proven
// is what makes this an operational contract, not a routine shortfall: a
// digest absent here means the resolution was constructed against a
// different row's witnesses).
func ApplyEffectiveExemptions(row MechanicalEvaluation, resolutions []ExemptionResolution) (MechanicalEvaluation, error) {
	accepted := make([]ExemptionResolution, 0, len(resolutions))
	anyRejected := false
	for _, r := range resolutions {
		if allProven(r.Resolution) {
			accepted = append(accepted, r)
		} else {
			anyRejected = true
		}
	}
	sort.Slice(accepted, func(i, j int) bool { return accepted[i].ID < accepted[j].ID })

	out := row
	out.Reasons = append([]ReasonCode{}, row.Reasons...)

	if len(accepted) == 0 {
		out.Exemptions = []ExemptionResolution{}
		out.After = row.Before
		if anyRejected {
			out.Reasons = addReason(out.Reasons, ReasonExemptionIneffective)
		}
		return out, nil
	}

	removed := map[string]bool{}
	for _, r := range accepted {
		for _, w := range r.RemovedClaims {
			found := false
			for _, c := range row.Claims {
				if c.ClaimDigest == w.Digest {
					found = true
					break
				}
			}
			if !found {
				return MechanicalEvaluation{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q names claim digest %q, absent from row %q's current claims", r.ID, w.Digest, row.ID)
			}
			removed[w.Digest] = true
		}
	}

	remainder := make([]TypedClaimRecord, 0, len(row.Claims))
	for _, c := range row.Claims {
		if !removed[c.ClaimDigest] {
			remainder = append(remainder, c)
		}
	}

	var afterProof SolverProof
	if row.Domain == domainPrincipalRelation {
		afterProof = rerunPrincipalRelationWithoutKernel(remainder, row.Before)
	} else {
		var err error
		afterProof, err = rerunSolver(row.Domain, remainder)
		if err != nil {
			return MechanicalEvaluation{}, err
		}
	}

	// "A mechanical conflict is covered only when the post-exemption
	// conjunction is satisfiable or disjoint" (§5.5): row.Scope is the
	// group's own already-proven scope proof, unchanged by exemption
	// application, so a proven-disjoint group never needed exemption
	// coverage from the solver result at all.
	covered := afterProof.State == SolverSatisfiable || row.Scope.State == ScopeDisjoint

	out.Exemptions = accepted
	out.After = afterProof
	if covered {
		out.State = ProofProven
		out.Reasons = []ReasonCode{ReasonExemptionEffective}
	} else {
		out.Reasons = addReason(out.Reasons, ReasonExemptionIneffective)
	}
	return out, nil
}

// rerunSolver reruns the identical domain solver (mechanical.go's own
// solveDiscrete/solveInterval/solvePathCapability — every one a pure
// function with no kernel or scope-resolver dependency) over remainder.
// domainPrincipalRelation is handled by rerunPrincipalRelationWithoutKernel
// instead; it never reaches this function.
func rerunSolver(domain string, remainder []TypedClaimRecord) (SolverProof, error) {
	claims := make([]policyartifact.Claim, len(remainder))
	for i, c := range remainder {
		claims[i] = c.Claim
	}
	switch domain {
	case domainDiscreteSet:
		return solveDiscrete(claims), nil
	case domainIntegerInterval:
		return solveInterval(claims)
	case domainPathCapability:
		return solvePathCapability(claims), nil
	default:
		return SolverProof{}, fmt.Errorf("policyconflict: apply exemptions: unsupported mechanical domain %q", domain)
	}
}

// rerunPrincipalRelationWithoutKernel is the identity domain's post-
// exemption rerun. ApplyEffectiveExemptions' fixed signature carries no
// context, profile, or actors (the exact API contract this package
// implements), so it cannot construct a fresh kernel authorization request
// the way solvePrincipalRelation does. The only kernel-independent fact
// still provable here is that removing every same-/different-principal
// claim leaves nothing left to require a relation at all — trivially
// satisfiable. Any remainder that still asserts a relation cannot be
// soundly re-verified without the kernel, so this conservatively returns
// before unchanged rather than inventing a new proof (never a false pass;
// see the task report's disclosed judgment call).
func rerunPrincipalRelationWithoutKernel(remainder []TypedClaimRecord, before SolverProof) SolverProof {
	for _, c := range remainder {
		if c.Claim.Operator == policyartifact.OpSamePrincipal || c.Claim.Operator == policyartifact.OpDifferentPrincipal {
			return before
		}
	}
	return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}
}
