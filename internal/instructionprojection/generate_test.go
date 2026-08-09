package instructionprojection

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyauthority"
)

func TestGenerate_ErrNotAdopted_WritesNothing(t *testing.T) {
	root := t.TempDir()

	res, err := Generate(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("Generate() error = %v, want errors.Is(err, ErrNotAdopted)", err)
	}
	if res != nil {
		t.Fatalf("Generate() result = %+v, want nil on ErrNotAdopted", res)
	}

	entries, rerr := os.ReadDir(root)
	if rerr != nil {
		t.Fatalf("ReadDir(root): %v", rerr)
	}
	if len(entries) != 0 {
		t.Fatalf("Generate() on an unadopted root wrote %d entries, want 0: %v", len(entries), entries)
	}
}

func TestGenerate_HappyPath_WritesManagedFilesAndManifest(t *testing.T) {
	root := newFixtureRoot(t)

	res, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if len(res.Adapters) != 1 {
		t.Fatalf("Generate() adapters = %d, want 1", len(res.Adapters))
	}
	a := res.Adapters[0]
	if a.AdapterID != "codex" || a.AdapterVersion != "1" {
		t.Fatalf("Generate() adapter identity = %+v, want codex/1", a)
	}
	if len(a.Files) != 2 {
		t.Fatalf("Generate() adapter files = %d, want 2", len(a.Files))
	}

	for _, f := range a.Files {
		full := filepath.Join(root, filepath.FromSlash(f.Path))
		data, rerr := os.ReadFile(full)
		if rerr != nil {
			t.Fatalf("reading generated %s: %v", f.Path, rerr)
		}
		if got := contentDigest(data); got != f.Digest {
			t.Fatalf("generated %s digest = %s, want %s", f.Path, got, f.Digest)
		}
	}

	// Content identical for every managed file of one adapter (the v1
	// rule).
	rootFile, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}
	nestedFile, err := os.ReadFile(filepath.Join(root, "docs", "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading docs/AGENTS.md: %v", err)
	}
	if string(rootFile) != string(nestedFile) {
		t.Fatalf("adapter codex's two managed files have different content, want identical bytes")
	}

	// No wall-clock/random content: the fixed marker lines only ever
	// carry resolved authority (adapter id/version, digests, profile).
	for _, forbidden := range []string{"T00:", "T01:", "T02:", "T03:", "T04:", "T05:", "T06:", "T07:", "T08:", "T09:"} {
		if strings.Contains(string(rootFile), forbidden) {
			t.Fatalf("generated content contains a timestamp-shaped fragment %q:\n%s", forbidden, rootFile)
		}
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(a.ManifestPath))
	mData, merr := os.ReadFile(manifestPath)
	if merr != nil {
		t.Fatalf("reading manifest %s: %v", a.ManifestPath, merr)
	}
	if got := contentDigest(mData); got != a.ManifestDigest {
		t.Fatalf("manifest digest = %s, want %s", got, a.ManifestDigest)
	}
	if a.ManifestPath != ".verdi/policy/projections/codex.json" {
		t.Fatalf("manifest path = %q, want .verdi/policy/projections/codex.json", a.ManifestPath)
	}
}

func TestGenerate_Deterministic_AcrossIndependentRoots(t *testing.T) {
	rootA := newFixtureRoot(t)
	rootB := newFixtureRoot(t)

	resA, err := Generate(rootA)
	if err != nil {
		t.Fatalf("Generate(rootA): %v", err)
	}
	resB, err := Generate(rootB)
	if err != nil {
		t.Fatalf("Generate(rootB): %v", err)
	}

	dataA, err := os.ReadFile(filepath.Join(rootA, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	dataB, err := os.ReadFile(filepath.Join(rootB, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(dataA) != string(dataB) {
		t.Fatalf("two independent Generate runs over identical fixture content produced different bytes")
	}
	if resA.Adapters[0].ManifestDigest != resB.Adapters[0].ManifestDigest {
		t.Fatalf("two independent Generate runs produced different manifest digests: %s vs %s",
			resA.Adapters[0].ManifestDigest, resB.Adapters[0].ManifestDigest)
	}
}

func TestGenerate_ZeroAdapters_WritesNothing(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, zeroAdapterStoreFiles())

	res, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate() unexpected error: %v", err)
	}
	if len(res.Adapters) != 0 {
		t.Fatalf("Generate() adapters = %d, want 0", len(res.Adapters))
	}

	entries, rerr := os.ReadDir(root)
	if rerr != nil {
		t.Fatalf("ReadDir(root): %v", rerr)
	}
	// Only the seeded .verdi tree exists; nothing new was written.
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) != 1 || names[0] != ".verdi" {
		t.Fatalf("Generate() with zero adapters wrote extra entries at root: %v", names)
	}
}

// zeroAdapterStoreFiles is a minimal valid constitution store declaring
// adapters: [] — the explicit "a constitution with zero adapters
// generates nothing and verifies clean" case.
func zeroAdapterStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Zero-adapter fixture constitution"
owners: [platform-team]
selected_profile: solo-default
environments: []
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters: []
---
No adapters declared.
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
