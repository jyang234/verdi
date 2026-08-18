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

// DispositionScaffoldData is the policy-disposition.md scaffold's own
// render input: PolicyScaffoldData's identity fields plus the minimal
// judge-result witness (target/claim identity), one approval, and an
// expiry (authority-design §8: judge-result needs no fallback-only control
// or time bound, but an expiry remains legal — this scaffold's minimal
// skeleton always renders one so a real disposition has a concrete
// re-review date to edit rather than a silently absent bound). InputID
// must already be the exact canonical digest of the witness it describes
// (Witness.InputID cleared then canonjson-digested, exactly what
// policyartifact.DecodeDisposition itself re-derives and checks) — the
// caller computes it, since only the caller knows the complete witness
// content this minimal single-claim skeleton commits to.
type DispositionScaffoldData struct {
	Name              string
	Title             string
	Owners            []string
	InputID           string
	TargetDigest      string
	ClaimID           string
	ClaimDigest       string
	Category          string
	AuthorityDigest   string
	ApprovalRole      string
	ApprovalPrincipal string
	Expiry            string
	TemplateIdentity  string
	TemplateDigest    string
}

// universalScope is the canonical scaffold's own fixed scope value:
// every one of the three policy-family templates renders the universal
// (unconstrained on every dimension) scope verbatim — none of
// PolicyScaffoldData/OverlayScaffoldData/ExemptionScaffoldData carries a
// scope field, so this IS the scaffold's own canonical default, checked
// exactly like policy's empty claims/instructions/payloads below (F1
// review round: "the scaffold's fixed structural parts equal the
// canonical scaffold defaults").
var universalScope = policyartifact.Scope{
	Phases:       []string{},
	Environments: []string{},
	Paths:        []string{},
	Refs:         []string{},
}

// RenderPolicy renders scaffold against data through
// designscaffold.RenderValue — never a second, competing render path —
// strict-decodes the result as a verdi.policy/v1 artifact
// (internal/policyartifact.DecodePolicy), and verifies the render/
// strict-decode kernel round trip AC-1's "verdi model check ... proves
// parity" language requires: the decoded id, title, owners, and
// template record must equal exactly what data supplied and scaffold
// itself resolved to, AND every other policy kernel field
// (kernelFieldTable's own policy row: scope, claims, instructions,
// payloads) equals this scaffold's own fixed canonical default — the
// scaffold renders a minimal placeholder skeleton, so scope is
// universal and claims/instructions/payloads are explicitly empty. A
// template that drops, hardcodes, or otherwise mutates or synthesizes
// ANY one of those kernel fields fails closed here, naming the field —
// the anti-synthesis check (AC-1: "a template cannot remove, rename,
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
	if !scopesEqual(p.Scope, universalScope) {
		return "", fmt.Errorf("humanartifact: rendered policy kernel mismatch: scope = %+v, want the canonical universal scope %+v", p.Scope, universalScope)
	}
	if len(p.Claims) != 0 {
		return "", fmt.Errorf("humanartifact: rendered policy kernel mismatch: claims = %v, want the canonical empty claims list (this scaffold renders a placeholder skeleton, never real claims)", p.Claims)
	}
	if len(p.Instructions) != 0 {
		return "", fmt.Errorf("humanartifact: rendered policy kernel mismatch: instructions = %v, want the canonical empty instructions list (this scaffold renders a placeholder skeleton, never template-authored instructions)", p.Instructions)
	}
	if len(p.Payloads) != 0 {
		return "", fmt.Errorf("humanartifact: rendered policy kernel mismatch: payloads = %v, want the canonical empty payloads map", p.Payloads)
	}
	return content, nil
}

