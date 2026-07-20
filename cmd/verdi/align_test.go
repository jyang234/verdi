package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/storyresolve"
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
// testResolveModelDigest is cmd/verdi's test-support call of the same
// resolveModelDigest (forgeboot.go) production verbs use to populate
// alignDeps.ModelDigest — every fixture built by this file has a real
// .verdi/verdi.yaml and no .verdi/model.yaml, so this resolves to
// model.Canonical()'s own digest, exactly what cmdAlign would compute for
// the same store.
func testResolveModelDigest(t *testing.T, root string) string {
	t.Helper()
	digest, err := resolveModelDigest(root)
	if err != nil {
		t.Fatalf("resolveModelDigest(%s): %v", root, err)
	}
	return digest
}

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

// alignFakeJudgeNewlineText writes a fake judge whose finding text carries
// an embedded newline (ADJ-53's j-4 fixture) — a legitimate, if rare, judge
// response shape (a judge is free-text; S5's own contract never constrains
// it to a single line) that internal/align's ingestion must normalize
// (internal/align/judge.go's normalizeJudgeText) rather than pass through
// raw, since align.RenderFindingLine's single-line bullet and the
// disposition verb's whole-line matcher both assume one line per finding.
func alignFakeJudgeNewlineText(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-newline\\\",\\\"text\\\":\\\"line one\\\\nline two\\\",\\\"confidence\\\":0.4}]}\"}\nEOF\n"
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

// alignFakeJudgeSlowOK writes a fake judge that sleeps briefly (well under
// any --wait bound this file uses) before emitting a valid OK envelope —
// used to prove a bounded wait blocks then completes normally, and that a
// concurrent reader never observes partial report content while the judge
// is still running.
func alignFakeJudgeSlowOK(t *testing.T, sleepSeconds int) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := fmt.Sprintf("#!/bin/sh\nsleep %d\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-1\\\",\\\"text\\\":\\\"looks aligned\\\",\\\"confidence\\\":0.9}]}\"}\nEOF\n", sleepSeconds)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

// alignFakeJudgeSentinel writes a fake judge that touches sentinelPath the
// MOMENT it starts running, then sleeps briefly before emitting a valid OK
// envelope — lets a test detect "the judge subprocess has actually started"
// without racing on timing alone (spec/judge-ergonomics ac-1: proving the
// report path is on stdout BEFORE the judge subprocess ever runs).
func alignFakeJudgeSentinel(t *testing.T, sentinelPath string) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := fmt.Sprintf("#!/bin/sh\ntouch %q\nsleep 1\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[{\\\"id\\\":\\\"j-1\\\",\\\"text\\\":\\\"looks aligned\\\",\\\"confidence\\\":0.9}]}\"}\nEOF\n", sentinelPath)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

// syncBuffer is a concurrency-safe io.Writer + String() over a bytes.Buffer
// — align_test.go's own tests need to read stdout WHILE runAlign is still
// executing in another goroutine (proving ordering, not just end-state),
// which a bare bytes.Buffer does not support safely.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
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
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}

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
		deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: judgeCmd, ModelDigest: testResolveModelDigest(t, repo.Dir)}
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
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeFailing(t), JudgeRequired: true, ModelDigest: testResolveModelDigest(t, repo.Dir)}

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
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeRequired: true, ModelDigest: testResolveModelDigest(t, repo.Dir)}

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
		ModelDigest:   testResolveModelDigest(t, repo.Dir),
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
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}

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
	living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
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
	frozenDeps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeDrift(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
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
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}

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

