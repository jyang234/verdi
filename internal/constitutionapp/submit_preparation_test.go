package constitutionapp

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestSubmitPreparation_BlockedOnProvenConflict(t *testing.T) {
	root := buildUnauthorizedExemptionFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if prep.ReadyForSubmission {
		t.Fatal("expected ReadyForSubmission == false when a target has a mechanically proven conflict")
	}
	if len(prep.BlockingReasons) == 0 {
		t.Fatal("expected at least one disclosed blocking reason")
	}
	if !prep.Validation.Proven {
		t.Fatal("the proposal itself should still validate cleanly")
	}
}

func TestSubmitPreparation_ReadyWithUnprovenJudge(t *testing.T) {
	root := buildConflictFixtureRepo(t)
	svc := testService()

	prep, typed := svc.SubmitPreparation(context.Background(), root, SubmitPreparationRequest{
		Targets: []ImpactTarget{{
			Spec:    "spec/operand-feature",
			Phase:   contextcompile.PhaseDesign,
			Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
			Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
		}},
	})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	// A judge-unavailable/unproven verdict is a human-review posture, not an
	// automatic submission block (AC-3: unresolved semantic candidates
	// "require human disposition" through the normal pull-request review
	// this operation never replaces).
	if !prep.ReadyForSubmission {
		t.Fatalf("expected ReadyForSubmission == true for an unproven (not violated) verdict, got blocking reasons %v", prep.BlockingReasons)
	}
}

func TestSubmitPreparation_InputInvalid(t *testing.T) {
	svc := testService()
	if _, typed := svc.SubmitPreparation(context.Background(), "", SubmitPreparationRequest{}); typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected verdict for missing root, got %+v", typed)
	}
}
