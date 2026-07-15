package workbench

// Tests for the evidence slot's wall surface (spec/evidence-slot ac-1/
// ac-3): the fold-derived record state of each DECLARED kind joins the
// card's existing per-kind obligation row — one row carrying both what
// the kind demands and what it holds, emitted by the one board renderer
// — and the never-synced story wall (no derived tree at all) renders
// every declared kind as a CALM empty slot, badged through the badge
// compute layer, never an error.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/wallbadge"
)

const slotWallSpecName = "widget-slot-story"

// slotWallSpec declares ac-1 with two kinds — behavioral (an authored
// obligation, below) and static (no obligation) — the exact shape ac-3's
// obligation names: one kind with an obligation and no record, one with
// neither, both reading as ONE row each.
const slotWallSpec = `---
id: spec/widget-slot-story
kind: spec
class: story
title: "Widget slot story"
status: draft
owners: [platform-team]
story: jira:WSL-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [behavioral, static], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/some-parent#ac-1" }
---
# Widget slot story

## Problem

p

## Outcome

o

## ac-1

Prose.
`

const slotWallObligation = `---
id: obligation/widget-slot-story--ac-1--behavioral
kind: obligation
title: "a Playwright test drives the widget"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/widget-slot-story" }
frozen: { at: 2026-07-13, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# a Playwright test drives the widget

Drive it end to end.
`

func newSlotWallFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + slotWallSpecName + "/spec.md":            slotWallSpec,
			".verdi/obligations/" + slotWallSpecName + "/ac-1--behavioral.md": slotWallObligation,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed slot wall fixture",
	}})
}

// TestEvidenceSlot_JoinsObligationRow drives the whole board path a
// browser hits (loadBoard: buildProjection, attachObligations,
// attachBadges) over a never-synced story wall and proves ac-3's join on
// the DATA and the MARKUP at once: each declared kind is ONE obligation
// view now carrying its slot state, each row's HTML holds both the
// demand half and the record-state chip, and the fold:empty-slot badge
// rides the card's existing badge surface (ac-2/dc-3's one attachment
// path). No derived tree exists — dc-1's ordinary authoring state — so
// both kinds read empty, calmly.
func TestEvidenceSlot_JoinsObligationRow(t *testing.T) {
	repo := newSlotWallFixture(t)
	s := &boardSpecServer{root: repo.Dir}
	proj, _, _, err := s.loadBoard(context.Background(), slotWallSpecName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}

	ac1 := badgeCardByID(t, proj, "ac-1")
	if len(ac1.Obligations) != 2 {
		t.Fatalf("ac-1 obligations = %+v, want 2 (one per declared kind — never a second list)", ac1.Obligations)
	}
	for _, o := range ac1.Obligations {
		if o.Slot != "empty" || o.SlotRecords != 0 {
			t.Errorf("%s view = %+v, want Slot \"empty\" with 0 records (no derived tree)", o.Kind, o)
		}
	}
	// The demand half is untouched by the join: behavioral keeps its
	// authored title, static keeps Present=false.
	if !ac1.Obligations[0].Present || ac1.Obligations[0].Kind != "behavioral" {
		t.Errorf("obligations[0] = %+v, want the authored behavioral demand intact", ac1.Obligations[0])
	}
	if ac1.Obligations[1].Present || ac1.Obligations[1].Kind != "static" {
		t.Errorf("obligations[1] = %+v, want the un-obligated static kind intact", ac1.Obligations[1])
	}

	// The empty slots badge through the badge compute layer onto the
	// card's own badge surface (dc-3): source fold:empty-slot, the
	// derived-tree location probed disclosed as absent.
	var slotBadge *badgeView
	for i := range ac1.Badges {
		if ac1.Badges[i].Source == "fold:empty-slot" {
			slotBadge = &ac1.Badges[i]
		}
	}
	if slotBadge == nil {
		t.Fatalf("ac-1.Badges = %+v, want a fold:empty-slot badge", ac1.Badges)
	}
	if slotBadge.Label != "2 empty slots" {
		t.Errorf("slot badge label = %q, want \"2 empty slots\"", slotBadge.Label)
	}
	foundProbe := false
	for _, in := range slotBadge.Inputs {
		if in.Name == "derived-tree" && in.Revision == "absent" && strings.Contains(in.Path, "spec--widget-slot-story") {
			foundProbe = true
		}
	}
	if !foundProbe {
		t.Errorf("slot badge inputs = %+v, want the derived-tree location probed with revision \"absent\" (dc-1's honest receipt)", slotBadge.Inputs)
	}

	// The markup: one renderer emits one row per kind, each carrying both
	// halves — the obligation content AND the record-state chip, in the
	// dashed pending register (data-slot-state="empty").
	body := renderBoardRegion(proj, &boardGitState{})
	for _, want := range []string{
		`data-testid="obligations-ac-1"`,
		`>a Playwright test drives the widget</span><span class="slot-chip slot-chip--empty" data-testid="slot-ac-1-behavioral" data-slot-state="empty">no record</span>`,
		`>no obligation</span><span class="slot-chip slot-chip--empty" data-testid="slot-ac-1-static" data-slot-state="empty">no record</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board region missing %q", want)
		}
	}
	// No card element repeats a kind: exactly one card-obligations block,
	// exactly two per-kind rows inside it.
	if got := strings.Count(body, `data-testid="obligations-ac-1"`); got != 1 {
		t.Errorf("obligations-ac-1 appears %d times, want exactly 1 (one per-kind list, ac-3)", got)
	}
	if got := strings.Count(body, `data-obligation-kind=`); got != 2 {
		t.Errorf("per-kind rows = %d, want exactly 2 (one row per declared kind, never a second list)", got)
	}
}

// TestEvidenceSlot_HeldSlotRendersCount is ac-1's flip: a derived-tree
// record of a declared kind (behavioral, bound to ac-1, at a real
// ancestor-or-self commit) fills exactly that kind's slot — the chip
// leaves the dashed register and counts its records — while the sibling
// static kind stays empty. Presence only: the markup never carries the
// record's pass/fail verdict (dc-4).
func TestEvidenceSlot_HeldSlotRendersCount(t *testing.T) {
	repo := newSlotWallFixture(t)
	derived := filepath.Join(repo.Dir, ".verdi", "data", "derived", "spec--widget-slot-story", repo.Head)
	if err := os.MkdirAll(derived, 0o755); err != nil {
		t.Fatalf("mkdir derived: %v", err)
	}
	record := `[{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"behavioral","verdict":"pass",` +
		`"witness":"golden: widget_flow","producer":"widget_flow","provenance":{"source":"ci","pipeline":"7","commit":"` + repo.Head + `"},` +
		`"digest":"sha256:ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12ab12"}]`
	if err := os.WriteFile(filepath.Join(derived, "verdicts.json"), []byte(record), 0o644); err != nil {
		t.Fatalf("writing verdicts.json: %v", err)
	}

	s := &boardSpecServer{root: repo.Dir}
	proj, _, _, err := s.loadBoard(context.Background(), slotWallSpecName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	ac1 := badgeCardByID(t, proj, "ac-1")
	if o := ac1.Obligations[0]; o.Slot != "held" || o.SlotRecords != 1 {
		t.Errorf("behavioral view = %+v, want held with 1 record", o)
	}
	if o := ac1.Obligations[1]; o.Slot != "empty" {
		t.Errorf("static view = %+v, want empty (the record fills only its own kind)", o)
	}

	body := renderBoardRegion(proj, &boardGitState{})
	for _, want := range []string{
		`data-testid="slot-ac-1-behavioral" data-slot-state="held">1 record</span>`,
		`data-testid="slot-ac-1-static" data-slot-state="empty">no record</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board region missing %q", want)
		}
	}
	// dc-4: presence, never verdicts — the fold's verdict vocabulary must
	// not reach the slot markup (the record deliberately carries
	// verdict:pass; scope the check to this card's slot chips).
	for _, verdictWord := range []string{`data-slot-state="pass`, `data-slot-state="fail`, `>pass<`, `>fail<`} {
		if strings.Contains(body, verdictWord) {
			t.Errorf("board region contains %q — slot state must disclose presence, never the fold's verdicts (dc-4)", verdictWord)
		}
	}
}

