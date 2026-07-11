package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/upstream"
)

const alignLoansvcFlowmapYAML = "version: 1\nservice: loansvc\n"

const alignAcceptanceBoundaryContractJSON = `{
  "service": "loansvc",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [ { "name": "notification-svc", "kind": "events" } ],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

// alignSpecMD is a feature spec with the same declared boundary the
// fixture's committed contract already satisfies (so `verdi align`'s
// computed section finds it holding, no regeneration drift needed). frozen
// is any syntactically valid sha — SpecFrontmatter.Validate only checks the
// shape, and none of this test suite exercises align's acceptance-baseline
// git lookup (that is internal/align's own TestCompute_* coverage).
const alignSpecMD = `---
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
impacts: [loansvc]
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
`

// buildAlignRepo builds a one-layer fixturegit repo carrying an accepted
// feature spec and its impacted service, then checks out
// feature/stale-decline — `verdi feature start`'s branch convention
// (internal/storyresolve.ResolveBuildSpec's inference target).
func buildAlignRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: gitlab\n",
				"loansvc/.flowmap.yaml":                     alignLoansvcFlowmapYAML,
				"loansvc/.flowmap/boundary-contract.json":   alignAcceptanceBoundaryContractJSON,
				".verdi/specs/active/stale-decline/spec.md": alignSpecMD,
			},
			Message: "scaffold + accepted spec",
		},
	})
	checkoutBranch(t, repo.Dir, "feature/stale-decline")
	return repo
}

// alignFakeJudgeOK writes a tiny shell script honoring S5's envelope shape
// (mirrors internal/align's own writeFakeJudge; duplicated here since
// cmd/verdi cannot import an internal/align _test.go helper).
func alignFakeJudgeOK(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-1\\\",\\\"text\\\":\\\"looks aligned\\\",\\\"confidence\\\":0.9}]}\"}\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

func alignFakeJudgeFailing(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

// boundaryWriteRunnerCmd wraps a Runner to simulate `flowmap boundary`'s
// real side effect (spike S1: it writes the contract file in place, no
// stdout mode), writing back the SAME contract the fixture already
// committed — no drift, so the declared boundary holds both at acceptance
// and at the regenerated build head. Package-local mirror of
// internal/align's own boundaryWriteRunner test double.
type boundaryWriteRunnerCmd struct {
	upstream.Runner
	svcDir         string
	branchContract []byte
}

func (r boundaryWriteRunnerCmd) Run(ctx context.Context, req upstream.Request) (upstream.Result, error) {
	res, err := r.Runner.Run(ctx, req)
	if err == nil && req.Bin == "flowmap" && req.Subcommand == "boundary" {
		_ = os.MkdirAll(filepath.Join(r.svcDir, ".flowmap"), 0o755)
		_ = os.WriteFile(filepath.Join(r.svcDir, ".flowmap", "boundary-contract.json"), r.branchContract, 0o644)
	}
	return res, err
}

func alignRunner(svcDir string) upstream.Runner {
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: []byte("{}"), ExitCode: 0})
	fr.Enqueue("flowmap", "boundary", upstream.Result{ExitCode: 0})
	return boundaryWriteRunnerCmd{Runner: fr, svcDir: svcDir, branchContract: []byte(alignAcceptanceBoundaryContractJSON)}
}

func readReport(t *testing.T, root string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "specs", "active", "stale-decline", "deviation-report.md"))
	if err != nil {
		t.Fatalf("reading deviation-report.md: %v", err)
	}
	return data
}

// TestRunAlign_WritesReport proves the full wiring: cmdAlign's testable
// core resolves the build-head spec from the branch, regenerates via the
// injected FakeRunner, runs the fake judge, and writes a decodable
// deviation-report.md into the spec's directory.
func TestRunAlign_WritesReport(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t)}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlign = %d, want 0; stderr=%s", got, stderr.String())
	}

	data := readReport(t, repo.Dir)
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		t.Fatalf("DecodeDeviation(written report): %v\n%s", err, data)
	}
	if decoded.Covers != repo.Head {
		t.Fatalf("Covers = %q, want %q", decoded.Covers, repo.Head)
	}
}

// TestRunAlign_ByteIdenticalAcrossRuns is the cmd-level analogue of
// internal/align's own determinism test, proving the full write-to-disk
// path (not just Generate in isolation) is byte-identical across runs
// against the same tree/commit.
func TestRunAlign_ByteIdenticalAcrossRuns(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	judgeCmd := alignFakeJudgeOK(t)

	run := func() []byte {
		deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: judgeCmd}
		var stdout, stderr bytes.Buffer
		if got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr); got != 0 {
			t.Fatalf("runAlign = %d, want 0; stderr=%s", got, stderr.String())
		}
		return readReport(t, repo.Dir)
	}

	first := run()
	second := run()
	if !bytes.Equal(first, second) {
		t.Fatalf("deviation-report.md not byte-identical across runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestRunAlign_JudgeRequiredAndFailing_ExitsOne proves
// align.judge_required: true with a judge that fails makes `verdi align`
// itself exit non-zero (exit 1, PLAN.md Phase 8's exit criteria), and
// writes no report.
func TestRunAlign_JudgeRequiredAndFailing_ExitsOne(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeFailing(t), JudgeRequired: true}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAlign(judge_required, failing judge) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")); err == nil {
		t.Fatal("deviation-report.md was written despite the judge_required failure")
	}
}

// TestRunAlign_JudgeRequiredAndNotConfigured_ExitsOne is the "no judge at
// all" half of the same requirement.
func TestRunAlign_JudgeRequiredAndNotConfigured_ExitsOne(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeRequired: true}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runAlign(judge_required, no judge_cmd) = %d, want 1; stderr=%s", got, stderr.String())
	}
}

// TestRunAlign_Freeze proves --freeze writes a Frozen stamp and a second
// run against the now-frozen report refuses (exit 1) rather than silently
// overwriting an immutable artifact.
func TestRunAlign_Freeze(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t)}

	var stdout, stderr bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, true, deps, &stdout, &stderr); got != 0 {
		t.Fatalf("runAlign(--freeze) = %d, want 0; stderr=%s", got, stderr.String())
	}
	fm, _, err := artifact.SplitFrontmatter(readReport(t, repo.Dir))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		t.Fatalf("DecodeDeviation: %v", err)
	}
	if decoded.Frozen == nil {
		t.Fatal("no Frozen stamp after --freeze")
	}

	var stdout2, stderr2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout2, &stderr2)
	if got != 1 {
		t.Fatalf("runAlign after freeze = %d, want 1 (frozen reports are immutable); stderr=%s", got, stderr2.String())
	}
}

// TestRunAlign_DispositionPreservation proves the cmd-level round trip: a
// human edits the written report's disposition, and a second `verdi align`
// run against the same tree/commit preserves it.
func TestRunAlign_DispositionPreservation(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t)}

	var stdout, stderr bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr); got != 0 {
		t.Fatalf("runAlign (first) = %d, want 0; stderr=%s", got, stderr.String())
	}

	path := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	// A human dispositions the computed "holds" finding by hand-editing the
	// file — exactly what `verdi align` reads back on the next run.
	edited := bytes.Replace(data, []byte("regenerated boundary contract)\" }"), []byte("regenerated boundary contract)\", disposition: fixed }"), 1)
	if bytes.Equal(edited, data) {
		t.Fatal("test setup: expected finding text not found in the first report")
	}
	if err := os.WriteFile(path, edited, 0o644); err != nil {
		t.Fatalf("writing edited report: %v", err)
	}

	var stdout2, stderr2 bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, deps, &stdout2, &stderr2); got != 0 {
		t.Fatalf("runAlign (second) = %d, want 0; stderr=%s", got, stderr2.String())
	}

	fm, _, err := artifact.SplitFrontmatter(readReport(t, repo.Dir))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		t.Fatalf("DecodeDeviation: %v", err)
	}
	found := false
	for _, f := range decoded.Findings {
		if f.Kind == artifact.FindingComputed && f.Dispositioned() {
			found = true
		}
	}
	if !found {
		t.Fatalf("the hand-added disposition did not survive regeneration: %+v", decoded.Findings)
	}
}

// TestRunAlign_Negative covers operational failures independent of any
// fixture: not on a build branch, and cmdAlign's own usage parsing.
func TestRunAlign_Negative(t *testing.T) {
	t.Run("not a build branch", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "m"}})
		var stdout, stderr bytes.Buffer
		got := runAlign(context.Background(), repo.Dir, false, alignDeps{}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("runAlign(not a build branch) = %d, want 2; stderr=%s", got, stderr.String())
		}
	})

	t.Run("unexpected argument", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdAlign([]string{"--freeze", "extra"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdAlign(unexpected extra arg) = %d, want 2", got)
		}
	})

	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdAlign(nil, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdAlign(no store root) = %d, want 2", got)
		}
	})
}
