package mcpserve

import (
	"context"
	"testing"
)

func TestGetDesignCapabilities_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignCapabilities(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		PolicyMode          string   `json:"policy_mode"`
		PermittedOperations []string `json:"permitted_operations"`
		MutationSchema      string   `json:"mutation_schema"`
	}
	toolResultJSON(t, result, &out)
	if out.PolicyMode != "draft-write" {
		t.Fatalf("PolicyMode = %q, want draft-write", out.PolicyMode)
	}
	if len(out.PermittedOperations) == 0 {
		t.Fatal("PermittedOperations is empty for mode draft-write")
	}
	if out.MutationSchema != "verdi.draftmutation/v1" {
		t.Fatalf("MutationSchema = %q", out.MutationSchema)
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
