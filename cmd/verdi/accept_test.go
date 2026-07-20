package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
)

// scaffoldAndDesign builds a fresh Phase 7 repo and runs design start on
// it (jira:LOAN-1482, --name stale-decline), returning the repo and the
// design branch's commit right after the scaffold (the pre-flip HEAD
// `accept` is expected to capture).
func scaffoldAndDesign(t *testing.T) (repo *fixturegit.Repo, preFlipHead string) {
	t.Helper()
	repo = buildPhase7Repo(t)
	ctx := context.Background()
	manifest := phase7Manifest(t)
	deps := designDeps{Provider: seedFakeProvider(t), Runner: nil, GoTest: fakeGoTest{}}

	var stdout, stderr bytes.Buffer
	if got := runDesignStart(ctx, repo.Dir, artifact.ClassFeature, "jira:LOAN-1482", "stale-decline", manifest, phase7Model(t), deps, &stdout, &stderr); got != 0 {
		t.Fatalf("runDesignStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	return repo, head
}

// TestRunAccept_Happy proves the mechanical flip: status changes, the
// frozen stamp's commit equals the pre-flip HEAD, its at date equals that
// commit's own committer date (never wall clock), and the flip itself is
// committed.
//
// guide-claim: 7.1-accept-freeze-obligations
func TestRunAccept_Happy(t *testing.T) {
	repo, preFlipHead := scaffoldAndDesign(t)
	ctx := context.Background()

	wantDate, err := gitx.CommitDate(ctx, repo.Dir, preFlipHead)
	if err != nil {
		t.Fatalf("CommitDate: %v", err)
	}
	wantAt := wantDate[:10]

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	spec, _ := readSpec(t, repo.Dir, "stale-decline")
	if spec.Status != "accepted-pending-build" {
		t.Fatalf("spec.Status = %q, want accepted-pending-build", spec.Status)
	}
	if spec.Frozen == nil {
		t.Fatal("spec.Frozen is nil, want a frozen stamp")
	}
	if spec.Frozen.Commit != preFlipHead {
		t.Fatalf("spec.Frozen.Commit = %q, want the pre-flip HEAD %q", spec.Frozen.Commit, preFlipHead)
	}
	if spec.Frozen.At != wantAt {
		t.Fatalf("spec.Frozen.At = %q, want the pre-flip commit's own date %q", spec.Frozen.At, wantAt)
	}

	newHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse(HEAD): %v", err)
	}
	if newHead == preFlipHead {
		t.Fatal("accept did not create a new commit for the flip")
	}
	// The flip's parent must be exactly the captured pre-flip HEAD — the
	// commit frozen.commit names really is "the content-final sha it
	// supersedes" (03 §Lifecycle), not some other ancestor.
	ok, err := gitx.IsAncestor(ctx, repo.Dir, preFlipHead, newHead)
	if err != nil || !ok {
		t.Fatalf("pre-flip HEAD %q is not an ancestor of the flip commit %q (err=%v)", preFlipHead, newHead, err)
	}
}

// TestRunAccept_PassesLintEngineInProcess proves the accepted spec is
// lint-clean, run in-process against internal/lint's real engine (VL-004
// is default-branch-scoped, so being on the design branch is fine; VL-009
// and VL-010 are exercised directly by giving the lint Context real
// git-derived facts).
func TestRunAccept_PassesLintEngineInProcess(t *testing.T) {
	repo, _ := scaffoldAndDesign(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	lctx := lint.Context{
		DefaultBranch: "main",
		CurrentBranch: "design/stale-decline",
		DiffBase:      repo.Head, // the store's init commit, common ancestor of main and the design branch
	}
	findings, err := lint.NewEngine().Run(ctx, repo.Dir, lctx, lint.Options{})
	if err != nil {
		t.Fatalf("lint.Run: %v", err)
	}
	// Only verdict-failure findings flip a real lint run's exit code
	// (lint/finding.go: SeverityDisclosure "is NOT a verdict failure ...
	// a clean run carrying only disclosures still exits 0"). The
	// round-four scaffold (design.go) is genuinely a new-class spec now
	// (it carries problem/outcome/stubs), so VL-017's disclosed-unproven
	// notice legitimately fires here (no data/mutable/ in this bare
	// fixturegit repo) — a printed disclosure, not a violation.
	var violations []lint.Finding
	for _, f := range findings {
		if f.Severity != lint.SeverityDisclosure {
			violations = append(violations, f)
		}
	}
	if len(violations) != 0 {
		t.Fatalf("accepted spec has %d lint violations, want 0: %+v", len(violations), violations)
	}
}

// TestRunAccept_StagesOnlyItsOwnPaths is D6-33's regression test: an
// unrelated untracked scratch file AND an unrelated modified TRACKED file
// sitting in the same checkout when accept runs must both stay out of the
// accept commit entirely — accept.go's own gitx.AddAll (`git add -A`) swept
// exactly this shape of unrelated content into two independent acceptance
// agents' commits in the same round-6 wave (round6-divergences.md D6-33).
func TestRunAccept_StagesOnlyItsOwnPaths(t *testing.T) {
	repo, _ := scaffoldAndDesign(t)
	ctx := context.Background()

	// An unrelated untracked scratch file (mirrors the real witness: a
	// leftover `./verdi-bin` build artifact never `git add`ed).
	scratchPath := filepath.Join(repo.Dir, "verdi-bin")
	if err := os.WriteFile(scratchPath, []byte("not a real binary\n"), 0o644); err != nil {
		t.Fatalf("writing scratch file: %v", err)
	}

	// An unrelated MODIFIED tracked file, left unstaged (never `git add`ed
	// either) — a second, independent shape of "something else in the
	// checkout accept must not sweep up".
	manifestPath := filepath.Join(repo.Dir, ".verdi", "verdi.yaml")
	modifiedManifest := phase7ManifestYAML + "# an in-progress, unrelated local edit\n"
	if err := os.WriteFile(manifestPath, []byte(modifiedManifest), 0o644); err != nil {
		t.Fatalf("writing modified manifest: %v", err)
	}

	beforeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if got := runAccept(ctx, repo.Dir, "spec/stale-decline", &stdout, &stderr); got != 0 {
		t.Fatalf("runAccept = %d, want 0; stderr=%s", got, stderr.String())
	}

	afterHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if afterHead == beforeHead {
		t.Fatal("accept did not create a new commit")
	}

	// The accept commit's tree contains ONLY the accepted spec.md — never
	// the scratch file, never the manifest's local edit.
	entries, err := gitx.DiffNameStatus(ctx, repo.Dir, beforeHead, afterHead)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Path != ".verdi/specs/active/stale-decline/spec.md" {
		t.Fatalf("accept commit's diff = %+v, want exactly one entry for the accepted spec.md", entries)
	}
	for _, e := range entries {
		if e.Path == "verdi-bin" || e.Path == ".verdi/verdi.yaml" {
			t.Fatalf("accept commit's diff = %+v, must not contain the unrelated scratch file or manifest edit", entries)
		}
	}

	// Belt and suspenders: the manifest's committed blob at the new HEAD is
	// still the ORIGINAL content — the local edit truly never entered the
	// tree, not merely "not listed as changed" by some diff quirk.
	committedManifest, err := gitx.Show(ctx, repo.Dir, afterHead, ".verdi/verdi.yaml")
	if err != nil {
		t.Fatalf("Show(afterHead, verdi.yaml): %v", err)
	}
	if string(committedManifest) != phase7ManifestYAML {
		t.Fatalf("committed .verdi/verdi.yaml diverged from the original — the local edit leaked into the commit:\n%s", committedManifest)
	}

	// The scratch file and the manifest's local edit are both still exactly
	// as this test left them: accept truly left them alone, rather than
	// e.g. staging-but-not-committing them.
	if got, err := os.ReadFile(scratchPath); err != nil || string(got) != "not a real binary\n" {
		t.Fatalf("scratch file after accept = %q, err=%v; want untouched", got, err)
	}
	if got, err := os.ReadFile(manifestPath); err != nil || string(got) != modifiedManifest {
		t.Fatalf("working-tree manifest after accept = %q, err=%v; want untouched (still carrying the local edit)", got, err)
	}
}

const componentSpecMD = `---
id: spec/some-component
kind: spec
title: "Some component"
owners: [platform-team]
class: component
status: active
---
# Some component
`

const alreadyAcceptedSpecMD = `---
id: spec/already-accepted
kind: spec
title: "Already accepted"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:LOAN-2000
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [static] }
frozen: { at: 2026-01-01, commit: deadbeefdeadbeefdeadbeefdeadbeefdeadbeef }
---
# Already accepted
`

func buildAcceptNegativeRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                            phase7ManifestYAML,
				".verdi/specs/active/some-component/spec.md":   componentSpecMD,
				".verdi/specs/active/already-accepted/spec.md": alreadyAcceptedSpecMD,
			},
			Message: "init store with pre-existing specs",
		},
	})
}

