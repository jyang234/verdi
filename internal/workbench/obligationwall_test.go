package workbench

// Render + load tests for the obligation wall (spec/obligation-wall ac-2):
// a STORY AC card discloses, per declared evidence kind, that kind's
// obligation — the authored title for a kind that HAS one, a disclosed
// "no obligation" badge for one that does not (dc-2). The obligations are
// loaded from disk by the ONE reader both this surface and `verdi matrix`
// consume (evidence.Obligations, dc-1) and projected onto cardView, then
// rendered as compact receipts in the board's existing card vocabulary.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
)

// obligationWallStorySpec is a STORY spec whose ac-1 declares TWO evidence
// kinds (behavioral, static). The fixture obligation below authors one for
// ac-1's BEHAVIORAL kind only, so ac-1 exercises both halves of ac-2 on one
// card: the authored obligation's title for behavioral, and the disclosed
// "no obligation" badge for the still-un-obligated static (dc-2). ac-2
// declares a single un-obligated kind — a second card proving the pure-
// disclosure path in isolation.
const obligationWallStorySpec = `---
id: spec/refi-decline-replay
kind: spec
class: story
title: "Refinancing decline replay"
status: draft
owners: [platform-team]
story: jira:LOAN-2203
problem: { text: "decline notices are not replayable after the fact", anchor: "#problem" }
outcome: { text: "every decline notice shown is reconstructable", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "support can replay every decline notice shown to an applicant", evidence: [behavioral, static], anchor: "#ac-1" }
  - { id: ac-2, text: "the audit log is tamper-evident", evidence: [static], anchor: "#ac-2" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Refinancing decline replay

## Problem

## Outcome

## ac-1

Replayable decline notices.

## ac-2

Tamper-evident audit log.
`

// obligationBehavioralFixture is the on-disk evidence-obligation artifact for
// obligationWallStorySpec's ac-1 behavioral kind: a fully-valid obligation
// (id/for_kind agreement, exactly one verifies edge to the WHOLE story spec,
// frozen), so evidence.Obligations strict-decodes it exactly as `verdi
// matrix` would. Its body prose is deliberately distinct from its title so a
// test can prove the PROSE (not just the title) reaches the wall (co-2).
const obligationBehavioralFixture = `---
id: obligation/refi-decline-replay--ac-1--behavioral
kind: obligation
title: "a Playwright test drives the replay view and asserts the notice reappears"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/refi-decline-replay" }
frozen: { at: 2026-07-13, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
# a Playwright test drives the replay view and asserts the notice reappears

Open a decline, mutate the underlying data, and assert the applicant-facing
notice reappears with the corrected reason.
`

// obligationStoreWithBehavioral drops obligationBehavioralFixture at the
// loader's exact on-disk home (.verdi/obligations/<spec-name>/<ac>--<kind>.md)
// under a fresh store root and returns the root. No git: attachObligations
// reads only the obligations tree, so the load path is provable without a
// checkout.
func obligationStoreWithBehavioral(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "obligations", "refi-decline-replay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir obligations: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ac-1--behavioral.md"), []byte(obligationBehavioralFixture), 0o644); err != nil {
		t.Fatalf("write obligation: %v", err)
	}
	return root
}

