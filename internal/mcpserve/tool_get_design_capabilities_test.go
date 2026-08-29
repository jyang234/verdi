package mcpserve

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/designapp"
)

// capabilitiesOut is the capability response's wire shape this package's
// tests read.
type capabilitiesOut struct {
	Schema              string   `json:"schema"`
	PolicyMode          string   `json:"policy_mode"`
	Mutable             bool     `json:"mutable"`
	PermittedOperations []string `json:"permitted_operations"`
	AvailableOperations []string `json:"available_operations"`
	MutationSchema      string   `json:"mutation_schema"`
	MutabilityRefusal   *struct {
		Precondition string `json:"precondition"`
		Detail       string `json:"detail"`
	} `json:"mutability_refusal"`
}

func TestGetDesignCapabilities_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignCapabilities(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out capabilitiesOut
	toolResultJSON(t, result, &out)
	if out.Schema != designapp.CapabilitiesResultSchema {
		t.Fatalf("schema = %q, want %q", out.Schema, designapp.CapabilitiesResultSchema)
	}
	if out.PolicyMode != "draft-write" {
		t.Fatalf("policy_mode = %q, want draft-write", out.PolicyMode)
	}
	if !out.Mutable || out.MutabilityRefusal != nil {
		t.Fatalf("mutable/mutability_refusal = %v/%+v, want a mutable draft", out.Mutable, out.MutabilityRefusal)
	}
	if len(out.PermittedOperations) == 0 {
		t.Fatal("permitted_operations is empty for a mutable draft in mode draft-write")
	}
	if len(out.AvailableOperations) != 6 {
		t.Fatalf("available_operations = %v, want all six ASD operations", out.AvailableOperations)
	}
	if out.MutationSchema != "verdi.draftmutation/v1" {
		t.Fatalf("mutation_schema = %q", out.MutationSchema)
	}
}

// TestGetDesignCapabilities_NonMutable proves the MCP surface reports the
// same refusal the kernel would apply, rather than advertising a write it
// cannot honor (CO-1).
func TestGetDesignCapabilities_NonMutable(t *testing.T) {
	b, _ := offModeASDTestBackend(t)
	result := b.GetDesignCapabilities(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out capabilitiesOut
	toolResultJSON(t, result, &out)
	if out.Mutable {
		t.Fatal("mutable = true for design_assistance mode off")
	}
	if out.MutabilityRefusal == nil || out.MutabilityRefusal.Precondition != designapp.PreconditionPolicyMode {
		t.Fatalf("mutability_refusal = %+v, want the policy-mode precondition", out.MutabilityRefusal)
	}
	if len(out.PermittedOperations) != 0 {
		t.Fatalf("permitted_operations = %v, want empty", out.PermittedOperations)
	}
	for _, operation := range out.AvailableOperations {
		if operation == "mutate_draft" {
			t.Fatalf("available_operations = %v, must withhold mutate_draft", out.AvailableOperations)
		}
	}
}

func TestGetDesignCapabilities_Negative(t *testing.T) {
	b, _ := newASDTestBackend(t)
	ctx := context.Background()

	t.Run("missing ref", func(t *testing.T) {
		if !isToolError(b.GetDesignCapabilities(ctx, mustArgs(t, map[string]any{}))) {
			t.Fatal("get_design_capabilities(no ref): want isError")
		}
	})
	t.Run("object fragment rejected", func(t *testing.T) {
		result := b.GetDesignCapabilities(ctx, mustArgs(t, map[string]any{"ref": "spec/sample#ac-1"}))
		if !isToolError(result) {
			t.Fatal("get_design_capabilities(fragment ref): want isError")
		}
	})
	t.Run("unknown spec", func(t *testing.T) {
		result := b.GetDesignCapabilities(ctx, mustArgs(t, map[string]any{"ref": "spec/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("get_design_capabilities(unknown spec): want isError")
		}
	})
}
