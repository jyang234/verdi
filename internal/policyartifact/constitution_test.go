package policyartifact

import (
	"strings"
	"testing"
)

func validConstitutionDoc() string {
	return `---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: "Project constitution"
owners: [platform-team]
selected_profile: solo-default
environments: [local, production]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, close]
  evidence_sources: [ci]
  escalation_metrics: [age-days]
subjects:
  action: [make-verify]
  configuration: [go-version]
  capability: []
  resource: [repo-tree]
  identity: [exemption-approval]
  evidence: [verify-receipt]
adapters:
  - id: codex
    version: "1"
    managed: [AGENTS.md]
    discovery_filenames: [AGENTS.md]
---
This store adopts the constitution capability.
`
}

func TestDecodeConstitution_Happy(t *testing.T) {
	c, err := DecodeConstitution([]byte(validConstitutionDoc()))
	if err != nil {
		t.Fatalf("DecodeConstitution: %v", err)
	}
	if c.SelectedProfile != "solo-default" {
		t.Fatalf("SelectedProfile = %q", c.SelectedProfile)
	}
	if len(c.Adapters) != 1 || c.Adapters[0].ID != "codex" {
		t.Fatalf("Adapters = %+v", c.Adapters)
	}
	// The governance catalog converts to the kernel's own catalog type.
	cat, err := c.GovernanceCatalog()
	if err != nil {
		t.Fatalf("GovernanceCatalog(): %v", err)
	}
	if err := cat.Validate(); err != nil {
		t.Fatalf("GovernanceCatalog().Validate(): %v", err)
	}
	if _, err := c.Digest(); err != nil {
		t.Fatalf("Digest: %v", err)
	}
}

// TestGovernanceCatalog_RefusesForgedConstitution proves the catalog
// egress verifies the seal: a hand-built constitution must never supply
// the vocabulary a governance profile validates against.
func TestGovernanceCatalog_RefusesForgedConstitution(t *testing.T) {
	var forged Constitution
	forged.Catalog = GovernanceCatalog{Roles: []string{"attacker-role"}}
	if _, err := forged.GovernanceCatalog(); err == nil {
		t.Fatal("hand-built constitution yielded a governance catalog")
	}
}

// TestSeal_RejectsForgeryAndMutation_AllKinds extends the policy seal
// negatives to every sealed kind (CO-6: every decoder gets negative
// coverage; the SI-21 posture is package-wide, not policy-only).
func TestSeal_RejectsForgeryAndMutation_AllKinds(t *testing.T) {
	t.Run("overlay", func(t *testing.T) {
		var forged Overlay
		if _, err := forged.Digest(); err == nil {
			t.Fatal("hand-built overlay yielded a digest")
		}
		o, err := DecodeOverlay([]byte(validOverlayDoc()))
		if err != nil {
			t.Fatalf("DecodeOverlay: %v", err)
		}
		o.Refines = "policy/other"
		if _, err := o.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
			t.Fatalf("mutated overlay Digest err = %v, want modification error", err)
		}
	})
	t.Run("exemption", func(t *testing.T) {
		var forged Exemption
		if _, err := forged.Digest(); err == nil {
			t.Fatal("hand-built exemption yielded a digest")
		}
		e, err := DecodeExemption([]byte(validExemptionDoc()))
		if err != nil {
			t.Fatalf("DecodeExemption: %v", err)
		}
		e.Expiry = "2099-01-01"
		if _, err := e.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
			t.Fatalf("mutated exemption Digest err = %v, want modification error", err)
		}
	})
	t.Run("constitution", func(t *testing.T) {
		var forged Constitution
		if _, err := forged.Digest(); err == nil {
			t.Fatal("hand-built constitution yielded a digest")
		}
		c, err := DecodeConstitution([]byte(validConstitutionDoc()))
		if err != nil {
			t.Fatalf("DecodeConstitution: %v", err)
		}
		c.SelectedProfile = "attacker-profile"
		if _, err := c.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
			t.Fatalf("mutated constitution Digest err = %v, want modification error", err)
		}
	})
}

