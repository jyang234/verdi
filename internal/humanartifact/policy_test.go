package humanartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
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

// testDispositionWitnessInputID computes the exact witness input_id
// testDispositionData's witness fields imply — the canonical digest of a
// policyartifact.SemanticWitness with InputID cleared, exactly what
// policyartifact's own (unexported) witnessInputID computes internally —
// so DecodeDisposition's own input_id-agreement check passes. Computed,
// never hand-typed, mirroring policy_test.go's own testWitnessClaimDigest
// discipline.
func testDispositionWitnessInputID(t *testing.T, targetDigest, claimID, claimDigest, category, authorityDigest string) string {
	t.Helper()
	universal := policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	w := policyartifact.SemanticWitness{
		TargetDigest: targetDigest,
		Claims: []policyartifact.SemanticClaimWitness{{
			ID:              claimID,
			Digest:          claimDigest,
			Category:        category,
			AuthorityDigest: authorityDigest,
			Scope:           universal,
			Values:          []string{},
		}},
		Exemptions: []policyartifact.SemanticExemptionWitness{},
	}
	id, err := canonjson.Digest(w)
	if err != nil {
		t.Fatalf("computing test witness input_id: %v", err)
	}
	return id
}

func testDispositionData(scaffold Scaffold) DispositionScaffoldData {
	targetDigest := testDigestFor("test-disposition-target")
	claimDigest := testDigestFor("test-disposition-claim")
	authorityDigest := testDigestFor("test-disposition-authority")
	// InputID is filled in by the caller below once the other fields are
	// fixed (it depends on them); tests that need a real DispositionScaffoldData
	// call testDispositionDataWithInputID(t, scaffold) instead.
	return DispositionScaffoldData{
		Name:              "test-disposition",
		Title:             "Test Disposition",
		Owners:            []string{"platform-team"},
		TargetDigest:      targetDigest,
		ClaimID:           "ac-example",
		ClaimDigest:       claimDigest,
		Category:          "acceptance-criterion",
		AuthorityDigest:   authorityDigest,
		ApprovalRole:      "policy-owner",
		ApprovalPrincipal: "principal/github-org/YWxpY2U",
		Expiry:            "2099-12-31",
		TemplateIdentity:  scaffold.Identity,
		TemplateDigest:    scaffold.Digest,
	}
}

// testDispositionDataWithInputID returns testDispositionData(scaffold) with
// a real, computed InputID matching its own witness fields.
func testDispositionDataWithInputID(t *testing.T, scaffold Scaffold) DispositionScaffoldData {
	t.Helper()
	data := testDispositionData(scaffold)
	data.InputID = testDispositionWitnessInputID(t, data.TargetDigest, data.ClaimID, data.ClaimDigest, data.Category, data.AuthorityDigest)
	return data
}

// testDigestFor is a well-formed sha256:<64 hex> placeholder computed from
// seed, mirroring testWitnessClaimDigest's own discipline.
func testDigestFor(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestRenderDisposition_Happy proves the canonical embedded
// policy-disposition.md scaffold renders complete, valid
// verdi.policy-disposition/v1 content whose kernel round-trips exactly.
func TestRenderDisposition_Happy(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataWithInputID(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition([]byte(content))
	if err != nil {
		t.Fatalf("test setup: DecodeDisposition on RenderDisposition's own output: %v", err)
	}
	if d.ID != "policy-disposition/test-disposition" {
		t.Fatalf("ID = %q", d.ID)
	}
	if d.Title != data.Title {
		t.Fatalf("Title = %q, want %q", d.Title, data.Title)
	}
	if d.Conclusion != policyartifact.DispositionNoConflict {
		t.Fatalf("Conclusion = %q, want no-conflict", d.Conclusion)
	}
	if d.Origin != policyartifact.DispositionJudgeResult {
		t.Fatalf("Origin = %q, want judge-result", d.Origin)
	}
	if d.Witness.InputID != data.InputID {
		t.Fatalf("Witness.InputID = %q, want %q", d.Witness.InputID, data.InputID)
	}
	if d.Template == nil || d.Template.Identity != scaffold.Identity || d.Template.Digest != scaffold.Digest {
		t.Fatalf("Template = %+v, want identity %q digest %q", d.Template, scaffold.Identity, scaffold.Digest)
	}
}

// TestRenderDisposition_Determinism mirrors TestRenderPolicy_Determinism.
func TestRenderDisposition_Determinism(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataWithInputID(t, scaffold)
	a, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition(a): %v", err)
	}
	b, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition(b): %v", err)
	}
	if a != b {
		t.Fatalf("RenderDisposition is not deterministic:\na=%q\nb=%q", a, b)
	}
	da, err := policyartifact.DecodeDisposition([]byte(a))
	if err != nil {
		t.Fatalf("DecodeDisposition(a): %v", err)
	}
	db, err := policyartifact.DecodeDisposition([]byte(b))
	if err != nil {
		t.Fatalf("DecodeDisposition(b): %v", err)
	}
	digestA, err := da.Digest()
	if err != nil {
		t.Fatalf("Digest(a): %v", err)
	}
	digestB, err := db.Digest()
	if err != nil {
		t.Fatalf("Digest(b): %v", err)
	}
	if digestA != digestB {
		t.Fatalf("decoded digests differ: %s vs %s", digestA, digestB)
	}
}

