package artifact

import (
	"strings"
	"testing"
)

func TestDecodeObligation_Happy(t *testing.T) {
	y := "id: obligation/loan-refi--ac-2--behavioral\n" +
		"kind: obligation\n" +
		"title: \"Charge API retried on stale decline\"\n" +
		"owners: [platform-team]\n" +
		"for_kind: behavioral\n" +
		"links:\n" +
		"  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n"
	fm, err := DecodeObligation([]byte(y))
	if err != nil {
		t.Fatalf("DecodeObligation: %v", err)
	}
	if fm.Frozen == nil {
		t.Fatal("Frozen is nil")
	}
	if fm.ForKind != EvidenceBehavioral {
		t.Fatalf("ForKind = %q, want %q", fm.ForKind, EvidenceBehavioral)
	}
	if len(fm.Links) != 1 || fm.Links[0].Type != LinkVerifies || fm.Links[0].Ref != "spec/loan-refi" {
		t.Fatalf("Links = %+v", fm.Links)
	}
}

func TestDecodeObligation_QualityStates(t *testing.T) {
	tests := []struct {
		name    string
		kind    EvidenceKind
		quality string
		state   ObligationQualityState
	}{
		{
			name:    "unresolved",
			kind:    EvidenceBehavioral,
			quality: "quality:\n  state: unresolved-design-debt\n",
			state:   ObligationQualityUnresolved,
		},
		{
			name: "mechanical elaborated",
			kind: EvidenceBehavioral,
			quality: "quality:\n  state: elaborated\n  claim: retries exactly once\n  falsifier: a second retry is observed\n  scope: stale decline path\n" +
				"  producer: { kind: checker, ref: \"verify:behavioral\" }\n" +
				"  authoritative_source: { kind: ci-job, ref: \"verify\" }\n" +
				"  freshness:\n    invalidated_by: [spec, code]\n    rule: rerun on accepted spec or code change\n",
			state: ObligationQualityElaborated,
		},
		{
			name: "attestation elaborated",
			kind: EvidenceAttestation,
			quality: "quality:\n  state: elaborated\n  claim: owner approved the outcome\n  falsifier: approval is withdrawn\n  scope: accepted outcome\n" +
				"  producer: { kind: authenticated-human, ref: \"role:owner\" }\n" +
				"  authoritative_source: { kind: governed-attestation, ref: \"approval:owner\" }\n" +
				"  freshness:\n    invalidated_by: [spec]\n    rule: renew after accepted spec change\n",
			state: ObligationQualityElaborated,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := DecodeObligation([]byte(obligationYAML(tt.kind, tt.quality)))
			if err != nil {
				t.Fatalf("DecodeObligation: %v", err)
			}
			if fm.Quality == nil || fm.Quality.State != tt.state {
				t.Fatalf("Quality = %+v, want state %q", fm.Quality, tt.state)
			}
		})
	}
}

func TestDecodeObligation_QualityLegacyAbsence(t *testing.T) {
	fm, err := DecodeObligation([]byte(obligationYAML(EvidenceBehavioral, "")))
	if err != nil {
		t.Fatalf("DecodeObligation: %v", err)
	}
	if fm.Quality != nil {
		t.Fatalf("Quality = %+v, want nil legacy absence", fm.Quality)
	}
}

