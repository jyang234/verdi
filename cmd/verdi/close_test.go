package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/provider/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// closeFixtureStorySpecMD is a story spec declaring [static, behavioral]
// evidence on its one AC — the exact shape spec/close-verb ac-3 targets: a
// verdi self-hosted story that can only fold to evidenced from `source: ci`
// records, never local/advisory ones (co-1).
const closeFixtureStorySpecMD = `---
id: spec/close-fixture
kind: spec
class: story
title: "Close fixture story"
status: accepted-pending-build
owners: [platform-team]
story: jira:CLOSE-1
problem: { text: "x", anchor: "#problem" }
outcome: { text: "y", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/loan-mgmt#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "the fixture behavior holds", evidence: [static, behavioral] }
frozen: { at: 2024-01-01, commit: ` + gateFakeFrozenCommit + `}
---
# Close fixture story
## Problem
x
## Outcome
y
`

// closeFixtureBindingsYAML binds the self-hosted producer's two fixed
// producer ids to the fixture story's own ac-1 — the same mechanism
// verdi.bindings.yaml wires for real at the repo root (selfevidence.go),
// exercised here end to end: producing the evidence AND closing on it in
// one test, rather than asserting the two phases only in isolation.
const closeFixtureBindingsYAML = `schema: verdi.bindings/v1
spec: spec/close-fixture
bindings:
  - { producer: verdi-verify-behavioral, kind: behavioral, acs: [ac-1] }
  - { producer: verdi-verify-static, kind: static, acs: [ac-1] }
`

// buildCloseFixtureRepo builds a fixturegit repo carrying: the target
// feature (loan-mgmt, reused from cascadecheck_test.go's featureV1SpecMD),
// the fixture story implementing it, and a root verdi.bindings.yaml —
// everything `verdi close` needs except the evidence itself, which each
// test writes (or produces) separately so both the happy and the
// not-eligible paths can share this one builder.
func buildCloseFixtureRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: github\n",
			".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
			".verdi/specs/active/close-fixture/spec.md": closeFixtureStorySpecMD,
			"verdi.bindings.yaml":                       closeFixtureBindingsYAML,
		},
		Message: "close fixture: feature + story + self-hosted bindings",
	}})
}

// writeCloseGateReport writes deviation-report.md directly into the
// close-fixture spec's own directory (X-13/X-16/X-17's closure-gate
// condition 4 needs a living, fully-dispositioned, head-covering report
// before close will freeze rather than refuse) — writeGateReport
// (gate_test.go) hardcodes "stale-decline" (that file's own fixture
// family), so this story's differently-named fixture needs its own copy of
// the same plain-write shape (never git-committed, read via os.ReadFile
// exactly as a real `verdi align` run before its own commit would leave
// it) — mirroring closepreflight_test.go's own writePreflightGateReport
// precedent for the identical reason.
func writeCloseGateReport(t *testing.T, root, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "close-fixture")
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

// poisonRecord renders a source:local, verdict:fail record for ac-1 —
// planted alongside the authoritative source:ci evidence in the happy-path
// test to prove co-1 for real: a violating LOCAL record must never affect
// the fold or block closure (only source: ci is ever consulted).
func poisonRecord(commit string) string {
	return `{"schema":"verdi.evidence/v1","evidence_for":["ac-1"],"kind":"static","verdict":"fail","witness":"a local, advisory, non-authoritative run","provenance":{"source":"local","commit":"` + commit + `"},"digest":"sha256:` + strings.Repeat("a", 64) + `"}`
}

