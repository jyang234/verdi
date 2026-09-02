package constitutionapp

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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
	if len(result.Accepted.Layers) != 3 {
		t.Fatalf("expected 3 source layers (policy, overlay, exemption), got %d: %+v", len(result.Accepted.Layers), result.Accepted.Layers)
	}
	if result.Accepted.Effective == nil {
		t.Fatal("expected a non-nil effective policy")
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