func TestDecodeObligation_QualityNegative(t *testing.T) {
	elaborated := "quality:\n  state: elaborated\n  claim: claim\n  falsifier: falsifier\n  scope: scope\n" +
		"  producer: { kind: checker, ref: \"verify:behavioral\" }\n" +
		"  authoritative_source: { kind: ci-job, ref: \"verify\" }\n" +
		"  freshness:\n    invalidated_by: [spec, code]\n    rule: rerun\n"
	tests := []struct {
		name    string
		kind    EvidenceKind
		quality string
	}{
		{"unknown state", EvidenceBehavioral, "quality:\n  state: complete\n"},
		{"unresolved carries claim", EvidenceBehavioral, "quality:\n  state: unresolved-design-debt\n  claim: not allowed\n"},
		{"missing claim", EvidenceBehavioral, strings.Replace(elaborated, "  claim: claim\n", "", 1)},
		{"missing falsifier", EvidenceBehavioral, strings.Replace(elaborated, "  falsifier: falsifier\n", "", 1)},
		{"missing scope", EvidenceBehavioral, strings.Replace(elaborated, "  scope: scope\n", "", 1)},
		{"missing producer", EvidenceBehavioral, strings.Replace(elaborated, "  producer: { kind: checker, ref: \"verify:behavioral\" }\n", "", 1)},
		{"missing source", EvidenceBehavioral, strings.Replace(elaborated, "  authoritative_source: { kind: ci-job, ref: \"verify\" }\n", "", 1)},
		{"missing freshness", EvidenceBehavioral, strings.Replace(elaborated, "  freshness:\n    invalidated_by: [spec, code]\n    rule: rerun\n", "", 1)},
		{"blank claim", EvidenceBehavioral, strings.Replace(elaborated, "claim: claim", "claim: \"   \"", 1)},
		{"unnormalized scope", EvidenceBehavioral, strings.Replace(elaborated, "scope: scope", "scope: \" scope \"", 1)},
		{"unknown producer", EvidenceBehavioral, strings.Replace(elaborated, "kind: checker", "kind: shell", 1)},
		{"blank producer ref", EvidenceBehavioral, strings.Replace(elaborated, "ref: \"verify:behavioral\"", "ref: \"\"", 1)},
		{"unknown source", EvidenceBehavioral, strings.Replace(elaborated, "kind: ci-job", "kind: local", 1)},
		{"duplicate invalidator", EvidenceBehavioral, strings.Replace(elaborated, "[spec, code]", "[spec, code, spec]", 1)},
		{"unknown invalidator", EvidenceBehavioral, strings.Replace(elaborated, "[spec, code]", "[spec, moon]", 1)},
		{"empty invalidators", EvidenceBehavioral, strings.Replace(elaborated, "[spec, code]", "[]", 1)},
		{"unknown quality field", EvidenceBehavioral, elaborated + "  surprise: true\n"},
		{"duplicate quality state", EvidenceBehavioral, "quality:\n  state: elaborated\n  state: elaborated\n" + strings.TrimPrefix(elaborated, "quality:\n  state: elaborated\n")},
		{"unknown producer field", EvidenceBehavioral, strings.Replace(elaborated, "producer: { kind: checker, ref: \"verify:behavioral\" }", "producer: { kind: checker, ref: \"verify:behavioral\", surprise: true }", 1)},
		{"duplicate producer field", EvidenceBehavioral, strings.Replace(elaborated, "producer: { kind: checker, ref: \"verify:behavioral\" }", "producer: { kind: checker, kind: checker, ref: \"verify:behavioral\" }", 1)},
		{"unknown freshness field", EvidenceBehavioral, strings.Replace(elaborated, "    rule: rerun\n", "    rule: rerun\n    surprise: true\n", 1)},
		{"mechanical with human producer", EvidenceBehavioral, strings.Replace(elaborated, "kind: checker", "kind: authenticated-human", 1)},
		{"mechanical with attestation source", EvidenceBehavioral, strings.Replace(elaborated, "kind: ci-job", "kind: governed-attestation", 1)},
		{"attestation with checker", EvidenceAttestation, elaborated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeObligation([]byte(obligationYAML(tt.kind, tt.quality))); err == nil {
				t.Fatal("DecodeObligation = nil error, want strict quality refusal")
			}
		})
	}
}

func TestDecodeObligation_UnresolvedRejectsMeaningKeysByPresence(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value string
	}{
		{"claim empty string", "claim", `""`},
		{"claim null", "claim", "null"},
		{"falsifier empty string", "falsifier", `""`},
		{"falsifier null", "falsifier", "null"},
		{"scope empty string", "scope", `""`},
		{"scope null", "scope", "null"},
		{"producer empty object", "producer", "{}"},
		{"producer null", "producer", "null"},
		{"authoritative source empty object", "authoritative_source", "{}"},
		{"authoritative source null", "authoritative_source", "null"},
		{"freshness empty object", "freshness", "{}"},
		{"freshness null", "freshness", "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quality := "quality:\n  state: unresolved-design-debt\n  " + tt.field + ": " + tt.value + "\n"
			if _, err := DecodeObligation([]byte(obligationYAML(EvidenceBehavioral, quality))); err == nil {
				t.Fatalf("DecodeObligation accepted unresolved %s key with value %s; key presence must be rejected", tt.field, tt.value)
			}
		})
	}
}

