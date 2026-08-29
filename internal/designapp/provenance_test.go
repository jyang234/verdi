package designapp

import (
	"context"
	"testing"
)

func TestGetDesignProvenance(t *testing.T) {
	t.Run("no sidecar yet is a clean empty result, never an error", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		result, err := svc.GetDesignProvenance(context.Background(), root, GetDesignProvenanceRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignProvenance: %v", err)
		}
		if result.Schema != ProvenanceResultSchema {
			t.Fatalf("Schema = %q, want %q", result.Schema, ProvenanceResultSchema)
		}
		if result.Entries == nil || len(result.Entries) != 0 {
			t.Fatalf("Entries = %v, want non-nil empty", result.Entries)
		}
		if result.Identity.Spec != "spec/sample" {
			t.Fatalf("Identity = %+v", result.Identity)
		}
	})

	t.Run("a mutation's provenance entry is returned unflattened", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "revised", "anchor": "#problem"},
		})
		svc := NewService()
		response, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed != nil {
			t.Fatalf("seeding mutation: %v", typed)
		}

		result, err := svc.GetDesignProvenance(context.Background(), root, GetDesignProvenanceRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetDesignProvenance: %v", err)
		}
		if len(result.Entries) != 1 {
			t.Fatalf("Entries = %v, want exactly one", result.Entries)
		}
		entry := result.Entries[0]
		if entry.Spec != "spec/sample" || entry.ResultDigest != response.Result.ResultDigest {
			t.Fatalf("Entries[0] = %+v, want it to match the applied mutation", entry)
		}
		if !entry.Attribution.Unauthenticated {
			t.Fatalf("Entries[0].Attribution = %+v, want the delegated-agent unauthenticated marker", entry.Attribution)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		_, err := svc.GetDesignProvenance(context.Background(), root, GetDesignProvenanceRequest{Spec: "not-a-ref"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetDesignProvenance(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("identity resolution failure is operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		svc := NewService()
		svc.Identity = failingIdentityReader{}
		_, err := svc.GetDesignProvenance(context.Background(), root, GetDesignProvenanceRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetDesignProvenance(bad identity) = %+v, want operational", err)
		}
	})
}
