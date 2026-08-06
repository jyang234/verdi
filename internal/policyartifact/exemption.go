package policyartifact

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// Exemption is one policy-exemption artifact — the single exemption
// artifact kind and home (DC-24; SI-4): a bounded, governed departure
// from named policy claims, carrying exact witnesses, ownership and
// approval facts, compensating controls, and an expiry or review
// condition (DC-8). Lifecycle-wide accountability and escalation around
// this artifact belong to Guided Lifecycle and Governance; later-wave
// conflict evaluation decides when an exemption actually excuses a
// proven conflict. This package owns only the artifact.
type Exemption struct {
	Schema               string          `json:"schema"`
	ID                   string          `json:"id"`
	Kind                 string          `json:"kind"`
	Title                string          `json:"title"`
	Owners               []string        `json:"owners"`
	Scope                Scope           `json:"scope"`
	Witnesses            []Witness       `json:"witnesses"`
	CompensatingControls []string        `json:"compensating_controls"`
	Approvals            []Approval      `json:"approvals"`
	Expiry               string          `json:"expiry,omitempty"`
	ReviewCondition      string          `json:"review_condition,omitempty"`
	Template             *TemplateRecord `json:"template,omitempty"`
	Rationale            string          `json:"rationale"`

	seal string
}

// Witness names one exact departed-from claim: the governing policy id,
// the claim id, and the claim's canonical content digest (ClaimDigest),
// so a later claim change visibly stales the exemption instead of
// silently widening it.
type Witness struct {
	Policy      string `yaml:"policy" json:"policy"`
	Claim       string `yaml:"claim" json:"claim"`
	ClaimDigest string `yaml:"claim_digest" json:"claim_digest"`
}

// Approval records one approval fact: the approving role and the
// approver's canonical kernel principal identity (DC-17: a process-local
// username or agent assertion is never authoritative identity, so the
// stored fact carries the kernel's canonical principal ID grammar).
// Whether the recorded principal actually satisfies the governing
// profile's separation-of-duties mode is kernel authorization work
// consumed by later gates, never re-interpreted here (DC-22).
type Approval struct {
	Role      string `yaml:"role" json:"role"`
	Principal string `yaml:"principal" json:"principal"`
}

type witnessDoc struct {
	Policy      *string `yaml:"policy"`
	Claim       *string `yaml:"claim"`
	ClaimDigest *string `yaml:"claim_digest"`
}

type approvalDoc struct {
	Role      *string `yaml:"role"`
	Principal *string `yaml:"principal"`
}

type exemptionDoc struct {
	kernelDoc            `yaml:",inline"`
	Scope                *scopeDoc      `yaml:"scope"`
	Witnesses            *[]witnessDoc  `yaml:"witnesses"`
	CompensatingControls *[]string      `yaml:"compensating_controls"`
	Approvals            *[]approvalDoc `yaml:"approvals"`
	Expiry               *string        `yaml:"expiry"`
	ReviewCondition      *string        `yaml:"review_condition"`
}

