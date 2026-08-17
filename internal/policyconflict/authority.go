// authority.go resolves the five-state authority-resolution substates
// (match, freshness, scope coverage, bound, authorization) an exemption or
// disposition needs before it may depart from or resolve a conflict
// (authority design §§4, 5.5, 8, 9; ledger SI-113/SI-114/SI-115). It never
// interprets a mechanical or semantic PROOF itself (mechanical.go, scope.go,
// semantic.go own that) and it never applies an accepted resolution
// (exemption.go's ApplyEffectiveExemptions does that with the resolutions
// this file returns) — this file only DERIVES the typed states, delegating
// every date comparison to the injected evaluated_on and every
// approval/separation/ownership/signature meaning to
// governanceprincipal.Authorize. It never compares principal strings.
package policyconflict

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// DateSource supplies the one injected UTC calendar date every bound
// comparison in this package uses (authority design §9): production reads
// the real current date; tests inject a fixed one. Neither resolver in this
// file calls it directly — the caller (the Task 9 orchestrator) obtains the
// date once and carries it as AuthorityInput.EvaluatedOn, so every
// resolution in one report shares exactly one stamp.
type DateSource interface {
	TodayUTC(ctx context.Context) (string, error)
}

// RefCoverageResolver proves whether one exemption's ref selector
// (container) directionally covers a different row ref (member) (authority
// design §5.5): a feature exemption can cover an implementing story, but
// covering never runs backwards. Two guarantees mirror RefRelationResolver
// (scope.go): a symmetric "overlap" answer is never a substitute for
// directional coverage, and only a genuinely different, resolver-proven
// pair may invoke it — exact equality and every non-ref dimension never do.
type RefCoverageResolver interface {
	Covers(ctx context.Context, container, member string) (ProofState, []string, error)
}

// AuthorityInput carries every fact both resolvers need beyond the
// mechanical row or semantic input itself (authority design §9): the
// injected evaluation date, the current target's exact digest (SI-114's
// separate exact target comparison), the sealed governance profile, sealed
// actor resolutions, and the loaded exemption/disposition artifacts a
// caller (Task 9) selected for consideration. Actors arrive already
// resolved by governanceprincipal.Resolver — this package never
// authenticates a principal itself, only asks the kernel to interpret
// already-sealed resolutions (design §9, Task 4 residual: a zero-value,
// unsealed resolution is rejected by governanceprincipal.Authorize itself,
// as an operational error, never a favorable default).
type AuthorityInput struct {
	EvaluatedOn  string
	TargetDigest string
	Profile      governanceprincipal.Profile
	Actors       []governanceprincipal.PrincipalResolution
	Exemptions   []policyartifact.Exemption
	Dispositions []policyartifact.Disposition
}

// The two fixed governance transitions SI-113 assigns (design §9): neither
// resolver ever derives a transition from a row subject, artifact name, or
// committed principal string.
const (
	transitionExemptionApproval   = "policy-exemption-approval"
	transitionDispositionApproval = "policy-disposition-approval"
)

// --- shared bound/authorization resolution ---------------------------------

// resolveBound proves the calendar-expiry/review-condition bound (authority
// design §9): proven for an expiry on or after evaluatedOn, violated with
// witness for an earlier expiry, and unproven when a live review condition
// is the only bound (v1 has no evidence source capable of proving one). A
// missing or malformed evaluatedOn is operational — never a favorable or
// unfavorable verdict, because no date was actually injected to compare
// against. expiry is assumed already a real calendar date (both exemption
// and disposition decode already enforce that grammar); a malformed expiry
// therefore reports the caller's own hand-built-operand defect
// operationally, the same posture this package takes everywhere else.
func resolveBound(expiry, review, evaluatedOn string) (ProofState, error) {
	if err := validateEvaluatedOn("authority_input.evaluated_on", evaluatedOn); err != nil {
		return "", err
	}
	if expiry == "" {
		if review == "" {
			return "", fmt.Errorf("policyconflict: resolve bound: neither an expiry nor a review condition was supplied")
		}
		return ProofUnproven, nil
	}
	evalDate, err := time.Parse("2006-01-02", evaluatedOn)
	if err != nil {
		return "", fmt.Errorf("policyconflict: resolve bound: evaluated_on %q: %w", evaluatedOn, err)
	}
	expDate, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return "", fmt.Errorf("policyconflict: resolve bound: expiry %q is not a real YYYY-MM-DD calendar date: %w", expiry, err)
	}
	if expDate.Before(evalDate) {
		return ProofViolatedWithWitness, nil
	}
	return ProofProven, nil
}

