package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVL006_NoEvidenceKind(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-006"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-006")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// --- R4-I-15 requiredness enforcement (this phase's judgment call: folded
// into VL-006, see vl006.go's doc comment) ---

// vl006NewClassMissingAllSpec is a new-class feature spec (it carries a
// round-four constraints: block, so isNewClassSpec reports it new) with no
// problem/outcome attributes and an AC with no anchor at all.
const vl006NewClassMissingAllSpec = `---
id: spec/vl-006-new-class-missing
kind: spec
class: feature
title: "VL-006: new-class feature missing problem/outcome/anchor"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
constraints:
  - { id: co-1, text: "placeholder constraint", anchor: "#co-1" }
---
# VL-006: new-class feature missing problem/outcome/anchor

## CO-1

Placeholder constraint.
`

// TestVL006_NewClassSpec_MissingProblemOutcomeAnchor_Fails is the exit
// criterion "a new-class spec missing problem/outcome/anchor fails with
// your chosen rule id": VL-006, per this phase's judgment call.
func TestVL006_NewClassSpec_MissingProblemOutcomeAnchor_Fails(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-new-class-missing/spec.md", vl006NewClassMissingAllSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-006")
	// problem missing, outcome missing, ac-1 anchor missing: 3 findings.
	if len(findings) != 3 {
		t.Fatalf("got %d findings, want 3:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL006_GrandfatheredSpec_MissingProblemOutcomeAnchor_NeverFires is the
// exit criterion's other half: "every v0 grandfathered corpus spec still
// passes untouched" — a feature spec carrying none of the round-four
// surface fields (no problem/outcome/stubs/supersession/constraints/
// decisions/open_questions) is grandfathered by isNewClassSpec's
// discriminator and never subject to the requiredness check, even though
// it has no problem/outcome and its AC carries no anchor.
func TestVL006_GrandfatheredSpec_MissingProblemOutcomeAnchor_NeverFires(t *testing.T) {
	const grandfatheredSpec = `---
id: spec/vl-006-grandfathered
kind: spec
class: feature
title: "VL-006: v0 grandfathered feature, no round-four surface at all"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-006: v0 grandfathered feature, no round-four surface at all
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-grandfathered/spec.md", grandfatheredSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-006" {
			t.Fatalf("VL-006 fired on a v0 grandfathered feature spec: %s", f.String())
		}
	}
}

// TestVL006_StorySpec_AlwaysNewClass proves the story-class half of the
// discriminator: the story class is always new (no v0 story class ever
// existed, R4-I-9), so a story spec with a missing AC anchor fires VL-006
// even though it otherwise looks minimal.
func TestVL006_StorySpec_AlwaysNewClass(t *testing.T) {
	const storyMissingAnchor = `---
id: spec/vl-006-story-missing-anchor
kind: spec
class: story
title: "VL-006: story spec, AC with no anchor"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
story: jira:LOAN-0088
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-006: story spec, AC with no anchor

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-story-missing-anchor/spec.md", storyMissingAnchor)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-006")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (missing AC anchor):\n%s", len(findings), findingsString(findings))
	}
}

// --- Stub acceptance_criteria integrity (this phase: folded into
// VL-006, same syntactic-stub-surface home the house steer names) ---

// vl006StubSpecTmpl is a feature spec declaring ac-1 and ac-2 with one
// stub whose acceptance_criteria list is the %s insertion point.
const vl006StubSpecTmpl = `---
id: spec/vl-006-stub
kind: spec
class: feature
title: "VL-006: feature with a stub"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
  - { id: ac-2, text: "placeholder", evidence: [static], anchor: "#ac-2" }
stubs:
  - { slug: badge-computes, acceptance_criteria: [%s] }
---
# VL-006: feature with a stub

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

## AC-2

Placeholder.
`