// TestRenderDisposition_RoundTripKernelFields proves every disposition
// kernel field the scaffold's minimal judge-result skeleton fixes (scope,
// witness content, conclusion, origin, judgment absence, compensating
// controls absence, review_condition absence) round-trips to exactly the
// fixed canonical default, and every field the caller supplies
// (id/title/owners/template, witness identity fields, approval, expiry)
// round-trips to exactly what data supplied.
func TestRenderDisposition_RoundTripKernelFields(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataWithInputID(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition([]byte(content))
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if !scopesEqual(d.Scope, universalScope) {
		t.Fatalf("Scope = %+v, want universal", d.Scope)
	}
	if d.Witness.TargetDigest != data.TargetDigest {
		t.Fatalf("Witness.TargetDigest = %q, want %q", d.Witness.TargetDigest, data.TargetDigest)
	}
	if len(d.Witness.Claims) != 1 {
		t.Fatalf("Witness.Claims = %+v, want exactly one", d.Witness.Claims)
	}
	claim := d.Witness.Claims[0]
	if claim.ID != data.ClaimID || claim.Digest != data.ClaimDigest || claim.Category != data.Category || claim.AuthorityDigest != data.AuthorityDigest {
		t.Fatalf("Witness.Claims[0] = %+v, want id/digest/category/authority_digest matching data", claim)
	}
	if !scopesEqual(claim.Scope, universalScope) {
		t.Fatalf("Witness.Claims[0].Scope = %+v, want universal", claim.Scope)
	}
	if len(claim.Values) != 0 {
		t.Fatalf("Witness.Claims[0].Values = %v, want empty", claim.Values)
	}
	if len(d.Witness.Exemptions) != 0 {
		t.Fatalf("Witness.Exemptions = %+v, want empty", d.Witness.Exemptions)
	}
	if d.Judgment != nil {
		t.Fatalf("Judgment = %+v, want none", d.Judgment)
	}
	if len(d.CompensatingControls) != 0 {
		t.Fatalf("CompensatingControls = %v, want empty", d.CompensatingControls)
	}
	wantApproval := policyartifact.Approval{Role: data.ApprovalRole, Principal: data.ApprovalPrincipal}
	if len(d.Approvals) != 1 || d.Approvals[0] != wantApproval {
		t.Fatalf("Approvals = %+v, want exactly [%+v]", d.Approvals, wantApproval)
	}
	if d.Expiry != data.Expiry {
		t.Fatalf("Expiry = %q, want %q", d.Expiry, data.Expiry)
	}
	if d.ReviewCondition != "" {
		t.Fatalf("ReviewCondition = %q, want empty", d.ReviewCondition)
	}
}

// TestRenderDisposition_StoreOverrideResolution proves a store override at
// .verdi/templates/policy-disposition.md wins over the embedded canonical
// default (mirroring TestRenderPolicy_StoreOverrideSabotage's own
// resolution proof for the exemption scaffold family).
func TestRenderDisposition_StoreOverrideResolution(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	canonical, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold(canonical): %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "templates", "policy-disposition.md"), canonical.Template, 0o644); err != nil {
		t.Fatal(err)
	}
	scaffold, err := ResolveScaffold(root, "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold(override): %v", err)
	}
	if scaffold.Identity != "store:.verdi/templates/policy-disposition.md" {
		t.Fatalf("Identity = %q, want the store override identity", scaffold.Identity)
	}
	data := testDispositionDataWithInputID(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition(store override): %v", err)
	}
	if _, err := policyartifact.DecodeDisposition([]byte(content)); err != nil {
		t.Fatalf("DecodeDisposition(store override output): %v", err)
	}
}

