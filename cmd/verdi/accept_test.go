package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/lint"
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
	if got := runDesignStart(ctx, repo.Dir, "jira:LOAN-1482", "stale-decline", manifest, deps, &stdout, &stderr); got != 0 {
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
	if len(findings) != 0 {
		t.Fatalf("accepted spec has %d lint findings, want 0: %+v", len(findings), findings)
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
