package lint

import (
	"path/filepath"
	"strings"
	"testing"
)

// vl004DraftOverlay is the shared testdata/violations/VL-004 fixture: a
// legacy `status: draft` feature spec at
// .verdi/specs/active/should-not-be-draft/spec.md. buildLintRepo commits
// every overlay directly onto the fixture repo's ONLY branch ("main"), so
// layering it in is itself "this spec's exact bytes are already on the
// default branch" — exactly Task 4's compatibility-disclosure case.
const vl004DraftOverlayDir = "VL-004"

// TestVL004_LegacyDraftAlreadyOnDefault_Discloses proves the core
// compatibility case (Task 4 step 4, first direction pairing): a legacy
// `status: draft` spec whose exact bytes are already reachable from the
// default branch is reported as merge-accepted with ONE SeverityDisclosure
// finding — never silently, and never as a plain violation (the design's
// "Artifact compatibility": "reported as merge-accepted with a
// compatibility disclosure rather than misrepresented as an active
// draft"). Both TargetsDefaultBoundary signals (linting the default branch
// itself; a CI run targeting it) reach the same result, since specstate's
// own Git-derived resolution is what actually decides the outcome, not
// which boundary signal tripped.
func TestVL004_LegacyDraftAlreadyOnDefault_Discloses(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
	}{
		{"linting the default branch itself", Context{DefaultBranch: "main", CurrentBranch: "main"}},
		{"CI run targeting the default branch", Context{DefaultBranch: "main", CurrentBranch: "feature/x", TargetBranch: "main", InCI: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI_DEFAULT_BRANCH", "main")
			repo := buildLintRepo(t, filepath.Join(violationsDir, vl004DraftOverlayDir))

			findings := runLint(t, repo.Dir, tc.ctx, Options{})
			onlyRule(t, findings, "VL-004")
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
			}
			f := findings[0]
			if f.Path != ".verdi/specs/active/should-not-be-draft/spec.md" {
				t.Fatalf("finding path = %q, want the draft overlay spec", f.Path)
			}
			if f.Severity != SeverityDisclosure {
				t.Fatalf("finding severity = %v, want SeverityDisclosure (never a verdict failure)", f.Severity)
			}
			if !strings.Contains(f.Message, "status: draft") || !strings.Contains(f.Message, "already reachable from the default branch") {
				t.Fatalf("finding message = %q, does not read as the migration-compatibility disclosure", f.Message)
			}
		})
	}
}

// TestVL004_StatuslessProposal_NoFinding proves the first brief direction:
// a NEW statusless proposal, linted as a CI run targeting the default
// branch, produces no VL-004 finding at all — status is optional now, and
// an omitted status carries no legacy-draft compatibility question in the
// first place.
func TestVL004_StatuslessProposal_NoFinding(t *testing.T) {
	const statuslessSpec = `---
id: spec/vl-004-statusless
kind: spec
class: feature
title: "VL-004: statusless proposal"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-004: statusless proposal
`
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-004-statusless/spec.md", statuslessSpec)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildLintRepo(t, dir)

	findings := runLint(t, repo.Dir, Context{DefaultBranch: "main", CurrentBranch: "feature/x", TargetBranch: "main", InCI: true}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-004" {
			t.Fatalf("VL-004 fired on a statusless proposal: %s", f.String())
		}
	}
}

// TestVL004_LegacyDraftOnlyOnDesignBranch_NoFinding proves the second
// brief direction: a legacy `status: draft` spec that exists ONLY on an
// unmerged design branch — never yet reachable from the default branch —
// produces no finding, even while CI is genuinely targeting the default
// branch (a real, still-under-review PR). Constructed with real git
// branching (unlike buildLintRepo's single-branch overlays) so specstate's
// own BlobAt(main, path) genuinely misses the file.
func TestVL004_LegacyDraftOnlyOnDesignBranch_NoFinding(t *testing.T) {
	repo := buildLintRepo(t)
	gitCheckoutNewBranch(t, repo.Dir, "design/should-not-be-draft")

	const draftSpec = `---
id: spec/should-not-be-draft-elsewhere
kind: spec
class: feature
title: "VL-004: draft only on the design branch"
status: draft
owners: [platform-team]
story: jira:LOAN-0003
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-004: draft only on the design branch
`
	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "should-not-be-draft-elsewhere", "spec.md")
	writeTestFile(t, specPath, draftSpec)
	commitAll(t, repo.Dir, "propose draft spec on the design branch")

	t.Setenv("CI_DEFAULT_BRANCH", "main")
	findings := runLint(t, repo.Dir, Context{DefaultBranch: "main", CurrentBranch: "design/should-not-be-draft", TargetBranch: "main", InCI: true}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-004" {
			t.Fatalf("VL-004 fired on a draft spec that never reached the default branch: %s", f.String())
		}
	}
}

// TestVL004_UnresolvableDefaultBranch_DisclosesInCIPath proves the review
// adjudication's binding fix: an unresolvable default branch, while a real
// CI run is targeting SOME branch (the PR-pipeline shape), must never fail
// the boundary check open silently — it discloses the missing proof
// instead of returning zero findings.
func TestVL004_UnresolvableDefaultBranch_DisclosesInCIPath(t *testing.T) {
	repo := buildLintRepo(t)
	// Deliberately no CI_DEFAULT_BRANCH: the fixture repo has no origin
	// remote at all, so specstate itself cannot resolve a default branch
	// either — this Context mirrors that same "can't prove it" state.
	findings := runLint(t, repo.Dir, Context{CurrentBranch: "feature/x", TargetBranch: "main", InCI: true}, Options{})
	onlyRule(t, findings, "VL-004")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	f := findings[0]
	if f.Severity != SeverityDisclosure {
		t.Fatalf("finding severity = %v, want SeverityDisclosure", f.Severity)
	}
	if !strings.Contains(f.Message, "no default branch could be resolved") {
		t.Fatalf("finding message = %q, want it to name the missing default-branch proof", f.Message)
	}
}

// TestVL004_UnresolvableDefaultBranch_SilentOutsideCI proves the fix is
// scoped to the CI-targeting path, not universal: an ordinary local lint
// run with no CI signal at all and an unresolvable default branch stays
// silent — matching every other git-aware rule's "can't prove it, don't
// guess" posture (e.g. VL-010's own empty-DiffBase silence) rather than
// disclosing on every plain local invocation.
func TestVL004_UnresolvableDefaultBranch_SilentOutsideCI(t *testing.T) {
	repo := buildLintRepo(t)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-004" {
			t.Fatalf("VL-004 fired outside CI with an unresolvable default branch: %s", f.String())
		}
	}
}

// TestVL004_OffDefaultBoundary_NoFinding proves the pre-existing I-14
// posture survives the rename verbatim: a resolvable default branch, but
// this run is neither on it nor a CI change targeting it, produces no
// finding at all — even over the same legacy-draft-on-default-branch
// overlay TestVL004_LegacyDraftAlreadyOnDefault_Discloses uses.
func TestVL004_OffDefaultBoundary_NoFinding(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildLintRepo(t, filepath.Join(violationsDir, vl004DraftOverlayDir))
	findings := runLint(t, repo.Dir, Context{DefaultBranch: "main", CurrentBranch: "feature/x"}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-004" {
			t.Fatalf("VL-004 fired off the default-branch boundary: %s", f.String())
		}
	}
}