// TestVL006_StubACRefs is table-driven over a stub's acceptance_criteria:
// entries naming declared ACs lint clean; a dangling ref fires VL-006
// naming the stub slug and the missing id.
func TestVL006_StubACRefs(t *testing.T) {
	cases := []struct {
		name     string
		acList   string
		wantFire bool
		wantIn   []string // substrings the finding message must carry
	}{
		{name: "all declared", acList: "ac-1, ac-2", wantFire: false},
		{name: "dangling ref", acList: "ac-1, ac-99", wantFire: true, wantIn: []string{"badge-computes", "ac-99"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := strings.Replace(vl006StubSpecTmpl, "%s", tc.acList, 1)
			dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-stub/spec.md", spec)
			repo := buildLintRepo(t, dir)
			findings := runLint(t, repo.Dir, Context{}, Options{})
			var got []Finding
			for _, f := range findings {
				if f.Rule == "VL-006" {
					got = append(got, f)
				}
			}
			if !tc.wantFire {
				if len(got) != 0 {
					t.Fatalf("VL-006 fired on a valid stub: %s", findingsString(got))
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d VL-006 findings, want 1:\n%s", len(got), findingsString(got))
			}
			for _, want := range tc.wantIn {
				if !strings.Contains(got[0].Message, want) {
					t.Errorf("finding %q does not name %q", got[0].Message, want)
				}
			}
		})
	}
}

// TestVL006_StubACRefs_GrandfatheredAndDecodeErrSkipped mirrors VL-006's
// existing guards: a grandfathered doc and a decode-error doc are never
// subject to the stub-AC check (grandfathered v0 specs never carried
// stubs, and a decode-failed doc has no Spec to read).
func TestVL006_StubACRefs_GrandfatheredAndDecodeErrSkipped(t *testing.T) {
	// A decode-error doc: an unknown frontmatter field fails DecodeStrict,
	// so d.Spec is nil and the stub check must not panic or fire VL-006.
	const decodeErr = `---
id: spec/vl-006-stub-decode-err
kind: spec
class: feature
title: "VL-006: stub spec that fails decode"
status: draft
owners: [platform-team]
bogus_field: nope
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
stubs:
  - { slug: badge-computes, acceptance_criteria: [ac-99] }
---
# VL-006: stub spec that fails decode
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-stub-decode-err/spec.md", decodeErr)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-006" {
			t.Fatalf("VL-006 fired on a decode-error doc: %s", f.String())
		}
	}

	// A grandfathered (archived, GrandfatherArchive on) doc with a dangling
	// stub ref is skipped too — the same guard line VL-006's other checks
	// already sit behind.
	const grandfatheredDangling = `---
id: spec/vl-006-stub-grandfathered
kind: spec
class: feature
title: "VL-006: archived stub spec, dangling ref"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
stubs:
  - { slug: badge-computes, acceptance_criteria: [ac-99] }
---
# VL-006: archived stub spec, dangling ref
`
	gdir := adHocOverlayDir(t, ".verdi/specs/archive/vl-006-stub-grandfathered/spec.md", grandfatheredDangling)
	grepo := buildLintRepo(t, gdir)
	gfindings := runLint(t, grepo.Dir, Context{}, Options{GrandfatherArchive: true})
	for _, f := range gfindings {
		if f.Rule == "VL-006" {
			t.Fatalf("VL-006 fired on a grandfathered archived doc: %s", f.String())
		}
	}
}

// TestVL006_NewClassSpec_FullyPopulated_Clean proves the positive
// complement: a new-class spec with problem/outcome and every object
// anchor present and resolving lints clean.
func TestVL006_NewClassSpec_FullyPopulated_Clean(t *testing.T) {
	const fullyPopulated = `---
id: spec/vl-006-new-class-clean
kind: spec
class: feature
title: "VL-006: new-class feature, fully populated"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-006: new-class feature, fully populated

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-new-class-clean/spec.md", fullyPopulated)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-006" {
			t.Fatalf("VL-006 fired on a fully-populated new-class spec: %s", f.String())
		}
	}
}
