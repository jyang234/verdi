package humanartifact

import (
	"fmt"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// Template filenames (bare, under .verdi/templates/ for overrides).
const (
	ConstitutionTemplate  = "policy-constitution.md"
	ProfileSoloTemplate   = "governance-profile-solo.md"
	ProfileTeamTemplate   = "governance-profile-team.md"
	StarterPolicyTemplate = "policy-starter.md"
	InventoryTemplate     = "constitution-consumers.json"
)

// ConstitutionScaffoldData is the policy-constitution.md scaffold's own
// render input: the kernel identity fields every constitution artifact
// carries (title, owners), the selected governance profile's id, and
// the resolved scaffold's own template identity and digest, which the
// template must record verbatim (RenderConstitution's kernel round-trip
// check fails closed the moment it does not).
type ConstitutionScaffoldData struct {
	Title            string
	Owners           []string
	ProfileID        string
	TemplateIdentity string
	TemplateDigest   string
}

// ProfileScaffoldData is the governance-profile-solo.md / governance-
// profile-team.md scaffolds' own render input: the profile's kernel id
// and class, and — solo only — the one subject the profile binds to
// every role (the local git identity a caller resolves before calling
// RenderProfile; the team template maps no one, so Subject is empty for
// it), plus the resolved scaffold's own template identity and digest.
type ProfileScaffoldData struct {
	ProfileID        string
	Class            governanceprincipal.Class // ClassSolo or ClassTeam
	Subject          string                    // solo: the local git identity; team: ""
	TemplateIdentity string
	TemplateDigest   string
}

// StarterPolicyScaffoldData is the policy-starter.md scaffold's own
// render input: the kernel identity fields every constitution artifact
// carries (title, owners) plus the one real rule this scaffold renders —
// the design_assistance payload's mode (R-W4-1: nothing else is
// synthesized) — and the resolved scaffold's own template identity and
// digest.
type StarterPolicyScaffoldData struct {
	Name                 string // "starter"
	Title                string
	Owners               []string
	DesignAssistanceMode string // "draft-write" | "proposal-only"
	TemplateIdentity     string
	TemplateDigest       string
}

// InventoryScaffoldData is the constitution-consumers.json scaffold's
// own render input: only the resolved scaffold's own template identity
// and digest — a starter inventory registers no consumers, so there is
// no other caller-supplied content to render.
type InventoryScaffoldData struct {
	TemplateIdentity string
	TemplateDigest   string
}

// RenderConstitution renders scaffold against data through
// designscaffold.RenderValue, strict-decodes the result
// (policyartifact.DecodeConstitution), and verifies the kernel round trip:
// id (fixed by the decoder), title, owners, template record, and — this
// scaffold's own kernel field — selected_profile must equal what data
// supplied.
func RenderConstitution(scaffold Scaffold, data ConstitutionScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering constitution scaffold: %w", err)
	}
	c, err := policyartifact.DecodeConstitution([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered constitution scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindConstitution + "/" + policyartifact.ConstitutionName
	if err := verifyKernelRoundTrip(policyartifact.KindConstitution, wantID, data.Title, data.Owners, c.ID, c.Title, c.Owners, c.Template, scaffold); err != nil {
		return "", err
	}
	if c.SelectedProfile != data.ProfileID {
		return "", fmt.Errorf("humanartifact: rendered constitution kernel mismatch: selected_profile = %q, want %q", c.SelectedProfile, data.ProfileID)
	}
	return content, nil
}

// RenderProfile renders scaffold (governance-profile-solo.md or
// governance-profile-team.md) against data through
// designscaffold.RenderValue, strict-decodes the result
// (policyartifact.DecodeStoredProfile against catalog), and verifies the
// kernel round trip: the decoded storage id and class must equal what
// data supplied, the decoded template record's identity and digest must
// equal scaffold's own resolved identity and digest, and every role
// mapping's subjects must equal exactly the caller's single supplied
// subject when data.Subject is non-empty (solo) — or there must be no
// role mappings at all when it is empty (team: the template maps no
// one). verifyKernelRoundTrip is not reused here: a stored profile
// carries no title/owners for it to check, and calling it with vacuous
// values would pass those checks trivially rather than proving anything,
// so the id/template comparisons are written out explicitly instead.
func RenderProfile(scaffold Scaffold, data ProfileScaffoldData, catalog governanceprincipal.Catalog) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering profile scaffold: %w", err)
	}
	sp, err := policyartifact.DecodeStoredProfile([]byte(content), catalog)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered profile scaffold failed strict decode: %w", err)
	}
	if sp.ID != data.ProfileID {
		return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: id = %q, want %q (a template must not hardcode, drop, or otherwise mutate the id)", sp.ID, data.ProfileID)
	}
	if sp.Profile.Class != data.Class {
		return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: class = %q, want %q", sp.Profile.Class, data.Class)
	}
	if sp.Profile.Template == nil {
		return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: template record is missing")
	}
	if sp.Profile.Template.Identity != scaffold.Identity {
		return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: template.identity = %q, want %q", sp.Profile.Template.Identity, scaffold.Identity)
	}
	if sp.Profile.Template.Digest != scaffold.Digest {
		return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: template.digest = %q, want %q", sp.Profile.Template.Digest, scaffold.Digest)
	}
	if data.Subject == "" {
		if len(sp.Profile.RoleMappings) != 0 {
			return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: role_mappings = %d entries, want none: no subject was supplied", len(sp.Profile.RoleMappings))
		}
		return content, nil
	}
	for i, m := range sp.Profile.RoleMappings {
		if len(m.Subjects) != 1 || m.Subjects[0] != data.Subject {
			return "", fmt.Errorf("humanartifact: rendered profile kernel mismatch: role_mappings[%d].subjects = %v, want exactly the caller's subject %q — a template must not synthesize identities", i, m.Subjects, data.Subject)
		}
	}
	return content, nil
}