// writePoisonLocalRecord appends poisonRecord to whatever verdicts.json
// already exists (produceSelfHostedEvidence's own source:ci records) at
// specRef's derived directory for commit — a plain JSON-array splice, since
// this is test-only fixture assembly, not production code.
func writePoisonLocalRecord(t *testing.T, root, specRef, commit string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specRef), commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "verdicts.json")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	var combined string
	if len(existing) == 0 {
		combined = "[" + poisonRecord(commit) + "]"
	} else {
		trimmed := strings.TrimSpace(string(existing))
		combined = strings.TrimSuffix(trimmed, "]") + "," + poisonRecord(commit) + "]"
	}
	if err := os.WriteFile(path, []byte(combined+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunClose_EndToEnd is the load-bearing hermetic proof (spec/close-verb
// ac-1, ac-2, ac-3): the self-hosted producer feeds a story declaring
// [static, behavioral] evidence all the way to evidenced on source: ci
// alone (a poisoned source: local fail record is planted and proven inert,
// co-1); `verdi close` then runs the closure gate (a reachable, empty fake
// forge — no disclosure, no open MRs; a living, fully-dispositioned report
// already covering head, so condition 4 — X-13/X-16/X-17 — passes and the
// freeze step genuinely takes the freeze-in-place path, D6-24), freezes the
// alignment report, builds and digests a real rollup.json, moves the whole
// quartet to specs/archive/ as a byte-identical (git-pure-rename) move,
// commits it on a closure branch, and publishes the rollup to the fake
// provider, which reads it back.
//
// guide-claim: 7.4-gate-close-rollup
func TestRunClose_EndToEnd(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()

	// Phase 2's producer, exercised for real: this is what makes a story
	// declaring [static, behavioral] reach evidenced from CI records alone.
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}
	// co-1, proven with a witness: a violating LOCAL record must never
	// gate closure.
	writePoisonLocalRecord(t, repo.Dir, "spec/close-fixture", repo.Head)
	// The corrected closure ritual (X-16): align (a living report covering
	// head) -> disposition (working-tree edit) -> close. dispositionedFindingYAML
	// (gate_test.go) is the same minimal, already-dispositioned filler every
	// merge-gate happy-path test already uses.
	writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	fp := fake.New()
	fg := forgefake.New() // reachable, no open MRs seeded: condition 3 passes outright
	manifest := &store.Manifest{}
	// A story spec carries no impacts: (02 §Kind registry: feature-only
	// field), so align.Compute's regeneration loop never actually iterates
	// — but Compute still requires a non-nil Runner unconditionally
	// (internal/align/computed.go), so an empty FakeRunner satisfies that
	// precondition without ever being called.
	deps := closeDeps{Forge: fg, Registry: fp, Runner: upstream.NewFakeRunner()}

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-fixture", manifest, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	// The archived quartet exists; the pre-close active directory is gone.
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-fixture")); !os.IsNotExist(err) {
		t.Fatalf("specs/active/close-fixture still exists after close (err=%v)", err)
	}
	archiveDir := filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture")

	// spec.md moved active→archive with its status line flipped
	// accepted-pending-build→closed (D6-11; 02 §Kind registry's
	// "… → closed(archive)" transition) and NOTHING else changed — the only
	// content change VL-010 admits within the archive move.
	archivedSpec, err := os.ReadFile(filepath.Join(archiveDir, "spec.md"))
	if err != nil {
		t.Fatalf("reading archived spec.md: %v", err)
	}
	wantArchivedSpec := strings.Replace(closeFixtureStorySpecMD, "status: accepted-pending-build", "status: closed", 1)
	if string(archivedSpec) != wantArchivedSpec {
		t.Fatalf("archived spec.md is not the pre-close content with a sole status: closed flip:\n--- got ---\n%s\n--- want ---\n%s", archivedSpec, wantArchivedSpec)
	}
	if !strings.Contains(string(archivedSpec), "\nstatus: closed\n") {
		t.Fatalf("archived spec.md does not carry status: closed:\n%s", archivedSpec)
	}

	// deviation-report.md is frozen.
	devRaw, err := os.ReadFile(filepath.Join(archiveDir, "deviation-report.md"))
	if err != nil {
		t.Fatalf("reading archived deviation-report.md: %v", err)
	}
	devFm, _, err := artifact.SplitFrontmatter(devRaw)
	if err != nil {
		t.Fatal(err)
	}
	dev, err := artifact.DecodeDeviation(devFm)
	if err != nil {
		t.Fatalf("DecodeDeviation: %v", err)
	}
	if dev.Frozen == nil {
		t.Fatal("archived deviation-report.md has no Frozen stamp")
	}
	if dev.Covers != repo.Head {
		t.Fatalf("deviation-report.md covers %q, want %q", dev.Covers, repo.Head)
	}

	// rollup.json validates, its digest recomputes, and it reports ac-1
	// evidenced (the whole point of ac-3's self-hosted producer) and the
	// story eligible.
	rollRaw, err := os.ReadFile(filepath.Join(archiveDir, "rollup.json"))
	if err != nil {
		t.Fatalf("reading archived rollup.json: %v", err)
	}
	roll, err := artifact.DecodeRollup(rollRaw)
	if err != nil {
		t.Fatalf("DecodeRollup: %v", err)
	}
	if !roll.Eligible {
		t.Fatalf("rollup.Eligible = false, want true: %+v", roll)
	}
	if len(roll.Criteria) != 1 || roll.Criteria[0].Status != artifact.CriterionEvidenced {
		t.Fatalf("rollup.Criteria = %+v, want ac-1 evidenced", roll.Criteria)
	}
	if roll.Commit != repo.Head || roll.Story != "jira:CLOSE-1" || roll.Ref != "spec/close-fixture" {
		t.Fatalf("rollup = %+v, unexpected story/ref/commit", roll)
	}
	wantDigest, err := rollupDigest(*roll)
	if err != nil {
		t.Fatal(err)
	}
	if roll.Digest != wantDigest {
		t.Fatalf("rollup.Digest = %q, recomputed = %q (not recomputable from pinned inputs)", roll.Digest, wantDigest)
	}

	// The rollup reaches the publish step for real and reads back through
	// the fake provider (ac-2).
	published, ok := fp.PublishedField("jira:CLOSE-1")
	if !ok {
		t.Fatal("fake provider has no published rollup for jira:CLOSE-1")
	}
	if published.Commit != repo.Head || !published.Eligible {
		t.Fatalf("published rollup = %+v, want commit=%s eligible=true", published, repo.Head)
	}
	if len(published.Criteria) != 1 || published.Criteria[0].Status != "evidenced" {
		t.Fatalf("published rollup criteria = %+v, want ac-1 evidenced", published.Criteria)
	}

	// dc-3: close stops at the branch — no MR is created, but the
	// instruction to push and open one is printed.
	if !strings.Contains(stdout.String(), "git push -u origin close/close-fixture") {
		t.Fatalf("stdout = %q, want the push instruction naming the closure branch", stdout.String())
	}

	// Git-level proof: the archive move is a rename of spec.md active→archive
	// on the closure branch. Because the status line flips in the move, it is
	// NO LONGER a 100%-similarity (R100) rename — VL-010's round-6 status-only
	// closed-flip exception, not the pure-rename one, is what admits it.
	branch := gitCurrentBranch(t, repo.Dir)
	if branch != "close/close-fixture" {
		t.Fatalf("current branch = %q, want close/close-fixture", branch)
	}
	diffOut := gitOutput(t, repo.Dir, "diff", "--name-status", "-M", repo.Head, "HEAD")
	renameLine := regexp.MustCompile(`R\d+\t\.verdi/specs/active/close-fixture/spec\.md\t\.verdi/specs/archive/close-fixture/spec\.md`)
	if !renameLine.MatchString(diffOut) {
		t.Fatalf("git diff --name-status -M did not report a rename for spec.md active->archive:\n%s", diffOut)
	}
	if strings.Contains(diffOut, "R100\t.verdi/specs/active/close-fixture/spec.md") {
		t.Fatalf("archive move is still R100 — the status flip should make it a sub-100%% rename:\n%s", diffOut)
	}

	// The load-bearing proof of the round-6 fix: re-linting the post-close
	// store in-process is clean of the two rules the un-flipped archive
	// tripped — VL-002 (status: closed under specs/archive/ is correct
	// placement) and VL-010 (the status-only apb→closed flip within the
	// active→archive move is admitted).
	lintFindings, err := lint.NewEngine().Run(ctx, repo.Dir, lint.Context{DiffBase: repo.Head}, lint.Options{})
	if err != nil {
		t.Fatalf("re-lint of post-close store: %v", err)
	}
	for _, f := range lintFindings {
		if f.Rule == "VL-002" || f.Rule == "VL-010" {
			t.Fatalf("re-lint of post-close store fired %s (the round-6 fix should make the archived quartet clean of it): %s", f.Rule, f.String())
		}
	}
}

func TestRunClose_PreExistingStagedPathsRefusedBeforeMutation(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}
	writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
	reportBefore, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading pre-close deviation report: %v", err)
	}
	appendCloseTestFile(t, filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), "# staged manifest note\n")
	appendCloseTestFile(t, filepath.Join(repo.Dir, "verdi.bindings.yaml"), "# staged binding note\n")
	gitOutput(t, repo.Dir, "add", ".verdi/verdi.yaml", "verdi.bindings.yaml")

	fp := fake.New()
	deps := closeDeps{Forge: forgefake.New(), Registry: fp, Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runClose(with staged paths) = %d, want operational exit 2; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	for _, path := range []string{".verdi/verdi.yaml", "verdi.bindings.yaml"} {
		if !strings.Contains(stderr.String(), path) {
			t.Fatalf("stderr = %q, want staged path %q named", stderr.String(), path)
		}
	}
	if branch := gitCurrentBranch(t, repo.Dir); branch != "main" {
		t.Fatalf("current branch = %q, want main (refusal must precede branch cut)", branch)
	}
	if hasLocalBranch(t, repo.Dir, "close/close-fixture") {
		t.Fatal("close/close-fixture exists despite pre-existing staged paths")
	}
	activeSpec, err := os.ReadFile(store.ActiveSpecPath(repo.Dir, "close-fixture"))
	if err != nil {
		t.Fatalf("reading active spec after refusal: %v", err)
	}
	if string(activeSpec) != closeFixtureStorySpecMD {
		t.Fatal("active spec changed despite staged-path refusal")
	}
	if _, err := os.Stat(store.ArchiveSpecDir(repo.Dir, "close-fixture")); !os.IsNotExist(err) {
		t.Fatalf("archive directory exists despite staged-path refusal: %v", err)
	}
	reportAfter, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("reading post-refusal deviation report: %v", err)
	}
	if !bytes.Equal(reportAfter, reportBefore) {
		t.Fatal("deviation report changed despite staged-path refusal")
	}
	if _, err := os.Stat(filepath.Join(store.ActiveSpecDir(repo.Dir, "close-fixture"), "rollup.json")); !os.IsNotExist(err) {
		t.Fatalf("rollup.json exists despite staged-path refusal: %v", err)
	}
	if _, ok := fp.PublishedField("jira:CLOSE-1"); ok {
		t.Fatal("rollup published despite staged-path refusal")
	}
}

