package contextcompile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// policyStoreDir is the one repo-relative directory the constitution store
// occupies, with its trailing separator. policyartifact owns the grammar
// INSIDE that directory (its Dir* constants below and ClassifyPolicyPath),
// but exports no constant naming the directory itself, so this package
// spells it exactly once here instead of at every store-path call site.
const policyStoreDir = ".verdi/policy/"

// operandArtifactKind maps each closed PolicyEntry* kind to the
// policyartifact ref-prefix kind an authorityOperandCandidate.ID must carry.
var operandArtifactKind = map[string]string{
	PolicyEntryPolicy:    policyartifact.KindPolicy,
	PolicyEntryOverlay:   policyartifact.KindOverlay,
	PolicyEntryExemption: policyartifact.KindExemption,
}

// operandArtifactDir maps each closed PolicyEntry* kind to the
// .verdi/policy/<dir> segment an authorityOperandCandidate.Path must carry.
var operandArtifactDir = map[string]string{
	PolicyEntryPolicy:    policyartifact.DirPolicies,
	PolicyEntryOverlay:   policyartifact.DirOverlays,
	PolicyEntryExemption: policyartifact.DirExemptions,
}

// validateAuthorityOperandCandidate checks one candidate's closed Kind,
// ID/Kind prefix agreement, canonical Path/ID agreement, and Digest grammar.
// The digest is the seal policyartifact's (*Policy|*Overlay|*Exemption).Digest
// returns, so it is checked through the package's shared "sha256:"+64-hex
// validateDigest grammar — the same form manifest digests carry.
// It does not validate Scope: EvaluateApplicability already
// fails closed on an invalid candidate.Scope via policyartifact.Scope.Validate.
func validateAuthorityOperandCandidate(field string, c authorityOperandCandidate) error {
	if err := validatePolicyEntryKind(field+".kind", c.Kind); err != nil {
		return err
	}
	kind, name, ok := strings.Cut(c.ID, "/")
	if !ok || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("contextcompile: %s.id: %q is not a valid <kind>/<name> policy artifact ref", field, c.ID)
	}
	if wantKind := operandArtifactKind[c.Kind]; kind != wantKind {
		return fmt.Errorf("contextcompile: %s.id: %q has kind prefix %q, want %q for candidate kind %q", field, c.ID, kind, wantKind, c.Kind)
	}
	if wantPath := policyStoreDir + operandArtifactDir[c.Kind] + "/" + name + ".md"; c.Path != wantPath {
		return fmt.Errorf("contextcompile: %s.path: %q does not match id %q (want %q)", field, c.Path, c.ID, wantPath)
	}
	return validateDigest(field+".digest", c.Digest)
}

// authorityOperandCandidate is one not-yet-selected authority operand
// candidate (policy, overlay, or exemption) considered for a compile.
type authorityOperandCandidate struct {
	Kind   string
	ID     string
	Path   string
	Digest string
	Scope  policyartifact.Scope
}

// authoritySelection is the result of selecting applicable authority
// operands for a single compile.
type authoritySelection struct {
	Operands    []PolicyOperand
	Selection   instructionprojection.Selection
	Disclosures []DisclosureCode
}

