package policyauthority

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// knownPolicyDirs is the constitution store's closed set of directories
// directly under .verdi/policy/ (policyartifact.ClassifyPolicyPath's own
// grammar, restated here so Load can fail closed on an unexpected
// directory even when it carries no files at all — the classifier alone
// only rejects unrecognized FILES).
var knownPolicyDirs = map[string]bool{
	policyartifact.DirPolicies:     true,
	policyartifact.DirOverlays:     true,
	policyartifact.DirExemptions:   true,
	policyartifact.DirDispositions: true,
	policyartifact.DirProfiles:     true,
	policyartifact.DirProjections:  true,
}

// Store is a fully loaded, fully cross-validated constitution store: the
// ONE representation Resolve accepts (DC-23). Only Load produces a Store
// that satisfies Resolve's gate — the unexported sealed marker is never
// settable from outside this package, so a hand-built or zero-value Store
// fails closed rather than silently resolving.
//
// Every exported field is a READ-ONLY view of what Load decoded and
// cross-validated. The maps are ordinary Go maps of pointers to
// individually sealed artifacts: mutating one does not disturb any
// artifact's own seal, so a mutated Store is not detectable from the
// artifacts alone. Resolve consequently re-proves the store's whole
// composition (crossValidate) before it resolves anything; a caller that
// inserts, deletes, or swaps an entry gets a named failure there, never a
// silently different effective policy.
type Store struct {
	Root         string
	Constitution *policyartifact.Constitution
	// Policies, Overlays, Exemptions, and Dispositions are keyed by their
	// full kinded artifact id ("policy/<name>", "policy-overlay/<name>",
	// "policy-exemption/<name>", "policy-disposition/<name>").
	Policies     map[string]*policyartifact.Policy
	Overlays     map[string]*policyartifact.Overlay
	Exemptions   map[string]*policyartifact.Exemption
	Dispositions map[string]*policyartifact.Disposition
	// Profiles is keyed by the profile's own kernel id (no kind prefix —
	// governanceprincipal's id grammar, matching Constitution.SelectedProfile).
	Profiles map[string]*policyartifact.StoredProfile

	sealed bool
}

