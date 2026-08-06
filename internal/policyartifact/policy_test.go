package policyartifact

import (
	"strings"
	"testing"
)

// validPolicyDoc returns a complete, valid policy artifact document.
func validPolicyDoc() string {
	return `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Pin the Go toolchain so builds are reproducible.
`
}

func TestDecodePolicy_Happy(t *testing.T) {
	p, err := DecodePolicy([]byte(validPolicyDoc()))
	if err != nil {
		t.Fatalf("DecodePolicy: %v", err)
	}
	if p.ID != "policy/go-toolchain" {
		t.Fatalf("ID = %q", p.ID)
	}
	if p.Name() != "go-toolchain" {
		t.Fatalf("Name() = %q", p.Name())
	}
	if len(p.Claims) != 2 {
		t.Fatalf("claims = %d, want 2", len(p.Claims))
	}
	// Normalization sorted the claim value set and the claims by id.
	if p.Claims[0].ID != "go-version" || p.Claims[0].Values[0] != "1.24" {
		t.Fatalf("normalization: claims[0] = %+v", p.Claims[0])
	}
	if !strings.Contains(p.Rationale, "reproducible") {
		t.Fatalf("Rationale = %q", p.Rationale)
	}
	d, err := p.Digest()
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	if !strings.HasPrefix(d, "sha256:") {
		t.Fatalf("digest %q", d)
	}
}

func TestDecodePolicy_DigestOrderInsensitive(t *testing.T) {
	// The same semantic content with claims and value sets in a different
	// source order must produce a byte-identical digest.
	reordered := `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims:
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.24", "1.25"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
instructions:
  - "Run make verify before claiming completion."
payloads: {}
---
Pin the Go toolchain so builds are reproducible.
`
	a, err := DecodePolicy([]byte(validPolicyDoc()))
	if err != nil {
		t.Fatalf("DecodePolicy(a): %v", err)
	}
	b, err := DecodePolicy([]byte(reordered))
	if err != nil {
		t.Fatalf("DecodePolicy(b): %v", err)
	}
	da, _ := a.Digest()
	db, _ := b.Digest()
	if da != db {
		t.Fatalf("digests differ: %s vs %s", da, db)
	}
}