// selectAuthorityOperands evaluates each candidate's applicability against
// the request and target, retaining applicable candidates as capsule policy
// operands and recording retained base-policy IDs in the selection.
func selectAuthorityOperands(candidates []authorityOperandCandidate, request Request, target ResolvedSpec) (authoritySelection, error) {
	if err := validateSpecWholeRef("target.ref", target.Ref); err != nil {
		return authoritySelection{}, fmt.Errorf("contextcompile: select authority operands: %w", err)
	}

	seen := make(map[string]bool, len(candidates))
	for i, candidate := range candidates {
		field := fmt.Sprintf("authority operand candidate[%d]", i)
		if err := validateAuthorityOperandCandidate(field, candidate); err != nil {
			return authoritySelection{}, fmt.Errorf("contextcompile: select authority operands: %w", err)
		}
		key := candidate.Kind + "\x00" + candidate.ID
		if seen[key] {
			return authoritySelection{}, fmt.Errorf("contextcompile: select authority operands: %s: duplicate candidate kind+id %q", field, key)
		}
		seen[key] = true
	}

	environment := ""
	if len(request.Scope.Environments) == 1 {
		environment = request.Scope.Environments[0]
	}

	operands := []PolicyOperand{}
	policyIDs := []string{}
	retainedUnknown := false
	for _, candidate := range candidates {
		result, err := EvaluateApplicability(ApplicabilityInput{
			Policy:        candidate.Scope,
			Request:       request.Scope,
			CandidatePath: target.Path,
			CandidateRef:  target.Ref,
			Phase:         request.Phase,
			Environment:   environment,
		})
		if err != nil {
			return authoritySelection{}, fmt.Errorf("contextcompile: authority operand %s applicability: %w", candidate.ID, err)
		}
		if result.State != ApplicabilityApplicable && result.State != ApplicabilityUnknown {
			continue
		}
		if result.State == ApplicabilityUnknown {
			retainedUnknown = true
		}
		operands = append(operands, PolicyOperand{
			Kind:   candidate.Kind,
			ID:     candidate.ID,
			Path:   candidate.Path,
			Digest: candidate.Digest,
			Scope:  cloneScope(candidate.Scope),
		})
		if candidate.Kind == PolicyEntryPolicy {
			policyIDs = append(policyIDs, candidate.ID)
		}
	}

	sort.Slice(operands, func(i, j int) bool {
		return operands[i].Kind+"\x00"+operands[i].ID < operands[j].Kind+"\x00"+operands[j].ID
	})
	sort.Strings(policyIDs)

	disclosures := []DisclosureCode{}
	if retainedUnknown {
		disclosures = []DisclosureCode{DisclosureApplicabilityUnknown}
	}

	return authoritySelection{
		Operands:    operands,
		Selection:   instructionprojection.Selection{PolicyIDs: policyIDs},
		Disclosures: disclosures,
	}, nil
}

