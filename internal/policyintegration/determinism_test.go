package policyintegration

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// determinismVariantFiles returns a constitution store whose artifacts
// are semantically identical across reorder=false/true but textually
// different: claim list order, claim values order, owner list order,
// constitution environment/catalog list order, and adapter managed-path
// list order are all reversed between the two variants. Every one of
// those dimensions is itself a semantic SET this store's decoders
// already sort at decode time (co-3) — the point of running the full
// chain over both variants is proving that guarantee survives
// end to end (cross-artifact resolution and projection rendering),
// not merely at each artifact's own isolated decode.
func determinismVariantFiles(t *testing.T, reordered bool) map[string]string {
	t.Helper()

	environments := "[local, production]"
	roles := "[author, reviewer, policy-owner]"
	owners := "[platform-team, qa-lead]"
	claims := `claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false`
	managed := "[AGENTS.md, docs/AGENTS.md]"

	if reordered {
		environments = "[production, local]"
		roles = "[policy-owner, reviewer, author]"
		owners = "[qa-lead, platform-team]"
		claims = `claims:
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.24", "1.25"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true`
		managed = "[docs/AGENTS.md, AGENTS.md]"
	}

	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Determinism fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: ` + environments + `
catalog:
  roles: ` + roles + `
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: [go-version]
  capability: []
  resource: []
  identity: []
  evidence: []
adapters:
  - id: codex
    version: "1"
    managed: ` + managed + `
    discovery_filenames: [AGENTS.md]
---
The determinism fixture constitution.
`,
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: ` + owners + `
scope: {phases: [], environments: [], paths: [], refs: []}
` + claims + `
instructions:
  - "Run make verify before claiming completion."
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
Pin the toolchain and the verification gate.
`,
		".verdi/policy/overlays/frontend-go-version.md": `---
schema: verdi.policy-overlay/v1
id: policy-overlay/frontend-go-version
kind: policy-overlay
title: "Frontend Go version overlay"
owners: [frontend-team]
refines: policy/go-toolchain
scope: {phases: [], environments: [], paths: ["web/"], refs: []}
refinements:
  - claim: go-version
    values: ["1.25"]
template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}
---
The frontend narrows the toolchain choice.
`,
		".verdi/policy/exemptions/legacy-service-go.md": `---
schema: verdi.policy-exemption/v1
id: policy-exemption/legacy-service-go
kind: policy-exemption
title: "Legacy service stays on Go 1.24"
owners: [service-team]
scope: {phases: [], environments: [], paths: ["services/legacy/"], refs: []}
witnesses:
  - policy: policy/go-toolchain
    claim: go-version
    claim_digest: "` + goVersionClaimDigest(t) + `"
compensating_controls:
  - "Weekly CVE review of the pinned toolchain."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2099-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
The legacy service departs from the toolchain policy under review.
`,
		".verdi/policy/profiles/solo-default.md": `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`,
	}
}

// TestDeterminism_ReorderedSourceSameDigestsSameBytes is the cross-check
// item: two textually different but semantically identical stores
// (reordered claims, values, owners, environment/catalog lists, and
// adapter managed-path lists) produce identical EffectivePolicy digests
// AND byte-identical Generate output, proven across the full
// Load -> Resolve -> Generate chain rather than any single artifact's
// own isolated decode.
func TestDeterminism_ReorderedSourceSameDigestsSameBytes(t *testing.T) {
	rootA := t.TempDir()
	writeTree(t, rootA, determinismVariantFiles(t, false))
	rootB := t.TempDir()
	writeTree(t, rootB, determinismVariantFiles(t, true))

	storeA, err := policyauthority.Load(rootA)
	if err != nil {
		t.Fatalf("Load(rootA): %v", err)
	}
	epA, err := policyauthority.Resolve(storeA)
	if err != nil {
		t.Fatalf("Resolve(rootA): %v", err)
	}
	digestA, err := epA.Digest()
	if err != nil {
		t.Fatalf("epA.Digest(): %v", err)
	}

	storeB, err := policyauthority.Load(rootB)
	if err != nil {
		t.Fatalf("Load(rootB): %v", err)
	}
	epB, err := policyauthority.Resolve(storeB)
	if err != nil {
		t.Fatalf("Resolve(rootB): %v", err)
	}
	digestB, err := epB.Digest()
	if err != nil {
		t.Fatalf("epB.Digest(): %v", err)
	}

	if digestA != digestB {
		t.Fatalf("EffectivePolicy digests differ across reordered-but-equivalent stores: %s vs %s", digestA, digestB)
	}

	resA, err := instructionprojection.Generate(rootA)
	if err != nil {
		t.Fatalf("Generate(rootA): %v", err)
	}
	resB, err := instructionprojection.Generate(rootB)
	if err != nil {
		t.Fatalf("Generate(rootB): %v", err)
	}
	if len(resA.Adapters) != 1 || len(resB.Adapters) != 1 {
		t.Fatalf("unexpected adapter counts: A=%d B=%d", len(resA.Adapters), len(resB.Adapters))
	}
	if resA.Adapters[0].ManifestDigest != resB.Adapters[0].ManifestDigest {
		t.Fatalf("manifest digests differ: A=%s B=%s", resA.Adapters[0].ManifestDigest, resB.Adapters[0].ManifestDigest)
	}

	for _, rel := range []string{"AGENTS.md", filepath.FromSlash("docs/AGENTS.md")} {
		bA, err := os.ReadFile(filepath.Join(rootA, rel))
		if err != nil {
			t.Fatalf("reading %s under rootA: %v", rel, err)
		}
		bB, err := os.ReadFile(filepath.Join(rootB, rel))
		if err != nil {
			t.Fatalf("reading %s under rootB: %v", rel, err)
		}
		if !bytes.Equal(bA, bB) {
			t.Fatalf("%s differs between reordered-but-equivalent stores:\nA=%q\nB=%q", rel, bA, bB)
		}
	}

	manifestA, err := os.ReadFile(filepath.Join(rootA, filepath.FromSlash(".verdi/policy/projections/codex.json")))
	if err != nil {
		t.Fatalf("reading manifest under rootA: %v", err)
	}
	manifestB, err := os.ReadFile(filepath.Join(rootB, filepath.FromSlash(".verdi/policy/projections/codex.json")))
	if err != nil {
		t.Fatalf("reading manifest under rootB: %v", err)
	}
	if !bytes.Equal(manifestA, manifestB) {
		t.Fatalf("manifest bytes differ between reordered-but-equivalent stores:\nA=%q\nB=%q", manifestA, manifestB)
	}
}
