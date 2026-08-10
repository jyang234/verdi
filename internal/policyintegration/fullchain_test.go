package policyintegration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestFullChain_ScaffoldResolveProjectDrift is AC-1's storage -> renderer
// -> resolution -> projection chain proven as ONE witnessed flow: a new
// policy is scaffolded through humanartifact's shared resolver/renderer,
// written into a hermetic store, participates in policyauthority's
// resolved effective policy, projects cleanly through
// instructionprojection, and — once edited — visibly stales that
// projection until it is regenerated.
func TestFullChain_ScaffoldResolveProjectDrift(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, baseStoreFiles(t))

	// --- storage -> renderer: scaffold a NEW policy through the one
	// shared resolver/renderer every creation surface uses (AC-1). No
	// store override exists, so this resolves the embedded canonical
	// policy.md template.
	scaffold, err := humanartifact.ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold: %v", err)
	}
	if scaffold.Identity != "embedded:policy.md" {
		t.Fatalf("scaffold.Identity = %q, want the embedded identity", scaffold.Identity)
	}
	data := humanartifact.PolicyScaffoldData{
		Name:             "extra-policy",
		Title:            "Extra Policy",
		Owners:           []string{"platform-team"},
		TemplateIdentity: scaffold.Identity,
		TemplateDigest:   scaffold.Digest,
	}
	content, err := humanartifact.RenderPolicy(scaffold, data)
	if err != nil {
		t.Fatalf("RenderPolicy: %v", err)
	}
	wantDecoded, err := policyartifact.DecodePolicy([]byte(content))
	if err != nil {
		t.Fatalf("test setup: DecodePolicy on the rendered content: %v", err)
	}
	wantDigest, err := wantDecoded.Digest()
	if err != nil {
		t.Fatalf("test setup: Digest of the rendered policy: %v", err)
	}

	extraPolicyPath := filepath.Join(root, ".verdi", "policy", "policies", "extra-policy.md")
	if err := os.WriteFile(extraPolicyPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing scaffolded policy: %v", err)
	}

	// --- resolution: Load + Resolve must see the scaffolded artifact as
	// a first-class participant in the effective policy.
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("policyauthority.Load: %v", err)
	}
	ep, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve: %v", err)
	}
	entry := findPolicyEntry(t, ep, "policy/extra-policy")
	if entry.PolicyDigest == "" {
		t.Fatal("effective policy entry for policy/extra-policy carries no digest")
	}
	if entry.PolicyDigest != wantDigest {
		t.Fatalf("effective policy entry digest = %q, want the scaffolded artifact's own digest %q", entry.PolicyDigest, wantDigest)
	}
	if len(entry.Instructions) != 0 {
		t.Fatalf("scaffolded policy entry Instructions = %v, want empty (the scaffold renders a placeholder skeleton)", entry.Instructions)
	}
	epDigestBefore, err := ep.Digest()
	if err != nil {
		t.Fatalf("EffectivePolicy.Digest() before Generate: %v", err)
	}

	// --- projection: Generate then Verify clean.
	if _, err := instructionprojection.Generate(root); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	report, err := instructionprojection.Verify(root)
	if err != nil {
		t.Fatalf("instructionprojection.Verify: %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() after Generate() not clean: %+v", report.Findings)
	}

	// The scaffolded policy has zero instructions, so it must not appear
	// in the rendered AGENTS.md body at all (renderProjection only
	// sections policies that have at least one instruction) — proving
	// the projection reads real resolved content, not merely "a policy
	// exists".
	agentsBefore, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	if strings.Contains(string(agentsBefore), "policy/extra-policy") {
		t.Fatalf("AGENTS.md unexpectedly sections the zero-instruction scaffolded policy:\n%s", agentsBefore)
	}

	// --- edit the scaffolded policy's instructions directly (a real
	// governance edit, not a second render) and re-resolve.
	edited := strings.Replace(string(content), "instructions: []", `instructions: ["A newly added instruction."]`, 1)
	if edited == string(content) {
		t.Fatal("test setup: instructions: [] not found in the rendered content to edit")
	}
	if err := os.WriteFile(extraPolicyPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("writing edited policy: %v", err)
	}

	store2, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("policyauthority.Load after edit: %v", err)
	}
	ep2, err := policyauthority.Resolve(store2)
	if err != nil {
		t.Fatalf("policyauthority.Resolve after edit: %v", err)
	}
	epDigestAfter, err := ep2.Digest()
	if err != nil {
		t.Fatalf("EffectivePolicy.Digest() after edit: %v", err)
	}
	if epDigestAfter == epDigestBefore {
		t.Fatal("effective policy digest did not change after editing the scaffolded policy's instructions")
	}

	// --- Verify must now report drift on the stale managed file AND
	// manifest-drift on the stale manifest, against the projections
	// Generate wrote BEFORE the edit.
	report2, err := instructionprojection.Verify(root)
	if err != nil {
		t.Fatalf("instructionprojection.Verify after edit: %v", err)
	}
	if report2.Clean() {
		t.Fatal("Verify() clean after editing an instruction that should change the projection, want drift + manifest-drift")
	}
	var sawDrift, sawManifestDrift bool
	for _, f := range report2.Findings {
		switch f.Code {
		case instructionprojection.ReasonDrift:
			if f.Path == "AGENTS.md" {
				sawDrift = true
			}
		case instructionprojection.ReasonManifestDrift:
			sawManifestDrift = true
		}
	}
	if !sawDrift {
		t.Fatalf("no drift finding for AGENTS.md in %+v", report2.Findings)
	}
	if !sawManifestDrift {
		t.Fatalf("no manifest-drift finding in %+v", report2.Findings)
	}

	// --- re-Generate must restore a clean Verify.
	if _, err := instructionprojection.Generate(root); err != nil {
		t.Fatalf("instructionprojection.Generate (second): %v", err)
	}
	report3, err := instructionprojection.Verify(root)
	if err != nil {
		t.Fatalf("instructionprojection.Verify after re-Generate: %v", err)
	}
	if !report3.Clean() {
		t.Fatalf("Verify() after re-Generate() not clean: %+v", report3.Findings)
	}
	agentsAfter, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md after re-Generate: %v", err)
	}
	if !strings.Contains(string(agentsAfter), "A newly added instruction.") {
		t.Fatalf("regenerated AGENTS.md does not carry the new instruction:\n%s", agentsAfter)
	}
}

// findPolicyEntry returns ep's effective-policy entry for policyID,
// failing the test if it is absent.
func findPolicyEntry(t *testing.T, ep *policyauthority.EffectivePolicy, policyID string) policyauthority.EffectivePolicyEntry {
	t.Helper()
	for _, e := range ep.Policies {
		if e.PolicyID == policyID {
			return e
		}
	}
	t.Fatalf("effective policy has no entry for %s (entries: %+v)", policyID, ep.Policies)
	return policyauthority.EffectivePolicyEntry{}
}
