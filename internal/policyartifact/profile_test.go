package policyartifact

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// validProfileDoc returns a stored governance-profile artifact: the
// kernel-owned profile document as frontmatter, rationale prose as body.
func validProfileDoc() string {
	return `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile for this store.
`
}

func testGovCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:             []string{"author", "reviewer", "policy-owner"},
		Transitions:       []string{"accept", "close"},
		EvidenceSources:   []string{"ci"},
		EscalationMetrics: []string{"age-days"},
	}
}

func TestDecodeStoredProfile_Happy(t *testing.T) {
	sp, err := DecodeStoredProfile([]byte(validProfileDoc()), testGovCatalog())
	if err != nil {
		t.Fatalf("DecodeStoredProfile: %v", err)
	}
	if sp.ID != "solo-default" {
		t.Fatalf("ID = %q", sp.ID)
	}
	if !strings.HasPrefix(sp.ProfileDigest, "sha256:") {
		t.Fatalf("ProfileDigest = %q", sp.ProfileDigest)
	}
	// The stored record carries the kernel's own sealed Profile value;
	// its digest agrees with the record's.
	d, err := sp.Profile.Digest()
	if err != nil {
		t.Fatalf("Profile.Digest: %v", err)
	}
	if d != sp.ProfileDigest {
		t.Fatalf("digest mismatch: record %s vs kernel %s", sp.ProfileDigest, d)
	}
}

func TestDecodeStoredProfile_Negative(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"kernel rejects unknown field", strings.Replace(validProfileDoc(), "class: solo", "class: solo\nmood: relaxed", 1), "mood"},
		{"kernel rejects unknown class", strings.Replace(validProfileDoc(), "class: solo", "class: emperor", 1), "class"},
		{"kernel rejects unknown transition", strings.Replace(validProfileDoc(), "applicable_transitions: [accept]", "applicable_transitions: [deploy]", 1), "transition"},
		{"empty body", strings.SplitN(validProfileDoc(), "The solo", 2)[0] + "\n", "rationale"},
		{"no frontmatter", "just prose\n", "frontmatter"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeStoredProfile([]byte(tt.doc), testGovCatalog())
			if err == nil {
				t.Fatalf("DecodeStoredProfile = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeStoredProfile error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}
