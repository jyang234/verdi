package designapp

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
)

// fakeAlignFindings is a fixed-answer AlignFindings port fake: production
// align.Compute execs pinned external CLIs (CO-7), which no test in this
// repository invokes hermetically without its own canned-fixture
// machinery (internal/align's own test suite already covers that exec
// path) — this package's job is only to prove GetDesignContext composes
// the port correctly, which a fake proves exactly as well (the 04 §port
// pattern's own justification for a port at all).
type fakeAlignFindings struct {
	result *align.ComputedResult
	err    error
}

func (f fakeAlignFindings) Findings(context.Context, string, *artifact.SpecFrontmatter, string) (*align.ComputedResult, error) {
	return f.result, f.err
}

func TestGetDesignContext(t *testing.T) {
	t.Run("happy path returns the bounded content set", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Align = fakeAlignFindings{result: &align.ComputedResult{Findings: []artifact.Finding{
			{ID: "f-1", Kind: artifact.FindingComputed, Text: "undeclared boundary"},
		}}}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.CurrentDraft == nil || result.CurrentDraft.ID != "spec/sample" {
			t.Fatalf("CurrentDraft = %+v", result.CurrentDraft)
		}
		if len(result.RatifiedDecisions) != 1 || result.RatifiedDecisions[0].ID != "dc-1" {
			t.Fatalf("RatifiedDecisions = %+v", result.RatifiedDecisions)
		}
		if !result.VerdiGoFindings.Available || len(result.VerdiGoFindings.Findings) != 1 {
			t.Fatalf("VerdiGoFindings = %+v", result.VerdiGoFindings)
		}
		if result.PolicyDigest == "" || result.ContextDigest == "" {
			t.Fatalf("digests missing: %+v", result)
		}
		if result.ParentFeature != nil {
			t.Fatalf("ParentFeature = %+v, want nil (testSpec's only link is depends-on, not implements)", result.ParentFeature)
		}
	})

	t.Run("an implements link resolves the parent feature", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		writeChildStory(t, root, "child-one") // links: implements spec/sample#ac-1 (a feature)
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/child-one"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.ParentFeature == nil || result.ParentFeature.Ref != "spec/sample#ac-1" || result.ParentFeature.Content == nil || result.ParentFeature.Content.ID != "spec/sample" {
			t.Fatalf("ParentFeature = %+v, want spec/sample resolved", result.ParentFeature)
		}
		// The resolved parent feature's own ratified decisions are folded
		// in alongside the current draft's own (context.go).
		found := false
		for _, decision := range result.RatifiedDecisions {
			if decision.ID == "dc-1" {
				found = true
			}
		}
		if !found {
			t.Fatalf("RatifiedDecisions = %+v, want the parent feature's dc-1 folded in", result.RatifiedDecisions)
		}
	})

	t.Run("no toolchain configured discloses Verdi-go findings as unavailable", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.VerdiGoFindings.Available || result.VerdiGoFindings.Reason == "" {
			t.Fatalf("VerdiGoFindings = %+v, want a disclosed unavailable reason", result.VerdiGoFindings)
		}
	})

	t.Run("nil align port discloses unavailable rather than failing", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Align = nil
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.VerdiGoFindings.Available {
			t.Fatal("VerdiGoFindings.Available = true, want false for a nil port")
		}
	})

	t.Run("align failure is disclosed, never fatal to the whole read", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Align = fakeAlignFindings{err: errors.New("boom")}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.VerdiGoFindings.Available {
			t.Fatal("VerdiGoFindings.Available = true, want false on adapter failure")
		}
	})

	t.Run("explicitly named child story is resolved", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		writeChildStory(t, root, "child-one")
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample", ChildStories: []string{"spec/child-one"}})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if len(result.ChildStories) != 1 || result.ChildStories[0].Ref != "spec/child-one" || result.ChildStories[0].Content == nil {
			t.Fatalf("ChildStories = %+v", result.ChildStories)
		}
	})

	t.Run("unresolvable explicit child story is not-found", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample", ChildStories: []string{"spec/does-not-exist"}})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "child-story-not-found" {
			t.Fatalf("GetDesignContext(bad child) = %+v, want verdict child-story-not-found", err)
		}
	})

	t.Run("invalid child story ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample", ChildStories: []string{"not-a-ref"}})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetDesignContext(invalid child ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "nope"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetDesignContext(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("missing spec is not-found", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/does-not-exist"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "spec-not-found" {
			t.Fatalf("GetDesignContext(missing spec) = %+v, want verdict spec-not-found", err)
		}
	})

	t.Run("nil policy source is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = nil
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignContext(nil policy) = %+v, want operational", err)
		}
	})
}
