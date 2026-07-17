package main

// This file covers spec/close-preflight's story-scope obligations:
// ac-1--behavioral (exact condition/kind/path disclosure, table-driven per
// defect class), ac-2--behavioral (the exit-code matrix and non-mutation,
// including the CI-guard disclosure clause), and ac-3--behavioral (the
// agreement property: each defect-class fixture drives BOTH --preflight
// and a real, unmodified `verdi close` in the same test body, on the
// byte-identical fixture, asserting they refuse for the exact same
// reason). Feature-scope obligations are covered in
// closepreflightfeature_test.go.
//
// Every test below calls runPreflight (this story's testable core,
// mirroring close_test.go's own runClose convention) directly over a
// fixturegit store — never a subprocess exec, never Playwright, no network
// (co-1).

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// preflightFixtureSpecMD declares THREE evidence kinds on its one AC
// (static, behavioral, attestation) — unlike buildCloseFixtureRepo's own
// fixture (close_test.go, [static, behavioral] only), this fixture can
// exercise the attestation absent/unauthored disclosure (dc-7) alongside
// the other two kinds, so a single fixture family drives every ac-1 defect
// class this file needs.
const preflightFixtureSpecMD = `---
id: spec/preflight-fixture
kind: spec
class: story
title: "Preflight fixture story"
status: accepted-pending-build
owners: [platform-team]
story: jira:PREFLIGHT-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [static, behavioral, attestation] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Preflight fixture story
## Problem
x
## Outcome
y
`

// buildPreflightFixtureRepo builds the shared base fixture: the loan-mgmt
// feature plus preflightFixtureSpecMD implementing it. Each subtest below
// seeds (or withholds) evidence/attestations/reports/forge state on top of
// this same base to produce exactly one defect class at a time.
func buildPreflightFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                             "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md":         featureV1SpecMD,
			".verdi/specs/active/preflight-fixture/spec.md": preflightFixtureSpecMD,
		},
		Message: "preflight fixture: feature + story declaring static+behavioral+attestation",
	}})
}

const preflightStoryRef = "spec/preflight-fixture"

// preflightStorySlug is store.RefSlug("jira:PREFLIGHT-1") — computed, never
// hand-typed, so this test file can never silently drift from the real
// slugging convention (mirroring the ac-1--attestation obligation's own
// "never a hand-typed string literal" concern).
func preflightStorySlug() string { return store.RefSlug("jira:PREFLIGHT-1") }

// preflightAttestationPath is the exact relative path preflight's own
// disclosure must name for the fixture's one AC.
func preflightAttestationPath() string {
	return filepath.ToSlash(filepath.Join(".verdi", "attestations", preflightStorySlug(), "ac-1.md"))
}

// preflightDerivedRoot is the exact relative derived-tree root preflight's
// own disclosure must name.
func preflightDerivedRoot() string {
	return filepath.ToSlash(filepath.Join(".verdi", "data", "derived", store.RefSlug(preflightStoryRef))) + "/"
}

