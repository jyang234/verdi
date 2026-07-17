package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestVL022_MisslugFixture is spec/attest-helper AC-3's primary witness:
// an attestation whose id/path both name "vl-022-story" (VL-011's own
// id/path agreement is satisfied) but whose `verifies` target's own
// story-ref slug is "jira-vl022-1" — the D6-18 class of bug VL-022 exists
// to catch, made a named, witness-carrying refusal instead of a silent
// fold-time absent.
func TestVL022_MisslugFixture(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "misslug"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-022")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if !strings.Contains(findings[0].Message, "jira-vl022-1") {
		t.Errorf("finding does not name the correct story-ref slug: %s", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, "vl-022-story") {
		t.Errorf("finding does not name the attestation's own (wrong) directory segment: %s", findings[0].Message)
	}
}

// TestVL022_CleanFixture is the positive complement: a correctly-slugged,
// well-formed attestation (directory jira-vl022-1, matching
// store.RefSlug("jira:VL022-1")) produces no VL-022 finding.
func TestVL022_CleanFixture(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "clean"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-022" {
			t.Fatalf("VL-022 fired on a correctly-slugged, well-formed attestation: %s", f.String())
		}
	}
}

// TestVL022_NoVerifiesFixture is DC-4's disclosed scope limit, proven at
// its sharpest: an attestation with NO verifies edge at all, sitting at a
// directory that does NOT match its nominal target's story-ref slug,
// still produces NO VL-022 finding — the rule is gated on verifies-
// PRESENCE alone, never on inferring slug correctness by any other means.
func TestVL022_NoVerifiesFixture(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "no-verifies"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-022" {
			t.Fatalf("VL-022 fired on an attestation with no verifies edge at all (dc-4 scope limit): %s", f.String())
		}
	}
}

// vl022UndeclaredACMD verifies spec/vl-022-story (correctly slugged,
// class: story) but its own id names ac-99, which that story does not
// declare — isolating the "undeclared AC" refusal shape from the slug
// check (the slug segment here is deliberately correct).
const vl022UndeclaredACMD = `---
id: attestation/jira-vl022-1--ac-99
kind: attestation
title: "VL-022: id names an AC the target story does not declare"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-story" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022: id names an AC the target story does not declare

spec/vl-022-story declares only ac-1; this attestation's own id names
ac-99, which that story does not declare — VL-022 must refuse it, naming
the undeclared ac and the target.
`

// TestVL022_UndeclaredAC proves the "target's acceptance_criteria does not
// declare the AC named by the attestation's own id/path" refusal shape,
// isolated from the slug check (the directory here is the correct slug).
func TestVL022_UndeclaredAC(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/attestations/jira-vl022-1/ac-99.md", vl022UndeclaredACMD)
	// The ad hoc overlay needs vl-022-story's own spec present too — chain
	// the VL-022/story-only overlay (the spec alone, no attestations)
	// alongside this one-off attestation.
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "story-only"), dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-022" {
			found = true
			if !strings.Contains(f.Message, "ac-99") {
				t.Errorf("finding does not name the undeclared ac: %s", f.Message)
			}
			if !strings.Contains(f.Message, "spec/vl-022-story") {
				t.Errorf("finding does not name the target: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-022 did not fire on an attestation whose id names an undeclared AC:\n%s", findingsString(findings))
	}
}

// vl022FeatureOutcomeMD is a legitimate R4-I-11 feature-outcome attestation:
// its verifies edge targets the golden corpus's own spec/stale-decline, a
// class: feature spec. Per Controller adjudication ADJ-51 (2026-07-16),
// VL-022's subject is STORY-targeting attestations only (the D6-18 misfiling
// class the story exists to kill); a feature-outcome attestation is OUTSIDE
// that subject and must be SKIPPED, never refused. The 11 real, frozen
// feature-outcome attestations across this repo's own store
// (diagram-proposals, evidence-obligations, true-closure) and
// examples/showcase (jira-loan-1482, escrow-autopay) are exactly this shape —
// firing on them is what broke `make verify` and is precisely the defect the
// rescope removes.
const vl022FeatureOutcomeMD = `---
id: attestation/stale-decline--ac-1
kind: attestation
title: "AC-1 outcome attested: a feature-outcome attestation (R4-I-11)"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/stale-decline" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# AC-1 outcome attestation

A feature-outcome attestation verifying a class: feature spec — outside
VL-022's story-scoped subject (ADJ-51). VL-022 must SKIP it, never refuse.
`