// TestRunAlign_RegeneratePreservesGenuineReportOnJudgeFailure is D6-24's
// regression proof for the ordinary (non-freeze) regenerate path: witnessed
// in round 6, a re-run whose judge timed out overwrote a living report
// carrying a genuine judge exchange (2 real findings + dispositions) with a
// synthetic judged-coverage-absent finding, destroying both. An align
// regeneration must never do that: when a genuine prior exchange
// (judge_integrity present) exists on disk and this run's judge fails to
// produce one, keep the prior report byte-for-byte and exit 2 (an
// operational failure — the judge failing to run is not a verdict), rather
// than silently destroying the last genuine exchange. PR #99's
// align.FreezeInPlace already covers the --freeze path; this is its
// ordinary-regenerate analogue (cmd/verdi's runAlignForSpec).
func TestRunAlign_RegeneratePreservesGenuineReportOnJudgeFailure(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	// A living align: the judge succeeds genuinely (judge_integrity recorded).
	living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, living, &out, &errb); got != 0 {
		t.Fatalf("runAlign (living) = %d, want 0; stderr=%s", got, errb.String())
	}

	// The human dispositions every finding — the judged one as an
	// owner-ratified accepted-deviation, the computed one(s) as fixed — the
	// witness's "2 real findings + dispositions" shape.
	fm := decodeReportFile(t, reportPath)
	for i := range fm.Findings {
		if fm.Findings[i].Kind == artifact.FindingJudged {
			fm.Findings[i].Disposition = artifact.FindingAcceptedDeviation
			fm.Findings[i].Note = "owner-ratified: intentional deviation"
		} else {
			fm.Findings[i].Disposition = artifact.FindingFixed
		}
	}
	_, body, err := artifact.SplitFrontmatter(readReport(t, repo.Dir))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if err := os.WriteFile(reportPath, align.RenderMarkdown(fm, string(body)), 0o644); err != nil {
		t.Fatalf("writing dispositioned living report: %v", err)
	}
	genuineBefore, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading dispositioned living report: %v", err)
	}

	// Re-run align (NOT --freeze) with a judge that now fails outright — the
	// witness's "timed out at the 2m ceiling" stand-in; any judge failure
	// takes the same absent-result path (judged.go's RunJudged).
	failingDeps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeFailing(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out2, errb2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, failingDeps, &out2, &errb2)

	if got != 2 {
		t.Fatalf("runAlign (regenerate, judge failing, genuine prior) = %d, want 2 (operational failure); stdout=%s stderr=%s", got, out2.String(), errb2.String())
	}
	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading report after failed regenerate: %v", err)
	}
	if !bytes.Equal(genuineBefore, after) {
		t.Fatalf("genuine living report was NOT preserved byte-for-byte across a failed-judge regeneration:\n--- before ---\n%s\n--- after ---\n%s", genuineBefore, after)
	}
	if !strings.Contains(errb2.String(), "D6-24") {
		t.Fatalf("stderr = %q, want a loud disclosure naming why the report was preserved (D6-24)", errb2.String())
	}
}

// TestRunAlign_NoPriorReport_JudgeFailure_WritesSynthetic is D6-24's fix
// negative-path proof: with no prior report on disk, there is nothing
// genuine to lose, so a failing (non-required) judge on a first-ever run
// must still degrade to the synthetic absence finding and succeed (exit 0),
// exactly as before this fix.
func TestRunAlign_NoPriorReport_JudgeFailure_WritesSynthetic(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeFailing(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &out, &errb)
	if got != 0 {
		t.Fatalf("runAlign (first run, no prior report, failing judge) = %d, want 0 (nothing genuine to lose); stderr=%s", got, errb.String())
	}

	fm := decodeReportFile(t, reportPath)
	if _, ok := findingByID(fm.Findings, align.AbsenceFindingID); !ok {
		t.Fatalf("expected the synthetic absence finding %s, got %+v", align.AbsenceFindingID, fm.Findings)
	}
	if fm.JudgeIntegrity != nil {
		t.Fatalf("synthetic report unexpectedly carries judge_integrity: %+v", fm.JudgeIntegrity)
	}
}

// TestRunAlign_PriorSynthetic_JudgeStillFailing_RegeneratesNormally is D6-24's
// fix negative-path proof for the OTHER "nothing genuine to lose" case: a
// prior report that is itself synthetic (no judge_integrity) has nothing
// genuine on disk either, so a still-failing judge must regenerate and
// overwrite it normally, exactly as before this fix.
func TestRunAlign_PriorSynthetic_JudgeStillFailing_RegeneratesNormally(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	// First run: no judge configured at all -> synthetic, no judge_integrity.
	firstDeps := alignDeps{Runner: alignRunner(svcDir), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, firstDeps, &out, &errb); got != 0 {
		t.Fatalf("runAlign (first, no judge configured) = %d, want 0; stderr=%s", got, errb.String())
	}
	firstReport := decodeReportFile(t, reportPath)
	if firstReport.JudgeIntegrity != nil {
		t.Fatalf("test setup: first report unexpectedly genuine: %+v", firstReport.JudgeIntegrity)
	}

	// Second run: judge now configured but fails outright -> still synthetic;
	// since the prior report was ITSELF synthetic (nothing genuine on disk),
	// today's plain-overwrite behavior must stand.
	secondDeps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeFailing(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out2, errb2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, secondDeps, &out2, &errb2)
	if got != 0 {
		t.Fatalf("runAlign (second, prior synthetic, judge failing) = %d, want 0 (regenerate as today); stderr=%s", got, errb2.String())
	}
	secondReport := decodeReportFile(t, reportPath)
	if _, ok := findingByID(secondReport.Findings, align.AbsenceFindingID); !ok {
		t.Fatalf("expected the regenerated report to still carry the synthetic absence finding: %+v", secondReport.Findings)
	}
}

