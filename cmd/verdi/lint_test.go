package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
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
// "VL-xxx path: message" line to stdout and exits 1.
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
// predecessor rules; problem/outcome/one outcome-AC keep it valid.
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
  - { id: ac-1, text: "a borrower can update their application", evidence: [behavioral], anchor: "#ac-1" }
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
	if !strings.Contains(stdout.String(), "notice: VL-017") {
		t.Fatalf("stdout = %q, want a printed \"notice: VL-017\" disclosure line (never silent)", stdout.String())
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

	lctx := buildLintContext(t.Context(), repo.Dir)
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

	lctx := buildLintContext(t.Context(), repo.Dir)
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
