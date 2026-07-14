package workbench

// Integration tests for the judged-findings case-file chip
// (spec/derivation-drawer ac-3) through the real load path — loadBoard
// over a real (hermetic, fixturegit) git store — and for
// gitCoversResolver, the wallbadge.CoversResolver port's gitx-backed
// adapter: covers resolution is a git integration and is proven against
// real history, never a mock of git itself.

import (
	"context"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

const judgedFixtureName = "widget-judged"

// judgedFixtureSpec declares two decisions — dc-3's set-comparison
// operand — and stays otherwise lint-quiet so the case file wears the
// judged chip alone.
func judgedFixtureSpec(outcome string) string {
	return `---
id: spec/widget-judged
kind: spec
class: feature
title: "Widget judged"
status: draft
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "` + outcome + `", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "does a thing", evidence: [runtime], anchor: "#ac-1" }
decisions:
  - { id: dc-1, text: "first decision", anchor: "#dc-1" }
  - { id: dc-2, text: "second decision", anchor: "#dc-2" }
---
# Widget judged

## Problem

p

## Outcome

` + outcome + `

## ac-1

Prose.

## dc-1

Prose.

## dc-2

Prose.
`
}

// judgedFixtureReport renders a valid decision-conflict report pinned at
// covers, scanning scanned.
func judgedFixtureReport(covers string, scanned []string) string {
	scannedYAML := "[]"
	if len(scanned) > 0 {
		scannedYAML = "[" + strings.Join(scanned, ", ") + "]"
	}
	return `---
schema: verdi.decisionconflict/v1
covers: ` + covers + `
findings:
  - { id: judged-dcf-1, kind: judged, text: "a swept conflict", disposition: no-conflict, note: "cleared on review" }
  - { id: judged-dcf-2, kind: judged, text: "an open conflict" }
sweep_provenance: { adr_corpus_digest: sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570, decisions_scanned: ` + scannedYAML + ` }
---
# Decision-conflict report
`
}

func judgedFullScan() []string {
	return []string{"spec/widget-judged#dc-1", "spec/widget-judged#dc-2"}
}

const judgedSpecPath = ".verdi/specs/active/" + judgedFixtureName + "/spec.md"
const judgedReportPath = ".verdi/specs/active/" + judgedFixtureName + "/decision-conflict-report.md"

// newJudgedFixture builds real git history: layer 1 commits the spec
// (its sha is the sweep's covers pin), layer 2 adds the report — and,
// when staleOutcome is non-empty, rewrites the spec so the wall renders
// different bytes than the pinned covers commit holds.
func newJudgedFixture(t *testing.T, scanned []string, staleOutcome string) string {
	t.Helper()
	layer1 := fixturegit.Layer{
		Files: map[string]string{
			judgedSpecPath:      judgedFixtureSpec("o"),
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed judged fixture",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1})
	layer2Files := map[string]string{
		judgedReportPath: judgedFixtureReport(repo.Heads[0], scanned),
	}
	if staleOutcome != "" {
		layer2Files[judgedSpecPath] = judgedFixtureSpec(staleOutcome)
	}
	repo2 := fixturegit.Build(t, []fixturegit.Layer{layer1, {Files: layer2Files, Message: "sweep report"}})
	return repo2.Dir
}

func judgedChip(t *testing.T, proj *BoardProjection) badgeView {
	t.Helper()
	for _, b := range proj.CaseFileBadges {
		if b.Source == "align:judged-sweep" {
			return b
		}
	}
	t.Fatalf("no align:judged-sweep chip on the case file: %+v", proj.CaseFileBadges)
	return badgeView{}
}

func TestJudgedSweepChip_FreshCompleteThroughLoadBoard(t *testing.T) {
	root := newJudgedFixture(t, judgedFullScan(), "")
	proj, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), judgedFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	chip := judgedChip(t, proj)

	if chip.Label != "2 judged findings" {
		t.Errorf("Label = %q", chip.Label)
	}
	if len(chip.Disclosures) != 0 {
		t.Errorf("a fresh, complete sweep must carry no mismatch line, got %q", chip.Disclosures)
	}
	if len(chip.Provenance) != 3 || !strings.HasPrefix(chip.Provenance[0], "sweep covers ") {
		t.Errorf("Provenance = %q, want covers/adr_corpus_digest/decisions_scanned", chip.Provenance)
	}
	if len(chip.Inputs) != 2 || chip.Inputs[0].Name != "covers" || chip.Inputs[1].Name != "decision-conflict-report" {
		t.Fatalf("Inputs = %+v", chip.Inputs)
	}
	if !strings.HasPrefix(chip.Inputs[1].Revision, "sha256:") {
		t.Errorf("report input revision %q is not a content digest", chip.Inputs[1].Revision)
	}
}

func TestJudgedSweepChip_StaleAndPartialDisclosed(t *testing.T) {
	// The report scans only dc-1 (partial) and covers layer 1's commit
	// while layer 2 rewrote the spec (stale): both contrasts must render
	// as disclosure lines on the one chip.
	root := newJudgedFixture(t, []string{"spec/widget-judged#dc-1"}, "an amended outcome")
	proj, _, _, err := (&boardSpecServer{root: root}).loadBoard(context.Background(), judgedFixtureName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	chip := judgedChip(t, proj)

	if len(chip.Disclosures) != 2 {
		t.Fatalf("Disclosures = %q, want the covers contrast and the missing decision id", chip.Disclosures)
	}
	if !strings.HasPrefix(chip.Disclosures[0], "sweep covers ") || !strings.Contains(chip.Disclosures[0], "; this wall renders sha256:") {
		t.Errorf("covers contrast line = %q", chip.Disclosures[0])
	}
	if strings.Contains(chip.Disclosures[0], "cannot resolve") {
		t.Errorf("a resolvable stale pin must contrast, not disclose inability: %q", chip.Disclosures[0])
	}
	if chip.Disclosures[1] != "dc-2 is not in decisions_scanned" {
		t.Errorf("missing-decision line = %q", chip.Disclosures[1])
	}
}

func TestGitCoversResolver(t *testing.T) {
	// Real history: commit 1 holds spec v1, commit 2 rewrites it. The
	// resolver must recover v1's digest at commit 1 (differing from the
	// working tree's), and report ok=false — never an error, never a
	// made-up digest — for a commit or path it cannot resolve.
	layer1 := fixturegit.Layer{
		Files:   map[string]string{judgedSpecPath: judgedFixtureSpec("o"), ".verdi/.gitignore": "data/\n"},
		Message: "v1",
	}
	layer2 := fixturegit.Layer{
		Files:   map[string]string{judgedSpecPath: judgedFixtureSpec("changed")},
		Message: "v2",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	r := gitCoversResolver{root: repo.Dir}
	ctx := context.Background()

	oldDigest, ok, err := r.SpecDigestAtCommit(ctx, repo.Heads[0], judgedSpecPath)
	if err != nil || !ok {
		t.Fatalf("resolving v1: ok=%v err=%v", ok, err)
	}
	if want := contentDigest([]byte(judgedFixtureSpec("o"))); oldDigest != want {
		t.Errorf("digest at commit 1 = %q, want %q", oldDigest, want)
	}
	newDigest, ok, err := r.SpecDigestAtCommit(ctx, repo.Heads[1], judgedSpecPath)
	if err != nil || !ok {
		t.Fatalf("resolving v2: ok=%v err=%v", ok, err)
	}
	if newDigest == oldDigest {
		t.Error("digests at the two commits collide — the resolver is not reading pinned history")
	}

	if _, ok, err := r.SpecDigestAtCommit(ctx, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", judgedSpecPath); err != nil || ok {
		t.Errorf("nonexistent commit: ok=%v err=%v, want disclosed-unproven (false, nil)", ok, err)
	}
	if _, ok, err := r.SpecDigestAtCommit(ctx, repo.Heads[0], "no/such/path.md"); err != nil || ok {
		t.Errorf("path absent at commit: ok=%v err=%v, want disclosed-unproven (false, nil)", ok, err)
	}
	if _, ok, err := (gitCoversResolver{root: t.TempDir()}).SpecDigestAtCommit(ctx, repo.Heads[0], judgedSpecPath); err != nil || ok {
		t.Errorf("non-repo root: ok=%v err=%v, want disclosed-unproven (false, nil)", ok, err)
	}
}
