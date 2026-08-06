package policyauthority

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// EffectivePolicySchema is the canonical machine-proof schema identifier
// for Resolve's output (co-3: canonical manifests are digest-bound and
// schema-tagged like every other proof shape in the store).
const EffectivePolicySchema = "verdi.effective-policy/v1"

// refinableOperators is the closed set of operators DC-3's narrow-only
// overlay refinement actually admits. equals, not-equals, same-principal,
// different-principal, path-read, and path-write are excluded: there is
// no narrower value for an exact-match or identity-relation operator, and
// identity semantics belong to the governance kernel (DC-22), never a
// second policy-authority interpretation.
var refinableOperators = map[policyartifact.Operator]bool{
	policyartifact.OpAllowedValues:   true,
	policyartifact.OpRequiredValues:  true,
	policyartifact.OpForbiddenValues: true,
	policyartifact.OpMinimum:         true,
	policyartifact.OpMaximum:         true,
}

// EffectivePolicy is the one canonical effective-policy shape (DC-23):
// every policy's claims after narrow-only overlay refinement, plus every
// exemption recorded as a carried fact (never evaluated — lifecycle and
// conflict semantics are later-wave work). Only Resolve produces a value
// whose Digest() succeeds; a hand-built or post-Resolve-mutated value
// fails the seal check the same way every internal/policyartifact and
// internal/governanceprincipal value does.
type EffectivePolicy struct {
	Schema             string                 `json:"schema"`
	ConstitutionDigest string                 `json:"constitution_digest"`
	ProfileID          string                 `json:"profile_id"`
	ProfileDigest      string                 `json:"profile_digest"`
	Policies           []EffectivePolicyEntry `json:"policies"`
	Exemptions         []EffectiveExemption   `json:"exemptions"`

	seal string
}

// EffectivePolicyEntry is one policy's post-refinement content: its
// identity and digest, effective claims, author-ordered instructions, and
// typed payloads.
type EffectivePolicyEntry struct {
	PolicyID     string                            `json:"policy_id"`
	PolicyDigest string                            `json:"policy_digest"`
	Claims       []EffectiveClaim                  `json:"claims"`
	Instructions []string                          `json:"instructions"`
	Payloads     map[string]policyartifact.Payload `json:"payloads"`
}

// EffectiveClaim is one base claim plus the narrow-only refinements that
// apply inside a strictly bounded part of it: every field the base claim
// carries (including its UNREFINED Values/Bound, which still govern
// everywhere the claim's own scope reaches), the base claim's own digest
// (the exact witness identity exemptions bind to, DC-8), and one
// ScopedRefinement per distinct overlay scope. A refinement never
// rewrites the base operand: DC-3 admits an overlay only "where the
// governing policy declares the subject overridable AND its scope is
// within the permitted refinement boundary", so a narrowing proven for
// one boundary is authority for that boundary alone.
type EffectiveClaim struct {
	ID              string                  `json:"id"`
	Family          policyartifact.Family   `json:"family"`
	Operator        policyartifact.Operator `json:"operator"`
	Subject         string                  `json:"subject"`
	Scope           policyartifact.Scope    `json:"scope"`
	Overridable     bool                    `json:"overridable"`
	Values          []string                `json:"values"`
	Bound           *int                    `json:"bound,omitempty"`
	BaseClaimDigest string                  `json:"base_claim_digest"`
	Refinements     []ScopedRefinement      `json:"refinements"`
}

// ScopedRefinement is the narrowed operand that applies within ONE
// overlay scope, together with the sorted ids of every overlay that
// contributed to it. Entries group by canonically IDENTICAL scope (all
// four dimensions equal as sorted sets); within a group the narrowing
// rules combine the contributions (intersection for allowed-values,
// union for required/forbidden-values, the strictest bound for
// minimum/maximum), and each contribution is proven to narrow the BASE
// claim.
//
// Refinements at DISTINCT scopes are RECORDED SEPARATELY, never merged:
// deciding whether two different scopes overlap, nest, or are disjoint
// requires the scope-comparison semantics that are Wave-3 work and that
// this unit does not own. Silently combining them would either invent
// that comparison or apply one boundary's narrowing outside it, so CO-1
// requires the honest recorded form instead. Values is present only for
// the value operators and Bound only for minimum/maximum: an operator
// never carries an operand kind it cannot take.
type ScopedRefinement struct {
	Scope    policyartifact.Scope `json:"scope"`
	Values   []string             `json:"values,omitempty"`
	Bound    *int                 `json:"bound,omitempty"`
	Overlays []string             `json:"overlays"`
}

