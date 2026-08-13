// mechanical.go proves typed mechanical conflicts (authority design §5,
// ledger SI-95): claims interact only inside one (family, subject) group,
// with an identity group additionally keyed on its canonical unordered role
// pair, and every group uses exactly one of four closed operand domains — a
// group whose claims mix operators from more than one domain is invalid
// authority and fails operationally rather than choosing a meaning.
//
// Each domain solver (solveDiscrete, solveInterval, solvePrincipalRelation,
// solvePathCapability) first proves the group's COMPLETE conjunction without
// scope: a satisfiable complete conjunction proves every scoped subset
// satisfiable (one row, ReasonMechanicalSatisfiable). For an unsatisfiable
// complete conjunction, the three-step witness procedure runs: every
// exact-scope subgroup is solved as its own N-claim conjunction; every
// unique differently-scoped claim pair is solved once; each witness that is
// itself unsatisfiable AND whose own scope proof (Task 5's CompareScopes/
// IntersectScopes) proves overlap becomes its own blocked-violated row.
// Proven-disjoint and scope-unknown witnesses never manufacture a
// blocked-violated row of their own.
//
// When neither step produces an overlap witness, step 3 asks whether proven
// disjointness alone settles the group (disjointnessSettlesGroup): when
// every claim subset that can co-apply at all is satisfiable, §5's
// "Proven-disjoint witnesses do not conflict" is the complete answer and
// the group is one proven ReasonScopeDisjoint row. Otherwise — any unknown
// scope, or a co-applicable subset left unresolved — the remaining
// higher-order case is one blocked-unproven row carrying the complete group
// (never a false pass, never a false conflict).
//
// solvePrincipalRelation additionally has ONE domain-direct outcome no other
// solver produces: SolverUnproven, emitted immediately (bypassing the scope
// witness procedure entirely — no amount of scope refinement can manufacture
// kernel evidence that was never supplied) whenever the profile carries no
// matching governanceprincipal.DistinctnessRule for the claim's exact
// (transition, canonical role pair, relation), or whenever the kernel's own
// authorization decision is unproven. A kernel decision that is violated
// for ANY reason is likewise never a proof (§5.3). This mirrors
// ReasonPrincipalRelationUnproven and ReasonPrincipalRelationViolated in
// the closed reason vocabulary.
package policyconflict

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// The four closed mechanical operand domains (authority design §5.1-§5.4).
const (
	domainDiscreteSet       = "discrete-set"
	domainIntegerInterval   = "integer-interval"
	domainPrincipalRelation = "principal-relation"
	domainPathCapability    = "path-capability"
)

// operatorDomain is the closed operator-to-domain map. An operator absent
// from this map cannot occur on a policyartifact.Claim that already passed
// Claim.Validate — its presence here is purely so a mixed-domain group is
// detected by table lookup rather than a chain of per-operator cases.
var operatorDomain = map[policyartifact.Operator]string{
	policyartifact.OpEquals:             domainDiscreteSet,
	policyartifact.OpNotEquals:          domainDiscreteSet,
	policyartifact.OpAllowedValues:      domainDiscreteSet,
	policyartifact.OpRequiredValues:     domainDiscreteSet,
	policyartifact.OpForbiddenValues:    domainDiscreteSet,
	policyartifact.OpMinimum:            domainIntegerInterval,
	policyartifact.OpMaximum:            domainIntegerInterval,
	policyartifact.OpSamePrincipal:      domainPrincipalRelation,
	policyartifact.OpDifferentPrincipal: domainPrincipalRelation,
	policyartifact.OpPathRead:           domainPathCapability,
	policyartifact.OpPathWrite:          domainPathCapability,
}

// MechanicalInput is EvaluateMechanical's complete operand set (authority
// design §5): Task 4's typed claims, the governing profile and authenticated
// actor facts the principal-relation domain's kernel call consumes, and
// Task 5's scope resolver.
type MechanicalInput struct {
	Claims  []contextcompile.TypedClaim
	Profile governanceprincipal.Profile
	Actors  []governanceprincipal.PrincipalResolution
	Refs    RefRelationResolver
}

// mechanicalGroupKey is one (family, subject[, canonical role pair]) group
// identity (authority design §5). RoleA/RoleB are empty outside the
// identity family.
type mechanicalGroupKey struct {
	family       policyartifact.Family
	subject      string
	roleA, roleB string
}

