// exemption.go applies structural exemption departure to one already-proven
// mechanical row (authority design §5.5, ledger SI-95). Task 8 owns how an
// ExemptionResolution's five authority-resolution states (match, freshness,
// scope coverage, bound, authorization) are derived; this file APPLIES only
// a resolution whose states are ALL already proven — removing exactly the
// exact-current claim witnesses it names and rerunning the identical domain
// solver over the remainder — while RECORDING every applicable resolution,
// applied or not, with its typed states intact (§10). It never interprets
// dates or principals, and it never edits row in place — every path returns
// a fresh value built from row's own fields.
package policyconflict

import (
	"fmt"
	"reflect"
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
// A resolution not all-proven (allProven) is rejected: it is never applied,
// and it is never treated as an operational caller defect either (an
// authority shortfall is routine three-valued-honesty territory, not a
// malformed call). It is still RETAINED on the returned row: authority
// design §10 has each row carry its applicable exemption resolutions with
// their typed match/freshness/scope/bound/authorization states, so a
// rejected resolution stays visible with the exact substate that rejected
// it instead of vanishing. Its ineffectiveness is additionally named by
// ReasonExemptionIneffective — including in the mixed case where another
// resolution does cover the conflict, so a row never silently loses the
// fact that an applicable exemption had no effect. Both reason codes may
// therefore appear on one row; each names a real outcome of that row's
// exemption application, and the wire's reason rule (a sorted-unique set
// from the closed vocabulary) admits them together.
//
// An accepted resolution's RemovedClaims must each name a claim digest
// exactly present among row.Claims (its "match" state already being proven
// is what makes this an operational contract, not a routine shortfall: a
// digest absent here means the resolution was constructed against a
// different row's witnesses).
func ApplyEffectiveExemptions(row MechanicalEvaluation, resolutions []ExemptionResolution) (MechanicalEvaluation, error) {
	if err := validateRowOperand(row); err != nil {
		return MechanicalEvaluation{}, err
	}
	applicable, err := dedupeResolutions(resolutions)
	if err != nil {
		return MechanicalEvaluation{}, err
	}

	accepted := make([]ExemptionResolution, 0, len(applicable))
	anyRejected := false
	for _, r := range applicable {
		if allProven(r.Resolution) {
			accepted = append(accepted, r)
		} else {
			anyRejected = true
		}
	}

	out := row
	out.Exemptions = applicable
	out.Reasons = append([]ReasonCode{}, row.Reasons...)

	if len(accepted) == 0 {
		out.After = row.Before
		if anyRejected && row.State != ProofProven {
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
		afterProof = rerunPrincipalRelationWithoutKernel(remainder)
	} else {
		var err error
		afterProof, err = rerunSolver(row.Domain, remainder)
		if err != nil {
			return MechanicalEvaluation{}, err
		}
	}
	out.After = afterProof

	// A row that is ALREADY proven — a satisfiable group, or one proven
	// harmless by disjoint scope — never was a mechanical conflict, and
	// §5.5's coverage rule is about covering a conflict. Applying an
	// exemption to it therefore records the resolutions and the recomputed
	// post-exemption proof without crediting (or blaming) any exemption for
	// a state the row already held on its own evidence.
	if row.State == ProofProven {
		return out, nil
	}

	switch afterProof.State {
	case SolverSatisfiable:
		// §5.5: "A mechanical conflict is covered only when the
		// post-exemption conjunction is satisfiable or disjoint."
		out.State = ProofProven
		out.Reasons = []ReasonCode{ReasonExemptionEffective}
		if anyRejected {
			out.Reasons = addReason(out.Reasons, ReasonExemptionIneffective)
		}
	case SolverUnproven:
		// The departure dissolved the original proof without producing a
		// new one (see rerunPrincipalRelationWithoutKernel). The row's
		// pre-exemption reason described a proof that no longer holds, so
		// it is replaced by the outcome that does.
		out.State = ProofUnproven
		out.Reasons = addReason([]ReasonCode{ReasonPrincipalRelationUnproven}, ReasonExemptionIneffective)
	default:
		out.Reasons = addReason(out.Reasons, ReasonExemptionIneffective)
	}
	return out, nil
}

// validateRowOperand mirrors EvaluateMechanical's own entry validation over
// the row this call reruns a solver across: a hand-built or mutated row
// whose claims never passed Claim.Validate is a caller defect reported as
// an operational error, never a panic inside a solver that trusts its
// operand grammar.
func validateRowOperand(row MechanicalEvaluation) error {
	if row.Domain == "" {
		return fmt.Errorf("policyconflict: apply exemptions: row %q carries no operand domain", row.ID)
	}
	for i, c := range row.Claims {
		if c.PolicyID == "" || c.PolicyDigest == "" || c.ClaimDigest == "" {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d]: policy id, policy digest, and claim digest are all required", row.ID, i)
		}
		if err := c.Claim.Validate(); err != nil {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d] %s: %w", row.ID, i, c.ClaimDigest, err)
		}
	}
	return nil
}

// dedupeResolutions returns resolutions in ascending ID order with
// identical duplicates collapsed, satisfying the wire's sorted-unique
// exemption identity rule (validate.go's requireSortedUnique over
// ExemptionResolution.ID). Two DIFFERENT resolutions sharing one id are
// contradictory operands — exactly the kernel's own treatment of
// conflicting duplicate records — so they are an operational error rather
// than a silent pick between two authorities.
func dedupeResolutions(resolutions []ExemptionResolution) ([]ExemptionResolution, error) {
	byID := make(map[string]ExemptionResolution, len(resolutions))
	out := make([]ExemptionResolution, 0, len(resolutions))
	for _, r := range resolutions {
		prev, ok := byID[r.ID]
		if ok {
			if !reflect.DeepEqual(prev, r) {
				return nil, fmt.Errorf("policyconflict: apply exemptions: two different resolutions share exemption id %q", r.ID)
			}
			continue
		}
		byID[r.ID] = r
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
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
// exemption rerun: §5.5 requires that "the same solver runs again", so the
// remainder is re-solved rather than having the original proof repeated.
// ApplyEffectiveExemptions' fixed signature carries no context, profile, or
// actors (the exact API contract this package implements), so it cannot
// construct a fresh kernel authorization request the way
// solvePrincipalRelation does — but §5.3 splits that solver into a
// kernel-free half and a kernel half, and only the kernel half is out of
// reach here:
//
//   - both relation operators still present: requiring same-principal and
//     different-principal for one transition and role pair is a textual
//     contradiction §5.3 proves without any kernel call — unsatisfiable;
//   - exactly one relation still required: provable only by the kernel,
//     which is unavailable, so the departure leaves the requirement
//     unproven — never the removed contradiction repeated (that proof no
//     longer holds) and never a manufactured pass;
//   - no relation left: nothing requires a relation at all — satisfiable.
func rerunPrincipalRelationWithoutKernel(remainder []TypedClaimRecord) SolverProof {
	hasSame, hasDiff := false, false
	roles := map[string]bool{}
	for _, c := range remainder {
		switch c.Claim.Operator {
		case policyartifact.OpSamePrincipal:
			hasSame = true
		case policyartifact.OpDifferentPrincipal:
			hasDiff = true
		default:
			continue
		}
		for _, v := range c.Claim.Values {
			roles[v] = true
		}
	}
	values := sortedKeysOf(roles)

	switch {
	case hasSame && hasDiff:
		return SolverProof{State: SolverUnsatisfiable, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: values}
	case hasSame || hasDiff:
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}
	default:
		return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}
	}
}