func TestRunClose_UnrelatedWorkingTreeChangesSurviveAndStayOutOfCommit(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}
	writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)

	modifiedPath := filepath.Join(repo.Dir, "verdi.bindings.yaml")
	const modifiedSuffix = "# unrelated working-tree edit\n"
	appendCloseTestFile(t, modifiedPath, modifiedSuffix)
	untrackedRel := "closure-scratch.txt"
	untrackedPath := filepath.Join(repo.Dir, untrackedRel)
	const untrackedContent = "keep this untracked scratch file\n"
	if err := os.WriteFile(untrackedPath, []byte(untrackedContent), 0o644); err != nil {
		t.Fatalf("writing unrelated untracked file: %v", err)
	}

	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}
	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("runClose(with unrelated dirty files) = %d, want 0; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	modifiedAfter, err := os.ReadFile(modifiedPath)
	if err != nil {
		t.Fatalf("reading unrelated modified file after close: %v", err)
	}
	if !strings.HasSuffix(string(modifiedAfter), modifiedSuffix) {
		t.Fatalf("unrelated modified file did not survive close: %q", modifiedAfter)
	}
	untrackedAfter, err := os.ReadFile(untrackedPath)
	if err != nil {
		t.Fatalf("reading unrelated untracked file after close: %v", err)
	}
	if string(untrackedAfter) != untrackedContent {
		t.Fatalf("unrelated untracked file = %q, want %q", untrackedAfter, untrackedContent)
	}
	status := gitOutput(t, repo.Dir, "status", "--short")
	for _, want := range []string{" M verdi.bindings.yaml", "?? " + untrackedRel} {
		if !strings.Contains(status, want) {
			t.Fatalf("git status after close = %q, want surviving working-tree entry %q", status, want)
		}
	}
	assertClosureCommitOwnsOnlySpecPaths(t, repo.Dir, "close-fixture")
	if got := strings.TrimSpace(gitOutput(t, repo.Dir, "ls-tree", "-r", "--name-only", "HEAD", "--", untrackedRel)); got != "" {
		t.Fatalf("unrelated untracked file entered closure commit tree: %q", got)
	}
	committedBindings := gitOutput(t, repo.Dir, "show", "HEAD:verdi.bindings.yaml")
	if strings.Contains(committedBindings, modifiedSuffix) {
		t.Fatal("unrelated tracked modification entered closure commit")
	}
}