// TestWriteSlotChip_Table is the render unit's own happy/negative table:
// every (kind, slot state) cell of the chip vocabulary, plus the
// no-state case (Slot == "") writing NOTHING — the byte-identical
// pre-badge markup for callers that never attach badges.
func TestWriteSlotChip_Table(t *testing.T) {
	cases := []struct {
		name string
		view obligationView
		want string
	}{
		{"empty record kind", obligationView{Kind: "static", Slot: "empty"},
			`<span class="slot-chip slot-chip--empty" data-testid="slot-ac-9-static" data-slot-state="empty">no record</span>`},
		{"held one record", obligationView{Kind: "behavioral", Slot: "held", SlotRecords: 1},
			`<span class="slot-chip slot-chip--held" data-testid="slot-ac-9-behavioral" data-slot-state="held">1 record</span>`},
		{"held many records", obligationView{Kind: "behavioral", Slot: "held", SlotRecords: 3},
			`<span class="slot-chip slot-chip--held" data-testid="slot-ac-9-behavioral" data-slot-state="held">3 records</span>`},
		{"empty attestation", obligationView{Kind: "attestation", Slot: "empty"},
			`<span class="slot-chip slot-chip--empty" data-testid="slot-ac-9-attestation" data-slot-state="empty">no attestation</span>`},
		{"held attestation", obligationView{Kind: "attestation", Slot: "held", SlotRecords: 1},
			`<span class="slot-chip slot-chip--held" data-testid="slot-ac-9-attestation" data-slot-state="held">attested</span>`},
		{"no computed state writes nothing", obligationView{Kind: "static"}, ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeSlotChip(&b, "ac-9", tc.view)
			if b.String() != tc.want {
				t.Errorf("writeSlotChip = %q, want %q", b.String(), tc.want)
			}
		})
	}
}

// TestMergeSlotStates_Table proves the join enriches by kind and never
// invents: a matching kind gains its state, a view with no matching
// state stays chip-free (Slot == ""), and no view is added or removed.
func TestMergeSlotStates_Table(t *testing.T) {
	views := []obligationView{
		{Kind: "behavioral", Present: true, Title: "t"},
		{Kind: "static"},
		{Kind: "runtime"},
	}
	mergeSlotStates(views, []wallbadge.SlotState{
		{Kind: "behavioral", Empty: false, Records: 2},
		{Kind: "static", Empty: true},
	})
	if len(views) != 3 {
		t.Fatalf("views = %d entries, want 3 (the join never adds or removes rows)", len(views))
	}
	if views[0].Slot != "held" || views[0].SlotRecords != 2 || !views[0].Present || views[0].Title != "t" {
		t.Errorf("behavioral = %+v, want held/2 with the demand half untouched", views[0])
	}
	if views[1].Slot != "empty" || views[1].SlotRecords != 0 {
		t.Errorf("static = %+v, want empty", views[1])
	}
	if views[2].Slot != "" {
		t.Errorf("runtime = %+v, want no state (no matching compute — never guessed)", views[2])
	}
}