// EffectiveExemption is one exemption carried into the effective policy
// as a recorded fact: no lifecycle or conflict disposition is computed
// here (that is later-wave gate work over these same recorded facts).
type EffectiveExemption struct {
	ExemptionID     string                    `json:"exemption_id"`
	Digest          string                    `json:"digest"`
	Witnesses       []policyartifact.Witness  `json:"witnesses"`
	Scope           policyartifact.Scope      `json:"scope"`
	Expiry          string                    `json:"expiry,omitempty"`
	ReviewCondition string                    `json:"review_condition,omitempty"`
	Owners          []string                  `json:"owners"`
	Approvals       []policyartifact.Approval `json:"approvals"`
}

// Resolve computes the one canonical effective policy from a Store that
// Load produced. It re-verifies every input artifact's seal (constitution,
// every policy, every overlay, every exemption, the selected profile)
// before using it, applies DC-3's narrow-only overlay refinement to every
// claim, and mints the result's own anti-forgery seal.
//
// The sealed marker proves the Store came from Load, but not that its
// exported maps still hold what Load cross-validated: they are ordinary
// maps of pointers to individually sealed artifacts, so an entry can be
// inserted, deleted, or swapped afterwards without disturbing any
// artifact's own seal. Resolve therefore re-proves the whole store's
// composition before resolving anything (CO-1: composition that is merely
// assumed is never a pass).
func Resolve(s *Store) (*EffectivePolicy, error) {
	if s == nil || !s.sealed {
		return nil, fmt.Errorf("policyauthority: Resolve requires a Store produced by Load")
	}
	if err := crossValidate(s); err != nil {
		return nil, err
	}

	conDigest, err := s.Constitution.Digest()
	if err != nil {
		return nil, fmt.Errorf("policyauthority: constitution: %w", err)
	}

	profile, ok := s.Profiles[s.Constitution.SelectedProfile]
	if !ok {
		return nil, fmt.Errorf("policyauthority: selected profile %q is not a loaded stored profile", s.Constitution.SelectedProfile)
	}
	profDigest, err := profile.Profile.Digest()
	if err != nil {
		return nil, fmt.Errorf("policyauthority: profile %s: %w", profile.ID, err)
	}

	overlaysByPolicy := map[string][]*policyartifact.Overlay{}
	for _, oid := range sortedKeys(s.Overlays) {
		o := s.Overlays[oid]
		if _, err := o.Digest(); err != nil {
			return nil, fmt.Errorf("policyauthority: overlay %s: %w", oid, err)
		}
		overlaysByPolicy[o.Refines] = append(overlaysByPolicy[o.Refines], o)
	}

	policyIDs := sortedKeys(s.Policies)
	entries := make([]EffectivePolicyEntry, 0, len(policyIDs))
	for _, pid := range policyIDs {
		p := s.Policies[pid]
		pDigest, err := p.Digest()
		if err != nil {
			return nil, fmt.Errorf("policyauthority: policy %s: %w", pid, err)
		}

		claims := make([]EffectiveClaim, 0, len(p.Claims))
		for _, c := range p.Claims {
			ec, err := refineClaim(pid, c, overlaysByPolicy[pid])
			if err != nil {
				return nil, err
			}
			claims = append(claims, ec)
		}

		entries = append(entries, EffectivePolicyEntry{
			PolicyID:     pid,
			PolicyDigest: pDigest,
			Claims:       claims,
			Instructions: append([]string{}, p.Instructions...),
			Payloads:     p.Payloads,
		})
	}

	exemptionIDs := sortedKeys(s.Exemptions)
	exemptions := make([]EffectiveExemption, 0, len(exemptionIDs))
	for _, eid := range exemptionIDs {
		e := s.Exemptions[eid]
		eDigest, err := e.Digest()
		if err != nil {
			return nil, fmt.Errorf("policyauthority: exemption %s: %w", eid, err)
		}
		exemptions = append(exemptions, EffectiveExemption{
			ExemptionID:     eid,
			Digest:          eDigest,
			Witnesses:       append([]policyartifact.Witness{}, e.Witnesses...),
			Scope:           e.Scope,
			Expiry:          e.Expiry,
			ReviewCondition: e.ReviewCondition,
			Owners:          append([]string{}, e.Owners...),
			Approvals:       append([]policyartifact.Approval{}, e.Approvals...),
		})
	}

	ep := &EffectivePolicy{
		Schema:             EffectivePolicySchema,
		ConstitutionDigest: conDigest,
		ProfileID:          profile.ID,
		ProfileDigest:      profDigest,
		Policies:           entries,
		Exemptions:         exemptions,
	}
	seal, err := canonjson.Digest(ep)
	if err != nil {
		return nil, fmt.Errorf("policyauthority: sealing effective policy: %w", err)
	}
	ep.seal = seal
	return ep, nil
}

// scopeGroup accumulates every refinement contribution that shares one
// canonically identical overlay scope. It starts from the BASE operand,
// so a group's combined operand is the narrowing of the base by exactly
// the overlays that reach that scope — never by an overlay bounded
// somewhere else.
type scopeGroup struct {
	scope    policyartifact.Scope
	values   []string
	bound    *int
	overlays []string
}