// TestRunClose_NotEligible_ExitsOneWithNoSideEffects proves the closure
// gate actually gates: with no evidence at all, `verdi close` exits 1,
// creates no closure branch, moves nothing, and publishes nothing.
func TestRunClose_NotEligible_ExitsOneWithNoSideEffects(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()

	fp := fake.New()
	fg := forgefake.New()
	manifest := &store.Manifest{}
	deps := closeDeps{Forge: fg, Registry: fp}

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-fixture", manifest, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runClose(no evidence) = %d, want 1; stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}

	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-fixture", "spec.md")); err != nil {
		t.Fatalf("spec.md should remain in specs/active/ after a failed close: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture")); !os.IsNotExist(err) {
		t.Fatal("specs/archive/close-fixture should not exist after a failed close")
	}
	if branch := gitCurrentBranch(t, repo.Dir); branch != "main" {
		t.Fatalf("current branch = %q after a failed close, want main (no closure branch cut)", branch)
	}
	if _, ok := fp.PublishedField("jira:CLOSE-1"); ok {
		t.Fatal("fake provider has a published rollup despite the closure gate failing")
	}
}

// TestRunClose_RefusesUndispositionedFindings is X-13/X-16/X-17's
// load-bearing red-first proof at the `verdi close` level: this exact
// fixture (fully eligible, condition 1 through 3 all green) is what today
// silently ARCHIVES an undispositioned report before this fix — close's
// own internal freeze-align call falls through to the regenerate path
// (no living report, or a stale/undispositioned one) and freezes what it
// finds, in the same motion, with nobody having reviewed it. After the
// fix, the SAME fixture refuses (exit 1), names the offenders, prints the
// ritual, and — the round's own repeated lesson — leaves NOTHING
// archived: no closure branch, no quartet move, no publish.
func TestRunClose_RefusesUndispositionedFindings(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root, head string) // "" setup = no report at all (X-17)
		wantSubstr []string
	}{
		{
			name:       "no report at all (X-17's literal scenario)",
			setup:      func(t *testing.T, root, head string) {},
			wantSubstr: []string{"no deviation-report.md found at", "the closure ritual is align"},
		},
		{
			name: "a living report covering head with an undispositioned finding (X-13's literal scenario)",
			setup: func(t *testing.T, root, head string) {
				writeCloseGateReport(t, root, head, undispositionedFindingYAML)
			},
			wantSubstr: []string{"undispositioned finding(s) [f-1]", "the closure ritual is align"},
		},
		{
			name: "a stale report (X-16's literal scenario: dispositions committed, HEAD moved)",
			setup: func(t *testing.T, root, head string) {
				writeCloseGateReport(t, root, "0000000000000000000000000000000000000b", dispositionedFindingYAML)
			},
			wantSubstr: []string{"covers 0000000000000000000000000000000000000b, not head", "the closure ritual is align"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildCloseFixtureRepo(t)
			ctx := context.Background()
			prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
			if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
				t.Fatalf("produceSelfHostedEvidence: %v", err)
			}
			tc.setup(t, repo.Dir, repo.Head)

			fp := fake.New()
			deps := closeDeps{Forge: forgefake.New(), Registry: fp, Runner: upstream.NewFakeRunner()}
			var stdout, stderr bytes.Buffer
			got := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &stdout, &stderr)
			if got != 1 {
				t.Fatalf("runClose(undispositioned) = %d, want 1 (verdict, not archived); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantSubstr {
				if !contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want it to contain %q (naming the offenders and the ritual)", stdout.String(), want)
				}
			}

			// The X-13/X-17 proof itself: nothing archived, no side effects —
			// silence must never ride into the archive.
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-fixture", "spec.md")); err != nil {
				t.Fatalf("spec.md should remain in specs/active/ after a refused close: %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture")); !os.IsNotExist(err) {
				t.Fatal("specs/archive/close-fixture should NOT exist — the undispositioned/stale report must never be silently frozen and archived")
			}
			if branch := gitCurrentBranch(t, repo.Dir); branch != "main" {
				t.Fatalf("current branch = %q after a refused close, want main (no closure branch cut)", branch)
			}
			if _, ok := fp.PublishedField("jira:CLOSE-1"); ok {
				t.Fatal("fake provider has a published rollup despite the closure gate failing")
			}
		})
	}
}

// TestRunClose_FeatureClass_DispatchesToFeatureClosure proves runClose no
// longer answers a feature-class target with I-23's old "not yet
// implemented" stub (superseded now that closefeature.go completes
// spec/close-verb's deferred feature half) — it reaches the REAL feature
// closure gate instead. buildCloseFixtureRepo's loan-mgmt feature has one
// implementing story (close-fixture, via its own implements edge into
// loan-mgmt#ac-1) that is still accepted-pending-build, not closed, so the
// feature gate's "every implementing story closed" condition fails and
// closure is refused (exit 1) — never the old exit-2 "not yet implemented"
// operational error. The full happy/negative feature-closure ritual is
// proven end to end in closefeature_test.go; this test only pins that
// close.go's dispatch is wired to it.
func TestRunClose_FeatureClass_DispatchesToFeatureClosure(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()
	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}

	var stdout, stderr bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/loan-mgmt", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 1 {
		t.Fatalf("runClose(feature spec) = %d, want 1 (feature closure gate refused, not an operational error); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	if strings.Contains(stderr.String(), "not yet implemented") {
		t.Fatalf("stderr = %q, the old not-yet-implemented refusal must be gone now that feature closure is real", stderr.String())
	}
	if !strings.Contains(stdout.String(), "closure(feature):") {
		t.Fatalf("stdout = %q, want it to show the real feature closure gate's conditions", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "loan-mgmt")); !os.IsNotExist(err) {
		t.Fatal("specs/archive/loan-mgmt should not exist after a refused feature closure")
	}
}

