package policyauthority

import (
	"testing"

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