func obligationCard(t *testing.T, proj *BoardProjection, id string) cardView {
	t.Helper()
	for _, c := range proj.Cards {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("no card %q in projection", id)
	return cardView{}
}

// TestObligationWall_StoryACCardRendersObligations is spec/obligation-wall
// ac-2: a story AC card renders, for each declared evidence kind, that kind's
// obligation — the authored title for a kind that HAS one (behavioral) and a
// disclosed "no obligation" badge for one that does not (static) — so an
// operator reads the AC's demands on the wall itself. Drives the whole board
// path a browser hits: buildProjection positions the card, attachObligations
// loads the obligation from disk through the one shared reader, and
// renderBoardRegion emits the story AC card markup.
func TestObligationWall_StoryACCardRendersObligations(t *testing.T) {
	root := obligationStoreWithBehavioral(t)
	fm := mustDecodeSpecForTest(t, obligationWallStorySpec)
	proj, err := buildProjection("refi-decline-replay", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, root, "refi-decline-replay", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	body := renderBoardRegion(proj, &boardGitState{})

	// ac-1's behavioral kind HAS an obligation: its title is the specific
	// demand, read on the wall (co-2, legible-without-the-sidecar), and the
	// obligation's prose rides the row tooltip so the fuller argument is a
	// hover away on the wall itself — never recovered from a sidecar file.
	// ac-1's static kind has NONE: the disclosed "no obligation" badge (dc-2).
	for _, want := range []string{
		`data-testid="obligations-ac-1"`,
		`data-obligation-kind="behavioral" data-obligation-present="true"`,
		`>a Playwright test drives the replay view and asserts the notice reappears</span>`,
		`Open a decline, mutate the underlying data`, // the prose, on the wall (co-2)
		`data-obligation-kind="static" data-obligation-present="false"`,
		`data-testid="obligation-none-ac-1-static"`,
		`>no obligation<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ac-1 obligation render missing %q", want)
		}
	}

	// The authored demand precedes the disclosed badge — the AC's own
	// declared order (behavioral before static), never reshuffled.
	if strings.Index(body, `data-obligation-present="true"`) > strings.Index(body, `data-obligation-present="false"`) {
		t.Error("obligations rendered out of the AC's declared kind order")
	}

	// ac-2 declares one kind with no obligation authored: a pure-disclosure
	// card that renders legibly (dc-2), never an error or an empty card.
	for _, want := range []string{
		`data-testid="obligations-ac-2"`,
		`data-testid="obligation-none-ac-2-static"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ac-2 disclosure render missing %q", want)
		}
	}
}

// TestAttachObligations_LoadsDeclaredKinds proves the projection zip: an AC's
// DECLARED kinds are each projected onto the card, keyed to the loaded
// obligation when one exists (Present + title + prose) and marked absent
// (Present=false) when none does — the disclosure posture, never dropping a
// declared kind. This is the seam the render test's HTML rides on.
func TestAttachObligations_LoadsDeclaredKinds(t *testing.T) {
	root := obligationStoreWithBehavioral(t)
	fm := mustDecodeSpecForTest(t, obligationWallStorySpec)
	proj, err := buildProjection("refi-decline-replay", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, root, "refi-decline-replay", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}

	ac1 := obligationCard(t, proj, "ac-1")
	if len(ac1.Obligations) != 2 {
		t.Fatalf("ac-1 has %d obligation views, want 2 (one per declared kind)", len(ac1.Obligations))
	}
	// Declared order preserved: behavioral (authored) then static (absent).
	beh := ac1.Obligations[0]
	if beh.Kind != "behavioral" || !beh.Present {
		t.Errorf("ac-1[0] = %+v, want behavioral Present", beh)
	}
	if beh.Title != "a Playwright test drives the replay view and asserts the notice reappears" {
		t.Errorf("behavioral obligation title = %q, want the authored demand", beh.Title)
	}
	if !strings.Contains(beh.Body, "Open a decline, mutate the underlying data") {
		t.Errorf("behavioral obligation body = %q, want the authored prose", beh.Body)
	}
	stat := ac1.Obligations[1]
	if stat.Kind != "static" || stat.Present || stat.Title != "" {
		t.Errorf("ac-1[1] = %+v, want static absent (no title)", stat)
	}
}

// TestAttachObligations_NoOpOffStoryClass proves obligations are a STORY-AC
// concept: a feature wall's AC cards are left untouched (a feature AC wears
// its coverage receipt instead), gating identically to the projection's own
// feature/story split.
func TestAttachObligations_NoOpOffStoryClass(t *testing.T) {
	fm := mustDecodeSpecForTest(t, scopingProjectionFixtureSpec) // class: feature
	proj, err := buildProjection("scoping-fixture", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	// Even pointed at a store with an obligation on disk, a feature wall
	// discloses nothing on its AC cards.
	root := obligationStoreWithBehavioral(t)
	if err := attachObligations(proj, root, "scoping-fixture", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	if got := obligationCard(t, proj, "ac-1").Obligations; got != nil {
		t.Errorf("feature AC card carries obligations %+v, want none", got)
	}
}

// TestAttachObligations_StoryACWithoutDeclaredKinds proves an AC that
// declares no evidence kinds gets no obligation views at all — nothing to
// disclose, so the card stays as it was (never an empty obligations block).
func TestAttachObligations_StoryACWithoutDeclaredKinds(t *testing.T) {
	// Built as a literal (an evidence-less AC does not pass DecodeSpec, by
	// design — this exercises attachObligations' own len(kinds)==0 guard).
	fm := &artifact.SpecFrontmatter{
		Class: artifact.ClassStory,
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "declares no evidence kind"},
		},
	}
	proj, err := buildProjection("s", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, t.TempDir(), "s", fm); err != nil {
		t.Fatalf("attachObligations: %v", err)
	}
	if got := obligationCard(t, proj, "ac-1").Obligations; got != nil {
		t.Errorf("evidence-less AC carries obligations %+v, want none", got)
	}
}

// TestAttachObligations_MalformedObligationSurfacesError proves the loader's
// three-valued posture propagates: genuine absence is Present=false (the
// ordinary disclosed case), but a broken obligation on disk is NOT silently
// treated as absent — it surfaces as an operational error, so an authoring
// fault is never hidden behind the same disclosure absence reserves.
func TestAttachObligations_MalformedObligationSurfacesError(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "obligations", "refi-decline-replay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir obligations: %v", err)
	}
	// id names for_kind behavioral, but the frontmatter for_kind is static:
	// DC-2's id/for_kind agreement is violated — DecodeObligation refuses it.
	const malformed = `---
id: obligation/refi-decline-replay--ac-1--behavioral
kind: obligation
title: "broken"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/refi-decline-replay" }
frozen: { at: 2026-07-13, commit: 89f9926e9739b97e23eb52efb16206d0ff10ff4f }
---
broken
`
	if err := os.WriteFile(filepath.Join(dir, "ac-1--behavioral.md"), []byte(malformed), 0o644); err != nil {
		t.Fatalf("write obligation: %v", err)
	}
	fm := mustDecodeSpecForTest(t, obligationWallStorySpec)
	proj, err := buildProjection("refi-decline-replay", fm, nil, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if err := attachObligations(proj, root, "refi-decline-replay", fm); err == nil {
		t.Fatal("attachObligations accepted a malformed obligation; want an error")
	}
}

