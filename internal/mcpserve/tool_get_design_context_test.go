package mcpserve

import (
	"context"
	"testing"
)

func TestGetDesignContext_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignContext(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		CurrentDraft struct {
			ID string `json:"ID"`
		} `json:"current_draft"`
		ContextDigest string `json:"context_digest"`
	}
	toolResultJSON(t, result, &out)
	if out.CurrentDraft.ID != "spec/sample" {
		t.Fatalf("CurrentDraft.ID = %q, want spec/sample", out.CurrentDraft.ID)
	}
	if out.ContextDigest == "" {
		t.Fatal("ContextDigest is empty")
	}
}

func TestGetDesignContext_Negative(t *testing.T) {
	b, _ := newASDTestBackend(t)
	ctx := context.Background()

	t.Run("missing ref", func(t *testing.T) {
		if !isToolError(b.GetDesignContext(ctx, mustArgs(t, map[string]any{}))) {
			t.Fatal("get_design_context(no ref): want isError")
		}
	})
	t.Run("pinned ref rejected", func(t *testing.T) {
		result := b.GetDesignContext(ctx, mustArgs(t, map[string]any{"ref": "spec/sample@0123456789abcdef0123456789abcdef01234567"}))
		if !isToolError(result) {
			t.Fatal("get_design_context(pinned ref): want isError")
		}
	})
	t.Run("unresolvable child story", func(t *testing.T) {
		result := b.GetDesignContext(ctx, mustArgs(t, map[string]any{"ref": "spec/sample", "child_stories": []string{"spec/does-not-exist"}}))
		if !isToolError(result) {
			t.Fatal("get_design_context(bad child story): want isError")
		}
	})
	t.Run("unknown spec", func(t *testing.T) {
		result := b.GetDesignContext(ctx, mustArgs(t, map[string]any{"ref": "spec/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("get_design_context(unknown spec): want isError")
		}
	})
}
