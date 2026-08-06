package instructionprojection

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMultiAdapter_GenerateThenVerify_Clean is the realistic two-adapter
// layout: codex manages AGENTS.md and discovers AGENTS.md; claude-code
// manages CLAUDE.md and discovers BOTH CLAUDE.md and AGENTS.md (the
// harness reads whatever the project put there, not only its own file).
//
// AC-1 requires "every discovered project instruction to be generated
// and digest-matched" — not managed by whichever adapter discovered it.
// AGENTS.md is generated and digest-matched by codex, so claude-code's
// discovery of it is satisfied and the whole layout verifies clean.
func TestMultiAdapter_GenerateThenVerify_Clean(t *testing.T) {
	root := newMultiFixtureRoot(t)

	res, err := Generate(root)
	if err != nil {
		t.Fatalf("Generate(): %v", err)
	}
	if len(res.Adapters) != 2 {
		t.Fatalf("Generate() adapters = %d, want 2: %+v", len(res.Adapters), res.Adapters)
	}
	if res.Adapters[0].AdapterID != "claude-code" || res.Adapters[1].AdapterID != "codex" {
		t.Fatalf("Generate() adapter order = %s, %s; want claude-code, codex (sorted by id)", res.Adapters[0].AdapterID, res.Adapters[1].AdapterID)
	}

	report, err := Verify(root)
	if err != nil {
		t.Fatalf("Verify(): %v", err)
	}
	if !report.Clean() {
		t.Fatalf("Verify() after Generate() on the multi-adapter fixture = %+v, want Clean()", report.Findings)
	}
}

