package governanceprincipal

import (
	"fmt"
	"unicode/utf8"
)

// validateProfile enforces the profile validation rules against the
// injected catalog before normalization. Every failure is an operational
// decode error: an invalid profile never becomes a Profile value.
func validateProfile(p Profile, catalog Catalog) error {
	if p.Schema != SchemaID {
		return fmt.Errorf("governanceprincipal: unknown profile schema %q: only %q is accepted", p.Schema, SchemaID)
	}
	if err := ValidateID(p.ID); err != nil {
		return fmt.Errorf("governanceprincipal: profile id: %w", err)
	}
	if err := p.Class.Validate(); err != nil {
		return err
	}

	if len(p.ApplicableTransitions) == 0 {
		return fmt.Errorf("governanceprincipal: applicable_transitions must be nonempty")
	}
	if err := uniqueCatalogSet("applicable_transitions", p.ApplicableTransitions, catalog.hasTransition, "transition"); err != nil {
		return err
	}

	if err := validateTrustSources(p); err != nil {
		return err
	}
	if err := validateRoleMappings(p, catalog); err != nil {
		return err
	}
	if err := validateOwnershipSources(p, catalog); err != nil {
		return err
	}
	if err := validateSignatureRequirements(p, catalog); err != nil {
		return err
	}
	if err := validateRequiredApprovers(p, catalog); err != nil {
		return err
	}
	if err := validateDistinctnessRules(p, catalog); err != nil {
		return err
	}
	if err := validateEvidenceSourceRestrictions(p, catalog); err != nil {
		return err
	}
	if err := validateEscalationThresholds(p, catalog); err != nil {
		return err
	}
	return validateClassCoverage(p)
}

// uniqueCatalogSet checks one flat list for member uniqueness and catalog
// membership.
func uniqueCatalogSet(field string, members []string, inCatalog func(string) bool, kind string) error {
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		if !inCatalog(m) {
			return fmt.Errorf("governanceprincipal: %s: unknown %s %q: not in the injected catalog", field, kind, m)
		}
		if seen[m] {
			return fmt.Errorf("governanceprincipal: %s: duplicate %s %q", field, kind, m)
		}
		seen[m] = true
	}
	return nil
}

// nonemptyUniqueCatalogSet additionally rejects an empty member list.
func nonemptyUniqueCatalogSet(field string, members []string, inCatalog func(string) bool, kind string) error {
	if len(members) == 0 {
		return fmt.Errorf("governanceprincipal: %s must be nonempty", field)
	}
	return uniqueCatalogSet(field, members, inCatalog, kind)
}

func validateTrustSources(p Profile) error {
	seen := make(map[string]bool, len(p.IdentityTrustSources))
	for i, ts := range p.IdentityTrustSources {
		if err := ValidateID(ts.ID); err != nil {
			return fmt.Errorf("governanceprincipal: identity_trust_sources[%d]: %w", i, err)
		}
		if err := ts.Kind.Validate(); err != nil {
			return fmt.Errorf("governanceprincipal: identity_trust_sources[%d]: %w", i, err)
		}
		if seen[ts.ID] {
			return fmt.Errorf("governanceprincipal: identity_trust_sources: duplicate id %q", ts.ID)
		}
		seen[ts.ID] = true
	}
	return nil
}

// resolveTrustSource resolves a trust-source reference and, when wantKind
// is nonempty, requires that kind.
func resolveTrustSource(p Profile, field, ref string, wantKind TrustSourceKind) error {
	ts, ok := p.trustSource(ref)
	if !ok {
		return fmt.Errorf("governanceprincipal: %s: trust source %q does not resolve within identity_trust_sources", field, ref)
	}
	if wantKind != "" && ts.Kind != wantKind {
		return fmt.Errorf("governanceprincipal: %s: trust source %q has kind %q, want kind %q", field, ref, ts.Kind, wantKind)
	}
	return nil
}

func validateRoleMappings(p Profile, catalog Catalog) error {
	seen := make(map[[2]string]bool, len(p.RoleMappings))
	for i, m := range p.RoleMappings {
		field := fmt.Sprintf("role_mappings[%d]", i)
		if !catalog.hasRole(m.Role) {
			return fmt.Errorf("governanceprincipal: %s: unknown role %q: not in the injected catalog", field, m.Role)
		}
		if err := resolveTrustSource(p, field, m.TrustSource, ""); err != nil {
			return err
		}
		key := [2]string{m.Role, m.TrustSource}
		if seen[key] {
			return fmt.Errorf("governanceprincipal: role_mappings: duplicate mapping for role %q and trust source %q", m.Role, m.TrustSource)
		}
		seen[key] = true
		if len(m.Subjects) == 0 {
			return fmt.Errorf("governanceprincipal: %s: subjects must be nonempty", field)
		}
		subjects := make(map[string]bool, len(m.Subjects))
		for _, s := range m.Subjects {
			if s == "" || !utf8.ValidString(s) {
				return fmt.Errorf("governanceprincipal: %s: subject must be a nonempty valid UTF-8 stable adapter subject", field)
			}
			if subjects[s] {
				return fmt.Errorf("governanceprincipal: %s: duplicate subject %q", field, s)
			}
			subjects[s] = true
		}
	}
	return nil
}