func TestDecodeConstitution_Negative(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"missing selected_profile", strings.Replace(validConstitutionDoc(), "selected_profile: solo-default\n", "", 1), "selected_profile"},
		{"bad profile id grammar", strings.Replace(validConstitutionDoc(), "selected_profile: solo-default", "selected_profile: \"Solo Default\"", 1), "id"},
		{"fixed id enforced", strings.Replace(validConstitutionDoc(), "id: policy-constitution/constitution", "id: policy-constitution/other", 1), "constitution"},
		{"missing catalog", strings.Replace(validConstitutionDoc(), "catalog:\n  roles: [author, reviewer, policy-owner]\n  transitions: [accept, close]\n  evidence_sources: [ci]\n  escalation_metrics: [age-days]\n", "", 1), "catalog"},
		{"missing catalog roles", strings.Replace(validConstitutionDoc(), "  roles: [author, reviewer, policy-owner]\n", "", 1), "roles"},
		{"duplicate catalog role", strings.Replace(validConstitutionDoc(), "roles: [author, reviewer, policy-owner]", "roles: [author, author]", 1), "duplicate"},
		{"missing subjects", strings.Replace(validConstitutionDoc(), "subjects:\n  action: [make-verify]\n  configuration: [go-version]\n  capability: []\n  resource: [repo-tree]\n  identity: [exemption-approval]\n  evidence: [verify-receipt]\n", "", 1), "subjects"},
		{"missing subjects family", strings.Replace(validConstitutionDoc(), "  capability: []\n", "", 1), "capability"},
		{"non-kebab subject", strings.Replace(validConstitutionDoc(), "configuration: [go-version]", "configuration: [Go_Version]", 1), "kebab"},
		{"duplicate subject", strings.Replace(validConstitutionDoc(), "action: [make-verify]", "action: [make-verify, make-verify]", 1), "duplicate"},
		{"missing adapters", strings.Replace(validConstitutionDoc(), "adapters:\n  - id: codex\n    version: \"1\"\n    managed: [AGENTS.md]\n    discovery_filenames: [AGENTS.md]\n", "", 1), "adapters"},
		{"adapter missing version", strings.Replace(validConstitutionDoc(), "    version: \"1\"\n", "", 1), "version"},
		{"adapter empty managed", strings.Replace(validConstitutionDoc(), "managed: [AGENTS.md]", "managed: []", 1), "managed"},
		{"adapter absolute managed path", strings.Replace(validConstitutionDoc(), "managed: [AGENTS.md]", "managed: [/etc/agents]", 1), "absolute"},
		{"adapter escaping managed path", strings.Replace(validConstitutionDoc(), "managed: [AGENTS.md]", "managed: [\"../AGENTS.md\"]", 1), "escape"},
		{"adapter empty discovery", strings.Replace(validConstitutionDoc(), "discovery_filenames: [AGENTS.md]", "discovery_filenames: []", 1), "discovery"},
		{"adapter discovery not bare filename", strings.Replace(validConstitutionDoc(), "discovery_filenames: [AGENTS.md]", "discovery_filenames: [\"docs/AGENTS.md\"]", 1), "bare filename"},
		{"duplicate adapter id", strings.Replace(validConstitutionDoc(), "adapters:\n  - id: codex", "adapters:\n  - id: codex\n    version: \"0\"\n    managed: [CLAUDE.md]\n    discovery_filenames: [CLAUDE.md]\n  - id: codex", 1), "duplicate"},
		{"missing environments", strings.Replace(validConstitutionDoc(), "environments: [local, production]\n", "", 1), "environments"},
		{"unknown field", strings.Replace(validConstitutionDoc(), "selected_profile: solo-default", "selected_profile: solo-default\nmotto: onward", 1), "motto"},
		{"empty body", strings.SplitN(validConstitutionDoc(), "This store", 2)[0] + "\n", "rationale"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeConstitution([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeConstitution = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeConstitution error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}
