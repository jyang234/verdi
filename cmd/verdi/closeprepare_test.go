package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/upstream"
)

func TestCmdClose_PrepareParsing(t *testing.T) {
	t.Setenv("CI", "true")

	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{
			name:     "missing explicit ref",
			args:     []string{"--prepare"},
			wantText: "verdi close --prepare <jira:STORY-KEY | spec/name> [--force-local]",
		},
		{
			name:     "prepare and preflight are mutually exclusive",
			args:     []string{"--prepare", "--preflight", "spec/example"},
			wantText: "--prepare and --preflight are mutually exclusive",
		},
		{
			name:     "extra positional argument",
			args:     []string{"--prepare", "spec/example", "spec/other"},
			wantText: `unexpected extra argument "spec/other"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rc := cmdClose(tc.args, &stdout, &stderr)
			if rc != 2 {
				t.Fatalf("cmdClose(%v) = %d, want 2; stdout=%s stderr=%s", tc.args, rc, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.wantText) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.wantText)
			}
		})
	}
}

func TestCmdClose_PrepareAcceptsExplicitStoryAndFeatureRefs(t *testing.T) {
	clearCIEnv(t)
	clearPrepareForgeEnv(t)

	t.Run("story spec ref", func(t *testing.T) {
		repo := readyCloseFixtureRepo(t)
		t.Chdir(repo.Dir)

		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"--prepare", "spec/close-fixture", "--force-local"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(--prepare story) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "close: --prepare: next command: verdi close spec/close-fixture --force-local") {
			t.Fatalf("stdout does not prove --prepare dispatch: %s", stdout.String())
		}
		if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-fixture", "spec.md")); err != nil {
			t.Fatalf("--prepare archived or removed the active story: %v", err)
		}
	})

	t.Run("feature resolved by explicit story ref", func(t *testing.T) {
		opts := defaultCloseFeatureFixtureOpts()
		opts.FeatureStory = "jira:FIXTURE-EPIC-1"
		repo := buildCloseFeatureRepo(t, opts)
		seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
		writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		t.Chdir(repo.Dir)

		var stdout, stderr bytes.Buffer
		rc := cmdClose([]string{"jira:FIXTURE-EPIC-1", "--prepare", "--force-local"}, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("cmdClose(--prepare feature) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "close: --prepare: next command: verdi close jira:FIXTURE-EPIC-1 --force-local") {
			t.Fatalf("stdout does not prove feature --prepare dispatch: %s", stdout.String())
		}
		if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "close-feature-fixture", "spec.md")); err != nil {
			t.Fatalf("--prepare archived or removed the active feature: %v", err)
		}
	})
}

func TestCmdClose_PrepareRunsOutsideCIWithoutForceLocal(t *testing.T) {
	clearCIEnv(t)
	clearPrepareForgeEnv(t)
	repo := readyCloseFixtureRepo(t)
	t.Chdir(repo.Dir)

	var stdout, stderr bytes.Buffer
	rc := cmdClose([]string{"--prepare", "spec/close-fixture"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("cmdClose(--prepare outside CI) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "close: --prepare: next command: verdi close spec/close-fixture\n") {
		t.Fatalf("stdout does not contain the unguarded local preparation result: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "refusing to publish outside CI") {
		t.Fatalf("--prepare was incorrectly restored behind the publish guard: %s", stderr.String())
	}
}

// TestClosePrepare_BuiltBinaryResumesCurrentJudgmentStop covers the one
// boundary the runPrepare and cmdClose tests above cannot: the documented
// `verdi close --prepare <ref>` grammar through the built binary, including a
// second process invocation over the same current report. The retry must
// return the same human-only stop without regenerating the report.
func TestClosePrepare_BuiltBinaryResumesCurrentJudgmentStop(t *testing.T) {
	clearCIEnv(t)
	clearPrepareForgeEnv(t)
	repo := buildCloseFixtureRepo(t)
	const findings = `  - { id: f-1, kind: computed, text: "human judgment remains" }
`
	writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, findings)
	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
	before, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	bin := buildVerdiBinary(t)

	for run := 1; run <= 2; run++ {
		code, stdout, stderr := runVerdi(t, bin, repo.Dir, "close", "--prepare", "spec/close-fixture")
		if code != 1 {
			t.Fatalf("built binary prepare run %d = %d, want 1; stdout=%s stderr=%s", run, code, stdout, stderr)
		}
		for _, want := range []string{
			"close: --prepare: JUDGMENT REQUIRED (1 undispositioned finding(s)",
			"verdi disposition --rationale '<human-authored rationale>' -- spec/close-fixture f-1 '<human-authored-disposition:fixed|accepted-deviation>'",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("built binary prepare run %d stdout missing %q: %s", run, want, stdout)
			}
		}
		if strings.Contains(stdout, "ALIGNMENT REQUIRED") {
			t.Fatalf("built binary prepare run %d regenerated a current report: %s", run, stdout)
		}
	}

	after, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("built binary prepare retry changed the current undispositioned report")
	}
}

func TestRunPrepare_GeneratesAbsentOrStaleReportForStoryAndFeature(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		specName string
		stale    bool
		build    func(*testing.T) *fixturegit.Repo
	}{
		{
			name:     "story absent",
			ref:      "spec/close-fixture",
			specName: "close-fixture",
			build:    buildCloseFixtureRepo,
		},
		{
			name:     "story stale",
			ref:      "spec/close-fixture",
			specName: "close-fixture",
			stale:    true,
			build:    buildCloseFixtureRepo,
		},
		{
			name:     "feature absent",
			ref:      "spec/close-feature-fixture",
			specName: "close-feature-fixture",
			build: func(t *testing.T) *fixturegit.Repo {
				return buildCloseFeatureRepo(t, defaultCloseFeatureFixtureOpts())
			},
		},
		{
			name:     "feature stale",
			ref:      "spec/close-feature-fixture",
			specName: "close-feature-fixture",
			stale:    true,
			build: func(t *testing.T) *fixturegit.Repo {
				return buildCloseFeatureRepo(t, defaultCloseFeatureFixtureOpts())
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.build(t)
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, tc.specName)
			var staleBytes []byte
			if tc.stale {
				writePrepareReport(t, repo.Dir, tc.specName, strings.Repeat("a", 40), dispositionedFindingYAML)
				var err error
				staleBytes, err = os.ReadFile(reportPath)
				if err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

			deps := closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()}
			var stdout, stderr bytes.Buffer
			rc := runPrepare(context.Background(), repo.Dir, tc.ref, &store.Manifest{}, deps, true, &stdout, &stderr)
			if rc != 1 {
				t.Fatalf("runPrepare = %d, want 1 (fresh findings need judgment); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
			}
			for _, want := range []string{"ALIGNMENT REQUIRED", "JUDGMENT REQUIRED", "verdi disposition --rationale", "-- " + tc.ref} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout missing %q: %s", want, stdout.String())
				}
			}

			raw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatalf("reading prepared report: %v", err)
			}
			if tc.stale && bytes.Equal(raw, staleBytes) {
				t.Fatal("stale report was not regenerated")
			}
			fm, _, err := artifact.SplitFrontmatter(raw)
			if err != nil {
				t.Fatal(err)
			}
			report, err := artifact.DecodeDeviation(fm)
			if err != nil {
				t.Fatal(err)
			}
			if report.Covers != repo.Head {
				t.Fatalf("report covers = %q, want HEAD %q", report.Covers, repo.Head)
			}
			if report.Frozen != nil {
				t.Fatalf("prepared report was frozen: %+v", report.Frozen)
			}
			if artifact.AllDispositioned(report.Findings) {
				t.Fatalf("fresh report unexpectedly has no judgment work: %+v", report.Findings)
			}

			after := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
			if before != after {
				t.Fatalf("prepare mutated outside target report:\nbefore: %s\nafter:  %s", before, after)
			}
		})
	}
}

// TestRunPrepare_JudgeTimeoutIsOperationalNotASyntheticFinding pins the
// bounded-wait contract close's freeze-align already inherits
// (freezeAlignDeps, close.go — spec/judge-ergonomics ac-3) onto
// preparation's refresh, which builds the SAME engine's deps.
//
// Without Wait, a judge that outruns its ceiling does not error: RunJudged
// degrades to the synthetic "judged coverage absent" finding
// (align.AbsenceFindingID), preparation writes that into the living report,
// prints it as JUDGMENT REQUIRED with a disposition template, and once a
// human dispositions it close's freeze takes the freeze-in-place branch and
// stamps the synthetic judge failure into the archive VERBATIM — never
// re-running the judge. Preparation is the step that produces the report
// close later freezes, so it must surface a judge timeout as the honest
// operational expiry instead of manufacturing judgment work out of it.
func TestRunPrepare_JudgeTimeoutIsOperationalNotASyntheticFinding(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
	deps := closeDeps{
		Runner:       upstream.NewFakeRunner(),
		JudgeCmd:     alignFakeJudgeSleepy(t), // sleeps 5s
		JudgeTimeout: 200 * time.Millisecond,
		Forge:        forgefake.New(),
	}

	var stdout, stderr bytes.Buffer
	rc := runPrepare(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("runPrepare(judge outruns its ceiling) = %d, want 2 (operational expiry, never a verdict); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("a timed-out judge left a report at %s (err=%v); nothing may be written on the expiry path", reportPath, err)
	}
	if strings.Contains(stdout.String(), "JUDGMENT REQUIRED") {
		t.Fatalf("a timed-out judge was presented as human judgment work: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), align.AbsenceFindingID) {
		t.Fatalf("preparation printed a disposition template for the synthetic %s finding: %s", align.AbsenceFindingID, stdout.String())
	}
	if !strings.Contains(stderr.String(), "terminated at the --wait bound") {
		t.Fatalf("stderr = %q, want the bounded-wait expiry diagnostic", stderr.String())
	}
}

// TestRunPrepare_JudgeTimeoutResumeHintSpeaksPreparation is the companion
// to the test above, and the reason preparation overrides exactly one field
// of freezeAlignDeps: alignDeps.ResumeHint is documented as the CALLING
// verb's own vocabulary (finding judged-close-inherits-aligns-resume-
// instructions-verbatim). close's hint tells the operator to re-run
// `verdi close` to complete the freeze and archive — for someone who ran
// --prepare precisely to NOT freeze or archive yet, inheriting it verbatim
// would point at the real ritual instead of at the resumable preparation
// they were running.
func TestRunPrepare_JudgeTimeoutResumeHintSpeaksPreparation(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	deps := closeDeps{
		Runner:       upstream.NewFakeRunner(),
		JudgeCmd:     alignFakeJudgeSleepy(t),
		JudgeTimeout: 200 * time.Millisecond,
		Forge:        forgefake.New(),
	}

	var stdout, stderr bytes.Buffer
	rc := runPrepare(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("runPrepare(judge outruns its ceiling) = %d, want 2; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "verdi close --prepare spec/close-fixture") {
		t.Fatalf("stderr = %q, want the resume hint to name preparation's own command", stderr.String())
	}
	if strings.Contains(stderr.String(), closeExpiryResumeHint) {
		t.Fatalf("stderr = %q, inherits close's own freeze/archive resume hint verbatim", stderr.String())
	}
	if strings.Contains(stderr.String(), alignExpiryResumeHint) {
		t.Fatalf("stderr = %q, inherits align's own --wait flag language verbatim", stderr.String())
	}
}

// TestPrepareAlignDeps_IsFreezeAlignDepsWithOnlyTheResumeHintOverridden is
// the structural guard behind the two behavioral tests above: preparation's
// align deps must come from close's single freezeAlignDeps construction, so
// a field added to alignDeps later cannot be silently dropped on this path
// the way Wait and ResumeHint once were. DeepEqual over the whole struct is
// deliberate — it fails on any new field preparation forgets.
func TestPrepareAlignDeps_IsFreezeAlignDepsWithOnlyTheResumeHintOverridden(t *testing.T) {
	deps := closeDeps{
		Runner:        upstream.NewFakeRunner(),
		JudgeCmd:      []string{"/bin/true"},
		JudgeRequired: true,
		JudgeTimeout:  7 * time.Second,
		Forge:         forgefake.New(),
	}
	const digest = "sha256:" + "abc"

	want := freezeAlignDeps(deps, digest)
	got := prepareAlignDeps(deps, digest, "spec/close-fixture")

	if got.ResumeHint == want.ResumeHint {
		t.Fatalf("ResumeHint = %q, want preparation's own resume vocabulary rather than close's", got.ResumeHint)
	}
	if !strings.Contains(got.ResumeHint, "verdi close --prepare spec/close-fixture") {
		t.Fatalf("ResumeHint = %q, want it to name the command that resumes this run", got.ResumeHint)
	}
	want.ResumeHint = got.ResumeHint
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prepareAlignDeps = %+v, want freezeAlignDeps' construction %+v with only ResumeHint overridden", got, want)
	}
	if !got.Wait {
		t.Fatal("Wait = false: a judge timeout would degrade into a synthetic finding instead of an operational expiry")
	}
}

func TestRunPrepare_CurrentUndispositionedPreservesBytesAndPrintsWorklist(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		specName string
		build    func(*testing.T) *fixturegit.Repo
	}{
		{name: "story", ref: "spec/close-fixture", specName: "close-fixture", build: buildCloseFixtureRepo},
		{
			name: "feature", ref: "spec/close-feature-fixture", specName: "close-feature-fixture",
			build: func(t *testing.T) *fixturegit.Repo { return buildCloseFeatureRepo(t, defaultCloseFeatureFixtureOpts()) },
		},
	}

	const findings = `  - { id: f-1, kind: computed, text: "first open finding" }
  - { id: f-2, kind: judged, text: "second open finding" }
  - { id: f-3, kind: computed, text: "already handled", disposition: fixed }
`
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.build(t)
			writePrepareReport(t, repo.Dir, tc.specName, repo.Head, findings)
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, tc.specName)
			beforeRaw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
			sentinel := filepath.Join(t.TempDir(), "judge-invoked")
			deps := closeDeps{
				Runner:   upstream.NewFakeRunner(),
				JudgeCmd: alignFakeJudgeSentinel(t, sentinel),
				Forge:    forgefake.New(),
			}

			runs := 1
			if tc.name == "story" {
				runs = 2
			}
			for run := 1; run <= runs; run++ {
				var stdout, stderr bytes.Buffer
				rc := runPrepare(context.Background(), repo.Dir, tc.ref, &store.Manifest{}, deps, true, &stdout, &stderr)
				if rc != 1 {
					t.Fatalf("runPrepare run %d = %d, want 1; stdout=%s stderr=%s", run, rc, stdout.String(), stderr.String())
				}
				if !strings.Contains(stdout.String(), "JUDGMENT REQUIRED (2 undispositioned finding(s)") {
					t.Fatalf("stdout missing judgment summary on run %d: %s", run, stdout.String())
				}
				for _, id := range []string{"f-1", "f-2"} {
					want := fmt.Sprintf("verdi disposition --rationale '<human-authored rationale>' -- %s %s '<human-authored-disposition:fixed|accepted-deviation>'", tc.ref, id)
					if strings.Count(stdout.String(), want) != 1 {
						t.Fatalf("stdout should contain one exact template %q on run %d: %s", want, run, stdout.String())
					}
				}
				if strings.Contains(stdout.String(), "verdi disposition "+tc.ref+" f-3 ") {
					t.Fatalf("stdout printed work for already-dispositioned f-3: %s", stdout.String())
				}
			}

			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("current report invoked judge; sentinel err=%v", err)
			}
			afterRaw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(beforeRaw, afterRaw) {
				t.Fatal("current undispositioned report changed bytes")
			}
			afterOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
			if beforeOutside != afterOutside {
				t.Fatalf("prepare mutated outside target report:\nbefore: %s\nafter:  %s", beforeOutside, afterOutside)
			}
		})
	}
}

// TestRunPrepare_DisclosesDispositionsBeforeRegeneratingAStaleReport pins
// the honesty obligation on preparation's one destructive path.
//
// A second preparation run at a MOVED head regenerates the report, and
// regeneration carries a disposition forward only where the regenerated
// finding's (kind, id, text) hash matches exactly (align.PreserveDispositions);
// judged findings additionally reach ReconcileJudged, which re-offers a
// non-matching prior ruling as a candidate a human must confirm. So a
// dispositioned finding whose text drifted — or which this run does not
// re-derive at all — loses its disposition AND its human-authored note.
//
// That behavior conforms to the design (retry safety is scoped to "the same
// repository state") and is NOT changed here. What was missing was the
// disclosure: the run destroyed human-authored judgment silently. Silence is
// never a pass, so preparation must name what it is about to regenerate
// over, before it regenerates, through the shared disclosure seam.
func TestRunPrepare_DisclosesDispositionsBeforeRegeneratingAStaleReport(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	const findings = `  - { id: f-1, kind: computed, text: "boundary holds", disposition: fixed, note: "verified by hand against the adapter" }
  - { id: f-2, kind: judged, text: "a prior semantic reading", disposition: accepted-deviation, note: "accepted for this release" }
  - { id: f-3, kind: computed, text: "still open" }
`
	writePrepareReport(t, repo.Dir, "close-fixture", strings.Repeat("a", 40), findings)
	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")

	var stdout, stderr bytes.Buffer
	rc := runPrepare(
		context.Background(),
		repo.Dir,
		"spec/close-fixture",
		&store.Manifest{},
		closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()},
		true,
		&stdout,
		&stderr,
	)
	if rc != 1 {
		t.Fatalf("runPrepare(stale report) = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"disclosed-unproven [close:prepare-regeneration] spec/close-fixture: f-1",
		"disclosed-unproven [close:prepare-regeneration] spec/close-fixture: f-2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout does not disclose the dispositioned finding %q it regenerated over:\n%s", want, out)
		}
	}
	if strings.Contains(out, "] spec/close-fixture: f-3") {
		t.Fatalf("stdout disclosed f-3, which carries no human disposition to lose:\n%s", out)
	}

	// Ordering is the whole point: a disclosure printed after the fact
	// would name work already destroyed.
	discloseAt := strings.Index(out, "disclosed-unproven [close:prepare-regeneration]")
	alignAt := strings.Index(out, "close: --prepare: ALIGNMENT REQUIRED")
	if discloseAt < 0 || alignAt < 0 || discloseAt > alignAt {
		t.Fatalf("disclosure must precede the refresh (disclosure at %d, alignment line at %d):\n%s", discloseAt, alignAt, out)
	}

	// The loss the disclosure is about is real, not hypothetical: neither
	// hand-written finding survives this run's regeneration with its
	// disposition intact.
	updated := decodeReportFile(t, reportPath)
	for _, id := range []string{"f-1", "f-2"} {
		if f, ok := findingByID(updated.Findings, id); ok && f.Dispositioned() {
			t.Fatalf("test premise broken: %s survived regeneration dispositioned (%+v); the disclosure would be describing nothing", id, f)
		}
	}
}

// TestRunPrepare_NoRegenerationDisclosureWithoutDispositionsToLose is the
// negative half: the disclosure is a report of real loss, so an absent
// report and a stale report carrying no human disposition must not print
// one. A disclosure that fires unconditionally teaches operators to ignore
// it.
func TestRunPrepare_NoRegenerationDisclosureWithoutDispositionsToLose(t *testing.T) {
	tests := []struct {
		name     string
		findings string
		// stale is false for the absent-report case.
		stale bool
	}{
		{name: "absent report"},
		{
			name:     "stale report with no dispositions",
			stale:    true,
			findings: "  - { id: f-1, kind: computed, text: \"still open\" }\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildCloseFixtureRepo(t)
			if tc.stale {
				writePrepareReport(t, repo.Dir, "close-fixture", strings.Repeat("a", 40), tc.findings)
			}

			var stdout, stderr bytes.Buffer
			rc := runPrepare(
				context.Background(),
				repo.Dir,
				"spec/close-fixture",
				&store.Manifest{},
				closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()},
				true,
				&stdout,
				&stderr,
			)
			if rc != 1 {
				t.Fatalf("runPrepare = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
			}
			if strings.Contains(stdout.String(), "close:prepare-regeneration") {
				t.Fatalf("preparation disclosed a loss with nothing to lose:\n%s", stdout.String())
			}
		})
	}
}

// alignFakeJudgeNoFindings writes a fake judge returning a well-formed S5
// envelope carrying an EMPTY findings list — a judge that genuinely ran and
// had nothing to say.
//
// It is what makes the regenerate-all-the-way-to-READY path reachable in a
// hermetic test. With no judge configured at all, RunJudged degrades to the
// synthetic judged-coverage-absent finding (internal/align/judged.go), which
// is undispositioned and stops preparation at JUDGMENT REQUIRED — so every
// existing regeneration test returns 1 and never reaches the summary line.
func alignFakeJudgeNoFindings(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	script := "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[]}\"}\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}
	return []string{path}
}

// TestRunPrepare_ReadySummaryCountsPreparationsOwnDisclosures is the whole
// point of counting rather than merely printing.
//
// Preparation's regeneration disclosure went out through the shared seam and
// NOTHING counted it: runPrepare returned whatever runPreflight printed, and
// that summary's disclosure count covers only the closure gate's own
// conditions plus preflight's own sources. So a run could print
// "a human-authored note is about to be destroyed" and then, three lines
// later, "READY (closure gate holds)" — with zero disclosures named. The
// design's state table binds READY to "the existing closure gate is fully
// satisfied with NO disclosures".
//
// The state is reachable exactly as reported: a living report covering an OLD
// head carries a dispositioned COMPUTED finding this run does not re-derive,
// so no undispositioned finding survives the refresh to stop preparation at
// JUDGMENT REQUIRED, the gate holds, and --force-local suppresses the publish
// guard. The note was destroyed; the summary said nothing.
func TestRunPrepare_ReadySummaryCountsPreparationsOwnDisclosures(t *testing.T) {
	repo := readyCloseFixtureRepo(t)
	const findings = `  - { id: f-1, kind: computed, text: "boundary holds", disposition: fixed, note: "verified by hand" }
`
	writePrepareReport(t, repo.Dir, "close-fixture", strings.Repeat("a", 40), findings)

	deps := closeDeps{
		Runner:   upstream.NewFakeRunner(),
		Forge:    forgefake.New(),
		JudgeCmd: alignFakeJudgeNoFindings(t),
	}
	var stdout, stderr bytes.Buffer
	rc := runPrepare(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runPrepare(stale report, regenerates to a holding gate) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}

	out := stdout.String()
	// Premise: this run really did disclose, and the loss it disclosed is real.
	if !strings.Contains(out, "disclosed-unproven [close:prepare-regeneration] spec/close-fixture: f-1") {
		t.Fatalf("fixture bug: this test exists to police a run that DISCLOSED; nothing was disclosed:\n%s", out)
	}
	updated := decodeReportFile(t, store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture"))
	if f, ok := findingByID(updated.Findings, "f-1"); ok {
		t.Fatalf("fixture bug: f-1 survived the refresh (%+v); the disclosure would be describing nothing", f)
	}

	if strings.Contains(out, "READY (closure gate holds") {
		t.Fatalf("a run that disclosed a destroyed human disposition printed bare READY, which the design reserves for a gate satisfied with NO disclosures:\n%s", out)
	}
	if !strings.Contains(out, "READY WITH DISCLOSURES (1 disclosure(s)") {
		t.Fatalf("preparation's own disclosure was printed but never counted into the summary:\n%s", out)
	}
}

// TestRunPrepare_RehearsesTheIndexGuardBeforeItWrites covers preparation's own
// half of the index-guard rehearsal.
//
// Preparation reached the guard only through runPreflight — and it reaches
// runPreflight only AFTER regenerating a stale report, and only when no
// finding is undispositioned. So in both of preparation's own stopping states
// (ALIGNMENT REQUIRED and JUDGMENT REQUIRED) an operator with a dirty index
// was never told that the real close refuses before it evaluates anything, and
// the one rehearsal that did run ran after preparation's single destructive
// write. Hoisting preflight's rehearsal above its resolve does not fix this:
// preparation never gets there.
//
// The rehearsal is a DISCLOSURE here exactly as it is there: no verdict moves,
// nothing new refuses, and preparation still writes only the target report.
func TestRunPrepare_RehearsesTheIndexGuardBeforeItWrites(t *testing.T) {
	ctx := context.Background()
	deps := func(t *testing.T) closeDeps {
		return closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()}
	}

	t.Run("disclosed before the refresh writes, in the ALIGNMENT REQUIRED state", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		appendCloseTestFile(t, filepath.Join(repo.Dir, "verdi.bindings.yaml"), "# staged\n")
		gitOutput(t, repo.Dir, "add", "--", "verdi.bindings.yaml")

		var stdout, stderr bytes.Buffer
		rc := runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps(t), true, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPrepare(absent report, dirty index) = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		out := stdout.String()
		guardAt := strings.Index(out, "disclosed-unproven ["+preflightIndexGuardSource+"]")
		if guardAt < 0 {
			t.Fatalf("preparation refreshed the report and never told the operator the real close refuses at its index guard:\n%s", out)
		}
		writeAt := strings.Index(out, "close: --prepare: ALIGNMENT REQUIRED")
		if writeAt < 0 || guardAt > writeAt {
			t.Fatalf("the rehearsal must precede preparation's own destructive write (guard at %d, refresh at %d):\n%s", guardAt, writeAt, out)
		}
	})

	t.Run("disclosed in the JUDGMENT REQUIRED state, which never reaches preflight", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, undispositionedFindingYAML)
		appendCloseTestFile(t, filepath.Join(repo.Dir, "verdi.bindings.yaml"), "# staged\n")
		gitOutput(t, repo.Dir, "add", "--", "verdi.bindings.yaml")

		var stdout, stderr bytes.Buffer
		rc := runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps(t), true, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPrepare(undispositioned, dirty index) = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stdout.String(), "disclosed-unproven ["+preflightIndexGuardSource+"]") {
			t.Fatalf("JUDGMENT REQUIRED returns before preflight ever runs, so this state disclosed nothing about the index guard:\n%s", stdout.String())
		}
	})

	t.Run("an interrupted closure's residue reaches its own diagnosis", func(t *testing.T) {
		repo := readyCloseFixtureRepo(t)
		if err := store.ArchiveMove(repo.Dir, "close-fixture"); err != nil {
			t.Fatalf("ArchiveMove: %v", err)
		}
		if err := stageClosureSpec(ctx, repo.Dir, "close-fixture"); err != nil {
			t.Fatalf("stageClosureSpec: %v", err)
		}

		var stdout, stderr bytes.Buffer
		rc := runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps(t), true, &stdout, &stderr)
		if rc != 2 {
			t.Fatalf("runPrepare(closure residue) = %d, want 2 — the ref genuinely no longer resolves; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		for _, want := range []string{"interrupted", store.SpecDirRelPath(store.ZoneArchive, "close-fixture"), "git commit"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("--prepare must reach the same residue diagnosis the real close gives, %q:\nstdout=%s\nstderr=%s", want, stdout.String(), stderr.String())
			}
		}
	})

	t.Run("a clean index rehearses nothing", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, undispositionedFindingYAML)

		var stdout, stderr bytes.Buffer
		rc := runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps(t), true, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPrepare(clean index) = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if strings.Contains(stdout.String(), preflightIndexGuardSource) {
			t.Fatalf("a clean index must rehearse nothing — a disclosure that fires unconditionally teaches operators to ignore it:\n%s", stdout.String())
		}
	})

	t.Run("preparation and preflight rehearse the guard once between them", func(t *testing.T) {
		repo := readyCloseFixtureRepo(t)
		appendCloseTestFile(t, filepath.Join(repo.Dir, "verdi.bindings.yaml"), "# staged\n")
		gitOutput(t, repo.Dir, "add", "--", "verdi.bindings.yaml")

		var stdout, stderr bytes.Buffer
		rc := runPrepare(ctx, repo.Dir, "spec/close-fixture", &store.Manifest{}, deps(t), true, &stdout, &stderr)
		if rc != 0 {
			t.Fatalf("runPrepare(ready, dirty index) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		out := stdout.String()
		if got := strings.Count(out, "disclosed-unproven ["+preflightIndexGuardSource+"]"); got != 1 {
			t.Fatalf("the index guard was rehearsed %d times in one run, want exactly 1:\n%s", got, out)
		}
		if !strings.Contains(out, "READY WITH DISCLOSURES (1 disclosure(s)") {
			t.Fatalf("one rehearsal must be counted once:\n%s", out)
		}
	})
}

// writeFrozenPrepareReport writes an ALREADY-FROZEN deviation-report.md
// into specName's active directory — writePrepareReport's frozen twin, and
// the story/prepare-path mirror of closefeature_test.go's
// writeFrozenCloseFeatureReport. A frozen living report is reachable state:
// close freezes the report in place BEFORE it moves the spec to the archive
// zone, so any failure between those two steps leaves exactly this on disk.
func writeFrozenPrepareReport(t *testing.T, root, specName, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", specName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%sfrozen: { at: 2024-01-01, commit: %s }
digest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, covers, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRunPrepare_FrozenLivingReportIsItsOwnOperatorState covers the state
// runPrepare never read: report.Frozen.
//
// A frozen report covering HEAD satisfied the freshness check (it covers
// HEAD) and the disposition check (the closure gate's disposition condition
// does not inspect the frozen stamp), so preparation fell through to a
// clean READY and printed `verdi close <ref>` as the next command — a
// command that structurally cannot succeed, because close's freeze step
// refuses an already-frozen report. A frozen report NOT covering HEAD hit
// that refusal one layer down, surfacing a bare `align:` line with no
// preparation framing, no diagnosis, and no next step.
//
// Preparation must name the state honestly, must not invent a pass path,
// and must not unfreeze anything: a frozen report is immutable, and
// deciding what to do about one is human work.
func TestRunPrepare_FrozenLivingReportIsItsOwnOperatorState(t *testing.T) {
	tests := []struct {
		name  string
		build func(*testing.T) *fixturegit.Repo
		// covers is the frozen report's covers commit; empty means HEAD.
		covers string
	}{
		{
			name:  "frozen report covering HEAD",
			build: readyCloseFixtureRepo,
		},
		{
			name:   "frozen report not covering HEAD",
			build:  buildCloseFixtureRepo,
			covers: strings.Repeat("a", 40),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.build(t)
			covers := tc.covers
			if covers == "" {
				covers = repo.Head
			}
			writeFrozenPrepareReport(t, repo.Dir, "close-fixture", covers, dispositionedFindingYAML)
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
			beforeRaw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

			var stdout, stderr bytes.Buffer
			rc := runPrepare(
				context.Background(),
				repo.Dir,
				"spec/close-fixture",
				&store.Manifest{},
				closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()},
				true,
				&stdout,
				&stderr,
			)
			if rc != 1 {
				t.Fatalf("runPrepare(frozen living report) = %d, want 1 (a verdict: preparation cannot proceed); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
			}
			if strings.Contains(stderr.String(), "align:") {
				t.Fatalf("stderr = %q: the frozen state reached the operator as an unframed align refusal, not as a preparation state", stderr.String())
			}
			for _, want := range []string{
				"close: --prepare: MECHANICAL WORK REQUIRED",
				"already frozen",
				store.DeviationReportRelPath(store.ZoneActive, "close-fixture"),
			} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout missing %q: %s", want, stdout.String())
				}
			}
			if strings.Contains(stdout.String(), "next command") {
				t.Fatalf("preparation offered a next command over a frozen report: %s", stdout.String())
			}
			if strings.Contains(stdout.String(), "READY") {
				t.Fatalf("preparation reported readiness over a frozen report: %s", stdout.String())
			}
			if strings.Contains(stdout.String(), "ALIGNMENT REQUIRED") || strings.Contains(stdout.String(), "JUDGMENT REQUIRED") {
				t.Fatalf("preparation misreported the frozen state as alignment or judgment work: %s", stdout.String())
			}

			decoded := decodeReportFile(t, reportPath)
			if decoded.Frozen == nil {
				t.Fatal("preparation unfroze the report")
			}
			assertPreparePreserved(t, repo.Dir, reportPath, beforeRaw, beforeOutside)
		})
	}
}

func TestRunPrepare_QuotesUnsafeFindingIDInDispositionTemplate(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	const unsafeID = `finding with spaces; $(touch SHOULD_NOT_EXIST) 'quoted'`
	findings := fmt.Sprintf("  - { id: %s, kind: computed, text: \"open finding\" }\n", strconv.Quote(unsafeID))
	writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, findings)

	var stdout, stderr bytes.Buffer
	rc := runPrepare(
		context.Background(),
		repo.Dir,
		"spec/close-fixture",
		&store.Manifest{},
		closeDeps{Forge: forgefake.New()},
		true,
		&stdout,
		&stderr,
	)
	if rc != 1 {
		t.Fatalf("runPrepare = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	const want = `verdi disposition --rationale '<human-authored rationale>' -- spec/close-fixture 'finding with spaces; $(touch SHOULD_NOT_EXIST) '"'"'quoted'"'"'' '<human-authored-disposition:fixed|accepted-deviation>'`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing safely quoted disposition template %q:\n%s", want, stdout.String())
	}
}

func TestRunPrepare_FlagShapedFindingIDTemplateDispositionsIntendedFinding(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	findings := []artifact.Finding{{
		ID:   "--amend",
		Kind: artifact.FindingComputed,
		Text: "flag-shaped finding remains a legal artifact id",
	}}
	report := &artifact.DeviationFrontmatter{
		Schema:   "verdi.deviation/v1",
		Covers:   repo.Head,
		Findings: findings,
		Digest:   "sha256:" + strings.Repeat("0", 64),
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("test setup: flag-shaped finding did not validate: %v", err)
	}
	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
	raw := align.RenderMarkdown(report, align.RenderBody(findings, nil, nil, nil, nil, nil))
	if err := os.WriteFile(reportPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	rc := runPrepare(
		context.Background(),
		repo.Dir,
		"spec/close-fixture",
		&store.Manifest{},
		closeDeps{Forge: forgefake.New()},
		true,
		&stdout,
		&stderr,
	)
	if rc != 1 {
		t.Fatalf("runPrepare = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	const wantTemplate = "verdi disposition --rationale '<human-authored rationale>' -- spec/close-fixture --amend '<human-authored-disposition:fixed|accepted-deviation>'"
	var template string
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "verdi disposition ") {
			template = line
			break
		}
	}
	if template != wantTemplate {
		t.Fatalf("emitted template = %q, want delimiter-safe template %q:\n%s", template, wantTemplate, stdout.String())
	}

	bin := buildVerdiBinary(t)
	command := strings.Replace(
		template,
		shellQuoteWord("<human-authored rationale>"),
		shellQuoteWord("reviewed the flag-shaped finding"),
		1,
	)
	command = strings.Replace(
		command,
		shellQuoteWord("<human-authored-disposition:fixed|accepted-deviation>"),
		shellQuoteWord("fixed"),
		1,
	)
	command = strings.Replace(command, "verdi disposition", shellQuoteWord(bin)+" disposition", 1)
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = repo.Dir
	var dispositionStdout, dispositionStderr bytes.Buffer
	cmd.Stdout = &dispositionStdout
	cmd.Stderr = &dispositionStderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("executing edited emitted template %q: %v; stdout=%s stderr=%s", command, err, dispositionStdout.String(), dispositionStderr.String())
	}
	updated := decodeReportFile(t, reportPath)
	finding, ok := findingByID(updated.Findings, "--amend")
	if !ok || finding.Disposition != artifact.FindingFixed || finding.Note != "reviewed the flag-shaped finding" {
		t.Fatalf("flag-shaped finding after disposition = %+v, want fixed with the supplied rationale", finding)
	}
}

func TestRunPrepare_FullyDispositionedRunsAuthoritativePreflight(t *testing.T) {
	t.Run("mechanical work required", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		beforeRaw, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

		var stdout, stderr bytes.Buffer
		rc := runPrepare(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, closeDeps{Forge: forgefake.New()}, true, &stdout, &stderr)
		if rc != 1 {
			t.Fatalf("runPrepare = %d, want 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		for _, want := range []string{"[FAIL] closure: 1.", "close: --preflight: NOT READY", "close: --prepare: MECHANICAL WORK REQUIRED"} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("stdout missing %q: %s", want, stdout.String())
			}
		}
		assertPreparePreserved(t, repo.Dir, reportPath, beforeRaw, beforeOutside)
	})

	tests := []struct {
		name        string
		ref         string
		build       func(*testing.T) *fixturegit.Repo
		forge       bool
		wantSummary string
	}{
		{
			name: "ready story", ref: "spec/close-fixture", build: readyCloseFixtureRepo, forge: true,
			wantSummary: "close: --preflight: READY (",
		},
		{
			name: "ready story with disclosures", ref: "spec/close-fixture", build: readyCloseFixtureRepo,
			wantSummary: "close: --preflight: READY WITH DISCLOSURES (1 disclosure(s);",
		},
		{
			name: "ready feature", ref: "spec/close-feature-fixture", forge: true,
			build: func(t *testing.T) *fixturegit.Repo {
				opts := defaultCloseFeatureFixtureOpts()
				repo := buildCloseFeatureRepo(t, opts)
				seedCloseFeatureEvidence(t, repo.Dir, repo.Head, opts)
				writeCloseFeatureGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
				return repo
			},
			wantSummary: "close: --preflight: READY (",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.build(t)
			ref, err := artifact.ParseRef(tc.ref)
			if err != nil {
				t.Fatal(err)
			}
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, ref.Name)
			beforeRaw, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
			deps := closeDeps{}
			if tc.forge {
				deps.Forge = forgefake.New()
			}

			var stdout, stderr bytes.Buffer
			rc := runPrepare(context.Background(), repo.Dir, tc.ref, &store.Manifest{}, deps, true, &stdout, &stderr)
			if rc != 0 {
				t.Fatalf("runPrepare = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.wantSummary) {
				t.Fatalf("stdout missing %q: %s", tc.wantSummary, stdout.String())
			}
			wantCommand := "close: --prepare: next command: verdi close " + tc.ref + " --force-local"
			if !strings.Contains(stdout.String(), wantCommand) {
				t.Fatalf("stdout missing exact close command %q: %s", wantCommand, stdout.String())
			}
			assertPreparePreserved(t, repo.Dir, reportPath, beforeRaw, beforeOutside)
		})
	}
}

// TestRunPrepare_NextCommandQuotesTheRefItEchoes covers the last unquoted
// word preparation emits: every word of the disposition templates is quoted,
// but the READY line echoed the caller's ref raw.
//
// storyresolve constrains what can resolve — a story ref must match
// artifact's scheme:key grammar, and a spec ref must be kebab-case — so most
// of the argument space cannot carry shell metacharacters. The exception is
// real: ParseRef also accepts a fragment ref (spec/<name>#<object-id>), and
// Resolve ignores the fragment when loading the spec, so
// `spec/close-fixture#ac-1` resolves and was echoed bare. Under zsh with
// extended_glob set — an ordinary user configuration — `#` is a pattern
// operator, and the pasted line dies with "no matches found" before it ever
// reaches the verb. Quoting the echoed word costs nothing and does not
// depend on that analysis staying true.
func TestRunPrepare_NextCommandQuotesTheRefItEchoes(t *testing.T) {
	repo := readyCloseFixtureRepo(t)

	var stdout, stderr bytes.Buffer
	rc := runPrepare(
		context.Background(),
		repo.Dir,
		"spec/close-fixture#ac-1",
		&store.Manifest{},
		closeDeps{Forge: forgefake.New()},
		true,
		&stdout,
		&stderr,
	)
	if rc != 0 {
		t.Fatalf("runPrepare(fragment ref) = %d, want 0; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	const want = `close: --prepare: next command: verdi close 'spec/close-fixture#ac-1' --force-local`
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing the safely quoted next command %q:\n%s", want, stdout.String())
	}
}

// prepareStoreWithoutGit builds a minimal, VALID store that is not inside a
// git repository: enough for storyresolve.Resolve to load the fixture story,
// but nothing for `git rev-parse HEAD` to answer. It is the only hermetic
// way to reach preparation's HEAD-resolution failure, which runs before any
// report is read. (If the machine's temp directory were itself inside a
// repository, rev-parse would succeed and the test would fail loudly rather
// than silently pass.)
func prepareStoreWithoutGit(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "specs", "active", "close-fixture")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nforge: github\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(closeFixtureStorySpecMD), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestRunPrepare_SetupFailuresReturn2BeforeAnyRefresh covers preparation's
// setup failures — every step that runs before the align engine is ever
// called. Each must be operational (exit 2), must carry preparation's own
// framing so the operator knows which verb refused, and must leave no report
// behind: none of these failures has decided anything about the target.
func TestRunPrepare_SetupFailuresReturn2BeforeAnyRefresh(t *testing.T) {
	tests := []struct {
		name       string
		ref        string
		build      func(*testing.T) string
		wantStderr []string
	}{
		{
			name:       "spec ref names no active spec",
			ref:        "spec/no-such-spec",
			build:      func(t *testing.T) string { return buildCloseFixtureRepo(t).Dir },
			wantStderr: []string{"close: --prepare:", filepath.Join("active", "no-such-spec", "spec.md")},
		},
		{
			name:       "argument is in neither accepted form",
			ref:        "not-a-ref",
			build:      func(t *testing.T) string { return buildCloseFixtureRepo(t).Dir },
			wantStderr: []string{"close: --prepare:", "neither a scheme-prefixed story ref"},
		},
		{
			name:       "story ref matches no active spec",
			ref:        "jira:NOT-A-STORY-1",
			build:      func(t *testing.T) string { return buildCloseFixtureRepo(t).Dir },
			wantStderr: []string{"close: --prepare:", "jira:NOT-A-STORY-1"},
		},
		{
			name:       "HEAD cannot be resolved",
			ref:        "spec/close-fixture",
			build:      prepareStoreWithoutGit,
			wantStderr: []string{"close: --prepare:", `gitx: RevParse("HEAD")`},
		},
		{
			name: "operating model cannot be resolved",
			ref:  "spec/close-fixture",
			build: func(t *testing.T) string {
				repo := buildCloseFixtureRepo(t)
				// A strict-decode failure in the store's own manifest: the
				// digest preparation must stamp into a refreshed report
				// cannot be derived, so the refresh must not run at all.
				if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "verdi.yaml"), []byte("schema: verdi.layout/v1\nnot_a_manifest_field: true\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return repo.Dir
			},
			wantStderr: []string{"close: --prepare:", "verdi.yaml"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.build(t)

			var stdout, stderr bytes.Buffer
			rc := runPrepare(
				context.Background(),
				root,
				tc.ref,
				&store.Manifest{},
				closeDeps{Runner: upstream.NewFakeRunner(), Forge: forgefake.New()},
				true,
				&stdout,
				&stderr,
			)
			if rc != 2 {
				t.Fatalf("runPrepare = %d, want 2 (operational); stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
				}
			}
			reportPath := store.DeviationReportPath(root, store.ZoneActive, "close-fixture")
			if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
				t.Fatalf("a setup failure wrote %s (err=%v); nothing may be written before the target is resolved and refreshable", reportPath, err)
			}
		})
	}
}

// TestRunPrepare_AlignFailurePropagatesItsVerdict covers the refresh-failed
// branch with align's OTHER exit class. A configured-but-absent required
// judge is align's verdict (exit 1), not an operational failure, and
// preparation must return the engine's own class rather than reclassifying
// it — the exit-2 half of the same branch is proven by
// TestRunPrepare_JudgeTimeoutIsOperationalNotASyntheticFinding.
func TestRunPrepare_AlignFailurePropagatesItsVerdict(t *testing.T) {
	repo := buildCloseFixtureRepo(t)
	reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
	deps := closeDeps{Runner: upstream.NewFakeRunner(), JudgeRequired: true, Forge: forgefake.New()}

	var stdout, stderr bytes.Buffer
	rc := runPrepare(context.Background(), repo.Dir, "spec/close-fixture", &store.Manifest{}, deps, true, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("runPrepare(required judge absent) = %d, want align's own verdict 1; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "judge_required") {
		t.Fatalf("stderr = %q, want align's own required-judge diagnostic", stderr.String())
	}
	if strings.Contains(stdout.String(), "ALIGNMENT REQUIRED") {
		t.Fatalf("a failed refresh was reported as a completed one: %s", stdout.String())
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("a failed refresh wrote %s (err=%v)", reportPath, err)
	}
}

// TestReloadRefreshedReport covers preparation's post-refresh contract with
// the align engine: a refresh that reported success must have left a
// decodable report where preparation is about to read it.
//
// The two failure rows are unreachable THROUGH runPrepare by construction —
// nothing executes between align's atomic write and this read, so no
// hermetic test can make the file vanish or rot in between. They are
// nonetheless real post-conditions of a shared engine this code does not
// own, so the check lives in a function that can be driven directly rather
// than as an inline branch no test can reach.
func TestReloadRefreshedReport(t *testing.T) {
	t.Run("decodable report", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, dispositionedFindingYAML)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")

		var stderr bytes.Buffer
		report, rc := reloadRefreshedReport(reportPath, &stderr)
		if rc != 0 {
			t.Fatalf("reloadRefreshedReport = %d, want 0; stderr=%s", rc, stderr.String())
		}
		if report == nil || report.Covers != repo.Head {
			t.Fatalf("report = %+v, want the report covering %s", report, repo.Head)
		}
		if stderr.String() != "" {
			t.Fatalf("stderr = %q, want silence on the success path", stderr.String())
		}
	})

	t.Run("report present but undecodable", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reportPath, []byte("not frontmatter\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stderr bytes.Buffer
		report, rc := reloadRefreshedReport(reportPath, &stderr)
		if rc != 2 {
			t.Fatalf("reloadRefreshedReport(undecodable) = %d, want 2; stderr=%s", rc, stderr.String())
		}
		if report != nil {
			t.Fatalf("report = %+v, want nil", report)
		}
		for _, want := range []string{"close: --prepare:", "frontmatter"} {
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
			}
		}
	})

	t.Run("report absent after a successful refresh", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")

		var stderr bytes.Buffer
		report, rc := reloadRefreshedReport(reportPath, &stderr)
		if rc != 2 {
			t.Fatalf("reloadRefreshedReport(absent) = %d, want 2; stderr=%s", rc, stderr.String())
		}
		if report != nil {
			t.Fatalf("report = %+v, want nil", report)
		}
		for _, want := range []string{"close: --prepare:", "align returned success but", reportPath} {
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
			}
		}
	})
}