// refineClaim records every applicable overlay's narrow-only refinement
// against c, grouped by the overlay scope that bounds it, and returns
// the effective claim.
func refineClaim(policyID string, c policyartifact.Claim, overlays []*policyartifact.Overlay) (EffectiveClaim, error) {
	baseDigest, err := policyartifact.ClaimDigest(c)
	if err != nil {
		return EffectiveClaim{}, fmt.Errorf("policyauthority: policy %s claim %s: digesting base claim: %w", policyID, c.ID, err)
	}

	// Every slice below is copied, never aliased from the Store, and
	// starts as an explicit empty slice, never nil: a claim with no
	// contributing overlay (or a minimum/maximum claim, which never
	// touches values at all) must still canonicalize its zero-value
	// fields as [] like every other semantic set in this store, not as
	// JSON null.
	ec := EffectiveClaim{
		ID:              c.ID,
		Family:          c.Family,
		Operator:        c.Operator,
		Subject:         c.Subject,
		Scope:           copyScope(c.Scope),
		Overridable:     c.Overridable,
		Values:          append([]string{}, c.Values...),
		BaseClaimDigest: baseDigest,
		Refinements:     []ScopedRefinement{},
	}
	if c.Bound != nil {
		b := *c.Bound
		ec.Bound = &b
	}

	groups := map[string]*scopeGroup{}
	var groupKeys []string

	for _, o := range overlays {
		for _, r := range o.Refinements {
			if r.Claim != c.ID {
				continue
			}
			// Defense in depth: Load's cross-validation already rejects a
			// refinement of a non-overridable claim, and Resolve re-proves
			// that composition, but the authority rule itself (DC-3: an
			// overlay may refine ONLY a declared-overridable surface) is
			// restated here so no future caller of refineClaim can reach a
			// narrowing without it.
			if !c.Overridable {
				return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: claim %s on policy %s is not overridable", o.ID, c.ID, policyID)
			}
			if !refinableOperators[c.Operator] {
				return EffectiveClaim{}, notRefinableError(o.ID, policyID, c)
			}
			if err := scopeSubset(c.Scope, o.Scope); err != nil {
				return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s refining policy %s claim %s: %w", o.ID, policyID, c.ID, err)
			}

			key, err := canonjson.Marshal(o.Scope)
			if err != nil {
				return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: canonicalizing scope: %w", o.ID, err)
			}
			g, ok := groups[string(key)]
			if !ok {
				g = &scopeGroup{
					scope:    copyScope(o.Scope),
					values:   append([]string{}, c.Values...),
					overlays: []string{},
				}
				if c.Bound != nil {
					b := *c.Bound
					g.bound = &b
				}
				groups[string(key)] = g
				groupKeys = append(groupKeys, string(key))
			}
			g.overlays = append(g.overlays, o.ID)

			switch c.Operator {
			case policyartifact.OpAllowedValues:
				if !isSubset(r.Values, c.Values) {
					return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: allowed-values refinement of policy %s claim %s is not a subset of the base values %v", o.ID, policyID, c.ID, c.Values)
				}
				g.values = intersect(g.values, r.Values)

			case policyartifact.OpRequiredValues, policyartifact.OpForbiddenValues:
				if !isSubset(c.Values, r.Values) {
					return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: %s refinement of policy %s claim %s drops a base value (must be a superset of %v)", o.ID, c.Operator, policyID, c.ID, c.Values)
				}
				g.values = union(g.values, r.Values)

			case policyartifact.OpMinimum:
				if r.Bound == nil || *r.Bound < *c.Bound {
					return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: minimum refinement of policy %s claim %s must be >= the base bound %d", o.ID, policyID, c.ID, *c.Bound)
				}
				if *r.Bound > *g.bound {
					b := *r.Bound
					g.bound = &b
				}

			case policyartifact.OpMaximum:
				if r.Bound == nil || *r.Bound > *c.Bound {
					return EffectiveClaim{}, fmt.Errorf("policyauthority: overlay %s: maximum refinement of policy %s claim %s must be <= the base bound %d", o.ID, policyID, c.ID, *c.Bound)
				}
				if *r.Bound < *g.bound {
					b := *r.Bound
					g.bound = &b
				}
			}
		}
	}

	for _, key := range groupKeys {
		g := groups[key]
		sort.Strings(g.overlays)
		sr := ScopedRefinement{Scope: g.scope, Overlays: g.overlays}
		switch c.Operator {
		case policyartifact.OpAllowedValues:
			if len(g.values) == 0 {
				return EffectiveClaim{}, fmt.Errorf("policyauthority: policy %s claim %s: overlays %v narrow allowed-values to an empty intersection", policyID, c.ID, g.overlays)
			}
			sr.Values = g.values
		case policyartifact.OpRequiredValues, policyartifact.OpForbiddenValues:
			sr.Values = g.values
		case policyartifact.OpMinimum, policyartifact.OpMaximum:
			sr.Bound = g.bound
		}
		ec.Refinements = append(ec.Refinements, sr)
	}
	if err := sortRefinementsByContent(ec.Refinements); err != nil {
		return EffectiveClaim{}, fmt.Errorf("policyauthority: policy %s claim %s: %w", policyID, c.ID, err)
	}
	return ec, nil
}

