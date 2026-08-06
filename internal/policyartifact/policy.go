package policyartifact

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// Policy is one canonical policy artifact (AC-1: policies state defaults
// and requirements; DC-3: overlays may refine only surfaces a policy
// declares overridable). Instructions are the policy's own ordered
// projection lines — authored content whose order is meaningful and
// digest-bound, unlike the semantic sets, which normalize sorted.
type Policy struct {
	Schema       string             `json:"schema"`
	ID           string             `json:"id"`
	Kind         string             `json:"kind"`
	Title        string             `json:"title"`
	Owners       []string           `json:"owners"`
	Scope        Scope              `json:"scope"`
	Claims       []Claim            `json:"claims"`
	Instructions []string           `json:"instructions"`
	Payloads     map[string]Payload `json:"payloads"`
	Template     *TemplateRecord    `json:"template,omitempty"`
	Rationale    string             `json:"rationale"`

	seal string
}

// policyDoc is the strict decode target for a policy artifact's
// frontmatter.
type policyDoc struct {
	kernelDoc    `yaml:",inline"`
	Scope        *scopeDoc            `yaml:"scope"`
	Claims       *[]claimDoc          `yaml:"claims"`
	Instructions *[]*string           `yaml:"instructions"`
	Payloads     map[string]yaml.Node `yaml:"payloads"`
}

// DecodePolicy strictly decodes data (a complete artifact file:
// frontmatter plus rationale body) as a verdi.policy/v1 artifact,
// validates it, normalizes its semantic sets, and seals the result.
func DecodePolicy(data []byte) (*Policy, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}

	// Presence of the payloads key must be distinguishable from an empty
	// map; decode it via a presence probe alongside the typed doc.
	var probe struct {
		Payloads *map[string]yaml.Node `yaml:"payloads"`
	}
	var doc policyDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(fm, &probe); err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}

	k, err := doc.kernelDoc.toKernel(SchemaPolicy, KindPolicy)
	if err != nil {
		return nil, err
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: policy field %s is missing: every policy field is mandatory (explicitly empty is [] / {})", field)
	}
	if doc.Scope == nil {
		return nil, missing("scope")
	}
	if doc.Claims == nil {
		return nil, missing("claims")
	}
	if doc.Instructions == nil {
		return nil, missing("instructions")
	}
	if probe.Payloads == nil {
		return nil, missing("payloads")
	}

	scope, err := doc.Scope.toScope("policy.scope")
	if err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	claims := make([]Claim, 0, len(*doc.Claims))
	seen := make(map[string]bool, len(*doc.Claims))
	for i, cd := range *doc.Claims {
		c, err := cd.toClaim(fmt.Sprintf("policy.claims[%d]", i))
		if err != nil {
			return nil, err
		}
		if err := c.Validate(); err != nil {
			return nil, err
		}
		if seen[c.ID] {
			return nil, fmt.Errorf("policyartifact: policy claims: duplicate claim id %q", c.ID)
		}
		seen[c.ID] = true
		claims = append(claims, c)
	}

	instructions := make([]string, 0, len(*doc.Instructions))
	for i, ins := range *doc.Instructions {
		if ins == nil || *ins == "" {
			return nil, fmt.Errorf("policyartifact: policy instructions[%d]: empty instruction", i)
		}
		if strings.ContainsAny(*ins, "\n\r") {
			return nil, fmt.Errorf("policyartifact: policy instructions[%d]: an instruction must be a single line", i)
		}
		instructions = append(instructions, *ins)
	}

	payloads, err := decodePayloads(doc.Payloads)
	if err != nil {
		return nil, err
	}

	rationale, err := requireRationale(KindPolicy, body)
	if err != nil {
		return nil, err
	}

	p := &Policy{
		Schema:       k.Schema,
		ID:           k.ID,
		Kind:         k.Kind,
		Title:        k.Title,
		Owners:       k.Owners,
		Scope:        scope,
		Claims:       claims,
		Instructions: instructions,
		Payloads:     payloads,
		Template:     k.Template,
		Rationale:    rationale,
	}
	normalizePolicy(p)
	seal, err := canonjson.Digest(p)
	if err != nil {
		return nil, err
	}
	p.seal = seal
	return p, nil
}

// normalizePolicy sorts the policy's semantic sets: owners (already
// sorted by the kernel), each claim's sets, and the claims themselves by
// id. Instructions keep author order — they are ordered content.
func normalizePolicy(p *Policy) {
	normalizeScope(&p.Scope)
	for i := range p.Claims {
		normalizeClaim(&p.Claims[i])
	}
	sort.Slice(p.Claims, func(i, j int) bool { return p.Claims[i].ID < p.Claims[j].ID })
	if p.Payloads == nil {
		p.Payloads = map[string]Payload{}
	}
	if p.Instructions == nil {
		p.Instructions = []string{}
	}
}

// Name returns the policy id's name half.
func (p *Policy) Name() string { return nameOf(p.ID) }

// Claim returns the claim named id, if present.
func (p *Policy) Claim(id string) (Claim, bool) {
	for _, c := range p.Claims {
		if c.ID == id {
			return c, true
		}
	}
	return Claim{}, false
}

// Digest returns the policy's canonical content address after proving
// the value is unmodified DecodePolicy output.
func (p *Policy) Digest() (string, error) {
	if err := p.checkSeal(); err != nil {
		return "", err
	}
	return p.seal, nil
}

func (p *Policy) checkSeal() error {
	return checkSealed("policy", p.ID, p.seal, p)
}
