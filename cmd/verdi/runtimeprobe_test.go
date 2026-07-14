package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgepkg "github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/runtimeprobe"
	"github.com/jyang234/verdi/internal/store"
)

// runtimeProbeFixtureSpecMD is a minimal story spec declaring evidence:
// [runtime] on its one AC — the target shape spec/runtime-evidence's
// producer entrypoint is exercised against.
const runtimeProbeFixtureSpecMD = `---
id: spec/runtime-fixture
kind: spec
class: story
title: "Runtime fixture story"
status: accepted-pending-build
owners: [platform-team]
story: jira:RUNTIME-FIXTURE-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
acceptance_criteria:
  - { id: ac-2, text: "the fixture check holds", evidence: [runtime] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Runtime fixture story
## Problem
x
## Outcome
y
`

// buildRuntimeProbeStore assembles a minimal store root: just the fixture
// story spec under specs/active/ — runProduceRuntime never touches git or
// derived-tree ancestry (unlike LoadRecords), so no fixturegit repo is
// needed here.
func buildRuntimeProbeStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	specDir := filepath.Join(root, ".verdi", "specs", "active", "runtime-fixture")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", specDir, err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(runtimeProbeFixtureSpecMD), 0o644); err != nil {
		t.Fatalf("writing spec.md: %v", err)
	}
	return root
}

// readRuntimeRecords reads back derived/<RefSlug(specRef)>/<commit>/
// runtime.json, reusing the same generic reader writeRuntimeRecord itself
// merges through.
func readRuntimeRecords(t *testing.T, root, specRef, commit string) []artifact.Evidence {
	t.Helper()
	path := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specRef), commit, "runtime.json")
	recs, err := readExistingEvidenceRecords(path)
	if err != nil {
		t.Fatalf("readExistingEvidenceRecords(%s): %v", path, err)
	}
	return recs
}

func fakeRuntimeDeps() (*fake.Forge, syncDeps, *bytes.Buffer, *bytes.Buffer) {
	f := fake.New()
	f.SetCIContext(forgepkg.CIInfo{Pipeline: "913", Job: "7"})
	var stdout, stderr bytes.Buffer
	return f, syncDeps{Forge: f, Stdout: &stdout, Stderr: &stderr}, &stdout, &stderr
}

// TestRunProduceRuntime_Happy proves the full emission path (spec/
// runtime-evidence ac-1, dc-1): given --story/--ac/--verdict/--witness
// inside a genuine CI environment, it writes exactly one well-formed,
// source: ci runtime record into derived/<spec>/<commit>/runtime.json,
// pulling pipeline/job from the forge's CIContext like every other
// producer.
func TestRunProduceRuntime_Happy(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps, stdout, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "GET /healthz -> 200", artifact.VerdictPass, false, deps)
	if code != 0 {
		t.Fatalf("runProduceRuntime = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}

	recs := readRuntimeRecords(t, root, "spec/runtime-fixture", testCommit)
	if len(recs) != 1 {
		t.Fatalf("runtime.json = %+v, want exactly 1 record", recs)
	}
	r := recs[0]
	if r.Kind != artifact.EvidenceRuntime {
		t.Errorf("Kind = %q, want runtime", r.Kind)
	}
	if len(r.EvidenceFor) != 1 || r.EvidenceFor[0] != "ac-2" {
		t.Errorf("EvidenceFor = %v, want [ac-2]", r.EvidenceFor)
	}
	if r.Verdict != artifact.VerdictPass {
		t.Errorf("Verdict = %q, want pass", r.Verdict)
	}
	if r.Producer != runtimeprobe.CheckID("jira:RUNTIME-FIXTURE-1", "ac-2") {
		t.Errorf("Producer = %q, want %q", r.Producer, runtimeprobe.CheckID("jira:RUNTIME-FIXTURE-1", "ac-2"))
	}
	if r.Provenance.Source != artifact.SourceCI {
		t.Errorf("Provenance.Source = %q, want ci", r.Provenance.Source)
	}
	if r.Provenance.Pipeline != "913" || r.Provenance.Job != "7" {
		t.Errorf("Provenance = %+v, want pipeline=913 job=7", r.Provenance)
	}
	if !strings.Contains(stdout.String(), "spec/runtime-fixture") || !strings.Contains(stdout.String(), "ac-2") {
		t.Errorf("stdout = %q, want it to name the spec and AC", stdout.String())
	}
}

// TestRunProduceRuntime_BareNoOp proves spec/runtime-evidence dc-3's honest
// scope: with none of --story/--ac/--verdict/--witness given (the shape
// verdi's own scheduled runtime-probe.yml invokes), it exits 0, discloses
// that nothing was reported, and writes NOTHING — never a fabricated
// "passing" record for a check that does not exist.
func TestRunProduceRuntime_BareNoOp(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps, stdout, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "", "", "", "", false, deps)
	if code != 0 {
		t.Fatalf("runProduceRuntime (bare) = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "nothing to report") {
		t.Errorf("stdout = %q, want a disclosed 'nothing to report' message", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".verdi", "data", "derived")); err == nil {
		t.Error("bare --produce-runtime wrote a derived/ tree; want nothing written")
	}
}

// TestRunProduceRuntime_Negative_RefusesOutsideCI proves --produce-runtime
// refuses to run outside a detected CI environment without --force-local —
// the same discipline --produce already applies (dc-3).
func TestRunProduceRuntime_Negative_RefusesOutsideCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	root := buildRuntimeProbeStore(t)
	_, deps, _, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "w", artifact.VerdictPass, false, deps)
	if code != 2 {
		t.Fatalf("runProduceRuntime outside CI without --force-local = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "--force-local") {
		t.Errorf("stderr = %q, want a mention of --force-local", stderr.String())
	}
}