// Load walks root/.verdi/policy/, strict-decodes every artifact through
// internal/policyartifact, and cross-validates the store as a whole. An
// absent .verdi/policy/ returns ErrNotAdopted; a present policy/ with no
// constitution.md returns ErrIncompleteAdoption; every other structural
// or cross-validation failure is returned naming the offending artifact
// and field.
func Load(root string) (*Store, error) {
	policyDir := filepath.Join(root, ".verdi", "policy")
	info, err := os.Lstat(policyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("policyauthority: %w", ErrNotAdopted)
		}
		return nil, fmt.Errorf("policyauthority: statting .verdi/policy: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("policyauthority: .verdi/policy is a symlink; the constitution store root must be a directory")
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("policyauthority: .verdi/policy exists but is not a directory")
	}

	fileRels, err := walkPolicyDir(policyDir)
	if err != nil {
		return nil, err
	}

	s := &Store{
		Root:         root,
		Policies:     map[string]*policyartifact.Policy{},
		Overlays:     map[string]*policyartifact.Overlay{},
		Exemptions:   map[string]*policyartifact.Exemption{},
		Dispositions: map[string]*policyartifact.Disposition{},
		Profiles:     map[string]*policyartifact.StoredProfile{},
	}
	var profileRels []string

	for _, rel := range fileRels {
		kind, name, err := policyartifact.ClassifyPolicyPath(rel)
		if err != nil {
			return nil, fmt.Errorf("policyauthority: %w", err)
		}
		data, err := os.ReadFile(filepath.Join(policyDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("policyauthority: reading %s: %w", rel, err)
		}

		switch kind {
		case policyartifact.KindConstitution:
			c, err := policyartifact.DecodeConstitution(data)
			if err != nil {
				return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
			}
			s.Constitution = c

		case policyartifact.KindPolicy:
			p, err := policyartifact.DecodePolicy(data)
			if err != nil {
				return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
			}
			if p.Name() != name {
				return nil, fmt.Errorf("policyauthority: %s: filename stem %q does not match policy name %q", rel, name, p.Name())
			}
			s.Policies[p.ID] = p

		case policyartifact.KindOverlay:
			o, err := policyartifact.DecodeOverlay(data)
			if err != nil {
				return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
			}
			if o.Name() != name {
				return nil, fmt.Errorf("policyauthority: %s: filename stem %q does not match overlay name %q", rel, name, o.Name())
			}
			s.Overlays[o.ID] = o

		case policyartifact.KindExemption:
			e, err := policyartifact.DecodeExemption(data)
			if err != nil {
				return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
			}
			if e.Name() != name {
				return nil, fmt.Errorf("policyauthority: %s: filename stem %q does not match exemption name %q", rel, name, e.Name())
			}
			s.Exemptions[e.ID] = e

		case policyartifact.KindDisposition:
			d, err := policyartifact.DecodeDisposition(data)
			if err != nil {
				return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
			}
			if d.Name() != name {
				return nil, fmt.Errorf("policyauthority: %s: filename stem %q does not match disposition name %q", rel, name, d.Name())
			}
			s.Dispositions[d.ID] = d

		case policyartifact.KindProfileStorage:
			// Profiles decode against the constitution's governance
			// catalog (DC-20), so they wait until the constitution itself
			// is decoded; stash the relative path and revisit below.
			profileRels = append(profileRels, rel)

		case policyartifact.KindProjectionManifest:
			// Generated OUTPUT, never authority input (DC-1: harness
			// instruction files and their manifests are projections of
			// the constitution). internal/instructionprojection generates
			// and verifies them against the resolved authority; loading
			// one here as authority would make a projection load-bearing,
			// exactly what DC-1 forbids. Admitted, skipped.

		default:
			return nil, fmt.Errorf("policyauthority: %s: unreachable: unhandled artifact kind %q", rel, kind)
		}
	}

	if s.Constitution == nil {
		return nil, fmt.Errorf("policyauthority: %w", ErrIncompleteAdoption)
	}

	catalog, err := s.Constitution.GovernanceCatalog()
	if err != nil {
		return nil, fmt.Errorf("policyauthority: constitution governance catalog: %w", err)
	}
	for _, rel := range profileRels {
		_, name, err := policyartifact.ClassifyPolicyPath(rel)
		if err != nil {
			return nil, fmt.Errorf("policyauthority: %w", err)
		}
		data, err := os.ReadFile(filepath.Join(policyDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("policyauthority: reading %s: %w", rel, err)
		}
		sp, err := policyartifact.DecodeStoredProfile(data, catalog)
		if err != nil {
			return nil, fmt.Errorf("policyauthority: decoding %s: %w", rel, err)
		}
		if sp.ID != name {
			return nil, fmt.Errorf("policyauthority: %s: filename stem %q does not match profile id %q", rel, name, sp.ID)
		}
		s.Profiles[sp.ID] = sp
	}

	if err := crossValidate(s); err != nil {
		return nil, err
	}

	s.sealed = true
	return s, nil
}

// walkPolicyDir returns the sorted, root-relative slash paths of every
// FILE under policyDir, failing closed on any directory directly or
// transitively under policyDir whose own path is not one of the five
// known directory names (an empty unrecognized directory carries no file
// for policyartifact.ClassifyPolicyPath to reject, so Load must check
// directories itself).
func walkPolicyDir(policyDir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(policyDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == policyDir {
			return nil
		}
		rel, rerr := filepath.Rel(policyDir, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		// A symlink is never store content. filepath.WalkDir does not
		// follow links, so a linked artifact would otherwise be read
		// through its target: content outside the Git-governed store, or
		// content that resolves differently on another checkout, would
		// enter the loaded authority. Fail closed naming the entry (the
		// unexpected-directory check below is the same posture for dirs).
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("policyauthority: %q under .verdi/policy/ is a symlink; the constitution store carries only regular files", rel)
		}
		if d.IsDir() {
			if !knownPolicyDirs[rel] {
				return fmt.Errorf("policyauthority: unexpected directory %q under .verdi/policy/ (known: %s, %s, %s, %s, %s, %s)",
					rel, policyartifact.DirPolicies, policyartifact.DirOverlays, policyartifact.DirExemptions, policyartifact.DirDispositions, policyartifact.DirProfiles, policyartifact.DirProjections)
			}
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("policyauthority: walking .verdi/policy: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

// crossValidate proves the store's artifacts agree with each other, not
// merely with their own self-contained grammar: selected-profile
// resolution, claim-subject registration, scope-environment
// registration, overlay refinement targets and operand-kind agreement,
// exemption witness freshness, approval-role registration, and
// payload-kind uniqueness across policies. Every failure names the
// offending artifact and field (co-1's three-valued honesty: a cross-
// validation gap is never a silent pass).
func crossValidate(s *Store) error {
	if err := checkMapKeyIdentity(s); err != nil {
		return err
	}

	if _, ok := s.Profiles[s.Constitution.SelectedProfile]; !ok {
		return fmt.Errorf("policyauthority: constitution selected_profile %q does not resolve to a loaded stored profile", s.Constitution.SelectedProfile)
	}

	environments := toSet(s.Constitution.Environments)
	roles := toSet(s.Constitution.Catalog.Roles)

	for _, pid := range sortedKeys(s.Policies) {
		p := s.Policies[pid]
		if err := checkScopeEnvironments("policy "+pid+" scope", p.Scope, environments); err != nil {
			return err
		}
		for _, c := range p.Claims {
			if !s.Constitution.Subjects.Has(c.Family, c.Subject) {
				return fmt.Errorf("policyauthority: policy %s claim %s: subject %q is not registered for family %q in the constitution subject catalog", pid, c.ID, c.Subject, c.Family)
			}
			if err := checkScopeEnvironments(fmt.Sprintf("policy %s claim %s scope", pid, c.ID), c.Scope, environments); err != nil {
				return err
			}
		}
	}

	if err := checkDuplicatePayloadKinds(s.Policies); err != nil {
		return err
	}

	for _, oid := range sortedKeys(s.Overlays) {
		o := s.Overlays[oid]
		if err := checkScopeEnvironments("overlay "+oid+" scope", o.Scope, environments); err != nil {
			return err
		}
		target, ok := s.Policies[o.Refines]
		if !ok {
			return fmt.Errorf("policyauthority: overlay %s refines %q, which is not a loaded policy", oid, o.Refines)
		}
		for _, r := range o.Refinements {
			claim, ok := target.Claim(r.Claim)
			if !ok {
				return fmt.Errorf("policyauthority: overlay %s: refinement targets claim %q, which does not exist on policy %s", oid, r.Claim, o.Refines)
			}
			if !claim.Overridable {
				return fmt.Errorf("policyauthority: overlay %s: claim %s on policy %s is not overridable", oid, claim.ID, o.Refines)
			}
			if err := checkOperandKind(oid, o.Refines, claim, r); err != nil {
				return err
			}
		}
	}

	for _, eid := range sortedKeys(s.Exemptions) {
		e := s.Exemptions[eid]
		if err := checkScopeEnvironments("exemption "+eid+" scope", e.Scope, environments); err != nil {
			return err
		}
		for _, w := range e.Witnesses {
			pol, ok := s.Policies[w.Policy]
			if !ok {
				return fmt.Errorf("policyauthority: exemption %s witness: policy %q is not loaded", eid, w.Policy)
			}
			claim, ok := pol.Claim(w.Claim)
			if !ok {
				return fmt.Errorf("policyauthority: exemption %s witness: claim %q does not exist on policy %s", eid, w.Claim, w.Policy)
			}
			current, err := policyartifact.ClaimDigest(claim)
			if err != nil {
				return fmt.Errorf("policyauthority: exemption %s witness: digesting current claim %s: %w", eid, w.Claim, err)
			}
			if current != w.ClaimDigest {
				return fmt.Errorf("policyauthority: exemption %s witness %s/%s: stale witness (claim_digest %s does not match the current claim digest %s)", eid, w.Policy, w.Claim, w.ClaimDigest, current)
			}
		}
		for _, a := range e.Approvals {
			if !roles[a.Role] {
				return fmt.Errorf("policyauthority: exemption %s approval: role %q is not a member of the constitution catalog's roles", eid, a.Role)
			}
		}
	}

	return nil
}

// checkMapKeyIdentity proves every map KEY equals the identity of the
// artifact stored under it. It runs first in crossValidate, before any
// check that selects an artifact by key.
//
// Every artifact in a Store is individually sealed, but the MAPS are
// not: rekeying an entry, or pointing a second key at an existing value,
// leaves every seal valid, so nothing a per-artifact check can see is
// wrong. Resolve selects artifacts by key throughout — the constitution's
// selected_profile, an overlay's refines target, an exemption's witness
// policy — so an unbound key makes Resolve emit one artifact's content
// under another's name: a wrong effective policy with an intact digest,
// which is worse than a refusal (CO-1). Load itself always keys by the
// decoded identity, so a store that fails here has been mutated after
// Load or hand-built; that is an OPERATIONAL error, matching the seal
// checks' own SI-21 posture, and Resolve re-runs crossValidate precisely
// so post-load mutation is caught rather than resolved.
//
// Maps and keys are both visited in fixed, sorted order so a store with
// several mismatches always names the same one (CO-3).
func checkMapKeyIdentity(s *Store) error {
	if err := checkKeyedIdentities("Policies", s.Policies, func(p *policyartifact.Policy) string { return p.ID }); err != nil {
		return err
	}
	if err := checkKeyedIdentities("Overlays", s.Overlays, func(o *policyartifact.Overlay) string { return o.ID }); err != nil {
		return err
	}
	if err := checkKeyedIdentities("Exemptions", s.Exemptions, func(e *policyartifact.Exemption) string { return e.ID }); err != nil {
		return err
	}
	if err := checkKeyedIdentities("Dispositions", s.Dispositions, func(d *policyartifact.Disposition) string { return d.ID }); err != nil {
		return err
	}
	return checkKeyedIdentities("Profiles", s.Profiles, func(p *policyartifact.StoredProfile) string { return p.ID })
}

// checkKeyedIdentities is checkMapKeyIdentity's one generic body: identity
// extracts the canonical id a value must be filed under (the full kinded
// id for policies, overlays, and exemptions; the bare kernel profile id
// for stored profiles, matching Constitution.SelectedProfile). A nil
// value is its own mismatch: an entry that carries no artifact can never
// be the artifact its key names.
func checkKeyedIdentities[T any](mapName string, m map[string]*T, identity func(*T) string) error {
	for _, key := range sortedKeys(m) {
		v := m[key]
		if v == nil {
			return fmt.Errorf("policyauthority: store %s map key %q holds no artifact; a store key must name the artifact filed under it", mapName, key)
		}
		if got := identity(v); got != key {
			return fmt.Errorf("policyauthority: store %s map key %q holds the artifact whose id is %q; a store key must equal the identity of the artifact filed under it (the store was modified after Load)", mapName, key, got)
		}
	}
	return nil
}

// checkOperandKind proves a refinement's target claim can be refined at
// all, and that the refinement's operand is the kind that claim's
// operator accepts: values for
// equals/not-equals/allowed-values/required-values/forbidden-values/
// path-read/path-write, bound for minimum/maximum. same-principal and
// different-principal claims take neither operand, so a refinement
// targeting them fails on the operand kind; equals, not-equals,
// path-read, and path-write take an operand but admit no NARROWER value
// under DC-3, so a refinement targeting them fails with the same named
// error class Resolve raises (notRefinableError). Load and Resolve must
// agree on this set: a store that loads clean and can never resolve is
// its own silent failure (CO-1).
func checkOperandKind(overlayID, policyID string, claim policyartifact.Claim, r policyartifact.Refinement) error {
	wantsValues := claim.Operator == policyartifact.OpEquals || claim.Operator == policyartifact.OpNotEquals ||
		claim.Operator == policyartifact.OpAllowedValues || claim.Operator == policyartifact.OpRequiredValues ||
		claim.Operator == policyartifact.OpForbiddenValues || claim.Operator == policyartifact.OpPathRead ||
		claim.Operator == policyartifact.OpPathWrite
	wantsBound := claim.Operator == policyartifact.OpMinimum || claim.Operator == policyartifact.OpMaximum

	hasValues := len(r.Values) > 0
	hasBound := r.Bound != nil

	switch {
	case wantsValues && !hasValues:
		return fmt.Errorf("policyauthority: overlay %s: refinement of claim %s (policy %s, operator %s) must carry a values operand", overlayID, claim.ID, policyID, claim.Operator)
	case wantsBound && !hasBound:
		return fmt.Errorf("policyauthority: overlay %s: refinement of claim %s (policy %s, operator %s) must carry a bound operand", overlayID, claim.ID, policyID, claim.Operator)
	case !wantsValues && !wantsBound:
		return fmt.Errorf("policyauthority: overlay %s: claim %s (policy %s, operator %s) accepts no refinement operand", overlayID, claim.ID, policyID, claim.Operator)
	}
	if !refinableOperators[claim.Operator] {
		return notRefinableError(overlayID, policyID, claim)
	}
	return nil
}

// checkDuplicatePayloadKinds fails closed the first time two distinct
// policies register the same payload kind: a payload kind's home must be
// unambiguous (DC-23's single-interpretation posture applied to
// feature-specific payload storage). Policies and kinds are both visited
// in sorted order so a store with several collisions always reports the
// same pair — a canonical failure never depends on Go's randomized map
// iteration order (CO-3).
func checkDuplicatePayloadKinds(policies map[string]*policyartifact.Policy) error {
	seenBy := map[string]string{}
	for _, pid := range sortedKeys(policies) {
		p := policies[pid]
		for _, kind := range sortedKeys(p.Payloads) {
			if owner, dup := seenBy[kind]; dup {
				return fmt.Errorf("policyauthority: payload kind %q is registered by both policy %s and policy %s", kind, owner, pid)
			}
			seenBy[kind] = pid
		}
	}
	return nil
}

// checkScopeEnvironments proves every environment scope.Environments
// names is a member of the constitution's registered environment set.
func checkScopeEnvironments(what string, scope policyartifact.Scope, environments map[string]bool) error {
	for _, e := range scope.Environments {
		if !environments[e] {
			return fmt.Errorf("policyauthority: %s: environment %q is not a member of the constitution's registered environments", what, e)
		}
	}
	return nil
}

func toSet(vs []string) map[string]bool {
	m := make(map[string]bool, len(vs))
	for _, v := range vs {
		m[v] = true
	}
	return m
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