func validateOwnershipSources(p Profile, catalog Catalog) error {
	seen := make(map[string]bool, len(p.OwnershipSources))
	for i, o := range p.OwnershipSources {
		field := fmt.Sprintf("ownership_sources[%d]", i)
		if err := ValidateID(o.ID); err != nil {
			return fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if seen[o.ID] {
			return fmt.Errorf("governanceprincipal: ownership_sources: duplicate id %q", o.ID)
		}
		seen[o.ID] = true
		if err := resolveTrustSource(p, field, o.TrustSource, TrustSourceOwnership); err != nil {
			return err
		}
		if err := nonemptyUniqueCatalogSet(field+".transitions", o.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		if err := nonemptyUniqueCatalogSet(field+".roles", o.Roles, catalog.hasRole, "role"); err != nil {
			return err
		}
	}
	return nil
}

func validateSignatureRequirements(p Profile, catalog Catalog) error {
	seen := make(map[[2]string]bool)
	for i, s := range p.SignatureRequirements {
		field := fmt.Sprintf("signature_requirements[%d]", i)
		if err := nonemptyUniqueCatalogSet(field+".transitions", s.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		if err := nonemptyUniqueCatalogSet(field+".roles", s.Roles, catalog.hasRole, "role"); err != nil {
			return err
		}
		if len(s.TrustSources) == 0 {
			return fmt.Errorf("governanceprincipal: %s.trust_sources must be nonempty", field)
		}
		sources := make(map[string]bool, len(s.TrustSources))
		for _, ref := range s.TrustSources {
			if err := resolveTrustSource(p, field, ref, TrustSourceSignedCommit); err != nil {
				return err
			}
			if sources[ref] {
				return fmt.Errorf("governanceprincipal: %s: duplicate trust source %q", field, ref)
			}
			sources[ref] = true
		}
		for _, t := range s.Transitions {
			for _, r := range s.Roles {
				key := [2]string{t, r}
				if seen[key] {
					return fmt.Errorf("governanceprincipal: signature_requirements: duplicate rule for transition %q and role %q", t, r)
				}
				seen[key] = true
			}
		}
	}
	return nil
}

func validateRequiredApprovers(p Profile, catalog Catalog) error {
	seen := make(map[[2]string]bool)
	for i, a := range p.RequiredApprovers {
		field := fmt.Sprintf("required_approvers[%d]", i)
		if err := nonemptyUniqueCatalogSet(field+".transitions", a.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		if err := nonemptyUniqueCatalogSet(field+".roles", a.Roles, catalog.hasRole, "role"); err != nil {
			return err
		}
		if a.Minimum <= 0 {
			return fmt.Errorf("governanceprincipal: %s: minimum must be positive, got %d", field, a.Minimum)
		}
		for _, t := range a.Transitions {
			for _, r := range a.Roles {
				key := [2]string{t, r}
				if seen[key] {
					return fmt.Errorf("governanceprincipal: required_approvers: duplicate rule for transition %q and role %q", t, r)
				}
				seen[key] = true
			}
		}
	}
	return nil
}

func validateDistinctnessRules(p Profile, catalog Catalog) error {
	seen := make(map[[3]string]bool)
	for i, d := range p.DistinctnessRules {
		field := fmt.Sprintf("distinctness_rules[%d]", i)
		if err := nonemptyUniqueCatalogSet(field+".transitions", d.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		for _, role := range []string{d.LeftRole, d.RightRole} {
			if !catalog.hasRole(role) {
				return fmt.Errorf("governanceprincipal: %s: unknown role %q: not in the injected catalog", field, role)
			}
		}
		if d.LeftRole == d.RightRole {
			return fmt.Errorf("governanceprincipal: %s: left_role and right_role must be different field references even for relation %q", field, d.Relation)
		}
		if err := d.Relation.Validate(); err != nil {
			return fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		for _, t := range d.Transitions {
			key := [3]string{t, d.LeftRole, d.RightRole}
			if seen[key] {
				return fmt.Errorf("governanceprincipal: distinctness_rules: duplicate rule for transition %q and roles %q/%q", t, d.LeftRole, d.RightRole)
			}
			seen[key] = true
		}
	}
	return nil
}

func validateEvidenceSourceRestrictions(p Profile, catalog Catalog) error {
	seen := make(map[string]bool)
	for i, e := range p.EvidenceSourceRestrictions {
		field := fmt.Sprintf("evidence_source_restrictions[%d]", i)
		if err := nonemptyUniqueCatalogSet(field+".transitions", e.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		// allowed_sources may be empty: an empty allowed set forbids every
		// presented source, which is a meaningful fail-closed restriction.
		if err := uniqueCatalogSet(field+".allowed_sources", e.AllowedSources, catalog.hasEvidenceSource, "evidence source"); err != nil {
			return err
		}
		for _, t := range e.Transitions {
			if seen[t] {
				return fmt.Errorf("governanceprincipal: evidence_source_restrictions: duplicate restriction for transition %q", t)
			}
			seen[t] = true
		}
	}
	return nil
}

func validateEscalationThresholds(p Profile, catalog Catalog) error {
	seen := make(map[[2]string]bool)
	for i, e := range p.EscalationThresholds {
		field := fmt.Sprintf("escalation_thresholds[%d]", i)
		if err := nonemptyUniqueCatalogSet(field+".transitions", e.Transitions, catalog.hasTransition, "transition"); err != nil {
			return err
		}
		if !catalog.hasEscalationMetric(e.Metric) {
			return fmt.Errorf("governanceprincipal: %s: unknown escalation metric %q: not in the injected catalog", field, e.Metric)
		}
		if e.AtLeast < 0 {
			return fmt.Errorf("governanceprincipal: %s: at_least must be nonnegative, got %d", field, e.AtLeast)
		}
		if err := nonemptyUniqueCatalogSet(field+".required_roles", e.RequiredRoles, catalog.hasRole, "role"); err != nil {
			return err
		}
		for _, t := range e.Transitions {
			key := [2]string{t, e.Metric}
			if seen[key] {
				return fmt.Errorf("governanceprincipal: escalation_thresholds: duplicate threshold for transition %q and metric %q", t, e.Metric)
			}
			seen[key] = true
		}
	}
	return nil
}

// validateClassCoverage enforces the class-specific rule coverage: team
// profiles need an approval rule and a different-principal rule for every
// applicable transition; high-assurance profiles additionally need
// signature, ownership, and evidence-source rules for every applicable
// transition. Solo and experimental profiles may keep every rule list
// explicitly empty.
func validateClassCoverage(p Profile) error {
	if p.Class != ClassTeam && p.Class != ClassHighAssurance {
		return nil
	}
	covered := func(transitions []string, t string) bool { return contains(transitions, t) }
	for _, t := range p.ApplicableTransitions {
		hasApproval := false
		for _, a := range p.RequiredApprovers {
			if covered(a.Transitions, t) {
				hasApproval = true
				break
			}
		}
		if !hasApproval {
			return fmt.Errorf("governanceprincipal: %s profile: no approval rule covers applicable transition %q", p.Class, t)
		}
		hasDifferent := false
		for _, d := range p.DistinctnessRules {
			if d.Relation == RelationDifferentPrincipal && covered(d.Transitions, t) {
				hasDifferent = true
				break
			}
		}
		if !hasDifferent {
			return fmt.Errorf("governanceprincipal: %s profile: no different-principal rule covers applicable transition %q", p.Class, t)
		}
		if p.Class != ClassHighAssurance {
			continue
		}
		hasSignature := false
		for _, s := range p.SignatureRequirements {
			if covered(s.Transitions, t) {
				hasSignature = true
				break
			}
		}
		if !hasSignature {
			return fmt.Errorf("governanceprincipal: high-assurance profile: no signature rule covers applicable transition %q", t)
		}
		hasOwnership := false
		for _, o := range p.OwnershipSources {
			if covered(o.Transitions, t) {
				hasOwnership = true
				break
			}
		}
		if !hasOwnership {
			return fmt.Errorf("governanceprincipal: high-assurance profile: no ownership rule covers applicable transition %q", t)
		}
		hasEvidence := false
		for _, e := range p.EvidenceSourceRestrictions {
			if covered(e.Transitions, t) {
				hasEvidence = true
				break
			}
		}
		if !hasEvidence {
			return fmt.Errorf("governanceprincipal: high-assurance profile: no evidence-source rule covers applicable transition %q", t)
		}
	}
	return nil
}
