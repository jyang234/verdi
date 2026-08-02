// Final fix wave C1 (merge-signaled spec acceptance): `verdi close` must
// work for STATUSLESS (merge-accepted) specs and must resolve its
// pre-closure precondition through the specstate projector BEFORE any
// mutation begins. Pre-fix, close.go's flipSpecStatusToClosed required
// exactly one `status: accepted-pending-build` line — a statusless spec
// has zero, and the failure landed AFTER the closure branch cut, the
// align freeze, and writeRollup: unrecoverable residue, and a retry could
// never succeed. These tests pin the fixed contract at BOTH rungs (story:
// runClose; feature: runCloseFeature via the same dispatch):
//
//   - a statusless spec closes cleanly end to end, its archive move a
//     PURE rename (no status line is invented — VL-002 reads an
//     archive-zone statusless spec as closed-by-zone, and VL-010's
//     pure-rename exception admits the move);
//   - the effective-state precondition refuses BEFORE any mutation:
//     proposed/diverged/superseded/closed refuse as a verdict (exit 1),
//     unproven exits 2 with the projector's own disclosures — and in
//     every refusal shape, no closure branch exists and no file moved.
package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// statuslessCloseStorySpecMD is a merge-accepted story spec: NO status:
// field and NO frozen: stamp at all (the design's Artifact compatibility
// section: new merge-accepted artifacts carry neither). Its acceptance is
// derived from its exact bytes being reachable from the default branch —
// the shape every post-migration spec has.
const statuslessCloseStorySpecMD = `---
id: spec/statusless-close
kind: spec
class: story
title: "Statusless close story"
owners: [platform-team]
story: jira:SL-CLOSE-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the statusless fixture check holds", evidence: [runtime] }
---
# Statusless close story
## Problem
x
## Outcome
y
`

// buildStatuslessCloseFixtureRepo mirrors buildRuntimeCloseFixtureRepo
// (close_runtime_test.go) with the statusless story in place of the legacy
// statused one. CI_DEFAULT_BRANCH pins the projector's default-branch
// resolution (this fixturegit repo has no origin remote), exactly as
// buildCloseFeatureRepo already documents for its own fixtures.
func buildStatuslessCloseFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md":        featureV1SpecMD,
			".verdi/specs/active/statusless-close/spec.md": statuslessCloseStorySpecMD,
		},
		Message: "statusless close fixture: feature + statusless merge-accepted story",
	}})
}

// writeStatuslessCloseGateReport mirrors writeCloseGateReport for this
// file's own fixture name.
func writeStatuslessCloseGateReport(t *testing.T, root, covers string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "statusless-close")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := "---\nschema: verdi.deviation/v1\ncovers: " + covers + "\nfindings:\n" + dispositionedFindingYAML + "\ndigest: sha256:" + strings.Repeat("0", 64) + "\n---\n# Alignment report\n"
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing deviation-report.md: %v", err)
	}
}

