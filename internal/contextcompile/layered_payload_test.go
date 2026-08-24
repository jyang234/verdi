package contextcompile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/policyartifact"
)

const contextLayeredPayloadKind = "zz_context_layered_payload"

type contextLayeredPayload struct {
	Values []string `json:"values" yaml:"values"`
}

func (p *contextLayeredPayload) PayloadKind() string { return contextLayeredPayloadKind }

func (p *contextLayeredPayload) Validate() error {
	if len(p.Values) == 0 {
		return fmt.Errorf("values are required")
	}
	return nil
}

func init() {
	policyartifact.RegisterLayeredPayloadKind(contextLayeredPayloadKind, func(raw []byte) (policyartifact.Payload, error) {
		var doc struct {
			Values *[]string `yaml:"values"`
		}
		if err := artifact.DecodeStrict(raw, &doc); err != nil {
			return nil, err
		}
		if doc.Values == nil {
			return nil, fmt.Errorf("values are missing")
		}
		return &contextLayeredPayload{Values: append([]string(nil), (*doc.Values)...)}, nil
	})
}

func TestSelectApplicableLayeredPayloadsUsesExactTargetScopeAndSealsCopies(t *testing.T) {
	root := installPolicyFixture(t)
	firstPath := filepath.Join(root, ".verdi", "policy", "policies", "go-toolchain.md")
	firstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first policy: %v", err)
	}
	first := strings.Replace(string(firstBytes),
		"scope: {phases: [], environments: [], paths: [], refs: []}",
		"scope: {phases: [build], environments: [local], paths: [.verdi/specs/active/story/spec.md], refs: [spec/story]}", 1)
	first = strings.Replace(first,
		"payloads:\n  design_assistance: {mode: proposal-only, layout: false}",
		"payloads: {zz_context_layered_payload: {values: [alpha, beta]}}", 1)
	if err := os.WriteFile(firstPath, []byte(first), 0o644); err != nil {
		t.Fatalf("write first policy: %v", err)
	}

	second := `---
schema: verdi.policy/v1
id: policy/disjoint
kind: policy
title: "Disjoint layered policy"
owners: [platform-team]
scope: {phases: [build], environments: [local], paths: [cmd/], refs: [spec/story]}
claims: []
instructions: []
payloads: {zz_context_layered_payload: {values: [beta]}}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
This layer is proven disjoint from the exact operation target.
`
	secondPath := filepath.Join(root, ".verdi", "policy", "policies", "disjoint.md")
	if err := os.WriteFile(secondPath, []byte(second), 0o644); err != nil {
		t.Fatalf("write second policy: %v", err)
	}

	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}
	selection, err := SelectApplicablePayloads(authority.Effective, contextLayeredPayloadKind, PayloadSelectionInput{
		Request:       policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		CandidatePath: ".verdi/specs/active/story/spec.md",
		CandidateRef:  "spec/story",
		Phase:         PhaseBuild,
		Environment:   "local",
	})
	if err != nil {
		t.Fatalf("SelectApplicablePayloads() error = %v", err)
	}
	layers, err := selection.Layers()
	if err != nil {
		t.Fatalf("selection.Layers() error = %v", err)
	}
	if len(layers) != 1 || layers[0].PolicyID != "policy/go-toolchain" {
		t.Fatalf("selected layers = %#v, want only exact-target policy/go-toolchain", layers)
	}
	layers[0].Payload.(*contextLayeredPayload).Values[0] = "mutated-copy"
	again, err := selection.Layers()
	if err != nil {
		t.Fatalf("selection.Layers() after copy mutation error = %v", err)
	}
	if got := again[0].Payload.(*contextLayeredPayload).Values[0]; got != "alpha" {
		t.Fatalf("sealed layer value = %q after caller mutation, want alpha", got)
	}
	if _, err := selection.Digest(); err != nil {
		t.Fatalf("selection.Digest() after caller mutation = %v", err)
	}
}

func TestSelectApplicableLayeredPayloadsBlocksUnknownApplicability(t *testing.T) {
	root := installPolicyFixture(t)
	policyPath := filepath.Join(root, ".verdi", "policy", "policies", "go-toolchain.md")
	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.Replace(string(data),
		"scope: {phases: [], environments: [], paths: [], refs: []}",
		"scope: {phases: [], environments: [], paths: [cmd/], refs: []}", 1)
	doc = strings.Replace(doc,
		"payloads:\n  design_assistance: {mode: proposal-only, layout: false}",
		"payloads: {zz_context_layered_payload: {values: [alpha]}}", 1)
	if err := os.WriteFile(policyPath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = SelectApplicablePayloads(authority.Effective, contextLayeredPayloadKind, PayloadSelectionInput{
		Request: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Phase:   PhaseBuild,
	})
	if err == nil || !strings.Contains(err.Error(), "applicability is unknown") {
		t.Fatalf("SelectApplicablePayloads() error = %v, want blocking unknown-applicability witness", err)
	}
}
