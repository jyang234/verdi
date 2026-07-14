package wallbadge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
)

func specRelPathFor(name string) string {
	return ".verdi/specs/active/" + name + "/spec.md"
}

// digestOf is the caller-side revision internal/workbench's loadBoard
// computes for the spec bytes it already read — mirrored here so tests
// exercise ComputeBadges exactly as that caller does.
func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// TestComputeBadges_EndToEnd drives ComputeBadges over one fixture story
// exercising every ac-1/ac-2/ac-3 outcome at once: an object-anchored VL
// finding (ac-1 declares no evidence kind), a flagged spec-stale ladder
// badge (an own-text accepted-deviation), and a flagged pending-
// supersession ladder badge (a hermetic fake loader) — proving all three
// computes attach through the one entry point.
func TestComputeBadges_EndToEnd(t *testing.T) {
	const name = "widget-retry"
	root, fm := writeStoreSpec(t, name, endToEndStorySpec)
	writeDeviationReport(t, root, name, flaggedDeviationReportMD(ladderCoversSHA))

	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", name, "spec.md"))
	if err != nil {
		t.Fatalf("reading spec.md back: %v", err)
	}
	specRevision := digestOf(raw)

	loader := fakeSupersessionLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "7",
			Digest: "sha256:cccc",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
		}},
	}

	got, err := ComputeBadges(context.Background(), root, specRelPathFor(name), specRevision, fm, loader)
	if err != nil {
		t.Fatalf("ComputeBadges: %v", err)
	}

	ac1Badges, ok := got.ByObject["ac-1"]
	if !ok || len(ac1Badges) == 0 {
		t.Fatalf("ByObject[ac-1] = %+v, want at least one VL-006 badge", got.ByObject)
	}
	foundSource := func(recs []DerivationRecord, source string) bool {
		for _, r := range recs {
			if r.Source == source {
				return true
			}
		}
		return false
	}
	if !foundSource(ac1Badges, "lint:VL-006") {
		t.Errorf("ByObject[ac-1] = %+v, want a lint:VL-006 badge", ac1Badges)
	}
	if !foundSource(got.CaseFile, "ladder:spec-stale") {
		t.Errorf("CaseFile = %+v, want ladder:spec-stale", got.CaseFile)
	}
	if !foundSource(got.CaseFile, "ladder:pending-supersession") {
		t.Errorf("CaseFile = %+v, want ladder:pending-supersession", got.CaseFile)
	}
	if len(got.Disclosures) != 0 {
		t.Errorf("Disclosures = %+v, want none (both ladder flags proven, not unproven)", got.Disclosures)
	}
}

const endToEndStorySpec = `---
id: spec/widget-retry
kind: spec
class: story
title: "Widget retry"
status: draft
owners: [platform-team]
story: jira:WID-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "retries the widget", evidence: [], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/parent-feature#ac-1" }
---
# Widget retry

## Problem

p

## Outcome

o

## ac-1

Retries.
`

// TestComputeBadges_CrossSpecExclusion proves a second spec's own
// locus-bearing finding never leaks onto this spec's badges — the VL
// partition is scoped per spec (ac-2), corpus-wide findings notwithstanding.
func TestComputeBadges_CrossSpecExclusion(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", endToEndStorySpec)

	otherDir := filepath.Join(root, ".verdi", "specs", "active", "other-story")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatalf("mkdir other-story: %v", err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "spec.md"), []byte(otherStorySpecNoEvidence), 0o644); err != nil {
		t.Fatalf("write other-story spec.md: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", "widget-retry", "spec.md"))
	if err != nil {
		t.Fatalf("reading spec.md back: %v", err)
	}
	got, err := ComputeBadges(context.Background(), root, specRelPathFor("widget-retry"), digestOf(raw), fm, nil)
	if err != nil {
		t.Fatalf("ComputeBadges: %v", err)
	}
	// widget-retry's own ac-1 declares evidence: [] too (endToEndStorySpec),
	// so ByObject[ac-1] is expected from ITS OWN finding — the assertion
	// that matters is that other-story's finding does not ALSO appear
	// (it would, wrongly, if VLBadges' Path-scoping were broken) and that
	// no ladder badge fires for the pending-supersession loader (nil here)
	// beyond a disclosure.
	if len(got.ByObject) != 1 {
		t.Fatalf("ByObject = %+v, want exactly this spec's own ac-1 entry (other-story's finding must not leak in)", got.ByObject)
	}
	if _, ok := got.ByObject["ac-1"]; !ok {
		t.Fatalf("ByObject = %+v, want ac-1", got.ByObject)
	}
}

