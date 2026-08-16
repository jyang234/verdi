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
// relation-bearing evidence is unproven. A relation-bearing kernel finding
// that is violated is likewise never a proof (§5.3). This mirrors
// ReasonPrincipalRelationUnproven and ReasonPrincipalRelationViolated in
// the closed reason vocabulary.
//
// Which kernel findings BEAR on the requested relation is itself fixed by
// §5.3's operand set (relationBearingFinding, ledger SI-106): whole-request
// authority findings, plus findings whose exact role or canonical role pair
// is this claim's. An unrelated approver, signature, ownership, or SECOND
// distinctness rule's shortfall never changes this relation. One kernel
// outcome is neither a pass nor a violation: an experimental profile forces
// an advisory effective posture, which is unproven under the row reason
// ReasonProfileExperimental (unprovenReasonFor). Kernel disclosures ride
// beside the rows in MechanicalResult, translated once by
// translateKernelDisclosures and hoisted to the report only by Task 9.
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
// and scope/identity digests are sha256:<64 lowercase hex> with no "," in
// them, so every ID parses back to exactly one group and suffix.
func (k mechanicalGroupKey) id() string {
	if k.family == policyartifact.FamilyIdentity {
		return fmt.Sprintf("%s:%s:%s+%s", k.family, k.subject, k.roleA, k.roleB)
	}
	return fmt.Sprintf("%s:%s", k.family, k.subject)
}

// claimIdentityDigest is the canonical content address of one claim's
// COMPOSITE (policy_id, claim_id) identity (ledger SI-109) — the component
// every step-2 pair row is built from.
//
// A pair component can never be the claim digest alone: two policies
// declaring BYTE-IDENTICAL contradictory claims would then mint two rows
// carrying one ID, which the report's own row-ID uniqueness rule refuses
// and which no sort can order deterministically. Digesting the composite
// identity keeps the suffix bounded and content-derived without adding a
// delimiter or escaping grammar, and one composite identity still names
// exactly one claim because drift under it already fails operationally in
// normalizeClaimOperands, which also collapses an exact repeat of one
// identity so no two members can mint the same pair suffix.
func claimIdentityDigest(c contextcompile.TypedClaim) (string, error) {
	d, err := canonjson.Digest(struct {
		ClaimID  string `json:"claim_id"`
		PolicyID string `json:"policy_id"`
	}{ClaimID: c.Claim.ID, PolicyID: c.PolicyID})
	if err != nil {
		return "", fmt.Errorf("policyconflict: digest claim identity (policy %q, claim %q): %w", c.PolicyID, c.Claim.ID, err)
	}
	return d, nil
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

// MechanicalResult is EvaluateMechanical's complete outcome (ledger
// SI-106): the deterministic mechanical rows, plus the translated kernel
// disclosures the identity domain's authorization produced. It is a
// RUNTIME-only value, not a wire document — Task 9 remains the single place
// that hoists Disclosures into the report's one top-level disclosure
// location, so no mechanical row is coupled to global disclosure state.
type MechanicalResult struct {
	Evaluations []MechanicalEvaluation
	Disclosures []Disclosure
}

// EvaluateMechanical proves every typed claim group's mechanical
// satisfiability (authority design §5). It groups in.Claims, solves each
// group's complete conjunction first, and — for an unsatisfiable complete
// conjunction — derives the deterministic scope-witness rows the package
// comment above describes. Rows are always returned in ascending ID order;
// identical inputs deep-equal outputs.
func EvaluateMechanical(ctx context.Context, in MechanicalInput) (MechanicalResult, error) {
	claims, err := normalizeClaimOperands(in.Claims)
	if err != nil {
		return MechanicalResult{}, err
	}

	// Sort on the claim digest first (the row-ID component) and break ties
	// on the composite identity, so two policies declaring byte-identical
	// claims still order deterministically against each other.
	ordered := append([]contextcompile.TypedClaim(nil), claims...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].ClaimDigest != ordered[j].ClaimDigest {
			return ordered[i].ClaimDigest < ordered[j].ClaimDigest
		}
		if ordered[i].PolicyID != ordered[j].PolicyID {
			return ordered[i].PolicyID < ordered[j].PolicyID
		}
		return ordered[i].Claim.ID < ordered[j].Claim.ID
	})

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
	var kernel []governanceprincipal.Disclosure
	for _, gk := range order {
		grows, gdisclosures, err := evaluateGroup(ctx, gk, buckets[gk], in.Profile, in.Actors, in.Refs)
		if err != nil {
			return MechanicalResult{}, err
		}
		rows = append(rows, grows...)
		kernel = append(kernel, gdisclosures...)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })

	disclosures, err := translateKernelDisclosures(kernel)
	if err != nil {
		return MechanicalResult{}, err
	}
	return MechanicalResult{Evaluations: rows, Disclosures: disclosures}, nil
}

