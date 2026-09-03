package constitutionapp

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestInspect_Happy(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	result, typed := svc.Inspect(context.Background(), root, InspectRequest{})
	if typed != nil {
		t.Fatalf("Inspect: %v", typed)
	}
	if result.Schema != InspectResultSchema {
		t.Fatalf("schema = %q, want %q", result.Schema, InspectResultSchema)
	}
	if !result.Accepted.Adopted || !result.Proposed.Adopted {
		t.Fatalf("expected both adopted, got accepted=%v proposed=%v", result.Accepted.Adopted, result.Proposed.Adopted)
	}
	if result.Identity.Branch != "main" {
		t.Fatalf("branch = %q, want main", result.Identity.Branch)
	}
	if result.Identity.Head != result.Identity.AcceptedHead {
		t.Fatalf("on main with no proposal, expected Head == AcceptedHead")
	}
	if len(result.Accepted.Layers) != 4 {
		t.Fatalf("expected 4 source layers (policy, overlay, exemption, disposition), got %d: %+v", len(result.Accepted.Layers), result.Accepted.Layers)
	}
	if result.Accepted.Effective == nil {
		t.Fatal("expected a non-nil effective policy")
	}
	assertLayerKindsExposed(t, result.Accepted.Layers)
	assertLayerKindsExposed(t, result.Proposed.Layers)
}

// assertLayerKindsExposed proves every constitution-store artifact KIND
// loadSnapshot knows how to project reaches the Snapshot's source-layer
// ledger with its own identity, ownership, scope, and digest — including
// the disposition layer, whose projection is otherwise the one branch of
// that ledger no fixture would exercise.
func assertLayerKindsExposed(t *testing.T, layers []SourceLayer) {
	t.Helper()
	byKind := map[string]SourceLayer{}
	for _, l := range layers {
		byKind[l.Kind] = l
	}
	for _, kind := range []string{
		policyartifact.KindPolicy,
		policyartifact.KindOverlay,
		policyartifact.KindExemption,
		policyartifact.KindDisposition,
	} {
		layer, ok := byKind[kind]
		if !ok {
			t.Fatalf("no %s source layer in %+v", kind, layers)
		}
		if layer.ID == "" || layer.Title == "" || len(layer.Owners) == 0 || layer.Digest == "" {
			t.Fatalf("%s layer is missing identity/ownership/digest: %+v", kind, layer)
		}
	}
	if got := byKind[policyartifact.KindDisposition]; got.ID != "policy-disposition/review-no-conflict" || len(got.Scope.Phases) != 1 || got.Scope.Phases[0] != "review" {
		t.Fatalf("disposition layer = %+v, want the fixture's own id and its declared review-phase scope", got)
	}
}

func TestInspect_NotAdopted(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README.md": "no constitution here\n"}, Message: "init"},
	})
	svc := testService()

	result, typed := svc.Inspect(context.Background(), repo.Dir, InspectRequest{})
	if typed != nil {
		t.Fatalf("Inspect: %v", typed)
	}
	if result.Accepted.Adopted {
		t.Fatal("expected accepted.Adopted == false")
	}
	if result.Accepted.Reason == "" {
		t.Fatal("expected a disclosed not-adopted reason")
	}
	if result.Proposed.Adopted {
		t.Fatal("expected proposed.Adopted == false")
	}
}

func TestInspect_InputInvalid(t *testing.T) {
	svc := testService()
	_, typed := svc.Inspect(context.Background(), "", InspectRequest{})
	if typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected a verdict failure for a missing root, got %+v", typed)
	}
}