// TestRunClose_StatuslessSpec_ClosesCleanly is C1's story-rung happy-path
// proof: a statusless, merge-accepted story closes end to end, and its
// archived spec.md is BYTE-IDENTICAL to the active original — the archive
// move is a pure rename (git R100), never an invented status flip.
func TestRunClose_StatuslessSpec_ClosesCleanly(t *testing.T) {
	t.Setenv("CI", "true") // a genuine, detected CI environment: stamps source: ci
	repo := buildStatuslessCloseFixtureRepo(t)
	ctx := context.Background()

	_, deps, stdout, stderr := fakeRuntimeDeps()
	if code := runProduceRuntime(ctx, repo.Dir, repo.Head, "spec/statusless-close", "ac-1", "GET /healthz -> 200", artifact.VerdictPass, false, deps); code != 0 {
		t.Fatalf("runProduceRuntime = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	writeStatuslessCloseGateReport(t, repo.Dir, repo.Head)

	fp := fake.New()
	closeD := closeDeps{Forge: forgefake.New(), Registry: fp, Runner: upstream.NewFakeRunner()}
	var closeStdout, closeStderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/statusless-close", &store.Manifest{}, closeD, &closeStdout, &closeStderr)
	if got != 0 {
		t.Fatalf("runClose(statusless) = %d, want 0; stdout=%s stderr=%s", got, closeStdout.String(), closeStderr.String())
	}

	// The active directory is gone; the archived spec.md is byte-identical
	// (no status line was invented — closed-by-zone, VL-002).
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "statusless-close")); !os.IsNotExist(err) {
		t.Fatalf("specs/active/statusless-close still exists after close (err=%v)", err)
	}
	archivedSpec, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "statusless-close", "spec.md"))
	if err != nil {
		t.Fatalf("reading archived spec.md: %v", err)
	}
	if string(archivedSpec) != statuslessCloseStorySpecMD {
		t.Fatalf("archived spec.md is not byte-identical to the statusless original:\n--- got ---\n%s\n--- want ---\n%s", archivedSpec, statuslessCloseStorySpecMD)
	}

	// The rollup archived and published as usual.
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "statusless-close", "rollup.json")); err != nil {
		t.Fatalf("archived rollup.json missing: %v", err)
	}
	if _, ok := fp.PublishedField("jira:SL-CLOSE-1"); !ok {
		t.Fatal("fake provider has no published rollup for jira:SL-CLOSE-1")
	}

	// Git-level proof: spec.md's archive move is a PURE rename — R100 —
	// VL-010's pure-rename exception, not the legacy status-flip one.
	diffOut := gitOutput(t, repo.Dir, "diff", "--name-status", "-M", repo.Head, "HEAD")
	pureRename := regexp.MustCompile(`R100\t\.verdi/specs/active/statusless-close/spec\.md\t\.verdi/specs/archive/statusless-close/spec\.md`)
	if !pureRename.MatchString(diffOut) {
		t.Fatalf("git diff --name-status -M did not report a PURE (R100) rename for the statusless spec.md:\n%s", diffOut)
	}

	// The post-close store re-lints clean of VL-002 (archive-zone
	// statusless is closed-by-zone) and VL-010 (the pure rename is
	// admitted) — mirroring TestRunClose_EndToEnd's own two-rule check.
	lintFindings, err := lint.NewEngine().Run(ctx, repo.Dir, lint.Context{DiffBase: repo.Head}, lint.Options{})
	if err != nil {
		t.Fatalf("re-lint of post-close store: %v", err)
	}
	for _, f := range lintFindings {
		if f.Rule == "VL-002" || f.Rule == "VL-010" {
			t.Fatalf("re-lint of post-close store fired %s on the statusless archive: %s", f.Rule, f.String())
		}
	}
}

// assertNoClosureResidue proves close mutated NOTHING: no closure branch
// exists, the active-zone spec directory is intact (specContent still on
// disk byte-for-byte), and no archive directory appeared.
func assertNoClosureResidue(t *testing.T, ctx context.Context, root, name string, wantActiveSpec []byte) {
	t.Helper()
	if _, err := gitx.RevParse(ctx, root, "close/"+name); err == nil {
		t.Fatalf("closure branch close/%s exists — the refusal must land BEFORE the branch cut", name)
	}
	if branch := gitCurrentBranch(t, root); branch != "main" {
		t.Fatalf("current branch = %q, want main (a refusal must not switch branches)", branch)
	}
	active, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", name, "spec.md"))
	if err != nil {
		t.Fatalf("active spec.md unreadable after refusal: %v", err)
	}
	if !bytes.Equal(active, wantActiveSpec) {
		t.Fatalf("active spec.md bytes changed across a refusal:\n--- got ---\n%s\n--- want ---\n%s", active, wantActiveSpec)
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "specs", "archive", name)); !os.IsNotExist(err) {
		t.Fatalf("specs/archive/%s exists after a refusal (err=%v)", name, err)
	}
}