func writePreflightAttestation(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", preflightStorySlug())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ac-1.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const preflightUnauthoredAttestationMD = `---
id: attestation/jira-preflight-1--ac-1
kind: attestation
title: "unauthored attestation scaffold: jira:PREFLIGHT-1 ac-1"
owners: [platform-team]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/preflight-fixture" }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + ` }
---
<!-- verdi:attestation-unauthored -->
This attestation was scaffolded by ` + "`verdi attest`" + ` for jira:PREFLIGHT-1 ac-1
and has not been authored.
`

const preflightAuthoredAttestationMD = `---
id: attestation/jira-preflight-1--ac-1
kind: attestation
title: "ac-1"
owners: [platform-team]
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + ` }
---
# ac-1
Verified by hand, per the fixture's own test narrative.
`

// writePreflightGateReport writes deviation-report.md directly into the
// preflight-fixture spec's own directory — writeGateReport (gate_test.go)
// hardcodes "stale-decline" (that file's own fixture family), so this
// story's differently-named fixture needs its own copy of the same
// plain-write shape (never git-committed, read via os.ReadFile exactly as
// a real `verdi align` run before its own commit would leave it).
func writePreflightGateReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "preflight-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

// snapshotRepo captures the working tree's full observable state — HEAD,
// current branch, every local branch, and the porcelain status — so a
// before/after comparison proves byte-for-byte non-mutation (ac-2): no
// branch created, no file written, no commit made, no ref moved.
func snapshotRepo(t *testing.T, dir string) string {
	t.Helper()
	return "HEAD=" + gitOutput(t, dir, "rev-parse", "HEAD") +
		"branch=" + gitOutput(t, dir, "symbolic-ref", "--short", "HEAD") +
		"branches=" + gitOutput(t, dir, "branch", "--list") +
		"status=" + gitOutput(t, dir, "status", "--porcelain")
}

// erroringOpenMRsForge wraps a *forgefake.Forge, overriding ListOpenMRs to
// return a transport error — dc-5's "a forge that IS configured/reachable
// but genuinely errors when called is operational" case. Hermetic: no
// network, internal/forge/fake simply has no built-in error-injection
// knob, so this is the small test-local double that adds one.
type erroringOpenMRsForge struct {
	*forgefake.Forge
}

func (erroringOpenMRsForge) ListOpenMRs(context.Context, string) ([]forge.OpenMR, error) {
	return nil, fmt.Errorf("fake: injected transport error listing open MRs")
}

var _ forge.Forge = erroringOpenMRsForge{}

// TestRunPreflight_StoryScope_DefectClasses is ac-1--behavioral's and
// ac-3--behavioral's combined exerciser: one subtest per defect class named
// in ac-1 (no-signal, pending, violated, spec-stale, pending-supersession),
// each building a fixture with exactly that one defect, running
// --preflight over it (asserting the exact condition/kind/path disclosure),
// then a real, unmodified verdi close on the byte-identical, still-
// unmutated fixture (asserting its refusal reason matches, not merely that
// it also fails) — never two independently hand-asserted expectations.
func TestRunPreflight_StoryScope_DefectClasses(t *testing.T) {
	ctx := context.Background()

	t.Run("no-signal: no evidence at all", func(t *testing.T) {
		repo := buildPreflightFixtureRepo(t)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		wantSubstrs := []string{
			"[FAIL] closure: 1. story eligible",
			"ac-1 static: no current passing record; derived-tree root probed: " + preflightDerivedRoot(),
			"ac-1 behavioral: no current passing record; derived-tree root probed: " + preflightDerivedRoot(),
			"ac-1 attestation: no file at " + preflightAttestationPath() + "; scaffold it with `verdi attest`",
		}
		for _, want := range wantSubstrs {
			if !strings.Contains(pstdout.String(), want) {
				t.Fatalf("preflight stdout missing %q:\n%s", want, pstdout.String())
			}
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "[FAIL] closure: 1. story eligible") {
			t.Fatalf("close stdout missing the SAME eligibility FAIL line preflight showed: %s", cstdout.String())
		}
	})

	t.Run("pending: some but not all declared kinds satisfied", func(t *testing.T) {
		repo := buildPreflightFixtureRepo(t)
		writeFixtureVerdicts(t, repo.Dir, preflightStoryRef, repo.Head, featureFixtureEvidenceJSON("ac-1", "static", "pass", repo.Head))
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if strings.Contains(pstdout.String(), "ac-1 static: no current passing record") {
			t.Fatalf("stdout wrongly names static as missing when a passing record exists:\n%s", pstdout.String())
		}
		for _, want := range []string{
			"[FAIL] closure: 1. story eligible",
			"ac-1 behavioral: no current passing record; derived-tree root probed: " + preflightDerivedRoot(),
			"ac-1 attestation: no file at " + preflightAttestationPath(),
		} {
			if !strings.Contains(pstdout.String(), want) {
				t.Fatalf("preflight stdout missing %q:\n%s", want, pstdout.String())
			}
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "[FAIL] closure: 1. story eligible") {
			t.Fatalf("close stdout missing the SAME eligibility FAIL line preflight showed: %s", cstdout.String())
		}
	})

	t.Run("violated: a failing current record", func(t *testing.T) {
		repo := buildPreflightFixtureRepo(t)
		writeFixtureVerdicts(t, repo.Dir, preflightStoryRef, repo.Head,
			featureFixtureEvidenceJSON("ac-1", "static", "fail", repo.Head),
			featureFixtureEvidenceJSON("ac-1", "behavioral", "pass", repo.Head),
		)
		writePreflightAttestation(t, repo.Dir, preflightAuthoredAttestationMD)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure: 1. story eligible") {
			t.Fatalf("preflight stdout missing eligibility FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "ac-1 static: no current passing record; derived-tree root probed: "+preflightDerivedRoot()) {
			t.Fatalf("preflight stdout should name static as the unsatisfied (failing) kind:\n%s", pstdout.String())
		}
		if strings.Contains(pstdout.String(), "ac-1 behavioral:") || strings.Contains(pstdout.String(), "ac-1 attestation:") {
			t.Fatalf("preflight stdout should NOT name behavioral/attestation as missing when both are satisfied:\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "[FAIL] closure: 1. story eligible") {
			t.Fatalf("close stdout missing the SAME eligibility FAIL line preflight showed: %s", cstdout.String())
		}
	})

	t.Run("spec-stale: own-text accepted-deviation finding", func(t *testing.T) {
		repo := buildPreflightFixtureRepo(t)
		writeFixtureVerdicts(t, repo.Dir, preflightStoryRef, repo.Head,
			featureFixtureEvidenceJSON("ac-1", "static", "pass", repo.Head),
			featureFixtureEvidenceJSON("ac-1", "behavioral", "pass", repo.Head),
		)
		writePreflightAttestation(t, repo.Dir, preflightAuthoredAttestationMD)
		writePreflightGateReport(t, repo.Dir, repo.Head, `  - { id: ac-1, kind: computed, text: "targets the AC's own declared text", disposition: accepted-deviation, note: "known drift" }
`)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if !strings.Contains(pstdout.String(), "[PASS] closure: 1. story eligible") {
			t.Fatalf("preflight stdout should show condition 1 PASS (fully evidenced):\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure: 2. no unresolved spec-stale flag") {
			t.Fatalf("preflight stdout missing spec-stale FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "spec-stale: own-text finding(s) [ac-1]") {
			t.Fatalf("preflight stdout missing the own-text finding id:\n%s", pstdout.String())
		}
		wantReportPath := filepath.ToSlash(filepath.Join(".verdi", "specs", "active", "preflight-fixture", "deviation-report.md"))
		if !strings.Contains(pstdout.String(), "deviation-report.md: "+wantReportPath) {
			t.Fatalf("preflight stdout missing the deviation-report.md path %q:\n%s", wantReportPath, pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "spec-stale: own-text finding(s) [ac-1]") {
			t.Fatalf("close stdout missing the SAME spec-stale reason preflight showed: %s", cstdout.String())
		}
	})

	t.Run("pending-supersession: an open MR touches an implemented object", func(t *testing.T) {
		// CI_DEFAULT_BRANCH pins lint.ResolveDefaultBranch's resolution to
		// "main" deterministically (the fixturegit repo carries no "origin"
		// remote for the git-plumbing fallback to discover) — matching the
		// target branch the open MR below is seeded against, exactly as a
		// real CI job's own env would.
		t.Setenv("CI_DEFAULT_BRANCH", "main")

		repo := buildPreflightFixtureRepo(t)
		writeFixtureVerdicts(t, repo.Dir, preflightStoryRef, repo.Head,
			featureFixtureEvidenceJSON("ac-1", "static", "pass", repo.Head),
			featureFixtureEvidenceJSON("ac-1", "behavioral", "pass", repo.Head),
		)
		writePreflightAttestation(t, repo.Dir, preflightAuthoredAttestationMD)

		fg := forgefake.New()
		fg.SeedOpenMR("main", forge.OpenMR{ID: "77", SourceBranch: "supersede-loan-mgmt", Title: "supersede loan-mgmt"})
		fg.SeedFile("supersede-loan-mgmt", ".verdi/specs/active/loan-mgmt-v2/spec.md",
			[]byte(featureV2SpecMD(`supersession:
  amended:
    - { id: ac-1, note: "corrected" }
`)))

		before := snapshotRepo(t, repo.Dir)
		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, fg, true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		if !strings.Contains(pstdout.String(), "[FAIL] closure: 3. no unresolved pending-supersession flag") {
			t.Fatalf("preflight stdout missing pending-supersession FAIL:\n%s", pstdout.String())
		}
		if !strings.Contains(pstdout.String(), "open supersession MR(s) [77] touch object(s) [ac-1]") {
			t.Fatalf("preflight stdout missing the MR id / touched object id:\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: fg, Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
		if !strings.Contains(cstdout.String(), "open supersession MR(s) [77] touch object(s) [ac-1]") {
			t.Fatalf("close stdout missing the SAME MR/object ids preflight showed: %s", cstdout.String())
		}
	})

	t.Run("unauthored attestation scaffold names the same path, distinctly from absent", func(t *testing.T) {
		repo := buildPreflightFixtureRepo(t)
		writeFixtureVerdicts(t, repo.Dir, preflightStoryRef, repo.Head,
			featureFixtureEvidenceJSON("ac-1", "static", "pass", repo.Head),
			featureFixtureEvidenceJSON("ac-1", "behavioral", "pass", repo.Head),
		)
		writePreflightAttestation(t, repo.Dir, preflightUnauthoredAttestationMD)
		before := snapshotRepo(t, repo.Dir)

		var pstdout, pstderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
		}
		want := "ac-1 attestation: a scaffold is present at " + preflightAttestationPath() + " but the claim is unauthored (sentinel present); author it"
		if !strings.Contains(pstdout.String(), want) {
			t.Fatalf("preflight stdout missing the unauthored-scaffold disclosure %q:\n%s", want, pstdout.String())
		}
		if strings.Contains(pstdout.String(), "scaffold it with `verdi attest`") {
			t.Fatalf("preflight stdout must distinguish unauthored from absent, not print the absent remedy:\n%s", pstdout.String())
		}

		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}

		var cstdout, cstderr bytes.Buffer
		gotClose := runClose(ctx, repo.Dir, preflightStoryRef, &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New()}, &cstdout, &cstderr)
		if gotClose != 1 {
			t.Fatalf("runClose = %d, want 1 (unauthored scaffold must not satisfy the fold); stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
		}
	})
}

// TestRunPreflight_StoryScope_ReadyThenClose is ac-3--behavioral's second
// half: a fixture with every condition satisfied reports ready
// (--preflight exit 0, no unmet conditions printed) and a subsequent real,
// unmodified verdi close on that same fixture succeeds (exit 0), actually
// archiving the quartet.
func TestRunPreflight_StoryScope_ReadyThenClose(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}

	before := snapshotRepo(t, repo.Dir)
	var pstdout, pstderr bytes.Buffer
	rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), true, &pstdout, &pstderr)
	if rc != 0 {
		t.Fatalf("runPreflight(ready) = %d, want 0; stdout=%s stderr=%s", rc, pstdout.String(), pstderr.String())
	}
	if strings.Contains(pstdout.String(), "[FAIL]") {
		t.Fatalf("ready preflight should show no FAIL condition:\n%s", pstdout.String())
	}
	if strings.Contains(pstdout.String(), "missing-evidence detail:") {
		t.Fatalf("ready preflight should print no missing-evidence detail:\n%s", pstdout.String())
	}
	if !strings.Contains(pstdout.String(), "READY") {
		t.Fatalf("ready preflight should say READY:\n%s", pstdout.String())
	}
	after := snapshotRepo(t, repo.Dir)
	if before != after {
		t.Fatalf("--preflight(ready) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
	}

	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}
	var cstdout, cstderr bytes.Buffer
	gotClose := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &cstdout, &cstderr)
	if gotClose != 0 {
		t.Fatalf("runClose(ready) = %d, want 0; stdout=%s stderr=%s", gotClose, cstdout.String(), cstderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture", "spec.md")); err != nil {
		t.Fatalf("real close should have archived the quartet after a READY preflight: %v", err)
	}
}

// TestRunPreflight_ExitCodeMatrixAndNonMutation is ac-2--behavioral's
// exerciser: three fixtures (ready/unmet/a genuine operational error) drive
// exit 0/1/2 respectively, each snapshotted before and after and asserted
// byte-identical.
func TestRunPreflight_ExitCodeMatrixAndNonMutation(t *testing.T) {
	ctx := context.Background()

	t.Run("ready: exit 0, no mutation", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
		if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
			t.Fatalf("produceSelfHostedEvidence: %v", err)
		}
		before := snapshotRepo(t, repo.Dir)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), true, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runPreflight(ready) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight(ready) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("unmet: exit 1, no mutation", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		before := snapshotRepo(t, repo.Dir)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), true, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPreflight(no evidence) = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight(unmet) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("operational: malformed derived record, no mutation", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		dir := filepath.Join(repo.Dir, ".verdi", "data", "derived", store.RefSlug("spec/close-fixture"), repo.Head)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte("not valid json"), 0o644); err != nil {
			t.Fatal(err)
		}
		before := snapshotRepo(t, repo.Dir)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), true, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("runPreflight(malformed derived record) = %d, want 2; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight(operational error) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("operational: forge transport error listing open MRs, no mutation", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		before := snapshotRepo(t, repo.Dir)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, erroringOpenMRsForge{forgefake.New()}, true, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("runPreflight(forge transport error) = %d, want 2; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		after := snapshotRepo(t, repo.Dir)
		if before != after {
			t.Fatalf("--preflight(forge error) mutated the repo:\nbefore: %s\nafter:  %s", before, after)
		}
	})
}

