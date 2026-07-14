package workbench

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
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

// TestBuildProjection_StubStoredPositionWinsOverComputed proves round
// 5.5's dc-6 amendment at the projection layer: a layout.json "stub:<slug>"
// entry passes through to the stub card's rendered X/Y verbatim, winning
// over the zone's computed lane default the same stub would otherwise
// take (mirroring how a stored object position already wins).
func TestBuildProjection_StubStoredPositionWinsOverComputed(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)

	// The computed baseline: no stored positions at all.
	baseline, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection (baseline): %v", err)
	}
	var baseX, baseY float64
	for _, sv := range baseline.StubViews {
		if sv.Slug == "plain-one" {
			baseX, baseY = sv.X, sv.Y
		}
	}

	stored := map[string]artifact.Position{"stub:plain-one": {X: 990, Y: 444}}
	p, err := buildProjection("scoping-fixture", fm, nil, stored, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	var found bool
	for _, sv := range p.StubViews {
		if sv.Slug != "plain-one" {
			continue
		}
		found = true
		if sv.X != 990 || sv.Y != 444 {
			t.Errorf("StubViews[plain-one] = (%v,%v), want stored verbatim (990,444)", sv.X, sv.Y)
		}
	}
	if !found {
		t.Fatal("stub plain-one missing from StubViews")
	}
	if baseX == 990 && baseY == 444 {
		t.Fatal("test fixture's computed default coincidentally matches the stored spot; pick a different probe position")
	}

	// Reload-determinism: rebuilding the projection from the same four
	// inputs reproduces the identical stored position.
	again, err := buildProjection("scoping-fixture", fm, nil, stored, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection (again): %v", err)
	}
	for _, sv := range again.StubViews {
		if sv.Slug == "plain-one" && (sv.X != 990 || sv.Y != 444) {
			t.Errorf("reload produced (%v,%v), want the same stored (990,444)", sv.X, sv.Y)
		}
	}
}

// TestBuildProjection_StubStoredPositionCollidesWithObject proves a
// stored stub position participates in R4-I-35 display-time collision
// resolution against an object card's stored position, using the stub's
// own footprint (StubCardHeight) — never rendering stacked.
func TestBuildProjection_StubStoredPositionCollidesWithObject(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	stored := map[string]artifact.Position{
		"ac-1":           {X: 40, Y: 20},
		"stub:plain-one": {X: 40, Y: 20}, // squarely on ac-1's stored spot
	}
	p, err := buildProjection("scoping-fixture", fm, nil, stored, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	var ac1X, ac1Y float64
	for _, c := range p.Cards {
		if c.ID == "ac-1" {
			ac1X, ac1Y = c.X, c.Y
		}
	}
	if ac1X != 40 || ac1Y != 20 {
		t.Fatalf("ac-1 (earlier zone, first claimant) = (%v,%v), want stored verbatim (40,20)", ac1X, ac1Y)
	}
	acRect := boardlayout.Rect{X: ac1X, Y: ac1Y, W: boardlayout.CardWidth, H: boardlayout.CardHeight}
	for _, sv := range p.StubViews {
		if sv.Slug != "plain-one" {
			continue
		}
		w, h := boardlayout.FootprintFor(boardlayout.ZoneStub)
		stubRect := boardlayout.Rect{X: sv.X, Y: sv.Y, W: w, H: h}
		if stubRect.X < acRect.X+acRect.W && acRect.X < stubRect.X+stubRect.W &&
			stubRect.Y < acRect.Y+acRect.H && acRect.Y < stubRect.Y+stubRect.H {
			t.Errorf("stub plain-one at (%v,%v) still renders overlapping ac-1's footprint", sv.X, sv.Y)
		}
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, annotations, nil, modeReadOnly)
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

// TestBuildProjection_ScopingEdges proves the scoping layer's projection
// (owner directive, fixing the flagged ac-3 deviation — "with their
// coverage yarn projected"): every declared story stub hangs one edge per
// AC it covers (displayed type "covers") and every spike stub one edge
// per open question it resolves (displayed type "resolves"), all under
// layer "scoping" with the stub's own "stub:<slug>" key as the From
// endpoint. Presentation-owned derivation of the stubs block — NOT
// document links: no reference card is ever minted for a stub endpoint,
// and the closed five-type edge vocabulary is untouched.
func TestBuildProjection_ScopingEdges(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}

	var scoping []edgeView
	for _, e := range p.Edges {
		if e.Layer == "scoping" {
			scoping = append(scoping, e)
		}
	}
	want := []edgeView{
		{Type: "covers", From: "stub:plain-one", To: "ac-1", Layer: "scoping"},
		{Type: "resolves", From: "stub:spike-one", To: "oq-1", Layer: "scoping"},
		{Type: "resolves", From: "stub:spike-two", To: "oq-1", Layer: "scoping"},
	}
	if len(scoping) != len(want) {
		t.Fatalf("scoping edges = %+v, want %+v", scoping, want)
	}
	for i, e := range scoping {
		if e != want[i] {
			t.Errorf("scoping edge[%d] = %+v, want %+v (declaration order)", i, e, want[i])
		}
	}

	// A stub endpoint is the stub card's own paper — never a reference
	// card (this fixture declares no external refs at all).
	if len(p.RefCards) != 0 {
		t.Errorf("scoping edges minted reference cards: %+v", p.RefCards)
	}

	// Deterministic: same inputs, same edges in the same order.
	again, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection (again): %v", err)
	}
	for i, e := range again.Edges {
		if e != p.Edges[i] {
			t.Fatalf("edge order not deterministic at %d: %+v vs %+v", i, e, p.Edges[i])
		}
	}
}

// TestBuildProjection_ScopingEdgeUndeclaredTargetDropped proves a stub
// attribution naming an AC/OQ id the spec does not declare projects no
// scoping edge (and mints nothing): the yarn ties two papers on THIS
// wall, and a dangling attribution is the linter's finding (VL-006
// family), not a phantom endpoint. The coverage receipts stay the honest
// counters they were.
func TestBuildProjection_ScopingEdgeUndeclaredTargetDropped(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)
	fm.Stubs = append(fm.Stubs,
		artifact.Stub{Slug: "dangling-story", AcceptanceCriteria: []string{"ac-99"}},
		artifact.Stub{Slug: "dangling-spike", Spike: true, Resolves: []string{"oq-99"}},
	)
	p, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	for _, e := range p.Edges {
		if e.Layer != "scoping" {
			continue
		}
		if e.To == "ac-99" || e.To == "oq-99" {
			t.Errorf("undeclared attribution target projected an edge: %+v", e)
		}
	}
	for _, rc := range p.RefCards {
		if rc.Ref == "ac-99" || rc.Ref == "oq-99" {
			t.Errorf("undeclared attribution target minted a reference card: %+v", rc)
		}
	}
	// The dangling stubs still render as cards — the band shows the
	// declared record; only the unresolvable yarn is withheld.
	if len(p.StubViews) != 5 {
		t.Errorf("len(StubViews) = %d, want 5", len(p.StubViews))
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
	p, err := buildProjection("scoping-fixture", fm, nil, nil, annotations, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	for _, e := range p.Edges {
		if e.Layer == "annotation" {
			t.Fatalf("a thread naming a non-live sticky id projected an edge: %+v", e)
		}
	}
}
