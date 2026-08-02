package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

const featureAliasFrozenCommit = "0000000000000000000000000000000000000a"

// featureAliasStorySpecMD renders a class: story spec.md at the given
// status — the story-grade unit `verdi build start`/`verdi feature start`
// (the deprecation alias) actually operate on post-round-four (05 §CLI:
// "build start ... fails unless the story's spec is accepted-pending-build").
// It carries no implements-target feature on disk (the cascade check,
// cascadecheck.go, tolerates a dangling implements target — findSupersedingSpec
// simply finds nothing to fold against), keeping this fixture minimal and
// focused on the deprecation-alias/status-precondition behavior under test.
func featureAliasStorySpecMD(status string) string {
	frozen := ""
	if status == "accepted-pending-build" {
		frozen = fmt.Sprintf("\nfrozen: { at: 2024-01-01, commit: %s }", featureAliasFrozenCommit)
	}
	return fmt.Sprintf(`---
id: spec/stale-decline
kind: spec
title: "Stale decline handling"
owners: [platform-team]
class: story
status: %s
story: jira:LOAN-1482
problem: { text: "borrowers see stale decline data", anchor: problem }
outcome: { text: "borrowers see current decline data", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }%s
---
# Stale decline handling

## Problem
x

## Outcome
x
`, status, frozen)
}

func buildFeatureAliasRepo(t *testing.T, status string) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         phase7ManifestYAML,
				".verdi/specs/active/stale-decline/spec.md": featureAliasStorySpecMD(status),
			},
			Message: "init store with story spec at " + status,
		},
	})
	// The build-start acceptance precondition now routes through
	// internal/specstate's real, git-backed projector (Task 5), which
	// resolves the default branch through its own precedence chain rather
	// than trusting any caller-passed ref; fixturegit repos carry no origin
	// remote, so every test here that needs the precondition to actually
	// resolve pins it.
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return repo
}

// TestRunFeatureStart_RefusesUnlandedProposal proves feature start refuses
// (exit 1) a story whose proposal has never landed on the default branch,
// and never mutates the repo (no branch switch, no commit) when it does.
// Replaces the pre-Task-5 "status: draft" fixture: under Git-derived state
// a `status: draft` document whose exact bytes are already the default
// branch's own committed content is accepted-pending-build regardless of
// that persisted word (Task 4's compatibility reading), so that fixture no
// longer exercises a refusal at all — this test now builds the genuine
// negative case, a story that exists only on an unmerged branch.
func TestRunFeatureStart_RefusesUnlandedProposal(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/verdi.yaml": phase7ManifestYAML},
			Message: "init store, no spec on main",
		},
	})
	checkoutBranch(t, repo.Dir, "design/stale-decline")
	writeTestFile(t, repo.Dir+"/.verdi/specs/active/stale-decline/spec.md", []byte(featureAliasStorySpecMD("accepted-pending-build")))
	add := exec.Command("git", "add", "-A")
	add.Dir = repo.Dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	commit := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "propose stale-decline")
	commit.Dir = repo.Dir
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	ctx := context.Background()

	before, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runFeatureStart(unlanded proposal) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "proposal has not landed") {
		t.Fatalf("stderr = %q, want it to name the refusal", stderr.String())
	}
	if !contains(stderr.String(), "deprecated") {
		t.Fatalf("stderr = %q, want the R4-I-6 deprecation notice", stderr.String())
	}

	after, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("a refused feature start must not switch branches: before=%q after=%q", before, after)
	}
}

// TestRunFeatureStart_Succeeds proves feature start cuts feature/<name>
// once the spec is accepted-pending-build, resolving via a story ref
// (I-30's scheme-prefixed form) and prints the R4-I-6 deprecation notice
// while still proceeding.
func TestRunFeatureStart_Succeeds(t *testing.T) {
	repo := buildFeatureAliasRepo(t, "accepted-pending-build")
	ctx := context.Background()

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "jira:LOAN-1482", deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runFeatureStart = %d, want 0; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "deprecated") {
		t.Fatalf("stderr = %q, want the R4-I-6 deprecation notice", stderr.String())
	}

	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/stale-decline" {
		t.Fatalf("CurrentBranch = %q, want feature/stale-decline", branch)
	}
}

// TestRunFeatureStart_SpecRefForm proves feature start also accepts the
// spec-ref form (I-30's second accepted form).
func TestRunFeatureStart_SpecRefForm(t *testing.T) {
	repo := buildFeatureAliasRepo(t, "accepted-pending-build")
	ctx := context.Background()

	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
	var stdout, stderr bytes.Buffer
	got := runFeatureStart(ctx, repo.Dir, "spec/stale-decline", deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runFeatureStart(spec ref) = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// TestRunFeatureStart_Negative covers runFeatureStart's own
// operational-error path: an unresolvable story/spec ref.
func TestRunFeatureStart_Negative(t *testing.T) {
	repo := buildFeatureAliasRepo(t, "accepted-pending-build")
	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	t.Run("unknown story ref", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := runFeatureStart(ctx, repo.Dir, "jira:NOPE-1", deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runFeatureStart(unknown story) = %d, want 2", got)
		}
	})

	t.Run("bare tracker key", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := runFeatureStart(ctx, repo.Dir, "LOAN-1482", deps, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runFeatureStart(bare key) = %d, want 2", got)
		}
	})
}

// TestCmdFeatureStart_UsageNegative proves cmdFeatureStart's own
// argument-count check.
func TestCmdFeatureStart_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdFeatureStart(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdFeatureStart(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdFeatureStart([]string{"a", "b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdFeatureStart(two args) = %d, want 2", got)
	}
}

// TestRunFeatureVerb_UnknownSubcommand mirrors design's own subcommand
// dispatch test.
func TestRunFeatureVerb_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runFeatureVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runFeatureVerb(bogus) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runFeatureVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runFeatureVerb(no args) = %d, want 2", got)
	}
}

// TestRun_FeatureDispatchesToRealVerb proves dispatch.go routes "feature"
// to the real implementation.
func TestRun_FeatureDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"feature", "start", "jira:LOAN-1482"}, &stderr)
	if got != 2 {
		t.Fatalf("run([feature start ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