// resolveAuthorization asks governanceprincipal.Authorize to interpret
// actors/approvals against profile for exactly transition (SI-113), and
// translates its decision into the one typed Authorization substate
// (authority design §9). It always requests authoritative posture: this
// package is always an authoritative consumer, so an experimental profile's
// forced advisory posture must read as unproven — never as the violated
// finding Authorize itself records for the posture mismatch — matching
// §5.3/§9's "Advisory/experimental kernel posture is unproven for the
// authoritative consumer, not evidence that the requested relation is
// violated" and "Experimental profile results are always blocked-unproven
// for authoritative consumers even when [everything else] is otherwise
// clean." Every other kernel finding (missing approver, wrong role,
// distinctness violation, unknown transition) maps directly: an explicit
// violated finding outranks unproven, exactly as Authorize itself already
// orders them, and an absent finding set is authorized.
func resolveAuthorization(profile governanceprincipal.Profile, transition string, actors []governanceprincipal.PrincipalResolution, approvals []policyartifact.Approval) (ProofState, error) {
	records := make([]governanceprincipal.ApprovalRecord, 0, len(approvals))
	for _, a := range approvals {
		records = append(records, governanceprincipal.ApprovalRecord{
			Role:        a.Role,
			PrincipalID: governanceprincipal.PrincipalID(a.Principal),
		})
	}
	decision, err := governanceprincipal.Authorize(profile, governanceprincipal.AuthorizationRequest{
		Transition:  transition,
		Posture:     governanceprincipal.PostureAuthoritative,
		Resolutions: actors,
		Approvals:   records,
	})
	if err != nil {
		return "", fmt.Errorf("policyconflict: resolve authorization: %w", err)
	}
	if decision.Posture == governanceprincipal.PostureAdvisory {
		return ProofUnproven, nil
	}
	switch decision.State {
	case governanceprincipal.AuthorizationAuthorized:
		return ProofProven, nil
	case governanceprincipal.AuthorizationViolated:
		return ProofViolatedWithWitness, nil
	case governanceprincipal.AuthorizationUnproven:
		return ProofUnproven, nil
	default:
		return "", fmt.Errorf("policyconflict: resolve authorization: unknown kernel decision state %q", decision.State)
	}
}

// --- exemption arm (authority design §5.5) ----------------------------------