// RenderOverlay is RenderPolicy's twin for the policy-overlay scaffold:
// the shared id/title/owners/template kernel round trip, universal
// scope, AND the overlay-specific kernel fields (kernelFieldTable's
// overlay row: refines, refinements) round-trip what data supplied —
// refines must equal data.RefinesPolicy exactly, and refinements must
// carry exactly the one placeholder refinement this scaffold renders:
// claim data.ClaimName, a nonempty (never dropped or bound-typed)
// values operand, and no second, synthesized refinement.
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
	if !scopesEqual(o.Scope, universalScope) {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: scope = %+v, want the canonical universal scope %+v", o.Scope, universalScope)
	}
	if o.Refines != data.RefinesPolicy {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: refines = %q, want %q (a template must not hardcode, drop, or otherwise mutate refines)", o.Refines, data.RefinesPolicy)
	}
	if len(o.Refinements) != 1 {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: refinements has %d entries, want exactly 1 (this scaffold renders one placeholder refinement; a template must not drop or synthesize extra ones)", len(o.Refinements))
	}
	ref := o.Refinements[0]
	if ref.Claim != data.ClaimName {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: refinements[0].claim = %q, want %q", ref.Claim, data.ClaimName)
	}
	if ref.Bound != nil {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: refinements[0].bound = %v, want none (this scaffold's placeholder refinement uses a values operand)", *ref.Bound)
	}
	if len(ref.Values) == 0 {
		return "", fmt.Errorf("humanartifact: rendered policy-overlay kernel mismatch: refinements[0].values is empty, want a nonempty placeholder value")
	}
	return content, nil
}

// RenderExemption is RenderPolicy's twin for the policy-exemption
// scaffold: the shared id/title/owners/template kernel round trip,
// universal scope, AND the exemption-specific kernel fields
// (kernelFieldTable's exemption row: witnesses, approvals, expiry,
// review_condition) round-trip what data supplied — exactly one witness
// (policy/claim/claim_digest) and one approval (role/principal)
// matching data field for field, expiry equal to data.Expiry, and
// review_condition empty (this scaffold renders expiry only, never a
// review condition — a nonempty one would be template-synthesized
// content with no data behind it).
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
	if !scopesEqual(e.Scope, universalScope) {
		return "", fmt.Errorf("humanartifact: rendered policy-exemption kernel mismatch: scope = %+v, want the canonical universal scope %+v", e.Scope, universalScope)
	}
	wantWitness := policyartifact.Witness{Policy: data.WitnessPolicy, Claim: data.WitnessClaim, ClaimDigest: data.WitnessClaimDigest}
	if len(e.Witnesses) != 1 || e.Witnesses[0] != wantWitness {
		return "", fmt.Errorf("humanartifact: rendered policy-exemption kernel mismatch: witnesses = %+v, want exactly [%+v] (a template must not hardcode, drop, or synthesize a witness)", e.Witnesses, wantWitness)
	}
	wantApproval := policyartifact.Approval{Role: data.ApprovalRole, Principal: data.ApprovalPrincipal}
	if len(e.Approvals) != 1 || e.Approvals[0] != wantApproval {
		return "", fmt.Errorf("humanartifact: rendered policy-exemption kernel mismatch: approvals = %+v, want exactly [%+v] (a template must not hardcode, drop, or synthesize an approval)", e.Approvals, wantApproval)
	}
	if e.Expiry != data.Expiry {
		return "", fmt.Errorf("humanartifact: rendered policy-exemption kernel mismatch: expiry = %q, want %q", e.Expiry, data.Expiry)
	}
	if e.ReviewCondition != "" {
		return "", fmt.Errorf("humanartifact: rendered policy-exemption kernel mismatch: review_condition = %q, want empty (this scaffold renders an expiry only)", e.ReviewCondition)
	}
	return content, nil
}

