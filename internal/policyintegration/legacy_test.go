package policyintegration

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestLegacyStore_NothingClaimsAuthority is DC-15's opt-in/reversible
// adoption boundary proven across all three cross-package entry points
// that could otherwise yield a value claiming constitution-backed
// authority: policyauthority.Load (the one loader), and
// instructionprojection.Generate/Verify (the only two callers that
// reach policyauthority.Resolve on a project's behalf). A store with
// .verdi/ but no .verdi/policy/ is an EXPECTED legacy state — every one
// of the three must fail with errors.Is(_, policyauthority.ErrNotAdopted)
// and write nothing. internal/humanartifact's ResolveScaffold/Render*
// functions take no root-adoption dependency at all (a template can be
// resolved and rendered before a project ever adopts a constitution —
// that is how the artifact this package's full-chain test writes gets
// created in the first place) so they are deliberately not exercised
// here as a fourth "entry point": they never claim constitution-backed
// authority to begin with.
func TestLegacyStore_NothingClaimsAuthority(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatalf("mkdir .verdi: %v", err)
	}
	before := hashTree(t, root)

	if _, err := policyauthority.Load(root); !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("policyauthority.Load() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if hashTree(t, root) != before {
		t.Fatal("policyauthority.Load() on a legacy store wrote to the tree")
	}

	if _, err := instructionprojection.Generate(root); !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Generate() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if hashTree(t, root) != before {
		t.Fatal("instructionprojection.Generate() on a legacy store wrote to the tree")
	}

	report, err := instructionprojection.Verify(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Verify() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if report != nil {
		t.Fatalf("instructionprojection.Verify() report = %+v, want nil on ErrNotAdopted", report)
	}
	if hashTree(t, root) != before {
		t.Fatal("instructionprojection.Verify() on a legacy store wrote to the tree")
	}
}

// TestLegacyStore_NoVerdiDirAtAll is TestLegacyStore_NothingClaimsAuthority's
// twin for a root that carries no .verdi/ at all — the most common real
// legacy project shape, not merely an empty .verdi/.
func TestLegacyStore_NoVerdiDirAtAll(t *testing.T) {
	root := t.TempDir()
	before := hashTree(t, root)

	if _, err := policyauthority.Load(root); !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("policyauthority.Load() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if _, err := instructionprojection.Generate(root); !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Generate() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if _, err := instructionprojection.Verify(root); !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Verify() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}
	if hashTree(t, root) != before {
		t.Fatal("a legacy root with no .verdi/ at all was written to")
	}
}

// TestIncompleteAdoption_NothingClaimsAuthority is DC-15's adoption
// boundary for the OTHER named failure mode: .verdi/policy/ exists (a
// project mid-adoption) but constitution.md is missing. Load,
// Generate, and Verify must each fail with
// errors.Is(_, policyauthority.ErrIncompleteAdoption) and write nothing.
func TestIncompleteAdoption_NothingClaimsAuthority(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
A policy committed before the constitution manifest itself.
`,
	})
	before := hashTree(t, root)

	if _, err := policyauthority.Load(root); !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("policyauthority.Load() error = %v, want errors.Is(_, ErrIncompleteAdoption)", err)
	}
	if hashTree(t, root) != before {
		t.Fatal("policyauthority.Load() on an incompletely adopted store wrote to the tree")
	}

	if _, err := instructionprojection.Generate(root); !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("instructionprojection.Generate() error = %v, want errors.Is(_, ErrIncompleteAdoption)", err)
	}
	if hashTree(t, root) != before {
		t.Fatal("instructionprojection.Generate() on an incompletely adopted store wrote to the tree")
	}

	report, err := instructionprojection.Verify(root)
	if !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("instructionprojection.Verify() error = %v, want errors.Is(_, ErrIncompleteAdoption)", err)
	}
	if report != nil {
		t.Fatalf("instructionprojection.Verify() report = %+v, want nil on ErrIncompleteAdoption", report)
	}
	if hashTree(t, root) != before {
		t.Fatal("instructionprojection.Verify() on an incompletely adopted store wrote to the tree")
	}
}