// ciEnvVars are every environment variable lint.ReadCIEnv/ResolveDefaultBranch
// reads — cleared in every CI-guard test, mirroring TestCmdClose_RefusesOutsideCI's
// own defensive-clearing convention (close_test.go), so an ambient CI
// environment running this very test suite can never leak in.
var ciEnvVars = []string{"CI", "GITHUB_ACTIONS", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF"}

func clearCIEnv(t *testing.T) {
	t.Helper()
	for _, v := range ciEnvVars {
		t.Setenv(v, "")
	}
}

// readyCloseFixtureRepo builds and fully evidences buildCloseFixtureRepo's
// fixture (close_test.go) — a ready-to-close story, reused here since the
// CI-guard clause fires regardless of the gate's own verdict and a ready
// fixture is the least noisy backdrop to isolate it against.
func readyCloseFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := buildCloseFixtureRepo(t)
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}
	return repo
}

// TestRunPreflight_CIGuardDisclosure is dc-1's added clause: outside a
// detected CI environment and without --force-local, --preflight discloses
// — once, informationally — that a real close would separately refuse at
// the CI-only publish guard, regardless of the gate verdict; the same run
// under CI, or with --force-local, prints no such line.
func TestRunPreflight_CIGuardDisclosure(t *testing.T) {
	ctx := context.Background()

	t.Run("outside CI, no --force-local: guard disclosure printed", func(t *testing.T) {
		clearCIEnv(t)
		repo := readyCloseFixtureRepo(t)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), false, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runPreflight = %d, want 0 (ready); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "disclosed-unproven [close:preflight-publish-guard]") {
			t.Fatalf("stdout should print the CI-publish-guard disclosure: %s", stdout.String())
		}
	})

	t.Run("CI simulated: guard disclosure NOT printed", func(t *testing.T) {
		clearCIEnv(t)
		t.Setenv("CI", "true")
		repo := readyCloseFixtureRepo(t)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), false, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runPreflight = %d, want 0 (ready); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "preflight-publish-guard") {
			t.Fatalf("stdout should NOT print the guard disclosure when CI is detected: %s", stdout.String())
		}
	})

	t.Run("--force-local: guard disclosure NOT printed", func(t *testing.T) {
		clearCIEnv(t)
		repo := readyCloseFixtureRepo(t)
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), true, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runPreflight = %d, want 0 (ready); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "preflight-publish-guard") {
			t.Fatalf("stdout should NOT print the guard disclosure when --force-local is set: %s", stdout.String())
		}
	})

	t.Run("guard disclosure fires regardless of the gate's own verdict (unmet fixture)", func(t *testing.T) {
		clearCIEnv(t)
		repo := buildCloseFixtureRepo(t) // no evidence produced: gate is unmet
		var stdout, stderr bytes.Buffer
		rc := runPreflight(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, forgefake.New(), false, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPreflight = %d, want 1 (unmet); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "disclosed-unproven [close:preflight-publish-guard]") {
			t.Fatalf("stdout should still print the CI-publish-guard disclosure on an unmet gate: %s", stdout.String())
		}
	})
}