// TestRunAlign_RegenerateWithGenuineJudgeCompletion_RegeneratesNormally is
// D6-24's fix boundary proof: a genuine prior report followed by a SECOND
// genuine judge completion (even a drifted one — a different id/text,
// mirroring the judge's own non-reproducibility) is ordinary regeneration,
// entirely unaffected by this fix. Genuine-to-genuine replacement —
// including the finding-identity drift this exposes — is explicitly out of
// scope for D6-24 (its own second half); PreserveDispositions' existing
// behavior must stand untouched.
func TestRunAlign_RegenerateWithGenuineJudgeCompletion_RegeneratesNormally(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, living, &out, &errb); got != 0 {
		t.Fatalf("runAlign (living) = %d, want 0; stderr=%s", got, errb.String())
	}

	driftDeps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeDrift(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out2, errb2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, driftDeps, &out2, &errb2)
	if got != 0 {
		t.Fatalf("runAlign (regenerate, genuine drifted judge) = %d, want 0 (unaffected by D6-24's fix); stderr=%s", got, errb2.String())
	}

	fm := decodeReportFile(t, reportPath)
	if fm.JudgeIntegrity == nil {
		t.Fatal("regenerated report lost its genuine judge_integrity — the fix must not block a genuine judge completion")
	}
	if _, ok := findingByID(fm.Findings, "judged-j-drift"); !ok {
		t.Fatalf("regenerated report does not carry the new judge's drifted finding: %+v", fm.Findings)
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

// TestCmdAlign_WaitFlagParsing_Negative proves --wait[=seconds]'s usage
// validation fails closed (exit 2, a named reason) BEFORE any store/judge
// work happens — mirroring TestRunAlign_Negative's "unexpected argument"/"no
// store root" shape, which also runs from an empty temp dir with no
// fixture, since bad syntax is rejected during argument parsing itself.
func TestCmdAlign_WaitFlagParsing_Negative(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"non-numeric seconds", []string{"--wait=abc"}},
		{"zero seconds", []string{"--wait=0"}},
		{"negative seconds", []string{"--wait=-5"}},
		{"fractional seconds", []string{"--wait=1.5"}},
		{"wait then diagram-sweep", []string{"--wait", "--diagram-sweep", "diagram/foo"}},
		{"diagram-sweep then wait", []string{"--diagram-sweep", "diagram/foo", "--wait=5"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var stdout, stderr bytes.Buffer
			got := cmdAlign(tc.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdAlign(%v) = %d, want 2; stdout=%s stderr=%s", tc.args, got, stdout.String(), stderr.String())
			}
			if stderr.String() == "" {
				t.Fatalf("cmdAlign(%v): stderr is empty, want a named reason (silence is never a pass)", tc.args)
			}
		})
	}
}

// TestCmdAlign_WaitBelowJudgeCeiling_RefusedAsUsageError is FIX 1's
// (finding judged-wait-bound-conflated-with-judge-kill-timeout) red-first
// pin: --wait exists to EXTEND how long align waits for the judge, never to
// truncate the judge's own run, so --wait=N below the resolved judge ceiling
// (here the built-in align.DefaultJudgeTimeout, since buildAlignRepo
// configures no align.judge_timeout_seconds) is a usage error (exit 2) that
// names BOTH the rejected bound and the ceiling — not a silent fold of N
// into the judge's exec timeout that kills a judge which would otherwise
// complete and gracefully degrade. This runs through cmdAlign (not runAlign)
// because the ceiling is only known after the manifest is resolved.
func TestCmdAlign_WaitBelowJudgeCeiling_RefusedAsUsageError(t *testing.T) {
	repo := buildAlignRepo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdAlign([]string{"--wait=1"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdAlign(--wait=1, default %s ceiling) = %d, want 2 (usage error); stderr=%s", align.DefaultJudgeTimeout, got, stderr.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "--wait=1") {
		t.Fatalf("stderr = %q, want it to name the rejected bound (--wait=1)", msg)
	}
	ceilingSecs := int(align.DefaultJudgeTimeout / time.Second)
	if !strings.Contains(msg, fmt.Sprintf("%d", ceilingSecs)) {
		t.Fatalf("stderr = %q, want it to name the judge ceiling (%ds) it fell below", msg, ceilingSecs)
	}
	// The refusal is pre-flight (a usage error), so nothing may be written.
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")); err == nil {
		t.Fatal("deviation-report.md was written despite the --wait usage refusal")
	}
}