// notRefinableError is the ONE named error both Load's structural
// cross-validation and Resolve's narrowing raise for an overlay
// targeting an operator DC-3's narrow-only refinement does not admit, so
// a store can never Load clean and then fail to Resolve on the same
// fact.
func notRefinableError(overlayID, policyID string, c policyartifact.Claim) error {
	return fmt.Errorf("policyauthority: overlay %s: claim %s (policy %s, operator %s) is not refinable (specificity alone never changes authority)", overlayID, c.ID, policyID, c.Operator)
}

// sortRefinementsByContent orders refinements by the canonical JSON
// encoding of each entry — the "complete field content" tiebreak this
// store uses for set entries that carry no single stable id (mirroring
// internal/governanceprincipal's sortByContent), so the output carries
// no incidental ordering (CO-3).
func sortRefinementsByContent(rs []ScopedRefinement) error {
	keys := make([]string, len(rs))
	for i, r := range rs {
		b, err := canonjson.Marshal(r)
		if err != nil {
			return fmt.Errorf("canonicalizing refinement for ordering: %w", err)
		}
		keys[i] = string(b)
	}
	idx := make([]int, len(rs))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool { return keys[idx[i]] < keys[idx[j]] })
	sorted := make([]ScopedRefinement, len(rs))
	for i, j := range idx {
		sorted[i] = rs[j]
	}
	copy(rs, sorted)
	return nil
}

// copyScope returns a deep copy of s whose dimensions share no backing
// array with the caller's value, with every dimension an explicit empty
// set rather than nil.
func copyScope(s policyartifact.Scope) policyartifact.Scope {
	return policyartifact.Scope{
		Phases:       append([]string{}, s.Phases...),
		Environments: append([]string{}, s.Environments...),
		Paths:        append([]string{}, s.Paths...),
		Refs:         append([]string{}, s.Refs...),
	}
}

// scopeSubset proves overlay is a provable subset of claim on every
// dimension (CO-1: unknown applicability can never silently pass). A
// claim dimension that is [] (universal) admits any overlay dimension. A
// claim dimension that is nonempty requires the overlay dimension to be
// nonempty and a subset by exact string membership.
func scopeSubset(claim, overlay policyartifact.Scope) error {
	dims := []struct {
		name           string
		claim, overlay []string
	}{
		{"phases", claim.Phases, overlay.Phases},
		{"environments", claim.Environments, overlay.Environments},
		{"paths", claim.Paths, overlay.Paths},
		{"refs", claim.Refs, overlay.Refs},
	}
	for _, d := range dims {
		if len(d.claim) == 0 {
			continue
		}
		if len(d.overlay) == 0 {
			return fmt.Errorf("overlay scope %s is universal but the claim's %s scope is not (overlay would claim more scope than the claim has)", d.name, d.name)
		}
		if !isSubset(d.overlay, d.claim) {
			return fmt.Errorf("overlay scope %s %v is not a provable subset of the claim's %s scope %v", d.name, d.overlay, d.name, d.claim)
		}
	}
	return nil
}

// isSubset reports whether every member of a is a member of b.
func isSubset(a, b []string) bool {
	set := toSet(b)
	for _, v := range a {
		if !set[v] {
			return false
		}
	}
	return true
}

// intersect returns the sorted, deduplicated intersection of a and b.
func intersect(a, b []string) []string {
	bs := toSet(b)
	seen := map[string]bool{}
	out := []string{}
	for _, v := range a {
		if bs[v] && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	sort.Strings(out)
	return out
}

// union returns the sorted, deduplicated union of a and b.
func union(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range append(append([]string(nil), a...), b...) {
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	sort.Strings(out)
	return out
}

// Digest returns ep's canonical content address after proving ep is
// unmodified Resolve output.
func (ep *EffectivePolicy) Digest() (string, error) {
	if ep == nil || ep.seal == "" {
		return "", fmt.Errorf("policyauthority: effective policy was not produced by Resolve")
	}
	d, err := canonjson.Digest(ep)
	if err != nil {
		return "", err
	}
	if d != ep.seal {
		return "", fmt.Errorf("policyauthority: effective policy was modified after Resolve")
	}
	return ep.seal, nil
}