// normalizeClaimOperands is EvaluateMechanical's entry contract over the
// typed claims it will solve (ledger SI-105):
//
//   - every operand carries its policy identity, policy digest and claim
//     digest, and passes Claim.Validate;
//   - every carried claim_digest is RECOMPUTED from the carried base claim,
//     so a hand-built or mutated digest is refused instead of silently
//     addressing content that no longer exists;
//   - one composite (policy_id, claim_id) identity names exactly one claim.
//     A repeated identity carrying different content is contradictory
//     authority and fails operationally rather than choosing a meaning.
//
// It is also the package's SINGLE member-normalization seam: an EXACT
// repeat of one composite identity is one operand, so the canonical member
// list it returns carries each composite identity once. Everything
// downstream — grouping, the domain solvers, both witness steps and the row
// records — reads that one list, which is what keeps step 2's "every UNIQUE
// differently-scoped claim pair" unique. Without it a legitimately repeated
// identity pairs twice with the same differently-scoped claim and mints two
// byte-identical rows carrying one ID, which the report's own row-ID
// uniqueness rule refuses. Deduplication is only over a repeat this function
// has just proven identical; two different contents under one composite
// identity are still refused above rather than collapsed.
func normalizeClaimOperands(claims []contextcompile.TypedClaim) ([]contextcompile.TypedClaim, error) {
	type identity struct{ policyID, claimID string }
	seen := make(map[identity]contextcompile.TypedClaim, len(claims))
	normalized := make([]contextcompile.TypedClaim, 0, len(claims))
	for i, c := range claims {
		if c.PolicyID == "" || c.PolicyDigest == "" || c.ClaimDigest == "" {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d]: policy id, policy digest, and claim digest are all required", i)
		}
		if err := c.Claim.Validate(); err != nil {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d] %s: %w", i, c.ClaimDigest, err)
		}
		recomputed, err := policyartifact.ClaimDigest(c.Claim)
		if err != nil {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d] (policy %q claim %q): digest claim: %w", i, c.PolicyID, c.Claim.ID, err)
		}
		if recomputed != c.ClaimDigest {
			return nil, fmt.Errorf("policyconflict: mechanical claim [%d] (policy %q claim %q): carried claim digest %s is not the canonical digest of its claim (%s)", i, c.PolicyID, c.Claim.ID, c.ClaimDigest, recomputed)
		}
		key := identity{c.PolicyID, c.Claim.ID}
		if prev, ok := seen[key]; ok {
			if prev.ClaimDigest != c.ClaimDigest || prev.PolicyDigest != c.PolicyDigest {
				return nil, fmt.Errorf("policyconflict: mechanical claim [%d]: two different claims share identity (policy %q, claim %q)", i, c.PolicyID, c.Claim.ID)
			}
			continue
		}
		seen[key] = c
		normalized = append(normalized, c)
	}
	return normalized, nil
}

