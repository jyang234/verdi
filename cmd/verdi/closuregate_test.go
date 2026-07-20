package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
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
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
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
		// Condition 4 (X-13/X-16/X-17): a living, fully-dispositioned report
		// already covering head, so the gate genuinely holds overall.
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
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
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
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
	ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, nil, repo.Head, &stdout)
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
	gotGate := runGate(ctx, repo.Dir, spec, repo.Head, "main", nil, &gstdout, &gstderr)
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
		// Condition 4 (X-13/X-16/X-17): so the disclosure on condition 3 is
		// the only thing keeping this gate from a full PASS.
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
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
		writeGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

		fakeForge := forgefake.New() // no seeded open MRs
		var stdout bytes.Buffer
		ok, err := runClosureGate(ctx, repo.Dir, spec, fakeForge, "main", nil, nil, repo.Head, &stdout)
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

// TestRunClosureGate_DispositionCompleteCondition is X-13/X-16/X-17's
// static register for the STORY closure gate's condition 4: every failure
// shape (no report at all — X-17's literal scenario; a stale-covers
// report; an undispositioned finding — X-13's literal scenario) refuses,
// naming the offenders and the closure ritual; a report that covers head
// with every finding dispositioned passes (D6-24: the freeze-in-place
// case must still hold). Mirrors gate_test.go's own
// TestGate_Condition3_FailsAlone in shape — the merge gate's condition 3
// and this closure-gate condition share the same underlying facts, just
// different remedy text.
func TestRunClosureGate_DispositionCompleteCondition(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root, head string)
		wantOK     bool
		wantSubstr []string
	}{
		{
			name:       "no report at all (X-17)",
			setup:      func(t *testing.T, root, head string) {},
			wantOK:     false,
			wantSubstr: []string{"no deviation-report.md found at", "the closure ritual is align"},
		},
		{
			name: "stale covers",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, "0000000000000000000000000000000000000b", dispositionedFindingYAML)
			},
			wantOK:     false,
			wantSubstr: []string{"covers 0000000000000000000000000000000000000b, not head", "the closure ritual is align"},
		},
		{
			name: "undispositioned finding (X-13)",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, head, undispositionedFindingYAML)
			},
			wantOK:     false,
			wantSubstr: []string{"undispositioned finding(s) [f-1]", "the closure ritual is align"},
		},
		{
			name: "fresh, fully dispositioned (D6-24: freeze-in-place still holds)",
			setup: func(t *testing.T, root, head string) {
				writeGateReport(t, root, head, dispositionedFindingYAML)
			},
			wantOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildClosureGateRepo(t)
			seedAttestation(t, repo.Dir) // condition 1 holds regardless
			spec, _ := readSpec(t, repo.Dir, "stale-decline")
			tc.setup(t, repo.Dir, repo.Head)

			var stdout bytes.Buffer
			ok, err := runClosureGate(context.Background(), repo.Dir, spec, forgefake.New(), "main", nil, nil, repo.Head, &stdout)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.wantOK {
				t.Fatalf("runClosureGate() = %v, want %v; stdout=%s", ok, tc.wantOK, stdout.String())
			}
			if tc.wantOK {
				if !contains(stdout.String(), "[PASS] closure: 4.") {
					t.Fatalf("stdout = %q, want condition 4 to PASS", stdout.String())
				}
				return
			}
			if !contains(stdout.String(), "[FAIL] closure: 4.") {
				t.Fatalf("stdout = %q, want condition 4 to FAIL", stdout.String())
			}
			for _, want := range tc.wantSubstr {
				if !contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), want)
				}
			}
		})
	}
}

// TestRunClosureGate_UnreadableAttestation_OperationalFailure pins ADJ-67 /
// D6-38 on the STORY closure gate. The round replaced the fold's stat-only
// AttestationExists with content-reading LoadAttestationState. On an
// attestation file that exists but cannot be read (mode 000), the old
// stat-only swallow returned (true, nil) — silently counting an unreadable
// file as a satisfied HUMAN attestation, so the gate computed a verdict (exit
// 0/1). The kept behavior propagates the os.ReadFile error out of Fold,
// through foldStoryEvidence's "folding evidence:" wrap and
// checkClosureEligible's "closure gate:" wrap, as an operational failure —
// exit 2 at the cmd level. This test asserts BOTH taxonomy views (the
// gate-function error path, matching this file's other runClosureGate tests,
// AND the cmd-level exit-2) and must FAIL if anyone restores the swallow.
func TestRunClosureGate_UnreadableAttestation_OperationalFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("DISCLOSURE: running as root — os.Chmod(0o000) does not restrict root's own reads, so this permission-based negative test cannot exercise the unreadable-attestation path under this user")
	}
	repo := buildClosureGateRepo(t)
	seedAttestation(t, repo.Dir) // authored attestation at attestations/jira-loan-1482/ac-1.md
	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	ctx := context.Background()

	attPath := filepath.Join(repo.Dir, ".verdi", "attestations", "jira-loan-1482", "ac-1.md")
	if err := os.Chmod(attPath, 0o000); err != nil {
		t.Fatalf("os.Chmod(%s, 0o000): %v", attPath, err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(attPath, 0o644) // restore so t.TempDir()'s own cleanup can remove it
	})

	// Gate-function taxonomy: a non-nil "closure gate:"-wrapped error, ok
	// false — never a swallowed (true, nil) eligible verdict.
	var stdout bytes.Buffer
	ok, err := runClosureGate(ctx, repo.Dir, spec, nil, "main", nil, nil, repo.Head, &stdout)
	if err == nil {
		t.Fatalf("runClosureGate(unreadable attestation) err = nil (ok=%v) — an unreadable attestation must fail closed, never swallow to a satisfied attestation (ADJ-67/D6-38); stdout=%s", ok, stdout.String())
	}
	if ok {
		t.Fatalf("runClosureGate(unreadable attestation) ok = true, want false on an operational failure")
	}
	if !contains(err.Error(), "closure gate:") {
		t.Fatalf("err = %q, want the closure-gate-wrapped error path (closuregate.go's checkClosureEligible)", err.Error())
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("err = %v, want it to wrap os.ErrPermission (the propagated os.ReadFile EACCES)", err)
	}

	// Cmd-level taxonomy: the same input drives `verdi close` to exit 2
	// (operational) — never 0 (clean) or 1 (a business-precondition refusal).
	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New()}
	var cstdout, cstderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/stale-decline", &store.Manifest{}, deps, &cstdout, &cstderr)
	if got != 2 {
		t.Fatalf("runClose(story, unreadable attestation) = %d, want 2 (operational); stdout=%s stderr=%s", got, cstdout.String(), cstderr.String())
	}
	if !contains(cstderr.String(), "loading attestation state") {
		t.Fatalf("stderr = %q, want it to name the propagated attestation read error", cstderr.String())
	}
}
