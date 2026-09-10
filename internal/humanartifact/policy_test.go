package humanartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
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

// testDispositionWitnessInputID computes the exact witness input_id the
// given claims/exemptions imply — the canonical digest of a
// policyartifact.SemanticWitness with InputID cleared, exactly what
// policyartifact's own (unexported) witnessInputID computes internally.
// DecodeDisposition no longer cross-checks this agreement (SI-114), but
// computing a real, self-consistent value here — never hand-typed —
// mirrors policy_test.go's own testWitnessClaimDigest discipline and keeps
// fixtures meaningful. Generalized (Task 3, docs/superpowers/specs/
// 2026-09-05-local-operator-disposition-design.md §2.3) to accept a
// complete claims/exemptions set, not only the one placeholder claim the
// original judge-result skeleton witnessed.
func testDispositionWitnessInputID(t *testing.T, targetDigest string, claims []policyartifact.SemanticClaimWitness, exemptions []policyartifact.SemanticExemptionWitness) string {
	t.Helper()
	w := policyartifact.SemanticWitness{
		TargetDigest: targetDigest,
		Claims:       claims,
		Exemptions:   exemptions,
	}
	id, err := canonjson.Digest(w)
	if err != nil {
		t.Fatalf("computing test witness input_id: %v", err)
	}
	return id
}

// testDispositionClaim returns the n'th test claim as both the
// DispositionScaffoldData shape RenderDisposition consumes and the
// policyartifact.SemanticClaimWitness the same content decodes to — the
// two must never drift, so every test builds both from this one function.
// n selects a distinct, kernel-legal policy-instruction claim id
// ("policy/test-policy#instruction-<n>"); n==1 deliberately reuses the
// UNSUFFIXED seeds ("test-disposition-claim"/"test-disposition-authority")
// the pre-multi-claim fixture always used, so
// TestRenderDisposition_ByteIdentityRegression's golden digests never move
// underneath it.
func testDispositionClaim(n int) (DispositionClaimData, policyartifact.SemanticClaimWitness) {
	claimSeed, authoritySeed := "test-disposition-claim", "test-disposition-authority"
	if n != 1 {
		claimSeed = fmt.Sprintf("%s-%d", claimSeed, n)
		authoritySeed = fmt.Sprintf("%s-%d", authoritySeed, n)
	}
	claim := DispositionClaimData{
		ID:              fmt.Sprintf("policy/test-policy#instruction-%d", n),
		Digest:          testDigestFor(claimSeed),
		Category:        "policy-instruction",
		AuthorityDigest: testDigestFor(authoritySeed),
		// policy-instruction is the one witness category whose scope is not
		// pinned to scope.refs == [ID] (validateSemanticClaimScope), so the
		// universal scope every test fixture used before multi-category
		// claims existed remains legal here.
		Scope: universalScope,
	}
	witness := policyartifact.SemanticClaimWitness{
		ID: claim.ID, Digest: claim.Digest, Category: claim.Category, AuthorityDigest: claim.AuthorityDigest,
		Scope: claim.Scope, Values: []string{},
	}
	return claim, witness
}

// testDispositionExemption returns the n'th test exemption as both the
// DispositionScaffoldData shape and the policyartifact.SemanticExemptionWitness
// the same content decodes to, mirroring testDispositionClaim's discipline.
func testDispositionExemption(n int) (DispositionExemptionData, policyartifact.SemanticExemptionWitness) {
	id := fmt.Sprintf("policy-exemption/test-exemption-%d", n)
	digest := testDigestFor(fmt.Sprintf("test-disposition-exemption-%d", n))
	return DispositionExemptionData{ID: id, Digest: digest}, policyartifact.SemanticExemptionWitness{ID: id, Digest: digest}
}

// testPrincipal derives a real canonical principal id for subject under a
// fixed "github-org" trust source — computed via the kernel's own
// constructor rather than hand-typed base64, so a second approver fixture
// can never silently carry a malformed principal.
func testPrincipal(t *testing.T, subject string) string {
	t.Helper()
	id, err := governanceprincipal.CanonicalPrincipalID("github-org", subject)
	if err != nil {
		t.Fatalf("CanonicalPrincipalID(%q): %v", subject, err)
	}
	return string(id)
}