// testDispositionTemplate is a minimal, valid, self-contained
// policy-disposition.md-shaped template — the sabotage table's own base,
// mirroring testPolicyTemplate's role.
const testDispositionTemplate = `---
schema: verdi.policy-disposition/v1
id: policy-disposition/{{.Name}}
kind: policy-disposition
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: {{printf "%q" .InputID}}
  target_digest: {{printf "%q" .TargetDigest}}
  claims:
    - id: {{safe .ClaimID}}
      digest: {{printf "%q" .ClaimDigest}}
      category: {{safe .Category}}
      authority_digest: {{printf "%q" .AuthorityDigest}}
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions: []
conclusion: no-conflict
origin: judge-result
approvals:
  - role: {{safe .ApprovalRole}}
    principal: {{safe .ApprovalPrincipal}}
expiry: {{printf "%q" .Expiry}}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Placeholder rationale.
`

// TestRenderDisposition_Sabotage is RenderPolicy/RenderExemption's own
// anti-synthesis proof for the disposition scaffold: a template that
// renames, drops, hardcodes, or otherwise mutates a kernel field fails
// RenderDisposition closed, each with an error naming the specific fault.
func TestRenderDisposition_Sabotage(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			"hardcode id ignoring data.Name",
			func(s string) string {
				return strings.Replace(s, "id: policy-disposition/{{.Name}}", "id: policy-disposition/hardcoded-name", 1)
			},
			"id",
		},
		{
			"hardcode conclusion to conflict",
			func(s string) string { return strings.Replace(s, "conclusion: no-conflict", "conclusion: conflict", 1) },
			"conclusion",
		},
		{
			"hardcode origin to human-fallback",
			func(s string) string {
				return strings.Replace(s,
					"origin: judge-result\napprovals:",
					"origin: human-fallback\ncompensating_controls:\n  - \"A control.\"\napprovals:", 1)
			},
			"origin",
		},
		{
			"hardcode expiry ignoring data.Expiry",
			func(s string) string {
				return strings.Replace(s, `expiry: {{printf "%q" .Expiry}}`, `expiry: "2030-06-15"`, 1)
			},
			"expiry",
		},
		{
			"synthesize a review_condition",
			func(s string) string {
				return strings.Replace(s, `expiry: {{printf "%q" .Expiry}}`, `expiry: {{printf "%q" .Expiry}}
review_condition: "synthesized review condition"`, 1)
			},
			"review_condition",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := scaffoldFromTemplate(tt.mutate(testDispositionTemplate))
			data := testDispositionDataWithInputID(t, scaffold)
			_, err := RenderDisposition(scaffold, data)
			if err == nil {
				t.Fatalf("RenderDisposition(sabotaged: %s) = nil error, want error containing %q", tt.name, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("RenderDisposition(sabotaged: %s) error = %v, want containing %q", tt.name, err, tt.wantSub)
			}
		})
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
		{
			"template-authored extra instruction",
			func(s string) string {
				return strings.Replace(s, "instructions: []", `instructions: ["a template-authored instruction data never asked for"]`, 1)
			},
			"instructions",
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

// testOverlayTemplate is a minimal, valid, self-contained policy-
// overlay.md-shaped template — the sabotage table's own base, mirroring
// testPolicyTemplate's role for TestRenderPolicy_Sabotage.
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

// TestRenderOverlay_Sabotage exercises the overlay twin's own kernel
// round trip end to end: a template that hardcodes its id, its refines
// target, or its refinement's claim each still strict-decodes clean (any
// valid policy/<name> id, refines target, and claim name are
// individually legal) but never matches what data supplied — each fails
// closed here, naming the specific mismatched field.
func TestRenderOverlay_Sabotage(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			"hardcode id ignoring data.Name",
			func(s string) string {
				return strings.Replace(s, "id: policy-overlay/{{.Name}}", "id: policy-overlay/hardcoded-name", 1)
			},
			"id",
		},
		{
			"hardcode refines ignoring data.RefinesPolicy",
			func(s string) string {
				return strings.Replace(s, "refines: {{safe .RefinesPolicy}}", "refines: policy/hardcoded-refines-target", 1)
			},
			"refines",
		},
		{
			"hardcode refinement claim ignoring data.ClaimName",
			func(s string) string {
				return strings.Replace(s, "claim: {{safe .ClaimName}}", "claim: hardcoded-claim", 1)
			},
			"claim",
		},
		{
			"synthesize an extra refinement",
			func(s string) string {
				return strings.Replace(s, `refinements:
  - claim: {{safe .ClaimName}}
    values: ["placeholder-value"]`, `refinements:
  - claim: {{safe .ClaimName}}
    values: ["placeholder-value"]
  - claim: synthesized-extra-claim
    values: ["x"]`, 1)
			},
			"refinements",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := scaffoldFromTemplate(tt.mutate(testOverlayTemplate))
			data := testOverlayData(scaffold)
			_, err := RenderOverlay(scaffold, data)
			if err == nil {
				t.Fatalf("RenderOverlay(sabotaged: %s) = nil error, want error containing %q", tt.name, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("RenderOverlay(sabotaged: %s) error = %v, want containing %q", tt.name, err, tt.wantSub)
			}
		})
	}
}