// authorityOperandCandidates derives one selector candidate for every
// policy, overlay, and exemption loaded into authority, failing closed on a
// nil store or a digest failure. Each candidate's Scope is a fresh copy,
// never aliasing store memory. Candidates are returned in canonical
// bytewise Kind+"\x00"+ID order.
func authorityOperandCandidates(authority PolicyAuthority) ([]authorityOperandCandidate, error) {
	if authority.Store == nil {
		return nil, fmt.Errorf("contextcompile: policy authority store is nil")
	}

	candidates := make([]authorityOperandCandidate, 0, len(authority.Store.Policies)+len(authority.Store.Overlays)+len(authority.Store.Exemptions))

	for _, policy := range authority.Store.Policies {
		if policy == nil {
			return nil, fmt.Errorf("contextcompile: policy authority store has a nil policy entry")
		}
		digest, err := policy.Digest()
		if err != nil {
			return nil, fmt.Errorf("contextcompile: digest policy %s: %w", policy.ID, err)
		}
		candidates = append(candidates, authorityOperandCandidate{
			Kind:   PolicyEntryPolicy,
			ID:     policy.ID,
			Path:   policyStoreDir + policyartifact.DirPolicies + "/" + policy.Name() + ".md",
			Digest: digest,
			Scope:  cloneScope(policy.Scope),
		})
	}
	for _, overlay := range authority.Store.Overlays {
		if overlay == nil {
			return nil, fmt.Errorf("contextcompile: policy authority store has a nil overlay entry")
		}
		digest, err := overlay.Digest()
		if err != nil {
			return nil, fmt.Errorf("contextcompile: digest overlay %s: %w", overlay.ID, err)
		}
		candidates = append(candidates, authorityOperandCandidate{
			Kind:   PolicyEntryOverlay,
			ID:     overlay.ID,
			Path:   policyStoreDir + policyartifact.DirOverlays + "/" + overlay.Name() + ".md",
			Digest: digest,
			Scope:  cloneScope(overlay.Scope),
		})
	}
	for _, exemption := range authority.Store.Exemptions {
		if exemption == nil {
			return nil, fmt.Errorf("contextcompile: policy authority store has a nil exemption entry")
		}
		digest, err := exemption.Digest()
		if err != nil {
			return nil, fmt.Errorf("contextcompile: digest exemption %s: %w", exemption.ID, err)
		}
		candidates = append(candidates, authorityOperandCandidate{
			Kind:   PolicyEntryExemption,
			ID:     exemption.ID,
			Path:   policyStoreDir + policyartifact.DirExemptions + "/" + exemption.Name() + ".md",
			Digest: digest,
			Scope:  cloneScope(exemption.Scope),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Kind+"\x00"+candidates[i].ID < candidates[j].Kind+"\x00"+candidates[j].ID
	})

	return candidates, nil
}

// storeAuthorityArtifact is one constitution-store artifact that authority
// design §5's store-authority row enumerates UNCONDITIONALLY — "Resolved
// constitution, profile, applicable policies/overlays/exemptions, ..." — as
// opposed to the policy/overlay/exemption operands, which enter only when
// stage 7's applicability selection retains them.
//
// Ref is the canonical policy-artifact ref (validatePolicyArtifactRef's
// grammar); Path is the tracked store path the ref lifts out of the
// head-tree source; Digest is the ADOPTED digest the resolved effective
// policy already binds, so a compile can prove the HEAD bytes it wraps as a
// payload are the same bytes that authority was resolved from.
type storeAuthorityArtifact struct {
	Ref    string
	Path   string
	Digest string
}

// resolvedAuthorityArtifacts returns the resolved constitution and the
// SELECTED governance profile as store-authority lifts (authority design
// §5). Neither is subject to the request's declared scope: §6 fixes that
// "scope still bounds repository material", and the applicable constitution
// is required capsule content for every phase — so leaving these two as
// head-tree repository files would let a narrowed `scope.paths` exclude the
// governing constitution from the capsule it governs.
//
// Both digests come from the already-resolved EffectivePolicy, never
// recomputed here from a second reading of the store.
func resolvedAuthorityArtifacts(authority PolicyAuthority) ([]storeAuthorityArtifact, error) {
	if authority.Store == nil {
		return nil, fmt.Errorf("contextcompile: resolved authority artifacts: policy authority store is nil")
	}
	if authority.Effective == nil {
		return nil, fmt.Errorf("contextcompile: resolved authority artifacts: policy authority effective policy is nil")
	}
	profileID := authority.Effective.ProfileID
	if _, ok := authority.Store.Profiles[profileID]; !ok {
		return nil, fmt.Errorf("contextcompile: resolved authority artifacts: resolved effective policy names profile %q, absent from the loaded store", profileID)
	}

	artifacts := []storeAuthorityArtifact{
		{
			Ref:    policyartifact.KindConstitution + "/" + policyartifact.ConstitutionName,
			Path:   policyStoreDir + policyartifact.ConstitutionName + ".md",
			Digest: authority.Effective.ConstitutionDigest,
		},
		{
			Ref:    policyartifact.KindProfileStorage + "/" + profileID,
			Path:   policyStoreDir + policyartifact.DirProfiles + "/" + profileID + ".md",
			Digest: authority.Effective.ProfileDigest,
		},
	}
	for _, a := range artifacts {
		if err := validatePolicyArtifactRef("resolved authority artifact ref", a.Ref); err != nil {
			return nil, err
		}
		if err := validateCandidatePath(a.Path); err != nil {
			return nil, fmt.Errorf("contextcompile: resolved authority artifact %s: %w", a.Ref, err)
		}
		if err := validateDigest("resolved authority artifact "+a.Ref+" digest", a.Digest); err != nil {
			return nil, err
		}
	}
	return artifacts, nil
}

// requireAdoptedAuthorityDigest proves the exact HEAD bytes a compile is
// about to wrap as an authority operand's payload are the same bytes the
// ADOPTED digest was computed from.
//
// Every adopted digest in this service comes from the working-tree store
// load (policyauthority.Load / Resolve), while every payload's bytes come
// from `git show <HEAD>`. Those two disagree whenever an adopted artifact
// has been edited without being committed, so without this check the
// manifest would publish the fresh adopted digest over older bytes —
// exactly the stale-authority substitution the digest is supposed to
// prevent.
//
// A divergence is inconsistent authority, so it fails closed OPERATIONALLY
// (authority design §10: "Malformed/noncanonical request or authority |
// Exit 2"), matching the TOCTOU discipline resolveOwners and
// reverifyGoverningFeature already apply to specification bytes. It is not
// a new refusal family: §10 fixes no exit-1 row for it.
func requireAdoptedAuthorityDigest(ref string, content []byte, adopted string, catalog governanceprincipal.Catalog) error {
	computed, err := adoptedAuthorityDigest(ref, content, catalog)
	if err != nil {
		return fmt.Errorf("contextcompile: authority operand %s: decode its exact HEAD bytes: %w", ref, err)
	}
	if computed != adopted {
		return fmt.Errorf("contextcompile: authority operand %s: the adopted digest %s does not bind its exact HEAD bytes (those digest as %s); the adopted store and HEAD have diverged", ref, adopted, computed)
	}
	return nil
}

// adoptedAuthorityDigest re-derives one authority artifact's canonical
// digest from content, dispatching on the artifact ref's own kind half
// through the SAME strict policyartifact/governanceprincipal decoders the
// store load used — never a second, compiler-private digest rule.
func adoptedAuthorityDigest(ref string, content []byte, catalog governanceprincipal.Catalog) (string, error) {
	kind, _, ok := strings.Cut(ref, "/")
	if !ok {
		return "", fmt.Errorf("%q is not a <kind>/<name> policy artifact ref", ref)
	}
	switch kind {
	case policyartifact.KindPolicy:
		decoded, err := policyartifact.DecodePolicy(content)
		if err != nil {
			return "", err
		}
		return decoded.Digest()
	case policyartifact.KindOverlay:
		decoded, err := policyartifact.DecodeOverlay(content)
		if err != nil {
			return "", err
		}
		return decoded.Digest()
	case policyartifact.KindExemption:
		decoded, err := policyartifact.DecodeExemption(content)
		if err != nil {
			return "", err
		}
		return decoded.Digest()
	case policyartifact.KindConstitution:
		decoded, err := policyartifact.DecodeConstitution(content)
		if err != nil {
			return "", err
		}
		return decoded.Digest()
	case policyartifact.KindProfileStorage:
		decoded, err := policyartifact.DecodeStoredProfile(content, catalog)
		if err != nil {
			return "", err
		}
		// The adopted profile digest EffectivePolicy records is the
		// kernel's own sealed profile digest (policyauthority's Resolve
		// takes profile.Profile.Digest()), so this re-derivation must take
		// exactly the same one.
		return decoded.Profile.Digest()
	default:
		return "", fmt.Errorf("unknown policy artifact kind %q", kind)
	}
}

// renderSelectedProjection renders authority's loaded store, effective
// policy, and adapter through the one pure instructionprojection.Render
// seam for selection, mapping each Rendered.Files entry to a ProjectionFile
// with a fresh content-byte copy. It fails closed on a nil authority.Store
// or authority.Effective before calling Render, and wraps any Render error.
// Render's own file order is the contract: this never re-sorts or filters.
func renderSelectedProjection(authority PolicyAuthority, selection instructionprojection.Selection) ([]ProjectionFile, error) {
	if authority.Store == nil {
		return nil, fmt.Errorf("contextcompile: render selected projection: policy authority store is nil")
	}
	if authority.Effective == nil {
		return nil, fmt.Errorf("contextcompile: render selected projection: policy authority effective policy is nil")
	}

	rendered, err := instructionprojection.Render(authority.Store, authority.Effective, authority.Adapter, selection)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: render selected projection: %w", err)
	}

	files := make([]ProjectionFile, 0, len(rendered.Files))
	for _, f := range rendered.Files {
		files = append(files, ProjectionFile{
			Path:    f.Path,
			Content: append([]byte(nil), f.Content...),
			Digest:  f.Digest,
		})
	}
	return files, nil
}

func cloneScope(in policyartifact.Scope) policyartifact.Scope {
	return policyartifact.Scope{
		Phases:       cloneStrings(in.Phases),
		Environments: cloneStrings(in.Environments),
		Paths:        cloneStrings(in.Paths),
		Refs:         cloneStrings(in.Refs),
	}
}