// TestRunClose_UnresolvableStory_ExitsOperational proves a story/spec
// argument that resolves to nothing is an operational error (2), not a
// silent nothing-to-close success.
func TestRunClose_UnresolvableStory_ExitsOperational(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New()}
	var stdout, stderr bytes.Buffer
	got := runClose(context.Background(), repo.Dir, "spec/does-not-exist", &store.Manifest{}, deps, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("runClose(unresolvable spec) = %d, want 2; stderr=%s", got, stderr.String())
	}
}

// TestFreezeAlignDeps_OptsCloseFreezeIntoBoundedWait is FIX 2's unit-level
// red-first pin (finding judged-close-cannot-reach-inherited-wait): the
// single builder both runClose and runCloseFeature use for their freeze-align
// step must carry Wait, so close's internal freeze-align inherits align's
// bounded-wait contract (spec/judge-ergonomics ac-3) — the "future story"
// alignDeps.Wait's own comment deferred, now realized. The bound stays the
// judge's own configured ceiling (JudgeTimeout threaded through unchanged —
// duration identical to today, only the timeout FAILURE SHAPE changes).
func TestFreezeAlignDeps_OptsCloseFreezeIntoBoundedWait(t *testing.T) {
	deps := closeDeps{
		Runner:        upstream.NewFakeRunner(),
		JudgeCmd:      []string{"claude", "-p"},
		JudgeRequired: true,
		JudgeTimeout:  42 * time.Second,
	}
	got := freezeAlignDeps(deps, "sha256:deadbeef")
	if !got.Wait {
		t.Fatal("freezeAlignDeps did not set Wait — close's freeze-align cannot reach align's bounded-wait contract (the finding's whole point)")
	}
	if got.JudgeTimeout != 42*time.Second {
		t.Fatalf("freezeAlignDeps JudgeTimeout = %s, want the judge's own ceiling preserved (42s — duration identical to today)", got.JudgeTimeout)
	}
	if !got.JudgeRequired || got.ModelDigest != "sha256:deadbeef" || got.JudgeCmd[0] != "claude" {
		t.Fatalf("freezeAlignDeps did not thread close's judge config/model digest faithfully: %+v", got)
	}
}

// TestCloseFreezeAlign_WaitReachableViaProductionDeps is FIX 2's behavioral
// red-first proof (same finding): close's freeze-align now runs Wait-bounded
// through close's OWN deps construction, not only through a hand-built
// Wait:true literal no production close invocation could ever produce (the
// finding's exact gap). The deps come from freezeAlignDeps — the single
// builder both runClose and runCloseFeature call — and drive the same
// runAlignForSpec(freeze=true) freeze step close uses; against a hung judge
// under a short ceiling the step exits 2 with the report path already
// printed and nothing written, the honest expiry contract inherited whole
// from the shared align engine. The story/feature closure gates keep a
// PASSING close on the freeze-in-place (no-judge) path, so this exercises
// the freeze step's judge path directly, exactly as it would run were a
// regenerate ever reached.
func TestCloseFreezeAlign_WaitReachableViaProductionDeps(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec, err := storyresolve.ResolveBuildSpec(repo.Dir, "feature/stale-decline")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}

	// Built exactly as cmdClose builds it (closeDeps -> freezeAlignDeps): a
	// hung judge (sleeps 5s) under a short ceiling stands in for the real
	// 6-7-minute judge against a bound.
	cd := closeDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t), JudgeTimeout: 200 * time.Millisecond}
	alignD := freezeAlignDeps(cd, testResolveModelDigest(t, repo.Dir))
	if !alignD.Wait {
		t.Fatal("freezeAlignDeps did not set Wait — close's freeze-align cannot reach the bounded-wait contract (the finding's whole point)")
	}

	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(context.Background(), repo.Dir, spec, repo.Head, true /* close always freezes */, alignD, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("close freeze-align (production deps, hung judge) = %d, want 2 (bounded-wait expiry); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	reportPath := filepath.Join(repo.Dir, ".verdi", "specs", "active", "stale-decline", "deviation-report.md")
	if firstLine := strings.SplitN(stdout.String(), "\n", 2)[0]; firstLine != reportPath {
		t.Fatalf("stdout first line = %q, want the report path %q printed before the bounded judge run", firstLine, reportPath)
	}
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatal("a frozen report was written despite the --wait expiry — close's freeze-align must write nothing on an operational timeout")
	}
	if !strings.Contains(stderr.String(), "terminated") {
		t.Fatalf("expiry stderr = %q, want the honest terminated-at-the-bound message close inherits from align", stderr.String())
	}
}

