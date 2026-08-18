package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/instructionprojection"
)

func TestReadinessProvisioningCreatesCanonicalRequestAndProjection(t *testing.T) {
	moduleRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	storeRoot := filepath.Join(scratch, "store")
	if err := runGit(t.Context(), "", nil, "init", "--quiet", "--initial-branch=main", storeRoot); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(storeRoot, ".verdi", "verdi.yaml")
	specPath := filepath.Join(storeRoot, ".verdi", "specs", "active", designSpecName, "spec.md")
	for _, path := range []string{manifestPath, specPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(manifestPath, []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(specPath, []byte(designSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGit(t.Context(), storeRoot, nil, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(t.Context(), storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "seed readiness fixture"); err != nil {
		t.Fatal(err)
	}
	if err := runGit(t.Context(), storeRoot, nil, "checkout", "--quiet", "-b", designBranch); err != nil {
		t.Fatal(err)
	}

	requestPath, err := provisionReadiness(t.Context(), moduleRoot, storeRoot)
	if err != nil {
		t.Fatalf("provisionReadiness: %v", err)
	}
	if filepath.Dir(requestPath) != storeRoot {
		t.Fatalf("request path = %q, want a caller input beneath physical store root %q", requestPath, storeRoot)
	}
	requestBytes, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	request, err := contextcompile.DecodeRequest(requestBytes)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	if request.Schema != contextcompile.RequestSchema || request.Adapter != (contextcompile.AdapterRef{ID: "codex", Version: "1"}) || request.Phase != contextcompile.PhaseDesign || request.Spec != "spec/"+designSpecName || request.Expected != nil {
		t.Fatalf("request identity = %+v, want canonical codex/design/%s with startup-bound expected", request, designSpecName)
	}
	if !reflect.DeepEqual(request.Scope.Phases, []string{}) || !reflect.DeepEqual(request.Scope.Environments, []string{}) || !reflect.DeepEqual(request.Scope.Paths, []string{}) || !reflect.DeepEqual(request.Scope.Refs, []string{}) {
		t.Fatalf("request scope = %+v, want explicit universal empty slices", request.Scope)
	}
	branch, err := gitOutput(t.Context(), storeRoot, "branch", "--show-current")
	if err != nil {
		t.Fatal(err)
	}
	if branch != designBranch {
		t.Fatalf("serving branch = %q, want %q preserved", branch, designBranch)
	}
	projection, err := instructionprojection.Verify(storeRoot)
	if err != nil {
		t.Fatalf("instructionprojection.Verify: %v", err)
	}
	if !projection.Clean() {
		t.Fatalf("instruction projection findings = %+v, want clean", projection.Findings)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "AGENTS.md")); err != nil {
		t.Fatalf("managed AGENTS.md projection: %v", err)
	}
	status, err := gitOutput(t.Context(), storeRoot, "status", "--porcelain")
	if err != nil {
		t.Fatal(err)
	}
	if status != "" {
		t.Fatalf("provisioned store status = %q, want committed deterministic fixtures", status)
	}
}

func TestReadinessProvisioningFailsClosedOnMissingPolicyFixtures(t *testing.T) {
	scratch := t.TempDir()
	storeRoot := filepath.Join(scratch, "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := provisionReadiness(t.Context(), filepath.Join(scratch, "missing-module"), storeRoot); err == nil {
		t.Fatal("provisionReadiness with missing policy fixtures returned nil error")
	}
}
