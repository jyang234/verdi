package workbench

// Tests for R-RR2-7 (readiness-recovery wave 2, SI-212): the obligation
// row attaches to EVERY class's acceptance-criterion cards, so a feature
// AC card wears one obligationView per declared evidence kind — Present
// reflecting the obligation document's real presence on disk exactly as
// for stories — and the evidence-slot chip has a row to land on. The
// story wall's rendered bytes are pinned unchanged (the pins below were
// captured at BASE e9ae7a0f, before the class guard was removed).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

const obligationRowFeatureName = "widget-row-feature"

// obligationRowFeatureSpec is a draft FEATURE whose ac-1 declares two
// kinds (static, attestation) and ac-2 one (attestation) — the shape the
// harness design wall (refi-decline-flow) has: every criterion declares
// attestation, none has an attestation or obligation on disk.
const obligationRowFeatureSpec = `---
id: spec/widget-row-feature
kind: spec
class: feature
title: "Widget row feature"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "does another thing", evidence: [attestation], anchor: "#ac-2" }
---
# Widget row feature

## Problem

p

## Outcome

o

## ac-1

Prose.

## ac-2

Prose.
`

// obligationRowFeatureObligation is an on-disk obligation for the FEATURE's
// ac-1 static kind: artifact.DecodeObligation checks the id's own
// <slug>--<ac>--<kind> shape and one fragment-less verifies edge, never
// the target's class, and evidence.Obligations keys the directory by the
// wall's own spec name — so a feature kind's obligation decodes exactly
// as a story's does.
const obligationRowFeatureObligation = `---
id: obligation/widget-row-feature--ac-1--static
kind: obligation
title: "a linter proves the widget's config is schema-valid"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/widget-row-feature" }
frozen: { at: 2026-07-13, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# a linter proves the widget's config is schema-valid

Run the schema check over every widget config.
`

func newObligationRowFeatureFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + obligationRowFeatureName + "/spec.md": obligationRowFeatureSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed obligation row feature fixture",
	}})
}

