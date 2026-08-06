// Package governanceprincipal is the shared governance-principal kernel:
// the single strict governance-profile schema, canonical principal
// identifiers and resolution, advisory attribution records, and the one
// authorization interpretation every governed surface consumes (GLG v3
// AC-3, DC-16..DC-25; CI v2 AC-1, DC-17..DC-23; owner rulings OD-1/OD-2).
//
// The kernel owns implementation only. It never reads ambient identity
// ($USER, Git display names, environment, network, repository
// configuration): adapters supply normalized trust facts through the
// TrustFactReader port, and only this package turns facts into
// authentication and authorization results (GLG DC-22). Profile storage,
// effective-policy resolution, lifecycle requirements, and every feature
// consumer stay with their owning authorities (GLG DC-25).
package governanceprincipal

import (
	"fmt"
	"regexp"
)

// SchemaID is the only accepted governance-profile schema identifier.
// Unknown schema versions fail closed.
const SchemaID = "verdi.governance-profile/v1"

// Class is the closed governance-profile class vocabulary (GLG DC-17).
type Class string

// The four profile classes. Unknown classes fail closed.
const (
	ClassSolo          Class = "solo"
	ClassTeam          Class = "team"
	ClassHighAssurance Class = "high-assurance"
	ClassExperimental  Class = "experimental"
)

