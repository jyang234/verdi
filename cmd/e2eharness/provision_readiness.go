package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// provisionReadiness adds the existing hermetic constitution fixture and its
// generated instruction projection to the serving design branch, then writes
// one canonical caller-owned context request beneath the physical checkout
// root. The request omits expected identity so verdi serve binds the exact
// current branch/HEAD during its one startup build.
func provisionReadiness(ctx context.Context, moduleRoot, storeRoot string) (string, error) {
	policyFiles := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	for _, rel := range policyFiles {
		source := filepath.Join(moduleRoot, "internal", "policyartifact", "testdata", "store", filepath.FromSlash(rel))
		data, err := os.ReadFile(source)
		if err != nil {
			return "", fmt.Errorf("reading readiness policy fixture %s: %w", rel, err)
		}
		target := filepath.Join(storeRoot, ".verdi", "policy", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", fmt.Errorf("creating readiness policy fixture directory %s: %w", rel, err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "", fmt.Errorf("writing readiness policy fixture %s: %w", rel, err)
		}
	}
	if _, err := instructionprojection.Generate(storeRoot); err != nil {
		return "", fmt.Errorf("generating readiness instruction projection: %w", err)
	}

	request := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseDesign,
		Scope: policyartifact.Scope{
			Phases:       []string{},
			Environments: []string{},
			Paths:        []string{},
			Refs:         []string{},
		},
		Spec: "spec/" + designSpecName,
	}
	data, err := contextcompile.EncodeRequest(request)
	if err != nil {
		return "", fmt.Errorf("encoding readiness context request: %w", err)
	}
	requestPath := filepath.Join(storeRoot, "context-request.json")
	if err := os.WriteFile(requestPath, data, 0o600); err != nil {
		return "", fmt.Errorf("writing readiness context request: %w", err)
	}
	if err := runGit(ctx, storeRoot, nil, "add", "--", ".verdi/policy", "AGENTS.md", filepath.Base(requestPath)); err != nil {
		return "", fmt.Errorf("staging readiness policy fixtures: %w", err)
	}
	if err := runGit(ctx, storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: readiness context fixtures"); err != nil {
		return "", fmt.Errorf("committing readiness policy fixtures: %w", err)
	}
	return requestPath, nil
}