// id is the group's deterministic, content-derived string identity: the
// prefix every row this group emits shares.
//
// The concatenation is collision-free by the operand grammar this package
// already enforces at entry, not by luck: EvaluateMechanical runs
// Claim.Validate on every claim, which requires family, subject, and
// identity role values to be kebab-case
// (policyartifact's `^[a-z0-9]+(?:-[a-z0-9]+)*$`), so none of them can
// contain ":", "+", "#", or ",". A row suffix (#complete, #scope:<digest>,
// #pair:<digest>,<digest>) starts at the one "#" the whole ID can contain,
// and claim/scope digests are sha256:<64 lowercase hex> with no "," in
// them, so every ID parses back to exactly one group and suffix.
func (k mechanicalGroupKey) id() string {
	if k.family == policyartifact.FamilyIdentity {
		return fmt.Sprintf("%s:%s:%s+%s", k.family, k.subject, k.roleA, k.roleB)
	}
	return fmt.Sprintf("%s:%s", k.family, k.subject)
}

// groupKeyFor derives c's group key. For the identity family the two role
// values normalize lexically (authority design §5.3: "The two roles are a
// semantic set and normalize lexically") so a reversed role spelling groups
// identically; Claim.Validate already guarantees exactly two distinct role
// values for identity-family claims.
func groupKeyFor(c policyartifact.Claim) mechanicalGroupKey {
	if c.Family == policyartifact.FamilyIdentity {
		roles := append([]string(nil), c.Values...)
		sort.Strings(roles)
		return mechanicalGroupKey{family: c.Family, subject: c.Subject, roleA: roles[0], roleB: roles[1]}
	}
	return mechanicalGroupKey{family: c.Family, subject: c.Subject}
}

// EvaluateMechanical proves every typed claim group's mechanical
// satisfiability (authority design §5). It groups in.Claims, solves each
// group's complete conjunction first, and — for an unsatisfiable complete
// conjunction — derives the deterministic scope-witness rows the package
// comment above describes. Rows are always returned in ascending ID order;
// identical inputs deep-equal outputs.
func EvaluateMechanical(ctx context.Context, in MechanicalInput) ([]MechanicalEvaluation, error) {
	for i, c := range in.Claims {
		if c.PolicyID == "" || c.PolicyDigest == "" || c.ClaimDigest == "" {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d]: policy id, policy digest, and claim digest are all required", i)
		}
		if err := c.Claim.Validate(); err != nil {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d] %s: %w", i, c.ClaimDigest, err)
		}
	}

	ordered := append([]contextcompile.TypedClaim(nil), in.Claims...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ClaimDigest < ordered[j].ClaimDigest })

	var order []mechanicalGroupKey
	buckets := make(map[mechanicalGroupKey][]contextcompile.TypedClaim)
	for _, c := range ordered {
		gk := groupKeyFor(c.Claim)
		if _, ok := buckets[gk]; !ok {
			order = append(order, gk)
		}
		buckets[gk] = append(buckets[gk], c)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].id() < order[j].id() })

	rows := []MechanicalEvaluation{}
	for _, gk := range order {
		grows, err := evaluateGroup(ctx, gk, buckets[gk], in.Profile, in.Actors, in.Refs)
		if err != nil {
			return nil, err
		}
		rows = append(rows, grows...)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows, nil
}

// determineDomain returns members' single shared operand domain, or an
// operational error when the group mixes operators from more than one
// domain (authority design §5: "invalid authority ... fails operationally").
func determineDomain(members []contextcompile.TypedClaim) (string, error) {
	var found string
	for _, m := range members {
		d, ok := operatorDomain[m.Claim.Operator]
		if !ok {
			return "", fmt.Errorf("policyconflict: claim %s: operator %q has no recognized mechanical domain", m.ClaimDigest, m.Claim.Operator)
		}
		if found == "" {
			found = d
			continue
		}
		if found != d {
			return "", fmt.Errorf("policyconflict: claim group (family=%s subject=%s): mixes operand domains %q and %q, which is invalid authority", m.Claim.Family, m.Claim.Subject, found, d)
		}
	}
	return found, nil
}

func claimsOf(members []contextcompile.TypedClaim) []policyartifact.Claim {
	out := make([]policyartifact.Claim, len(members))
	for i, m := range members {
		out[i] = m.Claim
	}
	return out
}

// solveGroup dispatches to the one domain solver claims' shared domain
// names. This is the package's single dispatch point — never a per-operator
// or per-operator-pair table.
func solveGroup(domain string, gk mechanicalGroupKey, claims []policyartifact.Claim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) (SolverProof, error) {
	switch domain {
	case domainDiscreteSet:
		return solveDiscrete(claims), nil
	case domainIntegerInterval:
		return solveInterval(claims)
	case domainPrincipalRelation:
		return solvePrincipalRelation(gk.subject, gk.roleA, gk.roleB, claims, profile, actors)
	case domainPathCapability:
		return solvePathCapability(claims), nil
	default:
		return SolverProof{}, fmt.Errorf("policyconflict: unknown mechanical domain %q", domain)
	}
}

