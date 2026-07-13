package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
)

// featureV1SpecMD is the never-touched predecessor; featureV2SpecMD is its
// superseding revision, classifying ac-1 per each subtest's needs (a
// %s placeholder for the supersession: block body).
const featureV1SpecMD = `---
id: spec/loan-mgmt
kind: spec
title: "Loan management"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static, attestation] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# Loan management
## Problem
x
## Outcome
y
`

func featureV2SpecMD(supersessionBlock string) string {
	return fmt.Sprintf(`---
id: spec/loan-mgmt-v2
kind: spec
title: "Loan management"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds, corrected", evidence: [static, attestation] }
links:
  - { type: supersedes, ref: "spec/loan-mgmt" }
%s
frozen: { at: 2024-02-01, commit: 0000000000000000000000000000000000000b }
---
# Loan management v2
## Problem
x
## Outcome
y
`, supersessionBlock)
}

func storySpecForCascade(t *testing.T) *artifact.SpecFrontmatter {
	t.Helper()
	return &artifact.SpecFrontmatter{
		Base: artifact.Base{
			ID:     "spec/stale-decline-story",
			Kind:   artifact.KindSpec,
			Title:  "Stale Decline",
			Owners: []string{"platform-team"},
			Links:  []artifact.Link{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}},
		},
		Class:   artifact.ClassStory,
		Status:  "accepted-pending-build",
		Story:   "jira:LOAN-1482",
		Problem: &artifact.Attribute{Text: "x", Anchor: "problem"},
		Outcome: &artifact.Attribute{Text: "y", Anchor: "outcome"},
		AcceptanceCriteria: []artifact.AcceptanceCriterion{
			{ID: "ac-1", Text: "static obligation holds", Evidence: []artifact.EvidenceKind{artifact.EvidenceStatic}},
		},
	}
}

// TestCheckCascadeReaffirmation covers 03 §The amendment ladder rung 4's
// three verdicts and the re-affirmation resolution path.
func TestCheckCascadeReaffirmation(t *testing.T) {
	t.Run("no merged supersession at all: unaffected", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                     phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md": featureV1SpecMD,
			},
			Message: "no supersession",
		}})
		ok, reason, err := checkCascadeReaffirmation(repo.Dir, storySpecForCascade(t))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("checkCascadeReaffirmation() ok=false (%s), want true", reason)
		}
	})

	t.Run("carried: unaffected", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md":    featureV1SpecMD,
				".verdi/specs/active/loan-mgmt-v2/spec.md": featureV2SpecMD("supersession:\n  carried: [ac-1]"),
			},
			Message: "carried supersession",
		}})
		ok, reason, err := checkCascadeReaffirmation(repo.Dir, storySpecForCascade(t))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("checkCascadeReaffirmation() ok=false (%s), want true (carried)", reason)
		}
	})

	t.Run("amended, no re-affirmation: blocked (stale)", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md":    featureV1SpecMD,
				".verdi/specs/active/loan-mgmt-v2/spec.md": featureV2SpecMD("supersession:\n  amended:\n    - { id: ac-1, note: \"corrected\" }"),
			},
			Message: "amended supersession, no reaffirmation",
		}})
		ok, reason, err := checkCascadeReaffirmation(repo.Dir, storySpecForCascade(t))
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("checkCascadeReaffirmation() ok=true, want false (stale, missing re-affirmation)")
		}
		if !contains(reason, "stale") || !contains(reason, "re-affirmation") {
			t.Fatalf("reason = %q, want it to name the stale/re-affirmation block", reason)
		}
	})

	t.Run("amended, with re-affirmation: unblocked", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md":        featureV1SpecMD,
				".verdi/specs/active/loan-mgmt-v2/spec.md":     featureV2SpecMD("supersession:\n  amended:\n    - { id: ac-1, note: \"corrected\" }"),
				".verdi/reaffirmations/jira-loan-1482/ac-1.md": reaffirmationMD(t),
			},
			Message: "amended supersession, with reaffirmation",
		}})
		ok, reason, err := checkCascadeReaffirmation(repo.Dir, storySpecForCascade(t))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("checkCascadeReaffirmation() ok=false (%s), want true (reaffirmed)", reason)
		}
	})

	t.Run("removed: blocked (invalidated), no resolution path", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md":    featureV1SpecMD,
				".verdi/specs/active/loan-mgmt-v2/spec.md": featureV2SpecMD("supersession:\n  removed:\n    - { id: ac-1, note: \"dropped\" }"),
			},
			Message: "removed supersession",
		}})
		ok, reason, err := checkCascadeReaffirmation(repo.Dir, storySpecForCascade(t))
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("checkCascadeReaffirmation() ok=true, want false (invalidated)")
		}
		if !contains(reason, "invalidated") {
			t.Fatalf("reason = %q, want it to name the invalidated block", reason)
		}
	})

	t.Run("no implements edges at all: trivially unaffected", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml": phase7ManifestYAML,
			},
			Message: "empty store",
		}})
		spec := storySpecForCascade(t)
		spec.Links = nil
		ok, _, err := checkCascadeReaffirmation(repo.Dir, spec)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("checkCascadeReaffirmation() ok=false, want true (no implements edges)")
		}
	})
}