// translateKernelDisclosures maps the kernel's own disclosure vocabulary
// onto the report's (authority design §5.3, ledger SI-106/SI-108). Only
// solo-role-collapse translates, to report code solo-principal-collapse;
// each principal/role membership becomes ONE witness token
// `<principal_id>:<role_id>`, and the tokens sort and deduplicate, so
// repeated kernel calls for the same group collapse into one disclosure.
//
// These are distinct closed vocabularies: an unknown kernel disclosure is
// an operational error rather than a new report label. Both component
// grammars forbid ":" — the token is therefore lossless and reversible —
// and each component is checked through the kernel's OWN exported grammar
// rather than a duplicated pattern here, so a component outside its closed
// grammar is refused instead of silently producing an ambiguous token.
func translateKernelDisclosures(kernel []governanceprincipal.Disclosure) ([]Disclosure, error) {
	tokens := map[string]bool{}
	for _, d := range kernel {
		if d.Code != governanceprincipal.ReasonSoloRoleCollapse {
			return nil, fmt.Errorf("policyconflict: kernel disclosure %q has no report translation", d.Code)
		}
		if err := d.PrincipalID.Validate(); err != nil {
			return nil, fmt.Errorf("policyconflict: kernel disclosure %q: %w", d.Code, err)
		}
		if len(d.Roles) == 0 {
			return nil, fmt.Errorf("policyconflict: kernel disclosure %q for principal %q names no role: the membership cannot be recorded losslessly", d.Code, d.PrincipalID)
		}
		for _, role := range d.Roles {
			if err := governanceprincipal.ValidateID(role); err != nil {
				return nil, fmt.Errorf("policyconflict: kernel disclosure %q for principal %q: role: %w", d.Code, d.PrincipalID, err)
			}
			tokens[string(d.PrincipalID)+":"+role] = true
		}
	}
	if len(tokens) == 0 {
		return []Disclosure{}, nil
	}
	return []Disclosure{{Code: DisclosureSoloPrincipalCollapse, Witnesses: sortedKeysOf(tokens)}}, nil
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
// or per-operator-pair table. Only the principal-relation domain calls the
// kernel, so it is the only one that can return kernel disclosures.
func solveGroup(domain string, gk mechanicalGroupKey, claims []policyartifact.Claim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) (SolverProof, []governanceprincipal.Disclosure, error) {
	switch domain {
	case domainDiscreteSet:
		return solveDiscrete(claims), nil, nil
	case domainIntegerInterval:
		proof, err := solveInterval(claims)
		return proof, nil, err
	case domainPrincipalRelation:
		return solvePrincipalRelation(gk.subject, gk.roleA, gk.roleB, claims, profile, actors)
	case domainPathCapability:
		return solvePathCapability(claims), nil, nil
	default:
		return SolverProof{}, nil, fmt.Errorf("policyconflict: unknown mechanical domain %q", domain)
	}
}

func scopeProofFor(ctx context.Context, members []contextcompile.TypedClaim, refs RefRelationResolver) (ScopeProof, error) {
	scopes := make([]policyartifact.Scope, len(members))
	for i, m := range members {
		scopes[i] = m.Claim.Scope
	}
	return IntersectScopes(ctx, scopes, refs)
}

func evaluateGroup(ctx context.Context, gk mechanicalGroupKey, members []contextcompile.TypedClaim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) ([]MechanicalEvaluation, []governanceprincipal.Disclosure, error) {
	domain, err := determineDomain(members)
	if err != nil {
		return nil, nil, err
	}

	before, disclosures, err := solveGroup(domain, gk, claimsOf(members), profile, actors)
	if err != nil {
		return nil, nil, err
	}

	switch before.State {
	case SolverSatisfiable:
		scope, err := scopeProofFor(ctx, members, refs)
		if err != nil {
			return nil, nil, err
		}
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofProven, []ReasonCode{ReasonMechanicalSatisfiable})}, disclosures, nil
	case SolverUnproven:
		scope, err := scopeProofFor(ctx, members, refs)
		if err != nil {
			return nil, nil, err
		}
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofUnproven, []ReasonCode{unprovenReasonFor(before)})}, disclosures, nil
	case SolverUnsatisfiable:
		rows, witnessDisclosures, err := deriveWitnessRows(ctx, gk, members, domain, before, profile, actors, refs)
		if err != nil {
			return nil, nil, err
		}
		return rows, append(disclosures, witnessDisclosures...), nil
	default:
		return nil, nil, fmt.Errorf("policyconflict: group %s: solver returned unknown state %q", gk.id(), before.State)
	}
}

