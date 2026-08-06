package governanceprincipal

import (
	"strings"
	"testing"
)

func mustErr(t *testing.T, raw []byte, wantSub string) {
	t.Helper()
	_, err := DecodeProfile(raw, testCatalog())
	if err == nil {
		t.Fatalf("DecodeProfile: expected error containing %q, got nil", wantSub)
	}
	if !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("DecodeProfile error %q does not contain %q", err.Error(), wantSub)
	}
}

// soloYAML is a valid solo profile whose rule lists are explicitly empty.
const soloYAML = `schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept, close]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123"]
  - role: reviewer
    trust_source: github
    subjects: ["user-123"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

// highAssuranceYAML provides every required rule family for its single
// applicable transition.
const highAssuranceYAML = `schema: verdi.governance-profile/v1
id: ha-default
class: high-assurance
applicable_transitions: [close]
identity_trust_sources:
  - { id: github, kind: forge }
  - { id: git-signature, kind: signed-commit }
  - { id: codeowners, kind: ownership }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123"]
  - role: reviewer
    trust_source: github
    subjects: ["user-456"]
ownership_sources:
  - id: repository-owners
    trust_source: codeowners
    transitions: [close]
    roles: [reviewer]
signature_requirements:
  - transitions: [close]
    roles: [author]
    trust_sources: [git-signature]
required_approvers:
  - transitions: [close]
    roles: [reviewer]
    minimum: 1
distinctness_rules:
  - transitions: [close]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions:
  - transitions: [close]
    allowed_sources: [merge-gate]