// Validate fails closed on any class outside the vocabulary.
func (c Class) Validate() error {
	switch c {
	case ClassSolo, ClassTeam, ClassHighAssurance, ClassExperimental:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown profile class %q", string(c))
}

// TrustSourceKind is the closed identity-trust-source kind vocabulary.
type TrustSourceKind string

// The four trust-source kinds. Unknown kinds fail closed.
const (
	TrustSourceForge            TrustSourceKind = "forge"
	TrustSourceSignedCommit     TrustSourceKind = "signed-commit"
	TrustSourceOwnership        TrustSourceKind = "ownership"
	TrustSourceIdentityProvider TrustSourceKind = "identity-provider"
)

// Validate fails closed on any kind outside the vocabulary.
func (k TrustSourceKind) Validate() error {
	switch k {
	case TrustSourceForge, TrustSourceSignedCommit, TrustSourceOwnership, TrustSourceIdentityProvider:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown trust-source kind %q", string(k))
}

// DistinctnessRelation is the closed distinctness-relation vocabulary.
type DistinctnessRelation string

// The two distinctness relations. Unknown relations fail closed.
const (
	RelationSamePrincipal      DistinctnessRelation = "same-principal"
	RelationDifferentPrincipal DistinctnessRelation = "different-principal"
)

// Validate fails closed on any relation outside the vocabulary.
func (r DistinctnessRelation) Validate() error {
	switch r {
	case RelationSamePrincipal, RelationDifferentPrincipal:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown distinctness relation %q", string(r))
}

// idRe is the repository slug posture for the profile ID and every
// profile-local ID: lowercase ASCII beginning with a letter, followed by
// lowercase letters, digits, '.', '_', or '-'.
var idRe = regexp.MustCompile(`^[a-z][a-z0-9._-]*$`)

// ValidateID checks id against the profile-local ID grammar.
func ValidateID(id string) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("governanceprincipal: invalid id %q: must be lowercase ASCII starting with a letter, then [a-z0-9._-]", id)
	}
	return nil
}

// Catalog is the injected closed universe a profile is validated against:
// roles, transitions, evidence sources, and escalation metrics. The kernel
// never defines these names itself (GLG DC-25); callers construct the
// catalog. Every profile reference outside the catalog fails closed.
type Catalog struct {
	Roles             []string
	Transitions       []string
	EvidenceSources   []string
	EscalationMetrics []string
}

// Validate checks that every catalog entry is a well-formed ID and that no
// set carries duplicates. A malformed catalog is an operational error.
func (c Catalog) Validate() error {
	for _, set := range []struct {
		name    string
		entries []string
	}{
		{"roles", c.Roles},
		{"transitions", c.Transitions},
		{"evidence sources", c.EvidenceSources},
		{"escalation metrics", c.EscalationMetrics},
	} {
		seen := make(map[string]bool, len(set.entries))
		for _, e := range set.entries {
			if err := ValidateID(e); err != nil {
				return fmt.Errorf("governanceprincipal: catalog %s: %w", set.name, err)
			}
			if seen[e] {
				return fmt.Errorf("governanceprincipal: catalog %s: duplicate entry %q", set.name, e)
			}
			seen[e] = true
		}
	}
	return nil
}

func (c Catalog) hasRole(role string) bool          { return contains(c.Roles, role) }
func (c Catalog) hasTransition(t string) bool       { return contains(c.Transitions, t) }
func (c Catalog) hasEvidenceSource(s string) bool   { return contains(c.EvidenceSources, s) }
func (c Catalog) hasEscalationMetric(m string) bool { return contains(c.EscalationMetrics, m) }

func contains(set []string, v string) bool {
	for _, e := range set {
		if e == v {
			return true
		}
	}
	return false
}

// TrustSource declares one acceptable identity trust source.
type TrustSource struct {
	ID   string          `yaml:"id" json:"id"`
	Kind TrustSourceKind `yaml:"kind" json:"kind"`
}

// RoleMapping maps authenticated subjects of one trust source to a role.
type RoleMapping struct {
	Role        string   `yaml:"role" json:"role"`
	TrustSource string   `yaml:"trust_source" json:"trust_source"`
	Subjects    []string `yaml:"subjects" json:"subjects"`
}

// OwnershipSource requires proven ownership from one ownership-kind trust
// source for the named roles at the named transitions.
type OwnershipSource struct {
	ID          string   `yaml:"id" json:"id"`
	TrustSource string   `yaml:"trust_source" json:"trust_source"`
	Transitions []string `yaml:"transitions" json:"transitions"`
	Roles       []string `yaml:"roles" json:"roles"`
}

// SignatureRequirement requires proven signature evidence from the named
// signed-commit trust sources for the named roles at the named transitions.
type SignatureRequirement struct {
	Transitions  []string `yaml:"transitions" json:"transitions"`
	Roles        []string `yaml:"roles" json:"roles"`
	TrustSources []string `yaml:"trust_sources" json:"trust_sources"`
}

// ApproverRequirement requires a minimum count of distinct authenticated
// principals filling the named roles at the named transitions.
type ApproverRequirement struct {
	Transitions []string `yaml:"transitions" json:"transitions"`
	Roles       []string `yaml:"roles" json:"roles"`
	Minimum     int      `yaml:"minimum" json:"minimum"`
}

// DistinctnessRule constrains the principals filling two distinct role
// fields at the named transitions.
type DistinctnessRule struct {
	Transitions []string             `yaml:"transitions" json:"transitions"`
	LeftRole    string               `yaml:"left_role" json:"left_role"`
	RightRole   string               `yaml:"right_role" json:"right_role"`
	Relation    DistinctnessRelation `yaml:"relation" json:"relation"`
}

// EvidenceSourceRestriction closes the set of evidence sources a
// transition may consume.
type EvidenceSourceRestriction struct {
	Transitions    []string `yaml:"transitions" json:"transitions"`
	AllowedSources []string `yaml:"allowed_sources" json:"allowed_sources"`
}

// EscalationThreshold makes every required role a required authenticated
// approver once a metric reaches its threshold at the named transitions.
type EscalationThreshold struct {
	Transitions   []string `yaml:"transitions" json:"transitions"`
	Metric        string   `yaml:"metric" json:"metric"`
	AtLeast       int      `yaml:"at_least" json:"at_least"`
	RequiredRoles []string `yaml:"required_roles" json:"required_roles"`
}

// Profile is one decoded, validated, normalized governance profile. Every
// set-like list is in deterministic normalized order, so two semantically
// equivalent profiles are equal values with equal digests. The artifact
// carries no self-declared digest field; Digest computes it.
type Profile struct {
	Schema                     string                      `json:"schema"`
	ID                         string                      `json:"id"`
	Class                      Class                       `json:"class"`
	ApplicableTransitions      []string                    `json:"applicable_transitions"`
	IdentityTrustSources       []TrustSource               `json:"identity_trust_sources"`
	RoleMappings               []RoleMapping               `json:"role_mappings"`
	OwnershipSources           []OwnershipSource           `json:"ownership_sources"`
	SignatureRequirements      []SignatureRequirement      `json:"signature_requirements"`
	RequiredApprovers          []ApproverRequirement       `json:"required_approvers"`
	DistinctnessRules          []DistinctnessRule          `json:"distinctness_rules"`
	EvidenceSourceRestrictions []EvidenceSourceRestriction `json:"evidence_source_restrictions"`
	EscalationThresholds       []EscalationThreshold       `json:"escalation_thresholds"`
}

// trustSource resolves a profile-local trust-source reference.
func (p Profile) trustSource(id string) (TrustSource, bool) {
	for _, ts := range p.IdentityTrustSources {
		if ts.ID == id {
			return ts, true
		}
	}
	return TrustSource{}, false
}
