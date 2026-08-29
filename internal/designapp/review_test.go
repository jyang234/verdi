package designapp

import (
	"context"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

func TestPrepareDesignReview(t *testing.T) {
	t.Run("no accepted baseline discloses an unavailable diff but a full packet", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		result, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("PrepareDesignReview: %v", err)
		}
		if result.Baseline.Available {
			t.Fatalf("Baseline = %+v, want unavailable (never accepted yet)", result.Baseline)
		}
		if result.Baseline.Reason == "" {
			t.Fatal("Baseline.Reason must disclose why (CO-1)")
		}
		if result.Problem != "old problem" || result.Outcome != "old outcome" {
			t.Fatalf("Problem/Outcome = %q/%q", result.Problem, result.Outcome)
		}
		if len(result.AcceptanceCriteria) != 1 || len(result.Constraints) != 1 || len(result.Decisions) != 1 || len(result.OpenQuestions) != 1 || len(result.Stubs) != 1 {
			t.Fatalf("packet content incomplete: %+v", result)
		}
		if result.Changes == nil || result.Warnings == nil || result.InferredOrUnresolved == nil || result.UnclassifiedEdits == nil {
			t.Fatalf("collections must be non-nil arrays: %+v", result)
		}
	})

	t.Run("since the review base reports the exact semantic diff", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		acceptTestSpec(t, root, []byte(testSpecAccepted))
		// Mutate the draft after acceptance so a real accepted baseline
		// exists to diff against.
		base, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, "sample"))
		if err != nil {
			t.Fatal(err)
		}
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "revised problem", "anchor": "#problem"},
		})
		svc := NewService()
		if _, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t)); typed != nil {
			t.Fatalf("seeding mutation: %v", typed)
		}

		result, err2 := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
		if err2 != nil {
			t.Fatalf("PrepareDesignReview: %v", err2)
		}
		if !result.Baseline.Available || result.Baseline.Branch != "main" || result.Baseline.Commit == "" {
			t.Fatalf("Baseline = %+v, want available at main", result.Baseline)
		}
		if len(result.Changes) != 1 || result.Changes[0].Target != "problem" || result.Changes[0].Change != "replaced" {
			t.Fatalf("Changes = %+v, want one replaced problem change", result.Changes)
		}
		if result.Problem != "revised problem" {
			t.Fatalf("Problem = %q, want the mutated value", result.Problem)
		}
	})

	t.Run("direct edit after a typed mutation is disclosed unclassified", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "typed edit", "anchor": "#problem"},
		})
		svc := NewService()
		response, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed != nil {
			t.Fatalf("seeding mutation: %v", typed)
		}

		// A raw Markdown edit on top of the typed mutation (AC-4): write
		// directly, bypassing the typed core entirely.
		specPath := store.SpecPath(root, store.ZoneActive, "sample")
		content, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		direct := append(content, []byte("\nDirect addition.\n")...)
		if err := os.WriteFile(specPath, direct, 0o644); err != nil {
			t.Fatal(err)
		}

		result, err2 := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
		if err2 != nil {
			t.Fatalf("PrepareDesignReview: %v", err2)
		}
		found := false
		for _, edit := range result.UnclassifiedEdits {
			if edit.FromDigest == response.Result.ResultDigest {
				found = true
			}
		}
		if !found {
			t.Fatalf("UnclassifiedEdits = %+v, want the open gap from the last typed digest", result.UnclassifiedEdits)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "nope"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("PrepareDesignReview(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("missing spec is not-found", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/does-not-exist"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "spec-not-found" {
			t.Fatalf("PrepareDesignReview(missing spec) = %+v, want verdict spec-not-found", err)
		}
	})

	t.Run("nil policy source is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = nil
		_, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("PrepareDesignReview(nil policy) = %+v, want operational", err)
		}
	})
}