escalation_thresholds: []
`

func TestDecodeProfileClassFixtures(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want Class
	}{
		{"solo with explicitly empty rule lists", soloYAML, ClassSolo},
		{"high-assurance with full coverage", highAssuranceYAML, ClassHighAssurance},
		{"experimental with explicitly empty rule lists", strings.Replace(strings.Replace(soloYAML, "class: solo", "class: experimental", 1), "id: solo-default", "id: exp-default", 1), ClassExperimental},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := mustDecode(t, []byte(tt.raw))
			if p.Class != tt.want {
				t.Errorf("Class = %q, want %q", p.Class, tt.want)
			}
		})
	}
}

func TestDecodeProfileMissingTopLevelFields(t *testing.T) {
	for _, field := range profileFieldOrder {
		t.Run(field, func(t *testing.T) {
			mustErr(t, profileYAML(field), "missing")
		})
	}
}

func TestDecodeProfileNullFieldIsNotEmptyList(t *testing.T) {
	mustErr(t, profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n"}), "missing")
}

func TestDecodeProfileFailClosed(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{"unknown top-level field", append(profileYAML(), []byte("surprise_field: 1\n")...), "surprise_field"},
		{"unknown nested field", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [close]\n    roles: [reviewer]\n    extra: true\n"}), "extra"},
		{"unknown schema version", profileYAMLWith(map[string]string{"schema": "schema: verdi.governance-profile/v2\n"}), "schema"},
		{"unknown class", profileYAMLWith(map[string]string{"class": "class: enterprise\n"}), "unknown profile class"},
		{"unknown trust-source kind", profileYAMLWith(map[string]string{"identity_trust_sources": "identity_trust_sources:\n  - { id: github, kind: oauth }\n"}), "unknown trust-source kind"},
		{"unknown distinctness relation", profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: [accept, close, merge-authorize]\n    left_role: author\n    right_role: reviewer\n    relation: distinct\n"}), "unknown distinctness relation"},
		{"yaml anchor", profileYAMLWith(map[string]string{"id": "id: &anchor team-default\n"}), "anchor"},
		{"yaml alias", profileYAMLWith(map[string]string{"id": "id: &anchor team-default\nnote_ref: *anchor\n"}), "anchor"},
		{"yaml custom tag", profileYAMLWith(map[string]string{"id": "id: !custom team-default\n"}), "tag"},
		{"invalid profile id", profileYAMLWith(map[string]string{"id": "id: Team-Default\n"}), "invalid id"},
		{"invalid trust-source id", profileYAMLWith(map[string]string{"identity_trust_sources": "identity_trust_sources:\n  - { id: GitHub, kind: forge }\n  - { id: git-signature, kind: signed-commit }\n  - { id: codeowners, kind: ownership }\n"}), "invalid id"},
		{"empty applicable_transitions", profileYAMLWith(map[string]string{"applicable_transitions": "applicable_transitions: []\n"}), "nonempty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestDecodeProfileCatalogUnknowns(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{"applicable transition", profileYAMLWith(map[string]string{"applicable_transitions": "applicable_transitions: [accept, close, merge-authorize, ship]\n"}), "transition"},
		{"role mapping role", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: emperor\n    trust_source: github\n    subjects: [\"user-123\"]\n"}), "role"},
		{"ownership transition", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [ship]\n    roles: [reviewer]\n"}), "transition"},
		{"ownership role", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [close]\n    roles: [emperor]\n"}), "role"},
		{"signature transition", profileYAMLWith(map[string]string{"signature_requirements": "signature_requirements:\n  - transitions: [ship]\n    roles: [author]\n    trust_sources: [git-signature]\n"}), "transition"},
		{"approver role", profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [merge-authorize]\n    roles: [emperor]\n    minimum: 1\n  - transitions: [close, accept]\n    roles: [reviewer]\n    minimum: 1\n"}), "role"},
		{"distinctness role", profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: [accept, close, merge-authorize]\n    left_role: emperor\n    right_role: reviewer\n    relation: different-principal\n"}), "role"},
		{"evidence source", profileYAMLWith(map[string]string{"evidence_source_restrictions": "evidence_source_restrictions:\n  - transitions: [close]\n    allowed_sources: [rumor]\n"}), "evidence source"},
		{"escalation metric", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: vibes\n    at_least: 1\n    required_roles: [owner]\n"}), "metric"},
		{"escalation required role", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: unresolved-exceptions\n    at_least: 1\n    required_roles: [emperor]\n"}), "role"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestDecodeProfileDuplicates(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{"applicable transitions member", profileYAMLWith(map[string]string{"applicable_transitions": "applicable_transitions: [accept, close, merge-authorize, accept]\n"}), "duplicate"},
		{"trust-source id", profileYAMLWith(map[string]string{"identity_trust_sources": "identity_trust_sources:\n  - { id: github, kind: forge }\n  - { id: github, kind: forge }\n  - { id: git-signature, kind: signed-commit }\n  - { id: codeowners, kind: ownership }\n"}), "duplicate"},
		{"role mapping pair", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: author\n    trust_source: github\n    subjects: [\"user-123\"]\n  - role: author\n    trust_source: github\n    subjects: [\"user-456\"]\n"}), "duplicate"},
		{"role mapping subject", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: author\n    trust_source: github\n    subjects: [\"user-123\", \"user-123\"]\n"}), "duplicate"},
		{"ownership id", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [close]\n    roles: [reviewer]\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [accept]\n    roles: [reviewer]\n"}), "duplicate"},
		{"signature pair", profileYAMLWith(map[string]string{"signature_requirements": "signature_requirements:\n  - transitions: [merge-authorize]\n    roles: [author]\n    trust_sources: [git-signature]\n  - transitions: [merge-authorize]\n    roles: [author]\n    trust_sources: [git-signature]\n"}), "duplicate"},
		{"approver pair across entries", profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [merge-authorize, close, accept]\n    roles: [reviewer]\n    minimum: 1\n  - transitions: [close]\n    roles: [reviewer]\n    minimum: 2\n"}), "duplicate"},
		{"distinctness tuple", profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: [accept, close, merge-authorize]\n    left_role: author\n    right_role: reviewer\n    relation: different-principal\n  - transitions: [accept]\n    left_role: author\n    right_role: reviewer\n    relation: same-principal\n"}), "duplicate"},
		{"evidence restriction transition", profileYAMLWith(map[string]string{"evidence_source_restrictions": "evidence_source_restrictions:\n  - transitions: [close]\n    allowed_sources: [merge-gate]\n  - transitions: [close]\n    allowed_sources: [ci-verify]\n"}), "duplicate"},
		{"escalation pair", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: unresolved-exceptions\n    at_least: 1\n    required_roles: [owner]\n  - transitions: [close]\n    metric: unresolved-exceptions\n    at_least: 3\n    required_roles: [owner]\n"}), "duplicate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestDecodeProfileReferences(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{"role mapping dangling trust source", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: author\n    trust_source: bitbucket\n    subjects: [\"user-123\"]\n"}), "trust source"},
		{"ownership dangling trust source", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: absentee\n    transitions: [close]\n    roles: [reviewer]\n"}), "trust source"},
		{"ownership wrong kind", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: github\n    transitions: [close]\n    roles: [reviewer]\n"}), "kind"},
		{"signature dangling trust source", profileYAMLWith(map[string]string{"signature_requirements": "signature_requirements:\n  - transitions: [merge-authorize]\n    roles: [author]\n    trust_sources: [absentee]\n"}), "trust source"},
		{"signature wrong kind", profileYAMLWith(map[string]string{"signature_requirements": "signature_requirements:\n  - transitions: [merge-authorize]\n    roles: [author]\n    trust_sources: [github]\n"}), "kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestDecodeProfileFieldRules(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{"empty subjects list", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: author\n    trust_source: github\n    subjects: []\n"}), "subjects"},
		{"empty subject string", profileYAMLWith(map[string]string{"role_mappings": "role_mappings:\n  - role: author\n    trust_source: github\n    subjects: [\"\"]\n"}), "subject"},
		{"zero approver minimum", profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [merge-authorize, close, accept]\n    roles: [reviewer]\n    minimum: 0\n"}), "minimum"},
		{"negative approver minimum", profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [merge-authorize, close, accept]\n    roles: [reviewer]\n    minimum: -1\n"}), "minimum"},
		{"missing at_least", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: unresolved-exceptions\n    required_roles: [owner]\n"}), "at_least"},
		{"negative at_least", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: unresolved-exceptions\n    at_least: -1\n    required_roles: [owner]\n"}), "at_least"},
		{"same left and right role", profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: [accept, close, merge-authorize]\n    left_role: author\n    right_role: author\n    relation: same-principal\n"}), "different field references"},
		{"empty ownership transitions", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: []\n    roles: [reviewer]\n"}), "transitions"},
		{"empty ownership roles", profileYAMLWith(map[string]string{"ownership_sources": "ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [close]\n    roles: []\n"}), "roles"},
		{"empty signature trust_sources", profileYAMLWith(map[string]string{"signature_requirements": "signature_requirements:\n  - transitions: [merge-authorize]\n    roles: [author]\n    trust_sources: []\n"}), "trust_sources"},
		{"empty approver roles", profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [merge-authorize, close, accept]\n    roles: []\n    minimum: 1\n"}), "roles"},
		{"empty distinctness transitions", profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: []\n    left_role: author\n    right_role: reviewer\n    relation: different-principal\n"}), "transitions"},
		{"empty escalation required_roles", profileYAMLWith(map[string]string{"escalation_thresholds": "escalation_thresholds:\n  - transitions: [close]\n    metric: unresolved-exceptions\n    at_least: 1\n    required_roles: []\n"}), "required_roles"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestDecodeProfileClassCoverage(t *testing.T) {
	haWithout := func(family, empty string) []byte {
		return []byte(strings.Replace(highAssuranceYAML, family, empty, 1))
	}
	tests := []struct {
		name    string
		raw     []byte
		wantSub string
	}{
		{
			"team missing approval coverage",
			profileYAMLWith(map[string]string{"required_approvers": "required_approvers:\n  - transitions: [close, accept]\n    roles: [reviewer]\n    minimum: 1\n"}),
			"approval",
		},
		{
			"team missing different-principal coverage",
			profileYAMLWith(map[string]string{"distinctness_rules": "distinctness_rules:\n  - transitions: [close, accept]\n    left_role: author\n    right_role: reviewer\n    relation: different-principal\n"}),
			"different-principal",
		},
		{
			"team empty rule lists rejected",
			profileYAMLWith(map[string]string{
				"required_approvers": "required_approvers: []\n",
				"distinctness_rules": "distinctness_rules: []\n",
			}),
			"team",
		},
		{
			"high-assurance missing approval",
			haWithout("required_approvers:\n  - transitions: [close]\n    roles: [reviewer]\n    minimum: 1\n", "required_approvers: []\n"),
			"approval",
		},
		{
			"high-assurance missing different-principal",
			haWithout("distinctness_rules:\n  - transitions: [close]\n    left_role: author\n    right_role: reviewer\n    relation: different-principal\n", "distinctness_rules: []\n"),
			"different-principal",
		},
		{
			"high-assurance missing signature",
			haWithout("signature_requirements:\n  - transitions: [close]\n    roles: [author]\n    trust_sources: [git-signature]\n", "signature_requirements: []\n"),
			"signature",
		},
		{
			"high-assurance missing ownership",
			haWithout("ownership_sources:\n  - id: repository-owners\n    trust_source: codeowners\n    transitions: [close]\n    roles: [reviewer]\n", "ownership_sources: []\n"),
			"ownership",
		},
		{
			"high-assurance missing evidence-source rule",
			haWithout("evidence_source_restrictions:\n  - transitions: [close]\n    allowed_sources: [merge-gate]\n", "evidence_source_restrictions: []\n"),
			"evidence-source",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mustErr(t, tt.raw, tt.wantSub)
		})
	}
}

func TestCatalogValidate(t *testing.T) {
	tests := []struct {
		name    string
		catalog Catalog
		wantSub string
	}{
		{"duplicate role", Catalog{Roles: []string{"author", "author"}}, "duplicate"},
		{"invalid transition id", Catalog{Transitions: []string{"Accept"}}, "invalid id"},
		{"invalid metric id", Catalog{EscalationMetrics: []string{"9metric"}}, "invalid id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.catalog.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("Catalog.Validate() = %v, want error containing %q", err, tt.wantSub)
			}
			if _, err := DecodeProfile(profileYAML(), tt.catalog); err == nil {
				t.Fatalf("DecodeProfile accepted malformed catalog")
			}
		})
	}
	if err := testCatalog().Validate(); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
}

// TestDecodeProfileAllowedSourcesPresence: allowed_sources must be
// present and non-null; a literal [] is the explicit empty set, and
// behaviorally equivalent profiles digest identically.
func TestDecodeProfileAllowedSourcesPresence(t *testing.T) {
	mustErr(t, profileYAMLWith(map[string]string{
		"evidence_source_restrictions": "evidence_source_restrictions:\n  - transitions: [close]\n",
	}), "allowed_sources")
	mustErr(t, profileYAMLWith(map[string]string{
		"evidence_source_restrictions": "evidence_source_restrictions:\n  - transitions: [close]\n    allowed_sources:\n",
	}), "allowed_sources")

	explicit := "evidence_source_restrictions:\n  - transitions: [close]\n    allowed_sources: []\n"
	p := mustDecode(t, profileYAMLWith(map[string]string{"evidence_source_restrictions": explicit}))
	if len(p.EvidenceSourceRestrictions) != 1 || p.EvidenceSourceRestrictions[0].AllowedSources == nil ||
		len(p.EvidenceSourceRestrictions[0].AllowedSources) != 0 {
		t.Fatalf("explicit [] allowed_sources = %+v, want present empty set", p.EvidenceSourceRestrictions)
	}
	d1, err := p.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	q := mustDecode(t, profileYAMLWith(map[string]string{"evidence_source_restrictions": explicit}))
	d2, err := q.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if d1 != d2 {
		t.Errorf("equivalent profiles digest differently: %s vs %s", d1, d2)
	}
}