func scopeProofFor(ctx context.Context, members []contextcompile.TypedClaim, refs RefRelationResolver) (ScopeProof, error) {
	scopes := make([]policyartifact.Scope, len(members))
	for i, m := range members {
		scopes[i] = m.Claim.Scope
	}
	return IntersectScopes(ctx, scopes, refs)
}

func evaluateGroup(ctx context.Context, gk mechanicalGroupKey, members []contextcompile.TypedClaim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) ([]MechanicalEvaluation, error) {
	domain, err := determineDomain(members)
	if err != nil {
		return nil, err
	}

	before, err := solveGroup(domain, gk, claimsOf(members), profile, actors)
	if err != nil {
		return nil, err
	}

	switch before.State {
	case SolverSatisfiable:
		scope, err := scopeProofFor(ctx, members, refs)
		if err != nil {
			return nil, err
		}
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofProven, []ReasonCode{ReasonMechanicalSatisfiable})}, nil
	case SolverUnproven:
		scope, err := scopeProofFor(ctx, members, refs)
		if err != nil {
			return nil, err
		}
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofUnproven, []ReasonCode{ReasonPrincipalRelationUnproven})}, nil
	case SolverUnsatisfiable:
		return deriveWitnessRows(ctx, gk, members, domain, before, profile, actors, refs)
	default:
		return nil, fmt.Errorf("policyconflict: group %s: solver returned unknown state %q", gk.id(), before.State)
	}
}

// deriveWitnessRows implements authority design §5's three-step procedure
// for an unsatisfiable complete conjunction.
func deriveWitnessRows(ctx context.Context, gk mechanicalGroupKey, members []contextcompile.TypedClaim, domain string, before SolverProof, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) ([]MechanicalEvaluation, error) {
	scopeDigests := make([]string, len(members))
	for i, m := range members {
		d, err := canonjson.Digest(m.Claim.Scope)
		if err != nil {
			return nil, fmt.Errorf("policyconflict: digest claim scope: %w", err)
		}
		scopeDigests[i] = d
	}

	var subgroupOrder []string
	subgroups := make(map[string][]int)
	for i, d := range scopeDigests {
		if _, ok := subgroups[d]; !ok {
			subgroupOrder = append(subgroupOrder, d)
		}
		subgroups[d] = append(subgroups[d], i)
	}

	var violated []MechanicalEvaluation

	// Step 1: every exact-scope subgroup, solved as its own N-claim
	// conjunction.
	for _, d := range subgroupOrder {
		idxs := subgroups[d]
		subMembers := make([]contextcompile.TypedClaim, len(idxs))
		for i, idx := range idxs {
			subMembers[i] = members[idx]
		}
		subBefore, err := solveGroup(domain, gk, claimsOf(subMembers), profile, actors)
		if err != nil {
			return nil, err
		}
		if subBefore.State != SolverUnsatisfiable {
			continue
		}
		subScope, err := scopeProofFor(ctx, subMembers, refs)
		if err != nil {
			return nil, err
		}
		if subScope.State != ScopeOverlap {
			continue
		}
		id := gk.id() + "#scope:" + d
		violated = append(violated, buildRow(id, gk, subMembers, subScope, domain, subBefore, ProofViolatedWithWitness, []ReasonCode{unsatReasonFor(domain, claimsOf(subMembers))}))
	}

	// Step 2: every unique differently-scoped claim pair, solved once.
	for i := 0; i < len(members); i++ {
		for j := i + 1; j < len(members); j++ {
			if scopeDigests[i] == scopeDigests[j] {
				continue
			}
			pairMembers := []contextcompile.TypedClaim{members[i], members[j]}
			pairBefore, err := solveGroup(domain, gk, claimsOf(pairMembers), profile, actors)
			if err != nil {
				return nil, err
			}
			if pairBefore.State != SolverUnsatisfiable {
				continue
			}
			pairScope, err := scopeProofFor(ctx, pairMembers, refs)
			if err != nil {
				return nil, err
			}
			if pairScope.State != ScopeOverlap {
				continue
			}
			id := gk.id() + "#pair:" + members[i].ClaimDigest + "," + members[j].ClaimDigest
			violated = append(violated, buildRow(id, gk, pairMembers, pairScope, domain, pairBefore, ProofViolatedWithWitness, []ReasonCode{unsatReasonFor(domain, claimsOf(pairMembers))}))
		}
	}

	if len(violated) > 0 {
		sort.Slice(violated, func(a, b int) bool { return violated[a].ID < violated[b].ID })
		return violated, nil
	}

	// Step 3: neither step proved an overlap witness. Two cases remain, and
	// they are not the same result (never a proven-disjoint or scope-unknown
	// witness upgraded into a conflict, and never silently dropped either).
	scope, err := scopeProofFor(ctx, members, refs)
	if err != nil {
		return nil, err
	}
	settled, err := disjointnessSettlesGroup(ctx, members, domain, gk, profile, actors, refs)
	if err != nil {
		return nil, err
	}
	if settled && scope.State == ScopeDisjoint {
		// Authority design §5: "Proven-disjoint witnesses do not conflict."
		// Nothing REMAINS for the higher-order case here, so this is a
		// proven row, not a withheld conclusion.
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofProven, []ReasonCode{ReasonScopeDisjoint})}, nil
	}
	// The genuine residual: some co-applicable subset could still be
	// unsatisfiable (or its scope is unknown), so one blocked-unproven row
	// carries the complete claim witness.
	return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofUnproven, []ReasonCode{ReasonHigherOrderScopeUnproven})}, nil
}

