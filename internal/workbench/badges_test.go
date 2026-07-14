package workbench

// Tests for spec/badge-computes: the wall badge compute layer's server-
// side attachment (ac-1), the VL-finding partition (ac-2), and the
// derivation record's data model reaching every surface. Chip/stamp
// MARKUP (dc-4's visual grammar) is the frontend phase's own work on this
// same branch — these tests exercise the DATA this phase delivers:
// BoardProjection.Cards[].Badges / .CaseFileBadges, populated identically
// regardless of which entry point (the page, the fragment, or
// LoadProjection) triggers loadBoard.
//
// Two separate fixtures are needed because `stubs:` (a feature-only
// field, artifact's own validateStory rejects it on a story spec) and the
// spec-stale/pending-supersession ladder (a STORY-only compute,
// mirroring internal/dex/lens.go's isStoryPage gate) can never coexist on
// one spec: badgeFeatureFixtureSpec exercises the VL-finding partition
// (ac-2); badgeStoryFixtureSpec exercises the ladder's disclosed-unproven
// outcome (ac-3) reaching the same Notices surface every board mode
// already renders.

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const badgeFeatureFixtureName = "widget-badge-feature"

// badgeFeatureFixtureSpec declares a feature whose stub "orphan-stub"
// names a nonexistent acceptance criterion (VL-006's checkStubACs: a
// "dangling stub ref", badge-computes ac-2's first, object-anchored
// bucket — chosen over the per-AC "declares no evidence kind" shape
// because THAT violation fails artifact.DecodeSpec's own
// AcceptanceCriterion.Validate outright, so loadBoard 500s before any
// badge could ever compute; a dangling stub reference, by contrast,
// decodes and validates cleanly — Stub.Validate only checks id SHAPE,
// never existence — so it reaches a live, renderable board exactly like
// a real authoring mistake would. internal/wallbadge's own compute-layer
// tests (TestComputeBadges_EndToEnd) exercise the no-evidence-kind shape
// directly, bypassing this decode boundary).
const badgeFeatureFixtureSpec = `---
id: spec/widget-badge-feature
kind: spec
class: feature
title: "Widget badge feature"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [runtime], anchor: "#ac-1" }
  - { id: ac-2, text: "does another thing", evidence: [runtime], anchor: "#ac-2" }
stubs:
  - { slug: orphan-stub, acceptance_criteria: [ac-99] }
---
# Widget badge feature

## Problem

p

## Outcome

o

## ac-1

Prose.

## ac-2

Prose.
`

const badgeFeatureFixtureLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": {
    "ac-1": { "x": 40, "y": 60 },
    "stub:dangling-not-a-real-stub": { "x": 500, "y": 500 }
  }
}
`

// newBadgeFeatureFixture builds a fresh fixture repo (real git, matching
// newBoardFixture's own pattern) with badgeFeatureFixtureSpec plus a
// layout.json carrying a DANGLING key (VL-018) — badge-computes ac-2's
// third bucket, fail-closed even though it lives inside this spec's own
// directory.
func newBadgeFeatureFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + badgeFeatureFixtureName + "/spec.md":     badgeFeatureFixtureSpec,
			".verdi/specs/active/" + badgeFeatureFixtureName + "/layout.json": badgeFeatureFixtureLayout,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed badge feature fixture",
	}})
	return repo.Dir
}

const badgeStoryFixtureName = "widget-badge-story"

// badgeStoryFixtureSpec declares a story whose sole implements edge,
// with no forge configured (Deps{} zero value), drives the pending-
// supersession ladder into its disclosed-unproven outcome (ac-3) — never
// a badge, never silence.
const badgeStoryFixtureSpec = `---
id: spec/widget-badge-story
kind: spec
class: story
title: "Widget badge story"
status: draft
owners: [platform-team]
story: jira:WBT-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [attestation], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/some-parent-feature#ac-1" }
---
# Widget badge story

## Problem

p

## Outcome

o

## ac-1