// deriveWitnessRows implements authority design §5's three-step procedure
// for an unsatisfiable complete conjunction.
func deriveWitnessRows(ctx context.Context, gk mechanicalGroupKey, members []contextcompile.TypedClaim, domain string, before SolverProof, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) ([]MechanicalEvaluation, []governanceprincipal.Disclosure, error) {
	var disclosures []governanceprincipal.Disclosure
	scopeDigests := make([]string, len(members))
	identityDigests := make([]string, len(members))
	for i, m := range members {
		d, err := canonjson.Digest(m.Claim.Scope)
		if err != nil {
			return nil, nil, fmt.Errorf("policyconflict: digest claim scope: %w", err)
		}
		scopeDigests[i] = d
		identityDigests[i], err = claimIdentityDigest(m)
		if err != nil {
			return nil, nil, err
		}
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
		subBefore, subDisclosures, err := solveGroup(domain, gk, claimsOf(subMembers), profile, actors)
		if err != nil {
			return nil, nil, err
		}
		disclosures = append(disclosures, subDisclosures...)
		if subBefore.State != SolverUnsatisfiable {
			continue
		}
		subScope, err := scopeProofFor(ctx, subMembers, refs)
		if err != nil {
			return nil, nil, err
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
			pairBefore, pairDisclosures, err := solveGroup(domain, gk, claimsOf(pairMembers), profile, actors)
			if err != nil {
				return nil, nil, err
			}
			disclosures = append(disclosures, pairDisclosures...)
			if pairBefore.State != SolverUnsatisfiable {
				continue
			}
			pairScope, err := scopeProofFor(ctx, pairMembers, refs)
			if err != nil {
				return nil, nil, err
			}
			if pairScope.State != ScopeOverlap {
				continue
			}
			// SI-109: the ordered pair of COMPOSITE identity digests, in
			// the members' own deterministic order.
			id := gk.id() + "#pair:" + identityDigests[i] + "," + identityDigests[j]
			violated = append(violated, buildRow(id, gk, pairMembers, pairScope, domain, pairBefore, ProofViolatedWithWitness, []ReasonCode{unsatReasonFor(domain, claimsOf(pairMembers))}))
		}
	}

	if len(violated) > 0 {
		sort.Slice(violated, func(a, b int) bool { return violated[a].ID < violated[b].ID })
		return violated, disclosures, nil
	}

	// Step 3: neither step proved an overlap witness. Two cases remain, and
	// they are not the same result (never a proven-disjoint or scope-unknown
	// witness upgraded into a conflict, and never silently dropped either).
	scope, err := scopeProofFor(ctx, members, refs)
	if err != nil {
		return nil, nil, err
	}
	settled, settleDisclosures, err := disjointnessSettlesGroup(ctx, members, domain, gk, profile, actors, refs)
	if err != nil {
		return nil, nil, err
	}
	disclosures = append(disclosures, settleDisclosures...)
	if settled && scope.State == ScopeDisjoint {
		// Authority design §5: "Proven-disjoint witnesses do not conflict."
		// Nothing REMAINS for the higher-order case here, so this is a
		// proven row, not a withheld conclusion.
		return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofProven, []ReasonCode{ReasonScopeDisjoint})}, disclosures, nil
	}
	// The genuine residual: some co-applicable subset could still be
	// unsatisfiable (or its scope is unknown), so one blocked-unproven row
	// carries the complete claim witness.
	return []MechanicalEvaluation{buildRow(gk.id()+"#complete", gk, members, scope, domain, before, ProofUnproven, []ReasonCode{ReasonHigherOrderScopeUnproven})}, disclosures, nil
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
func disjointnessSettlesGroup(ctx context.Context, members []contextcompile.TypedClaim, domain string, gk mechanicalGroupKey, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, refs RefRelationResolver) (bool, []governanceprincipal.Disclosure, error) {
	var disclosures []governanceprincipal.Disclosure
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
				return false, nil, err
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
		return false, disclosures, nil
	}
	for _, root := range componentOrder {
		proof, componentDisclosures, err := solveGroup(domain, gk, claimsOf(components[root]), profile, actors)
		if err != nil {
			return false, nil, err
		}
		disclosures = append(disclosures, componentDisclosures...)
		if proof.State != SolverSatisfiable {
			return false, disclosures, nil
		}
	}
	return true, disclosures, nil
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