// disjointnessSettlesGroup reports whether proven scope disjointness alone
// settles an unsatisfiable group that steps 1-2 produced no overlap witness
// for — i.e. whether NO co-applicable claim subset can be unsatisfiable.
//
// Why exhaustive PAIR disjointness covers N>=3 co-application. A subset of
// claims can be co-applicable only if its N-way scope intersection is
// nonempty (§4.4's product rule), and an N-way intersection is contained in
// every one of its member pairs' intersections. So one proven-disjoint pair
// inside a subset empties every higher-order intersection that contains
// both members: a co-applicable subset can never straddle a proven-disjoint
// pair. Build the graph whose edges are exactly the claim pairs NOT proven
// disjoint (overlap AND unknown are both edges — unknown is never assumed
// disjoint) and every co-applicable subset is a clique in it, hence lies
// wholly inside one connected component. Conjunction is monotone, so if a
// component's own complete conjunction is satisfiable, every subset of it
// is too. When that holds for every component, no co-applicable
// unsatisfiable subset exists at any arity and the group's unsatisfiability
// is entirely an artifact of combining claims that never co-apply.
//
// The converse is deliberately weak: one unsatisfiable component (or one
// unknown-scope edge holding claims together) leaves the group unproven
// rather than violated, because steps 1-2 already failed to exhibit the
// proven-overlap witness §5 requires for blocked-violated.
func disjointnessSettlesGroup(ctx context.Context, members []contextcompile.TypedClaim, domain string, gk mechanicalGroupKey, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) (bool, error) {
	n := len(members)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			pairScope, err := scopeProofFor(ctx, []contextcompile.TypedClaim{members[i], members[j]}, refs)
			if err != nil {
				return false, err
			}
			if pairScope.State == ScopeDisjoint {
				continue
			}
			ri, rj := find(i), find(j)
			if ri != rj {
				parent[ri] = rj
			}
		}
	}

	componentOrder := make([]int, 0, n)
	components := make(map[int][]contextcompile.TypedClaim, n)
	for i := 0; i < n; i++ {
		root := find(i)
		if _, ok := components[root]; !ok {
			componentOrder = append(componentOrder, root)
		}
		components[root] = append(components[root], members[i])
	}
	if len(componentOrder) < 2 {
		// One component means no proven-disjoint pair separated anything:
		// the group's own unsatisfiability stands unexplained by scope.
		return false, nil
	}
	for _, root := range componentOrder {
		proof, err := solveGroup(domain, gk, claimsOf(components[root]), profile, actors)
		if err != nil {
			return false, err
		}
		if proof.State != SolverSatisfiable {
			return false, nil
		}
	}
	return true, nil
}