// TestRunProduceRuntime_ForceLocal_StampsSourceLocal proves --force-local
// lets --produce-runtime run outside CI for local testing, prints a
// disclosed NON-AUTHORITATIVE warning, and stamps source: local — never
// source: ci, mirroring sync.go's runProduce --force-local precedent
// exactly (dc-3).
func TestRunProduceRuntime_ForceLocal_StampsSourceLocal(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	root := buildRuntimeProbeStore(t)
	_, deps, _, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "manual check", artifact.VerdictPass, true, deps)
	if code != 0 {
		t.Fatalf("runProduceRuntime --force-local = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "NON-AUTHORITATIVE") {
		t.Errorf("stderr = %q, want a disclosed NON-AUTHORITATIVE warning", stderr.String())
	}
	recs := readRuntimeRecords(t, root, "spec/runtime-fixture", testCommit)
	if len(recs) != 1 || recs[0].Provenance.Source != artifact.SourceLocal {
		t.Fatalf("records = %+v, want exactly 1 record with Provenance.Source=local", recs)
	}
}

// TestRunProduceRuntime_Negative_PartialArgs proves the four value flags
// must be given all together or not at all — a partial set is a usage
// error, not a silent partial no-op.
func TestRunProduceRuntime_Negative_PartialArgs(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps, _, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "", "", false, deps)
	if code != 2 {
		t.Fatalf("runProduceRuntime with only --story/--ac set = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "must all be given together") {
		t.Errorf("stderr = %q, want a message about giving all four flags together", stderr.String())
	}
}

