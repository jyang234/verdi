package lint

import (
	"fmt"
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
// problem/outcome attributes and an AC with no anchor at all. ac-1 declares
// attestation (L-M14 remedy 1's own new requirement) precisely so this
// fixture stays a clean, single-concern proof of the requiredness family
// alone — it is status: draft (unfrozen), so without attestation it would
// ALSO trip checkFeatureACAttestation, a 4th, orthogonal finding this test
// does not exist to prove.
const vl006NewClassMissingAllSpec = `---
id: spec/vl-006-new-class-missing
kind: spec
class: feature
title: "VL-006: new-class feature missing problem/outcome/anchor"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static, attestation] }
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
// stub whose acceptance_criteria list is the %s insertion point. Both ACs
// declare attestation (L-M14 remedy 1) so this status: draft fixture stays
// a clean, single-concern proof of the stub-AC-integrity check alone.
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
  - { id: ac-1, text: "placeholder", evidence: [static, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "placeholder", evidence: [static, attestation], anchor: "#ac-2" }
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

// vl006SpikeStubSpecTmpl is a feature spec declaring oq-1 and oq-2 with
// one spike stub whose resolves list is the %s insertion point — the
// DC-4 sibling check to checkStubACs, folded into the same VL-006 rule
// (vl006.go's doc comment: "the rule that already validates stub
// acceptance_criteria"). ac-1 declares attestation (L-M14 remedy 1) so
// this status: draft fixture stays a clean, single-concern proof of the
// spike-stub-resolves-integrity check alone.
const vl006SpikeStubSpecTmpl = `---
id: spec/vl-006-spike-stub
kind: spec
class: feature
title: "VL-006: feature with a spike stub"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static, attestation], anchor: "#ac-1" }
open_questions:
  - { id: oq-1, text: "placeholder", anchor: "#oq-1" }
  - { id: oq-2, text: "placeholder", anchor: "#oq-2" }
stubs:
  - { slug: retry-strategy-spike, spike: true, resolves: [%s] }
---
# VL-006: feature with a spike stub

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.

## OQ-1

Placeholder.

## OQ-2

Placeholder.
`

// TestVL006_SpikeStubResolvesRefs is the resolves-side analogue of
// TestVL006_StubACRefs: a spike stub's resolves entries naming declared
// open questions lint clean; a dangling ref fires VL-006 naming the stub
// slug and the missing id.
func TestVL006_SpikeStubResolvesRefs(t *testing.T) {
	cases := []struct {
		name     string
		oqList   string
		wantFire bool
		wantIn   []string
	}{
		{name: "all declared", oqList: "oq-1, oq-2", wantFire: false},
		{name: "dangling ref", oqList: "oq-1, oq-99", wantFire: true, wantIn: []string{"retry-strategy-spike", "oq-99"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := strings.Replace(vl006SpikeStubSpecTmpl, "%s", tc.oqList, 1)
			dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-spike-stub/spec.md", spec)
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
					t.Fatalf("VL-006 fired on a valid spike stub: %s", findingsString(got))
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
// anchor present and resolving lints clean. ac-1 declares attestation
// (L-M14 remedy 1) — required for a status: draft feature spec's own AC
// to lint clean now that checkFeatureACAttestation exists.
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
  - { id: ac-1, text: "placeholder", evidence: [static, attestation], anchor: "#ac-1" }
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

// --- L-M14 remedy 1: feature-AC attestation floor (03 §Declarations and
// binding / §The feature fold's outcome floor) ---

// vl006FeatureACSpecTmpl renders a draft (unfrozen), new-class feature spec
// with one AC whose evidence list is the %s insertion point.
const vl006FeatureACSpecTmpl = `---
id: spec/vl-006-feature-ac
kind: spec
class: feature
title: "VL-006: feature AC attestation floor"
status: draft
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [%s], anchor: "#ac-1" }
---
# VL-006: feature AC attestation floor

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// attestationFloorFindings filters findings down to checkFeatureACAttestation's
// own message shape, so a test can isolate it from any other VL-006 finding
// the same fixture might also carry.
func attestationFloorFindings(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Rule == "VL-006" && strings.Contains(f.Message, "does not declare attestation") {
			out = append(out, f)
		}
	}
	return out
}