// TestCmdAlign_WaitAtOrAboveCeiling_NotRefused pins the other half of the
// adjudication (finding judged-wait-bound-conflated-with-judge-kill-timeout):
// a bare --wait (waits exactly the ceiling) and --wait=N with N >= the
// ceiling stay legal — they must clear the usage guard into the run itself
// (where this toolchain-less fixture then fails at Compute for an unrelated
// reason). The proof they cleared the guard: the report path reached stdout
// (runAlignForSpec prints it before any Compute/judge work) and the ceiling
// refusal never fired. Guards against a future over-strict guard.
func TestCmdAlign_WaitAtOrAboveCeiling_NotRefused(t *testing.T) {
	ceilingSecs := int(align.DefaultJudgeTimeout / time.Second)
	cases := [][]string{
		{"--wait"},                                // bare: waits exactly the ceiling
		{fmt.Sprintf("--wait=%d", ceilingSecs)},   // == ceiling
		{fmt.Sprintf("--wait=%d", ceilingSecs*2)}, // > ceiling (extends)
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			repo := buildAlignRepo(t)
			t.Chdir(repo.Dir)
			var stdout, stderr bytes.Buffer
			cmdAlign(args, &stdout, &stderr)
			if strings.Contains(stderr.String(), "patience ceiling") {
				t.Fatalf("cmdAlign(%v) was refused by the ceiling guard, want it legal; stderr=%s", args, stderr.String())
			}
			reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
			if !strings.Contains(stdout.String(), reportPath) {
				t.Fatalf("cmdAlign(%v): report path not on stdout — a legal --wait must clear the guard into the run; stdout=%s stderr=%s", args, stdout.String(), stderr.String())
			}
		})
	}
}

// TestRunAlign_Wait_ExpiryMessageStatesJudgeTerminated is FIX 1's second
// red-first pin (same finding): the wait-expiry message must state what
// actually happened to the judge subprocess — it was terminated at the bound
// and cannot complete this run — and must NOT carry the old "check it later"
// phrasing, which lied, since a killed judge never populates the printed path
// on its own. Driven at the runAlign layer (a short injected JudgeTimeout is
// the hermetic stand-in for a real bound) so the message text is asserted
// directly.
func TestRunAlign_Wait_ExpiryMessageStatesJudgeTerminated(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t), // sleeps 5s
		Wait: true, JudgeTimeout: 200 * time.Millisecond, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAlign(Wait, hung judge) = %d, want 2; stderr=%s", got, stderr.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "terminated") {
		t.Fatalf("expiry stderr = %q, want it to state the judge subprocess was terminated at the bound", msg)
	}
	if strings.Contains(msg, "check it later") {
		t.Fatalf("expiry stderr = %q, still carries the misleading \"check it later\" phrasing (a terminated judge never populates the path on its own)", msg)
	}
}

// TestRunAlign_Wait_RejectedOnDesignBranch proves --wait is explicitly
// refused (never silently ignored) on a design branch: spec/judge-ergonomics
// scopes --wait to build-branch align and close's freeze-align only
// (decision-conflict mode is untouched by this story), so a caller that
// passes --wait there must be told loudly, not have the flag quietly do
// nothing.
func TestRunAlign_Wait_RejectedOnDesignBranch(t *testing.T) {
	repo := buildAlignDesignRepo(t)
	deps := alignDeps{Wait: true}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAlign(design branch, Wait) = %d, want 2; stderr=%s", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--wait") {
		t.Fatalf("stderr = %q, want it to name --wait as the reason", stderr.String())
	}
}