// TestVL022_FeatureOutcomeSkipped is FIX 1's own witness (ADJ-51): a verifies
// edge to a class: feature spec is a feature-outcome attestation, OUTSIDE
// VL-022's story-scoped subject — SKIPPED, never refused, no baseline map.
// This replaces the pre-rescope TestVL022_WrongClass, whose assertion (a
// feature target REFUSED as "wrong class") is exactly the behavior the
// controller ruled out: the store's own real data (11 legitimate feature-
// outcome attestations) refutes the frozen dc-4 premise, so the rule's
// subject narrows to what it actually kills.
func TestVL022_FeatureOutcomeSkipped(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/attestations/stale-decline/ac-1.md", vl022FeatureOutcomeMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-022" {
			t.Fatalf("VL-022 fired on a feature-outcome attestation (ADJ-51: story-scoped subject, feature targets out of scope): %s", f.String())
		}
	}
}

// vl022MultiEdgeMD carries TWO verifies edges: the first (spec/vl-022-story)
// is clean — a class: story spec declaring ac-1 whose own story-ref slug
// (jira-vl022-1) matches this attestation's directory — and the second
// (spec/no-such-spec-at-all) is unresolvable. A hand-annotated attestation
// can carry more than one verifies edge (AttestationFrontmatter.Validate
// places no cardinality constraint), so VL-022 must validate EVERY edge, not
// break at the first (ADJ-51 finding 4).
const vl022MultiEdgeMD = `---
id: attestation/jira-vl022-1--ac-1
kind: attestation
title: "VL-022: two verifies edges, second one misfiled"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-story" }
  - { type: verifies, ref: "spec/no-such-spec-at-all" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022: multiple verifies edges

The first verifies edge is clean; the SECOND (spec/no-such-spec-at-all) is
unresolvable. VL-022 must fire on the second, proving it checks every edge.
`

// TestVL022_MultipleVerifiesEdges proves ADJ-51 finding 4: VL-022 validates
// every verifies edge, not only the first. Against the pre-fix rule (which
// broke at the first, clean edge) this fixture produced NO finding — its
// misfiled second edge folded silently; the fix makes it a named refusal.
func TestVL022_MultipleVerifiesEdges(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/attestations/jira-vl022-1/ac-1.md", vl022MultiEdgeMD)
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "story-only"), dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-022" {
			found = true
			if !strings.Contains(f.Message, "spec/no-such-spec-at-all") {
				t.Errorf("finding does not name the second (misfiled) verifies edge: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-022 did not fire on the SECOND verifies edge (checks only the first?):\n%s", findingsString(findings))
	}
}

// vl022UnresolvableMD verifies a spec that does not exist anywhere in the
// committed zone at all.
const vl022UnresolvableMD = `---
id: attestation/unresolvable-attempt--ac-1
kind: attestation
title: "VL-022: attestation verifies a spec that does not exist"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/no-such-spec-at-all" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022: attestation verifies a spec that does not exist
`

// TestVL022_UnresolvableTarget_FailsClosed proves the fail-closed posture
// for a verifies target that does not resolve at all (mirroring vl019's
// own "fail closed toward no-flip, never toward one"): VL-022 must still
// fire, never silently pass an attestation just because its target cannot
// even be loaded.
func TestVL022_UnresolvableTarget_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/attestations/unresolvable-attempt/ac-1.md", vl022UnresolvableMD)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-022" {
			found = true
			if !strings.Contains(f.Message, "spec/no-such-spec-at-all") {
				t.Errorf("finding does not name the offending target: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-022 did not fire on an unresolvable verifies target:\n%s", findingsString(findings))
	}
}

// vl022FragmentFormMD carries a fragment-bearing verifies edge — the
// closed spec-object edge vocabulary's own invalid form for a non-
// implements/resolves/supersedes/exempts/depends-on link (02 §Link
// taxonomy), which VL-003 independently rejects; this test checks VL-022's
// own presence, not rule exclusivity.
const vl022FragmentFormMD = `---
id: attestation/fragment-attempt--ac-1
kind: attestation
title: "VL-022: attestation verifies a fragment (the invalid form)"
owners: [platform-team]
links:
  - { type: verifies, ref: "spec/vl-022-story#ac-1" }
frozen: { at: 2026-07-16, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }
---
# VL-022: attestation verifies a fragment (the invalid form)

An attestation's verifies edge names the WHOLE spec (the AC lives in the
id and path); a fragment-bearing edge is never valid. VL-022 must refuse
it as not a whole spec ref.
`

// TestVL022_FragmentForm_FailsClosed proves a fragment-bearing verifies
// edge is refused as "not a whole spec ref" (mirroring vl019's own
// migration-guard test).
func TestVL022_FragmentForm_FailsClosed(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/attestations/fragment-attempt/ac-1.md", vl022FragmentFormMD)
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-022", "story-only"), dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	found := false
	for _, f := range findings {
		if f.Rule == "VL-022" {
			found = true
			if !strings.Contains(f.Message, "spec/vl-022-story#ac-1") {
				t.Errorf("finding does not name the offending target: %s", f.Message)
			}
			if !strings.Contains(f.Message, "whole") {
				t.Errorf("finding does not explain the whole-spec requirement: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatalf("VL-022 did not fire on a fragment-bearing verifies target:\n%s", findingsString(findings))
	}
}
