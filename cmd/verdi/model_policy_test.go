// Focused tests for `verdi model check`'s policy-scaffold parity
// extension (spec/context-integrity-v2 AC-1: "verdi model check renders
// and strict-decodes every configured template and proves parity across
// creation surfaces", applied here to the three constitution scaffolds —
// policy.md, policy-overlay.md, policy-exemption.md — through the
// internal/humanartifact seam, never a second render path). Mirrors
// model_test.go's own writeModelCheckStoreRoot fixture and calls
// runModelCheck directly (the testable core), rather than the built
// binary, for a fast, focused round trip over just this extension.
package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// TestModelCheck_PolicyScaffolds_OK proves a plain store with no
// .verdi/templates/ override at all still exits 0 — the three embedded
// canonical policy scaffolds (policy.md, policy-overlay.md,
// policy-exemption.md) round-trip clean against fixed placeholder data,
// exactly like every other class's own template round trip.
func TestModelCheck_PolicyScaffolds_OK(t *testing.T) {
	root := writeModelCheckStoreRoot(t, "")

	var stdout, stderr bytes.Buffer
	code := runModelCheck(root, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runModelCheck (embedded policy scaffolds) exit = %d, want 0\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "model: OK — verdi.model/v1, ") {
		t.Fatalf("stdout = %q, want it to start with the OK line", stdout.String())
	}
}

// TestModelCheck_PolicyScaffold_StoreOverride_HardcodedID_Exit2 is the
// sabotage half: a store override under .verdi/templates/policy.md that
// hardcodes a different id than the placeholder data supplies still
// strict-decodes clean (any valid policy/<name> id is legal) but fails
// the kernel round-trip humanartifact.RenderPolicy verifies — model
// check must fail closed at exit 2, naming policy.md.
func TestModelCheck_PolicyScaffold_StoreOverride_HardcodedID_Exit2(t *testing.T) {
	root := writeModelCheckStoreRoot(t, "")
	const sabotaged = `---
schema: verdi.policy/v1
id: policy/hardcoded-name-not-the-placeholder
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
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "policy.md"), []byte(sabotaged))

	var stdout, stderr bytes.Buffer
	code := runModelCheck(root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runModelCheck (policy.md hardcoded id) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "policy.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file policy.md", stderr.String())
	}
	// "(kind policy)" is checkPolicyScaffold's own exact wrap token
	// (model.go) — asserting on it, rather than the bare substring
	// "policy" (already trivially true of "policy.md" itself), actually
	// proves the kind is named.
	if !strings.Contains(stderr.String(), "(kind policy)") {
		t.Fatalf("stderr = %q, want it to name the kind via the \"(kind policy)\" token", stderr.String())
	}
}

// TestModelCheck_PolicyScaffold_StoreOverride_UnknownField_Exit2 is the
// other sabotage half: a store override whose rendered output carries an
// unrecognized frontmatter field fails strict decode (never the kernel
// round trip specifically) — still exit 2, still naming the file.
func TestModelCheck_PolicyScaffold_StoreOverride_UnknownField_Exit2(t *testing.T) {
	root := writeModelCheckStoreRoot(t, "")
	const sabotaged = `---
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
mystery_unknown_field: 1
---
Placeholder rationale.
`
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "policy-overlay.md"), []byte(sabotaged))

	var stdout, stderr bytes.Buffer
	code := runModelCheck(root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runModelCheck (policy-overlay.md unknown field) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "policy-overlay.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file policy-overlay.md", stderr.String())
	}
}

// TestModelCheck_PolicyExemptionScaffold_StoreOverride_BrokenSyntax_Exit2
// covers the third scaffold (policy-exemption.md) and the malformed-
// template-syntax failure mode, over a store override.
func TestModelCheck_PolicyExemptionScaffold_StoreOverride_BrokenSyntax_Exit2(t *testing.T) {
	root := writeModelCheckStoreRoot(t, "")
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "policy-exemption.md"), []byte("title: {{.Title\n"))

	var stdout, stderr bytes.Buffer
	code := runModelCheck(root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runModelCheck (policy-exemption.md malformed syntax) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "policy-exemption.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file policy-exemption.md", stderr.String())
	}
}

// TestModelCheck_PolicyExemptionScaffold_StoreOverride_HardcodedWitnessExpiryPrincipal_Exit2
// is the F1 review round's own required end-to-end probe: a store
// override under .verdi/templates/policy-exemption.md that hardcodes
// its expiry, its witness policy, AND its approval principal all still
// strict-decodes clean individually (a real calendar date, a real
// policy/<name> id, and a real canonical principal id are each
// independently legal) but none of the three matches the fixed
// placeholder data checkExemptionScaffold supplies — model check must
// fail closed at exit 2, naming policy-exemption.md.
func TestModelCheck_PolicyExemptionScaffold_StoreOverride_HardcodedWitnessExpiryPrincipal_Exit2(t *testing.T) {
	root := writeModelCheckStoreRoot(t, "")
	const sabotaged = `---
schema: verdi.policy-exemption/v1
id: policy-exemption/{{.Name}}
kind: policy-exemption
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
witnesses:
  - policy: policy/hardcoded-witness-target
    claim: {{safe .WitnessClaim}}
    claim_digest: {{printf "%q" .WitnessClaimDigest}}
compensating_controls:
  - "Placeholder compensating control."
approvals:
  - role: {{safe .ApprovalRole}}
    principal: principal/github-org/aGFyZGNvZGVk
expiry: "2030-06-15"
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Placeholder rationale.
`
	writeTestFile(t, filepath.Join(root, ".verdi", "templates", "policy-exemption.md"), []byte(sabotaged))

	var stdout, stderr bytes.Buffer
	code := runModelCheck(root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runModelCheck (policy-exemption.md hardcoded witness/expiry/principal) exit = %d, want 2\nstdout: %s\nstderr: %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "policy-exemption.md") {
		t.Fatalf("stderr = %q, want it to name the offending template file policy-exemption.md", stderr.String())
	}
	if !strings.Contains(stderr.String(), "(kind policy-exemption)") {
		t.Fatalf("stderr = %q, want it to name the kind via the \"(kind policy-exemption)\" token", stderr.String())
	}
}
