package workbench

import (
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// scopingProjectionFixtureSpec is a feature-class spec with two ACs, two
// open questions, one plain stub covering ac-1, and one spike stub
// resolving oq-1 twice over (oq-2 stays unclaimed) — spec/scoping-canvas
// ac-3/ac-4/ac-5: BoardProjection's additive StubViews/ACCoverage/OQClaims
// fields, pure functions of the frontmatter alone (co-2).
const scopingProjectionFixtureSpec = `---
id: spec/scoping-fixture
kind: spec
class: feature
title: "Scoping fixture"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "covered", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "uncovered", evidence: [attestation], anchor: "#ac-2" }
open_questions:
  - { id: oq-1, text: "claimed twice", anchor: "#oq-1" }
  - { id: oq-2, text: "unclaimed", anchor: "#oq-2" }
stubs:
  - { slug: plain-one, acceptance_criteria: [ac-1] }
  - { slug: spike-one, spike: true, resolves: [oq-1] }
  - { slug: spike-two, spike: true, resolves: [oq-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Scoping fixture

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## oq-1

Prose.

## oq-2

Prose.
`

func mustDecodeSpecForTest(t *testing.T, y string) *artifact.SpecFrontmatter {
	t.Helper()
	fm, body, err := artifact.SplitFrontmatter([]byte(y))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	_ = body
	return spec
}

// TestBuildProjection_StubViewsAndCoverage proves StubViews mirrors
// fm.Stubs verbatim, ACCoverage counts every non-spike stub claiming an
// AC ("covered by N stubs" / "no stub" is Coverage==0), and OQClaims
// counts every spike stub resolving an OQ (the multi-claim smell number).
func TestBuildProjection_StubViewsAndCoverage(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}

	if p.Status != "accepted-pending-build" {
		t.Fatalf("Status = %q, want accepted-pending-build", p.Status)
	}

	if len(p.StubViews) != 3 {
		t.Fatalf("len(StubViews) = %d, want 3", len(p.StubViews))
	}
	bySlug := make(map[string]StubView, len(p.StubViews))
	for _, sv := range p.StubViews {
		bySlug[sv.Slug] = sv
	}
	plain, ok := bySlug["plain-one"]
	if !ok || plain.Spike || len(plain.AcceptanceCriteria) != 1 || plain.AcceptanceCriteria[0] != "ac-1" {
		t.Fatalf("StubViews[plain-one] = %+v", plain)
	}
	spike, ok := bySlug["spike-one"]
	if !ok || !spike.Spike || len(spike.Resolves) != 1 || spike.Resolves[0] != "oq-1" {
		t.Fatalf("StubViews[spike-one] = %+v", spike)
	}

	if got := p.ACCoverage["ac-1"]; got != 1 {
		t.Errorf("ACCoverage[ac-1] = %d, want 1", got)
	}
	if got := p.ACCoverage["ac-2"]; got != 0 {
		t.Errorf("ACCoverage[ac-2] = %d, want 0 (no stub)", got)
	}
	if got := p.OQClaims["oq-1"]; got != 2 {
		t.Errorf("OQClaims[oq-1] = %d, want 2 (the multi-claim smell)", got)
	}
	if got := p.OQClaims["oq-2"]; got != 0 {
		t.Errorf("OQClaims[oq-2] = %d, want 0", got)
	}
}

// TestBuildProjection_RelatesEndpointNamesLiveSticky proves round 5.4's
// attribution-thread projection: a relates annotation whose target names
// a live story/spike sticky (by annotation id) on this board projects an
// edge with the sticky id as an endpoint (02 §Record schemas: "a relates
// endpoint may name a board annotation by id").
func TestBuildProjection_RelatesEndpointNamesLiveSticky(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	const storyStickyID = "a-01J8Z0K3AAAAAAAAAAAAAAAAAA"
	const threadID = "a-01J8Z0K4BBBBBBBBBBBBBBBBBB"
	annotations := []*artifact.Annotation{
		{
			ID: storyStickyID, TS: "2026-07-10T14:02:11Z", Author: "j",
			Type: artifact.AnnotationStory, Body: "borrower self-serve update", Status: artifact.AnnotationOpen,
			Board: &artifact.BoardAnchor{Story: "scoping-fixture", X: 10, Y: 20},
		},
		{
			ID: threadID, TS: "2026-07-10T14:03:00Z", Author: "j",
			Type: artifact.AnnotationRelates, Body: "relates: story ~ ac-1", Status: artifact.AnnotationOpen,
			Target:  &artifact.Target{Ref: storyStickyID},
			TargetB: &artifact.Target{Ref: "spec/scoping-fixture@7f3c2a1", Selector: artifact.Selector{Heading: "ac-1"}},
		},
	}
	p, err := buildProjection("scoping-fixture", fm, nil, annotations, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	var found bool
	for _, e := range p.Edges {
		if e.Layer == "annotation" && e.From == storyStickyID && e.To == "ac-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no annotation-layer edge from the sticky id to ac-1; edges = %+v", p.Edges)
	}
	if len(p.Stickies) != 1 || p.Stickies[0].ID != storyStickyID {
		t.Fatalf("Stickies = %+v, want the one story sticky", p.Stickies)
	}
}

// TestBuildProjection_RelatesEndpointNamesDeadSticky_Dropped proves a
// relates thread naming a sticky id that is NOT live on this board (never
// existed, or already graduated) is dropped rather than projected — the
// same "a thread of some other board/spec" honesty relatesEndpoint
// already gives a stale artifact-ref endpoint.
func TestBuildProjection_RelatesEndpointNamesDeadSticky_Dropped(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	const threadID = "a-01J8Z0K4BBBBBBBBBBBBBBBBBB"
	annotations := []*artifact.Annotation{
		{
			ID: threadID, TS: "2026-07-10T14:03:00Z", Author: "j",
			Type: artifact.AnnotationRelates, Body: "relates: ghost ~ ac-1", Status: artifact.AnnotationOpen,
			Target:  &artifact.Target{Ref: "a-01J8Z0K5CCCCCCCCCCCCCCCCCC"},
			TargetB: &artifact.Target{Ref: "spec/scoping-fixture@7f3c2a1", Selector: artifact.Selector{Heading: "ac-1"}},
		},
	}
	p, err := buildProjection("scoping-fixture", fm, nil, annotations, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	for _, e := range p.Edges {
		if e.Layer == "annotation" {
			t.Fatalf("a thread naming a non-live sticky id projected an edge: %+v", e)
		}
	}
}