Prose.
`

// newBadgeStoryFixture builds a fresh fixture repo for badgeStoryFixtureSpec.
func newBadgeStoryFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + badgeStoryFixtureName + "/spec.md": badgeStoryFixtureSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed badge story fixture",
	}})
	return repo.Dir
}

func badgeCardByID(t *testing.T, proj *BoardProjection, id string) cardView {
	t.Helper()
	for _, c := range proj.Cards {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no card %q in projection", id)
	return cardView{}
}

func badgeStubBySlug(proj *BoardProjection, slug string) *StubView {
	for i := range proj.StubViews {
		if proj.StubViews[i].Slug == slug {
			return &proj.StubViews[i]
		}
	}
	return nil
}

// TestAttachBadges_VLPartition is badge-computes ac-2, driven through the
// real HTTP-facing load path (loadBoard): an object-anchored VL-006
// finding (a stub's dangling acceptance_criteria reference) badges
// exactly that STUB's own card; a card naming no locus-bearing finding
// stays bare; and VL-018's dangling layout key (its Path lies INSIDE this
// spec's own directory) badges nothing anywhere — fail-closed, exactly
// the third-bucket case the obligation names.
func TestAttachBadges_VLPartition(t *testing.T) {
	root := newBadgeFeatureFixture(t)
	s := &boardSpecServer{root: root}
	proj, _, _, err := s.loadBoard(context.Background(), badgeFeatureFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}

	stub := badgeStubBySlug(proj, "orphan-stub")
	if stub == nil {
		t.Fatalf("no stub view %q in projection: %+v", "orphan-stub", proj.StubViews)
	}
	if len(stub.Badges) != 1 {
		t.Fatalf("stub orphan-stub.Badges = %+v, want exactly one VL-006 badge", stub.Badges)
	}
	if stub.Badges[0].Source != "lint:VL-006" {
		t.Errorf("stub.Badges[0].Source = %q, want lint:VL-006", stub.Badges[0].Source)
	}
	if stub.Badges[0].Target != "stub:orphan-stub" {
		t.Errorf("stub.Badges[0].Target = %q, want stub:orphan-stub", stub.Badges[0].Target)
	}
	if len(stub.Badges[0].Inputs) != 1 || stub.Badges[0].Inputs[0].Revision == "" {
		t.Errorf("stub.Badges[0].Inputs = %+v, want one input with a non-empty revision", stub.Badges[0].Inputs)
	}

	ac1 := badgeCardByID(t, proj, "ac-1")
	if len(ac1.Badges) != 0 {
		t.Errorf("ac-1.Badges = %+v, want none (ac-1 names no locus-bearing finding)", ac1.Badges)
	}
	ac2 := badgeCardByID(t, proj, "ac-2")
	if len(ac2.Badges) != 0 {
		t.Errorf("ac-2.Badges = %+v, want none", ac2.Badges)
	}

	// VL-018's dangling "stub:dangling-not-a-real-stub" layout key fires
	// in `verdi lint` (proven separately by internal/lint's own VL-018
	// tests) but declares no wall locus at all — it must not badge ANY
	// card, any stub view, or the case file, even though its Path is this
	// spec's own layout.json, sitting in this spec's own directory.
	for _, c := range proj.Cards {
		for _, b := range c.Badges {
			if b.Source == "lint:VL-018" {
				t.Fatalf("card %s carries a VL-018 badge %+v — VL-018 must declare no locus (fail-closed)", c.ID, b)
			}
		}
	}
	for _, sv := range proj.StubViews {
		for _, b := range sv.Badges {
			if b.Source == "lint:VL-018" {
				t.Fatalf("stub %s carries a VL-018 badge %+v — VL-018 must declare no locus (fail-closed)", sv.Slug, b)
			}
		}
	}
	for _, b := range proj.CaseFileBadges {
		if b.Source == "lint:VL-018" {
			t.Fatalf("case file carries a VL-018 badge %+v — VL-018 must declare no locus (fail-closed)", b)
		}
	}
}

// TestAttachBadges_PendingSupersessionDisclosedUnproven proves ac-3's
// disclosed-unproven outcome reaches the projection's Notices (the one
// existing generic disclosure surface every board mode already renders,
// spec/badge-computes co-2/ac-3): a story implementing a feature, with no
// forge configured (Deps{} zero value — this test's server carries no
// SupersessionCandidates loader), gets a disclosure, never a badge and
// never silence.
func TestAttachBadges_PendingSupersessionDisclosedUnproven(t *testing.T) {
	root := newBadgeStoryFixture(t)
	s := &boardSpecServer{root: root}
	proj, _, _, err := s.loadBoard(context.Background(), badgeStoryFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	found := false
	for _, n := range proj.Notices {
		if strings.Contains(n, "pending-supersession is disclosed-unproven") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Notices = %+v, want a pending-supersession disclosed-unproven notice", proj.Notices)
	}
	for _, b := range proj.CaseFileBadges {
		if b.Source == "ladder:pending-supersession" {
			t.Fatalf("CaseFileBadges = %+v, want no pending-supersession BADGE when unproven", proj.CaseFileBadges)
		}
	}
}

// TestAttachBadges_SameAcrossPageFragmentAndLoadProjection is badge-
// computes ac-1: the page, the post-mutation fragment, and get_board's
// LoadProjection all reach badge data through the ONE call site
// (loadBoard, boardspec.go) — proven here by driving all three real entry
// points against the same fixture and asserting on the firing compute's
// own identity (its disclosure text), not just its presence. HTML chip
// markup is the frontend phase's own work (dc-4); what this phase
// guarantees is that the identical BoardProjection value — Cards[].
// Badges, CaseFileBadges, Notices — is available to whichever surface
// renders it, and that the disclosure half (which the board's existing
// generic notice rendering ALREADY surfaces) is visibly identical right
// now on the page and the fragment.
func TestAttachBadges_SameAcrossPageFragmentAndLoadProjection(t *testing.T) {
	root := newBadgeStoryFixture(t)
	h := NewHandler(root)

	pageRec := getBoard(t, h, badgeStoryFixtureName)
	if pageRec.Code != 200 {
		t.Fatalf("GET board page = %d\n%s", pageRec.Code, pageRec.Body.String())
	}
	fragRec := httptest.NewRecorder()
	h.ServeHTTP(fragRec, httptest.NewRequest("GET", "/board/spec/"+badgeStoryFixtureName+"/fragment", nil))
	if fragRec.Code != 200 {
		t.Fatalf("GET board fragment = %d\n%s", fragRec.Code, fragRec.Body.String())
	}
	for _, body := range []string{pageRec.Body.String(), fragRec.Body.String()} {
		if !strings.Contains(body, "pending-supersession is disclosed-unproven") {
			t.Errorf("surface missing the pending-supersession disclosure notice:\n%s", body)
		}
	}

	direct, _, err := LoadProjection(context.Background(), root, badgeStoryFixtureName, nil, "", nil)
	if err != nil {
		t.Fatalf("LoadProjection: %v", err)
	}
	foundDisclosure := false
	for _, n := range direct.Notices {
		if strings.Contains(n, "pending-supersession is disclosed-unproven") {
			foundDisclosure = true
		}
	}
	if !foundDisclosure {
		t.Fatalf("LoadProjection's Notices = %+v, want the pending-supersession disclosure (same as the page/fragment)", direct.Notices)
	}
}

// TestAttachBadges_ErrorPropagates is the operational-negative path: a
// store root the badge compute layer cannot walk (no .verdi/ tree at all
// — internal/wallbadge.ComputeBadges' own lint.BuildSnapshot fails)
// propagates an error from attachBadges, never a silently badge-free
// projection.
func TestAttachBadges_ErrorPropagates(t *testing.T) {
	badRoot := filepath.Join(t.TempDir(), "does-not-exist")
	fm, err := artifact.DecodeSpec(mustFrontmatterBytes(t, badgeStoryFixtureSpec))
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if err := attachBadges(context.Background(), &BoardProjection{}, badRoot, badgeStoryFixtureName, []byte(badgeStoryFixtureSpec), fm, nil); err == nil {
		t.Fatal("attachBadges over an unwalkable root: got nil error, want one")
	}
}

func mustFrontmatterBytes(t *testing.T, specMD string) []byte {
	t.Helper()
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(specMD))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	return fmBytes
}
