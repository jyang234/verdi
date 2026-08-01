package main

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
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
	got := runBuildStart(ctx, repo.Dir, "spec/loan-mgmt", specstate.NewProjector(), deps, &stdout, &stderr)
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

// statuslessBuildStorySpecMD is a minimal, valid, STATUSLESS active story
// spec.md — Task 4's compatibility grammar (feature/story specs may omit
// status:) proven all the way through build start's own Git-derived
// acceptance precondition.
const statuslessBuildStorySpecMD = `---
id: spec/widget-story
kind: spec
class: story
title: "Widget story"
owners: [platform-team]
story: jira:WIDGET-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
---
# body
`

// TestRunBuildStart_StatuslessExactDefaultBranch_Starts proves a statusless
// story whose exact bytes have landed on the default branch resolves
// AcceptedPendingBuild (Task 4's compatibility reading, now reaching build
// start's own precondition via the specStateResolver seam) and starts the
// build.
func TestRunBuildStart_StatuslessExactDefaultBranch_Starts(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/widget-story/spec.md": statuslessBuildStorySpecMD,
			},
			Message: "init store with a statusless, landed story",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/widget-story", specstate.NewProjector(), deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runBuildStart(statusless, landed) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	branch, err := gitx.CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch != "feature/widget-story" {
		t.Fatalf("branch = %q, want feature/widget-story; stdout=%s", branch, stdout.String())
	}
}

// TestRunBuildStart_UnmergedProposal_RefusesAsVerdict proves a story that
// exists only on an unmerged branch (never landed on the default branch)
// refuses as a verdict failure (exit 1) naming that its proposal has not
// landed — never a silent proceed, and never conflated with the
// operational Unproven case below.
func TestRunBuildStart_UnmergedProposal_RefusesAsVerdict(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files:   map[string]string{".verdi/verdi.yaml": phase7ManifestYAML},
			Message: "init store, no spec on main",
		},
	})
	checkoutBranch(t, repo.Dir, "design/widget-story")
	writeTestFile(t, repo.Dir+"/.verdi/specs/active/widget-story/spec.md", []byte(statuslessBuildStorySpecMD))
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repo.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@t", "commit", "--quiet", "--no-verify", "-m", "propose widget-story")
	cmd.Dir = repo.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/widget-story", specstate.NewProjector(), deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runBuildStart(unmerged proposal) = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "proposal has not landed") {
		t.Fatalf("stderr = %q, want it to say the proposal has not landed", stderr.String())
	}
}

// buildstartSupersededPredecessorMD and buildstartSupersessorMD build a
// minimal, valid Git-derived supersession pair: a story-grade, statusless
// grandfathered feature spec (no problem/outcome, so build start's own
// birds-eye-feature guard does not fire) named as the predecessor by a
// landed successor's `links: supersedes` edge plus a validated
// `supersession:` block — the exact two-signal shape
// internal/specstate/resolve_test.go's own superseded case proves at the
// package level, exercised here end to end through real git and build
// start's CLI seam.
const buildstartSupersededPredecessorMD = `---
id: spec/old-feature
kind: spec
class: feature
title: "Old feature"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

const buildstartSupersessorMD = `---
id: spec/new-feature
kind: spec
class: feature
title: "New feature"
owners: [platform-team]
status: accepted-pending-build
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: supersedes, ref: spec/old-feature }
supersession:
  added: [ac-1]
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
`

// TestRunBuildStart_Superseded_RefusesAsVerdict proves a predecessor spec
// that a landed, validated successor names is refused as a verdict failure
// (exit 1, D-12: never re-buildable), driven through the specStateResolver
// seam rather than a raw persisted status field.
func TestRunBuildStart_Superseded_RefusesAsVerdict(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                       phase7ManifestYAML,
				".verdi/specs/active/old-feature/spec.md": buildstartSupersededPredecessorMD,
				".verdi/specs/active/new-feature/spec.md": buildstartSupersessorMD,
			},
			Message: "init store with a landed supersession pair",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/old-feature", specstate.NewProjector(), deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runBuildStart(superseded) = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "spec/new-feature") {
		t.Fatalf("stderr = %q, want it to name the successor spec/new-feature", stderr.String())
	}
}

// legacySupersededStoryMD is a class: story predecessor carrying an
// EXPLICIT legacy `status: superseded` field whose successor has already
// closed (moved to the archive zone) — the disclosure-seam live-witness
// shape (fix-round-1 finding 1): a class: story spec can never carry a
// validated supersession: block (internal/artifact's validateStory
// rejects it), so specstate's own two-signal successor-corpus proof can
// never independently confirm a story-level (rung-3) supersession; the
// legacy status field is the only signal that exists for it.
const legacySupersededStoryMD = `---
id: spec/disclosure-seam
kind: spec
class: story
status: superseded
title: "Disclosure seam"
owners: [platform-team]
story: jira:DS-1
problem: { text: "x", anchor: problem }
outcome: { text: "y", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
`

// TestRunBuildStart_LegacySupersededExactLanded_RefusesAsVerdict is
// fix-round-1 finding 1's build-start proof: a story predecessor whose
// exact bytes are landed and carry an EXPLICIT legacy `status: superseded`
// field, whose successor is NOT visible to the active-zone-only
// successor-corpus scan (already closed to the archive zone — the live
// disclosure-seam witness), still refuses as a verdict (exit 1) — the
// projector's own legacy-terminal-status compatibility read, not a
// silent proceed into AcceptedPendingBuild.
func TestRunBuildStart_LegacySupersededExactLanded_RefusesAsVerdict(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                           phase7ManifestYAML,
				".verdi/specs/active/disclosure-seam/spec.md": legacySupersededStoryMD,
			},
			Message: "init store with a landed, legacy-superseded story predecessor",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/disclosure-seam", specstate.NewProjector(), deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runBuildStart(legacy superseded, exact landed) = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "superseded") {
		t.Fatalf("stderr = %q, want it to name the superseded refusal", stderr.String())
	}
}

// TestRunBuildStart_UnresolvableDefaultBranch_OperationalError proves that
// when the default branch itself cannot be resolved at all, build start
// cannot honestly decide the acceptance precondition and refuses
// operationally (exit 2) rather than guessing either way. fix-round-1
// finding 4: the message restores the pre-Task-5 D6-6 legible refusal
// (every source tried, plus the `git remote set-head` remedy), not just a
// terser generic disclosure.
func TestRunBuildStart_UnresolvableDefaultBranch_OperationalError(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                        phase7ManifestYAML,
				".verdi/specs/active/widget-story/spec.md": statuslessBuildStorySpecMD,
			},
			Message: "init store, no default branch resolvable",
		},
	})
	t.Setenv("CI_DEFAULT_BRANCH", "")

	ctx := context.Background()
	deps := syncDeps{Runner: nil, GoTest: fakeGoTest{}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}

	var stdout, stderr bytes.Buffer
	got := runBuildStart(ctx, repo.Dir, "spec/widget-story", specstate.NewProjector(), deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runBuildStart(unresolvable default branch) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	out := stderr.String()
	for _, want := range []string{"CI_DEFAULT_BRANCH", "git remote HEAD", "origin/main", "origin/master", "git remote set-head origin"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want it to mention %q (D6-6: name every source tried plus the remedy)", out, want)
		}
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
