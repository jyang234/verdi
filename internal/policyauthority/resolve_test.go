package policyauthority

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

func TestResolve_HappyPath(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if ep.Schema != EffectivePolicySchema {
		t.Fatalf("Schema = %q, want %q", ep.Schema, EffectivePolicySchema)
	}
	if ep.ProfileID != "solo-default" {
		t.Fatalf("ProfileID = %q, want solo-default", ep.ProfileID)
	}
	if len(ep.Policies) != 1 {
		t.Fatalf("len(Policies) = %d, want 1", len(ep.Policies))
	}
	entry := ep.Policies[0]
	if entry.PolicyID != "policy/go-toolchain" {
		t.Fatalf("PolicyID = %q, want policy/go-toolchain", entry.PolicyID)
	}
	var goVersion, verifyRequired *EffectiveClaim
	for i := range entry.Claims {
		switch entry.Claims[i].ID {
		case "go-version":
			goVersion = &entry.Claims[i]
		case "verify-required":
			verifyRequired = &entry.Claims[i]
		}
	}
	if goVersion == nil || verifyRequired == nil {
		t.Fatalf("missing expected claims in %+v", entry.Claims)
	}
	if len(goVersion.Values) != 1 || goVersion.Values[0] != "1.25" {
		t.Fatalf("go-version effective values = %v, want [1.25] (overlay-narrowed)", goVersion.Values)
	}
	if len(goVersion.AppliedOverlays) != 1 || goVersion.AppliedOverlays[0] != "policy-overlay/frontend-go-version" {
		t.Fatalf("go-version AppliedOverlays = %v", goVersion.AppliedOverlays)
	}
	if len(verifyRequired.AppliedOverlays) != 0 {
		t.Fatalf("verify-required AppliedOverlays = %v, want empty", verifyRequired.AppliedOverlays)
	}
	if len(ep.Exemptions) != 1 || ep.Exemptions[0].ExemptionID != "policy-exemption/legacy-service-go" {
		t.Fatalf("Exemptions = %+v", ep.Exemptions)
	}

	if _, err := ep.Digest(); err != nil {
		t.Fatalf("Digest() error: %v", err)
	}
}

func TestResolve_DeterministicOverSourceReordering(t *testing.T) {
	rootA := t.TempDir()
	writeTree(t, rootA, minimalStoreFiles())
	sA, err := Load(rootA)
	if err != nil {
		t.Fatalf("Load(A) error: %v", err)
	}
	epA, err := Resolve(sA)
	if err != nil {
		t.Fatalf("Resolve(A) error: %v", err)
	}

	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
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
    overridable: true
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Pin the toolchain and the verification gate.
`
	rootB := t.TempDir()
	writeTree(t, rootB, files)
	sB, err := Load(rootB)
	if err != nil {
		t.Fatalf("Load(B) error: %v", err)
	}
	epB, err := Resolve(sB)
	if err != nil {
		t.Fatalf("Resolve(B) error: %v", err)
	}

	bytesA, err := canonjson.Marshal(epA)
	if err != nil {
		t.Fatalf("Marshal(A) error: %v", err)
	}
	bytesB, err := canonjson.Marshal(epB)
	if err != nil {
		t.Fatalf("Marshal(B) error: %v", err)
	}
	if string(bytesA) != string(bytesB) {
		t.Fatalf("canonjson.Marshal differs across source reordering:\nA=%s\nB=%s", bytesA, bytesB)
	}
	digestA, err := epA.Digest()
	if err != nil {
		t.Fatalf("Digest(A) error: %v", err)
	}
	digestB, err := epB.Digest()
	if err != nil {
		t.Fatalf("Digest(B) error: %v", err)
	}
	if digestA != digestB {
		t.Fatalf("Digest differs across source reordering: A=%s B=%s", digestA, digestB)
	}
}

// TestResolve_EmptySetsAreExplicitNeverNull proves every zero-value
// semantic set in the resolved output canonicalizes as JSON [], matching
// this store's "explicit empty set is []" convention (internal/
// policyartifact's own scope/claim decoders), never as JSON null: a
// minimum-operator claim never touches values at all, and a claim with no
// contributing overlay has no applied overlays.
func TestResolve_EmptySetsAreExplicitNeverNull(t *testing.T) {
	files := rulesStoreFiles()
	root := t.TempDir()
	writeTree(t, root, files)
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	data, err := canonjson.Marshal(ep)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if strings.Contains(string(data), "null") {
		t.Fatalf("resolved effective policy contains JSON null (want explicit [] everywhere):\n%s", data)
	}
}

func TestResolve_HandBuiltStoreRejected(t *testing.T) {
	if _, err := Resolve(&Store{}); err == nil {
		t.Fatal("Resolve(&Store{}) succeeded, want error")
	}
	if _, err := Resolve(nil); err == nil {
		t.Fatal("Resolve(nil) succeeded, want error")
	}
}

func TestEffectivePolicy_DigestRejectsMutation(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	ep.ProfileID = "tampered"
	if _, err := ep.Digest(); err == nil {
		t.Fatal("Digest() on a mutated EffectivePolicy succeeded, want error")
	}
}
