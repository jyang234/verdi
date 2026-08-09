package policyartifact

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Family is one of the six Verdi-owned constraint families
// (spec/context-integrity-v2 AC-3 fixes the family set; this unit owns
// the typed catalog and registration shapes AC-1 needs, not Wave 3
// conflict evaluation). The set is closed: an unknown family fails
// decode (co-2).
type Family string

const (
	FamilyAction        Family = "action"
	FamilyConfiguration Family = "configuration"
	FamilyCapability    Family = "capability"
	FamilyResource      Family = "resource"
	FamilyIdentity      Family = "identity"
	FamilyEvidence      Family = "evidence"
)

var knownFamilies = map[Family]bool{
	FamilyAction:        true,
	FamilyConfiguration: true,
	FamilyCapability:    true,
	FamilyResource:      true,
	FamilyIdentity:      true,
	FamilyEvidence:      true,
}

// Validate reports whether f is one of the six Verdi-owned families.
func (f Family) Validate() error {
	if !knownFamilies[f] {
		return fmt.Errorf("policyartifact: unknown constraint family %q (known: action, configuration, capability, resource, identity, evidence)", f)
	}
	return nil
}

// Operator is one of the Verdi-owned comparison operators (AC-3's initial
// operator set, owned here as typed vocabulary; comparison SEMANTICS are
// Wave 3's conflict evaluator). Projects register subjects and values;
// they can never register an operator (DC-5).
type Operator string

const (
	OpEquals             Operator = "equals"
	OpNotEquals          Operator = "not-equals"
	OpAllowedValues      Operator = "allowed-values"
	OpRequiredValues     Operator = "required-values"
	OpForbiddenValues    Operator = "forbidden-values"
	OpMinimum            Operator = "minimum"
	OpMaximum            Operator = "maximum"
	OpSamePrincipal      Operator = "same-principal"
	OpDifferentPrincipal Operator = "different-principal"
	OpPathRead           Operator = "path-read"
	OpPathWrite          Operator = "path-write"
)

var knownOperators = map[Operator]bool{
	OpEquals: true, OpNotEquals: true,
	OpAllowedValues: true, OpRequiredValues: true, OpForbiddenValues: true,
	OpMinimum: true, OpMaximum: true,
	OpSamePrincipal: true, OpDifferentPrincipal: true,
	OpPathRead: true, OpPathWrite: true,
}

// Validate reports whether o is one of the known operators.
func (o Operator) Validate() error {
	if !knownOperators[o] {
		return fmt.Errorf("policyartifact: unknown operator %q", o)
	}
	return nil
}

// setValued reports whether o's Values operand is a semantic set (sorted
// at normalization) rather than positional content.
func (o Operator) setValued() bool {
	switch o {
	case OpAllowedValues, OpRequiredValues, OpForbiddenValues, OpPathRead, OpPathWrite:
		return true
	}
	return false
}

// Claim is one typed policy claim (AC-1: "typed claims"): a registered
// subject constrained by a Verdi-owned operator within a family, bounded
// by a scope, with an explicit overridable declaration (DC-3: an overlay
// may refine ONLY a surface the governing policy declares overridable —
// the default is not overridable, and the field is mandatory-explicit so
// silence never grants refinement).
type Claim struct {
	ID          string   `yaml:"id" json:"id"`
	Family      Family   `yaml:"family" json:"family"`
	Operator    Operator `yaml:"operator" json:"operator"`
	Subject     string   `yaml:"subject" json:"subject"`
	Values      []string `yaml:"values" json:"values"`
	Bound       *int     `yaml:"bound,omitempty" json:"bound,omitempty"`
	Scope       Scope    `yaml:"scope" json:"scope"`
	Overridable bool     `yaml:"overridable" json:"overridable"`
}

// claimDoc is Claim's strict decode target: pointers make every field's
// presence explicit.
type claimDoc struct {
	ID          *string   `yaml:"id"`
	Family      *string   `yaml:"family"`
	Operator    *string   `yaml:"operator"`
	Subject     *string   `yaml:"subject"`
	Values      *[]string `yaml:"values"`
	Bound       *int      `yaml:"bound"`
	Scope       *scopeDoc `yaml:"scope"`
	Overridable *bool     `yaml:"overridable"`
}

func (d claimDoc) toClaim(field string) (Claim, error) {
	missing := func(sub string) error {
		return fmt.Errorf("policyartifact: %s.%s is missing: every claim field except bound is mandatory (values' explicitly empty set is [])", field, sub)
	}
	switch {
	case d.ID == nil:
		return Claim{}, missing("id")
	case d.Family == nil:
		return Claim{}, missing("family")
	case d.Operator == nil:
		return Claim{}, missing("operator")
	case d.Subject == nil:
		return Claim{}, missing("subject")
	case d.Values == nil:
		return Claim{}, missing("values")
	case d.Scope == nil:
		return Claim{}, missing("scope")
	case d.Overridable == nil:
		return Claim{}, missing("overridable")
	}
	scope, err := d.Scope.toScope(field + ".scope")
	if err != nil {
		return Claim{}, err
	}
	return Claim{
		ID:          *d.ID,
		Family:      Family(*d.Family),
		Operator:    Operator(*d.Operator),
		Subject:     *d.Subject,
		Values:      emptyIfNil(*d.Values),
		Bound:       d.Bound,
		Scope:       scope,
		Overridable: *d.Overridable,
	}, nil
}