// RenderStarterPolicy renders scaffold (policy-starter.md) against data
// through designscaffold.RenderValue, strict-decodes the result
// (policyartifact.DecodePolicy), and verifies the kernel round trip: id
// (fixed from data.Name), title, owners, and template record must equal
// what data supplied and scaffold itself resolved to; scope is the
// canonical universal scope; claims and instructions are empty; and
// payloads carries exactly one entry — the design_assistance payload,
// with mode data.DesignAssistanceMode and layout false (R-W4-1: the one
// real rule this scaffold renders; nothing else is synthesized). An
// unknown mode is refused by the payload's own Validate, which
// DecodePolicy already runs.
func RenderStarterPolicy(scaffold Scaffold, data StarterPolicyScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering starter policy scaffold: %w", err)
	}
	p, err := policyartifact.DecodePolicy([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered starter policy scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindPolicy + "/" + data.Name
	if err := verifyKernelRoundTrip(policyartifact.KindPolicy, wantID, data.Title, data.Owners, p.ID, p.Title, p.Owners, p.Template, scaffold); err != nil {
		return "", err
	}
	if !scopesEqual(p.Scope, universalScope) {
		return "", fmt.Errorf("humanartifact: rendered starter policy kernel mismatch: scope = %+v, want the canonical universal scope %+v", p.Scope, universalScope)
	}
	if len(p.Claims) != 0 {
		return "", fmt.Errorf("humanartifact: rendered starter policy kernel mismatch: claims = %v, want none: the starter renders one real rule, never claims", p.Claims)
	}
	if len(p.Instructions) != 0 {
		return "", fmt.Errorf("humanartifact: rendered starter policy kernel mismatch: instructions = %v, want none: the starter renders one real rule, never instructions", p.Instructions)
	}
	if len(p.Payloads) != 1 {
		return "", fmt.Errorf("humanartifact: rendered starter policy kernel mismatch: payloads = %v, want exactly one design_assistance payload with mode %q", p.Payloads, data.DesignAssistanceMode)
	}
	da, ok := p.Payloads[policyartifact.DesignAssistancePayloadKind].(*policyartifact.DesignAssistancePayload)
	if !ok || da.Mode != data.DesignAssistanceMode || da.Layout {
		return "", fmt.Errorf("humanartifact: rendered starter policy kernel mismatch: payloads = %v, want exactly one design_assistance payload with mode %q", p.Payloads, data.DesignAssistanceMode)
	}
	return content, nil
}

// RenderConsumersInventory renders scaffold (constitution-consumers.json)
// against data through designscaffold.RenderValue, strict-decodes the
// result (constitutionimpact.DecodeInventory, which itself enforces the
// canonical-bytes check — a non-canonical override fails closed there),
// and verifies the kernel round trip: consumers must be empty (a starter
// inventory registers none) and the template record's identity and
// digest must equal scaffold's own resolved identity and digest.
func RenderConsumersInventory(scaffold Scaffold, data InventoryScaffoldData) ([]byte, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return nil, fmt.Errorf("humanartifact: rendering consumers inventory scaffold: %w", err)
	}
	inv, err := constitutionimpact.DecodeInventory([]byte(content))
	if err != nil {
		return nil, fmt.Errorf("humanartifact: rendered consumers inventory scaffold failed strict decode: %w", err)
	}
	if len(inv.Consumers) != 0 {
		return nil, fmt.Errorf("humanartifact: rendered consumers inventory kernel mismatch: consumers = %d, want none: the starter registers no consumers", len(inv.Consumers))
	}
	if inv.Template == nil {
		return nil, fmt.Errorf("humanartifact: rendered consumers inventory kernel mismatch: template record is missing")
	}
	if inv.Template.Identity != scaffold.Identity {
		return nil, fmt.Errorf("humanartifact: rendered consumers inventory kernel mismatch: template.identity = %q, want %q", inv.Template.Identity, scaffold.Identity)
	}
	if inv.Template.Digest != scaffold.Digest {
		return nil, fmt.Errorf("humanartifact: rendered consumers inventory kernel mismatch: template.digest = %q, want %q", inv.Template.Digest, scaffold.Digest)
	}
	return []byte(content), nil
}