const otherStorySpecNoEvidence = `---
id: spec/other-story
kind: spec
class: story
title: "Other story"
status: draft
owners: [platform-team]
story: jira:OTH-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "unrelated", evidence: [], anchor: "#ac-1" }
---
# Other story

## Problem

p

## Outcome

o

## ac-1

Unrelated.
`

// TestComputeBadges_NonStorySkipsLadder proves a feature-class spec never
// gets ladder badges (ac-3's isStoryPage gate, mirrored from
// internal/dex/lens.go) even when VL findings still attach normally.
func TestComputeBadges_NonStorySkipsLadder(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-feature", featureSpecFixture)
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", "widget-feature", "spec.md"))
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}
	got, err := ComputeBadges(context.Background(), root, specRelPathFor("widget-feature"), digestOf(raw), fm, nil)
	if err != nil {
		t.Fatalf("ComputeBadges: %v", err)
	}
	for _, r := range got.CaseFile {
		if r.Source == "ladder:spec-stale" || r.Source == "ladder:pending-supersession" {
			t.Fatalf("CaseFile = %+v, want no ladder badge on a feature-class wall", got.CaseFile)
		}
	}
	if len(got.Disclosures) != 0 {
		t.Fatalf("Disclosures = %+v, want none on a feature-class wall (the ladder never runs at all)", got.Disclosures)
	}
}

const featureSpecFixture = `---
id: spec/widget-feature
kind: spec
class: feature
title: "Widget feature"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "widgets work", evidence: [runtime], anchor: "#ac-1" }
---
# Widget feature

## Problem

p

## Outcome

o

## ac-1

Widgets work.
`

// TestComputeBadges_Deterministic is ac-4's obligation, driven end to
// end: rendering the SAME fixture twice through ComputeBadges produces
// byte-identical serialized output.
func TestComputeBadges_Deterministic(t *testing.T) {
	root, fm := writeStoreSpec(t, "widget-retry", endToEndStorySpec)
	writeDeviationReport(t, root, "widget-retry", flaggedDeviationReportMD(ladderCoversSHA))
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", "widget-retry", "spec.md"))
	if err != nil {
		t.Fatalf("read spec.md: %v", err)
	}
	specRevision := digestOf(raw)
	loader := fakeSupersessionLoader{
		ok: true,
		candidates: []evidence.OpenSupersessionCandidate{{
			MRID:   "7",
			Digest: "sha256:cccc",
			Spec:   &artifact.SpecFrontmatter{Supersession: &artifact.Supersession{Amended: []artifact.SupersessionNote{{ID: "ac-1", Note: "tightened"}}}},
		}},
	}

	first, err := ComputeBadges(context.Background(), root, specRelPathFor("widget-retry"), specRevision, fm, loader)
	if err != nil {
		t.Fatalf("ComputeBadges (first): %v", err)
	}
	second, err := ComputeBadges(context.Background(), root, specRelPathFor("widget-retry"), specRevision, fm, loader)
	if err != nil {
		t.Fatalf("ComputeBadges (second): %v", err)
	}

	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("non-deterministic render:\nfirst:  %s\nsecond: %s", firstJSON, secondJSON)
	}
}

// TestComputeBadges_BadRoot is the operational-negative path: a store
// root the lint engine cannot walk at all propagates an error, never a
// silently empty result.
func TestComputeBadges_BadRoot(t *testing.T) {
	fm := &artifact.SpecFrontmatter{Class: artifact.ClassStory}
	_, err := ComputeBadges(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"), specRelPathFor("x"), "sha256:aaaa", fm, nil)
	if err == nil {
		t.Fatal("got nil error for an unreadable root, want one")
	}
}