func TestRunPrepare_OperationalErrorsReturn2WithoutMutation(t *testing.T) {
	t.Run("malformed current report", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		if err := os.WriteFile(reportPath, []byte("not frontmatter\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		beforeRaw, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

		var stdout, stderr bytes.Buffer
		rc := runPrepare(
			context.Background(),
			repo.Dir,
			"spec/close-fixture",
			&store.Manifest{},
			closeDeps{Forge: forgefake.New()},
			true,
			&stdout,
			&stderr,
		)
		if rc != 2 {
			t.Fatalf("runPrepare(malformed report) = %d, want 2; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "frontmatter") {
			t.Fatalf("stderr = %q, want malformed-report detail", stderr.String())
		}
		assertPreparePreserved(t, repo.Dir, reportPath, beforeRaw, beforeOutside)
	})

	t.Run("downstream preflight transport error", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		writeCloseGateReport(t, repo.Dir, repo.Head, dispositionedFindingYAML)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		beforeRaw, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeOutside := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
		deps := closeDeps{Forge: erroringOpenMRsForge{forgefake.New()}}

		var stdout, stderr bytes.Buffer
		rc := runPrepare(
			context.Background(),
			repo.Dir,
			"spec/close-fixture",
			&store.Manifest{},
			deps,
			true,
			&stdout,
			&stderr,
		)
		if rc != 2 {
			t.Fatalf("runPrepare(preflight transport error) = %d, want 2; stdout=%s stderr=%s", rc, stdout.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "injected transport error") {
			t.Fatalf("stderr = %q, want downstream operational detail", stderr.String())
		}
		assertPreparePreserved(t, repo.Dir, reportPath, beforeRaw, beforeOutside)
	})
}

func writePrepareReport(t *testing.T, root, specName, covers, findingsYAML string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", specName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%sdigest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, strings.Repeat("0", 64))
	if err := os.WriteFile(filepath.Join(dir, "deviation-report.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotOutsidePrepareReport_DetectsIndexAndNonBranchRefMutations(t *testing.T) {
	t.Run("index", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		writePrepareReport(t, repo.Dir, "close-fixture", repo.Head, dispositionedFindingYAML)
		before := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

		gitOutput(t, repo.Dir, "add", store.DeviationReportRelPath(store.ZoneActive, "close-fixture"))

		after := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
		if before == after {
			t.Fatal("snapshot did not detect a target-report index mutation")
		}
	})

	t.Run("non-branch ref", func(t *testing.T) {
		repo := buildCloseFixtureRepo(t)
		reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, "close-fixture")
		before := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)

		gitOutput(t, repo.Dir, "update-ref", "refs/tags/prepare-sentinel", repo.Head)

		after := snapshotOutsidePrepareReport(t, repo.Dir, reportPath)
		if before == after {
			t.Fatal("snapshot did not detect a non-branch ref mutation")
		}
	})
}

func snapshotOutsidePrepareReport(t *testing.T, root, reportPath string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() || path == reportPath {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		entries = append(entries, filepath.ToSlash(rel)+"="+hex.EncodeToString(sum[:]))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot worktree: %v", err)
	}
	sort.Strings(entries)
	return "HEAD=" + gitOutput(t, root, "rev-parse", "HEAD") +
		"branch=" + gitOutput(t, root, "symbolic-ref", "--short", "HEAD") +
		"index=" + gitOutput(t, root, "ls-files", "--stage") +
		"refs=" + gitOutput(t, root, "for-each-ref", "--format=%(refname)=%(objectname) %(symref)") +
		"files=" + strings.Join(entries, ",")
}

func assertPreparePreserved(t *testing.T, root, reportPath string, beforeRaw []byte, beforeOutside string) {
	t.Helper()
	afterRaw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(beforeRaw, afterRaw) {
		t.Fatal("prepare rewrote the target report")
	}
	afterOutside := snapshotOutsidePrepareReport(t, root, reportPath)
	if beforeOutside != afterOutside {
		t.Fatalf("prepare mutated outside target report:\nbefore: %s\nafter:  %s", beforeOutside, afterOutside)
	}
}

var prepareForgeEnvVars = []string{
	"CI_API_V4_URL",
	"CI_PROJECT_ID",
	"CI_JOB_TOKEN",
	"GITHUB_REPOSITORY_OWNER",
	"GITHUB_REPOSITORY",
	"GITHUB_TOKEN",
}

func clearPrepareForgeEnv(t *testing.T) {
	t.Helper()
	for _, name := range prepareForgeEnvVars {
		t.Setenv(name, "")
	}
}
