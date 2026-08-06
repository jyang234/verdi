package policyartifact

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// KindProfileStorage names the stored governance-profile artifact kind
// within this store's own grammar. It is a storage classification only:
// the artifact's schema and semantics belong to the governance-principal
// kernel (DC-20).
const KindProfileStorage = "governance-profile"

// Dir names within .verdi/policy/ (SI-6: this unit owns the directory's
// internal grammar).
const (
	DirPolicies   = "policies"
	DirOverlays   = "overlays"
	DirExemptions = "exemptions"
	DirProfiles   = "profiles"
)

// ClassifyPolicyPath maps a .verdi/policy/-relative slash path to the
// constitution artifact kind it must decode as and its name half. The
// grammar is closed (D1's posture applied inside the directory): an
// entry matching no row is an error, never silently skipped — a
// constitution store carries only constitution artifacts.
func ClassifyPolicyPath(rel string) (kind, name string, err error) {
	if rel == ConstitutionName+".md" {
		return KindConstitution, ConstitutionName, nil
	}
	dir, file, ok := strings.Cut(rel, "/")
	if !ok || strings.Contains(file, "/") || !strings.HasSuffix(file, ".md") {
		return "", "", fmt.Errorf("policyartifact: unrecognized entry %q under .verdi/policy/ (known: constitution.md, %s/<name>.md, %s/<name>.md, %s/<name>.md, %s/<name>.md)", rel, DirPolicies, DirOverlays, DirExemptions, DirProfiles)
	}
	stem := strings.TrimSuffix(file, ".md")
	switch dir {
	case DirPolicies, DirOverlays, DirExemptions:
		if !kebabRe.MatchString(stem) {
			return "", "", fmt.Errorf("policyartifact: entry %q under .verdi/policy/%s: name %q must be kebab-case", rel, dir, stem)
		}
		switch dir {
		case DirPolicies:
			return KindPolicy, stem, nil
		case DirOverlays:
			return KindOverlay, stem, nil
		default:
			return KindExemption, stem, nil
		}
	case DirProfiles:
		// The file stem must equal the profile's kernel id, so it uses
		// the kernel's own id grammar — called, never copied, so the two
		// can never drift (DC-20: the kernel owns profile schema).
		if err := governanceprincipal.ValidateID(stem); err != nil {
			return "", "", fmt.Errorf("policyartifact: entry %q under .verdi/policy/%s: name must match the kernel profile id grammar: %w", rel, dir, err)
		}
		return KindProfileStorage, stem, nil
	}
	return "", "", fmt.Errorf("policyartifact: unrecognized entry %q under .verdi/policy/ (known: constitution.md, %s/<name>.md, %s/<name>.md, %s/<name>.md, %s/<name>.md)", rel, DirPolicies, DirOverlays, DirExemptions, DirProfiles)
}