// TestMultiAdapter_DeterministicAcrossRoots proves the cross-adapter
// content and manifest bytes are deterministic (CO-3): two independent
// roots seeded from the same fixture produce byte-identical files and
// manifest digests, and the two adapters' own outputs stay distinct
// (each carries its own adapter identity, so they can never be confused
// for one another).
func TestMultiAdapter_DeterministicAcrossRoots(t *testing.T) {
	rootA := newMultiFixtureRoot(t)
	rootB := newMultiFixtureRoot(t)

	resA, err := Generate(rootA)
	if err != nil {
		t.Fatalf("Generate(rootA): %v", err)
	}
	resB, err := Generate(rootB)
	if err != nil {
		t.Fatalf("Generate(rootB): %v", err)
	}

	for _, rel := range []string{"AGENTS.md", "CLAUDE.md"} {
		a, err := os.ReadFile(filepath.Join(rootA, rel))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(rootB, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Fatalf("%s differs across two independent Generate runs over identical fixture content", rel)
		}
	}

	for i := range resA.Adapters {
		if resA.Adapters[i].ManifestDigest != resB.Adapters[i].ManifestDigest {
			t.Fatalf("adapter %s manifest digest differs across roots: %s vs %s",
				resA.Adapters[i].AdapterID, resA.Adapters[i].ManifestDigest, resB.Adapters[i].ManifestDigest)
		}
	}
	if resA.Adapters[0].ManifestDigest == resA.Adapters[1].ManifestDigest {
		t.Fatal("the two adapters produced identical manifest digests; each manifest must carry its own adapter identity")
	}

	agents, err := os.ReadFile(filepath.Join(rootA, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	claude, err := os.ReadFile(filepath.Join(rootA, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(agents) == string(claude) {
		t.Fatal("the two adapters' generated content is identical; each projection must carry its own adapter identity")
	}
	if !strings.Contains(string(agents), "adapter=codex ") || !strings.Contains(string(claude), "adapter=claude-code ") {
		t.Fatalf("generated files do not carry their own adapter identity:\n%s\n---\n%s", agents, claude)
	}
}

// TestGenerate_OverlappingManagedPaths_FailsClosed proves the
// unsatisfiable-constitution case fails closed rather than resolving
// last-writer-wins: two adapters declaring the SAME managed path can
// never both be true on disk (each adapter renders its own content), and
// writing one of them would leave the other adapter's manifest asserting
// a digest the disk contradicts — a manifest that lies (CO-1). Generate
// must name both adapter ids and the path, and write nothing.
func TestGenerate_OverlappingManagedPaths_FailsClosed(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, overlappingAdapterStoreFiles())

	res, err := Generate(root)
	if err == nil {
		t.Fatalf("Generate() with two adapters managing AGENTS.md = %+v, nil error; want a fail-closed overlap error", res)
	}
	if !errors.Is(err, ErrOverlappingManagedPath) {
		t.Fatalf("Generate() error = %v, want errors.Is(err, ErrOverlappingManagedPath)", err)
	}
	for _, want := range []string{"claude-code", "codex", "AGENTS.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Generate() error = %v, want it to name %q", err, want)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Fatalf("Generate() wrote AGENTS.md despite failing closed (stat err = %v)", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(root, ".verdi", "policy", "projections")); !os.IsNotExist(statErr) {
		t.Fatalf("Generate() wrote a manifest directory despite failing closed (stat err = %v)", statErr)
	}
}

// TestGenerate_CaseVariantManagedPaths_FailClosed pins the FOLDED half
// of the overlap rule: on a case-insensitive filesystem AGENTS.md and
// agents.md are one physical file, so two adapters declaring the
// case-variant pair — or one adapter declaring both spellings — is the
// same unsatisfiable surface as a byte-identical collision, and a
// byte-exact check would let Generate exit 0 with a manifest the disk
// contradicts. Both shapes must be refused on every platform.
func TestGenerate_CaseVariantManagedPaths_FailClosed(t *testing.T) {
	t.Run("across adapters", func(t *testing.T) {
		root := t.TempDir()
		files := overlappingAdapterStoreFiles()
		files[".verdi/policy/constitution.md"] = strings.Replace(
			files[".verdi/policy/constitution.md"],
			"  - id: claude-code\n    version: \"1\"\n    managed: [AGENTS.md]",
			"  - id: claude-code\n    version: \"1\"\n    managed: [agents.md]", 1)
		writeTree(t, root, files)

		_, err := Generate(root)
		if !errors.Is(err, ErrOverlappingManagedPath) {
			t.Fatalf("Generate() error = %v, want errors.Is(err, ErrOverlappingManagedPath)", err)
		}
		for _, want := range []string{"codex", "claude-code", "AGENTS.md", "agents.md"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("Generate() error = %v, want it to name %q", err, want)
			}
		}
		if _, statErr := os.Lstat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(statErr) {
			t.Fatalf("Generate() wrote AGENTS.md despite failing closed (stat err = %v)", statErr)
		}
	})
	t.Run("within one adapter", func(t *testing.T) {
		root := t.TempDir()
		files := overlappingAdapterStoreFiles()
		files[".verdi/policy/constitution.md"] = strings.Replace(
			files[".verdi/policy/constitution.md"],
			"adapters:\n  - id: codex\n    version: \"1\"\n    managed: [AGENTS.md]\n    discovery_filenames: [AGENTS.md]\n  - id: claude-code\n    version: \"1\"\n    managed: [AGENTS.md]\n    discovery_filenames: [AGENTS.md]",
			"adapters:\n  - id: codex\n    version: \"1\"\n    managed: [AGENTS.md, agents.md]\n    discovery_filenames: [AGENTS.md]", 1)
		writeTree(t, root, files)

		_, err := Generate(root)
		if !errors.Is(err, ErrOverlappingManagedPath) {
			t.Fatalf("Generate() error = %v, want errors.Is(err, ErrOverlappingManagedPath)", err)
		}
	})
}

// TestVerify_OverlappingManagedPaths_FailsClosed pins the same rule on
// the read side: an unsatisfiable constitution is an operational error
// naming the conflict, never a findings report whose "drift" would point
// a reader at the file instead of at the constitution that can never be
// satisfied.
func TestVerify_OverlappingManagedPaths_FailsClosed(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, overlappingAdapterStoreFiles())

	report, err := Verify(root)
	if err == nil {
		t.Fatalf("Verify() with two adapters managing AGENTS.md = %+v, nil error; want a fail-closed overlap error", report)
	}
	if !errors.Is(err, ErrOverlappingManagedPath) {
		t.Fatalf("Verify() error = %v, want errors.Is(err, ErrOverlappingManagedPath)", err)
	}
	if report != nil {
		t.Fatalf("Verify() report = %+v, want nil alongside the overlap error", report)
	}
}

// overlappingAdapterStoreFiles is a store whose two adapters declare the
// same managed path — the state Generate and Verify must both refuse.
func overlappingAdapterStoreFiles() map[string]string {
	return map[string]string{
		".verdi/policy/constitution.md": `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Overlapping-adapter fixture constitution"
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
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
  - id: claude-code
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
Two adapters claiming one file: an unsatisfiable projection surface.
`,
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Pin the toolchain.
`,
		".verdi/policy/profiles/solo-default.md": soloDefaultProfileDoc,
	}
}
