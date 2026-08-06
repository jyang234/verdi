package governanceprincipal

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// testCatalog is the injected closed catalog every decode test resolves
// against. The kernel never hardcodes these names (DC-25: transition and
// role semantics stay with their owning authorities); callers supply them.
func testCatalog() Catalog {
	return Catalog{
		Roles:             []string{"author", "reviewer", "owner"},
		Transitions:       []string{"accept", "close", "merge-authorize"},
		EvidenceSources:   []string{"merge-gate", "ci-verify"},
		EscalationMetrics: []string{"unresolved-exceptions"},
	}
}

// profileFieldOrder fixes the concatenation order of the section map so
// omission tests can drop exactly one top-level field.
var profileFieldOrder = []string{
	"schema",
	"id",
	"class",
	"applicable_transitions",
	"identity_trust_sources",
	"role_mappings",
	"ownership_sources",
	"signature_requirements",
	"required_approvers",
	"distinctness_rules",
	"evidence_source_restrictions",
	"escalation_thresholds",
}

// profileSections is a complete, valid team profile in deliberately
// unsorted member order, one YAML block per top-level field.
var profileSections = map[string]string{
	"schema":                 "schema: verdi.governance-profile/v1\n",
	"id":                     "id: team-default\n",
	"class":                  "class: team\n",
	"applicable_transitions": "applicable_transitions: [close, accept, merge-authorize]\n",
	"identity_trust_sources": `identity_trust_sources:
  - { id: github, kind: forge }
  - { id: git-signature, kind: signed-commit }
  - { id: codeowners, kind: ownership }
  - { id: corporate-idp, kind: identity-provider }
`,
	"role_mappings": `role_mappings:
  - role: reviewer
    trust_source: github
    subjects: ["user-456", "user-789"]
  - role: author
    trust_source: github
    subjects: ["user-123"]
`,
	"ownership_sources": `ownership_sources:
  - id: repository-owners
    trust_source: codeowners
    transitions: [close]
    roles: [reviewer]
`,
	"signature_requirements": `signature_requirements:
  - transitions: [merge-authorize]
    roles: [author]
    trust_sources: [git-signature]
`,
	"required_approvers": `required_approvers:
  - transitions: [merge-authorize]
    roles: [reviewer]
    minimum: 1
  - transitions: [close, accept]
    roles: [reviewer]
    minimum: 1
`,
	"distinctness_rules": `distinctness_rules:
  - transitions: [close, accept, merge-authorize]
    left_role: author
    right_role: reviewer
    relation: different-principal
`,
	"evidence_source_restrictions": `evidence_source_restrictions:
  - transitions: [close]
    allowed_sources: [merge-gate]
`,
	"escalation_thresholds": `escalation_thresholds:
  - transitions: [close]
    metric: unresolved-exceptions
    at_least: 1
    required_roles: [owner]
`,
}

// profileYAML concatenates every top-level section except the named
// omissions, in the fixed field order.
func profileYAML(omit ...string) []byte {
	omitted := map[string]bool{}
	for _, o := range omit {
		omitted[o] = true
	}
	var b strings.Builder
	for _, f := range profileFieldOrder {
		if !omitted[f] {
			b.WriteString(profileSections[f])
		}
	}
	return []byte(b.String())
}

// profileYAMLWith swaps the named sections for replacement blocks.
func profileYAMLWith(replace map[string]string) []byte {
	var b strings.Builder
	for _, f := range profileFieldOrder {
		if r, ok := replace[f]; ok {
			b.WriteString(r)
		} else {
			b.WriteString(profileSections[f])
		}
	}
	return []byte(b.String())
}