func TestDecodePolicy_Negative(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"unknown top-level field", strings.Replace(validPolicyDoc(), "payloads: {}", "payloads: {}\nextra: field", 1), "field extra not found"},
		{"wrong schema", strings.Replace(validPolicyDoc(), "verdi.policy/v1", "verdi.policy/v2", 1), "schema"},
		{"missing schema", strings.Replace(validPolicyDoc(), "schema: verdi.policy/v1\n", "", 1), "schema"},
		{"kind mismatch", strings.Replace(validPolicyDoc(), "kind: policy", "kind: overlay", 1), "kind"},
		{"id kind mismatch", strings.Replace(validPolicyDoc(), "id: policy/go-toolchain", "id: overlay/go-toolchain", 1), "id"},
		{"non-kebab name", strings.Replace(validPolicyDoc(), "id: policy/go-toolchain", "id: policy/Go_Toolchain", 1), "kebab"},
		{"missing title", strings.Replace(validPolicyDoc(), "title: \"Go toolchain policy\"\n", "", 1), "title"},
		{"empty owners", strings.Replace(validPolicyDoc(), "owners: [platform-team]", "owners: []", 1), "owners"},
		{"duplicate owners", strings.Replace(validPolicyDoc(), "owners: [platform-team]", "owners: [a, a]", 1), "duplicate"},
		{"missing scope", strings.Replace(validPolicyDoc(), "scope: {phases: [], environments: [], paths: [], refs: []}\nclaims:", "claims:", 1), "scope"},
		{"missing claims", strings.Replace(validPolicyDoc(), "claims:\n  - id: go-version\n    family: configuration\n    operator: allowed-values\n    subject: go-version\n    values: [\"1.25\", \"1.24\"]\n    scope: {phases: [], environments: [], paths: [], refs: []}\n    overridable: true\n  - id: verify-required\n    family: action\n    operator: required-values\n    subject: make-verify\n    values: [clean-exit]\n    scope: {phases: [build], environments: [], paths: [], refs: []}\n    overridable: false\n", "", 1), "claims"},
		{"missing instructions", strings.Replace(validPolicyDoc(), "instructions:\n  - \"Run make verify before claiming completion.\"\n", "", 1), "instructions"},
		{"missing payloads", strings.Replace(validPolicyDoc(), "payloads: {}\n", "", 1), "payloads"},
		{"duplicate claim ids", strings.Replace(validPolicyDoc(), "id: verify-required", "id: go-version", 1), "duplicate"},
		{"unknown payload kind", strings.Replace(validPolicyDoc(), "payloads: {}", "payloads:\n  mystery_feature: {x: 1}", 1), "unknown payload kind"},
		{"unknown claim operator", strings.Replace(validPolicyDoc(), "operator: allowed-values", "operator: fuzzy-match", 1), "operator"},
		{"unknown scope dimension", strings.Replace(validPolicyDoc(), "scope: {phases: [build], environments: [], paths: [], refs: []}", "scope: {phases: [], environments: [], paths: [], refs: [], hosts: []}", 1), "hosts"},
		{"empty body", strings.SplitN(validPolicyDoc(), "Pin the Go", 2)[0] + "\n", "rationale"},
		{"yaml alias", strings.Replace(validPolicyDoc(), "owners: [platform-team]", "owners: &o [platform-team]", 1), "anchor"},
		{"empty instruction", strings.Replace(validPolicyDoc(), "- \"Run make verify before claiming completion.\"", "- \"\"", 1), "instruction"},
		{"multiline instruction", strings.Replace(validPolicyDoc(), "- \"Run make verify before claiming completion.\"", "- \"line one\\nline two\"", 1), "single line"},
		{"instructions null entry", strings.Replace(validPolicyDoc(), "- \"Run make verify before claiming completion.\"", "- null", 1), "instruction"},
		{"template with bad digest", strings.Replace(validPolicyDoc(), "payloads: {}", "payloads: {}\ntemplate: {identity: \"embedded:policy.md\", digest: \"nothex\"}", 1), "digest"},
		{"template missing identity", strings.Replace(validPolicyDoc(), "payloads: {}", "payloads: {}\ntemplate: {digest: \"sha256:0000000000000000000000000000000000000000000000000000000000000000\"}", 1), "identity"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePolicy([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodePolicy = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodePolicy error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

func TestPolicySeal_RejectsForgeryAndMutation(t *testing.T) {
	// A hand-built value never yields a digest.
	var forged Policy
	forged.ID = "policy/forged"
	if _, err := forged.Digest(); err == nil {
		t.Fatal("hand-built policy yielded a digest")
	}
	// A post-decode mutation is detected.
	p, err := DecodePolicy([]byte(validPolicyDoc()))
	if err != nil {
		t.Fatalf("DecodePolicy: %v", err)
	}
	p.Title = "tampered"
	if _, err := p.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
		t.Fatalf("mutated policy Digest err = %v, want modification error", err)
	}
}

func TestDecodePolicy_DesignAssistancePayload(t *testing.T) {
	doc := strings.Replace(validPolicyDoc(), "payloads: {}",
		"payloads:\n  design_assistance: {mode: proposal-only, layout: false}", 1)
	p, err := DecodePolicy([]byte(doc))
	if err != nil {
		t.Fatalf("DecodePolicy: %v", err)
	}
	pl, ok := p.Payloads["design_assistance"].(*DesignAssistancePayload)
	if !ok {
		t.Fatalf("payload type = %T", p.Payloads["design_assistance"])
	}
	if pl.Mode != "proposal-only" {
		t.Fatalf("mode = %q", pl.Mode)
	}

	bad := []struct {
		name, payload, wantSub string
	}{
		{"unknown mode", "{mode: full-auto, layout: false}", "mode"},
		{"layout true", "{mode: off, layout: true}", "layout"},
		{"missing mode", "{layout: false}", "mode"},
		{"unknown field", "{mode: off, layout: false, extra: 1}", "extra"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			doc := strings.Replace(validPolicyDoc(), "payloads: {}",
				"payloads:\n  design_assistance: "+tt.payload, 1)
			_, err := DecodePolicy([]byte(doc))
			if err == nil || !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}
