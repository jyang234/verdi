package humanartifact

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// PolicyScaffoldData is the policy.md scaffold's own render input: the
// kernel identity fields every constitution artifact carries (AC-1)
// plus the resolved scaffold's own template identity and digest, which
// the template must record verbatim (RenderPolicy's kernel round-trip
// check fails closed the moment it does not).
type PolicyScaffoldData struct {
	Name             string
	Title            string
	Owners           []string
	TemplateIdentity string
	TemplateDigest   string
}

// OverlayScaffoldData is the policy-overlay.md scaffold's own render
// input: PolicyScaffoldData's identity fields plus the governing policy
// it refines and the one claim its placeholder refinement narrows (DC-3:
// an overlay refines only surfaces its governing policy declares
// overridable — this scaffold renders a single placeholder refinement,
// never a real narrowing decision).
type OverlayScaffoldData struct {
	Name             string
	Title            string
	Owners           []string
	RefinesPolicy    string
	ClaimName        string
	TemplateIdentity string
	TemplateDigest   string
}

// ExemptionScaffoldData is the policy-exemption.md scaffold's own render
// input: PolicyScaffoldData's identity fields plus the exact witness,
// approval, and expiry a bounded departure requires (DC-8: "every
// departure is bounded").
type ExemptionScaffoldData struct {
	Name               string
	Title              string
	Owners             []string
	WitnessPolicy      string
	WitnessClaim       string
	WitnessClaimDigest string
	ApprovalRole       string
	ApprovalPrincipal  string
	Expiry             string
	TemplateIdentity   string
	TemplateDigest     string
}

// RenderPolicy renders scaffold against data through
// designscaffold.RenderValue — never a second, competing render path —
// strict-decodes the result as a verdi.policy/v1 artifact
// (internal/policyartifact.DecodePolicy), and verifies the render/
// strict-decode kernel round trip AC-1's "verdi model check ... proves
// parity" language requires: the decoded id, title, owners, and
// template record must equal exactly what data supplied and scaffold
// itself resolved to. A template that drops, hardcodes, or otherwise
// mutates any one of those fails closed here, naming the field — the
// anti-synthesis check (AC-1: "a template cannot remove, rename,
// retype, or synthesize kernel fields").
func RenderPolicy(scaffold Scaffold, data PolicyScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering policy scaffold: %w", err)
	}
	p, err := policyartifact.DecodePolicy([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered policy scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindPolicy + "/" + data.Name
	if err := verifyKernelRoundTrip(policyartifact.KindPolicy, wantID, data.Title, data.Owners, p.ID, p.Title, p.Owners, p.Template, scaffold); err != nil {
		return "", err
	}
	return content, nil
}

// RenderOverlay is RenderPolicy's twin for the policy-overlay scaffold —
// see RenderPolicy's own doc comment for the render/decode/verify
// contract this mirrors exactly.
func RenderOverlay(scaffold Scaffold, data OverlayScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering overlay scaffold: %w", err)
	}
	o, err := policyartifact.DecodeOverlay([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered overlay scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindOverlay + "/" + data.Name
	if err := verifyKernelRoundTrip(policyartifact.KindOverlay, wantID, data.Title, data.Owners, o.ID, o.Title, o.Owners, o.Template, scaffold); err != nil {
		return "", err
	}
	return content, nil
}

// RenderExemption is RenderPolicy's twin for the policy-exemption
// scaffold — see RenderPolicy's own doc comment for the render/decode/
// verify contract this mirrors exactly.
func RenderExemption(scaffold Scaffold, data ExemptionScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering exemption scaffold: %w", err)
	}
	e, err := policyartifact.DecodeExemption([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered exemption scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindExemption + "/" + data.Name
	if err := verifyKernelRoundTrip(policyartifact.KindExemption, wantID, data.Title, data.Owners, e.ID, e.Title, e.Owners, e.Template, scaffold); err != nil {
		return "", err
	}
	return content, nil
}

// verifyKernelRoundTrip is the render/strict-decode kernel-parity proof
// RenderPolicy/RenderOverlay/RenderExemption each apply after a
// successful decode: the decoded id (built from kind + data's own Name),
// title, and owner set must equal what the caller's data supplied, and
// the decoded template record's identity and digest must equal
// scaffold's own resolved Identity and Digest — the ground truth
// ResolveScaffold established, not merely the data the template
// happened to render. A mismatch on any one of those names the specific
// field, never a bare "kernel mismatch".
func verifyKernelRoundTrip(kind, wantID, wantTitle string, wantOwners []string, gotID, gotTitle string, gotOwners []string, gotTemplate *policyartifact.TemplateRecord, scaffold Scaffold) error {
	if gotID != wantID {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: id = %q, want %q (a template must not hardcode, drop, or otherwise mutate the id)", kind, gotID, wantID)
	}
	if gotTitle != wantTitle {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: title = %q, want %q", kind, gotTitle, wantTitle)
	}
	if !equalOwnerSet(gotOwners, wantOwners) {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: owners = %v, want %v", kind, gotOwners, wantOwners)
	}
	if gotTemplate == nil {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: template record is missing", kind)
	}
	if gotTemplate.Identity != scaffold.Identity {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: template.identity = %q, want %q", kind, gotTemplate.Identity, scaffold.Identity)
	}
	if gotTemplate.Digest != scaffold.Digest {
		return fmt.Errorf("humanartifact: rendered %s kernel mismatch: template.digest = %q, want %q", kind, gotTemplate.Digest, scaffold.Digest)
	}
	return nil
}

// equalOwnerSet compares got and want as sets (order-insensitive — the
// decoded owners are already kernel-sorted, policyartifact's own
// toKernel), leaving the input slices untouched.
func equalOwnerSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}