func mustDecode(t *testing.T, raw []byte) Profile {
	t.Helper()
	p, err := DecodeProfile(raw, testCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: unexpected error: %v", err)
	}
	return p
}

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func TestDecodeProfileHappyPath(t *testing.T) {
	p := mustDecode(t, profileYAML())

	if p.Schema != SchemaID {
		t.Errorf("Schema = %q, want %q", p.Schema, SchemaID)
	}
	if p.ID != "team-default" {
		t.Errorf("ID = %q, want team-default", p.ID)
	}
	if p.Class != ClassTeam {
		t.Errorf("Class = %q, want %q", p.Class, ClassTeam)
	}
	// Set-like lists come back in normalized deterministic order.
	if want := []string{"accept", "close", "merge-authorize"}; !reflect.DeepEqual(p.ApplicableTransitions, want) {
		t.Errorf("ApplicableTransitions = %v, want %v", p.ApplicableTransitions, want)
	}
	if len(p.IdentityTrustSources) != 4 || p.IdentityTrustSources[0].ID != "codeowners" {
		t.Errorf("IdentityTrustSources not sorted by id: %+v", p.IdentityTrustSources)
	}
	if p.IdentityTrustSources[0].Kind != TrustSourceOwnership {
		t.Errorf("codeowners kind = %q, want %q", p.IdentityTrustSources[0].Kind, TrustSourceOwnership)
	}
	if len(p.RoleMappings) != 2 || p.RoleMappings[0].Role != "author" {
		t.Errorf("RoleMappings not sorted by role: %+v", p.RoleMappings)
	}
	if want := []string{"user-456", "user-789"}; !reflect.DeepEqual(p.RoleMappings[1].Subjects, want) {
		t.Errorf("reviewer subjects = %v, want %v", p.RoleMappings[1].Subjects, want)
	}
	if len(p.RequiredApprovers) != 2 {
		t.Fatalf("RequiredApprovers len = %d, want 2", len(p.RequiredApprovers))
	}
	// Inner transition sets are sorted too.
	if want := []string{"accept", "close"}; !reflect.DeepEqual(p.RequiredApprovers[0].Transitions, want) {
		t.Errorf("first approver rule transitions = %v, want %v (entries and members sorted)", p.RequiredApprovers[0].Transitions, want)
	}
	if len(p.DistinctnessRules) != 1 || p.DistinctnessRules[0].Relation != RelationDifferentPrincipal {
		t.Errorf("DistinctnessRules = %+v", p.DistinctnessRules)
	}
	if want := []string{"accept", "close", "merge-authorize"}; !reflect.DeepEqual(p.DistinctnessRules[0].Transitions, want) {
		t.Errorf("distinctness transitions = %v, want %v", p.DistinctnessRules[0].Transitions, want)
	}
	if len(p.EscalationThresholds) != 1 || p.EscalationThresholds[0].AtLeast != 1 {
		t.Errorf("EscalationThresholds = %+v", p.EscalationThresholds)
	}

	d, err := p.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !digestRe.MatchString(d) {
		t.Errorf("Digest = %q, want canonical sha256 form", d)
	}
}

// TestDecodeProfileOrderIndependence: semantically equivalent profiles
// whose set-like lists arrive in different orders normalize to the same
// value and digest.
func TestDecodeProfileOrderIndependence(t *testing.T) {
	base := mustDecode(t, profileYAML())

	shuffled := profileYAMLWith(map[string]string{
		"applicable_transitions": "applicable_transitions: [merge-authorize, accept, close]\n",
		"identity_trust_sources": `identity_trust_sources:
  - { id: corporate-idp, kind: identity-provider }
  - { id: codeowners, kind: ownership }
  - { id: git-signature, kind: signed-commit }
  - { id: github, kind: forge }
`,
		"role_mappings": `role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123"]
  - role: reviewer
    trust_source: github
    subjects: ["user-789", "user-456"]
`,
		"required_approvers": `required_approvers:
  - transitions: [accept, close]
    roles: [reviewer]
    minimum: 1
  - transitions: [merge-authorize]
    roles: [reviewer]
    minimum: 1
`,
		"distinctness_rules": `distinctness_rules:
  - transitions: [merge-authorize, close, accept]
    left_role: author
    right_role: reviewer
    relation: different-principal
`,
	})
	other := mustDecode(t, shuffled)

	if !reflect.DeepEqual(base, other) {
		t.Errorf("normalized profiles differ:\nbase:  %+v\nother: %+v", base, other)
	}
	db, err := base.Digest()
	if err != nil {
		t.Fatalf("base digest: %v", err)
	}
	do, err := other.Digest()
	if err != nil {
		t.Fatalf("other digest: %v", err)
	}
	if db != do {
		t.Errorf("digests differ across equivalent input orderings: %s vs %s", db, do)
	}
}