// TestCloseFreezeAlign_ExpiryMessageSpeaksCloseVerb is the red-first pin for
// finding judged-close-inherits-aligns-resume-instructions-verbatim: when
// close's freeze-align hits the bounded-wait expiry, its stderr guidance must
// speak CLOSE's verb, not align's flag language inherited verbatim. close
// exposes no --wait flag (so "a longer --wait" is not something a close caller
// can act on) and re-running *align* mid-close is the wrong resume verb — the
// close aborted at exit 2 and only re-running close completes the freeze and
// archive. So the expiry message must name `verdi close` as the way to resume,
// carry no --wait flag language at all, and never tell the caller to "Re-run
// align". The deps come from freezeAlignDeps — the single builder both
// runClose and runCloseFeature use — proving the close-appropriate resume hint
// is threaded through production deps, not a hand-built literal. align's own
// expiry message is unchanged and still speaks --wait
// (TestRunAlign_Wait_ExpiryMessageStatesJudgeTerminated keeps passing).
func TestCloseFreezeAlign_ExpiryMessageSpeaksCloseVerb(t *testing.T) {
	repo := buildAlignRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec, err := storyresolve.ResolveBuildSpec(repo.Dir, "feature/stale-decline")
	if err != nil {
		t.Fatalf("ResolveBuildSpec: %v", err)
	}

	// Exactly cmdClose's construction (closeDeps -> freezeAlignDeps): a hung
	// judge (sleeps 5s) under a short ceiling stands in for the real judge
	// against a bound the caller's patience cannot outlast.
	cd := closeDeps{Runner: alignRunner(svcDir), JudgeCmd: alignFakeJudgeSleepy(t), JudgeTimeout: 200 * time.Millisecond}
	alignD := freezeAlignDeps(cd, testResolveModelDigest(t, repo.Dir))

	var stdout, stderr bytes.Buffer
	got := runAlignForSpec(context.Background(), repo.Dir, spec, repo.Head, true /* close always freezes */, alignD, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("close freeze-align (production deps, hung judge) = %d, want 2 (bounded-wait expiry); stdout=%s stderr=%s", got, stdout.String(), stderr.String())
	}
	msg := stderr.String()
	if !strings.Contains(msg, "terminated") {
		t.Fatalf("expiry stderr = %q, want it to stay honest that the judge was terminated at the bound", msg)
	}
	if !strings.Contains(msg, "verdi close") {
		t.Fatalf("expiry stderr = %q, want the resume GUIDANCE to name `verdi close` as the resume verb (close aborted at exit 2; only re-running close completes the freeze + archive)", msg)
	}
	if strings.Contains(msg, "Re-run align") {
		t.Fatalf("expiry stderr = %q, resume guidance still tells the caller to Re-run align — the wrong resume verb mid-close", msg)
	}
	if strings.Contains(msg, "longer --wait") {
		t.Fatalf("expiry stderr = %q, resume guidance still speaks --wait flag language — close exposes no --wait flag, so a close caller cannot act on it (no flag language for close)", msg)
	}
}

