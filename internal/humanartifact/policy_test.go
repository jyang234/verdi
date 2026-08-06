package humanartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

func testPolicyData(scaffold Scaffold) PolicyScaffoldData {
	return PolicyScaffoldData{
		Name:             "test-policy",
		Title:            "Test Policy",
		Owners:           []string{"platform-team", "qa-lead"},
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
}

func testOverlayData(scaffold Scaffold) OverlayScaffoldData {
	return OverlayScaffoldData{
		Name:             "test-overlay",
		Title:            "Test Overlay",
		Owners:           []string{"frontend-team"},
		RefinesPolicy:    "policy/go-toolchain",
		ClaimName:        "go-version",
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
}

// testWitnessClaimDigest is a well-formed sha256:<64 hex> placeholder —
// computed, not hand-typed, so its length can never silently drift from
// the real grammar policyartifact.sha256Re enforces.
func testWitnessClaimDigest() string {
	sum := sha256.Sum256([]byte("test-witness-claim"))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func testExemptionData(scaffold Scaffold) ExemptionScaffoldData {
	return ExemptionScaffoldData{
		Name:               "test-exemption",
		Title:              "Test Exemption",
		Owners:             []string{"service-team"},
		WitnessPolicy:      "policy/go-toolchain",
		WitnessClaim:       "go-version",
		WitnessClaimDigest: testWitnessClaimDigest(),
		ApprovalRole:       "policy-owner",
		ApprovalPrincipal:  "principal/github-org/YWxpY2U",
		Expiry:             "2099-12-31",
		TemplateIdentity:   scaffold.Identity,
		TemplateDigest:     scaffold.Digest,
	}
}

// TestRenderPolicy_Happy proves the canonical embedded policy.md scaffold
// renders complete, valid verdi.policy/v1 content whose kernel round-
// trips exactly: id/title/owners/template match what the data supplied
// and the scaffold resolved to.
func TestRenderPolicy_Happy(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testPolicyData(scaffold)
	content, err := RenderPolicy(scaffold, data)
	if err != nil {
		t.Fatalf("RenderPolicy: %v", err)
	}
	p, err := policyartifact.DecodePolicy([]byte(content))
	if err != nil {
		t.Fatalf("test setup: DecodePolicy on RenderPolicy's own output: %v", err)
	}
	if p.ID != "policy/test-policy" {
		t.Fatalf("ID = %q", p.ID)
	}
	if p.Title != data.Title {
		t.Fatalf("Title = %q, want %q", p.Title, data.Title)
	}
	if p.Template == nil || p.Template.Identity != scaffold.Identity || p.Template.Digest != scaffold.Digest {
		t.Fatalf("Template = %+v, want identity %q digest %q", p.Template, scaffold.Identity, scaffold.Digest)
	}
}

func TestRenderOverlay_Happy(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-overlay.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testOverlayData(scaffold)
	content, err := RenderOverlay(scaffold, data)
	if err != nil {
		t.Fatalf("RenderOverlay: %v", err)
	}
	o, err := policyartifact.DecodeOverlay([]byte(content))
	if err != nil {
		t.Fatalf("test setup: DecodeOverlay on RenderOverlay's own output: %v", err)
	}
	if o.ID != "policy-overlay/test-overlay" {
		t.Fatalf("ID = %q", o.ID)
	}
	if o.Refines != data.RefinesPolicy {
		t.Fatalf("Refines = %q, want %q", o.Refines, data.RefinesPolicy)
	}
	if o.Template == nil || o.Template.Identity != scaffold.Identity || o.Template.Digest != scaffold.Digest {
		t.Fatalf("Template = %+v, want identity %q digest %q", o.Template, scaffold.Identity, scaffold.Digest)
	}
}

func TestRenderExemption_Happy(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-exemption.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testExemptionData(scaffold)
	content, err := RenderExemption(scaffold, data)
	if err != nil {
		t.Fatalf("RenderExemption: %v", err)
	}
	e, err := policyartifact.DecodeExemption([]byte(content))
	if err != nil {
		t.Fatalf("test setup: DecodeExemption on RenderExemption's own output: %v", err)
	}
	if e.ID != "policy-exemption/test-exemption" {
		t.Fatalf("ID = %q", e.ID)
	}
	if e.Expiry != data.Expiry {
		t.Fatalf("Expiry = %q, want %q", e.Expiry, data.Expiry)
	}
	if e.Template == nil || e.Template.Identity != scaffold.Identity || e.Template.Digest != scaffold.Digest {
		t.Fatalf("Template = %+v, want identity %q digest %q", e.Template, scaffold.Identity, scaffold.Digest)
	}
}

// TestRenderPolicy_Determinism proves the same scaffold+data rendered
// twice yields identical bytes and identical decoded artifact digests.
func TestRenderPolicy_Determinism(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testPolicyData(scaffold)
	a, err := RenderPolicy(scaffold, data)
	if err != nil {
		t.Fatalf("RenderPolicy(a): %v", err)
	}
	b, err := RenderPolicy(scaffold, data)
	if err != nil {
		t.Fatalf("RenderPolicy(b): %v", err)
	}
	if a != b {
		t.Fatalf("RenderPolicy is not deterministic:\na=%q\nb=%q", a, b)
	}
	pa, err := policyartifact.DecodePolicy([]byte(a))
	if err != nil {
		t.Fatalf("DecodePolicy(a): %v", err)
	}
	pb, err := policyartifact.DecodePolicy([]byte(b))
	if err != nil {
		t.Fatalf("DecodePolicy(b): %v", err)
	}
	da, err := pa.Digest()
	if err != nil {
		t.Fatalf("Digest(a): %v", err)
	}
	db, err := pb.Digest()
	if err != nil {
		t.Fatalf("Digest(b): %v", err)
	}
	if da != db {
		t.Fatalf("decoded digests differ: %s vs %s", da, db)
	}
}

// testPolicyTemplate is a minimal, valid, self-contained policy.md-shaped
// template — used as the sabotage tests' base so each perturbation
// exercises exactly one failure mode without depending on the embedded
// canonical template's own exact text.
const testPolicyTemplate = `---
schema: verdi.policy/v1
id: policy/{{.Name}}
kind: policy
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads: {}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Placeholder rationale.
`

func scaffoldFromTemplate(tmpl string) Scaffold {
	b := []byte(tmpl)
	sum := sha256.Sum256(b)
	return Scaffold{
		Identity: "test:sabotage",
		Digest:   "sha256:" + hex.EncodeToString(sum[:]),
		Template: b,
	}
}

// TestRenderPolicy_Sabotage is AC-1's anti-synthesis proof exercised
// end to end: a template that renames, drops, hardcodes, or otherwise
// mutates a kernel field fails RenderPolicy closed, each with an error
// naming the specific fault.
func TestRenderPolicy_Sabotage(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			"rename kernel key title to heading",
			func(s string) string { return strings.Replace(s, "title:", "heading:", 1) },
			"strict decode",
		},
		{
			"drop title entirely",
			func(s string) string {
				return strings.Replace(s, `title: {{printf "%q" .Title}}
`, "", 1)
			},
			"title",
		},
		{
			"hardcode id ignoring data.Name",
			func(s string) string {
				return strings.Replace(s, "id: policy/{{.Name}}", "id: policy/hardcoded-name", 1)
			},
			"id",
		},
		{
			"emit an extra unknown key",
			func(s string) string {
				return strings.Replace(s, "claims: []", "claims: []\nextra_unknown_field: 1", 1)
			},
			"strict decode",
		},
		{
			"change owners ignoring data.Owners",
			func(s string) string {
				return strings.Replace(s, "owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]", "owners: [hardcoded-team]", 1)
			},
			"owners",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := scaffoldFromTemplate(tt.mutate(testPolicyTemplate))
			data := testPolicyData(scaffold)
			_, err := RenderPolicy(scaffold, data)
			if err == nil {
				t.Fatalf("RenderPolicy(sabotaged: %s) = nil error, want error containing %q", tt.name, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("RenderPolicy(sabotaged: %s) error = %v, want containing %q", tt.name, err, tt.wantSub)
			}
		})
	}
}

// TestRenderPolicy_StoreOverrideSabotage proves the sabotage guard also
// fires over a real store-override scaffold resolved through
// ResolveScaffold, not merely a hand-built in-memory one.
func TestRenderPolicy_StoreOverrideSabotage(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	sabotaged := strings.Replace(testPolicyTemplate, "id: policy/{{.Name}}", "id: policy/hardcoded-name", 1)
	if err := os.WriteFile(filepath.Join(root, ".verdi", "templates", "policy.md"), []byte(sabotaged), 0o644); err != nil {
		t.Fatal(err)
	}
	scaffold, err := ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	if scaffold.Identity != "store:.verdi/templates/policy.md" {
		t.Fatalf("Identity = %q, want the store override identity", scaffold.Identity)
	}
	data := testPolicyData(scaffold)
	if _, err := RenderPolicy(scaffold, data); err == nil {
		t.Fatal("RenderPolicy(store-override hardcoded id) = nil error, want a kernel mismatch")
	} else if !strings.Contains(err.Error(), "id") {
		t.Fatalf("error = %v, want it to name the id mismatch", err)
	}
}

// TestRenderOverlay_Sabotage exercises the overlay twin's own kernel
// mismatch: a template that hardcodes its refines target rather than
// rendering data.RefinesPolicy still strict-decodes clean (any valid
// policy/<name> id is legal), but never matches the KERNEL round-trip
// this package additionally verifies — id must match data.Name.
func TestRenderOverlay_Sabotage(t *testing.T) {
	const testOverlayTemplate = `---
schema: verdi.policy-overlay/v1
id: policy-overlay/{{.Name}}
kind: policy-overlay
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
refines: {{safe .RefinesPolicy}}
scope: {phases: [], environments: [], paths: [], refs: []}
refinements:
  - claim: {{safe .ClaimName}}
    values: ["placeholder-value"]
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Placeholder rationale.
`
	sabotaged := strings.Replace(testOverlayTemplate, "id: policy-overlay/{{.Name}}", "id: policy-overlay/hardcoded-name", 1)
	scaffold := scaffoldFromTemplate(sabotaged)
	data := testOverlayData(scaffold)
	_, err := RenderOverlay(scaffold, data)
	if err == nil {
		t.Fatal("RenderOverlay(hardcoded id) = nil error, want a kernel mismatch")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Fatalf("error = %v, want it to name the id mismatch", err)
	}
}
