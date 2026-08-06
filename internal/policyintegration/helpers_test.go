package policyintegration

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// writeTree materializes files (relative-path -> content) under root,
// creating parent directories as needed — the same pattern every
// sibling package's own tests use (internal/policyauthority/
// store_test.go, internal/instructionprojection/fixture_helpers_test.go);
// copied rather than imported per this lane's write-set boundary.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// hashTree returns a deterministic content address for every entry under
// root: each FILE's repo-relative slash path plus its exact bytes, AND
// each DIRECTORY's repo-relative slash path on its own (prefixed "dir:"
// so a directory entry can never collide with a same-named file's own
// "file:" entry) — hashed in sorted-key order. Directories are recorded
// too, not merely files: a caller that stray-creates an empty directory
// (e.g. a premature os.MkdirAll before an adoption check ever runs) must
// still register as a change here, or the "nothing written" proofs this
// helper backs would pass over a real write that just happens to carry
// no file yet. root itself is never recorded (every legacy/incomplete
// fixture's root pre-exists via t.TempDir(), so recording it would be
// constant noise, never a witnessed change).
func hashTree(t *testing.T, root string) string {
	t.Helper()
	entries := map[string][]byte{}
	var keys []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			key := "dir:" + rel
			entries[key] = nil
			keys = append(keys, key)
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		key := "file:" + rel
		entries[key] = data
		keys = append(keys, key)
		return nil
	})
	if err != nil {
		t.Fatalf("hashTree(%s): %v", root, err)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write(entries[k])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TestHashTree_CatchesStrayDirectory is hashTree's own self-proof: a
// stray directory created with no file inside it (the shape a premature
// os.MkdirAll leaves behind — e.g. a hypothetical .verdi/cache/loader
// dir a loader might create before an adoption check even runs) must
// still change hashTree's output. Without this, a "nothing written"
// proof built only from file contents would pass over that exact write.
func TestHashTree_CatchesStrayDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	before := hashTree(t, root)

	strayDir := filepath.Join(root, ".verdi", "cache", "loader")
	if err := os.MkdirAll(strayDir, 0o755); err != nil {
		t.Fatalf("mkdir stray dir: %v", err)
	}
	after := hashTree(t, root)

	if after == before {
		t.Fatalf("hashTree(%s) unchanged after creating a stray, file-less directory %s; the dir-only change was not caught", root, strayDir)
	}
}

// goVersionClaim is the exact normalized (decode-time) shape of the
// go-toolchain policy's go-version claim every fixture in this package
// shares: values already sorted, every scope dimension the explicit
// empty set. Reused, never re-typed, so a witness computed from it via
// policyartifact.ClaimDigest matches exactly what policyauthority's own
// cross-validation recomputes from the decoded policy (store.go's
// crossValidate calls policyartifact.ClaimDigest on the decoded —
// already normalized — claim).
var goVersionClaim = policyartifact.Claim{
	ID:          "go-version",
	Family:      policyartifact.FamilyConfiguration,
	Operator:    policyartifact.OpAllowedValues,
	Subject:     "go-version",
	Values:      []string{"1.24", "1.25"},
	Scope:       policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
	Overridable: true,
}

// goVersionClaimDigest computes goVersionClaim's own witness digest at
// runtime (never hand-typed) — the exact value an exemption fixture's
// claim_digest witness field must carry to stay a real, non-stale
// witness (DC-8).
func goVersionClaimDigest(t *testing.T) string {
	t.Helper()
	d, err := policyartifact.ClaimDigest(goVersionClaim)
	if err != nil {
		t.Fatalf("policyartifact.ClaimDigest(goVersionClaim): %v", err)
	}
	return d
}

// baseStoreFiles returns a small but complete constitution-store file
// set, sharing the exact artifact shapes internal/policyauthority's own
// minimalStoreFiles and internal/instructionprojection's own
// testdata/store use (copied per this lane's read-only boundary on the
// sibling packages, never imported): one constitution with one codex
// adapter (AC-1's projection surface), one policy with an overridable
// allowed-values claim and real instructions, one overlay narrowing that
// claim, one exemption witnessing it, and one solo profile.
func baseStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Integration fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
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
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
The fixture constitution driving this package's cross-package proofs.
`,
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
instructions:
  - "Run make verify before claiming completion."
payloads: {}
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