// writeFrozenCloseReport writes an ALREADY-FROZEN deviation-report.md into the
// close-fixture spec's directory (covers head, one dispositioned finding). The
// closure gate's condition 4 (checkDispositionCompleteCondition) does not
// inspect the frozen stamp, so the gate PASSES and close reaches the branch
// cut — but runAlignForSpec's freeze step then refuses ("a frozen alignment
// report is immutable", align.go), returning non-zero AFTER close/<name> is
// already cut. This is the reachable sibling of the finding's judge-expiry: a
// judge expiry itself is unreachable through a gate-passing close because
// condition 4 (X-13/X-16/X-17) forces the no-judge freeze-in-place path, but
// the residue it leaves — close/<name> cut with nothing committed, the retry
// blocked at the next cut — is byte-for-byte the same post-cut freeze-align
// failure the fix must unwind.
func writeFrozenCloseReport(t *testing.T, root, covers string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", "close-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%sfrozen: { at: 2024-01-01, commit: %s }
digest: sha256:%s
---
# Alignment report
`, covers, dispositionedFindingYAML, covers, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing frozen deviation-report.md: %v", err)
	}
}

// hasLocalBranch reports whether dir carries a local branch named name.
func hasLocalBranch(t *testing.T, dir, name string) bool {
	t.Helper()
	return strings.TrimSpace(gitOutput(t, dir, "branch", "--list", name)) != ""
}

// TestRunClose_FreezeAlignFailure_UnwindsBranchCutAndRetryCompletes is the
// red-first proof for finding
// judged-close-resume-hint-names-a-path-close-itself-refuses: runClose cuts
// close/<name> BEFORE the freeze step (close.go), so a freeze-align failure
// used to leave the branch behind and the resume hint's promised retry then
// died at CheckoutNewBranch's no-clobber refusal — "the hint promises a path
// the verb's own residue blocks". The fix unwinds the cut on that failure
// path. This drives the WHOLE runClose verb through a real freeze-align
// failure (an already-frozen active-zone report: gate passes condition 4,
// freeze refuses — the reachable sibling of the finding's judge-expiry, see
// writeFrozenCloseReport) rather than runAlignForSpec directly, which was the
// finding's own criticism of the ac-3 tests, and asserts the whole loop:
// non-zero exit, the original branch restored, close/<name> gone, and a
// SECOND runClose proceeding PAST the branch cut to a clean archived closure
// once the blocking condition clears (the human re-aligns — the analog of
// "the judge window now allows").
func TestRunClose_FreezeAlignFailure_UnwindsBranchCutAndRetryCompletes(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	ctx := context.Background()

	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}
	writeFrozenCloseReport(t, repo.Dir, repo.Head)

	deps := closeDeps{Forge: forgefake.New(), Registry: fake.New(), Runner: upstream.NewFakeRunner()}
	originalBranch := gitCurrentBranch(t, repo.Dir)

	var o1, e1 bytes.Buffer
	got := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &o1, &e1)
	if got == 0 {
		t.Fatalf("runClose(frozen report) = 0, want non-zero (freeze-align refused past the branch cut); stdout=%s stderr=%s", o1.String(), e1.String())
	}
	if b := gitCurrentBranch(t, repo.Dir); b != originalBranch {
		t.Fatalf("after a freeze-align failure, current branch = %q, want the original %q restored — the branch cut was not unwound", b, originalBranch)
	}
	if hasLocalBranch(t, repo.Dir, "close/close-fixture") {
		t.Fatal("close/close-fixture still exists after a freeze-align failure — the branch cut was not unwound, so the resume hint's retry is blocked at the next cut")
	}

	// The retry the resume hint promises now COMPLETES. The blocking condition
	// clears — the human re-aligns, leaving a living, dispositioned report
	// covering head — and a second `verdi close` proceeds past the branch cut
	// to a clean archived closure rather than aborting at CheckoutNewBranch's
	// no-clobber refusal (which, without the unwind, it did: proven by the red
	// state where this second call exits 2 with "already exists").
	writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
	var o2, e2 bytes.Buffer
	got2 := runClose(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, &o2, &e2)
	if got2 != 0 {
		t.Fatalf("second runClose (the promised retry) = %d, want 0 (retry completes); stdout=%s stderr=%s", got2, o2.String(), e2.String())
	}
	if strings.Contains(e2.String(), "already exists") {
		t.Fatalf("second runClose aborted at the branch cut (%q) — the residue still blocks the retry the hint promises", e2.String())
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "archive", "close-fixture", "spec.md")); err != nil {
		t.Fatalf("the retry did not archive the spec (specs/archive/close-fixture/spec.md absent): %v", err)
	}
}

func TestStageClosureSpec_AddPathsFailurePreservesUnrelatedState(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			"staged.txt":   "committed staged content\n",
			"unstaged.txt": "committed unstaged content\n",
		},
		Message: "seed closure staging failure fixture",
	}})

	stagedPath := filepath.Join(repo.Dir, "staged.txt")
	if err := os.WriteFile(stagedPath, []byte("unrelated staged content\n"), 0o644); err != nil {
		t.Fatalf("writing staged fixture file: %v", err)
	}
	gitOutput(t, repo.Dir, "add", "--", "staged.txt")

	unstagedPath := filepath.Join(repo.Dir, "unstaged.txt")
	if err := os.WriteFile(unstagedPath, []byte("unrelated unstaged content\n"), 0o644); err != nil {
		t.Fatalf("writing unstaged fixture file: %v", err)
	}
	untrackedPath := filepath.Join(repo.Dir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("unrelated untracked content\n"), 0o644); err != nil {
		t.Fatalf("writing untracked fixture file: %v", err)
	}

	statusBefore := gitOutput(t, repo.Dir, "status", "--short")
	for _, want := range []string{"M  staged.txt", " M unstaged.txt", "?? untracked.txt"} {
		if !strings.Contains(statusBefore, want) {
			t.Fatalf("precondition git status = %q, want %q", statusBefore, want)
		}
	}
	indexBefore := gitOutput(t, repo.Dir, "diff", "--cached", "--binary", "--")
	worktreeBefore := gitOutput(t, repo.Dir, "diff", "--binary", "--")
	untrackedBefore, err := os.ReadFile(untrackedPath)
	if err != nil {
		t.Fatalf("reading untracked fixture file before staging failure: %v", err)
	}

	const missingTarget = "missing-closure-target"
	err = stageClosureSpec(context.Background(), repo.Dir, missingTarget)
	if err == nil {
		t.Fatal("stageClosureSpec(nonexistent target) = nil, want AddPaths error")
	}
	for _, want := range []string{"staging closure paths", "spec/" + missingTarget} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("stageClosureSpec error = %q, want context %q", err, want)
		}
	}

	if got := gitOutput(t, repo.Dir, "status", "--short"); got != statusBefore {
		t.Fatalf("git status changed after staging failure:\nbefore: %q\nafter:  %q", statusBefore, got)
	}
	if got := gitOutput(t, repo.Dir, "diff", "--cached", "--binary", "--"); got != indexBefore {
		t.Fatalf("index changed after staging failure:\nbefore:\n%s\nafter:\n%s", indexBefore, got)
	}
	if got := gitOutput(t, repo.Dir, "diff", "--binary", "--"); got != worktreeBefore {
		t.Fatalf("working-tree diff changed after staging failure:\nbefore:\n%s\nafter:\n%s", worktreeBefore, got)
	}
	untrackedAfter, err := os.ReadFile(untrackedPath)
	if err != nil {
		t.Fatalf("reading untracked fixture file after staging failure: %v", err)
	}
	if !bytes.Equal(untrackedAfter, untrackedBefore) {
		t.Fatalf("untracked file changed after staging failure: got %q, want %q", untrackedAfter, untrackedBefore)
	}
}

// TestUnwindClosureBranchCut unit-tests the shared unwind helper directly,
// covering BOTH clauses of the fix contract — in particular the "if anything
// was somehow committed, leave it and say so honestly" clause, which the
// runClose-driven tests never reach (close's pre-commit window always leaves
// an empty branch).
func TestUnwindClosureBranchCut(t *testing.T) {
	ctx := context.Background()

	t.Run("empty branch: restores the original and deletes", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "x\n"}, Message: "m"}})
		cutPoint := repo.Head
		checkoutBranch(t, repo.Dir, "close/foo") // git checkout -b close/foo at HEAD

		var stderr bytes.Buffer
		unwindClosureBranchCut(ctx, repo.Dir, "main", "close/foo", cutPoint, &stderr)

		if b := gitCurrentBranch(t, repo.Dir); b != "main" {
			t.Fatalf("current branch = %q, want main restored", b)
		}
		if hasLocalBranch(t, repo.Dir, "close/foo") {
			t.Fatal("close/foo was not deleted")
		}
		if s := strings.TrimSpace(stderr.String()); s != "" {
			t.Fatalf("a clean unwind wrote to stderr = %q, want it silent (only giving-up branches disclose)", s)
		}
	})

	t.Run("committed branch: leaves it in place and discloses, never discarding", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"a.txt": "x\n"}, Message: "m"}})
		cutPoint := repo.Head
		checkoutBranch(t, repo.Dir, "close/foo")
		// A commit moves close/foo's tip beyond the cut point.
		if err := os.WriteFile(filepath.Join(repo.Dir, "onbranch.txt"), []byte("work\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitOutput(t, repo.Dir, "add", "-A")
		gitOutput(t, repo.Dir, "commit", "-m", "committed work on close/foo")

		var stderr bytes.Buffer
		unwindClosureBranchCut(ctx, repo.Dir, "main", "close/foo", cutPoint, &stderr)

		if !hasLocalBranch(t, repo.Dir, "close/foo") {
			t.Fatal("close/foo carrying a commit beyond its cut point was deleted — committed work discarded")
		}
		if !strings.Contains(stderr.String(), "beyond its cut point") {
			t.Fatalf("stderr = %q, want an honest disclosure that the branch carries commit(s) beyond its cut point", stderr.String())
		}
	})
}

// TestCmdClose_RefusesOutsideCI proves 04 §Semantics's "PublishRollup runs
// in CI only" gates `verdi close` itself (it calls PublishRollup directly,
// spec/close-verb ac-2), mirroring rollup.go's own --force-local precedent
// (I-32) exactly: refused by default outside a detected CI environment,
// proceeding with a disclosed NON-AUTHORITATIVE warning under --force-local.
func TestCmdClose_RefusesOutsideCI(t *testing.T) {
	for _, v := range []string{"CI", "GITHUB_ACTIONS", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME", "GITHUB_BASE_REF"} {
		t.Setenv(v, "")
	}
	t.Chdir(t.TempDir())

	t.Run("no --force-local: refused", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdClose([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdClose outside CI = %d, want 2", got)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty on a CI refusal", stdout.String())
		}
		if !strings.Contains(stderr.String(), "CI only") && !strings.Contains(stderr.String(), "outside CI") {
			t.Fatalf("stderr = %q, want it to explain the CI-only refusal", stderr.String())
		}
		if !strings.Contains(stderr.String(), "--force-local") {
			t.Fatalf("stderr = %q, want it to name the --force-local escape hatch", stderr.String())
		}
	})

	t.Run("--force-local: proceeds with a disclosed non-authoritative warning", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		// No store root under t.TempDir(), so this still exits 2 — but past
		// the CI check, on the store-root error, proving --force-local
		// actually let it through.
		got := cmdClose([]string{"jira:LOAN-1482", "--force-local"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdClose(--force-local, no store) = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "NON-AUTHORITATIVE") {
			t.Fatalf("stderr = %q, want a disclosed NON-AUTHORITATIVE warning", stderr.String())
		}
		if strings.Contains(stderr.String(), "refusing to publish") {
			t.Fatalf("stderr = %q, --force-local should not still be refused", stderr.String())
		}
	})
}

// TestCmdClose_Negative covers cmdClose's own argument-parsing errors,
// independent of CI detection.
func TestCmdClose_Negative(t *testing.T) {
	t.Setenv("CI", "true")

	t.Run("no story argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdClose(nil, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdClose(no args) = %d, want 2", got)
		}
		if !strings.Contains(stderr.String(), "usage: verdi close") {
			t.Fatalf("stderr = %q, want the usage message", stderr.String())
		}
	})

	t.Run("extra positional argument", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		got := cmdClose([]string{"jira:LOAN-1482", "spec/other"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdClose(two positional args) = %d, want 2", got)
		}
	})

	t.Run("no store root", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var stdout, stderr bytes.Buffer
		got := cmdClose([]string{"jira:LOAN-1482"}, &stdout, &stderr)
		if got != 2 {
			t.Fatalf("cmdClose(no store root) = %d, want 2", got)
		}
	})
}

func gitCurrentBranch(t *testing.T, dir string) string {
	t.Helper()
	return strings.TrimSpace(gitOutput(t, dir, "symbolic-ref", "--short", "HEAD"))
}

func appendCloseTestFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("opening %s for append: %v", path, err)
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		t.Fatalf("appending to %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing %s after append: %v", path, err)
	}
}

func assertClosureCommitOwnsOnlySpecPaths(t *testing.T, dir, name string) {
	t.Helper()
	activePrefix := store.SpecDirRelPath(store.ZoneActive, name) + "/"
	archivePrefix := store.SpecDirRelPath(store.ZoneArchive, name) + "/"
	changed := strings.Fields(gitOutput(t, dir, "diff", "--no-renames", "--name-only", "HEAD^", "HEAD"))
	if len(changed) == 0 {
		t.Fatal("closure commit changed no paths")
	}
	var sawActive, sawArchive bool
	for _, path := range changed {
		switch {
		case strings.HasPrefix(path, activePrefix):
			sawActive = true
		case strings.HasPrefix(path, archivePrefix):
			sawArchive = true
		default:
			t.Fatalf("closure commit changed unrelated path %q; all changed paths: %v", path, changed)
		}
	}
	if !sawActive || !sawArchive {
		t.Fatalf("closure commit paths = %v, want both %s and %s trees", changed, activePrefix, archivePrefix)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