// RenderDisposition is RenderPolicy's twin for the policy-disposition
// scaffold: the shared id/title/owners/template kernel round trip,
// universal scope, AND the disposition-specific kernel fields
// (kernelFieldTable's disposition row: witness, conclusion, origin,
// judgment, compensating_controls, approvals, expiry, review_condition)
// round-trip either what data supplied (witness identity fields, the one
// approval, expiry) or this scaffold's own fixed canonical defaults
// (conclusion no-conflict, origin judge-result, no judgment provenance, no
// compensating controls, no review condition) — a minimal judge-result
// skeleton, not a creation-verb-exposed artifact (authority-design §8).
func RenderDisposition(scaffold Scaffold, data DispositionScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering disposition scaffold: %w", err)
	}
	d, err := policyartifact.DecodeDisposition([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered disposition scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindDisposition + "/" + data.Name
	if err := verifyKernelRoundTrip(policyartifact.KindDisposition, wantID, data.Title, data.Owners, d.ID, d.Title, d.Owners, d.Template, scaffold); err != nil {
		return "", err
	}
	if !scopesEqual(d.Scope, universalScope) {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: scope = %+v, want the canonical universal scope %+v", d.Scope, universalScope)
	}
	if d.Witness.InputID != data.InputID {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: witness.input_id = %q, want %q", d.Witness.InputID, data.InputID)
	}
	if d.Witness.TargetDigest != data.TargetDigest {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: witness.target_digest = %q, want %q", d.Witness.TargetDigest, data.TargetDigest)
	}
	wantClaim := policyartifact.SemanticClaimWitness{
		ID:              data.ClaimID,
		Digest:          data.ClaimDigest,
		Category:        data.Category,
		AuthorityDigest: data.AuthorityDigest,
		Scope:           universalScope,
		Values:          []string{},
	}
	if len(d.Witness.Claims) != 1 || !semanticClaimWitnessEqual(d.Witness.Claims[0], wantClaim) {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: witness.claims = %+v, want exactly [%+v] (a template must not hardcode, drop, or synthesize a claim witness)", d.Witness.Claims, wantClaim)
	}
	if len(d.Witness.Exemptions) != 0 {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: witness.exemptions = %+v, want empty (this scaffold names no exemption)", d.Witness.Exemptions)
	}
	if d.Conclusion != policyartifact.DispositionNoConflict {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: conclusion = %q, want %q (this scaffold's fixed canonical default)", d.Conclusion, policyartifact.DispositionNoConflict)
	}
	if d.Origin != policyartifact.DispositionJudgeResult {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: origin = %q, want %q (this scaffold's fixed canonical default)", d.Origin, policyartifact.DispositionJudgeResult)
	}
	if d.Judgment != nil {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: judgment = %+v, want none (this scaffold never fabricates judgment provenance)", d.Judgment)
	}
	if len(d.CompensatingControls) != 0 {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: compensating_controls = %v, want empty (a judge-result ruling needs none)", d.CompensatingControls)
	}
	wantApproval := policyartifact.Approval{Role: data.ApprovalRole, Principal: data.ApprovalPrincipal}
	if len(d.Approvals) != 1 || d.Approvals[0] != wantApproval {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: approvals = %+v, want exactly [%+v] (a template must not hardcode, drop, or synthesize an approval)", d.Approvals, wantApproval)
	}
	if d.Expiry != data.Expiry {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: expiry = %q, want %q", d.Expiry, data.Expiry)
	}
	if d.ReviewCondition != "" {
		return "", fmt.Errorf("humanartifact: rendered policy-disposition kernel mismatch: review_condition = %q, want empty (this scaffold renders an expiry only)", d.ReviewCondition)
	}
	return content, nil
}

// semanticClaimWitnessEqual compares two SemanticClaimWitness values field
// by field — like scopesEqual, an explicit comparison since the type's
// slice/pointer fields make it non-comparable via ==. want.Bound is always
// nil (this scaffold's fixed canonical default never fabricates a bound),
// so got.Bound must also be nil.
func semanticClaimWitnessEqual(got, want policyartifact.SemanticClaimWitness) bool {
	return got.ID == want.ID &&
		got.Digest == want.Digest &&
		got.Category == want.Category &&
		got.AuthorityDigest == want.AuthorityDigest &&
		scopesEqual(got.Scope, want.Scope) &&
		stringSlicesEqualExact(got.Values, want.Values) &&
		got.Bound == nil && want.Bound == nil
}

// scopesEqual compares two Scope values dimension by dimension.
// policyartifact.Scope's own slice fields make it non-comparable via
// ==, so this is the round trip's own explicit check.
func scopesEqual(a, b policyartifact.Scope) bool {
	return stringSlicesEqualExact(a.Phases, b.Phases) &&
		stringSlicesEqualExact(a.Environments, b.Environments) &&
		stringSlicesEqualExact(a.Paths, b.Paths) &&
		stringSlicesEqualExact(a.Refs, b.Refs)
}

// stringSlicesEqualExact compares two string slices element by element,
// in order — unlike equalOwnerSet, the scope dimensions this backs are
// already normalized (normalizeScope sorts each), so an ORDER
// difference here would itself be a genuine decode anomaly, not
// incidental variance to tolerate.
func stringSlicesEqualExact(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
