package policyartifact

import (
	"strings"
	"testing"
)

// overlayTemplateLine and exemptionTemplateLine are the two docs'
// scaffold-provenance lines, named so negative cases can remove exactly
// them.
const (
	overlayTemplateLine   = `template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}`
	exemptionTemplateLine = `template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}`
)

func validOverlayDoc() string {
	return `---
schema: verdi.policy-overlay/v1
id: policy-overlay/frontend-go-version
kind: policy-overlay
title: "Frontend Go version overlay"
owners: [frontend-team]
refines: policy/go-toolchain
scope: {phases: [], environments: [], paths: ["web/"], refs: []}
refinements:
  - claim: go-version
    values: ["1.25"]
template: {identity: "embedded:policy-overlay.md", digest: "sha256:c42fbc9f6c30311c940c91199d018ce99930466aad1e56108389f5d9a4be04e6"}
---
The frontend build pins the newer toolchain.
`
}

func TestDecodeOverlay_Happy(t *testing.T) {
	o, err := DecodeOverlay([]byte(validOverlayDoc()))
	if err != nil {
		t.Fatalf("DecodeOverlay: %v", err)
	}
	if o.Refines != "policy/go-toolchain" {
		t.Fatalf("Refines = %q", o.Refines)
	}
	if len(o.Refinements) != 1 || o.Refinements[0].Claim != "go-version" {
		t.Fatalf("Refinements = %+v", o.Refinements)
	}
	if _, err := o.Digest(); err != nil {
		t.Fatalf("Digest: %v", err)
	}
}

func TestDecodeOverlay_Negative(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"missing refines", strings.Replace(validOverlayDoc(), "refines: policy/go-toolchain\n", "", 1), "refines"},
		{"refines a non-policy id", strings.Replace(validOverlayDoc(), "refines: policy/go-toolchain", "refines: policy-overlay/other", 1), "policy/"},
		{"empty refinements", strings.Replace(validOverlayDoc(), "refinements:\n  - claim: go-version\n    values: [\"1.25\"]\n", "refinements: []\n", 1), "at least one"},
		{"missing refinements", strings.Replace(validOverlayDoc(), "refinements:\n  - claim: go-version\n    values: [\"1.25\"]\n", "", 1), "refinements"},
		{"duplicate refinement claims", strings.Replace(validOverlayDoc(), "  - claim: go-version\n    values: [\"1.25\"]\n", "  - claim: go-version\n    values: [\"1.25\"]\n  - claim: go-version\n    values: [\"1.24\"]\n", 1), "duplicate"},
		{"refinement without operand", strings.Replace(validOverlayDoc(), "    values: [\"1.25\"]\n", "", 1), "operand"},
		{"refinement with both operands", strings.Replace(validOverlayDoc(), "    values: [\"1.25\"]", "    values: [\"1.25\"]\n    bound: 3", 1), "exactly one"},
		{"unknown field", strings.Replace(validOverlayDoc(), "refines: policy/go-toolchain", "refines: policy/go-toolchain\nweight: 3", 1), "weight"},
		{"empty body", strings.SplitN(validOverlayDoc(), "The frontend", 2)[0] + "\n", "rationale"},
		{"template absent", strings.Replace(validOverlayDoc(), overlayTemplateLine+"\n", "", 1), "template"},
		{"missing overlay scope", strings.Replace(validOverlayDoc(), "scope: {phases: [], environments: [], paths: [\"web/\"], refs: []}\n", "", 1), "scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeOverlay([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeOverlay = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeOverlay error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

func validExemptionDoc() string {
	return `---
schema: verdi.policy-exemption/v1
id: policy-exemption/legacy-service-go
kind: policy-exemption
title: "Legacy service stays on Go 1.23"
owners: [service-team]
scope: {phases: [], environments: [], paths: ["services/legacy/"], refs: []}
witnesses:
  - policy: policy/go-toolchain
    claim: go-version
    claim_digest: "sha256:1111111111111111111111111111111111111111111111111111111111111111"
compensating_controls:
  - "Weekly CVE review of the pinned toolchain."
approvals:
  - role: policy-owner
    principal: principal/github-org/YWxpY2U
expiry: "2026-12-31"
template: {identity: "embedded:policy-exemption.md", digest: "sha256:cf3977e08d4259c963e3b7ca9b974e2334d35548ac155b0e972bc7441733dad9"}
---
The legacy service cannot move until its cgo dependency updates.
`
}

func TestDecodeExemption_Happy(t *testing.T) {
	e, err := DecodeExemption([]byte(validExemptionDoc()))
	if err != nil {
		t.Fatalf("DecodeExemption: %v", err)
	}
	if len(e.Witnesses) != 1 || e.Witnesses[0].Policy != "policy/go-toolchain" {
		t.Fatalf("Witnesses = %+v", e.Witnesses)
	}
	if e.Expiry != "2026-12-31" {
		t.Fatalf("Expiry = %q", e.Expiry)
	}
	if _, err := e.Digest(); err != nil {
		t.Fatalf("Digest: %v", err)
	}
}

func TestDecodeExemption_ReviewConditionInsteadOfExpiry(t *testing.T) {
	doc := strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "review_condition: \"Review when the cgo dependency ships a fixed release.\"", 1)
	if _, err := DecodeExemption([]byte(doc)); err != nil {
		t.Fatalf("DecodeExemption: %v", err)
	}
}