// unsatReasonFor names an unsatisfiable witness row's outcome from the
// domain that proved it (ledger SI-103: one stable consumer label per
// outcome). The identity domain has two distinct unsatisfiable outcomes:
// requiring both relations for one transition and role pair is what
// authority design §5.3 itself calls "a mechanical conflict" — a textual
// contradiction proved without the kernel — while a single required
// relation is unsatisfiable only because the KERNEL returned a violated
// authorization, which is principal-relation-violated. Every other domain
// has exactly one unsatisfiable outcome.
func unsatReasonFor(domain string, claims []policyartifact.Claim) ReasonCode {
	if domain != domainPrincipalRelation {
		return ReasonMechanicalConflict
	}
	hasSame, hasDiff := false, false
	for _, c := range claims {
		switch c.Operator {
		case policyartifact.OpSamePrincipal:
			hasSame = true
		case policyartifact.OpDifferentPrincipal:
			hasDiff = true
		}
	}
	if hasSame && hasDiff {
		return ReasonMechanicalConflict
	}
	return ReasonPrincipalRelationViolated
}

func buildRow(id string, gk mechanicalGroupKey, members []contextcompile.TypedClaim, scope ScopeProof, domain string, proof SolverProof, state ProofState, reasons []ReasonCode) MechanicalEvaluation {
	records := collapseClaimRecords(members)

	sortedReasons := append([]ReasonCode(nil), reasons...)
	sort.Slice(sortedReasons, func(i, j int) bool { return sortedReasons[i] < sortedReasons[j] })

	return MechanicalEvaluation{
		ID:         id,
		Family:     gk.family,
		Subject:    gk.subject,
		Claims:     records,
		Scope:      scope,
		Domain:     domain,
		Before:     proof,
		Exemptions: []ExemptionResolution{},
		After:      proof,
		State:      state,
		Reasons:    sortedReasons,
	}
}

// collapseClaimRecords turns members into the row's sorted claim records
// under the frozen wire's OWN row-claim identity rule: validate.go keys a
// row's claims on the claim digest alone (requireSortedUnique over
// TypedClaimRecord.ClaimDigest), so one claim digest is exactly one row
// identity. Two policies that declare a byte-identical claim therefore
// contribute one identity — and one solver constraint, which a conjunction
// of identical claims already is — recorded once, attributed to the lowest
// (policy id, policy digest) so the row is deterministic. Emitting one
// record per contributing policy instead would make every such row fail the
// wire's duplicate-identity check outright.
//
// Consequence, disclosed rather than hidden: TypedClaimRecord carries
// exactly one policy identity, so the report cannot show that a second
// policy declared the same claim. Which policies an exemption must be
// authorized against for such a shared claim is therefore not decidable
// from this row; it belongs to the exemption's own match/authorization
// derivation (Task 8), which evaluates against the complete typed-claim set
// where both policy identities are still present. Recorded for the
// invention ledger as a wire tension, not resolved silently here.
func collapseClaimRecords(members []contextcompile.TypedClaim) []TypedClaimRecord {
	byDigest := make(map[string]TypedClaimRecord, len(members))
	for _, m := range members {
		rec := TypedClaimRecord{PolicyID: m.PolicyID, PolicyDigest: m.PolicyDigest, ClaimDigest: m.ClaimDigest, Claim: m.Claim}
		prev, ok := byDigest[m.ClaimDigest]
		if !ok || rec.PolicyID < prev.PolicyID || (rec.PolicyID == prev.PolicyID && rec.PolicyDigest < prev.PolicyDigest) {
			byDigest[m.ClaimDigest] = rec
		}
	}
	records := make([]TypedClaimRecord, 0, len(byDigest))
	for _, rec := range byDigest {
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ClaimDigest < records[j].ClaimDigest })
	return records
}

// --- solveDiscrete (authority design §5.1) ----------------------------------

