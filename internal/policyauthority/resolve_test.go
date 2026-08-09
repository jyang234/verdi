package policyauthority

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyartifact"
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
	// The BASE claim keeps its own (universal-scope) values: the overlay
	// narrows only within the overlay's own scope, never claim-wide.
	if len(goVersion.Values) != 2 || goVersion.Values[0] != "1.24" || goVersion.Values[1] != "1.25" {
		t.Fatalf("go-version base values = %v, want [1.24 1.25]", goVersion.Values)
	}
	if len(goVersion.Refinements) != 1 {
		t.Fatalf("go-version Refinements = %+v, want exactly one", goVersion.Refinements)
	}
	ref := goVersion.Refinements[0]
	if len(ref.Scope.Paths) != 1 || ref.Scope.Paths[0] != "web/" {
		t.Fatalf("refinement scope paths = %v, want [web/]", ref.Scope.Paths)
	}
	if len(ref.Values) != 1 || ref.Values[0] != "1.25" {
		t.Fatalf("refinement values = %v, want [1.25]", ref.Values)
	}
	if len(ref.Overlays) != 1 || ref.Overlays[0] != "policy-overlay/frontend-go-version" {
		t.Fatalf("refinement overlays = %v", ref.Overlays)
	}
	if len(verifyRequired.Refinements) != 0 {
		t.Fatalf("verify-required Refinements = %+v, want empty", verifyRequired.Refinements)
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
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
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

// findClaim returns the named claim of the named policy in ep, failing
// the test if either is absent.
func findClaim(t *testing.T, ep *EffectivePolicy, policyID, claimID string) *EffectiveClaim {
	t.Helper()
	for i := range ep.Policies {
		if ep.Policies[i].PolicyID != policyID {
			continue
		}
		for j := range ep.Policies[i].Claims {
			if ep.Policies[i].Claims[j].ID == claimID {
				return &ep.Policies[i].Claims[j]
			}
		}
	}
	t.Fatalf("policy %s claim %s not present in resolved output", policyID, claimID)
	return nil
}

// TestResolve_RefinementCarriesItsOwnScope proves an overlay's scope
// survives into the canonical output attached to the refinement it
// bounds (DC-3: an overlay is valid only "where ... its scope is within
// the permitted refinement boundary" — a refinement whose boundary is
// dropped would silently apply the narrowing claim-wide).
func TestResolve_RefinementCarriesItsOwnScope(t *testing.T) {
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
	data, err := canonjson.Marshal(ep)
	if err != nil {
		t.Fatalf("Marshal() error: %v", err)
	}
	if !strings.Contains(string(data), `"web/"`) {
		t.Fatalf("canonical output does not carry the overlay's own scope path:\n%s", data)
	}
}

// TestResolve_RefinementsGroupByIdenticalScope proves refinements group
// by canonically identical overlay scope: two overlays sharing one scope
// combine under the narrowing rules, while a third overlay at a
// different scope stays a separate recorded entry (combining ACROSS
// scopes needs scope-comparison semantics this unit does not own, so
// CO-1 requires recording, never silent combination).
func TestResolve_RefinementsGroupByIdenticalScope(t *testing.T) {
	files := rulesStoreFiles()
	files[".verdi/policy/overlays/o1.md"] = overlayFile("o1", `"web/"`, "allowed-region", `values: ["us-east", "us-west"]`)
	files[".verdi/policy/overlays/o2.md"] = overlayFile("o2", `"web/"`, "allowed-region", `values: ["us-east", "eu-west"]`)
	files[".verdi/policy/overlays/o3.md"] = overlayFile("o3", `"services/"`, "allowed-region", `values: ["us-west"]`)

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

	claim := findClaim(t, ep, "policy/rules", "allowed-region")
	if len(claim.Values) != 3 {
		t.Fatalf("base values = %v, want the unrefined base set", claim.Values)
	}
	if len(claim.Refinements) != 2 {
		t.Fatalf("Refinements = %+v, want exactly two scope groups", claim.Refinements)
	}

	byPath := map[string]ScopedRefinement{}
	for _, r := range claim.Refinements {
		if len(r.Scope.Paths) != 1 {
			t.Fatalf("refinement scope paths = %v, want exactly one", r.Scope.Paths)
		}
		byPath[r.Scope.Paths[0]] = r
	}

	web, ok := byPath["web/"]
	if !ok {
		t.Fatalf("no refinement recorded at scope web/: %+v", claim.Refinements)
	}
	if len(web.Values) != 1 || web.Values[0] != "us-east" {
		t.Fatalf("web/ refinement values = %v, want the [us-east] intersection", web.Values)
	}
	if len(web.Overlays) != 2 || web.Overlays[0] != "policy-overlay/o1" || web.Overlays[1] != "policy-overlay/o2" {
		t.Fatalf("web/ refinement overlays = %v, want both contributors sorted", web.Overlays)
	}

	svc, ok := byPath["services/"]
	if !ok {
		t.Fatalf("no refinement recorded at scope services/: %+v", claim.Refinements)
	}
	if len(svc.Values) != 1 || svc.Values[0] != "us-west" {
		t.Fatalf("services/ refinement values = %v, want [us-west] (never combined across scopes)", svc.Values)
	}
	if len(svc.Overlays) != 1 || svc.Overlays[0] != "policy-overlay/o3" {
		t.Fatalf("services/ refinement overlays = %v", svc.Overlays)
	}
}

// TestResolve_RefinementAtTheClaimScopeIsStillScoped proves an overlay
// whose scope EQUALS the claim's scope produces an ordinary
// ScopedRefinement carrying that scope: the representation is uniform,
// never flattened back into the base operand.
func TestResolve_RefinementAtTheClaimScopeIsStillScoped(t *testing.T) {
	files := rulesStoreFiles()
	files[".verdi/policy/overlays/o.md"] = overlayFile("o", `"services/", "web/"`, "scoped-region", `values: ["us-east"]`)
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
	claim := findClaim(t, ep, "policy/rules", "scoped-region")
	if len(claim.Values) != 2 {
		t.Fatalf("base values = %v, want the unrefined base set", claim.Values)
	}
	if len(claim.Refinements) != 1 {
		t.Fatalf("Refinements = %+v, want one entry", claim.Refinements)
	}
	got := claim.Refinements[0].Scope.Paths
	if len(got) != 2 || got[0] != "services/" || got[1] != "web/" {
		t.Fatalf("refinement scope paths = %v, want the claim's own scope restated", got)
	}
}

// TestResolve_BoundRefinementsGroupByScope proves the bound operators
// carry their narrowed bound (never a values operand) per scope group.
func TestResolve_BoundRefinementsGroupByScope(t *testing.T) {
	files := rulesStoreFiles()
	files[".verdi/policy/overlays/o1.md"] = overlayFile("o1", `"web/"`, "coverage-min", `bound: 80`)
	files[".verdi/policy/overlays/o2.md"] = overlayFile("o2", `"web/"`, "coverage-min", `bound: 85`)
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
	claim := findClaim(t, ep, "policy/rules", "coverage-min")
	if claim.Bound == nil || *claim.Bound != 70 {
		t.Fatalf("base bound = %v, want the unrefined 70", claim.Bound)
	}
	if len(claim.Refinements) != 1 {
		t.Fatalf("Refinements = %+v, want one scope group", claim.Refinements)
	}
	r := claim.Refinements[0]
	if r.Bound == nil || *r.Bound != 85 {
		t.Fatalf("refinement bound = %v, want the strictest (85) of the group", r.Bound)
	}
	if len(r.Values) != 0 {
		t.Fatalf("refinement values = %v, want none for a bound operator", r.Values)
	}
}

func TestEffectivePolicy_DigestRejectsHandBuilt(t *testing.T) {
	var nilEP *EffectivePolicy
	if _, err := nilEP.Digest(); err == nil {
		t.Fatal("(*EffectivePolicy)(nil).Digest() succeeded, want error")
	}
	if _, err := (&EffectivePolicy{Schema: EffectivePolicySchema}).Digest(); err == nil {
		t.Fatal("hand-built EffectivePolicy.Digest() succeeded, want error")
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
	// The Store the output came from is untouched by that mutation.
	if _, err := s.Policies["policy/go-toolchain"].Digest(); err != nil {
		t.Fatalf("store policy digest after output mutation: %v", err)
	}
}

// tamperScope rewrites every member of s in place, the way a caller
// holding an aliased slice would.
func tamperScope(s *policyartifact.Scope) {
	for i := range s.Phases {
		s.Phases[i] = "review"
	}
	for i := range s.Environments {
		s.Environments[i] = "tampered-env"
	}
	for i := range s.Paths {
		s.Paths[i] = "tampered/"
	}
	for i := range s.Refs {
		s.Refs[i] = "spec/tampered"
	}
}

// TestResolve_OutputDoesNotAliasStore proves the resolved value shares no
// mutable memory with the Store: rewriting every scope slice and payload
// map the output carries leaves every stored artifact's own digest intact
// and leaves the store resolving to exactly the same effective policy.
// Handing a caller a value that can corrupt the authority it was derived
// from would defeat both seals at once.
func TestResolve_OutputDoesNotAliasStore(t *testing.T) {
	root := filepath.Join("testdata", "store")
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	want, err := ep.Digest()
	if err != nil {
		t.Fatalf("Digest() error: %v", err)
	}
	policyDigestBefore, err := s.Policies["policy/go-toolchain"].Digest()
	if err != nil {
		t.Fatalf("policy Digest() error: %v", err)
	}

	for i := range ep.Policies {
		entry := &ep.Policies[i]
		for kind := range entry.Payloads {
			delete(entry.Payloads, kind)
		}
		entry.Payloads[testPayloadKind] = &testPayload{Note: "tampered"}
		for j := range entry.Claims {
			c := &entry.Claims[j]
			tamperScope(&c.Scope)
			for k := range c.Refinements {
				tamperScope(&c.Refinements[k].Scope)
			}
		}
	}
	for i := range ep.Exemptions {
		tamperScope(&ep.Exemptions[i].Scope)
	}

	policyDigestAfter, err := s.Policies["policy/go-toolchain"].Digest()
	if err != nil {
		t.Fatalf("policy Digest() after output mutation: %v", err)
	}
	if policyDigestAfter != policyDigestBefore {
		t.Fatalf("stored policy digest changed through the resolved output: %s -> %s", policyDigestBefore, policyDigestAfter)
	}
	if _, err := s.Exemptions["policy-exemption/legacy-service-go"].Digest(); err != nil {
		t.Fatalf("stored exemption Digest() after output mutation: %v", err)
	}

	again, err := Resolve(s)
	if err != nil {
		t.Fatalf("second Resolve() after output mutation: %v", err)
	}
	got, err := again.Digest()
	if err != nil {
		t.Fatalf("second Digest(): %v", err)
	}
	if got != want {
		t.Fatalf("store resolves differently after output mutation: %s, want %s", got, want)
	}
}
