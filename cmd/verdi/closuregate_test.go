package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
)

const closureGateStorySpecMD = `---
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
  - { id: ac-1, text: "static obligation holds", evidence: [attestation] }
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

func buildClosureGateRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/stale-decline/spec.md": closureGateStorySpecMD,
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
		},
		Message: "closure gate fixture",
	}})
	checkoutBranch(t, repo.Dir, "feature/stale-decline")
	return repo
}

func seedAttestation(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", "jira-loan-1482")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: attestation/jira-loan-1482--ac-1\nkind: attestation\ntitle: \"ac-1\"\nowners: [platform-team]\nfrozen: { at: 2024-01-01, commit: " + gateFakeFrozenCommit + " }\n---\n# ac-1\n"
	if err := os.WriteFile(filepath.Join(dir, "ac-1.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunClosureGate_EligibleCondition proves the closure gate's condition
// 1: not eligible without the attestation, eligible with it.
func TestRunClosureGate_EligibleCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	t.Run("no attestation: not eligible", func(t *testing.T) {
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("runClosureGate() = true, want false (no attestation, not eligible)")
		}
		if !contains(stdout.String(), "[FAIL] closure: 1.") {
			t.Fatalf("stdout = %q, want condition 1 to FAIL", stdout.String())
		}
	})

	t.Run("attestation present: eligible, closure gate passes", func(t *testing.T) {
		seedAttestation(t, repo.Dir)
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("runClosureGate() = false, want true; stdout=%s", stdout.String())
		}
	})
}

// TestRunClosureGate_SpecStaleCondition proves the closure gate blocks on
// an unresolved spec-stale flag (03 §The amendment ladder's rung-arbitrage
// counter-pressure) and passes once no such flag is raised.
func TestRunClosureGate_SpecStaleCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	// Trigger (a): an accepted-deviation finding whose id equals the
	// story's own declared AC id (R4-I-18's operationalization).
	writeGateReport(t, repo.Dir, repo.Head, `  - { id: ac-1, kind: computed, text: "targets the AC's own declared text", disposition: accepted-deviation, note: "known drift" }
`)

	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, repo.Head, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("runClosureGate() = true, want false (spec-stale, own-text trigger)")
	}
	if !contains(stdout.String(), "[FAIL] closure: 2.") {
		t.Fatalf("stdout = %q, want condition 2 to FAIL", stdout.String())
	}
}

// TestRunClosureGate_PendingSupersessionCondition proves the exit
// criterion verbatim: "a pending-supersession flag blocks verdi close but
// not verdi build start/verdi gate while the manifest MR is open." An open
// (unmerged) supersession MR is visible only through the forge port —
// checkPendingSupersessionCondition (closuregate.go) is the only place
// this phase reads it; build start and gate (cascadecheck.go) never
// consult the forge at all, so they cannot be affected by it — that
// asymmetry, not a runtime check, is what proves the second half of this
// exit criterion.
func TestRunClosureGate_PendingSupersessionCondition(t *testing.T) {
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir)
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	fakeForge := forgefake.New()
	fakeForge.SeedOpenMR("main", forge.OpenMR{ID: "42", SourceBranch: "supersede-loan-mgmt", Title: "supersede loan-mgmt"})
	fakeForge.SeedFile("supersede-loan-mgmt", ".verdi/specs/active/loan-mgmt-v2/spec.md",
		[]byte(featureV2SpecMD("supersession:\n  amended:\n    - { id: ac-1, note: \"corrected\" }")))

	// 1. The closure gate is blocked while the MR is open.
	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, repo.Head, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("runClosureGate() = true, want false (pending-supersession, open MR); stdout=%s", stdout.String())
	}
	if !contains(stdout.String(), "[FAIL] closure: 3.") {
		t.Fatalf("stdout = %q, want condition 3 to FAIL", stdout.String())
	}

	// 2. verdi build start is NOT blocked — it never reads the forge, only
	// merged (local) supersessions, and none exists here.
	buildDeps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	fresh := freshClosureGateRepoForBuildStart(t)
	var bstdout, bstderr bytes.Buffer
	got := runBuildStart(context.Background(), fresh.Dir, "spec/stale-decline", buildDeps, &bstdout, &bstderr)
	if got != 0 {
		t.Fatalf("runBuildStart (pending-supersession only, not merged) = %d, want 0; stderr=%s", got, bstderr.String())
	}

	// 3. verdi gate is NOT blocked either, for the same reason (condition 4
	// only ever consults local, merged specs/active/ — cascadecheck.go).
	var gstdout, gstderr bytes.Buffer
	gotGate := runGate(ctx, repo.Dir, spec, repo.Head, "main", &gstdout, &gstderr)
	if gotGate != 0 {
		t.Fatalf("runGate (pending-supersession only, not merged) = %d, want 0; stdout=%s stderr=%s", gotGate, gstdout.String(), gstderr.String())
	}
}

// TestRunClosureGate_PendingSupersessionDisclosedUnproven proves the
// three-valued honesty fix (constitution 2/10): when the story implements a
// feature but the open-MR input is unavailable (nil/unreachable forge), the
// pending-supersession condition is reported disclosed-unproven — rendered
// through the shared internal/disclosure seam (spec/disclosure-seam-v2,
// ac-1), never a silent pass — while a reachable forge that finds no open
// supersession MR passes the condition outright.
func TestRunClosureGate_PendingSupersessionDisclosedUnproven(t *testing.T) {
	ctx := context.Background()

	t.Run("nil forge: disclosed-unproven notice, not a silent pass", func(t *testing.T) {
		repo := buildClosureGateRepo(t)
		seedAttestation(t, repo.Dir)
		spec, _ := readSpec(t, repo.Dir, "stale-decline")

		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		// Disclosure is not failure: eligible + not spec-stale + disclosed
		// pending-supersession still leaves the gate un-failed.
		if !ok {
			t.Fatalf("runClosureGate() = false, want true (disclosure is not failure); stdout=%s", stdout.String())
		}
		if !contains(stdout.String(), "closure: disclosed-unproven [gate:pending-supersession]:") {
			t.Fatalf("stdout = %q, want condition 3 disclosed through the shared internal/disclosure rendering, never a silent pass", stdout.String())
		}
		if contains(stdout.String(), "[PASS] closure: 3.") {
			t.Fatalf("stdout = %q, condition 3 must NOT silently pass on a nil forge", stdout.String())
		}
	})

	t.Run("reachable forge, no open MR: condition 3 passes", func(t *testing.T) {
		repo := buildClosureGateRepo(t)
		seedAttestation(t, repo.Dir)
		spec, _ := readSpec(t, repo.Dir, "stale-decline")

		fakeForge := forgefake.New() // no seeded open MRs
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, repo.Head, &stdout)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("runClosureGate() = false, want true (no open supersession MR); stdout=%s", stdout.String())
		}
		if !contains(stdout.String(), "[PASS] closure: 3.") {
			t.Fatalf("stdout = %q, want condition 3 to PASS with a reachable forge and no open MR", stdout.String())
		}
	})
}

// freshClosureGateRepoForBuildStart builds a repo whose story spec is
// still status: draft-free accepted-pending-build with NO build branch cut
// yet, isolated from buildClosureGateRepo's own repo (which already sits
// on feature/stale-decline) so runBuildStart can cut the branch cleanly.
func freshClosureGateRepoForBuildStart(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/stale-decline/spec.md": closureGateStorySpecMD,
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
		},
		Message: "closure gate fixture, no build branch yet",
	}})
}
