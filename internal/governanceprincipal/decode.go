package governanceprincipal

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// profileDoc is the strict decode target. Every top-level field is a
// pointer so an omitted (or null) field is distinguishable from an
// explicitly empty list: omission is invalid; `[]` is a present, empty
// set where the class permits it.
type profileDoc struct {
	Schema                     *string                      `yaml:"schema"`
	ID                         *string                      `yaml:"id"`
	Class                      *string                      `yaml:"class"`
	ApplicableTransitions      *[]string                    `yaml:"applicable_transitions"`
	IdentityTrustSources       *[]TrustSource               `yaml:"identity_trust_sources"`
	RoleMappings               *[]RoleMapping               `yaml:"role_mappings"`
	OwnershipSources           *[]OwnershipSource           `yaml:"ownership_sources"`
	SignatureRequirements      *[]SignatureRequirement      `yaml:"signature_requirements"`
	RequiredApprovers          *[]ApproverRequirement       `yaml:"required_approvers"`
	DistinctnessRules          *[]DistinctnessRule          `yaml:"distinctness_rules"`
	EvidenceSourceRestrictions *[]EvidenceSourceRestriction `yaml:"evidence_source_restrictions"`
	EscalationThresholds       *[]escalationDoc             `yaml:"escalation_thresholds"`
}

// escalationDoc mirrors EscalationThreshold with a pointer at_least so a
// missing threshold value is distinguishable from an explicit zero.
type escalationDoc struct {
	Transitions   []string `yaml:"transitions"`
	Metric        string   `yaml:"metric"`
	AtLeast       *int     `yaml:"at_least"`
	RequiredRoles []string `yaml:"required_roles"`
}