func testDispositionData(scaffold Scaffold) DispositionScaffoldData {
	claim, _ := testDispositionClaim(1)
	// InputID is filled in by the caller below once the other fields are
	// fixed (it depends on them); tests that need a real DispositionScaffoldData
	// call testDispositionDataWithInputID(t, scaffold) instead.
	return DispositionScaffoldData{
		Name:             "test-disposition",
		Title:            "Test Disposition",
		Owners:           []string{"platform-team"},
		TargetDigest:     testDigestFor("test-disposition-target"),
		Claims:           []DispositionClaimData{claim},
		Conclusion:       string(policyartifact.DispositionNoConflict),
		Origin:           string(policyartifact.DispositionJudgeResult),
		Approvals:        []DispositionApprovalData{{Role: "policy-owner", Principal: "principal/github-org/YWxpY2U"}},
		Expiry:           "2099-12-31",
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
}

// testDispositionDataWithInputID returns testDispositionData(scaffold) with
// a real, computed InputID matching its own single-claim witness — the
// exact pre-multi-claim fixture shape, preserved unchanged so
// TestRenderDisposition_ByteIdentityRegression keeps proving the original
// rendering path byte-for-byte.
func testDispositionDataWithInputID(t *testing.T, scaffold Scaffold) DispositionScaffoldData {
	t.Helper()
	data := testDispositionData(scaffold)
	_, witness := testDispositionClaim(1)
	data.InputID = testDispositionWitnessInputID(t, data.TargetDigest, []policyartifact.SemanticClaimWitness{witness}, []policyartifact.SemanticExemptionWitness{})
	return data
}

// testDispositionDataMultiClaim returns a DispositionScaffoldData with two
// claims, one exemption, and two approvals — Task 3's multi-claim
// extension exercised directly against RenderDisposition, independent of
// the cmd/verdi verb.
func testDispositionDataMultiClaim(t *testing.T, scaffold Scaffold) DispositionScaffoldData {
	t.Helper()
	claim1, w1 := testDispositionClaim(1)
	claim2, w2 := testDispositionClaim(2)
	exemption, ew := testDispositionExemption(1)
	data := DispositionScaffoldData{
		Name:         "test-disposition-multi",
		Title:        "Test Disposition Multi",
		Owners:       []string{"platform-team"},
		TargetDigest: testDigestFor("test-disposition-target"),
		Claims:       []DispositionClaimData{claim1, claim2},
		Exemptions:   []DispositionExemptionData{exemption},
		Conclusion:   string(policyartifact.DispositionNoConflict),
		Origin:       string(policyartifact.DispositionJudgeResult),
		Approvals: []DispositionApprovalData{
			{Role: "policy-owner", Principal: "principal/github-org/YWxpY2U"},
			{Role: "security-owner", Principal: testPrincipal(t, "bob")},
		},
		Expiry:           "2099-12-31",
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
	data.InputID = testDispositionWitnessInputID(t, data.TargetDigest, []policyartifact.SemanticClaimWitness{w1, w2}, []policyartifact.SemanticExemptionWitness{ew})
	return data
}

// testDispositionDataHumanFallback returns a DispositionScaffoldData whose
// origin is human-fallback with one compensating control — Task 3's
// human-fallback extension exercised directly against RenderDisposition.
func testDispositionDataHumanFallback(t *testing.T, scaffold Scaffold) DispositionScaffoldData {
	t.Helper()
	data := testDispositionData(scaffold)
	data.Name = "test-disposition-fallback"
	data.Origin = string(policyartifact.DispositionHumanFallback)
	data.CompensatingControls = []string{"Manual review by the policy owner before merge."}
	_, witness := testDispositionClaim(1)
	data.InputID = testDispositionWitnessInputID(t, data.TargetDigest, []policyartifact.SemanticClaimWitness{witness}, []policyartifact.SemanticExemptionWitness{})
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
	want := data.Claims[0]
	if claim.ID != want.ID || claim.Digest != want.Digest || claim.Category != want.Category || claim.AuthorityDigest != want.AuthorityDigest {
		t.Fatalf("Witness.Claims[0] = %+v, want id/digest/category/authority_digest matching data.Claims[0] %+v", claim, want)
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
	wantApproval := policyartifact.Approval{Role: data.Approvals[0].Role, Principal: data.Approvals[0].Principal}
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
// mirroring testPolicyTemplate's role. Byte-identical to the canonical
// embedded scaffold (internal/designscaffold/templates/policy-disposition.md)
// so a sabotage mutation below exercises the same structure the real
// template renders.
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
{{range .Claims}}    - id: {{safe .ID}}
      digest: {{printf "%q" .Digest}}
      category: {{safe .Category}}
      authority_digest: {{printf "%q" .AuthorityDigest}}
      scope: {phases: [{{range $i, $v := .Scope.Phases}}{{if $i}}, {{end}}{{safe $v}}{{end}}], environments: [{{range $i, $v := .Scope.Environments}}{{if $i}}, {{end}}{{safe $v}}{{end}}], paths: [{{range $i, $v := .Scope.Paths}}{{if $i}}, {{end}}{{safe $v}}{{end}}], refs: [{{range $i, $v := .Scope.Refs}}{{if $i}}, {{end}}{{safe $v}}{{end}}]}
      values: []
{{end}}  exemptions: [{{range $i, $e := .Exemptions}}{{if $i}}, {{end}}{id: {{printf "%q" $e.ID}}, digest: {{printf "%q" $e.Digest}}}{{end}}]
conclusion: {{.Conclusion}}
origin: {{.Origin}}
{{if .CompensatingControls}}compensating_controls:
{{range .CompensatingControls}}  - {{printf "%q" .}}
{{end}}{{end}}approvals:
{{range .Approvals}}  - role: {{safe .Role}}
    principal: {{safe .Principal}}
{{end}}expiry: {{printf "%q" .Expiry}}
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
			func(s string) string {
				return strings.Replace(s, "conclusion: {{.Conclusion}}", "conclusion: conflict", 1)
			},
			"conclusion",
		},
		{
			"hardcode origin to human-fallback",
			func(s string) string {
				old := "origin: {{.Origin}}\n{{if .CompensatingControls}}compensating_controls:\n{{range .CompensatingControls}}  - {{printf \"%q\" .}}\n{{end}}{{end}}approvals:"
				new := "origin: human-fallback\ncompensating_controls:\n  - \"A control.\"\napprovals:"
				return strings.Replace(s, old, new, 1)
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
		{
			"synthesize an extra claim beyond data.Claims",
			func(s string) string {
				old := "{{end}}  exemptions:"
				extra := fmt.Sprintf(
					"    - id: policy/test-policy#instruction-99\n"+
						"      digest: %q\n"+
						"      category: policy-instruction\n"+
						"      authority_digest: %q\n"+
						"      scope: {phases: [], environments: [], paths: [], refs: []}\n"+
						"      values: []\n"+
						"  exemptions:",
					testDigestFor("sabotage-extra-claim-digest"), testDigestFor("sabotage-extra-claim-authority"))
				return strings.Replace(s, old, "{{end}}"+extra, 1)
			},
			"claims",
		},
		{
			"synthesize an extra approval beyond data.Approvals",
			func(s string) string {
				old := "{{end}}expiry:"
				// Reuses testDispositionData's own known-good principal under
				// a DIFFERENT role, so decode's (role, principal) duplicate
				// check never fires and the anti-synthesis check below is
				// reached on a structurally valid, decodable document.
				extra := "  - role: synthesized-owner\n" +
					"    principal: principal/github-org/YWxpY2U\n" +
					"expiry:"
				return strings.Replace(s, old, "{{end}}"+extra, 1)
			},
			"approvals",
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

// dispositionByteIdentityGoldenFormat is the exact byte output
// RenderDisposition produced for testDispositionData's single-claim/
// single-approval/judge-result/no-controls/no-exemptions shape before this
// file gained multi-claim, multi-approval, exemption, and human-fallback
// support (Task 3, docs/superpowers/specs/2026-09-05-local-operator-
// disposition-design.md §2.3). The %q placeholder is the resolved
// scaffold's own template digest — necessarily different from the
// pre-change template's digest, since the template itself gained the
// range/if constructs multi-claim rendering requires — every other byte is
// pinned exactly as originally captured.
const dispositionByteIdentityGoldenFormat = `---
schema: verdi.policy-disposition/v1
id: policy-disposition/test-disposition
kind: policy-disposition
title: "Test Disposition"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: "sha256:1ad1cb978571fd924576be3db70ea03ab00a22f69a7f54a383260121fc1a1ee2"
  target_digest: "sha256:40174022ec741c7a2b05162d805d464ef3c7b63b1ce13792025a23ff3334210d"
  claims:
    - id: policy/test-policy#instruction-1
      digest: "sha256:b10384977978c8ab275438c4d23f9ab7ff0d9da8fc0d4d41ba94b863932d2096"
      category: policy-instruction
      authority_digest: "sha256:50158216016971021258dcd62c6862f36f152cdfad77128a7fff1400ebb88ae2"
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions: []
conclusion: no-conflict
origin: judge-result
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2099-12-31"
template: {identity: "embedded:policy-disposition.md", digest: %q}
---
TODO: replace with the real rationale before accept.
`

// TestRenderDisposition_ByteIdentityRegression is the dispatch's own
// required proof that extending DispositionScaffoldData/RenderDisposition
// to a real, possibly-multi-element witness (multi-claim, multi-approval,
// exemptions, human-fallback) left the pre-existing single-claim/
// judge-result rendering path byte-for-byte unchanged, up to the
// necessarily-moved template self-digest (see the golden's own doc
// comment).
func TestRenderDisposition_ByteIdentityRegression(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataWithInputID(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition: %v", err)
	}
	want := fmt.Sprintf(dispositionByteIdentityGoldenFormat, scaffold.Digest)
	if content != want {
		t.Fatalf("RenderDisposition output changed for the pre-existing single-claim/judge-result shape:\n--- got ---\n%s\n--- want ---\n%s", content, want)
	}
}

// TestRenderDisposition_MultiClaim proves RenderDisposition's multi-claim
// extension: every claim, the exemption, and every approval data supplies
// round-trip to exactly what was given, in order, and the rendered content
// decodes and validates through the frozen policyartifact decoder.
func TestRenderDisposition_MultiClaim(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataMultiClaim(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition([]byte(content))
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if len(d.Witness.Claims) != 2 {
		t.Fatalf("Witness.Claims = %+v, want exactly 2", d.Witness.Claims)
	}
	for i, want := range data.Claims {
		got := d.Witness.Claims[i]
		if got.ID != want.ID || got.Digest != want.Digest || got.Category != want.Category || got.AuthorityDigest != want.AuthorityDigest {
			t.Fatalf("Witness.Claims[%d] = %+v, want matching data.Claims[%d] %+v", i, got, i, want)
		}
	}
	if len(d.Witness.Exemptions) != 1 || d.Witness.Exemptions[0].ID != data.Exemptions[0].ID || d.Witness.Exemptions[0].Digest != data.Exemptions[0].Digest {
		t.Fatalf("Witness.Exemptions = %+v, want exactly [%+v]", d.Witness.Exemptions, data.Exemptions[0])
	}
	wantApprovals := []policyartifact.Approval{
		{Role: data.Approvals[0].Role, Principal: data.Approvals[0].Principal},
		{Role: data.Approvals[1].Role, Principal: data.Approvals[1].Principal},
	}
	if !approvalSetEqual(d.Approvals, wantApprovals) {
		t.Fatalf("Approvals = %+v, want the set %+v", d.Approvals, wantApprovals)
	}
	if d.Origin != policyartifact.DispositionJudgeResult {
		t.Fatalf("Origin = %q, want judge-result", d.Origin)
	}
}

// TestRenderDisposition_HumanFallback proves RenderDisposition's
// human-fallback extension: origin and the compensating-control list
// round-trip to exactly what data supplied, no judgment provenance is
// fabricated, and the human-fallback-specific decode rules (at least one
// compensating control, a real expiry or review condition) are satisfied
// by what this scaffold renders.
func TestRenderDisposition_HumanFallback(t *testing.T) {
	scaffold, err := ResolveScaffold(t.TempDir(), "policy-disposition.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	data := testDispositionDataHumanFallback(t, scaffold)
	content, err := RenderDisposition(scaffold, data)
	if err != nil {
		t.Fatalf("RenderDisposition: %v", err)
	}
	d, err := policyartifact.DecodeDisposition([]byte(content))
	if err != nil {
		t.Fatalf("DecodeDisposition: %v", err)
	}
	if d.Origin != policyartifact.DispositionHumanFallback {
		t.Fatalf("Origin = %q, want human-fallback", d.Origin)
	}
	if !stringSlicesEqualExact(d.CompensatingControls, data.CompensatingControls) {
		t.Fatalf("CompensatingControls = %v, want %v", d.CompensatingControls, data.CompensatingControls)
	}
	if d.Judgment != nil {
		t.Fatalf("Judgment = %+v, want none (RenderDisposition never fabricates judgment provenance)", d.Judgment)
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