// Validate checks c's complete grammar: id and subject shapes, closed
// family and operator vocabularies, operator/operand agreement, the
// family/operator compatibility matrix, value-set grammar, and scope
// validity.
func (c Claim) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("policyartifact: claim id is required")
	}
	if !kebabRe.MatchString(c.ID) {
		return fmt.Errorf("policyartifact: claim id %q must be kebab-case", c.ID)
	}
	if err := c.Family.Validate(); err != nil {
		return err
	}
	if err := c.Operator.Validate(); err != nil {
		return err
	}
	if c.Subject == "" {
		return fmt.Errorf("policyartifact: claim %s: subject is required", c.ID)
	}
	if !kebabRe.MatchString(c.Subject) {
		return fmt.Errorf("policyartifact: claim %s: subject %q must be kebab-case", c.ID, c.Subject)
	}

	// Family/operator compatibility: the identity family admits exactly
	// the two principal operators, and they appear nowhere else — every
	// identity evaluation call belongs to the kernel's authorization
	// interpretation (DC-22), so this package fixes only the vocabulary.
	// The path operators name resource-family subjects exclusively.
	principalOp := c.Operator == OpSamePrincipal || c.Operator == OpDifferentPrincipal
	if c.Family == FamilyIdentity && !principalOp {
		return fmt.Errorf("policyartifact: claim %s: identity family admits only same-principal and different-principal operators, got %q", c.ID, c.Operator)
	}
	if principalOp && c.Family != FamilyIdentity {
		return fmt.Errorf("policyartifact: claim %s: operator %q is only valid in the identity family, got %q", c.ID, c.Operator, c.Family)
	}
	pathOp := c.Operator == OpPathRead || c.Operator == OpPathWrite
	if pathOp && c.Family != FamilyResource {
		return fmt.Errorf("policyartifact: claim %s: operator %q is only valid in the resource family, got %q", c.ID, c.Operator, c.Family)
	}

	// Operator/operand agreement.
	switch c.Operator {
	case OpEquals, OpNotEquals:
		if len(c.Values) != 1 {
			return fmt.Errorf("policyartifact: claim %s: operator %q requires exactly one value, got %d", c.ID, c.Operator, len(c.Values))
		}
		if c.Bound != nil {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes no bound", c.ID, c.Operator)
		}
	case OpAllowedValues, OpRequiredValues, OpForbiddenValues:
		if len(c.Values) == 0 {
			return fmt.Errorf("policyartifact: claim %s: operator %q requires at least one value", c.ID, c.Operator)
		}
		if c.Bound != nil {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes no bound", c.ID, c.Operator)
		}
	case OpMinimum, OpMaximum:
		if c.Bound == nil {
			return fmt.Errorf("policyartifact: claim %s: operator %q requires a bound", c.ID, c.Operator)
		}
		if len(c.Values) != 0 {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes a bound, not values", c.ID, c.Operator)
		}
	case OpSamePrincipal, OpDifferentPrincipal:
		if len(c.Values) != 0 {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes no values", c.ID, c.Operator)
		}
		if c.Bound != nil {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes no bound", c.ID, c.Operator)
		}
	case OpPathRead, OpPathWrite:
		if len(c.Values) == 0 {
			return fmt.Errorf("policyartifact: claim %s: operator %q requires at least one path value", c.ID, c.Operator)
		}
		if c.Bound != nil {
			return fmt.Errorf("policyartifact: claim %s: operator %q takes no bound", c.ID, c.Operator)
		}
		for _, v := range c.Values {
			if err := validateRelPath(v); err != nil {
				return fmt.Errorf("policyartifact: claim %s: %w", c.ID, err)
			}
		}
	}
	if c.Bound != nil && c.Operator != OpMinimum && c.Operator != OpMaximum {
		return fmt.Errorf("policyartifact: claim %s: bound is only valid with minimum/maximum", c.ID)
	}

	// Value-set grammar for set-valued operators: unique, nonempty
	// members (path operators validated their members above).
	if c.Operator.setValued() || c.Operator == OpEquals || c.Operator == OpNotEquals {
		seen := make(map[string]bool, len(c.Values))
		for _, v := range c.Values {
			if v == "" {
				return fmt.Errorf("policyartifact: claim %s: empty value", c.ID)
			}
			if seen[v] {
				return fmt.Errorf("policyartifact: claim %s: duplicate value %q", c.ID, v)
			}
			seen[v] = true
		}
	}

	if err := c.Scope.Validate(); err != nil {
		return fmt.Errorf("policyartifact: claim %s: %w", c.ID, err)
	}
	return nil
}

// normalizeClaim sorts c's semantic sets: set-valued operator operands
// and every scope dimension. Equals/not-equals values are positional
// (a single value) and left as-is.
func normalizeClaim(c *Claim) {
	if c.Values == nil {
		c.Values = []string{}
	}
	if c.Operator.setValued() {
		sort.Strings(c.Values)
	}
	normalizeScope(&c.Scope)
}

// ClaimDigest returns the canonical content address of a normalized
// claim — the exact witness identity an exemption or disposition binds
// to (DC-8's "exact witnesses").
func ClaimDigest(c Claim) (string, error) {
	return canonjson.Digest(c)
}