// TestPreflightGuardDisclosure_AgreesWithRealGuard is dc-1's own follow-on
// fix: the disclosure's condition must be read from the identical boolean
// evaluation the real guard performs — proven by driving BOTH the guard's
// own refusal (a real, non---preflight close outside CI without
// --force-local) and closePublishGuardRefuses (the one predicate --preflight's
// disclosure and cmdClose's own guard both call) from the identical
// environment/flag setup in one test, asserting they agree — never two
// independently hand-asserted booleans that could drift apart.
func TestPreflightGuardDisclosure_AgreesWithRealGuard(t *testing.T) {
	clearCIEnv(t)
	t.Chdir(t.TempDir())

	var cstdout, cstderr bytes.Buffer
	rc := cmdClose([]string{"jira:LOAN-1482"}, &cstdout, &cstderr)
	if rc != 2 {
		t.Fatalf("cmdClose outside CI = %d, want 2 (guard refusal)", rc)
	}
	guardRefused := strings.Contains(cstderr.String(), "refusing to publish outside CI")
	if !guardRefused {
		t.Fatalf("test setup bug: the real guard should have refused outside CI without --force-local: stderr=%s", cstderr.String())
	}

	if got := closePublishGuardRefuses(false); got != guardRefused {
		t.Fatalf("closePublishGuardRefuses(false) = %v, want %v (must agree with the real guard's own refusal under the identical inputs)", got, guardRefused)
	}

	// And the flip side: --force-local makes neither refuse.
	var fstdout, fstderr bytes.Buffer
	rcForceLocal := cmdClose([]string{"jira:LOAN-1482", "--force-local"}, &fstdout, &fstderr)
	forceLocalGuardRefused := strings.Contains(fstderr.String(), "refusing to publish outside CI")
	_ = rcForceLocal // --force-local still exits 2 further down (no store root); only the guard clause itself is under test here
	if forceLocalGuardRefused {
		t.Fatalf("--force-local should not hit the guard refusal: stderr=%s", fstderr.String())
	}
	if got := closePublishGuardRefuses(true); got != forceLocalGuardRefused {
		t.Fatalf("closePublishGuardRefuses(true) = %v, want %v", got, forceLocalGuardRefused)
	}
}

