package designapp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/specstate"
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
		if result.Schema != ContextResultSchema {
			t.Fatalf("Schema = %q, want %q", result.Schema, ContextResultSchema)
		}
		if result.CurrentDraft == nil || result.CurrentDraft.ID != "spec/sample" {
			t.Fatalf("CurrentDraft = %+v", result.CurrentDraft)
		}
		// AC-5's applicable project POLICY, not merely its digest.
		if result.ApplicablePolicy.Mode != "draft-write" || result.ApplicablePolicy.PolicyID == "" {
			t.Fatalf("ApplicablePolicy = %+v, want the resolved design_assistance content", result.ApplicablePolicy)
		}
		if result.ApplicablePolicy.Layout {
			t.Fatal("ApplicablePolicy.Layout must be false in v1")
		}
		// The draft's OWN decisions are never ratified authority: dc-1 is
		// visible inside CurrentDraft and nowhere in ratified_decisions.
		if len(result.RatifiedDecisions) != 0 {
			t.Fatalf("RatifiedDecisions = %+v, want empty (the draft's own decisions are not ratified)", result.RatifiedDecisions)
		}
		if result.RatifiedDecisionsPosture.Reason != RatifiedNoParentFeature {
			t.Fatalf("RatifiedDecisionsPosture = %+v, want reason %q", result.RatifiedDecisionsPosture, RatifiedNoParentFeature)
		}
		if len(result.CurrentDraft.Decisions) != 1 || result.CurrentDraft.Decisions[0].ID != "dc-1" {
			t.Fatalf("CurrentDraft.Decisions = %+v, want the draft's own dc-1 still visible", result.CurrentDraft.Decisions)
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

	t.Run("an ACCEPTED parent feature's decisions are ratified authority", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		// Commit the parent feature's exact current bytes onto the default
		// branch: its Git-derived state becomes accepted-pending-build (the
		// exact active-zone revision is reachable from main), which is what
		// makes its decisions ratified rather than proposed.
		acceptTestSpec(t, root, []byte(testSpec))
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
		if result.ParentFeature.State != specstate.AcceptedPendingBuild {
			t.Fatalf("ParentFeature.State = %q, want accepted-pending-build", result.ParentFeature.State)
		}
		if len(result.RatifiedDecisions) != 1 || result.RatifiedDecisions[0].ID != "dc-1" {
			t.Fatalf("RatifiedDecisions = %+v, want the accepted parent's dc-1", result.RatifiedDecisions)
		}
		if result.RatifiedDecisionsPosture.Reason != RatifiedFromAcceptedParent ||
			result.RatifiedDecisionsPosture.Source != "spec/sample#ac-1" {
			t.Fatalf("RatifiedDecisionsPosture = %+v", result.RatifiedDecisionsPosture)
		}
	})

	t.Run("a PROPOSED parent feature contributes no ratified decisions", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		writeChildStory(t, root, "child-one")
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/child-one"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if result.ParentFeature == nil || result.ParentFeature.State != specstate.Proposed {
			t.Fatalf("ParentFeature = %+v, want a resolved, proposed parent", result.ParentFeature)
		}
		if len(result.RatifiedDecisions) != 0 {
			t.Fatalf("RatifiedDecisions = %+v, want empty for a proposed parent", result.RatifiedDecisions)
		}
		if result.RatifiedDecisionsPosture.Reason != RatifiedParentNotAccepted ||
			result.RatifiedDecisionsPosture.ParentState != specstate.Proposed {
			t.Fatalf("RatifiedDecisionsPosture = %+v, want the disclosed not-accepted posture", result.RatifiedDecisionsPosture)
		}
		// The parent's own decisions remain visible inside parent_feature —
		// withheld from "ratified", never hidden.
		if len(result.ParentFeature.Content.Decisions) != 1 {
			t.Fatalf("ParentFeature.Content.Decisions = %+v, want the parent's own dc-1 still visible", result.ParentFeature.Content.Decisions)
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

	t.Run("nil state projector is operational when a parent must be classified", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		writeChildStory(t, root, "child-one")
		svc := NewService()
		svc.State = nil
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		_, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/child-one"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignContext(nil state) = %+v, want operational", err)
		}
	})
}

// TestGetDesignContextPinnedReferences covers AC-5's "the spec's declared
// pinned context references" on a spec that actually declares some — the
// happy resolution through the shared internal/index seam, and the
// disclosed refusal when a well-formed pinned ref names nothing that
// resolves (never a silently dropped entry).
func TestGetDesignContextPinnedReferences(t *testing.T) {
	t.Run("declared pinned refs resolve through the shared index seam", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		head := gitHead(t, root)
		writePinnedContextSpec(t, root, "adr/0001-context@"+head)
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignContext: %v", err)
		}
		if len(result.PinnedContext) != 1 {
			t.Fatalf("PinnedContext = %+v, want exactly one resolved entry", result.PinnedContext)
		}
		item := result.PinnedContext[0]
		if item.Kind != "adr" || item.Title != "Pinned context ADR" || item.Body == "" {
			t.Fatalf("PinnedContext[0] = %+v, want the committed ADR's own kind/title/body", item)
		}
		if item.Ref != "adr/0001-context" {
			t.Fatalf("PinnedContext[0].Ref = %q", item.Ref)
		}
	})

	t.Run("an unresolvable pinned ref is a disclosed failure, never a silent omission", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		head := gitHead(t, root)
		writePinnedContextSpec(t, root, "adr/does-not-exist@"+head)
		svc := NewService()
		svc.Align = fakeAlignFindings{err: ErrAlignUnavailable}
		result, err := svc.GetDesignContext(context.Background(), root, GetDesignContextRequest{Spec: "spec/sample"})
		if err == nil {
			t.Fatalf("GetDesignContext(unresolvable pinned ref) = %+v, want a typed failure", result)
		}
		if err.Classification != ClassificationOperational || err.Code != "io-failure" {
			t.Fatalf("GetDesignContext(unresolvable pinned ref) = %+v, want operational io-failure", err)
		}
		if !strings.Contains(err.Detail, "adr/does-not-exist") {
			t.Fatalf("Detail = %q, must name the unresolvable ref", err.Detail)
		}
	})
}
