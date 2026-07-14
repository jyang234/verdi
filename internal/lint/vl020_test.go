package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// vl020StoryMD is a minimal, otherwise-lint-clean STORY spec declaring one
// real acceptance criterion (ac-1, evidence: [behavioral]), mirroring
// vl019_test.go's own vl019StorySpecMD template (problem/outcome, a story:
// tracker ref, and an implements edge into the golden corpus's own
// stale-decline#ac-1) but accepted-pending-build with a frozen stamp, so
// VL-020's own (non-draft) gate has something to check.
const vl020StoryMD = `---
id: spec/vl-020-story
kind: spec
class: story
title: "VL-020: accepted story declaring one evidence kind"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0299
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: accepted story declaring one evidence kind

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// vl020ObligationBehavioralMD is a valid obligation backing vl020StoryMD's
// ac-1 behavioral kind — decodes cleanly and satisfies VL-011 (path
// agreement) and VL-019 (verifies a real STORY ac) on its own, so writing it
// never trips an unrelated rule.
const vl020ObligationBehavioralMD = `---
id: obligation/vl-020-story--ac-1--behavioral
kind: obligation
title: "VL-020: behavioral obligation for vl-020-story ac-1"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vl-020-story" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: behavioral obligation for vl-020-story ac-1

What the behavioral evidence must specifically show.
`

// vl020PartialStoryMD is vl020StoryMD's two-kind sibling: ac-1 declares
// BOTH static and behavioral, for the "one of two obligations present"
// table case.
const vl020PartialStoryMD = `---
id: spec/vl-020-partial
kind: spec
class: story
title: "VL-020: accepted story declaring two evidence kinds"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0399
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static, behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: accepted story declaring two evidence kinds

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

const vl020PartialObligationStaticMD = `---
id: obligation/vl-020-partial--ac-1--static
kind: obligation
title: "VL-020: static obligation for vl-020-partial ac-1 (behavioral still missing)"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/vl-020-partial" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: static obligation for vl-020-partial ac-1 (behavioral still missing)

What the static evidence must specifically show.
`

// TestVL020_ObligationExistence_TableDriven is the core ac-1 gate,
// table-driven over hermetic fixtures (co-1): a declared kind with no
// obligation refuses, naming the missing (ac, kind); with the obligation
// present it is clean; and a two-kind AC with only one obligation refuses
// naming only the still-missing kind.
func TestVL020_ObligationExistence_TableDriven(t *testing.T) {
	cases := []struct {
		name        string
		specMD      string
		specDir     string // .verdi/specs/active/<specDir>/spec.md
		obligations map[string]string
		wantFire    bool
		wantIn      []string // substrings the sole finding's message must carry
		wantAbsent  []string // substrings the sole finding's message must NOT carry
	}{
		{
			name:     "single declared kind, no obligation at all: refused, naming the missing (ac, kind)",
			specMD:   vl020StoryMD,
			specDir:  "vl-020-story",
			wantFire: true,
			wantIn:   []string{"ac-1", "behavioral", ".verdi/obligations/vl-020-story/ac-1--behavioral.md"},
		},
		{
			name:    "single declared kind, obligation present: clean",
			specMD:  vl020StoryMD,
			specDir: "vl-020-story",
			obligations: map[string]string{
				"vl-020-story/ac-1--behavioral.md": vl020ObligationBehavioralMD,
			},
			wantFire: false,
		},
		{
			name:    "two declared kinds, one obligation: refused, naming only the missing one",
			specMD:  vl020PartialStoryMD,
			specDir: "vl-020-partial",
			obligations: map[string]string{
				"vl-020-partial/ac-1--static.md": vl020PartialObligationStaticMD,
			},
			wantFire:   true,
			wantIn:     []string{"ac-1", "behavioral"},
			wantAbsent: []string{"no obligation at .verdi/obligations/vl-020-partial/ac-1--static.md"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, ".verdi/specs/active", tc.specDir, "spec.md"), tc.specMD)
			for rel, content := range tc.obligations {
				writeTestFile(t, filepath.Join(dir, ".verdi/obligations", filepath.FromSlash(rel)), content)
			}

			repo := buildLintRepo(t, dir)
			findings := runLint(t, repo.Dir, Context{}, Options{})

			if !tc.wantFire {
				for _, f := range findings {
					if f.Rule == "VL-020" {
						t.Fatalf("VL-020 fired unexpectedly: %s", findingsString(findings))
					}
				}
				return
			}

			onlyRule(t, findings, "VL-020")
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
			}
			wantPath := ".verdi/specs/active/" + tc.specDir + "/spec.md"
			if findings[0].Path != wantPath {
				t.Errorf("finding path = %q, want %q (the story spec, not the obligation)", findings[0].Path, wantPath)
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(findings[0].Message, want) {
					t.Errorf("finding %q does not name %q", findings[0].Message, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(findings[0].Message, absent) {
					t.Errorf("finding %q unexpectedly names the already-satisfied kind: %q", findings[0].Message, absent)
				}
			}
		})
	}
}

