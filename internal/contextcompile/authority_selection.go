package contextcompile

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

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
	if wantPath := ".verdi/policy/" + operandArtifactDir[c.Kind] + "/" + name + ".md"; c.Path != wantPath {
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
			Path:   ".verdi/policy/policies/" + policy.Name() + ".md",
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
			Path:   ".verdi/policy/overlays/" + overlay.Name() + ".md",
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
			Path:   ".verdi/policy/exemptions/" + exemption.Name() + ".md",
			Digest: digest,
			Scope:  cloneScope(exemption.Scope),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Kind+"\x00"+candidates[i].ID < candidates[j].Kind+"\x00"+candidates[j].ID
	})

	return candidates, nil
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
