package policyartifact

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// Overlay is one policy-overlay artifact (DC-3: an overlay may refine
// ONLY surfaces its governing policy declares overridable, within the
// permitted refinement boundary — specificity alone never changes
// authority). Whether each refinement is actually permitted and narrow
// is proven against the governing policy at effective-policy resolution;
// this type owns the self-contained grammar.
type Overlay struct {
	Schema      string          `json:"schema"`
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Owners      []string        `json:"owners"`
	Refines     string          `json:"refines"`
	Scope       Scope           `json:"scope"`
	Refinements []Refinement    `json:"refinements"`
	Template    *TemplateRecord `json:"template,omitempty"`
	Rationale   string          `json:"rationale"`

	seal string
}

// Refinement narrows one overridable claim of the governing policy:
// exactly one operand (values for set/scalar operators, bound for
// minimum/maximum) replaces the claim's, and only in the narrowing
// direction.
type Refinement struct {
	Claim  string   `yaml:"claim" json:"claim"`
	Values []string `yaml:"values,omitempty" json:"values,omitempty"`
	Bound  *int     `yaml:"bound,omitempty" json:"bound,omitempty"`
}

type refinementDoc struct {
	Claim  *string   `yaml:"claim"`
	Values *[]string `yaml:"values"`
	Bound  *int      `yaml:"bound"`
}

type overlayDoc struct {
	kernelDoc   `yaml:",inline"`
	Refines     *string          `yaml:"refines"`
	Scope       *scopeDoc        `yaml:"scope"`
	Refinements *[]refinementDoc `yaml:"refinements"`
}

// DecodeOverlay strictly decodes data as a verdi.policy-overlay/v1
// artifact, validates its grammar, normalizes it, and seals the result.
func DecodeOverlay(data []byte) (*Overlay, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	var doc overlayDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	k, err := doc.toKernel(SchemaOverlay, KindOverlay)
	if err != nil {
		return nil, err
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: overlay field %s is missing: every overlay field is mandatory", field)
	}
	if doc.Refines == nil {
		return nil, missing("refines")
	}
	if doc.Scope == nil {
		return nil, missing("scope")
	}
	if doc.Refinements == nil {
		return nil, missing("refinements")
	}

	if _, err := parseKindedID(*doc.Refines, KindPolicy); err != nil {
		return nil, fmt.Errorf("policyartifact: overlay refines %q: must reference a policy/<name> id: %w", *doc.Refines, err)
	}

	scope, err := doc.Scope.toScope("overlay.scope")
	if err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	if len(*doc.Refinements) == 0 {
		return nil, fmt.Errorf("policyartifact: overlay refinements must carry at least one refinement")
	}
	refinements := make([]Refinement, 0, len(*doc.Refinements))
	seen := make(map[string]bool, len(*doc.Refinements))
	for i, rd := range *doc.Refinements {
		if rd.Claim == nil || *rd.Claim == "" {
			return nil, fmt.Errorf("policyartifact: overlay refinements[%d]: claim is required", i)
		}
		if !kebabRe.MatchString(*rd.Claim) {
			return nil, fmt.Errorf("policyartifact: overlay refinements[%d]: claim %q must be kebab-case", i, *rd.Claim)
		}
		if seen[*rd.Claim] {
			return nil, fmt.Errorf("policyartifact: overlay refinements: duplicate claim %q", *rd.Claim)
		}
		seen[*rd.Claim] = true
		hasValues := rd.Values != nil
		hasBound := rd.Bound != nil
		if !hasValues && !hasBound {
			return nil, fmt.Errorf("policyartifact: overlay refinements[%d] (%s): a refinement must carry an operand (values or bound)", i, *rd.Claim)
		}
		if hasValues && hasBound {
			return nil, fmt.Errorf("policyartifact: overlay refinements[%d] (%s): a refinement carries exactly one operand, not both values and bound", i, *rd.Claim)
		}
		r := Refinement{Claim: *rd.Claim, Bound: rd.Bound}
		if hasValues {
			vals := emptyIfNil(*rd.Values)
			if len(vals) == 0 {
				return nil, fmt.Errorf("policyartifact: overlay refinements[%d] (%s): values must be nonempty", i, *rd.Claim)
			}
			if err := uniqueSet(fmt.Sprintf("overlay.refinements[%d].values", i), vals, func(v string) error {
				if v == "" {
					return fmt.Errorf("empty value")
				}
				return nil
			}); err != nil {
				return nil, err
			}
			r.Values = vals
		}
		refinements = append(refinements, r)
	}

	rationale, err := requireRationale(KindOverlay, body)
	if err != nil {
		return nil, err
	}

	o := &Overlay{
		Schema:      k.Schema,
		ID:          k.ID,
		Kind:        k.Kind,
		Title:       k.Title,
		Owners:      k.Owners,
		Refines:     *doc.Refines,
		Scope:       scope,
		Refinements: refinements,
		Template:    k.Template,
		Rationale:   rationale,
	}
	normalizeScope(&o.Scope)
	for i := range o.Refinements {
		sort.Strings(o.Refinements[i].Values)
	}
	sort.Slice(o.Refinements, func(i, j int) bool { return o.Refinements[i].Claim < o.Refinements[j].Claim })
	seal, err := canonjson.Digest(o)
	if err != nil {
		return nil, err
	}
	o.seal = seal
	return o, nil
}

// Name returns the overlay id's name half.
func (o *Overlay) Name() string { return nameOf(o.ID) }

// Digest returns the overlay's canonical content address after proving
// the value is unmodified DecodeOverlay output.
func (o *Overlay) Digest() (string, error) {
	if err := o.checkSeal(); err != nil {
		return "", err
	}
	return o.seal, nil
}

func (o *Overlay) checkSeal() error {
	return checkSealed("overlay", o.ID, o.seal, o)
}