// testExemptionTemplate is a minimal, valid, self-contained policy-
// exemption.md-shaped template — the sabotage table's own base.
const testExemptionTemplate = `---
schema: verdi.policy-exemption/v1
id: policy-exemption/{{.Name}}
kind: policy-exemption
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
witnesses:
  - policy: {{safe .WitnessPolicy}}
    claim: {{safe .WitnessClaim}}
    claim_digest: {{printf "%q" .WitnessClaimDigest}}
compensating_controls:
  - "Placeholder compensating control."
approvals:
  - role: {{safe .ApprovalRole}}
    principal: {{safe .ApprovalPrincipal}}
expiry: {{printf "%q" .Expiry}}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Placeholder rationale.
`

// TestRenderExemption_Sabotage is F1's own required register: a store-
// override-shaped template that hardcodes the expiry, the witness
// policy, or the approval principal each still strict-decodes clean
// (any real calendar date, any policy/<name> id, and any canonical
// principal id are individually legal) but never matches what data
// supplied — each fails closed here, naming the specific field.
func TestRenderExemption_Sabotage(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantSub string
	}{
		{
			"hardcode expiry ignoring data.Expiry",
			func(s string) string {
				return strings.Replace(s, `expiry: {{printf "%q" .Expiry}}`, `expiry: "2030-06-15"`, 1)
			},
			"expiry",
		},
		{
			"hardcode witness policy ignoring data.WitnessPolicy",
			func(s string) string {
				return strings.Replace(s, "policy: {{safe .WitnessPolicy}}", "policy: policy/hardcoded-witness-target", 1)
			},
			"witnesses",
		},
		{
			"hardcode approval principal ignoring data.ApprovalPrincipal",
			func(s string) string {
				return strings.Replace(s, "principal: {{safe .ApprovalPrincipal}}", "principal: principal/github-org/aGFyZGNvZGVk", 1)
			},
			"approvals",
		},
		{
			"synthesize a review_condition alongside expiry",
			func(s string) string {
				return strings.Replace(s, `expiry: {{printf "%q" .Expiry}}`, `expiry: {{printf "%q" .Expiry}}
review_condition: "synthesized review condition"`, 1)
			},
			"review_condition",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scaffold := scaffoldFromTemplate(tt.mutate(testExemptionTemplate))
			data := testExemptionData(scaffold)
			_, err := RenderExemption(scaffold, data)
			if err == nil {
				t.Fatalf("RenderExemption(sabotaged: %s) = nil error, want error containing %q", tt.name, tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("RenderExemption(sabotaged: %s) error = %v, want containing %q", tt.name, err, tt.wantSub)
			}
		})
	}
}
