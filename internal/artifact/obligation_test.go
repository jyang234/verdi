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
		"  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
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
	if len(fm.Links) != 1 || fm.Links[0].Type != LinkVerifies || fm.Links[0].Ref != "spec/loan-refi#ac-2" {
		t.Fatalf("Links = %+v", fm.Links)
	}
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
  - { type: verifies, ref: "spec/loan-refi#ac-2" }
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
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"malformed id: single segment": "id: obligation/loan-refi\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"malformed id: four segments": "id: obligation/loan-refi--ac-2--behavioral--extra\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"id/for_kind disagreement": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: static\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"unknown frontmatter field": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\nbogus_field: true\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"missing verifies (no links at all)": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"verifies wrong link type": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: implements, ref: \"spec/loan-refi#ac-2\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"more than one verifies link": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n  - { type: verifies, ref: \"spec/loan-refi#ac-3\" }\n" +
			"frozen: { at: 2026-07-13, commit: 3e91ab2 }\n",

		"missing frozen": "id: obligation/loan-refi--ac-2--behavioral\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: behavioral\n" +
			"links:\n  - { type: verifies, ref: \"spec/loan-refi#ac-2\" }\n",

		"for_kind not a known evidence kind": "id: obligation/loan-refi--ac-2--bogus\n" +
			"kind: obligation\ntitle: Foo\nowners: [x]\nfor_kind: bogus\n" +
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