// solveDiscrete proves the discrete-set domain's complete conjunction.
// equals values must agree; allowed-values sets intersect; required-values
// and forbidden-values (plus every not-equals value) union. With an equals
// value, its singleton must satisfy every other bound. Without one, every
// required value must remain allowed (when an allowed bound exists) and
// unforbidden, and — when an allowed bound exists — at least one allowed,
// unforbidden value must remain. With no allowed bound, an open-domain
// symbolic witness proves satisfiability whenever nothing else already
// forces a forbidden exact value.
func solveDiscrete(claims []policyartifact.Claim) SolverProof {
	var equalsVals []string
	var allowedSets [][]string
	forbidden := map[string]bool{}
	required := map[string]bool{}
	hasAllowedBound := false

	for _, c := range claims {
		switch c.Operator {
		case policyartifact.OpEquals:
			v := c.Values[0]
			if !stringsContain(equalsVals, v) {
				equalsVals = append(equalsVals, v)
			}
		case policyartifact.OpNotEquals:
			forbidden[c.Values[0]] = true
		case policyartifact.OpAllowedValues:
			hasAllowedBound = true
			allowedSets = append(allowedSets, c.Values)
		case policyartifact.OpRequiredValues:
			for _, v := range c.Values {
				required[v] = true
			}
		case policyartifact.OpForbiddenValues:
			for _, v := range c.Values {
				forbidden[v] = true
			}
		}
	}
	sort.Strings(equalsVals)
	forbiddenList := sortedKeysOf(forbidden)
	requiredList := sortedKeysOf(required)

	var effectiveAllowed []string
	if hasAllowedBound {
		effectiveAllowed = sortedUniqueCopy(allowedSets[0])
		for _, s := range allowedSets[1:] {
			effectiveAllowed = intersectSortedStrings(effectiveAllowed, sortedUniqueCopy(s))
		}
	}

	if len(equalsVals) > 1 {
		return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: requiredList, Forbidden: forbiddenList, Witnesses: equalsVals}
	}
	if len(equalsVals) == 1 {
		v := equalsVals[0]
		if hasAllowedBound && !stringsContain(effectiveAllowed, v) {
			return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{v}, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{v}}
		}
		if forbidden[v] {
			return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{v}, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{v}}
		}
		for _, r := range requiredList {
			if r != v {
				return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{v}, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{r}}
			}
		}
		return SolverProof{State: SolverSatisfiable, Domain: domainDiscreteSet, Values: []string{v}, Required: requiredList, Forbidden: forbiddenList, OpenDomain: false, Witnesses: []string{v}}
	}

	for _, r := range requiredList {
		if forbidden[r] {
			return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{r}}
		}
		if hasAllowedBound && !stringsContain(effectiveAllowed, r) {
			return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: effectiveAllowed, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{r}}
		}
	}

	if hasAllowedBound {
		if len(requiredList) > 0 {
			return SolverProof{State: SolverSatisfiable, Domain: domainDiscreteSet, Values: effectiveAllowed, Required: requiredList, Forbidden: forbiddenList, Witnesses: requiredList}
		}
		var witness []string
		for _, v := range effectiveAllowed {
			if !forbidden[v] {
				witness = []string{v}
				break
			}
		}
		if witness == nil {
			return SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: effectiveAllowed, Required: requiredList, Forbidden: forbiddenList, Witnesses: []string{}}
		}
		return SolverProof{State: SolverSatisfiable, Domain: domainDiscreteSet, Values: effectiveAllowed, Required: requiredList, Forbidden: forbiddenList, Witnesses: witness}
	}

	if len(requiredList) > 0 {
		return SolverProof{State: SolverSatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: requiredList, Forbidden: forbiddenList, Witnesses: requiredList}
	}
	return SolverProof{State: SolverSatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: []string{}, Forbidden: forbiddenList, OpenDomain: true, Witnesses: []string{}}
}

// --- solveInterval (authority design §5.2) ----------------------------------

// solveInterval proves the integer-interval domain's complete conjunction:
// effective minimum is the max of every minimum bound, effective maximum is
// the min of every maximum bound, and the conjunction is unsatisfiable
// exactly when effective minimum exceeds effective maximum. A claim
// carrying no bound is a malformed operand this solver fails on rather than
// guessing at (decode already rejects it upstream; this is a defensive
// second gate).
func solveInterval(claims []policyartifact.Claim) (SolverProof, error) {
	var effMin, effMax *int
	for _, c := range claims {
		if c.Bound == nil {
			return SolverProof{}, fmt.Errorf("policyconflict: interval solver: claim %s operator %q carries no bound", c.ID, c.Operator)
		}
		switch c.Operator {
		case policyartifact.OpMinimum:
			if effMin == nil || *c.Bound > *effMin {
				v := *c.Bound
				effMin = &v
			}
		case policyartifact.OpMaximum:
			if effMax == nil || *c.Bound < *effMax {
				v := *c.Bound
				effMax = &v
			}
		default:
			return SolverProof{}, fmt.Errorf("policyconflict: interval solver: unexpected operator %q", c.Operator)
		}
	}

	proof := SolverProof{State: SolverSatisfiable, Domain: domainIntegerInterval, Values: []string{}, Required: []string{}, Forbidden: []string{}, Minimum: effMin, Maximum: effMax, Witnesses: []string{}}
	if effMin != nil && effMax != nil && *effMin > *effMax {
		proof.State = SolverUnsatisfiable
		// The exact incompatible bound pair is the witness. Its NUMERIC
		// order (minimum then maximum) is not its lexical order — minimum 8
		// with maximum 6 renders as "8" then "6" — and the wire's own
		// canonical-lexical-order rule (validate.go's
		// requireSortedUniqueStrings over a solver proof's witness set)
		// governs every recorded set, so the pair is emitted sorted-unique.
		// The bounds themselves stay individually addressable and unordered
		// in the already-typed Minimum/Maximum fields.
		proof.Witnesses = sortedUniqueCopy([]string{strconv.Itoa(*effMin), strconv.Itoa(*effMax)})
	}
	return proof, nil
}