// profileMutators mutates every authority-bearing profile family; shared
// by the content-sensitivity and seal-detection tests.
var profileMutators = []struct {
	name   string
	mutate func(*Profile)
}{
	{"id", func(p *Profile) { p.ID = "team-other" }},
	{"class", func(p *Profile) { p.Class = ClassHighAssurance }},
	{"applicable_transitions", func(p *Profile) { p.ApplicableTransitions = p.ApplicableTransitions[:2] }},
	{"trust_source_id", func(p *Profile) { p.IdentityTrustSources[0].ID = "other-owners" }},
	{"trust_source_kind", func(p *Profile) { p.IdentityTrustSources[0].Kind = TrustSourceForge }},
	{"role_mapping_role", func(p *Profile) { p.RoleMappings[0].Role = "owner" }},
	{"role_mapping_trust_source", func(p *Profile) { p.RoleMappings[0].TrustSource = "corporate-idp" }},
	{"role_mapping_subjects", func(p *Profile) { p.RoleMappings[0].Subjects = []string{"user-999"} }},
	{"ownership_id", func(p *Profile) { p.OwnershipSources[0].ID = "other" }},
	{"ownership_trust_source", func(p *Profile) { p.OwnershipSources[0].TrustSource = "corporate-idp" }},
	{"ownership_transitions", func(p *Profile) { p.OwnershipSources[0].Transitions = []string{"accept"} }},
	{"ownership_roles", func(p *Profile) { p.OwnershipSources[0].Roles = []string{"owner"} }},
	{"signature_transitions", func(p *Profile) { p.SignatureRequirements[0].Transitions = []string{"close"} }},
	{"signature_roles", func(p *Profile) { p.SignatureRequirements[0].Roles = []string{"reviewer"} }},
	{"signature_trust_sources", func(p *Profile) { p.SignatureRequirements[0].TrustSources = []string{"github"} }},
	{"approver_transitions", func(p *Profile) { p.RequiredApprovers[0].Transitions = []string{"accept"} }},
	{"approver_roles", func(p *Profile) { p.RequiredApprovers[0].Roles = []string{"owner"} }},
	{"approver_minimum", func(p *Profile) { p.RequiredApprovers[0].Minimum = 2 }},
	{"distinctness_transitions", func(p *Profile) { p.DistinctnessRules[0].Transitions = []string{"accept"} }},
	{"distinctness_left", func(p *Profile) { p.DistinctnessRules[0].LeftRole = "owner" }},
	{"distinctness_right", func(p *Profile) { p.DistinctnessRules[0].RightRole = "owner" }},
	{"distinctness_relation", func(p *Profile) { p.DistinctnessRules[0].Relation = RelationSamePrincipal }},
	{"evidence_transitions", func(p *Profile) { p.EvidenceSourceRestrictions[0].Transitions = []string{"accept"} }},
	{"evidence_allowed_sources", func(p *Profile) { p.EvidenceSourceRestrictions[0].AllowedSources = []string{"ci-verify"} }},
	{"escalation_transitions", func(p *Profile) { p.EscalationThresholds[0].Transitions = []string{"accept"} }},
	{"escalation_metric", func(p *Profile) { p.EscalationThresholds[0].Metric = "other-metric" }},
	{"escalation_at_least", func(p *Profile) { p.EscalationThresholds[0].AtLeast = 5 }},
	{"escalation_required_roles", func(p *Profile) { p.EscalationThresholds[0].RequiredRoles = []string{"reviewer"} }},
}

// TestProfileDigestSensitivity: the canonical content digest moves when
// any semantic field moves. Mutated values are compared through the
// unexported content digest because the public Digest refuses mutated
// profiles outright (see TestProfileSealDetectsMutation).
func TestProfileDigestSensitivity(t *testing.T) {
	for _, tt := range profileMutators {
		t.Run(tt.name, func(t *testing.T) {
			base := mustDecode(t, profileYAML())
			baseDigest, err := contentDigest(base)
			if err != nil {
				t.Fatalf("base digest: %v", err)
			}
			mutated := mustDecode(t, profileYAML())
			tt.mutate(&mutated)
			mutatedDigest, err := contentDigest(mutated)
			if err != nil {
				t.Fatalf("mutated digest: %v", err)
			}
			if baseDigest == mutatedDigest {
				t.Errorf("digest did not change when %s changed", tt.name)
			}
		})
	}
}