func reaffirmationMD(t *testing.T) string {
	t.Helper()
	return `---
id: reaffirmation/jira-loan-1482--ac-1
kind: reaffirmation
title: "Re-affirm ac-1"
owners: [platform-team]
object: "spec/loan-mgmt-v2@0000000000000000000000000000000000000b#ac-1"
hash: { old: "sha256:0000000000000000000000000000000000000000000000000000000000000000", new: "sha256:1111111111111111111111111111111111111111111111111111111111111111" }
frozen: { at: 2024-02-02, commit: 0000000000000000000000000000000000000c }
---
# Re-affirmation
`
}

// TestGate_Condition4_CascadeBlock proves runGate's condition 4 fails
// closed on a cascade-stale story missing a re-affirmation, and passes
// once the re-affirmation record is added — exercised through the full
// gate entry point (not just the shared checkCascadeReaffirmation helper
// already covered above), proving the wiring in gate.go itself.
func TestGate_Condition4_CascadeBlock(t *testing.T) {
	// Spec name "stale-decline" (not "-story") deliberately matches
	// writeGateReport's own hardcoded deviation-report.md path
	// (gate_test.go) — reused unchanged rather than parameterizing a
	// shared test helper this phase does not otherwise need to touch.
	specMD := `---
id: spec/stale-decline
kind: spec
class: story
title: "Stale decline story"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# body
## Problem
x
## Outcome
y
`
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/stale-decline/spec.md": specMD,
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
			".verdi/specs/active/loan-mgmt-v2/spec.md":  featureV2SpecMD("supersession:\n  amended:\n    - { id: ac-1, note: \"corrected\" }"),
		},
		Message: "scaffold + cascade-stale story",
	}})
	checkoutBranch(t, repo.Dir, "feature/stale-decline")
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	spec, _ := readSpec(t, repo.Dir, "stale-decline")

	ctx := context.Background()
	var stdout, stderr bytes.Buffer
	got := runGate(ctx, repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runGate (cascade-stale, no reaffirmation) = %d, want 1; stdout=%s", got, stdout.String())
	}
	assertConditionFails(t, stdout.String(), 4)

	// Add the re-affirmation record and re-run: gate should now pass.
	reaffDir := filepath.Join(repo.Dir, ".verdi", "reaffirmations", store.RefSlug("jira:LOAN-1482"))
	if err := os.MkdirAll(reaffDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(reaffDir, "ac-1.md"), []byte(reaffirmationMD(t)), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	got = runGate(ctx, repo.Dir, spec, repo.Head, "main", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runGate (cascade-stale, reaffirmed) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	assertConditionPasses(t, stdout.String(), 4)
}