func obligationYAML(kind EvidenceKind, quality string) string {
	return "id: obligation/loan-refi--ac-2--" + string(kind) + "\n" +
		"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: " + string(kind) + "\n" + quality +
		"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
		"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n"
}

// TestDecodeObligation_FullDocument_RoundTrips is the "round-trips through
// the internal/artifact seam" evidence AC-1 calls for: a realistic full
// document (frontmatter + prose body), split via SplitFrontmatter exactly
// as internal/index and internal/lint's walk do, decodes cleanly and
// preserves both the frontmatter fields and the body prose.
func TestDecodeObligation_FullDocument_RoundTrips(t *testing.T) {
	doc := []byte(`---
id: obligation/loan-refi--ac-2--behavioral
kind: obligation
title: "Charge API retried on stale decline"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/loan-refi" }
frozen: { at: 2026-07-13, commit: 3e91ab2 }
---
# Charge API retried on stale decline

A Playwright e2e test drives a stale-decline scenario end to end and
asserts the charge API is retried through the outbox exactly once.
`)
	fm, body, err := SplitFrontmatter(doc)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	obligation, err := DecodeObligation(fm)
	if err != nil {
		t.Fatalf("DecodeObligation: %v", err)
	}
	if obligation.ID != "obligation/loan-refi--ac-2--behavioral" {
		t.Errorf("ID = %q", obligation.ID)
	}
	if obligation.Title != "Charge API retried on stale decline" {
		t.Errorf("Title = %q", obligation.Title)
	}
	if !strings.Contains(string(body), "Playwright e2e test") {
		t.Errorf("body prose not preserved: %q", body)
	}
}

func TestDecodeObligation_Negative(t *testing.T) {
	cases := map[string]string{
		"malformed id: only two segments": "id: obligation/loan-refi--ac-2\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"malformed id: single segment": "id: obligation/loan-refi\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"malformed id: four segments": "id: obligation/loan-refi--ac-2--behavioral--extra\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"id/for_kind disagreement": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: static\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"unknown frontmatter field": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\nbogus_field: true\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"missing verifies (no links at all)": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"verifies wrong link type": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: implements, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"more than one verifies link": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"missing frozen": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n",

		"for_kind not a known evidence kind": "id: obligation/loan-refi--ac-2--bogus\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: bogus\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		// The canonical verifies form is a WHOLE story spec (the AC is named
		// by the id, mirroring an attestation). A verifies edge carrying an
		// object fragment is rejected by the base link vocabulary (02 §Link
		// taxonomy: verifies is not one of the five fragment-eligible edge
		// types), so it never decodes.
		"verifies ref carries a fragment": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeObligation([]byte(y)); err == nil {
				t.Fatalf("DecodeObligation(%s): want error, got nil\n---\n%s", name, y)
			}
		})
	}
}

func TestSplitObligationName(t *testing.T) {
	story, ac, forKind, ok := SplitObligationName("loan-refi--ac-2--behavioral")
	if !ok {
		t.Fatal("SplitObligationName: ok = false, want true")
	}
	if story != "loan-refi" || ac != "ac-2" || forKind != "behavioral" {
		t.Fatalf("SplitObligationName = (%q, %q, %q), want (loan-refi, ac-2, behavioral)", story, ac, forKind)
	}
}

func TestSplitObligationName_Negative(t *testing.T) {
	cases := []string{
		"loan-refi",
		"loan-refi--ac-2",
		"loan-refi--ac-2--behavioral--extra",
		"",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, _, ok := SplitObligationName(name); ok {
				t.Fatalf("SplitObligationName(%q): ok = true, want false", name)
			}
		})
	}
}