// TestRunProduceRuntime_Negative_DanglingAC proves an AC id the resolved
// spec does not declare is a hard error (03 §Declarations: "a misspelled
// ac-3 must never surface as a silent no-signal") — never silently
// accepted into a record nothing will ever query back out.
func TestRunProduceRuntime_Negative_DanglingAC(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps, _, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-99", "w", artifact.VerdictPass, false, deps)
	if code != 2 {
		t.Fatalf("runProduceRuntime with a dangling AC = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "does not declare ac") {
		t.Errorf("stderr = %q, want a dangling-AC message", stderr.String())
	}
}

// TestRunProduceRuntime_Negative_UnresolvableStory proves a story/spec
// argument that resolves to nothing is a hard, surfaced error.
func TestRunProduceRuntime_Negative_UnresolvableStory(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps, _, stderr := fakeRuntimeDeps()

	code := runProduceRuntime(context.Background(), root, testCommit, "spec/does-not-exist", "ac-2", "w", artifact.VerdictPass, false, deps)
	if code != 2 {
		t.Fatalf("runProduceRuntime with an unresolvable story = %d, want 2", code)
	}
	if stderr.String() == "" {
		t.Error("stderr is empty, want an error naming the unresolvable story")
	}
}

// TestRunProduceRuntime_MergeIdempotentAcrossRuns proves a second run for
// the SAME (story, AC) replaces the first record rather than appending a
// duplicate — mergeEvidenceByProducer's existing idempotency-across-retries
// contract (selfevidence.go), reused unchanged here.
func TestRunProduceRuntime_MergeIdempotentAcrossRuns(t *testing.T) {
	t.Setenv("CI", "true")
	root := buildRuntimeProbeStore(t)
	_, deps1, _, _ := fakeRuntimeDeps()
	if code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "first check: 500", artifact.VerdictFail, false, deps1); code != 0 {
		t.Fatalf("first runProduceRuntime = %d, want 0", code)
	}
	_, deps2, _, _ := fakeRuntimeDeps()
	if code := runProduceRuntime(context.Background(), root, testCommit, "spec/runtime-fixture", "ac-2", "retry check: 200", artifact.VerdictPass, false, deps2); code != 0 {
		t.Fatalf("second runProduceRuntime = %d, want 0", code)
	}

	recs := readRuntimeRecords(t, root, "spec/runtime-fixture", testCommit)
	if len(recs) != 1 {
		t.Fatalf("records = %+v, want exactly 1 (the retry replaces the first, never appends)", recs)
	}
	if recs[0].Verdict != artifact.VerdictPass || recs[0].Witness != "retry check: 200" {
		t.Fatalf("records[0] = %+v, want the LATEST run's verdict/witness to have won", recs[0])
	}
}

// TestRunProduceRuntime_Binary_FailVerdictExitsZero pins runtimeprobe.go's
// header-stated transcription semantic (spec/fail-loud ac-2) end to end,
// driving the real, compiled verdi binary against a fixturegit store
// (co-1) rather than calling runProduceRuntime in-process like every test
// above: verdi STAMPS an externally computed verdict, it does not compute
// one, so a --verdict fail emission that successfully writes its record
// is still exit 0 (emission success, regardless of the verdict's value) —
// contrast sync.go's --produce path, whose evaluateBundle surfaces its
// OWN computed verdicts as exit 1. The fail verdict itself is real and
// recorded; it is consumed downstream by the fold, never by this
// producer's exit code.
func TestRunProduceRuntime_Binary_FailVerdictExitsZero(t *testing.T) {
	bin := buildVerdiBinary(t)

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                           "schema: verdi.layout/v1\nforge: gitlab\n",
			".verdi/specs/active/runtime-fixture/spec.md": runtimeProbeFixtureSpecMD,
		},
		Message: "seed runtime-fixture store",
	}})

	t.Setenv("CI", "true")
	cmd := exec.Command(bin, "sync", "--produce-runtime",
		"--story", "spec/runtime-fixture",
		"--ac", "ac-2",
		"--verdict", "fail",
		"--witness", "GET /healthz -> 500",
	)
	cmd.Dir = repo.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("verdi sync --produce-runtime --verdict fail exited %d, want 0 (emission success is exit 0 regardless of the stamped verdict's value); output:\n%s", exitErr.ExitCode(), out)
		}
		t.Fatalf("running verdi sync --produce-runtime: %v; output:\n%s", err, out)
	}

	recs := readRuntimeRecords(t, repo.Dir, "spec/runtime-fixture", repo.Head)
	if len(recs) != 1 {
		t.Fatalf("runtime.json = %+v, want exactly 1 record", recs)
	}
	if recs[0].Verdict != artifact.VerdictFail {
		t.Fatalf("recs[0].Verdict = %q, want fail — the exit-0 emission must still durably record the real fail verdict, never launder it", recs[0].Verdict)
	}
}
