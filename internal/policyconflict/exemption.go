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
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// ExemptionApplication carries the one mechanical row being recomputed and
// the exact sealed kernel operands Service already used for its initial solve.
type ExemptionApplication struct {
	Row         MechanicalEvaluation
	Resolutions []ExemptionResolution
	Profile     governanceprincipal.Profile
	Actors      []governanceprincipal.PrincipalResolution
}

// ExemptionApplicationResult is one recomputed row plus any translated kernel
// disclosures produced only by its post-exemption principal-relation solve.
type ExemptionApplicationResult struct {
	Evaluation  MechanicalEvaluation
	Disclosures []Disclosure
}

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
// An accepted resolution's RemovedClaims must each name a claim EXACTLY
// present among row.Claims — same composite (policy_id, claim_id) identity
// AND same claim digest (its "match" state already being proven is what
// makes this an operational contract, not a routine shortfall: an identity
// absent here means the resolution was constructed against a different
// row's witnesses, and a digest mismatch means the claim changed under it).
func ApplyEffectiveExemptions(ctx context.Context, in ExemptionApplication) (ExemptionApplicationResult, error) {
	if err := ctx.Err(); err != nil {
		return ExemptionApplicationResult{}, fmt.Errorf("policyconflict: apply exemptions: %w", err)
	}
	row, resolutions := in.Row, in.Resolutions
	if err := validateRowOperand(row); err != nil {
		return ExemptionApplicationResult{}, err
	}
	applicable, err := dedupeResolutions(resolutions)
	if err != nil {
		return ExemptionApplicationResult{}, err
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
		return ExemptionApplicationResult{Evaluation: out, Disclosures: []Disclosure{}}, nil
	}

	removed := map[claimIdentity]bool{}
	for _, r := range accepted {
		for _, w := range r.RemovedClaims {
			id := claimIdentity{policyID: w.PolicyID, claimID: w.ClaimID}
			found := false
			for _, c := range row.Claims {
				if identityOf(c) != id {
					continue
				}
				// Authority design §5.5: "their digest must match that exact
				// current row claim". A stale digest is a claim that changed
				// under the exemption, never a silent widening.
				if c.ClaimDigest != w.ClaimDigest {
					return ExemptionApplicationResult{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q names claim (policy %q, claim %q) with digest %s, but row %q's current claim digests to %s", r.ID, w.PolicyID, w.ClaimID, w.ClaimDigest, row.ID, c.ClaimDigest)
				}
				found = true
				break
			}
			if !found {
				return ExemptionApplicationResult{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q names claim (policy %q, claim %q), absent from row %q's current claims", r.ID, w.PolicyID, w.ClaimID, row.ID)
			}
			removed[id] = true
		}
	}

	remainder := make([]TypedClaimRecord, 0, len(row.Claims))
	for _, c := range row.Claims {
		if !removed[identityOf(c)] {
			remainder = append(remainder, c)
		}
	}

	afterProof, disclosures, err := rerunSolver(row, remainder, in.Profile, in.Actors)
	if err != nil {
		return ExemptionApplicationResult{}, err
	}
	out.After = afterProof

	// A row that is ALREADY proven — a satisfiable group, or one proven
	// harmless by disjoint scope — never was a mechanical conflict, and
	// §5.5's coverage rule is about covering a conflict. Applying an
	// exemption to it therefore records the resolutions and the recomputed
	// post-exemption proof without crediting (or blaming) any exemption for
	// a state the row already held on its own evidence.
	if row.State == ProofProven {
		return ExemptionApplicationResult{Evaluation: out, Disclosures: disclosures}, nil
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
		// new one. The row's
		// pre-exemption reason described a proof that no longer holds, so
		// it is replaced by the outcome that does.
		out.State = ProofUnproven
		out.Reasons = addReason([]ReasonCode{unprovenReasonFor(afterProof)}, ReasonExemptionIneffective)
	case SolverUnsatisfiable:
		// The report carries only the original scope proof. An
		// unsatisfiable remainder therefore cannot promote an originally
		// unproven scope conclusion to a witnessed violation; retain the
		// row's proved state/reasons and record only that the exemption did
		// not cover it.
		out.State = row.State
		out.Reasons = addReason(row.Reasons, ReasonExemptionIneffective)
	}
	return ExemptionApplicationResult{Evaluation: out, Disclosures: disclosures}, nil
}

// claimIdentity is a row claim's composite identity (ledger SI-105): the
// governing policy plus the claim id. Byte-identical claims from two
// policies are two identities, so an exemption departs from one of them
// without touching the other.
type claimIdentity struct {
	policyID string
	claimID  string
}

func identityOf(c TypedClaimRecord) claimIdentity {
	return claimIdentity{policyID: c.PolicyID, claimID: c.Claim.ID}
}

// validateRowOperand mirrors EvaluateMechanical's own entry validation over
// the row this call reruns a solver across: a hand-built or mutated row
// whose claims never passed Claim.Validate — or whose carried claim digest
// does not recompute from the claim it carries — is a caller defect
// reported as an operational error, never a panic inside a solver that
// trusts its operand grammar and never a departure addressed at content
// that no longer exists.
func validateRowOperand(row MechanicalEvaluation) error {
	if row.Domain == "" {
		return fmt.Errorf("policyconflict: apply exemptions: row %q carries no operand domain", row.ID)
	}
	seen := make(map[claimIdentity]bool, len(row.Claims))
	for i, c := range row.Claims {
		if c.PolicyID == "" || c.PolicyDigest == "" || c.ClaimDigest == "" {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d]: policy id, policy digest, and claim digest are all required", row.ID, i)
		}
		if err := c.Claim.Validate(); err != nil {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d] %s: %w", row.ID, i, c.ClaimDigest, err)
		}
		recomputed, err := policyartifact.ClaimDigest(c.Claim)
		if err != nil {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d]: digest claim: %w", row.ID, i, err)
		}
		if recomputed != c.ClaimDigest {
			return fmt.Errorf("policyconflict: apply exemptions: row %q claim [%d] (policy %q claim %q): carried claim digest %s is not the canonical digest of its claim (%s)", row.ID, i, c.PolicyID, c.Claim.ID, c.ClaimDigest, recomputed)
		}
		id := identityOf(c)
		if seen[id] {
			return fmt.Errorf("policyconflict: apply exemptions: row %q carries two claims for identity (policy %q, claim %q)", row.ID, c.PolicyID, c.Claim.ID)
		}
		seen[id] = true
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
//
// Each resolution's removal set is normalized first (normalizeRemovals), so
// the identity comparison below is over canonical content and every
// resolution this function returns already satisfies the wire's own
// composite ordering and cardinality rules.
func dedupeResolutions(resolutions []ExemptionResolution) ([]ExemptionResolution, error) {
	byID := make(map[string]ExemptionResolution, len(resolutions))
	out := make([]ExemptionResolution, 0, len(resolutions))
	for _, raw := range resolutions {
		r, err := normalizeRemovals(raw)
		if err != nil {
			return nil, err
		}
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

// normalizeRemovals returns r with its removal set canonicalized, and
// enforces authority design §5.5's mandatory-present, conditional-
// cardinality contract:
//
//   - the set is never absent — a nil set is a caller defect, because §5.5
//     requires the explicit empty set rather than silence;
//   - an all-five-proven resolution names at least one witness, and a
//     resolution that is not all-proven names none, because it removed
//     nothing and must not record a departure it never made;
//   - witnesses sort and deduplicate by their composite (policy_id,
//     claim_id) identity. One identity carrying two DIFFERENT digests is
//     contradictory authority, not a duplicate to collapse.
func normalizeRemovals(r ExemptionResolution) (ExemptionResolution, error) {
	if r.RemovedClaims == nil {
		return ExemptionResolution{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q: removed claims must be present (an explicitly empty set is [])", r.ID)
	}
	switch {
	case allProven(r.Resolution) && len(r.RemovedClaims) == 0:
		return ExemptionResolution{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q: an all-proven resolution must name at least one removed claim", r.ID)
	case !allProven(r.Resolution) && len(r.RemovedClaims) > 0:
		return ExemptionResolution{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q: a resolution that is not all-proven removed nothing and must name the explicit empty removal set, got %d", r.ID, len(r.RemovedClaims))
	}

	byIdentity := make(map[claimIdentity]MechanicalClaimWitness, len(r.RemovedClaims))
	witnesses := make([]MechanicalClaimWitness, 0, len(r.RemovedClaims))
	for _, w := range r.RemovedClaims {
		id := claimIdentity{policyID: w.PolicyID, claimID: w.ClaimID}
		if prev, ok := byIdentity[id]; ok {
			if prev != w {
				return ExemptionResolution{}, fmt.Errorf("policyconflict: apply exemptions: exemption %q names claim (policy %q, claim %q) with two different digests", r.ID, w.PolicyID, w.ClaimID)
			}
			continue
		}
		byIdentity[id] = w
		witnesses = append(witnesses, w)
	}
	sort.Slice(witnesses, func(i, j int) bool {
		if witnesses[i].PolicyID != witnesses[j].PolicyID {
			return witnesses[i].PolicyID < witnesses[j].PolicyID
		}
		return witnesses[i].ClaimID < witnesses[j].ClaimID
	})
	r.RemovedClaims = witnesses
	return r, nil
}

// rerunSolver dispatches through mechanical.go's single domain-solver seam,
// including the principal-relation kernel path with the same sealed profile
// and authenticated actors used by the initial solve.
func rerunSolver(row MechanicalEvaluation, remainder []TypedClaimRecord, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) (SolverProof, []Disclosure, error) {
	group := groupKeyFor(row.Claims[0].Claim)
	claims := claimsFromRecords(remainder)
	proof, kernelDisclosures, err := solveGroup(row.Domain, group, claims, profile, actors)
	if err != nil {
		return SolverProof{}, nil, err
	}
	disclosures, err := translateKernelDisclosures(kernelDisclosures)
	if err != nil {
		return SolverProof{}, nil, err
	}
	return proof, disclosures, nil
}

func claimsFromRecords(records []TypedClaimRecord) []policyartifact.Claim {
	claims := make([]policyartifact.Claim, len(records))
	for i, c := range records {
		claims[i] = c.Claim
	}
	return claims
}