// TestWriteObligations_MarkupContract pins the compact card markup writeObligations
// emits directly from a cardView (no disk), so the render vocabulary is
// covered independently of the load path: a present row carries its kind tag,
// the title as visible text, and the body in the tooltip; a missing row
// carries the dashed "no obligation" badge under its own stable testid.
func TestWriteObligations_MarkupContract(t *testing.T) {
	c := cardView{
		ID:   "ac-9",
		Kind: string(boardlayout.ZoneAC),
		Obligations: []obligationView{
			{Kind: "runtime", Present: true, Title: "a canary asserts the retry drains", Body: "drive the live retry loop and assert the queue drains under load"},
			{Kind: "attestation", Present: false},
		},
	}
	var b strings.Builder
	writeObligations(&b, c)
	got := b.String()

	for _, want := range []string{
		`<div class="card-obligations" data-testid="obligations-ac-9">`,
		`<span class="obligation-kind">runtime</span>`,
		`title="drive the live retry loop and assert the queue drains under load">a canary asserts the retry drains</span>`,
		`<div class="obligation obligation--none" data-obligation-kind="attestation" data-obligation-present="false">`,
		`<span class="obligation-badge" data-testid="obligation-none-ac-9-attestation">no obligation</span>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("writeObligations markup missing %q\n--- got ---\n%s", want, got)
		}
	}
}