// ResolveExemptionAuthority derives one ExemptionResolution for every
// exemption in in.Exemptions APPLICABLE to row — an exemption naming no
// witness sharing a current row claim's composite (policy_id, claim_id) is
// OMITTED entirely, never returned as a rejected resolution (SI-115). The
// returned slice is sorted ascending by exemption ID; it is the exact input
// exemption.go's ApplyEffectiveExemptions consumes to apply the accepted
// (all-five-proven) resolutions and record the rejected ones. row is never
// mutated. A hand-built or mutated row, a hand-built (never-decoded)
// exemption, a missing/malformed evaluated_on, or a kernel operand defect
// is an operational error — never a favorable or unfavorable verdict.
func ResolveExemptionAuthority(ctx context.Context, in AuthorityInput, row MechanicalEvaluation, resolver RefCoverageResolver) ([]ExemptionResolution, error) {
	if err := validateRowOperand(row); err != nil {
		return nil, err
	}
	if err := validateScopeProof("row.scope", row.Scope); err != nil {
		return nil, fmt.Errorf("policyconflict: resolve exemption authority: %w", err)
	}
	if err := validateEvaluatedOn("authority_input.evaluated_on", in.EvaluatedOn); err != nil {
		return nil, err
	}

	out := make([]ExemptionResolution, 0, len(in.Exemptions))
	seen := make(map[string]policyartifact.Exemption, len(in.Exemptions))
	for _, e := range in.Exemptions {
		if prev, ok := seen[e.ID]; ok {
			if !reflect.DeepEqual(prev, e) {
				return nil, fmt.Errorf("policyconflict: resolve exemption authority: two different exemption artifacts share id %q", e.ID)
			}
			continue
		}
		seen[e.ID] = e

		resolution, applicable, err := resolveExemption(ctx, in, row, e, resolver)
		if err != nil {
			return nil, err
		}
		if applicable {
			out = append(out, resolution)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// resolveExemption reports e's ExemptionResolution against row and whether
// e is applicable at all (SI-115: an inapplicable exemption is omitted, not
// rejected).
func resolveExemption(ctx context.Context, in AuthorityInput, row MechanicalEvaluation, e policyartifact.Exemption, resolver RefCoverageResolver) (ExemptionResolution, bool, error) {
	matched := make([]TypedClaimRecord, 0, len(row.Claims))
	fresh := true
	for _, c := range row.Claims {
		for _, w := range e.Witnesses {
			if w.Policy != c.PolicyID || w.Claim != c.Claim.ID {
				continue
			}
			matched = append(matched, c)
			if w.ClaimDigest != c.ClaimDigest {
				fresh = false
			}
			break
		}
	}
	if len(matched) == 0 {
		return ExemptionResolution{}, false, nil
	}

	resolution := AuthorityResolution{Match: ProofProven}
	if fresh {
		resolution.Freshness = ProofProven
	} else {
		resolution.Freshness = ProofViolatedWithWitness
	}

	scopeState, err := resolveExemptionScope(ctx, row.Scope, e.Scope, resolver)
	if err != nil {
		return ExemptionResolution{}, false, err
	}
	resolution.Scope = scopeState

	boundState, err := resolveBound(e.Expiry, e.ReviewCondition, in.EvaluatedOn)
	if err != nil {
		return ExemptionResolution{}, false, err
	}
	resolution.Bound = boundState

	authState, err := resolveAuthorization(in.Profile, transitionExemptionApproval, in.Actors, e.Approvals)
	if err != nil {
		return ExemptionResolution{}, false, err
	}
	resolution.Authorization = authState

	digest, err := e.Digest()
	if err != nil {
		return ExemptionResolution{}, false, fmt.Errorf("policyconflict: resolve exemption authority: exemption %q: %w", e.ID, err)
	}

	removed := make([]MechanicalClaimWitness, 0)
	if allProven(resolution) {
		for _, c := range matched {
			removed = append(removed, MechanicalClaimWitness{PolicyID: c.PolicyID, ClaimID: c.Claim.ID, ClaimDigest: c.ClaimDigest})
		}
		sort.Slice(removed, func(i, j int) bool {
			if removed[i].PolicyID != removed[j].PolicyID {
				return removed[i].PolicyID < removed[j].PolicyID
			}
			return removed[i].ClaimID < removed[j].ClaimID
		})
	}

	return ExemptionResolution{ID: e.ID, Digest: digest, Resolution: resolution, RemovedClaims: removed}, true, nil
}

// resolveExemptionScope proves directional coverage over row's own carried
// four-dimensional ScopeProof intersection (authority design §5.5): it
// never reruns a symmetric scope comparison and never accepts symmetric
// overlap as coverage. Each dimension is judged independently against
// exScope's matching selector set; the combined state is violated if any
// dimension is not contained, else unproven if any is unknown, else proven.
func resolveExemptionScope(ctx context.Context, row ScopeProof, exScope policyartifact.Scope, resolver RefCoverageResolver) (ProofState, error) {
	dims := map[string]bool{"phase": false, "environment": false, "path": false, "ref": false}
	violated, unproven := false, false
	for _, d := range row.Dimensions {
		if _, ok := dims[d.Dimension]; !ok {
			return "", fmt.Errorf("policyconflict: resolve exemption scope: unknown scope dimension %q", d.Dimension)
		}
		dims[d.Dimension] = true

		var state ProofState
		var err error
		switch d.Dimension {
		case "phase":
			state = coverSetDimension(d, exScope.Phases)
		case "environment":
			state = coverSetDimension(d, exScope.Environments)
		case "path":
			state = coverPathDimension(d, exScope.Paths)
		case "ref":
			state, err = coverRefDimension(ctx, d, exScope.Refs, resolver)
		}
		if err != nil {
			return "", err
		}
		switch state {
		case ProofViolatedWithWitness:
			violated = true
		case ProofUnproven:
			unproven = true
		}
	}
	for name, present := range dims {
		if !present {
			return "", fmt.Errorf("policyconflict: resolve exemption scope: row scope proof carries no %s dimension", name)
		}
	}

	switch {
	case violated:
		return ProofViolatedWithWitness, nil
	case unproven:
		return ProofUnproven, nil
	default:
		return ProofProven, nil
	}
}

// coverSetDimension covers d (phase or environment) via exact set
// containment (authority design §4.1/§5.5): a proven-disjoint row dimension
// is an empty conflict set and is trivially covered; a proven-overlap
// dimension with an empty intersection is universal, coverable only by an
// equally universal (empty) exemption dimension; otherwise every
// intersection value must occur in the exemption's own set, unless that set
// is itself empty (universal, covering every member). The closed
// phase/environment vocabularies make this proof complete — never unknown.
func coverSetDimension(d DimensionProof, exemptionValues []string) ProofState {
	switch d.State {
	case ScopeUnknown:
		return ProofUnproven
	case ScopeDisjoint:
		return ProofProven
	}
	if len(exemptionValues) == 0 {
		return ProofProven
	}
	if len(d.Intersection) == 0 {
		return ProofViolatedWithWitness
	}
	for _, v := range d.Intersection {
		if !sortedContains(exemptionValues, v) {
			return ProofViolatedWithWitness
		}
	}
	return ProofProven
}

// coverPathDimension covers d (path) the same way coverSetDimension does,
// substituting §4.2's segment-aware containment (scope.go's pathContains)
// for exact set membership: every intersection selector must be contained
// by at least one exemption path selector.
func coverPathDimension(d DimensionProof, exemptionPaths []string) ProofState {
	switch d.State {
	case ScopeUnknown:
		return ProofUnproven
	case ScopeDisjoint:
		return ProofProven
	}
	if len(exemptionPaths) == 0 {
		return ProofProven
	}
	if len(d.Intersection) == 0 {
		return ProofViolatedWithWitness
	}
	for _, v := range d.Intersection {
		covered := false
		for _, ex := range exemptionPaths {
			if pathContains(ex, v) {
				covered = true
				break
			}
		}
		if !covered {
			return ProofViolatedWithWitness
		}
	}
	return ProofProven
}

// coverRefDimension covers d (ref) the same way, substituting exact
// equality or a directionally proven RefCoverageResolver relation for path
// containment (authority design §5.5: "Ref requires exact equality or a
// directionally proven container-to-member relation... a symmetric overlap
// result is insufficient"). The ref port is invoked only when a genuinely
// different, nonuniversal pair needs it — exact, universal, and every
// non-ref dimension never invoke it.
func coverRefDimension(ctx context.Context, d DimensionProof, exemptionRefs []string, resolver RefCoverageResolver) (ProofState, error) {
	switch d.State {
	case ScopeUnknown:
		return ProofUnproven, nil
	case ScopeDisjoint:
		return ProofProven, nil
	}
	if len(exemptionRefs) == 0 {
		return ProofProven, nil
	}
	if len(d.Intersection) == 0 {
		return ProofViolatedWithWitness, nil
	}
	anyUnproven := false
	for _, r := range d.Intersection {
		state, err := coverRefMember(ctx, r, exemptionRefs, resolver)
		if err != nil {
			return "", err
		}
		switch state {
		case ProofViolatedWithWitness:
			return ProofViolatedWithWitness, nil
		case ProofUnproven:
			anyUnproven = true
		}
	}
	if anyUnproven {
		return ProofUnproven, nil
	}
	return ProofProven, nil
}

// coverRefMember proves whether some exemption ref selector covers member r:
// exact equality first (no resolver call), else the directional
// RefCoverageResolver relation from every selector to r. r is covered the
// moment any selector proves it; it is definitively NOT covered only when
// every selector's relation is proven false; any remaining unproven
// relation — with no selector proving coverage — leaves r's own coverage
// unknown rather than guessed either way.
func coverRefMember(ctx context.Context, r string, exemptionRefs []string, resolver RefCoverageResolver) (ProofState, error) {
	for _, ex := range exemptionRefs {
		if ex == r {
			return ProofProven, nil
		}
	}
	if resolver == nil {
		return "", fmt.Errorf("policyconflict: resolve exemption scope: a ref coverage resolver is required to relate %q to a different exemption ref selector", r)
	}
	sawUnproven := false
	for _, ex := range exemptionRefs {
		state, _, err := resolver.Covers(ctx, ex, r)
		if err != nil {
			return "", fmt.Errorf("policyconflict: ref coverage resolver: %q vs %q: %w", ex, r, err)
		}
		if err := state.Validate(); err != nil {
			return "", fmt.Errorf("policyconflict: ref coverage resolver returned an invalid proof state for %q vs %q: %w", ex, r, err)
		}
		switch state {
		case ProofProven:
			return ProofProven, nil
		case ProofUnproven:
			sawUnproven = true
		}
	}
	if sawUnproven {
		return ProofUnproven, nil
	}
	return ProofViolatedWithWitness, nil
}

// sortedContains reports whether v occurs in sorted, which must already be
// canonical-lexical-order (every exemption Scope dimension is normalized at
// decode; every DimensionProof set is validated sorted-unique).
func sortedContains(sorted []string, v string) bool {
	i := sort.SearchStrings(sorted, v)
	return i < len(sorted) && sorted[i] == v
}

// --- disposition arm (authority design §8/§9, ledger SI-114/SI-115) --------

// semanticClaimIdentity is one claim's complete canonical witness identity
// (SI-114's "normalized claim identity set"): its id, content digest, and
// closed §6 category — the same three-field identity ValidateJudgeResult
// (semantic.go) already uses to prove a judge finding names a real claim
// this exact input carried, applied here to prove a disposition's witness
// names the same claims the current semantic input does. No partial subset
// (id-only, digest-only) is ever compared.
type semanticClaimIdentity struct {
	id       string
	digest   string
	category string
}

// proseClaimIdentities derives the current semantic input's claim identity
// set from its sorted-unique prose claims.
func proseClaimIdentities(claims []contextcompile.ProseClaim) []semanticClaimIdentity {
	out := make([]semanticClaimIdentity, len(claims))
	for i, c := range claims {
		out[i] = semanticClaimIdentity{id: c.ID, digest: c.TextDigest, category: c.Category}
	}
	return out
}

// witnessClaimIdentities derives a disposition's own committed claim
// identity set the same way, from its decoded witness.
func witnessClaimIdentities(claims []policyartifact.SemanticClaimWitness) []semanticClaimIdentity {
	out := make([]semanticClaimIdentity, len(claims))
	for i, c := range claims {
		out[i] = semanticClaimIdentity{id: c.ID, digest: c.Digest, category: c.Category}
	}
	return out
}

// ResolveDispositionAuthority derives one DispositionResolution for every
// disposition in in.Dispositions against the current semanticInput
// (authority design §8, ledger SI-114/SI-115) — unlike the exemption arm, a
// disposition governs the complete semantic input and has no narrower scope
// to be inapplicable to, so nothing here is omitted; a disposition whose
// committed witness does not currently match simply resolves as a definite
// mismatch rather than being silently dropped. primary/challenger, when
// supplied, must already have been validated against this exact
// semanticInput (ValidatedExchange.RecordDigest equal to its digest) — a
// caller that hands a judge exchange validated against a DIFFERENT semantic
// input is a cross-snapshot operand defect (authority design §13 item 2),
// reported operationally, never silently accepted. Cache presence is never
// required or consulted here (§8: "Cache presence is never required to load
// or validate a disposition") — a disposition's own JudgmentProvenance
// citation is informational only and is never read by this function.
func ResolveDispositionAuthority(in AuthorityInput, semanticInput SemanticInput, primary, challenger *ValidatedExchange) ([]DispositionResolution, error) {
	if err := validateEvaluatedOn("authority_input.evaluated_on", in.EvaluatedOn); err != nil {
		return nil, err
	}
	if err := validateSemanticInput(semanticInput); err != nil {
		return nil, fmt.Errorf("policyconflict: resolve disposition authority: %w", err)
	}
	currentDigest, err := semanticInputDigest(semanticInput)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: resolve disposition authority: %w", err)
	}
	if primary != nil && primary.RecordDigest != currentDigest {
		return nil, fmt.Errorf("policyconflict: resolve disposition authority: primary exchange record digest %s does not match the current semantic input digest %s (cross-snapshot operand)", primary.RecordDigest, currentDigest)
	}
	if challenger != nil && challenger.RecordDigest != currentDigest {
		return nil, fmt.Errorf("policyconflict: resolve disposition authority: challenger exchange record digest %s does not match the current semantic input digest %s (cross-snapshot operand)", challenger.RecordDigest, currentDigest)
	}

	currentClaims := proseClaimIdentities(semanticInput.Claims)

	out := make([]DispositionResolution, 0, len(in.Dispositions))
	seen := make(map[string]policyartifact.Disposition, len(in.Dispositions))
	for _, d := range in.Dispositions {
		if prev, ok := seen[d.ID]; ok {
			if !reflect.DeepEqual(prev, d) {
				return nil, fmt.Errorf("policyconflict: resolve disposition authority: two different disposition artifacts share id %q", d.ID)
			}
			continue
		}
		seen[d.ID] = d

		res, err := resolveDisposition(in, currentDigest, currentClaims, semanticInput.Exemptions, d)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// resolveDisposition derives d's DispositionResolution against the current
// operands. Match/Freshness/Scope share one exact-complete-input state
// (SI-114/SI-115: "Freshness and Scope carry the SAME state as that exact
// complete-input match... no partial component match is favorable"):
// proven only when input_id, target digest, normalized claim identities,
// and normalized exemption identities all agree; a definite mismatch in any
// one of the four is violated-with-witness. A judge-result disposition's
// Bound follows that same state; a human-fallback disposition's Bound uses
// the injected expiry/review-condition rule instead.
func resolveDisposition(in AuthorityInput, currentDigest string, currentClaims []semanticClaimIdentity, currentExemptions []policyartifact.SemanticExemptionWitness, d policyartifact.Disposition) (DispositionResolution, error) {
	matchState := ProofProven
	switch {
	case d.Witness.InputID != currentDigest:
		matchState = ProofViolatedWithWitness
	case d.Witness.TargetDigest != in.TargetDigest:
		matchState = ProofViolatedWithWitness
	case !reflect.DeepEqual(currentClaims, witnessClaimIdentities(d.Witness.Claims)):
		matchState = ProofViolatedWithWitness
	case !reflect.DeepEqual(currentExemptions, d.Witness.Exemptions):
		matchState = ProofViolatedWithWitness
	}

	resolution := AuthorityResolution{Match: matchState, Freshness: matchState, Scope: matchState}

	switch d.Origin {
	case policyartifact.DispositionJudgeResult:
		resolution.Bound = matchState
	case policyartifact.DispositionHumanFallback:
		bound, err := resolveBound(d.Expiry, d.ReviewCondition, in.EvaluatedOn)
		if err != nil {
			return DispositionResolution{}, fmt.Errorf("policyconflict: resolve disposition authority: disposition %q: %w", d.ID, err)
		}
		resolution.Bound = bound
	default:
		return DispositionResolution{}, fmt.Errorf("policyconflict: resolve disposition authority: disposition %q: unknown origin %q", d.ID, d.Origin)
	}

	authState, err := resolveAuthorization(in.Profile, transitionDispositionApproval, in.Actors, d.Approvals)
	if err != nil {
		return DispositionResolution{}, err
	}
	resolution.Authorization = authState

	digest, err := d.Digest()
	if err != nil {
		return DispositionResolution{}, fmt.Errorf("policyconflict: resolve disposition authority: disposition %q: %w", d.ID, err)
	}

	return DispositionResolution{ID: d.ID, Digest: digest, Conclusion: d.Conclusion, Resolution: resolution}, nil
}
