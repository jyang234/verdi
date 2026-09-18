package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func fixtureSpec(t *testing.T) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	doc := []byte(`---
id: spec/lockbox
kind: spec
title: "Lockbox"
owners: [platform-team]
class: feature
links:
  - { type: supersedes, ref: "spec/lockbox-v0" }
problem: { text: "Keys are shared.", anchor: problem }
outcome: { text: "Each key has one holder.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "A key opens one box.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "A lost key is revoked.", evidence: [static], anchor: ac-2 }
constraints:
  - { id: co-1, text: "No network.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "One holder per key.", anchor: dc-1 }
open_questions:
  - { id: oq-1, text: "Who audits holders?", anchor: oq-1 }
  - { id: oq-2, text: "How long is a revocation valid?", anchor: oq-2 }
stubs:
  - { slug: key-holder, acceptance_criteria: [ac-1] }
  - { slug: audit-probe, spike: true, resolves: [oq-1] }
supersession:
  carried: [ac-1]
  amended: [ { id: ac-2, note: "tightened" } ]
  amended_advisory: []
  removed: []
  added: [co-1, dc-1, oq-1, oq-2]
---
# Lockbox

## Problem

Keys are shared today.

## Outcome

One holder.

## ac-1

Proven by opening.

## ac-2

## co-1

## dc-1

Because two holders means no holder.

## oq-1

## oq-2
`)
	fmBytes, body, err := artifact.SplitFrontmatter(doc)
	if err != nil {
		t.Fatal(err)
	}
	_ = fmBytes
	fm, err := artifact.DecodeSpec(doc)
	if err != nil {
		t.Fatal(err)
	}
	return fm, body
}

func TestBuildSpecKind(t *testing.T) {
	fm, body := fixtureSpec(t)
	in := Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("a", 40)}, Facts: FactsFromSpec(fm), Kind: KindSpec}
	doc, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Lockbox" || doc.Kind != KindSpec {
		t.Fatalf("title/kind = %q/%q", doc.Title, doc.Kind)
	}
	if doc.Stamp.Engine != EngineDigest() {
		t.Fatalf("Build must fill Stamp.Engine")
	}
	if !doc.ProblemKnown || doc.ProblemText != "Keys are shared." || !doc.OutcomeKnown {
		t.Fatalf("problem/outcome = %+v", doc)
	}
	if len(doc.Decisions) != 1 || doc.Decisions[0].Detail != "Because two holders means no holder." {
		t.Fatalf("decisions = %+v", doc.Decisions)
	}
	if len(doc.Criteria) != 2 || !doc.Criteria[0].CoverageKnown || len(doc.Criteria[0].Coverage) != 1 || doc.Criteria[0].Coverage[0] != "key-holder" || doc.Criteria[0].Detail != "Proven by opening." {
		t.Fatalf("criteria = %+v", doc.Criteria)
	}
	if len(doc.Criteria[1].Coverage) != 0 || !doc.Criteria[1].CoverageKnown {
		t.Fatalf("ac-2 must be known-uncovered, got %+v", doc.Criteria[1])
	}
	if len(doc.Questions) != 2 || doc.Questions[0].Claims[0] != "audit-probe" || len(doc.Questions[1].Claims) != 0 {
		t.Fatalf("questions = %+v", doc.Questions)
	}
	if len(doc.Plan) != 2 || doc.Plan[0].Slug != "key-holder" || !doc.Plan[1].Spike {
		t.Fatalf("plan = %+v", doc.Plan)
	}
	if doc.EvidenceKnown {
		t.Fatalf("evidence must be unknown when facts carry none")
	}
	var supersedes string
	for _, kv := range doc.Identity {
		if kv.Label == "Supersedes" {
			supersedes = kv.Value
		}
	}
	if supersedes != "spec/lockbox-v0" {
		t.Fatalf("identity rows = %+v", doc.Identity)
	}
	if doc.Words.Story == "" || doc.Words.Spike == "" || doc.Words.Feature == "" {
		t.Fatalf("words must resolve through a nil model to the bare ids: %+v", doc.Words)
	}
}

func TestBuildKindsSelectSections(t *testing.T) {
	fm, body := fixtureSpec(t)
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		doc, err := Build(Input{Spec: fm, Body: body, Kind: k, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("b", 40)}, Facts: FactsFromSpec(fm)})
		if err != nil {
			t.Fatal(err)
		}
		if len(doc.Sections) != len(k.Sections()) {
			t.Errorf("%s sections = %v", k, doc.Sections)
		}
	}
}

func TestBuildUnknownFactsAndEvidence(t *testing.T) {
	fm, body := fixtureSpec(t)
	doc, err := Build(Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("c", 40)}})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Criteria[0].CoverageKnown || doc.Questions[0].ClaimsKnown {
		t.Fatalf("nil facts must render as unknown coverage/claims: %+v", doc.Criteria[0])
	}
	withEvidence := Facts{Evidence: map[string]ACEvidence{"ac-2": {Status: "violated", Summary: "nothing implements it"}}, EvidenceSource: "matrix at c"}
	doc, err = Build(Input{Spec: fm, Body: body, Kind: KindTasks, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("c", 40)}, Facts: withEvidence})
	if err != nil {
		t.Fatal(err)
	}
	if !doc.EvidenceKnown || len(doc.Evidence) != 2 || doc.Evidence[0].ID != "ac-1" || doc.Evidence[0].Status != "" || doc.Evidence[1].Status != "violated" {
		t.Fatalf("evidence rows must follow criteria order and carry blanks for absent ids: %+v", doc.Evidence)
	}
}

func TestBuildRejectsBadInput(t *testing.T) {
	fm, body := fixtureSpec(t)
	cases := []struct {
		name string
		in   Input
	}{
		{"nil spec", Input{Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/x", Commit: strings.Repeat("a", 40)}}},
		{"bad kind", Input{Spec: fm, Body: body, Kind: "chapter", Stamp: Stamp{Ref: "spec/x", Commit: strings.Repeat("a", 40)}}},
		{"empty ref", Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Commit: strings.Repeat("a", 40)}}},
		{"short commit", Input{Spec: fm, Body: body, Kind: KindSpec, Stamp: Stamp{Ref: "spec/x", Commit: "abc"}}},
	}
	for _, c := range cases {
		if _, err := Build(c.in); err == nil {
			t.Errorf("%s: want error", c.name)
		}
	}
}