func TestDecodeExemption_Negative(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"no expiry or review condition", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"\n", "", 1), "expiry or review"},
		{"bad expiry date", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "expiry: \"soon\"", 1), "date"},
		{"empty witnesses", strings.Replace(validExemptionDoc(), "witnesses:\n  - policy: policy/go-toolchain\n    claim: go-version\n    claim_digest: \"sha256:1111111111111111111111111111111111111111111111111111111111111111\"\n", "witnesses: []\n", 1), "witness"},
		{"witness bad digest", strings.Replace(validExemptionDoc(), "sha256:1111111111111111111111111111111111111111111111111111111111111111", "sha1:beef", 1), "digest"},
		{"witness bad policy ref", strings.Replace(validExemptionDoc(), "policy: policy/go-toolchain", "policy: overlay/x", 1), "policy/"},
		{"empty compensating controls", strings.Replace(validExemptionDoc(), "compensating_controls:\n  - \"Weekly CVE review of the pinned toolchain.\"\n", "compensating_controls: []\n", 1), "compensating"},
		{"empty approvals", strings.Replace(validExemptionDoc(), "approvals:\n  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n", "approvals: []\n", 1), "approval"},
		{"bad approval principal", strings.Replace(validExemptionDoc(), "principal: principal/github-org/YWxpY2U", "principal: alice", 1), "principal"},
		{"bad approval role", strings.Replace(validExemptionDoc(), "role: policy-owner", "role: Policy Owner", 1), "role"},
		{"unknown field", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "expiry: \"2026-12-31\"\nseverity: high", 1), "severity"},
		{"empty body", strings.SplitN(validExemptionDoc(), "The legacy", 2)[0] + "\n", "rationale"},
		{"template absent", strings.Replace(validExemptionDoc(), exemptionTemplateLine+"\n", "", 1), "template"},
		{"impossible expiry day", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "expiry: \"2026-02-31\"", 1), "calendar"},
		{"impossible expiry month", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "expiry: \"9999-99-99\"", 1), "calendar"},
		{"blank review condition", strings.Replace(validExemptionDoc(), "expiry: \"2026-12-31\"", "review_condition: \"   \"", 1), "blank"},
		{"blank compensating control", strings.Replace(validExemptionDoc(), "- \"Weekly CVE review of the pinned toolchain.\"", "- \"   \"", 1), "empty control"},
		{"multiline compensating control", strings.Replace(validExemptionDoc(), "- \"Weekly CVE review of the pinned toolchain.\"", "- \"one\\ntwo\"", 1), "single line"},
		{"blank title", strings.Replace(validExemptionDoc(), "title: \"Legacy service stays on Go 1.23\"", "title: \"   \"", 1), "blank"},
		{"non-kebab owner", strings.Replace(validExemptionDoc(), "owners: [service-team]", "owners: [\"Alice Smith\"]", 1), "kebab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeExemption([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeExemption = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeExemption error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}
