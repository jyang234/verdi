package mcpserve

import (
	"context"
	"testing"
)

func TestGetDesignProvenance_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignProvenance(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		Entries []map[string]any `json:"entries"`
	}
	toolResultJSON(t, result, &out)
	if out.Entries == nil || len(out.Entries) != 0 {
		t.Fatalf("Entries = %v, want non-nil empty (no mutation applied yet)", out.Entries)
	}
}

func TestGetDesignProvenance_Negative(t *testing.T) {
	b, _ := newASDTestBackend(t)
	ctx := context.Background()

	t.Run("missing ref", func(t *testing.T) {
		if !isToolError(b.GetDesignProvenance(ctx, mustArgs(t, map[string]any{}))) {
			t.Fatal("get_design_provenance(no ref): want isError")
		}
	})
	t.Run("pinned ref rejected", func(t *testing.T) {
		result := b.GetDesignProvenance(ctx, mustArgs(t, map[string]any{"ref": "spec/sample@0123456789abcdef0123456789abcdef01234567"}))
		if !isToolError(result) {
			t.Fatal("get_design_provenance(pinned ref): want isError")
		}
	})
	t.Run("unknown spec", func(t *testing.T) {
		result := b.GetDesignProvenance(ctx, mustArgs(t, map[string]any{"ref": "spec/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("get_design_provenance(unknown spec): want isError")
		}
	})
}
