package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/upstream"
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

// alignFakeJudgeDrift writes a fake judge returning a DIFFERENT judged finding
// (id AND text) than alignFakeJudgeOK — the hermetic stand-in for the real,
// non-reproducible judge (03 §Alignment report) emitting fresh wording when
// re-run at freeze time. This is the D6-21-exposed condition: once
// judge_timeout_seconds rose past the judge's runtime, freeze stopped timing
// out into a stable synthetic finding and began re-judging, whose fresh
// content-hash identities PreserveDispositions cannot match. A faithful freeze
// must never let this drift reach the archived report.
func alignFakeJudgeDrift(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-drift\\\",\\\"text\\\":\\\"a fresh, differently-worded semantic reading\\\",\\\"confidence\\\":0.4}]}\"}\nEOF\n"
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

// alignFakeJudgeSleepy writes a fake judge that always sleeps 5s regardless
// of scheduling load (mirrors internal/align/judge_test.go's own
// fakeJudgeTimeoutScript) — used to prove a configured JudgeTimeout (D6-21)
// actually reaches the exec, rather than internal/align's own
// DefaultJudgeTimeout (120s) always winning.
func alignFakeJudgeSleepy(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 5\necho \"should never get here\"\n"), 0o755); err != nil {
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

// decodeReportFile reads, splits, and strict-decodes the deviation report at
// path — the read-back half of the freeze round trip.
func decodeReportFile(t *testing.T, path string) *artifact.DeviationFrontmatter {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatalf("SplitFrontmatter(%s): %v", path, err)
	}
	decoded, err := artifact.DecodeDeviation(fm)
	if err != nil {
		t.Fatalf("DecodeDeviation(%s): %v", path, err)
	}
	return decoded
}

func findingByID(fs []artifact.Finding, id string) (artifact.Finding, bool) {
	for _, f := range fs {
		if f.ID == id {
			return f, true
		}
	}
	return artifact.Finding{}, false
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

// TestRunAlign_ConfiguredJudgeTimeoutReachesInvocation proves D6-21's
// wiring end to end at the cmd layer: verdi.yaml's align.judge_timeout_seconds,
// threaded by cmdAlign into alignDeps.JudgeTimeout and from there into
// align.Input.JudgeTimeout, actually reaches the judge exec — not just
// internal/align's own DefaultJudgeTimeout (120s), which would make this
// test hang for two minutes instead of returning promptly. A short
// (100ms) configured JudgeTimeout against a judge that always sleeps 5s
// must fail at the timeout stage well within seconds, and (JudgeRequired:
// true) surface as `verdi align`'s own exit 1 — mirroring
// TestRunAlign_JudgeRequiredAndFailing_ExitsOne's shape and
// internal/align/judge_test.go's TestRunJudgeOnce_Timeout's timing
// reasoning (fakeJudgeTimeoutScript sleeps 5s regardless of scheduling
// load, so a 100ms timeout fires deterministically either way).
func TestRunAlign_ConfiguredJudgeTimeoutReachesInvocation(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{
		Runner:        alignRunner(svcDir),
		JudgeCmd:      alignFakeJudgeSleepy(t),
		JudgeRequired: true,
		JudgeTimeout:  100 * time.Millisecond,
	}

	var stdout, stderr bytes.Buffer
	start := time.Now()
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	elapsed := time.Since(start)

	if got != 1 {
		t.Fatalf("runAlign(configured 100ms timeout, sleepy judge, judge_required) = %d, want 1; stderr=%s", got, stderr.String())
	}
	if elapsed > 4*time.Second {
		t.Fatalf("runAlign took %s, want it to return promptly after the configured 100ms timeout, not wait for the sleep 5 or the 120s default", elapsed)
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

// TestRunAlign_FreezePreservesDispositions is the freeze-preservation
// regression proof (close-freeze fix): freezing a spec whose LIVING
// deviation-report is fresh and fully dispositioned must archive those exact
// findings + dispositions verbatim and NEVER re-run the judge. The judge is
// non-reproducible (03 §Alignment report), so a freeze-time re-run emits fresh
// content-hash finding identities that PreserveDispositions cannot match,
// silently erasing every human disposition (e.g. corpus-renovation's
// owner-ratified accepted-deviation). runClose freezes through this exact
// runAlignForSpec path, so proving it here covers `verdi close`'s freeze step.
func TestRunAlign_FreezePreservesDispositions(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	// A living align: the judge reads one judged finding (j-1, "looks aligned").
	living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, living, &out, &errb); got != 0 {
		t.Fatalf("runAlign (living) = %d, want 0; stderr=%s", got, errb.String())
	}

	// The human dispositions EVERY finding: the judged one as an owner-ratified
	// accepted-deviation (the ADJ-16 shape), the computed one(s) as fixed.
	// Decode → set → re-render keeps this robust to the fixture's exact finding
	// count while leaving covers == HEAD unchanged.
	fm := decodeReportFile(t, reportPath)
	var judgedID, judgedText string
	for i := range fm.Findings {
		if fm.Findings[i].Kind == artifact.FindingJudged {
			fm.Findings[i].Disposition = artifact.FindingAcceptedDeviation
			fm.Findings[i].Note = "owner-ratified: intentional, tracked separately"
			judgedID = fm.Findings[i].ID
			judgedText = fm.Findings[i].Text
		} else {
			fm.Findings[i].Disposition = artifact.FindingFixed
		}
	}
	if judgedID == "" {
		t.Fatal("test setup: living report carries no judged finding to disposition")
	}
	_, body, err := artifact.SplitFrontmatter(readReport(t, repo.Dir))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if err := os.WriteFile(reportPath, align.RenderMarkdown(fm, string(body)), 0o644); err != nil {
		t.Fatalf("writing dispositioned living report: %v", err)
	}

	// Freeze — with a judge that now DRIFTS (different id + text). A faithful
	// freeze must ignore it and stamp the adjudicated living report as-is.
	frozenDeps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeDrift(t)}
	var out2, errb2 bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, true, frozenDeps, &out2, &errb2); got != 0 {
		t.Fatalf("runAlign (freeze) = %d, want 0; stderr=%s", got, errb2.String())
	}

	frozen := decodeReportFile(t, reportPath)
	if frozen.Frozen == nil {
		t.Fatal("frozen report carries no Frozen stamp")
	}
	for _, f := range frozen.Findings {
		if !f.Dispositioned() {
			t.Fatalf("finding %s (%q) lost its disposition across freeze — freeze re-judged instead of preserving the adjudicated report: %+v", f.ID, f.Text, f)
		}
	}
	j, ok := findingByID(frozen.Findings, judgedID)
	if !ok {
		t.Fatalf("judged finding %s vanished from the frozen report (judge drift leaked through): %+v", judgedID, frozen.Findings)
	}
	if j.Text != judgedText || j.Disposition != artifact.FindingAcceptedDeviation {
		t.Fatalf("judged finding = {text:%q disposition:%q}, want the living {text:%q disposition:%q}", j.Text, j.Disposition, judgedText, artifact.FindingAcceptedDeviation)
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