// TestVL006_FeatureACAttestation is L-M14 remedy 1's static register: a
// draft, new-class feature AC missing attestation among its declared
// evidence kinds fires VL-006, naming the AC and the outcome-floor
// rationale (03 §The feature fold); declaring attestation — alone or
// alongside another kind — clears it.
func TestVL006_FeatureACAttestation(t *testing.T) {
	cases := []struct {
		name     string
		evidence string
		wantFire bool
	}{
		{"missing attestation entirely (static only)", "static", true},
		{"missing attestation entirely (behavioral only)", "behavioral", true},
		{"attestation alone", "attestation", false},
		{"attestation alongside another kind", "static, attestation", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := fmt.Sprintf(vl006FeatureACSpecTmpl, tc.evidence)
			dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-feature-ac/spec.md", spec)
			repo := buildLintRepo(t, dir)
			findings := runLint(t, repo.Dir, Context{}, Options{})
			got := attestationFloorFindings(findings)
			if !tc.wantFire {
				if len(got) != 0 {
					t.Fatalf("attestation-floor check fired unexpectedly: %s", findingsString(got))
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d attestation-floor findings, want 1:\n%s", len(got), findingsString(got))
			}
			if !strings.Contains(got[0].Message, "ac-1") {
				t.Errorf("finding %q does not name ac-1", got[0].Message)
			}
		})
	}
}

// TestVL006_FeatureACAttestation_StoryClassNeverRequired proves the check
// is feature-only: a story spec's AC carries no outcome-floor concept
// (03's text is explicit the floor is a feature-level addition), so a
// story AC declaring only static evidence never trips it — the story-class
// row of TestVL006_StorySpec_AlwaysNewClass already proves that fixture's
// OWN finding is the missing-anchor one, not an attestation one; this test
// names the attestation-floor exemption directly.
func TestVL006_FeatureACAttestation_StoryClassNeverRequired(t *testing.T) {
	const storySpec = `---
id: spec/vl-006-story-ac-no-attestation
kind: spec
class: story
title: "VL-006: story AC, no attestation required"
status: draft
owners: [platform-team]
story: jira:VL-006-1
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
---
# VL-006: story AC, no attestation required
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-story-ac-no-attestation/spec.md", storySpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if got := attestationFloorFindings(findings); len(got) != 0 {
		t.Fatalf("attestation-floor check fired on a class: story AC (feature-only, 03 §The feature fold): %s", findingsString(got))
	}
}

// vl006FeatureACFrozenSpecTmpl renders an ALREADY-ACCEPTED (frozen: set),
// new-class feature spec whose one AC does not declare attestation — the
// %s insertion point is the status (accepted-pending-build or closed, both
// carry a frozen: stamp). The commit cited is stale-decline's own real,
// reachable frozen commit (examples/showcase, also used elsewhere in this
// package's tests) so VL-009's reachability check stays clean and never
// muddies this test's own onlyRule/attestationFloorFindings assertions.
const vl006FeatureACFrozenSpecTmpl = `---
id: spec/vl-006-feature-ac-frozen
kind: spec
class: feature
title: "VL-006: already-accepted feature AC, no attestation"
status: %s
owners: [platform-team]
problem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2026-05-14, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-006: already-accepted feature AC, no attestation

## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

// TestVL006_FeatureACAttestation_GrandfatheredByAcceptance is L-M14 remedy
// 1's grandfathering proof, BROADER than archive location alone
// (vl006.go's checkFeatureACAttestation doc comment): an already-accepted
// (frozen: set) feature AC missing attestation never fires, whether it is
// still active (accepted-pending-build) or already archived (closed) — the
// same "amending evidence kinds on a frozen spec requires full
// supersession" reasoning L-M14's own operating-model adjudication used,
// applied generally rather than re-litigated per spec.
func TestVL006_FeatureACAttestation_GrandfatheredByAcceptance(t *testing.T) {
	t.Run("active, accepted-pending-build", func(t *testing.T) {
		spec := fmt.Sprintf(vl006FeatureACFrozenSpecTmpl, "accepted-pending-build")
		dir := adHocOverlayDir(t, ".verdi/specs/active/vl-006-feature-ac-frozen/spec.md", spec)
		repo := buildLintRepo(t, dir)
		findings := runLint(t, repo.Dir, Context{}, Options{})
		if got := attestationFloorFindings(findings); len(got) != 0 {
			t.Fatalf("attestation-floor check fired on an already-accepted, still-active feature spec: %s", findingsString(got))
		}
	})

	t.Run("archived, closed", func(t *testing.T) {
		spec := fmt.Sprintf(vl006FeatureACFrozenSpecTmpl, "closed")
		dir := adHocOverlayDir(t, ".verdi/specs/archive/vl-006-feature-ac-frozen/spec.md", spec)
		repo := buildLintRepo(t, dir)
		findings := runLint(t, repo.Dir, Context{}, Options{})
		if got := attestationFloorFindings(findings); len(got) != 0 {
			t.Fatalf("attestation-floor check fired on an archived feature spec (the literal spec/operating-model case): %s", findingsString(got))
		}
	})
}
