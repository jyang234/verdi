package designapp

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

func TestGetDesignCapabilities(t *testing.T) {
	t.Run("draft-write mode reports the full closed operation vocabulary", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignCapabilities: %v", err)
		}
		if result.PolicyMode != "draft-write" {
			t.Fatalf("PolicyMode = %q, want draft-write", result.PolicyMode)
		}
		if len(result.PermittedOperations) != len(draftWriteOperations) {
			t.Fatalf("PermittedOperations = %v, want the full %d-operation vocabulary", result.PermittedOperations, len(draftWriteOperations))
		}
		if result.SpecState != specstate.Proposed {
			t.Fatalf("SpecState = %q, want proposed (unmerged design branch)", result.SpecState)
		}
		if result.MutationSchema == "" || result.ResultSchema == "" || result.PolicyDigest == "" || result.CurrentDigest == "" {
			t.Fatalf("CapabilitiesResult missing a required identity/digest field: %+v", result)
		}
		if result.Layout {
			t.Fatal("Layout must be false in v1")
		}
		if !result.Review.SemanticPacketAvailable {
			t.Fatal("Review.SemanticPacketAvailable must be true (AC-6 is non-configurable)")
		}
		if result.DirectMarkdown.Origin != "disclose" {
			t.Fatalf("DirectMarkdown.Origin = %q, want disclose (CO-1 default)", result.DirectMarkdown.Origin)
		}
		if len(result.AvailableOperations) != 6 {
			t.Fatalf("AvailableOperations = %v, want exactly the six ASD operations", result.AvailableOperations)
		}
	})

	t.Run("off mode reports no permitted operations", func(t *testing.T) {
		root := newTestStore(t, "off")
		svc := NewService()
		result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignCapabilities: %v", err)
		}
		if result.PolicyMode != "off" {
			t.Fatalf("PolicyMode = %q, want off", result.PolicyMode)
		}
		if len(result.PermittedOperations) != 0 {
			t.Fatalf("PermittedOperations = %v, want empty for mode off", result.PermittedOperations)
		}
	})

	t.Run("proposal-only mode reports no permitted operations", func(t *testing.T) {
		root := newTestStore(t, "proposal-only")
		svc := NewService()
		result, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignCapabilities: %v", err)
		}
		if len(result.PermittedOperations) != 0 {
			t.Fatalf("PermittedOperations = %v, want empty for mode proposal-only", result.PermittedOperations)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "nope"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetDesignCapabilities(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("missing spec is not-found", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/does-not-exist"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "spec-not-found" {
			t.Fatalf("GetDesignCapabilities(missing spec) = %+v, want verdict spec-not-found", err)
		}
	})

	t.Run("no policy authority adopted is a verdict", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = staticPolicySourceFor(t, nil, errPolicyNotAdopted)
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationVerdict {
			t.Fatalf("GetDesignCapabilities(no policy authority) = %+v, want verdict", err)
		}
	})

	t.Run("nil state projector is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.State = nil
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignCapabilities(nil state) = %+v, want operational", err)
		}
	})

	t.Run("nil policy source is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Policy = nil
		_, err := svc.GetDesignCapabilities(context.Background(), root, GetDesignCapabilitiesRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignCapabilities(nil policy) = %+v, want operational", err)
		}
	})
}
