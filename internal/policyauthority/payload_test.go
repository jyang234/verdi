package policyauthority

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// TestResolve_PayloadFlowsThroughTyped proves a payload kind registered
// via policyartifact.RegisterPayloadKind reaches Resolve's output as its
// own concrete type, never as an untyped map (DC-23/OD-5: feature
// governance configuration is a typed payload inside this one system, and
// this package adds no second resolver or untyped fallback around it).
func TestResolve_PayloadFlowsThroughTyped(t *testing.T) {
	root := "testdata/store"
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ep, err := Resolve(s)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	var entry *EffectivePolicyEntry
	for i := range ep.Policies {
		if ep.Policies[i].PolicyID == "policy/go-toolchain" {
			entry = &ep.Policies[i]
		}
	}
	if entry == nil {
		t.Fatal("policy/go-toolchain not present in resolved policies")
	}

	raw, ok := entry.Payloads[policyartifact.DesignAssistancePayloadKind]
	if !ok {
		t.Fatal("design_assistance payload missing from resolved policy")
	}
	dap, ok := raw.(*policyartifact.DesignAssistancePayload)
	if !ok {
		t.Fatalf("payload is %T, want *policyartifact.DesignAssistancePayload (typed flow-through)", raw)
	}
	if dap.Mode != "proposal-only" {
		t.Fatalf("Mode = %q, want proposal-only", dap.Mode)
	}
}

// testPayloadKind is a second registered payload kind that exists only in
// this package's test binary, so tests can exercise behavior that needs
// TWO typed kinds (the production registry currently holds only
// design_assistance). It registers through the ordinary
// policyartifact.RegisterPayloadKind seam — there is no untyped fallback
// to fake one with.
const testPayloadKind = "zz_test_payload"

type testPayload struct {
	Note string `yaml:"note" json:"note"`
}

func (p *testPayload) PayloadKind() string { return testPayloadKind }

func (p *testPayload) Validate() error {
	if p.Note == "" {
		return fmt.Errorf("note is required")
	}
	return nil
}

func init() {
	policyartifact.RegisterPayloadKind(testPayloadKind, func(raw []byte) (policyartifact.Payload, error) {
		var doc struct {
			Note *string `yaml:"note"`
		}
		if err := artifact.DecodeStrict(raw, &doc); err != nil {
			return nil, err
		}
		if doc.Note == nil {
			return nil, fmt.Errorf("note is missing")
		}
		return &testPayload{Note: *doc.Note}, nil
	})
}

// TestLoad_DuplicatePayloadKindIsDeterministic proves the duplicate-kind
// report names the same pair on every run when a policy registers more
// than one kind: a canonical failure may not depend on Go's randomized
// map iteration order (CO-3). The kinds are reported in sorted order, so
// design_assistance — not zz_test_payload — is the named collision.
func TestLoad_DuplicatePayloadKindIsDeterministic(t *testing.T) {
	both := "payloads: {design_assistance: {mode: off, layout: false}, zz_test_payload: {note: hello}}"
	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = strings.Replace(
		files[".verdi/policy/policies/go-toolchain.md"], "payloads: {}", both, 1)
	files[".verdi/policy/policies/second.md"] = `---
schema: verdi.policy/v1
id: policy/second
kind: policy
title: "Second policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
` + both + `
---
A second policy colliding on both payload kinds at once.
`
	root := t.TempDir()
	writeTree(t, root, files)

	for i := 0; i < 50; i++ {
		_, err := Load(root)
		if err == nil {
			t.Fatal("Load() succeeded, want a duplicate-payload-kind error")
		}
		if !strings.Contains(err.Error(), `"design_assistance"`) {
			t.Fatalf("run %d: error = %v, want the sorted-first kind design_assistance named on every run", i, err)
		}
	}
}