// TestObligationRow_FeatureWallCarriesSlotChips drives the whole board
// path a browser hits (loadBoard: buildProjection, attachObligations,
// attachBadges) over a never-synced FEATURE wall: each AC card carries
// one obligationView per declared kind, Present=false (no obligation on
// disk), Slot "empty" after the badges join — and the rendered card
// holds the obligation row with the attestation kind's own empty chip.
func TestObligationRow_FeatureWallCarriesSlotChips(t *testing.T) {
	repo := newObligationRowFeatureFixture(t)
	s := &boardSpecServer{root: repo.Dir}
	proj, _, _, _, err := s.loadBoard(context.Background(), obligationRowFeatureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	if proj.Class != string(artifact.ClassFeature) {
		t.Fatalf("projection class = %q, want feature", proj.Class)
	}

	ac1 := badgeCardByID(t, proj, "ac-1")
	if len(ac1.Obligations) != 2 {
		t.Fatalf("ac-1 obligations = %+v, want 2 (one per declared kind)", ac1.Obligations)
	}
	for i, kind := range []string{"static", "attestation"} {
		o := ac1.Obligations[i]
		if o.Kind != kind || o.Present || o.Slot != "empty" || o.SlotRecords != 0 {
			t.Errorf("ac-1[%d] = %+v, want kind %s, absent, Slot \"empty\"", i, o, kind)
		}
	}
	ac2 := badgeCardByID(t, proj, "ac-2")
	if len(ac2.Obligations) != 1 || ac2.Obligations[0].Kind != "attestation" || ac2.Obligations[0].Present || ac2.Obligations[0].Slot != "empty" {
		t.Errorf("ac-2 obligations = %+v, want exactly [attestation, absent, Slot \"empty\"]", ac2.Obligations)
	}

	body := renderBoardRegion(proj, &boardGitState{}, testASDView())
	for _, want := range []string{
		`data-testid="obligations-ac-1"`,
		`data-testid="obligation-none-ac-1-static">no obligation</span><span class="slot-chip slot-chip--empty" data-testid="slot-ac-1-static" data-slot-state="empty">no record</span>`,
		`data-testid="obligation-none-ac-1-attestation">no obligation</span><span class="slot-chip slot-chip--empty" data-testid="slot-ac-1-attestation" data-slot-state="empty">no attestation</span>`,
		`data-testid="obligations-ac-2"`,
		`data-testid="slot-ac-2-attestation" data-slot-state="empty">no attestation</span>`,
		// The feature card keeps its coverage receipt beside the row.
		`data-testid="coverage-ac-1"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("feature board region missing %q", want)
		}
	}
	// One per-kind list per card, exactly one row per declared kind.
	if got := strings.Count(body, `data-testid="obligations-ac-1"`); got != 1 {
		t.Errorf("obligations-ac-1 appears %d times, want 1", got)
	}
	if got := strings.Count(body, `data-obligation-kind=`); got != 3 {
		t.Errorf("per-kind rows = %d, want 3 (2 on ac-1, 1 on ac-2)", got)
	}
}

// TestObligationRow_FeatureACWithoutDeclaredKinds pins the unchanged
// guard: a feature AC declaring no evidence kinds gets no views and no
// row (never an empty obligations block).
func TestObligationRow_FeatureACWithoutDeclaredKinds(t *testing.T) {
	fm := &artifact.SpecFrontmatter{
		Class: artifact.ClassFeature,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "declares no evidence kind"},
		},
	}
	proj, err := buildProjectionFM("f", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, t.TempDir(), "f", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	if got := obligationCard(t, proj, "ac-1").Obligations; got != nil {
		t.Errorf("evidence-less feature AC carries obligations %+v, want none", got)
	}
	body := renderBoardRegion(proj, &boardGitState{}, testASDView())
	if strings.Contains(body, `data-testid="obligations-ac-1"`) {
		t.Error("evidence-less feature AC rendered an obligations block")
	}
}

// TestObligationRow_FeatureObligationOnDiskReadsPresent proves Present
// reflects the store for a feature exactly as for a story: an obligation
// document at .verdi/obligations/<feature-name>/ac-1--static.md reads
// Present=true with its title and prose, while the sibling attestation
// kind stays the disclosed absence.
func TestObligationRow_FeatureObligationOnDiskReadsPresent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "obligations", obligationRowFeatureName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir obligations: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ac-1--static.md"), []byte(obligationRowFeatureObligation), 0o644); err != nil {
		t.Fatalf("write obligation: %v", err)
	}
	fm := mustDecodeSpecForTest(t, obligationRowFeatureSpec)
	proj, err := buildProjectionFM(obligationRowFeatureName, fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, root, obligationRowFeatureName, fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	ac1 := obligationCard(t, proj, "ac-1")
	if len(ac1.Obligations) != 2 {
		t.Fatalf("ac-1 obligations = %+v, want 2", ac1.Obligations)
	}
	st := ac1.Obligations[0]
	if st.Kind != "static" || !st.Present || st.Title != "a linter proves the widget's config is schema-valid" || !strings.Contains(st.Body, "Run the schema check") {
		t.Errorf("ac-1[0] = %+v, want the authored static obligation present", st)
	}
	if at := ac1.Obligations[1]; at.Kind != "attestation" || at.Present {
		t.Errorf("ac-1[1] = %+v, want attestation absent", at)
	}
	body := renderBoardRegion(proj, &boardGitState{}, testASDView())
	for _, want := range []string{
		`data-obligation-kind="static" data-obligation-present="true"`,
		`>a linter proves the widget&#39;s config is schema-valid</span>`,
		`data-testid="obligation-none-ac-1-attestation">no obligation</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("feature obligation render missing %q", want)
		}
	}
}

// Story-wall render digests captured at BASE e9ae7a0f (the commit before
// attachObligations lost its story-only guard). Both walls take the exact
// paths a story wall took then; a changed digest means the story render
// moved, which R-RR2-7 forbids.
const (
	storyWallDigestBase     = "9a1e8b19538c39a43262612b4c5ac047785ad28f0bebd7108657a68bb10dc424"
	slotStoryWallDigestBase = "067adca9779a68c6710c04c9dac5b008a29432b134ef589061b9c8752216799e"
)

func renderDigest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// TestObligationRow_StoryWallBytesUnchanged pins the story wall: the
// obligation-wall story fixture (attachObligations only) and the slot
// story wall (the whole loadBoard path: obligations + badges + slots)
// render byte-identically to BASE. Each is rendered twice to prove the
// digest is deterministic before it is compared.
func TestObligationRow_StoryWallBytesUnchanged(t *testing.T) {
	root := obligationStoreWithBehavioral(t)
	fm := mustDecodeSpecForTest(t, obligationWallStorySpec)
	proj, err := buildProjectionFM("refi-decline-replay", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, root, "refi-decline-replay", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	first := renderDigest(renderBoardRegion(proj, &boardGitState{}, testASDView()))
	second := renderDigest(renderBoardRegion(proj, &boardGitState{}, testASDView()))
	if first != second {
		t.Fatalf("story wall render is not deterministic: %s vs %s", first, second)
	}
	if first != storyWallDigestBase {
		t.Errorf("story wall render digest = %s, want BASE %s (story bytes must not change)", first, storyWallDigestBase)
	}

	repo := newSlotWallFixture(t)
	s := &boardSpecServer{root: repo.Dir}
	slotProj, _, _, _, err := s.loadBoard(context.Background(), slotWallSpecName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	// The story-only ladder disclosure names the fixture's absolute root
	// (a fresh temp dir per process); it is the one legitimately varying
	// substring, normalized away so the pin covers every other byte.
	normalize := func(body string) string { return strings.ReplaceAll(body, repo.Dir, "<root>") }
	slotFirst := renderDigest(normalize(renderBoardRegion(slotProj, &boardGitState{}, testASDView())))
	slotSecond := renderDigest(normalize(renderBoardRegion(slotProj, &boardGitState{}, testASDView())))
	if slotFirst != slotSecond {
		t.Fatalf("slot story wall render is not deterministic: %s vs %s", slotFirst, slotSecond)
	}
	if slotFirst != slotStoryWallDigestBase {
		t.Errorf("slot story wall render digest = %s, want BASE %s (story bytes must not change)", slotFirst, slotStoryWallDigestBase)
	}
}
