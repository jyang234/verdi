package main

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
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

// This file is spec/runtime-evidence's co-2/dc-2 proof at the `verdi close`
// level: "rollup.json is [runtime records'] first durable residence in the
// corpus" (03 §Runtime evidence residence). Pre-close, a runtime record
// lives only under the NEVER-committed .verdi/data/derived/ tree
// (CLAUDE.md: "Nothing under .verdi/data/ is ever committed"); rollup.json,
// part of the archived quartet close.go commits, is the first place that
// record's outcome becomes a durable, git-tracked fact. This test proves
// that chain end to end: produce a runtime record -> close the story -> the
// committed rollup.json reflects it.

// runtimeCloseFixtureStorySpecMD declares evidence: [runtime] ONLY on its
// one AC — unlike close_test.go's closeFixtureStorySpecMD ([static,
// behavioral]), so this test is a clean proof that runtime alone, with no
// other kind's help, can carry a story to eligible.
const runtimeCloseFixtureStorySpecMD = `---
id: spec/runtime-close-fixture
kind: spec
class: story
title: "Runtime close fixture story"
status: accepted-pending-build
owners: [platform-team]
story: jira:RUNTIME-CLOSE-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture check holds", evidence: [runtime] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Runtime close fixture story
## Problem
x
## Outcome
y
`

// buildRuntimeCloseFixtureRepo mirrors buildCloseFixtureRepo (close_test.go)
// but carries no verdi.bindings.yaml — this story's only evidence comes from
// the runtime producer, exercised directly below, not the self-hosted
// static/behavioral path.
func buildRuntimeCloseFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                                 "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md":             featureV1SpecMD,
			".verdi/specs/active/runtime-close-fixture/spec.md": runtimeCloseFixtureStorySpecMD,
		},
		Message: "runtime close fixture: feature + story declaring evidence: [runtime]",
	}})
}

// writeRuntimeCloseGateReport mirrors close_test.go's own
// writeCloseGateReport for this file's differently-named fixture
// (runtime-close-fixture) — X-13/X-16/X-17's closure-gate condition 4
// needs a living, fully-dispositioned, head-covering report before close
// will freeze rather than refuse.
func writeRuntimeCloseGateReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "runtime-close-fixture")
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

// TestRunClose_RuntimeEvidenceReachesRollup is spec/runtime-evidence's
// end-to-end proof at the close level (ac-2, co-2, dc-2): a real
// runtime-evidence producer run (runProduceRuntime, exercised for real, not
// mocked) feeds a story declaring evidence: [runtime] all the way to
// eligible, and the committed rollup.json — runtime's first durable
// residence in the corpus — records ac-1 evidenced with a summary naming
// the runtime verdict.
func TestRunClose_RuntimeEvidenceReachesRollup(t *testing.T) {
	t.Setenv("CI", "true") // a genuine, detected CI environment: stamps source: ci (co-1's authoritative-only posture)
	repo := buildRuntimeCloseFixtureRepo(t)
	ctx := context.Background()

	// The real producer entrypoint, exercised end to end: this is what
	// makes a story declaring evidence: [runtime] reach evidenced. Closure
	// (03 §The fold) folds ONLY source: ci records, so forceLocal stays
	// false here — a forceLocal run would stamp source: local, which the
	// closure gate below would then correctly refuse (co-1's own proof,
	// covered separately by internal/evidence's
	// TestRuntimeEvidence_LocalRecordIgnoredUnderPreviewFalse).
	_, deps, stdout, stderr := fakeRuntimeDeps()
	code := runProduceRuntime(ctx, repo.Dir, repo.Head, "spec/runtime-close-fixture", "ac-1", "GET /healthz -> 200", artifact.VerdictPass, false, deps)
	if code != 0 {
		t.Fatalf("runProduceRuntime = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	// The corrected closure ritual (X-16): align (a living report covering
	// head) -> disposition (working-tree edit) -> close.
	writeRuntimeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	fp := fake.New()
	fg := forgefake.New() // reachable, no open MRs seeded
	manifest := &store.Manifest{}
	closeDeps := closeDeps{Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner()}

	var closeStdout, closeStderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/runtime-close-fixture", manifest, closeDeps, &closeStdout, &closeStderr)
	if got != 0 {
		t.Fatalf("runClose = %d, want 0; stdout=%s stderr=%s", got, closeStdout.String(), closeStderr.String())
	}

	archiveDir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "runtime-close-fixture")
	rollRaw, err := os.ReadFile(filepath.Join(archiveDir, "rollup.json"))
	if err != nil {
		t.Fatalf("reading archived rollup.json: %v", err)
	}
	roll, err := artifact.DecodeRollup(rollRaw)
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if !roll.Eligible {
		t.Fatalf("rollup.Eligible = false, want true (runtime alone should satisfy ac-1): %+v", roll)
	}
	if len(roll.Criteria) != 1 {
		t.Fatalf("rollup.Criteria = %+v, want exactly 1", roll.Criteria)
	}
	c := roll.Criteria[0]
	if c.Status != artifact.CriterionEvidenced {
		t.Fatalf("rollup.Criteria[0].Status = %q, want evidenced", c.Status)
	}
	if c.Summary != "runtime:pass" {
		t.Fatalf("rollup.Criteria[0].Summary = %q, want %q (runtime's own verdict, durably recorded)", c.Summary, "runtime:pass")
	}
	if roll.Story != "jira:RUNTIME-CLOSE-1" || roll.Commit != repo.Head {
		t.Fatalf("rollup = %+v, unexpected story/commit", roll)
	}

	// The rollup reaches the publish step for real, carrying the same
	// runtime-evidenced criterion.
	published, ok := fp.PublishedField("jira:RUNTIME-CLOSE-1")
	if !ok {
		t.Fatal("fake provider has no published rollup for jira:RUNTIME-CLOSE-1")
	}
	if len(published.Criteria) != 1 || published.Criteria[0].Status != "evidenced" {
		t.Fatalf("published rollup criteria = %+v, want ac-1 evidenced", published.Criteria)
	}
}