// TestCmdClose_Preflight_Dispatch covers cmdClose's own --preflight
// argument-parsing and dispatch: order-independence with the story arg and
// with --force-local (dc-1), running outside CI without ever reaching the
// CI-only refusal or printing the --force-local escape-hatch warning
// (ac-2's obligation), and the updated usage text.
func TestCmdClose_Preflight_Dispatch(t *testing.T) {
	clearCIEnv(t)

	buildReadyDir := func(t *testing.T) string {
		t.Helper()
		repo := buildCloseFixtureRepo(t)
		prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
		if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
			t.Fatalf("produceSelfHostedEvidence: %v", err)
		}
		return repo.Dir
	}

	t.Run("--preflight before the story arg", func(t *testing.T) {
		t.Chdir(buildReadyDir(t))
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--preflight", "spec/close-fixture"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(--preflight, ready) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
	})

	t.Run("--preflight after the story arg", func(t *testing.T) {
		t.Chdir(buildReadyDir(t))
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"spec/close-fixture", "--preflight"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(story, --preflight) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
	})

	t.Run("--preflight runs outside CI without --force-local and never hits the publish guard", func(t *testing.T) {
		t.Chdir(buildReadyDir(t))
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--preflight", "spec/close-fixture"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(--preflight) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "refusing to publish outside CI") {
			t.Fatalf("--preflight must never hit the CI-only publish-guard refusal: stderr=%s", stderr.String())
		}
		if strings.Contains(stdout.String(), "NON-AUTHORITATIVE") || strings.Contains(stderr.String(), "NON-AUTHORITATIVE") {
			t.Fatalf("--preflight must never print the --force-local escape-hatch warning text: stdout=%s stderr=%s", stdout.String(), stderr.String())
		}
	})

	t.Run("--preflight + --force-local coexist, order-independent, suppressing the guard disclosure", func(t *testing.T) {
		t.Chdir(buildReadyDir(t))
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--force-local", "--preflight", "spec/close-fixture"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(--force-local, --preflight) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), "preflight-publish-guard") {
			t.Fatalf("--force-local should suppress the guard disclosure: %s", stdout.String())
		}
	})

	t.Run("usage text names --preflight", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		rc := cmdClose(nil, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("cmdClose(no args) = %d, want 2", rc)
		}
		if !strings.Contains(stderr.String(), "--preflight") {
			t.Fatalf("usage text should name --preflight: %s", stderr.String())
		}
	})

	t.Run("--preflight with no store root is operational", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--preflight", "jira:LOAN-1482"}, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("cmdClose(--preflight, no store) = %d, want 2", rc)
		}
	})

	t.Run("--preflight with an unresolvable ref is operational", func(t *testing.T) {
		t.Chdir(buildReadyDir(t))
		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--preflight", "spec/does-not-exist"}, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("cmdClose(--preflight, unresolvable spec) = %d, want 2; stderr=%s", rc, stderr.String())
		}
	})
}