// TestRunAlign_ReportPathPrintedBeforeJudgeRuns proves spec/judge-ergonomics
// ac-1's ordering claim behaviorally, not just by code inspection: the
// report path is stdout's first line WHILE the judge subprocess is
// confirmed still running (a sentinel file the fake judge touches the
// instant it starts), and the report itself does not exist on disk yet at
// that same moment.
func TestRunAlign_ReportPathPrintedBeforeJudgeRuns(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	sentinel := filepath.Join(t.TempDir(), "judge-started")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSentinel(t, sentinel), ModelDigest: testResolveModelDigest(t, repo.Dir)}

	var stdout syncBuffer
	var stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sentinel); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("judge subprocess never started (sentinel file never appeared)")
		}
		time.Sleep(5 * time.Millisecond)
	}

	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	if firstLine != reportPath {
		t.Fatalf("stdout first line = %q while the judge subprocess was already confirmed running; want the report path %q printed BEFORE the judge runs", firstLine, reportPath)
	}
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatal("deviation-report.md already exists while the judge subprocess is still mid-run — the path must be observable before the file is")
	}

	got := <-done
	if got != 0 {
		t.Fatalf("runAlign = %d, want 0; stderr=%s", got, stderr.String())
	}
}

// TestRunAlign_ReportNeverPartiallyObservable proves spec/judge-ergonomics
// ac-1's atomic-write claim: a reader polling the report path throughout a
// run (against a judge slow enough to give the poller a real window) must
// only ever observe "does not exist yet" or a fully frontmatter-decodable
// file — never a truncated/partial write, because align.go now writes
// through the atomicfile seam (temp-then-rename) instead of a raw
// os.WriteFile.
func TestRunAlign_ReportNeverPartiallyObservable(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSlowOK(t, 1), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	stop := make(chan struct{})
	var pollErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(reportPath)
			if err == nil {
				if _, _, splitErr := artifact.SplitFrontmatter(data); splitErr != nil {
					pollErr = fmt.Errorf("observed non-final content mid-run: %v\n%s", splitErr, data)
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	close(stop)
	wg.Wait()

	if got != 0 {
		t.Fatalf("runAlign = %d, want 0; stderr=%s", got, stderr.String())
	}
	if pollErr != nil {
		t.Fatal(pollErr)
	}
}

// TestRunAlign_Wait_CompletesWithinBound proves spec/judge-ergonomics ac-2's
// completing half at the cmd layer: Wait true, a JudgeTimeout comfortably
// longer than the judge's own delay, blocks internally then exits 0 with
// the report written once the judge finishes — ordinary success, just
// bounded.
func TestRunAlign_Wait_CompletesWithinBound(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSlowOK(t, 1),
		Wait: true, JudgeTimeout: 10 * time.Second, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlign(Wait, completing judge) = %d, want 0; stderr=%s", got, stderr.String())
	}
	fm := decodeReportFile(t, filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md"))
	if fm.JudgeIntegrity == nil {
		t.Fatal("report carries no judge_integrity — the judge's genuine completion under Wait was not recorded")
	}
}

// TestRunAlign_Wait_ExpiresExitsTwoWithPathPrinted is the plan's own named
// case: --wait's bound (here via a short injected JudgeTimeout, the
// cmd-level equivalent of --wait=1) against a hung fake judge exits 2 (an
// operational timeout, not a verdict), promptly, with the report path
// already on stdout's first line and — since this is a first-ever run — no
// report file written at all.
func TestRunAlign_Wait_ExpiresExitsTwoWithPathPrinted(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t), // sleeps 5s
		Wait: true, JudgeTimeout: 200 * time.Millisecond, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	start := time.Now()
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	elapsed := time.Since(start)

	if got != 2 {
		t.Fatalf("runAlign(Wait, hung judge) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if elapsed > 4*time.Second {
		t.Fatalf("runAlign(Wait=200ms) took %s, want it bounded near the timeout, not the sleep 5", elapsed)
	}
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	if firstLine != reportPath {
		t.Fatalf("stdout first line = %q, want the report path %q printed even though the run expired", firstLine, reportPath)
	}
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatal("deviation-report.md was written despite --wait expiring — nothing genuine to write on an operational timeout")
	}
}

// TestRunAlign_Wait_ExpiryPreservesExistingReport is
// TestRunAlign_Wait_ExpiresExitsTwoWithPathPrinted's regeneration analogue,
// mirroring D6-24's own preserve-on-failure shape: a REGENERATE run whose
// judge expires under Wait must leave a pre-existing genuine report
// byte-for-byte untouched, never overwritten with a partial or synthetic
// edition.
func TestRunAlign_Wait_ExpiryPreservesExistingReport(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")

	living := alignDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeOK(t), ModelDigest: testResolveModelDigest(t, repo.Dir)}
	var out, errb bytes.Buffer
	if got := runAlign(context.Background(), repo.Dir, false, living, &out, &errb); got != 0 {
		t.Fatalf("runAlign (living) = %d, want 0; stderr=%s", got, errb.String())
	}
	genuineBefore, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading living report: %v", err)
	}

	waitDeps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t),
		Wait: true, JudgeTimeout: 200 * time.Millisecond, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}
	var out2, errb2 bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, waitDeps, &out2, &errb2)
	if got != 2 {
		t.Fatalf("runAlign (regenerate, Wait expires) = %d, want 2; stdout=%s stderr=%s", got, out2.String(), errb2.String())
	}
	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading report after expired regenerate: %v", err)
	}
	if !bytes.Equal(genuineBefore, after) {
		t.Fatalf("genuine living report was NOT preserved byte-for-byte across a --wait expiry:\n--- before ---\n%s\n--- after ---\n%s", genuineBefore, after)
	}
}

