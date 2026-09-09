package mcpserve

import (
	"context"
	"testing"
)

func TestPrepareDesignReview_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.PrepareDesignReview(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		Problem  string           `json:"problem"`
		Baseline map[string]any   `json:"baseline"`
		Changes  []map[string]any `json:"changes"`
	}
	toolResultJSON(t, result, &out)
	if out.Problem != "old problem" {
		t.Fatalf("Problem = %q, want old problem", out.Problem)
	}
	if out.Baseline["available"] != false {
		t.Fatalf("Baseline = %+v, want unavailable (never accepted yet)", out.Baseline)
	}
	if out.Changes == nil {
		t.Fatal("Changes must be a non-nil array")
	}
}

func TestPrepareDesignReview_Negative(t *testing.T) {
	b, _ := newASDTestBackend(t)
	ctx := context.Background()

	t.Run("missing ref", func(t *testing.T) {
		if !isToolError(b.PrepareDesignReview(ctx, mustArgs(t, map[string]any{}))) {
			t.Fatal("prepare_design_review(no ref): want isError")
		}
	})
	t.Run("object fragment rejected", func(t *testing.T) {
		result := b.PrepareDesignReview(ctx, mustArgs(t, map[string]any{"ref": "spec/sample#ac-1"}))
		if !isToolError(result) {
			t.Fatal("prepare_design_review(fragment ref): want isError")
		}
	})
	t.Run("unknown spec", func(t *testing.T) {
		result := b.PrepareDesignReview(ctx, mustArgs(t, map[string]any{"ref": "spec/does-not-exist"}))
		if !isToolError(result) {
			t.Fatal("prepare_design_review(unknown spec): want isError")
		}
	})
}