// TestRunClose_EffectiveStatePrecondition_RefusesBeforeMutation is C1's
// story-rung refusal proof: every non-AcceptedPendingBuild effective state
// refuses BEFORE the branch cut, the freeze, and every other mutation —
// proposed/diverged/superseded as a verdict (exit 1), unproven as
// operational (exit 2) with the projector's own disclosures.
func TestRunClose_EffectiveStatePrecondition_RefusesBeforeMutation(t *testing.T) {
	t.Setenv("CI", "true")

	t.Run("diverged working-tree bytes refuse as a verdict, no residue", func(t *testing.T) {
		repo := buildStatuslessCloseFixtureRepo(t)
		ctx := context.Background()
		specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "statusless-close", "spec.md")
		edited := []byte(statuslessCloseStorySpecMD + "\nlocal, uncommitted edit\n")
		if err := os.WriteFile(specPath, edited, 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		got := runClose(ctx, repo.Dir, "spec/statusless-close", &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}, &stdout, &stderr)
		if got != 1 {
			t.Fatalf("runClose(diverged) = %d, want 1 (verdict refusal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "diverge") {
			t.Fatalf("stderr = %q, want the refusal to name the divergence from the accepted revision", stderr.String())
		}
		assertNoClosureResidue(t, ctx, repo.Dir, "statusless-close", edited)
	})

	t.Run("legacy superseded refuses as a verdict, no residue", func(t *testing.T) {
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		supersededSpec := strings.Replace(statuslessCloseStorySpecMD,
			"story: jira:SL-CLOSE-1\n",
			"story: jira:SL-CLOSE-1\nstatus: superseded\n", 1)
		supersededSpec = strings.Replace(supersededSpec, "---\n# Statusless close story",
			"frozen: { at: 2024-01-01, commit: "+gateFakeFrozenCommit+" }\n---\n# Statusless close story", 1)
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":                            "schema: verdi.layout/v1\nforge: github\n",
				".verdi/specs/active/loan-mgmt/spec.md":        featureV1SpecMD,
				".verdi/specs/active/statusless-close/spec.md": supersededSpec,
			},
			Message: "superseded-story close fixture",
		}})
		ctx := context.Background()

		var stdout, stderr bytes.Buffer
		got := runClose(ctx, repo.Dir, "spec/statusless-close", &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}, &stdout, &stderr)
		if got != 1 {
			t.Fatalf("runClose(superseded) = %d, want 1 (verdict refusal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "superseded") {
			t.Fatalf("stderr = %q, want the refusal to name the superseded state", stderr.String())
		}
		assertNoClosureResidue(t, ctx, repo.Dir, "statusless-close", []byte(supersededSpec))
	})

	t.Run("unproven state exits 2 with disclosures, no residue", func(t *testing.T) {
		repo := buildStatuslessCloseFixtureRepo(t)
		// No default branch is resolvable at all: CI_DEFAULT_BRANCH unset,
		// no origin remote in a fixturegit repo.
		t.Setenv("CI_DEFAULT_BRANCH", "")
		ctx := context.Background()

		var stdout, stderr bytes.Buffer
		got := runClose(ctx, repo.Dir, "spec/statusless-close", &store.Manifest{}, closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runClose(unproven) = %d, want 2 (operational); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "cannot be proven") {
			t.Fatalf("stderr = %q, want the unproven disclosure", stderr.String())
		}
		assertNoClosureResidue(t, ctx, repo.Dir, "statusless-close", []byte(statuslessCloseStorySpecMD))
	})
}

// statuslessCloseFeatureSpecMD builds the feature-rung statusless fixture:
// closeFeatureSpecMD's exact shape minus its status: and frozen: lines —
// the merge-accepted feature shape.
func statuslessCloseFeatureSpecMD(base string) string {
	out := strings.Replace(base, "status: accepted-pending-build\n", "", 1)
	return regexp.MustCompile(`(?m)^frozen: \{[^\n]*\}\n`).ReplaceAllString(out, "")
}

