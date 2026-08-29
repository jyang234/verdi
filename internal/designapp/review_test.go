package designapp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
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
		if result.Schema != ReviewResultSchema {
			t.Fatalf("Schema = %q, want %q", result.Schema, ReviewResultSchema)
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

	// A broken repository must NOT be reported as the verdict-free "spec is
	// not yet present on the default branch": that would silently pass off
	// an unanswerable question as a proven absence (CO-1). The default
	// branch's ref is repointed at a real blob, so it still resolves as a
	// ref while every tree read against it fails.
	t.Run("a git failure reading the review base is operational, not an absent baseline", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		breakDefaultBranchRef(t, root, "main")
		svc := NewService()
		result, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
		if err == nil {
			t.Fatalf("PrepareDesignReview(broken default branch) = %+v, want an operational failure", result.Baseline)
		}
		if err.Classification != ClassificationOperational || err.Code != "io-failure" {
			t.Fatalf("PrepareDesignReview(broken default branch) = %+v, want operational io-failure", err)
		}
	})
}

// TestPrepareDesignReviewWireGrammar proves the packet's nested
// projections speak snake_case and carry exactly AC-6's content list —
// "acceptance criteria and declared evidence kinds" included — with no
// leaked Go field name (CO-2). It asserts over the marshaled keys, since
// a typed decode would match field names case-insensitively.
func TestPrepareDesignReviewWireGrammar(t *testing.T) {
	root := newTestStore(t, "draft-write")
	svc := NewService()
	result, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
	if err != nil {
		t.Fatalf("PrepareDesignReview: %v", err)
	}
	encoded, marshalErr := canonjson.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var decoded struct {
		Schema             string                       `json:"schema"`
		AcceptanceCriteria []map[string]json.RawMessage `json:"acceptance_criteria"`
		Constraints        []map[string]json.RawMessage `json:"constraints"`
		Decisions          []map[string]json.RawMessage `json:"decisions"`
		OpenQuestions      []map[string]json.RawMessage `json:"open_questions"`
		Stubs              []map[string]json.RawMessage `json:"stubs"`
	}
	if unmarshalErr := json.Unmarshal(encoded, &decoded); unmarshalErr != nil {
		t.Fatalf("decoding packet: %v\n%s", unmarshalErr, encoded)
	}
	if decoded.Schema != ReviewResultSchema {
		t.Fatalf("schema = %q", decoded.Schema)
	}
	for _, want := range []string{"id", "text", "evidence", "anchor"} {
		if _, ok := decoded.AcceptanceCriteria[0][want]; !ok {
			t.Fatalf("acceptance_criteria[0] is missing %q: %v", want, decoded.AcceptanceCriteria[0])
		}
	}
	for _, forbidden := range []string{"ID", "Text", "Evidence", "Anchor"} {
		if _, ok := decoded.AcceptanceCriteria[0][forbidden]; ok {
			t.Fatalf("acceptance_criteria[0] leaks the Go-named key %q", forbidden)
		}
	}
	for name, block := range map[string][]map[string]json.RawMessage{
		"constraints": decoded.Constraints, "decisions": decoded.Decisions, "open_questions": decoded.OpenQuestions,
	} {
		if len(block) != 1 {
			t.Fatalf("%s = %v, want exactly the fixture's one entry", name, block)
		}
		if _, ok := block[0]["id"]; !ok {
			t.Fatalf("%s[0] is missing the snake_case id key: %v", name, block[0])
		}
	}
	if len(decoded.Stubs) != 1 {
		t.Fatalf("stubs = %v", decoded.Stubs)
	}
	if _, ok := decoded.Stubs[0]["acceptance_criteria"]; !ok {
		t.Fatalf("stubs[0] is missing acceptance_criteria: %v", decoded.Stubs[0])
	}
}

// TestPrepareDesignReviewInferredOrUnresolved covers AC-6's "objects
// classified as ai-inferred or unresolved" disclosure on a real sidecar
// produced by the typed core (excerpts carried on the mutation request),
// for both classifications, and proves a human-stated excerpt is not
// flagged.
func TestPrepareDesignReviewInferredOrUnresolved(t *testing.T) {
	for _, tc := range []struct {
		name           string
		classification designprovenance.ExcerptClassification
		wantFlagged    bool
	}{
		{name: "ai-inferred", classification: designprovenance.ClassificationAIInferred, wantFlagged: true},
		{name: "unresolved", classification: designprovenance.ClassificationUnresolved, wantFlagged: true},
		{name: "human-stated", classification: designprovenance.ClassificationHumanStated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newTestStore(t, "draft-write")
			base := []byte(testSpec)
			req := mutateRequestWithExcerpts(t, root, "design/sample", gitHead(t, root), base,
				[]map[string]any{{"op": "set-problem", "text": "restated problem", "anchor": "#problem"}},
				[]map[string]any{{
					"target": "problem", "classification": string(tc.classification),
					"representation": string(designprovenance.RepresentationParaphrase),
					"text":           "the agent's own restatement",
				}})
			svc := NewService()
			if _, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t)); typed != nil {
				t.Fatalf("seeding mutation: %v", typed)
			}

			result, err := svc.PrepareDesignReview(context.Background(), root, PrepareDesignReviewRequest{Spec: "spec/sample"})
			if err != nil {
				t.Fatalf("PrepareDesignReview: %v", err)
			}
			var flagged *ExcerptFlag
			for i, flag := range result.InferredOrUnresolved {
				if flag.Target == "problem" {
					flagged = &result.InferredOrUnresolved[i]
				}
			}
			if !tc.wantFlagged {
				if flagged != nil {
					t.Fatalf("InferredOrUnresolved = %+v, want no flag for a human-stated excerpt", result.InferredOrUnresolved)
				}
				return
			}
			if flagged == nil {
				t.Fatalf("InferredOrUnresolved = %+v, want the problem target flagged", result.InferredOrUnresolved)
			}
			if flagged.Classification != string(tc.classification) {
				t.Fatalf("flag classification = %q, want %q", flagged.Classification, tc.classification)
			}
		})
	}
}