// TestRunAlign_Wait_DefaultOffPreservesGracefulDegrade is the cmd-level
// regression pin complementing internal/align's own
// TestRunJudged_WaitFalse_TimeoutStillDegrades: with Wait left at its zero
// value (no --wait passed), a judge timeout must still degrade to the
// synthetic absence finding and exit 0, exactly as before this story.
func TestRunAlign_Wait_DefaultOffPreservesGracefulDegrade(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t),
		JudgeTimeout: 200 * time.Millisecond, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	got := runAlign(context.Background(), repo.Dir, false, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlign(Wait=false, timeout) = %d, want 0 (graceful degrade unchanged); stderr=%s", got, stderr.String())
	}
	fm := decodeReportFile(t, filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md"))
	if _, ok := findingByID(fm.Findings, align.AbsenceFindingID); !ok {
		t.Fatalf("expected the synthetic absence finding, got %+v", fm.Findings)
	}
}

// TestRunAlignForSpec_CloseFreezeAlign_WaitExpires proves spec/judge-ergonomics
// ac-3 directly: runAlignForSpec is the EXACT function and call shape
// close.go's runClose/runCloseFeature use for freeze-align
// (runAlignForSpec(ctx, root, spec, head, true, alignD, stdout, stderr)) — this
// test calls it the same way, with Wait set, proving close's freeze-align
// inherits the identical first-line path, atomic-write, and bounded
// --wait/exit-2-with-path-on-expiry behavior from the one shared engine
// hook, not a second implementation.
func TestRunAlignForSpec_CloseFreezeAlign_WaitExpires(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec, err := storyresolve.ResolveBuildSpec(repo.Dir, "feature/stale-decline")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t),
		Wait: true, JudgeTimeout: 200 * time.Millisecond, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(context.Background(), repo.Dir, spec, repo.Head, true /* freeze — close's own call always passes true */, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runAlignForSpec(freeze=true, Wait, hung judge) = %d, want 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	if firstLine != reportPath {
		t.Fatalf("stdout first line = %q, want the report path %q — close's freeze-align must print it exactly like align's own CLI does", firstLine, reportPath)
	}
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatal("no frozen report should exist — close's freeze-align must not write on a --wait expiry either")
	}
}

// TestRunAlignForSpec_CloseFreezeAlign_WaitCompletes is
// TestRunAlignForSpec_CloseFreezeAlign_WaitExpires's completing half: the
// same close call shape (freeze=true) with a judge that finishes inside the
// bound writes a genuinely frozen report through the atomicfile seam and
// exits 0, exactly like align's own --wait success path.
func TestRunAlignForSpec_CloseFreezeAlign_WaitCompletes(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec, err := storyresolve.ResolveBuildSpec(repo.Dir, "feature/stale-decline")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}
	deps := alignDeps{
		Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSlowOK(t, 1),
		Wait: true, JudgeTimeout: 10 * time.Second, ModelDigest: testResolveModelDigest(t, repo.Dir),
	}

	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(context.Background(), repo.Dir, spec, repo.Head, true, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runAlignForSpec(freeze=true, Wait, completing judge) = %d, want 0; stderr=%s", got, stderr.String())
	}
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]
	if firstLine != reportPath {
		t.Fatalf("stdout first line = %q, want the report path %q", firstLine, reportPath)
	}
	fm := decodeReportFile(t, reportPath)
	if fm.Frozen == nil {
		t.Fatal("close's freeze-align call (freeze=true) produced a report with no Frozen stamp")
	}
	if fm.JudgeIntegrity == nil {
		t.Fatal("report carries no judge_integrity — the judge's genuine completion under Wait was not recorded")
	}
}
