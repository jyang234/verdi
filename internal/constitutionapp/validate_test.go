package constitutionapp

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestValidate_Proven(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()

	result, typed := svc.Validate(context.Background(), root, ValidateRequest{})
	if typed != nil {
		t.Fatalf("Validate: %v", typed)
	}
	if !result.Proven {
		t.Fatal("expected Proven == true")
	}
	if result.Schema != ValidateResultSchema {
		t.Fatalf("schema = %q, want %q", result.Schema, ValidateResultSchema)
	}
}

// TestValidate_CorruptedPolicy proves the required corrupted-policy test
// scenario: editing a claim referenced by an exemption witness without
// updating the witness's own pinned claim_digest makes the exemption a
// stale witness — a real internal/policyauthority cross-validation failure,
// never a synthetic error constitutionapp invents.
func TestValidate_CorruptedPolicy(t *testing.T) {
	root := buildFixtureRepo(t)
	path := root + "/.verdi/policy/policies/go-toolchain.md"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupted := strings.Replace(string(data), `values: ["1.25", "1.24"]`, `values: ["1.25", "1.24", "1.23"]`, 1)
	if corrupted == string(data) {
		t.Fatal("fixture replacement matched nothing")
	}
	if err := os.WriteFile(path, []byte(corrupted), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := testService()
	_, typed := svc.Validate(context.Background(), root, ValidateRequest{})
	if typed == nil {
		t.Fatal("expected a corrupted-policy verdict, got a clean result")
	}
	if typed.Classification != ClassificationVerdict {
		t.Fatalf("classification = %q, want verdict", typed.Classification)
	}
	if typed.Code != "corrupted-policy" {
		t.Fatalf("code = %q, want corrupted-policy", typed.Code)
	}
	if !strings.Contains(typed.Detail+typed.Error(), "stale witness") {
		t.Fatalf("expected the stale-witness cause to be visible, got %v", typed)
	}
}

func TestValidate_InputInvalid(t *testing.T) {
	svc := testService()
	_, typed := svc.Validate(context.Background(), "", ValidateRequest{})
	if typed == nil || typed.Classification != ClassificationVerdict {
		t.Fatalf("expected a verdict failure for a missing root, got %+v", typed)
	}
}