// vl020FeatureMD is a feature spec whose ac-1 declares an evidence kind and
// carries no obligation — feature ACs are exempt (dc-3): obligations are a
// story-level concern only.
const vl020FeatureMD = `---
id: spec/vl-020-feature
kind: spec
class: feature
title: "VL-020: feature spec declaring an evidence kind, no obligation"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: feature spec declaring an evidence kind, no obligation

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL020_FeatureACNoObligation_Clean proves dc-3's scoping: a FEATURE
// AC's declared kind never requires an obligation.
func TestVL020_FeatureACNoObligation_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-020-feature/spec.md", vl020FeatureMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-020" {
			t.Fatalf("VL-020 fired on a feature AC (obligations are story-only, dc-3): %s", f.String())
		}
	}
}

// vl020DraftStoryMD is a DRAFT story spec (no frozen stamp needed) whose
// ac-1 declares a kind with no obligation.
const vl020DraftStoryMD = `---
id: spec/vl-020-draft
kind: spec
class: story
title: "VL-020: draft story with an un-obligated kind"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0499
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
---
# VL-020: draft story with an un-obligated kind

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL020_DraftStoryUnobligatedKind_Tolerated is the VL-006
// activation-timing case mirrored (co-2 / spec/evidence-obligations co-2):
// "a draft story with an un-obligated kind is not refused for that reason;
// the refusal is reserved for the accept / activation path."
func TestVL020_DraftStoryUnobligatedKind_Tolerated(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-020-draft/spec.md", vl020DraftStoryMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-020" {
			t.Fatalf("VL-020 fired on a draft story (authoring must never be blocked, co-2): %s", f.String())
		}
	}
}

// vl020ClosedStoryMD is a CLOSED (archived) story with an un-obligated
// kind, NOT one of obligationGateBaseline's named entries — it proves the
// gate applies to every non-draft status, not merely
// accepted-pending-build.
const vl020ClosedStoryMD = `---
id: spec/vl-020-closed
kind: spec
class: story
title: "VL-020: closed story with an un-obligated kind"
status: closed
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0599
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: closed story with an un-obligated kind

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL020_ClosedStoryMissingObligation_StillRefuses proves the gate is
// NOT merely "status == accepted-pending-build": any non-draft status
// (here, closed, and — deliberately — NOT one of obligationGateBaseline's
// entries, and NOT run with GrandfatherArchive) is gated exactly the same
// way.
func TestVL020_ClosedStoryMissingObligation_StillRefuses(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/archive/vl-020-closed/spec.md", vl020ClosedStoryMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-020")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "ac-1") || !strings.Contains(findings[0].Message, "behavioral") {
		t.Errorf("finding does not name the missing (ac, kind): %s", findings[0].Message)
	}
}

// vl020BaselineStandInMD is a synthetic spec using the literal id/directory
// name "obligation-gate" — one of obligationGateBaseline's real entries —
// to pin the disclosed pre-existing-corpus exemption's actual mechanism
// under this package's hermetic test harness (testdata/corpus does not
// carry the real spec/obligation-gate content at all, so this is a
// deliberate stand-in exercising the exemption map, not a re-test of this
// story's own real spec).
const vl020BaselineStandInMD = `---
id: spec/obligation-gate
kind: spec
class: story
title: "VL-020: baseline-exempt pre-existing spec (synthetic stand-in)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0699
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: baseline-exempt pre-existing spec (synthetic stand-in)

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL020_BaselineExemptSpec_Tolerated pins obligationGateBaseline's own
// behavior: a story spec whose directory name is one of its named entries
// is tolerated even though it is non-draft and has no obligation — the
// disclosed, one-time, pre-existing-corpus exemption (see the map's doc
// comment in vl020.go).
func TestVL020_BaselineExemptSpec_Tolerated(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/obligation-gate/spec.md", vl020BaselineStandInMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-020" {
			t.Fatalf("VL-020 fired on a baseline-exempt spec name: %s", f.String())
		}
	}
}

// TestVL020_DecodeErrorDoc_NeverFiresOrPanics mirrors VL-006's own guard: a
// document that fails decode has d.Spec == nil, so the per-AC loop must
// never run (and must never panic) against it.
func TestVL020_DecodeErrorDoc_NeverFiresOrPanics(t *testing.T) {
	const decodeErr = `---
id: spec/vl-020-decode-err
kind: spec
class: story
title: "VL-020: story spec that fails decode"
status: accepted-pending-build
owners: [platform-team]
bogus_field: nope
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
---
# VL-020: story spec that fails decode
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-020-decode-err/spec.md", decodeErr)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-020" {
			t.Fatalf("VL-020 fired on a decode-error doc: %s", f.String())
		}
	}
}

// TestVL020_GrandfatheredArchivedDoc_NeverFires mirrors VL-006/VL-019's own
// guard: a grandfathered (archived, GrandfatherArchive on) doc with an
// un-obligated kind is skipped, the same guard line every other rule here
// sits behind.
func TestVL020_GrandfatheredArchivedDoc_NeverFires(t *testing.T) {
	const archived = `---
id: spec/vl-020-grandfathered
kind: spec
class: story
title: "VL-020: archived story, grandfathered"
status: closed
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0799
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [behavioral], anchor: "#ac-1" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
---
# VL-020: archived story, grandfathered

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`
	dir := adHocOverlayDir(t, ".verdi/specs/archive/vl-020-grandfathered/spec.md", archived)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{GrandfatherArchive: true})
	for _, f := range findings {
		if f.Rule == "VL-020" {
			t.Fatalf("VL-020 fired on a grandfathered archived doc: %s", f.String())
		}
	}
}
