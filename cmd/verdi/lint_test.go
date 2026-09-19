package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/lint"
)

const lintTestManifest = `schema: verdi.layout/v1
forge: gitlab
lint:
  gated_generated: []
`

const lintTestGitattributes = `.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

const lintTestCleanADR = `---
id: adr/0001-example
kind: adr
title: "Example ADR"
status: proposed
owners: [platform-team]
---
# Example ADR
`

const lintTestBadADR = `---
id: adr/0002-bad
kind: adr
title: "Bad ADR"
status: proposed
owners: [platform-team]
bogus_field: nope
---
# Bad ADR
`

func buildMinimalStore(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	// The fixture repo has no origin, so an ambient CI environment (a
	// pull_request runner's CI/GITHUB_ACTIONS/GITHUB_BASE_REF) would make
	// VL-004 truthfully disclose the RUNNER's unresolvable target branch on
	// this clean store. Tests that exercise CI on purpose set their own
	// values after building the fixture.
	clearCIEnv(t)
	all := map[string]string{
		".verdi/verdi.yaml": lintTestManifest,
		".gitattributes":    lintTestGitattributes,
	}
	for k, v := range files {
		all[k] = v
	}
	return fixturegit.Build(t, []fixturegit.Layer{{Files: all, Message: "minimal store"}})
}

// TestRunLintVerb_CleanExitsZero proves a clean store prints nothing and
// exits 0.
func TestRunLintVerb_CleanExitsZero(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{".verdi/adr/0001-example.md": lintTestCleanADR})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runLintVerb exit = %d, want 0; stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on a clean store", stdout.String())
	}
}

// TestRunLintVerb_FindingsExitOne proves a store with a violation prints a
// "message (path) [VL-xxx]" line (R-W4-5's sentence-first grammar) to
// stdout and exits 1.
func TestRunLintVerb_FindingsExitOne(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{
		".verdi/adr/0001-example.md": lintTestCleanADR,
		".verdi/adr/0002-bad.md":     lintTestBadADR,
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runLintVerb exit = %d, want 1; stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "VL-001") || !strings.Contains(stdout.String(), "0002-bad.md") {
		t.Fatalf("stdout = %q, want a VL-001 line naming 0002-bad.md", stdout.String())
	}
}

// lintTestNewClassFeature is a minimal, otherwise-clean round-four
// `class: feature` spec — a new-class spec (isNewClassSpec) that VL-017
// scopes its disclosed-unproven report to. Draft status avoids the freeze/
// predecessor rules; problem/outcome/one outcome-AC keep it valid. ac-1
// declares attestation (L-M14 remedy 1, internal/lint/vl006.go's
// checkFeatureACAttestation) — unfrozen, so not otherwise grandfathered.
const lintTestNewClassFeature = `---
id: spec/borrower-update
kind: spec
class: feature
title: "Borrower update"
status: draft
owners: [platform-team]
problem: { text: "the update API has no PUT route", anchor: "#problem" }
outcome: { text: "a borrower can update their application", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can update their application", evidence: [behavioral, attestation], anchor: "#ac-1" }
---
# Borrower update

## Problem

The update API has no PUT route.

## Outcome

A borrower can update their application.

## AC-1

A borrower can update their application.
`

// TestRunLintVerb_VL017DisclosureOnly_ExitsZero is the M-1 adjudication: a
// bare CI clone (no data/mutable/, per 01 §Zones — fixturegit never commits
// it) of a repo carrying a new-class spec reports VL-017 disclosed-unproven.
// That report is printed (never silent) but is NOT a verdict failure, so a
// run whose only findings are disclosures exits 0 — CI stays green once a
// new-class spec exists (adjudicated at W2 wave close).
func TestRunLintVerb_VL017DisclosureOnly_ExitsZero(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{
		".verdi/specs/active/borrower-update/spec.md": lintTestNewClassFeature,
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runLintVerb exit = %d, want 0 (VL-017 disclosure is not a verdict failure); stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "disclosed-unproven [lint:VL-017]") {
		t.Fatalf("stdout = %q, want a printed \"disclosed-unproven [lint:VL-017]\" disclosure line (never silent)", stdout.String())
	}
}

// lintTestNewClassFeatureTwo is a second, independent new-class feature
// spec — distinct from lintTestNewClassFeature only by id/dir/title/prose
// — that TestRunLintVerb_VL017Disclosure_PrintedOncePerRun needs to prove
// ac-3's "once per run" collapse against MORE THAN ONE affected spec: a
// single-spec fixture cannot distinguish "printed once" from "printed
// once per spec, and this store happens to have exactly one spec."
const lintTestNewClassFeatureTwo = `---
id: spec/borrower-update-two
kind: spec
class: feature
title: "Borrower update two"
status: draft
owners: [platform-team]
problem: { text: "the update API has no PATCH route", anchor: "#problem" }
outcome: { text: "a borrower can partially update their application", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a borrower can partially update their application", evidence: [behavioral, attestation], anchor: "#ac-1" }
---
# Borrower update two

## Problem

The update API has no PATCH route.

## Outcome

A borrower can partially update their application.

## AC-1

A borrower can partially update their application.
`

// TestRunLintVerb_VL017Disclosure_PrintedOncePerRun is spec/uat-round-1
// ac-3's CLI exerciser, closing UAT-002: "on a normal local checkout with
// N specs the CLI prints the [VL-017] paragraph N times." On a checkout
// with the mutable zone absent and TWO applicable specs, `verdi lint`
// must print VL-017's disclosure exactly ONCE, naming both affected
// specs, describe the condition as the mutable zone being absent from
// this checkout (never committed, 01 §Zones), and never assert a bare
// clone — while still exiting 0 (disclosure is not a verdict failure,
// co-2/ac-3 unchanged).
func TestRunLintVerb_VL017Disclosure_PrintedOncePerRun(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{
		".verdi/specs/active/borrower-update/spec.md":     lintTestNewClassFeature,
		".verdi/specs/active/borrower-update-two/spec.md": lintTestNewClassFeatureTwo,
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runLintVerb exit = %d, want 0 (VL-017 disclosure is not a verdict failure); stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if n := strings.Count(out, "disclosed-unproven [lint:VL-017]"); n != 1 {
		t.Fatalf("stdout printed the VL-017 disclosure %d time(s), want exactly 1 (ac-3, closing UAT-002):\n%s", n, out)
	}
	for _, path := range []string{
		".verdi/specs/active/borrower-update/spec.md",
		".verdi/specs/active/borrower-update-two/spec.md",
	} {
		if !strings.Contains(out, path) {
			t.Fatalf("stdout missing affected spec path %q in the once-per-run disclosure:\n%s", path, out)
		}
	}
	if strings.Contains(out, "bare clone") {
		t.Fatalf("stdout must not assert a bare clone (ac-3):\n%s", out)
	}
	if !strings.Contains(out, "mutable zone (.verdi/data/mutable/) is absent from this checkout") {
		t.Fatalf("stdout must describe the mutable-zone-absent condition (ac-3):\n%s", out)
	}
	if !strings.Contains(out, "never committed") || !strings.Contains(out, "01 §Zones") {
		t.Fatalf("stdout must cite the never-committed fact and 01 §Zones (ac-3):\n%s", out)
	}
}

// TestRunLintVerb_ViolationAlongsideVL017Disclosures_ExitsOne is the
// mixed-severity path the two single-severity tests above leave open: a
// store that carries BOTH a real violation and VL-017's once-per-run
// disclosure. It pins the two halves of the exit contract against each
// other in one run — the collapse must not swallow, reorder away or mask
// the violation (exit stays 1, CLAUDE.md's verdict exit), and the
// violation must not suppress the disclosure or restore its per-spec
// repetition (printed, exactly once, naming both affected specs —
// spec/uat-round-1 ac-3 and co-5: a quieter disclosure is still a
// disclosure).
func TestRunLintVerb_ViolationAlongsideVL017Disclosures_ExitsOne(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{
		".verdi/adr/0002-bad.md":                          lintTestBadADR,
		".verdi/specs/active/borrower-update/spec.md":     lintTestNewClassFeature,
		".verdi/specs/active/borrower-update-two/spec.md": lintTestNewClassFeatureTwo,
	})
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runLintVerb exit = %d, want 1 (a real violation is present); stdout=%q stderr=%q", got, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "VL-001") || !strings.Contains(out, "0002-bad.md") {
		t.Fatalf("stdout = %q, want the VL-001 violation line naming 0002-bad.md printed alongside the disclosure", out)
	}
	if n := strings.Count(out, "disclosed-unproven [lint:VL-017]"); n != 1 {
		t.Fatalf("stdout printed the VL-017 disclosure %d time(s), want exactly 1 (ac-3, closing UAT-002):\n%s", n, out)
	}
	for _, path := range []string{
		".verdi/specs/active/borrower-update/spec.md",
		".verdi/specs/active/borrower-update-two/spec.md",
	} {
		if !strings.Contains(out, path) {
			t.Fatalf("stdout missing affected spec path %q in the once-per-run disclosure:\n%s", path, out)
		}
	}
}

// TestRunLintVerb_NoStoreRoot_ExitTwo proves an operational failure (no
// store root findable) exits 2 with a stderr message, not a Finding.
func TestRunLintVerb_NoStoreRoot_ExitTwo(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	got := runLintVerb(nil, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runLintVerb exit = %d, want 2", got)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr empty, want an operational error message")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
}

// TestBuildLintContext_NoRemote_UnknownDefaultBranch proves the common
// local-fixture case (no configured "origin" remote, not running in CI)
// leaves DefaultBranch/DiffBase unknown rather than guessing.
func TestBuildLintContext_NoRemote_UnknownDefaultBranch(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{".verdi/adr/0001-example.md": lintTestCleanADR})

	for _, k := range []string{"CI", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_ACTIONS", "GITHUB_BASE_REF"} {
		t.Setenv(k, "")
	}

	lctx := lint.BuildContext(t.Context(), repo.Dir)
	if lctx.DefaultBranch != "" {
		t.Fatalf("DefaultBranch = %q, want empty (no origin remote configured)", lctx.DefaultBranch)
	}
	if lctx.DiffBase != "" {
		t.Fatalf("DiffBase = %q, want empty (no default branch to merge-base against)", lctx.DiffBase)
	}
	if lctx.CurrentBranch != "main" {
		t.Fatalf("CurrentBranch = %q, want %q", lctx.CurrentBranch, "main")
	}
}

// TestBuildLintContext_CIEnv proves CI-declared branch names flow through.
func TestBuildLintContext_CIEnv(t *testing.T) {
	repo := buildMinimalStore(t, map[string]string{".verdi/adr/0001-example.md": lintTestCleanADR})

	t.Setenv("CI", "true")
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Setenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "main")

	lctx := lint.BuildContext(t.Context(), repo.Dir)
	if !lctx.InCI {
		t.Fatal("InCI = false, want true")
	}
	if lctx.DefaultBranch != "main" {
		t.Fatalf("DefaultBranch = %q, want main", lctx.DefaultBranch)
	}
	if lctx.TargetBranch != "main" {
		t.Fatalf("TargetBranch = %q, want main", lctx.TargetBranch)
	}
	// DefaultBranch ("main") is a real, resolvable ref in this repo (the
	// fixturegit-built repo's own branch), so merge-base(HEAD, main)
	// resolves to HEAD itself.
	if lctx.DiffBase != repo.Head {
		t.Fatalf("DiffBase = %q, want %q (merge-base(HEAD, main) == HEAD)", lctx.DiffBase, repo.Head)
	}
}