// DecodeProfile strictly decodes raw as a governance-profile artifact,
// validates it against the injected catalog, and returns the normalized
// copy: every set-like list ordered by stable IDs and then by complete
// field content, so semantically equivalent inputs produce the same value
// and digest. Unknown fields, schema versions, classes, kinds, relations,
// YAML anchors, aliases, and custom tags fail closed through the single
// internal/artifact strict-decode seam.
func DecodeProfile(raw []byte, catalog Catalog) (Profile, error) {
	if err := catalog.Validate(); err != nil {
		return Profile{}, err
	}

	var doc profileDoc
	if err := artifact.DecodeStrict(raw, &doc); err != nil {
		return Profile{}, fmt.Errorf("governanceprincipal: decoding profile: %w", err)
	}

	p, err := docToProfile(doc)
	if err != nil {
		return Profile{}, err
	}
	if err := validateProfile(p, catalog); err != nil {
		return Profile{}, err
	}
	if err := normalizeProfile(&p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

// docToProfile enforces top-level field presence and materializes the
// normalized value type.
func docToProfile(doc profileDoc) (Profile, error) {
	missing := func(field string) error {
		return fmt.Errorf("governanceprincipal: profile field %s is missing: every top-level field is mandatory (an explicitly empty list is [])", field)
	}
	switch {
	case doc.Schema == nil:
		return Profile{}, missing("schema")
	case doc.ID == nil:
		return Profile{}, missing("id")
	case doc.Class == nil:
		return Profile{}, missing("class")
	case doc.ApplicableTransitions == nil:
		return Profile{}, missing("applicable_transitions")
	case doc.IdentityTrustSources == nil:
		return Profile{}, missing("identity_trust_sources")
	case doc.RoleMappings == nil:
		return Profile{}, missing("role_mappings")
	case doc.OwnershipSources == nil:
		return Profile{}, missing("ownership_sources")
	case doc.SignatureRequirements == nil:
		return Profile{}, missing("signature_requirements")
	case doc.RequiredApprovers == nil:
		return Profile{}, missing("required_approvers")
	case doc.DistinctnessRules == nil:
		return Profile{}, missing("distinctness_rules")
	case doc.EvidenceSourceRestrictions == nil:
		return Profile{}, missing("evidence_source_restrictions")
	case doc.EscalationThresholds == nil:
		return Profile{}, missing("escalation_thresholds")
	}

	thresholds := make([]EscalationThreshold, 0, len(*doc.EscalationThresholds))
	for i, e := range *doc.EscalationThresholds {
		if e.AtLeast == nil {
			return Profile{}, fmt.Errorf("governanceprincipal: escalation_thresholds[%d]: at_least is missing", i)
		}
		thresholds = append(thresholds, EscalationThreshold{
			Transitions:   e.Transitions,
			Metric:        e.Metric,
			AtLeast:       *e.AtLeast,
			RequiredRoles: e.RequiredRoles,
		})
	}

	return Profile{
		Schema:                     *doc.Schema,
		ID:                         *doc.ID,
		Class:                      Class(*doc.Class),
		ApplicableTransitions:      *doc.ApplicableTransitions,
		IdentityTrustSources:       *doc.IdentityTrustSources,
		RoleMappings:               *doc.RoleMappings,
		OwnershipSources:           *doc.OwnershipSources,
		SignatureRequirements:      *doc.SignatureRequirements,
		RequiredApprovers:          *doc.RequiredApprovers,
		DistinctnessRules:          *doc.DistinctnessRules,
		EvidenceSourceRestrictions: *doc.EvidenceSourceRestrictions,
		EscalationThresholds:       thresholds,
	}, nil
}

// Digest returns the profile's content address: sha256 over the canonical
// JSON encoding of the normalized profile (internal/canonjson).
func (p Profile) Digest() (string, error) {
	return canonjson.Digest(p)
}

// normalizeProfile sorts every set-like list deterministically: entries
// with stable IDs by those IDs, everything else by complete field content
// (canonical JSON bytes). Nothing in the profile represents ordered
// evidence, so every list is a semantic set.
func normalizeProfile(p *Profile) error {
	sort.Strings(p.ApplicableTransitions)

	sort.Slice(p.IdentityTrustSources, func(i, j int) bool {
		return p.IdentityTrustSources[i].ID < p.IdentityTrustSources[j].ID
	})

	for i := range p.RoleMappings {
		sort.Strings(p.RoleMappings[i].Subjects)
	}
	sort.Slice(p.RoleMappings, func(i, j int) bool {
		a, b := p.RoleMappings[i], p.RoleMappings[j]
		if a.Role != b.Role {
			return a.Role < b.Role
		}
		return a.TrustSource < b.TrustSource
	})

	for i := range p.OwnershipSources {
		sort.Strings(p.OwnershipSources[i].Transitions)
		sort.Strings(p.OwnershipSources[i].Roles)
	}
	sort.Slice(p.OwnershipSources, func(i, j int) bool {
		return p.OwnershipSources[i].ID < p.OwnershipSources[j].ID
	})

	for i := range p.SignatureRequirements {
		sort.Strings(p.SignatureRequirements[i].Transitions)
		sort.Strings(p.SignatureRequirements[i].Roles)
		sort.Strings(p.SignatureRequirements[i].TrustSources)
	}
	if err := sortByContent(p.SignatureRequirements); err != nil {
		return err
	}

	for i := range p.RequiredApprovers {
		sort.Strings(p.RequiredApprovers[i].Transitions)
		sort.Strings(p.RequiredApprovers[i].Roles)
	}
	if err := sortByContent(p.RequiredApprovers); err != nil {
		return err
	}

	for i := range p.DistinctnessRules {
		sort.Strings(p.DistinctnessRules[i].Transitions)
	}
	if err := sortByContent(p.DistinctnessRules); err != nil {
		return err
	}

	for i := range p.EvidenceSourceRestrictions {
		sort.Strings(p.EvidenceSourceRestrictions[i].Transitions)
		sort.Strings(p.EvidenceSourceRestrictions[i].AllowedSources)
	}
	if err := sortByContent(p.EvidenceSourceRestrictions); err != nil {
		return err
	}

	for i := range p.EscalationThresholds {
		sort.Strings(p.EscalationThresholds[i].Transitions)
		sort.Strings(p.EscalationThresholds[i].RequiredRoles)
	}
	return sortByContent(p.EscalationThresholds)
}

// sortByContent orders items by the canonical JSON encoding of each
// element — the "complete field content" tiebreak for set entries that
// carry no single stable ID.
func sortByContent[T any](items []T) error {
	keys := make([][]byte, len(items))
	for i, it := range items {
		b, err := canonjson.Marshal(it)
		if err != nil {
			return fmt.Errorf("governanceprincipal: normalizing profile entry: %w", err)
		}
		keys[i] = b
	}
	sort.Sort(&byContent[T]{items: items, keys: keys})
	return nil
}

type byContent[T any] struct {
	items []T
	keys  [][]byte
}

func (s *byContent[T]) Len() int           { return len(s.items) }
func (s *byContent[T]) Less(i, j int) bool { return bytes.Compare(s.keys[i], s.keys[j]) < 0 }
func (s *byContent[T]) Swap(i, j int) {
	s.items[i], s.items[j] = s.items[j], s.items[i]
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}