// unprovenReasonFor names an unproven row's outcome from the proof that
// withheld it, mirroring unsatReasonFor's witness-derived rule. An
// experimental profile forces an advisory effective posture: authority
// design §5.3 makes that UNPROVEN for the authoritative consumer — never a
// relation violation and never an authoritative pass — under its own row
// reason profile-experimental. Every other withheld relation keeps
// principal-relation-unproven.
func unprovenReasonFor(proof SolverProof) ReasonCode {
	if stringsContain(proof.Witnesses, governanceprincipal.ReasonExperimentalAuthorityForbidden) {
		return ReasonProfileExperimental
	}
	return ReasonPrincipalRelationUnproven
}

func buildRow(id string, gk mechanicalGroupKey, members []contextcompile.TypedClaim, scope ScopeProof, domain string, proof SolverProof, state ProofState, reasons []ReasonCode) MechanicalEvaluation {
	records := claimRecordsFor(members)

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

// claimRecordsFor turns members into the row's claim records under ledger
// SI-105's composite identity rule: a record is keyed by (policy_id,
// claim_id), so equal claim BYTES declared by two different policies stay
// TWO records. Policy identity is part of an exemption witness — an
// exemption departs from one policy's claim and not the other's, and a row
// that collapsed them could not express that — so it is never discarded
// merely because the bytes agree.
//
// Deduplication is therefore only over a genuinely repeated composite
// identity, which normalizeClaimOperands has already proven carries
// identical content and already collapsed on the member list; the records
// sort by that same composite key, which is the order the wire requires.
func claimRecordsFor(members []contextcompile.TypedClaim) []TypedClaimRecord {
	type identity struct{ policyID, claimID string }
	seen := make(map[identity]bool, len(members))
	records := make([]TypedClaimRecord, 0, len(members))
	for _, m := range members {
		key := identity{m.PolicyID, m.Claim.ID}
		if seen[key] {
			continue
		}
		seen[key] = true
		records = append(records, TypedClaimRecord{PolicyID: m.PolicyID, PolicyDigest: m.PolicyDigest, ClaimDigest: m.ClaimDigest, Claim: m.Claim})
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].PolicyID != records[j].PolicyID {
			return records[i].PolicyID < records[j].PolicyID
		}
		return records[i].Claim.ID < records[j].Claim.ID
	})
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
func solvePrincipalRelation(transition, roleA, roleB string, claims []policyartifact.Claim, profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution) (SolverProof, []governanceprincipal.Disclosure, error) {
	hasSame, hasDiff := false, false
	for _, c := range claims {
		switch c.Operator {
		case policyartifact.OpSamePrincipal:
			hasSame = true
		case policyartifact.OpDifferentPrincipal:
			hasDiff = true
		default:
			return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: unexpected operator %q", c.Operator)
		}
	}

	if !hasSame && !hasDiff {
		return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}, nil, nil
	}
	if hasSame && hasDiff {
		roles := sortedUniqueCopy([]string{roleA, roleB})
		return SolverProof{
			State: SolverUnsatisfiable, Domain: domainPrincipalRelation,
			Values: roles, Required: []string{}, Forbidden: []string{},
			Witnesses: roles,
		}, nil, nil
	}

	relation := governanceprincipal.RelationSamePrincipal
	if hasDiff {
		relation = governanceprincipal.RelationDifferentPrincipal
	}

	if !stringsContain(profile.ApplicableTransitions, transition) {
		return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: transition %q is not registered in profile %q's applicable transitions", transition, profile.ID)
	}
	if !roleRegistered(profile, roleA) {
		return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: role %q is not registered in profile %q's role mappings", roleA, profile.ID)
	}
	if !roleRegistered(profile, roleB) {
		return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: role %q is not registered in profile %q's role mappings", roleB, profile.ID)
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
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: sortedUniqueCopy([]string{roleA, roleB}), Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}, nil, nil
	}

	approvals, err := buildRoleApprovals(profile, actors, roleA, roleB)
	if err != nil {
		return SolverProof{}, nil, err
	}
	decision, err := governanceprincipal.Authorize(profile, governanceprincipal.AuthorizationRequest{
		Transition:  transition,
		Posture:     governanceprincipal.PostureAuthoritative,
		Resolutions: actors,
		Approvals:   approvals,
	})
	if err != nil {
		return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: kernel authorization: %w", err)
	}

	// An unknown decision state is a kernel contract this package cannot
	// interpret: fail closed rather than read it as any outcome.
	switch decision.State {
	case governanceprincipal.AuthorizationAuthorized, governanceprincipal.AuthorizationViolated, governanceprincipal.AuthorizationUnproven:
	default:
		return SolverProof{}, nil, fmt.Errorf("policyconflict: principal-relation solver: kernel returned unknown authorization state %q", decision.State)
	}

	// Authority design §5.3 fixes this question's operand set exactly: the
	// evaluator "constructs one kernel authorization request over the exact
	// authenticated resolutions, transition, canonical role pair, profile,
	// and separation mode", and "Requiring one relation is proven only when
	// the kernel returns that conclusion; violated and unproven kernel
	// results remain violated-with-witness or unproven respectively."
	//
	// The kernel's WHOLE-decision state answers a broader question than
	// that operand set: governanceprincipal degrades a decision to unproven
	// on any finding at all, including approver-count, signature,
	// ownership, evidence-source and escalation shortfalls about roles this
	// claim never names. Those are not part of §5.3's operand set, so they
	// are not evidence about this relation — consuming them wholesale
	// blocks a relation the kernel actually proved. The outcome is
	// therefore derived from the findings that BEAR on this claim
	// (relationBearingFinding), never from an unrelated rule's shortfall
	// and never from silence about a rule the kernel did run.
	witnessSet := map[string]bool{}
	relationViolated, relationUnproven, relationExperimental := false, false, false
	for _, f := range decision.Findings {
		if !relationBearingFinding(f, roleA, roleB) {
			continue
		}
		switch {
		case f.Code == governanceprincipal.ReasonExperimentalAuthorityForbidden:
			// §5.3: an experimental profile's advisory posture is UNPROVEN
			// for this authoritative consumer, not evidence that the
			// requested relation is violated. It is recorded separately from
			// the violated/unproven tallies so it can never be read as a
			// mechanical conflict.
			relationExperimental = true
		case f.State == governanceprincipal.AuthorizationViolated:
			relationViolated = true
		case f.State == governanceprincipal.AuthorizationUnproven:
			relationUnproven = true
		}
		// A relation-bearing finding's stable code plus whichever role or
		// principal it names is the witness this proof carries. (Findings'
		// own three-valued contribution stays typed inside the kernel
		// decision — the row's closed reason vocabulary names the outcome,
		// not each finding.)
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

	switch {
	case relationExperimental:
		// Outranks every other outcome deliberately: an experimental
		// profile can produce NO authoritative conclusion at all, so it
		// yields neither a pass nor a violation — only an unproven row,
		// whose reason is profile-experimental (unprovenReasonFor).
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, decision.Disclosures, nil
	case relationViolated:
		return SolverProof{State: SolverUnsatisfiable, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, decision.Disclosures, nil
	case relationUnproven:
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, decision.Disclosures, nil
	case decision.Posture != governanceprincipal.PostureAuthoritative:
		// A downgraded effective posture is not the authoritative answer
		// this request asked for, so it never proves the relation. (The
		// landed kernel only downgrades an experimental profile, which the
		// case above already names; this guard keeps a future posture-only
		// downgrade from reading as proof.)
		return SolverProof{State: SolverUnproven, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: witnesses}, decision.Disclosures, nil
	default:
		// The kernel ran the matching DistinctnessRule (checked above) and
		// reported no adverse relation-bearing evidence: that IS the
		// kernel returning this claim's conclusion.
		return SolverProof{State: SolverSatisfiable, Domain: domainPrincipalRelation, Values: values, Required: []string{}, Forbidden: []string{}, Witnesses: values}, decision.Disclosures, nil
	}
}

// relationBearingFinding classifies one kernel finding as evidence about
// THIS claim's relation, under authority design §5.3's operand set (the
// exact authenticated resolutions, transition, canonical role pair,
// profile, and separation mode) and ledger SI-106:
//
//   - whole-request authority findings — an experimental profile's
//     forbidden authoritative authorization, or a transition the profile
//     does not govern — invalidate the authorization the relation would be
//     proven through, so they always bear on it;
//   - a finding carrying a role PAIR is a rule defined over that pair
//     (every distinctness finding). It bears only when the pair IS this
//     claim's canonical pair: a SECOND distinctness rule's violation or
//     shortfall is about a different relation and can never flip this one;
//   - every other finding — principal-*, role-not-authorized, signature-*,
//     ownership-*, evidence-source-*, escalation-*,
//     required-approver-missing — bears only when it names one of this
//     claim's two roles. A shortfall about a role this claim never names
//     (an approver count, another role's signature) is outside §5.3's
//     operand set and is not relation evidence.
func relationBearingFinding(f governanceprincipal.Finding, roleA, roleB string) bool {
	switch f.Code {
	case governanceprincipal.ReasonExperimentalAuthorityForbidden, governanceprincipal.ReasonTransitionNotApplicable:
		return true
	}
	if len(f.Roles) > 0 {
		// Both sides normalize lexically (the two roles are a semantic set),
		// so a reversed spelling of the same relation identifies identically.
		lo, hi := roleA, roleB
		if lo > hi {
			lo, hi = hi, lo
		}
		return len(f.Roles) == 2 && f.Roles[0] == lo && f.Roles[1] == hi
	}
	return f.Role == roleA || f.Role == roleB
}

// buildRoleApprovals proposes one candidate ApprovalRecord per authenticated
// actor the KERNEL's own exported role-membership query
// (governanceprincipal.HoldsRole, ledger SI-106) finds eligible for roleA or
// roleB. This package therefore holds no second interpretation of role
// mapping semantics. It never compares principal strings to each other —
// it only asks whether one actor's own already-authenticated claim matches
// one role's declared mapping, which is the same check the kernel
// re-verifies inside Authorize before it ever reasons about distinctness.
func buildRoleApprovals(profile governanceprincipal.Profile, actors []governanceprincipal.PrincipalResolution, roleA, roleB string) ([]governanceprincipal.ApprovalRecord, error) {
	var out []governanceprincipal.ApprovalRecord
	for _, role := range []string{roleA, roleB} {
		for _, actor := range actors {
			if actor.State != governanceprincipal.ResolutionAuthenticated {
				continue
			}
			holds, err := governanceprincipal.HoldsRole(profile, actor.Claim, role)
			if err != nil {
				return nil, fmt.Errorf("policyconflict: principal-relation solver: role membership: %w", err)
			}
			if holds {
				out = append(out, governanceprincipal.ApprovalRecord{Role: role, PrincipalID: actor.PrincipalID})
			}
		}
	}
	return out, nil
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