// --- solvePathCapability (authority design §5.4) ----------------------------

// solvePathCapability proves the path-capability domain's complete
// conjunction. Authority design §5.4 unions SAME-KIND requirements ("Same-
// kind path requirements union") and lets read and write coexist, so this
// domain never manufactures a conflict (DC-5: a missing execution grant is
// an unmet requirement, not a conflict this solver reports).
//
// Values records the group's whole canonical requirement set. Because the
// two kinds union independently, the per-kind canonical sets are recorded
// too: each witness is its own operator-qualified path
// ("path-read:internal/"), so which access a value was required for
// survives the union instead of collapsing into one anonymous set. Both
// recorded sets stay inside the frozen SolverProof's existing fields.
func solvePathCapability(claims []policyartifact.Claim) SolverProof {
	values := map[string]bool{}
	perKind := map[string]bool{}
	for _, c := range claims {
		for _, v := range c.Values {
			values[v] = true
			perKind[string(c.Operator)+":"+v] = true
		}
	}
	return SolverProof{
		State: SolverSatisfiable, Domain: domainPathCapability,
		Values: sortedKeysOf(values), Required: []string{}, Forbidden: []string{},
		Witnesses: sortedKeysOf(perKind),
	}
}

// --- solvePrincipalRelation (authority design §5.3) -------------------------

// solvePrincipalRelation proves the identity domain's complete conjunction.
// Requiring both same-principal and different-principal for one transition
// and canonical role pair is a pure textual contradiction, provable without
// any kernel call. Requiring a single relation is provable only by
// constructing one kernel authorization request — the evaluator never
// compares principal strings; role-membership matching below uses only
// profile.RoleMappings' exported trust-source/subject data to decide which
// authenticated actors are even eligible to fill a role, and the kernel
// alone (governanceprincipal.Authorize) decides distinctness. A relation
// with no matching governanceprincipal.DistinctnessRule on the profile has
// no kernel evidence at all and is unproven rather than a silent pass.
func solvePrincipalRelation(transition, roleA, roleB string, claims []policyartifact.Claim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) (SolverProof, error) {
	hasSame, hasDiff := false, false
	for _, c := range claims {
		switch c.Operator {
		case policyartifact.OpSamePrincipal:
			hasSame = true
		case policyartifact.OpDifferentPrincipal:
			hasDiff = true
		default:
			return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: unexpected operator %q", c.Operator)
		}
	}

	if !hasSame && !hasDiff {
		return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}, nil
	}
	if hasSame && hasDiff {
		roles := sortedUniqueCopy([]string{roleA, roleB})
		return SolverProof{
			State: SolverUnsatisfiable, Domain: domainPrincipalRelation,
			Values: roles, Required: []string{}, Forbidden: []string{},
			Witnesses: roles,
		}, nil
	}

	relation := governanceprincipal.RelationSamePrincipal
	if hasDiff {
		relation = governanceprincipal.RelationDifferentPrincipal
	}

	if !stringsContain(profile.ApplicableTransitions, transition) {
		return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: transition %q is not registered in profile %q's applicable transitions", transition, profile.ID)
	}
	if !roleRegistered(profile, roleA) {
		return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: role %q is not registered in profile %q's role mappings", roleA, profile.ID)
	}
	if !roleRegistered(profile, roleB) {
		return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: role %q is not registered in profile %q's role mappings", roleB, profile.ID)
	}

	matched := false
	for _, r := range profile.DistinctnessRules {
		if !stringsContain(r.Transitions, transition) {
			continue
		}
		lo, hi := r.LeftRole, r.RightRole
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo == roleA && hi == roleB && r.Relation == relation {
			matched = true
			break
		}
	}
	if !matched {
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: sortedUniqueCopy([]string{roleA, roleB}), Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}, nil
	}

	// DEFERRED (owner-gated, frozen Task 3 wire): the kernel decision's
	// Disclosures — notably a solo profile's proven author/approver
	// collapse — have no carrier on a MechanicalEvaluation, whose only
	// disclosure channel is the report's top-level list this function
	// cannot reach. They are deliberately not consumed here.
	approvals := buildRoleApprovals(profile, actors, roleA, roleB)
	decision, err := governanceprincipal.Authorize(profile, governanceprincipal.AuthorizationRequest{
		Transition:  transition,
		Posture:     governanceprincipal.PostureAuthoritative,
		Resolutions: actors,
		Approvals:   approvals,
	})
	if err != nil {
		return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: kernel authorization: %w", err)
	}

	// Authority design §5.3 is literal about the whole decision, not about
	// one finding code: "Requiring one relation is proven only when the
	// kernel returns that conclusion; violated and unproven kernel results
	// remain violated-with-witness or unproven respectively." The kernel's
	// three-valued State is therefore mapped faithfully — a decision
	// violated for ANY reason (an experimental profile's forbidden
	// authoritative authorization, an unauthorized role, a violated
	// principal resolution) is never a mechanical proof, and a decision
	// carrying only unproven findings is never upgraded either. Reading a
	// distinctness finding code alone would have discarded exactly those
	// results and manufactured a proven row from a violated kernel answer.
	distinctnessShortfall := false
	witnessSet := map[string]bool{}
	for _, f := range decision.Findings {
		if f.Code == governanceprincipal.ReasonDistinctnessViolated || f.Code == governanceprincipal.ReasonDistinctnessUnproven {
			distinctnessShortfall = true
		}
		// Every finding the kernel returns is part of this relation's
		// evidence; its stable code plus whichever role/principal it names
		// is the witness the report carries. (Findings' own three-valued
		// contribution stays typed inside the kernel decision — the row's
		// closed reason vocabulary names the outcome, not each finding.)
		witnessSet[f.Code] = true
		if f.Role != "" {
			witnessSet["role:"+f.Role] = true
		}
		if f.PrincipalID != "" {
			witnessSet["principal:"+string(f.PrincipalID)] = true
		}
	}
	witnesses := sortedKeysOf(witnessSet)
	values := sortedUniqueCopy([]string{roleA, roleB})

	switch decision.State {
	case governanceprincipal.AuthorizationViolated:
		return SolverProof{State: SolverUnsatisfiable, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, nil
	case governanceprincipal.AuthorizationUnproven:
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, nil
	case governanceprincipal.AuthorizationAuthorized:
		// An authorized decision carries no findings at all, so the
		// distinctness conclusion this claim asked for is exactly what the
		// kernel returned. The two guards below never fire against the
		// landed kernel; they exist so a future kernel that authorized
		// while still reporting a distinctness shortfall, or that answered
		// under a downgraded (advisory) posture, would withhold the proof
		// rather than have this package read authorization as distinctness.
		if distinctnessShortfall || decision.Posture != governanceprincipal.PostureAuthoritative {
			return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, nil
		}
		return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: values}, nil
	default:
		return SolverProof{}, fmt.Errorf("policyconflict: principal-relation solver: kernel returned unknown authorization state %q", decision.State)
	}
}

