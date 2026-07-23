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
	"sort"
	"strconv"
	"strings"
	"testing"

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
			wantText: "usage: verdi close",
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

func TestShellQuoteWord(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple finding id", input: "finding-1", want: "finding-1"},
		{name: "canonical spec ref", input: "spec/close-fixture", want: "spec/close-fixture"},
		{name: "empty", input: "", want: `''`},
		{name: "spaces", input: "two words", want: `'two words'`},
		{name: "shell metacharacters", input: `$HOME; $(touch nope) | <bad>`, want: `'$HOME; $(touch nope) | <bad>'`},
		{name: "single quote", input: `judge's finding`, want: `'judge'"'"'s finding'`},
		{name: "newline", input: "line\nbreak", want: "'line\nbreak'"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shellQuoteWord(tc.input); got != tc.want {
				t.Fatalf("shellQuoteWord(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
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
