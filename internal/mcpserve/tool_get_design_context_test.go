package mcpserve

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jyang234/verdi/internal/designapp"
)

func TestGetDesignContext_Happy(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignContext(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		Schema       string `json:"schema"`
		CurrentDraft struct {
			ID string `json:"id"`
		} `json:"current_draft"`
		ApplicablePolicy struct {
			Mode string `json:"mode"`
		} `json:"applicable_policy"`
		ContextDigest string `json:"context_digest"`
	}
	toolResultJSON(t, result, &out)
	if out.Schema != designapp.ContextResultSchema {
		t.Fatalf("schema = %q, want %q", out.Schema, designapp.ContextResultSchema)
	}
	if out.CurrentDraft.ID != "spec/sample" {
		t.Fatalf("current_draft.id = %q, want spec/sample", out.CurrentDraft.ID)
	}
	if out.ApplicablePolicy.Mode != "draft-write" {
		t.Fatalf("applicable_policy.mode = %q, want draft-write", out.ApplicablePolicy.Mode)
	}
	if out.ContextDigest == "" {
		t.Fatal("context_digest is empty")
	}
}

// TestGetDesignContext_WireGrammar proves the response speaks the
// repository's snake_case wire grammar and leaks no internal authoring or
// lifecycle field (CO-2). encoding/json matches field names
// case-insensitively, so a typed decode alone would never catch a leaked
// Go field name — this asserts over the RAW keys.
func TestGetDesignContext_WireGrammar(t *testing.T) {
	b, _ := newASDTestBackend(t)
	result := b.GetDesignContext(context.Background(), mustArgs(t, map[string]any{"ref": "spec/sample"}))
	var out struct {
		CurrentDraft map[string]json.RawMessage `json:"current_draft"`
	}
	toolResultJSON(t, result, &out)
	for _, want := range []string{"id", "kind", "class", "title", "owners", "problem", "outcome", "context", "links", "acceptance_criteria", "constraints", "decisions", "open_questions", "stubs"} {
		if _, ok := out.CurrentDraft[want]; !ok {
			t.Fatalf("current_draft is missing snake_case key %q: %v", want, out.CurrentDraft)
		}
	}
	for _, forbidden := range []string{"ID", "Kind", "Title", "Owners", "AcceptanceCriteria", "OpenQuestions", "Base", "Custom", "Frozen", "Schema", "Status", "Dispositions", "Provenance", "Supersession", "Declares", "Impacts", "Spike"} {
		if _, ok := out.CurrentDraft[forbidden]; ok {
			t.Fatalf("current_draft leaks the internal or Go-named key %q: %v", forbidden, out.CurrentDraft)
		}
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
