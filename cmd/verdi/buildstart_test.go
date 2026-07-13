package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

const birdsEyeFeatureSpecMD = `---
id: spec/loan-mgmt
kind: spec
title: "Loan management"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:LOAN-1483
problem: { text: "borrowers cannot see accurate status", anchor: problem }
outcome: { text: "borrowers see accurate status", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static, attestation] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# Loan management

## Problem
x
## Outcome
x
`

func buildBirdsEyeFeatureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                     phase7ManifestYAML,
				".verdi/specs/active/loan-mgmt/spec.md": birdsEyeFeatureSpecMD,
			},
			Message: "init store with a round-four birds-eye feature",
		},
	})
}

// TestRunBuildStart_RefusesBirdsEyeFeature proves build start refuses
// (exit 2, operational — a targeting mistake, not a business precondition)
// a round-four class: feature spec: it has no code of its own to build
// against.
func TestRunBuildStart_RefusesBirdsEyeFeature(t *testing.T) {
	repo := buildBirdsEyeFeatureRepo(t)
	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/loan-mgmt", deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runBuildStart(birds-eye feature) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if !contains(stderr.String(), "feature spec") {
		t.Fatalf("stderr = %q, want it to name the birds-eye-feature refusal", stderr.String())
	}

	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "feature/loan-mgmt" {
		t.Fatal("a refused build start must not cut a build branch")
	}
}

// TestCmdBuildStart_UsageNegative proves cmdBuildStart's own
// argument-count check.
func TestCmdBuildStart_UsageNegative(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := cmdBuildStart(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdBuildStart(no args) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := cmdBuildStart([]string{"a", "b"}, &stdout, &stderr); got != 2 {
		t.Fatalf("cmdBuildStart(two args) = %d, want 2", got)
	}
}

// TestRunBuildVerb_UnknownSubcommand mirrors design/feature's own
// subcommand dispatch tests.
func TestRunBuildVerb_UnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := runBuildVerb([]string{"bogus"}, &stdout, &stderr); got != 2 {
		t.Fatalf("runBuildVerb(bogus) = %d, want 2", got)
	}
	stdout.Reset()
	stderr.Reset()
	if got := runBuildVerb(nil, &stdout, &stderr); got != 2 {
		t.Fatalf("runBuildVerb(no args) = %d, want 2", got)
	}
}

// TestRun_BuildDispatchesToRealVerb proves dispatch.go routes "build" to
// the real implementation (R4-I-6).
func TestRun_BuildDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"build", "start", "jira:LOAN-1482"}, &stderr)
	if got != 2 {
		t.Fatalf("run([build start ...]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}
