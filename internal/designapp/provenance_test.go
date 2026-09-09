package designapp

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/store"
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
		// Wave 6 Task 1A / SI-176: every current writer emits v2, and a
		// delegated agent's provenance always records a resolved policy
		// digest (mutateActor's fixture store is adopted with a
		// design_assistance payload).
		if entry.Schema != designprovenance.SchemaV2 || entry.Policy == nil || entry.Policy.State != designprovenance.PolicyResolved || entry.Policy.Digest == "" {
			t.Fatalf("Entries[0].Policy = %+v, want a resolved v2 policy", entry.Policy)
		}
	})

	// TestGetDesignProvenance's own case for the explicit browser-human's
	// not-applicable arm: the read path (GetDesignProvenance's decode of
	// the committed sidecar) must return BOTH v2 policy arms unflattened,
	// not only the resolved one a delegated-agent mutation produces.
	// Writing the not-applicable sidecar entry directly (rather than
	// wiring a fake PolicySource into a full Mutation service) keeps this
	// a focused proof of the READ/projection path specifically — the
	// WRITE path for both arms is already proven end-to-end in
	// internal/draftmutation/service_test.go.
	t.Run("a not-applicable v2 policy entry projects unflattened too", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		before := draftmutation.DigestBytes([]byte(testSpec))
		after := draftmutation.DigestBytes([]byte(strings.Replace(testSpec, `text: "old problem"`, `text: "browser problem"`, 1)))
		notApplicable := designprovenance.Policy{State: designprovenance.PolicyNotApplicable}
		entry := designprovenance.Entry{
			Schema: designprovenance.SchemaV2, Spec: "spec/sample",
			PreviousDigest: before,
			ResultDigest:   after,
			Attribution:    governanceprincipal.NewUnauthenticatedAttribution(),
			Policy:         &notApplicable,
			Context:        designprovenance.UnavailableContext(),
			Operations:     []designprovenance.Operation{{Op: designprovenance.OpSetProblem, Text: "browser problem", Anchor: "#problem"}},
			Changes:        []designprovenance.Change{{Target: "problem", Change: designprovenance.ChangeReplaced, BeforeDigest: before, AfterDigest: after}},
			Excerpts:       []designprovenance.Excerpt{},
		}
		if err := entry.Seal(); err != nil {
			t.Fatalf("Seal: %v", err)
		}
		encoded, err := designprovenance.EncodeEntry(entry)
		if err != nil {
			t.Fatalf("EncodeEntry: %v", err)
		}
		path := store.DesignProvenancePath(root, store.ZoneActive, "sample")
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}

		svc := NewService()
		result, typed := svc.GetDesignProvenance(context.Background(), root, GetDesignProvenanceRequest{Spec: "spec/sample"})
		if typed != nil {
			t.Fatalf("GetDesignProvenance: %v", typed)
		}
		if len(result.Entries) != 1 {
			t.Fatalf("Entries = %+v, want exactly one", result.Entries)
		}
		got := result.Entries[0]
		if got.Schema != designprovenance.SchemaV2 || got.Policy == nil || got.Policy.State != designprovenance.PolicyNotApplicable || got.Policy.Digest != "" {
			t.Fatalf("Entries[0].Policy = %+v, want the honest not-applicable arm", got.Policy)
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