// DecodeExemption strictly decodes data as a verdi.policy-exemption/v1
// artifact, validates its bounded-departure grammar, normalizes it, and
// seals the result.
func DecodeExemption(data []byte) (*Exemption, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	var doc exemptionDoc
	if err := artifact.DecodeStrict(fm, &doc); err != nil {
		return nil, err
	}
	k, err := doc.kernelDoc.toKernel(SchemaExemption, KindExemption)
	if err != nil {
		return nil, err
	}
	missing := func(field string) error {
		return fmt.Errorf("policyartifact: exemption field %s is missing: every exemption field except expiry/review_condition is mandatory", field)
	}
	if doc.Scope == nil {
		return nil, missing("scope")
	}
	if doc.Witnesses == nil {
		return nil, missing("witnesses")
	}
	if doc.CompensatingControls == nil {
		return nil, missing("compensating_controls")
	}
	if doc.Approvals == nil {
		return nil, missing("approvals")
	}

	scope, err := doc.Scope.toScope("exemption.scope")
	if err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}

	if len(*doc.Witnesses) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must name at least one exact witness")
	}
	witnesses := make([]Witness, 0, len(*doc.Witnesses))
	seenWitness := make(map[string]bool, len(*doc.Witnesses))
	for i, wd := range *doc.Witnesses {
		if wd.Policy == nil || wd.Claim == nil || wd.ClaimDigest == nil {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: policy, claim, and claim_digest are all required", i)
		}
		if _, err := parseKindedID(*wd.Policy, KindPolicy); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: policy %q must be a policy/<name> id: %w", i, *wd.Policy, err)
		}
		if !kebabRe.MatchString(*wd.Claim) {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: claim %q must be kebab-case", i, *wd.Claim)
		}
		if !sha256Re.MatchString(*wd.ClaimDigest) {
			return nil, fmt.Errorf("policyartifact: exemption witnesses[%d]: claim_digest %q is not sha256:<64 hex> form", i, *wd.ClaimDigest)
		}
		key := *wd.Policy + "#" + *wd.Claim
		if seenWitness[key] {
			return nil, fmt.Errorf("policyartifact: exemption witnesses: duplicate witness %s", key)
		}
		seenWitness[key] = true
		witnesses = append(witnesses, Witness{Policy: *wd.Policy, Claim: *wd.Claim, ClaimDigest: *wd.ClaimDigest})
	}

	// Compensating controls are authored ORDERED content, like a policy's
	// instructions: their order is the author's and is digest-bound; they
	// are never sorted or deduplicated. Each entry must carry real text.
	if len(*doc.CompensatingControls) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must name at least one compensating control")
	}
	for i, c := range *doc.CompensatingControls {
		if strings.TrimSpace(c) == "" {
			return nil, fmt.Errorf("policyartifact: exemption compensating_controls[%d]: empty control", i)
		}
		if strings.ContainsAny(c, "\n\r") {
			return nil, fmt.Errorf("policyartifact: exemption compensating_controls[%d]: a control must be a single line", i)
		}
	}

	if len(*doc.Approvals) == 0 {
		return nil, fmt.Errorf("policyartifact: exemption must record at least one approval fact")
	}
	approvals := make([]Approval, 0, len(*doc.Approvals))
	seenApproval := make(map[string]bool, len(*doc.Approvals))
	for i, ad := range *doc.Approvals {
		if ad.Role == nil || ad.Principal == nil {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: role and principal are both required", i)
		}
		if !kebabRe.MatchString(*ad.Role) {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: role %q must be kebab-case", i, *ad.Role)
		}
		if err := governanceprincipal.PrincipalID(*ad.Principal).Validate(); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption approvals[%d]: principal: %w", i, err)
		}
		key := *ad.Role + "\x00" + *ad.Principal
		if seenApproval[key] {
			return nil, fmt.Errorf("policyartifact: exemption approvals: duplicate approval (%s, %s)", *ad.Role, *ad.Principal)
		}
		seenApproval[key] = true
		approvals = append(approvals, Approval{Role: *ad.Role, Principal: *ad.Principal})
	}

	expiry := ""
	if doc.Expiry != nil {
		expiry = *doc.Expiry
		// A real calendar date, not just the shape of one: a departure
		// stamped 2026-02-31 or 9999-99-99 would be a permanently
		// unbounded exemption wearing a bound (DC-8; CO-2 fails closed).
		if _, err := time.Parse("2006-01-02", expiry); err != nil {
			return nil, fmt.Errorf("policyartifact: exemption expiry %q is not a real YYYY-MM-DD calendar date", expiry)
		}
	}
	review := ""
	if doc.ReviewCondition != nil {
		review = *doc.ReviewCondition
		if strings.TrimSpace(review) == "" {
			return nil, fmt.Errorf("policyartifact: exemption review_condition must carry a named condition, not blank text")
		}
	}
	if expiry == "" && review == "" {
		return nil, fmt.Errorf("policyartifact: exemption must carry an expiry or review condition (DC-8: every departure is bounded)")
	}

	rationale, err := requireRationale(KindExemption, body)
	if err != nil {
		return nil, err
	}

	e := &Exemption{
		Schema:               k.Schema,
		ID:                   k.ID,
		Kind:                 k.Kind,
		Title:                k.Title,
		Owners:               k.Owners,
		Scope:                scope,
		Witnesses:            witnesses,
		CompensatingControls: *doc.CompensatingControls,
		Approvals:            approvals,
		Expiry:               expiry,
		ReviewCondition:      review,
		Template:             k.Template,
		Rationale:            rationale,
	}
	normalizeScope(&e.Scope)
	sort.Slice(e.Witnesses, func(i, j int) bool {
		if e.Witnesses[i].Policy != e.Witnesses[j].Policy {
			return e.Witnesses[i].Policy < e.Witnesses[j].Policy
		}
		return e.Witnesses[i].Claim < e.Witnesses[j].Claim
	})
	sort.Slice(e.Approvals, func(i, j int) bool {
		if e.Approvals[i].Role != e.Approvals[j].Role {
			return e.Approvals[i].Role < e.Approvals[j].Role
		}
		return e.Approvals[i].Principal < e.Approvals[j].Principal
	})
	seal, err := canonjson.Digest(e)
	if err != nil {
		return nil, err
	}
	e.seal = seal
	return e, nil
}

// Name returns the exemption id's name half.
func (e *Exemption) Name() string { return nameOf(e.ID) }

// Digest returns the exemption's canonical content address after proving
// the value is unmodified DecodeExemption output.
func (e *Exemption) Digest() (string, error) {
	if err := e.checkSeal(); err != nil {
		return "", err
	}
	return e.seal, nil
}

func (e *Exemption) checkSeal() error {
	return checkSealed("exemption", e.ID, e.seal, e)
}