// TestRunCloseFeature_StatuslessSpec_ClosesCleanly is C1's feature-rung
// happy-path proof, mirroring TestRunCloseFeature_EndToEnd with a
// statusless, merge-accepted feature: closes end to end, archived spec.md
// byte-identical, pure R100 rename.
func TestRunCloseFeature_StatuslessSpec_ClosesCleanly(t *testing.T) {
	t.Setenv("CI", "true")
	opts := defaultCloseFeatureFixtureOpts()
	scaffoldSHA := featureCloseScaffoldSHA(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	statuslessFeature := statuslessCloseFeatureSpecMD(closeFeatureSpecMD(scaffoldSHA, ""))
	repo := fixturegit.Build(t, []fixturegit.Layer{
		featureCloseScaffoldLayer,
		{
			Files: map[string]string{
				".verdi/specs/active/close-feature-fixture/spec.md":          statuslessFeature,
				".verdi/specs/archive/fixture-story-one/spec.md":             closeFeatureStorySpecMD("fixture-story-one", scaffoldSHA, "closed", "jira:FIXTURE-STORY-1", "ac-1"),
				".verdi/specs/archive/fixture-story-one/deviation-report.md": closeFeatureStoryDeviationMD(scaffoldSHA),
				".verdi/specs/archive/fixture-story-two/spec.md":             closeFeatureStorySpecMD("fixture-story-two", scaffoldSHA, "closed", "jira:FIXTURE-STORY-2", "ac-2"),
				".verdi/specs/archive/fixture-story-two/deviation-report.md": closeFeatureStoryDeviationMD(scaffoldSHA),
				".verdi/obligations/fixture-story-one/ac-1--static.md":       closeFeatureStoryObligationMD("fixture-story-one", scaffoldSHA),
				".verdi/obligations/fixture-story-two/ac-1--static.md":       closeFeatureStoryObligationMD("fixture-story-two", scaffoldSHA),
			},
			Message: "add statusless close-feature-fixture + its two closed implementing stories",
		},
	})
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose(statusless feature) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	archivedSpec, err := os.ReadFile(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-feature-fixture", "spec.md"))
	if err != nil {
		t.Fatalf("reading archived spec.md: %v", err)
	}
	if string(archivedSpec) != statuslessFeature {
		t.Fatalf("archived feature spec.md is not byte-identical to the statusless original:\n--- got ---\n%s\n--- want ---\n%s", archivedSpec, statuslessFeature)
	}
	diffOut := gitOutput(t, repo.Dir, "diff", "--name-status", "-M", repo.Head, "HEAD")
	pureRename := regexp.MustCompile(`R100\t\.verdi/specs/active/close-feature-fixture/spec\.md\t\.verdi/specs/archive/close-feature-fixture/spec\.md`)
	if !pureRename.MatchString(diffOut) {
		t.Fatalf("git diff --name-status -M did not report a PURE (R100) rename for the statusless feature spec.md:\n%s", diffOut)
	}
}

// TestRunCloseFeature_EffectiveStatePrecondition_RefusesBeforeMutation is
// C1's feature-rung refusal proof: diverged working-tree bytes on the
// feature refuse as a verdict BEFORE any mutation.
func TestRunCloseFeature_EffectiveStatePrecondition_RefusesBeforeMutation(t *testing.T) {
	t.Setenv("CI", "true")
	opts := defaultCloseFeatureFixtureOpts()
	repo := buildCloseFeatureRepo(t, opts)
	seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
	writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	ctx := context.Background()

	specPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture", "spec.md")
	original, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := append(append([]byte{}, original...), []byte("\nlocal, uncommitted edit\n")...)
	if err := os.WriteFile(specPath, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-feature-fixture", &store.Manifest{}, closeFeatureDeps(fake.New()), &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runClose(diverged feature) = %d, want 1 (verdict refusal); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "diverge") {
		t.Fatalf("stderr = %q, want the refusal to name the divergence", stderr.String())
	}
	assertNoClosureResidue(t, ctx, repo.Dir, "close-feature-fixture", edited)
}