// buildRoleApprovals proposes one candidate ApprovalRecord per authenticated
// actor that this package's own role-membership check (profile.RoleMappings'
// exported trust-source/subject data) finds eligible for roleA or roleB.
// This never compares principal strings to each other — it only asks
// whether one actor's own already-authenticated claim matches one role's
// declared mapping, mirroring the same check the kernel re-verifies inside
// Authorize before it ever reasons about distinctness.
func buildRoleApprovals(profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, roleA, roleB string) []governanceprincipal.ApprovalRecord {
	var out []governanceprincipal.ApprovalRecord
	for _, role := range []string{roleA, roleB} {
		for _, actor := range actors {
			if actor.State != governanceprincipal.ResolutionAuthenticated {
				continue
			}
			if holdsRoleLocal(profile, actor, role) {
				out = append(out, governanceprincipal.ApprovalRecord{Role: role, PrincipalID: actor.PrincipalID})
			}
		}
	}
	return out
}

// holdsRoleLocal mirrors the kernel's own unexported holdsRole. DEFERRED
// (owner-gated): the proper seam is an exported governanceprincipal method,
// which is outside this task's write set — the duplication stays until that
// kernel change is authorized.
func holdsRoleLocal(profile governanceprincipal.Profile, actor governanceprincipal.PrincipalResolution, role string) bool {
	for _, m := range profile.RoleMappings {
		if m.Role == role && m.TrustSource == actor.Claim.TrustSource && stringsContain(m.Subjects, actor.Claim.Subject) {
			return true
		}
	}
	return false
}

func roleRegistered(profile governanceprincipal.Profile, role string) bool {
	for _, m := range profile.RoleMappings {
		if m.Role == role {
			return true
		}
	}
	return false
}

// --- small shared helpers ----------------------------------------------------

func stringsContain(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func sortedKeysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