// TestRunAccept_RefusesComponentSpec proves accept refuses a component
// spec (no story, no acceptance criteria — never accept-able) with exit 1,
// a verdict failure, leaving the file untouched.
func TestRunAccept_RefusesComponentSpec(t *testing.T) {
	repo := buildAcceptNegativeRepo(t)
	ctx := context.Background()

	before, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/some-component", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAccept(component spec) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "component") {
		t.Fatalf("stderr = %q, want it to name the component-spec refusal", stderr.String())
	}

	after, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("a refused accept must not create a commit")
	}
}

// TestRunAccept_RefusesNonDraft proves accept refuses a spec that is
// already accepted-pending-build (or any non-draft status), with exit 1.
func TestRunAccept_RefusesNonDraft(t *testing.T) {
	repo := buildAcceptNegativeRepo(t)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runAccept(ctx, repo.Dir, "spec/already-accepted", &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAccept(non-draft spec) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "not draft") {
		t.Fatalf("stderr = %q, want it to name the non-draft refusal", stderr.String())
	}
}

// TestRunAccept_Negative covers runAccept's own operational-error paths.
func TestRunAccept_Negative(t *testing.T) {
	ctx := context.Background()

	t.Run("not a spec ref", func(t *testing.T) {
		repo := buildAcceptNegativeRepo(t)
		var stdout, stderr bytes.Buffer
		got := runAccept(ctx, repo.Dir, "jira:LOAN-1482", &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runAccept(story ref) = %d, want 2", got)
		}
	})

	t.Run("no such spec", func(t *testing.T) {
		repo := buildAcceptNegativeRepo(t)
		var stdout, stderr bytes.Buffer
		got := runAccept(ctx, repo.Dir, "spec/does-not-exist", &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runAccept(missing spec) = %d, want 2", got)
		}
	})
}

// TestCmdAccept_UsageNegative proves cmdAccept's own argument-count check.
func TestCmdAccept_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdAccept(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdAccept(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdAccept([]string{"spec/a", "spec/b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdAccept(two args) = %d, want 2", got)
	}
}

// TestRun_AcceptDispatchesToRealVerb proves dispatch.go routes "accept" to
// the real implementation.
func TestRun_AcceptDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"accept", "spec/x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([accept spec/x]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
