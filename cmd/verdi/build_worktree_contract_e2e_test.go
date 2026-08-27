package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

const (
	atcFeatureName = "enum-spike"
	atcStorySlug   = "loan-1490"
	atcReportRel   = ".verdi/specs/active/enum-spike/deviation-report.md"
)

const atcManifestYAML = `schema: verdi.layout/v1
forge: gitlab
toolchain:
  module: github.com/jyang234/golang-code-graph
  commit: cd38b1a56bb782177a207d741a39807821cf2c1c
`

const atcSpikeStorySpecMD = `---
id: spec/enum-spike
kind: spec
class: story
title: "Enumeration spike"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1490
spike: true
problem: { text: "which enumeration approach is right", anchor: "#problem" }
outcome: { text: "a recommendation recorded", anchor: "#outcome" }
links:
  - { type: resolves, ref: "spec/some-feature#oq-1" }
frozen: { at: 2024-01-01, commit: 000000000000000000000000000000000000000a }
---
# Enumeration spike

## Problem

Which enumeration approach is right?

## Outcome

A recommendation is recorded.
`

const atcGitIgnore = `/.vatc/
/.verdi/data/
`

type atcFixtureOptions struct {
	storySlug        string
	epoch            int
	wrongFeatureBase bool
}

type atcRunwayFixture struct {
	primary          string
	runway           string
	runwayRel        string
	expectedBase     string
	wrongBase        string
	childEnvironment atcChildEnvironment
}

type atcChildEnvironment struct {
	base      []string
	toolPath  string
	gitBinary string
}

type atcVerdiEnvironmentOptions struct {
	gitExecutableDir string
	gitSentinel      string
}

type atcCandidateFile struct {
	Path   string
	Mode   string
	Digest string
}

type atcRepositorySnapshot struct {
	Branch         string
	Detached       bool
	Head           string
	Tree           string
	Index          []byte
	IndexDigest    string
	Status         string
	ChangedPaths   []string
	UntrackedPaths []string
	CandidateFiles []atcCandidateFile
}

type atcProcessObservation struct {
	ExitCode  int
	ExitClass string
	Stdout    string
	Stderr    string
}

type atcTranscriptRow struct {
	Command      string
	Argv         []string
	ExitCode     int
	ExitClass    string
	Branch       string
	Detached     bool
	Head         string
	Tree         string
	Status       string
	ChangedPaths []string
}

func TestBuildCommandsFromATCRunway_BuildStart(t *testing.T) {
	bin := buildVerdiBinary(t)
	fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 1})

	primaryBefore := mustATCPrimarySnapshot(t, fixture)
	runwayBefore := mustATCRunwaySnapshot(t, fixture)
	if !runwayBefore.Detached {
		t.Fatalf("runway starts on branch %q, want detached at accepted-story base %s", runwayBefore.Branch, fixture.expectedBase)
	}

	argv := []string{"build", "start", "spec/" + atcFeatureName}
	observed, err := runATCVerdi(bin, fixture.runway, argv, fixture.childEnvironment, atcVerdiEnvironmentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if observed.ExitCode != 0 || observed.ExitClass != "clean" {
		t.Fatalf("verdi %v exit=%d class=%s, want 0/clean; stdout=%s stderr=%s", argv, observed.ExitCode, observed.ExitClass, observed.Stdout, observed.Stderr)
	}

	primaryAfter := mustATCPrimarySnapshot(t, fixture)
	assertATCRepositoryUnchanged(t, "primary across build start", primaryBefore, primaryAfter)

	runwayAfter := mustATCRunwaySnapshot(t, fixture)
	wantRunway := runwayBefore
	wantRunway.Branch = "feature/" + atcFeatureName
	wantRunway.Detached = false
	if diff := atcSnapshotDifference(wantRunway, runwayAfter); diff != "" {
		t.Fatalf("build start changed more than the runway branch attachment: %s", diff)
	}
	if runwayAfter.Head != fixture.expectedBase {
		t.Fatalf("runway HEAD = %s, want accepted-story base %s", runwayAfter.Head, fixture.expectedBase)
	}
	if primaryAfter.Branch == runwayAfter.Branch {
		t.Fatalf("primary silently checked out runway branch %q", runwayAfter.Branch)
	}

	t.Logf("BuildStart transcript: command=build start argv=%q exit=%s branch=%s head=%s tree=%s status=%q changed=%v",
		argv, observed.ExitClass, runwayAfter.Branch, runwayAfter.Head, runwayAfter.Tree, runwayAfter.Status, runwayAfter.ChangedPaths)
}

func TestBuildCommandsFromATCRunway_Lifecycle(t *testing.T) {
	bin := buildVerdiBinary(t)
	fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 2})
	primaryBefore := mustATCPrimarySnapshot(t, fixture)
	runwayBefore := mustATCRunwaySnapshot(t, fixture)

	var transcript []atcTranscriptRow
	runBoundary := func(command string, argv []string, wantExit int, wantClass, wantWitness string) atcRepositorySnapshot {
		t.Helper()
		observed, err := runATCVerdi(bin, fixture.runway, argv, fixture.childEnvironment, atcVerdiEnvironmentOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if observed.ExitCode != wantExit || observed.ExitClass != wantClass {
			t.Fatalf("verdi %v exit=%d class=%s, want %d/%s; stdout=%s stderr=%s", argv, observed.ExitCode, observed.ExitClass, wantExit, wantClass, observed.Stdout, observed.Stderr)
		}
		if !strings.Contains(observed.Stdout+"\n"+observed.Stderr, wantWitness) {
			t.Fatalf("verdi %v output lacks deterministic witness %q; stdout=%s stderr=%s", argv, wantWitness, observed.Stdout, observed.Stderr)
		}

		primaryAfter := mustATCPrimarySnapshot(t, fixture)
		assertATCRepositoryUnchanged(t, "primary after "+command, primaryBefore, primaryAfter)
		runwayAfter := mustATCRunwaySnapshot(t, fixture)
		transcript = append(transcript, atcTranscriptRow{
			Command:      command,
			Argv:         append([]string(nil), argv...),
			ExitCode:     observed.ExitCode,
			ExitClass:    observed.ExitClass,
			Branch:       runwayAfter.Branch,
			Detached:     runwayAfter.Detached,
			Head:         runwayAfter.Head,
			Tree:         runwayAfter.Tree,
			Status:       runwayAfter.Status,
			ChangedPaths: append([]string(nil), runwayAfter.ChangedPaths...),
		})
		return runwayAfter
	}

	afterStart := runBoundary("build start", []string{"build", "start", "spec/" + atcFeatureName}, 0, "clean", "created branch feature/"+atcFeatureName)
	wantAfterStart := runwayBefore
	wantAfterStart.Branch = "feature/" + atcFeatureName
	wantAfterStart.Detached = false
	if diff := atcSnapshotDifference(wantAfterStart, afterStart); diff != "" {
		t.Fatalf("build start changed more than the runway branch attachment: %s", diff)
	}

	afterAlign := runBoundary("align", []string{"align"}, 0, "clean", atcReportRel)
	assertATCChangedPaths(t, "align", afterAlign, []string{atcReportRel})
	reportPath := filepath.Join(fixture.runway, filepath.FromSlash(atcReportRel))
	report := decodeReportFile(t, reportPath)
	if len(report.Findings) != 1 || report.Findings[0].Dispositioned() {
		t.Fatalf("align findings = %+v, want one honest undispositioned synthetic judge-absence finding", report.Findings)
	}
	findingID := report.Findings[0].ID

	afterDisposition := runBoundary("disposition", []string{
		"disposition", "spec/" + atcFeatureName, findingID, "fixed", "--rationale", "synthetic judge absence acknowledged for hermetic runway proof",
	}, 0, "clean", "recorded spec/"+atcFeatureName)
	assertATCChangedPaths(t, "disposition", afterDisposition, []string{atcReportRel})
	if bytes.Equal(atcCandidateDigest(afterAlign, atcReportRel), atcCandidateDigest(afterDisposition, atcReportRel)) {
		t.Fatal("disposition reported success without changing the runway report bytes")
	}

	afterGate := runBoundary("gate", []string{"gate"}, 1, "verdict", "countersign")
	assertATCRepositoryUnchanged(t, "runway across gate verdict", afterDisposition, afterGate)

	afterPrepare := runBoundary("close --prepare", []string{"close", "--prepare", "spec/" + atcFeatureName, "--force-local"}, 2, "operational", "declares no acceptance criteria")
	assertATCRepositoryUnchanged(t, "runway across close --prepare verdict", afterGate, afterPrepare)

	if len(transcript) != 5 {
		t.Fatalf("transcript has %d rows, want one row per lifecycle command (5)", len(transcript))
	}
	for i, row := range transcript {
		if row.Command == "" || len(row.Argv) == 0 || row.ExitClass == "" || row.Head == "" || row.Tree == "" {
			t.Fatalf("transcript row %d is incomplete: %+v", i, row)
		}
		if row.Detached {
			t.Fatalf("transcript row %d (%s) unexpectedly detached after build start: %+v", i, row.Command, row)
		}
		if row.Branch != "feature/"+atcFeatureName || row.Head != fixture.expectedBase {
			t.Fatalf("transcript row %d escaped the runway/base: %+v", i, row)
		}
		t.Logf("Lifecycle transcript[%d]: command=%s argv=%q exit=%d/%s branch=%s head=%s tree=%s status=%q changed=%v",
			i, row.Command, row.Argv, row.ExitCode, row.ExitClass, row.Branch, row.Head, row.Tree, row.Status, row.ChangedPaths)
	}
}

func TestBuildCommandsFromATCRunway_Refusals(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("existing feature branch at wrong base", func(t *testing.T) {
		fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 3, wrongFeatureBase: true})
		if fixture.wrongBase == "" || fixture.wrongBase == fixture.expectedBase {
			t.Fatalf("wrong-base fixture has branch base %q and expected base %q", fixture.wrongBase, fixture.expectedBase)
		}
		primaryBefore := mustATCPrimarySnapshot(t, fixture)
		runwayBefore := mustATCRunwaySnapshot(t, fixture)

		argv := []string{"build", "start", "spec/" + atcFeatureName}
		observed, err := runATCVerdi(bin, fixture.runway, argv, fixture.childEnvironment, atcVerdiEnvironmentOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if observed.ExitCode != 2 || observed.ExitClass != "operational" {
			t.Fatalf("wrong-base branch exit=%d class=%s, want 2/operational; stdout=%s stderr=%s", observed.ExitCode, observed.ExitClass, observed.Stdout, observed.Stderr)
		}
		for _, want := range []string{"feature/" + atcFeatureName, "already exists"} {
			if !strings.Contains(observed.Stderr, want) {
				t.Fatalf("wrong-base stderr = %q, want deterministic witness %q", observed.Stderr, want)
			}
		}
		assertATCRepositoryUnchanged(t, "wrong-base primary refusal", primaryBefore, mustATCPrimarySnapshot(t, fixture))
		assertATCRepositoryUnchanged(t, "wrong-base runway refusal", runwayBefore, mustATCRunwaySnapshot(t, fixture))
		t.Logf("Refusal transcript: case=wrong-base expected=%s existing=%s argv=%q exit=%s branch=DETACHED head=%s witness=%q",
			fixture.expectedBase, fixture.wrongBase, argv, observed.ExitClass, runwayBefore.Head, "feature/enum-spike already exists")
	})

	t.Run("injected Git process failure", func(t *testing.T) {
		fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 4})
		primaryBefore := mustATCPrimarySnapshot(t, fixture)
		runwayBefore := mustATCRunwaySnapshot(t, fixture)
		fakeDir := writeATCGitFailureFake(t)
		sentinel := filepath.Join(t.TempDir(), "git-invoked")
		entries, err := os.ReadDir(fakeDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "git" {
			t.Fatalf("process fake directory = %v, want only the git executable", entries)
		}

		argv := []string{"build", "start", "spec/" + atcFeatureName}
		observed, err := runATCVerdi(bin, fixture.runway, argv, fixture.childEnvironment, atcVerdiEnvironmentOptions{
			gitExecutableDir: fakeDir,
			gitSentinel:      sentinel,
		})
		if err != nil {
			t.Fatal(err)
		}
		if observed.ExitCode != 2 || observed.ExitClass != "operational" {
			t.Fatalf("injected Git failure exit=%d class=%s, want 2/operational; stdout=%s stderr=%s", observed.ExitCode, observed.ExitClass, observed.Stdout, observed.Stderr)
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("Git fake invocation sentinel: %v", err)
		}
		for _, want := range []string{"cannot determine the default branch", "failing closed", "git remote set-head"} {
			if !strings.Contains(observed.Stderr, want) {
				t.Fatalf("stderr = %q, want deterministic operational witness %q", observed.Stderr, want)
			}
		}
		assertATCRepositoryUnchanged(t, "Git-failure primary refusal", primaryBefore, mustATCPrimarySnapshot(t, fixture))
		assertATCRepositoryUnchanged(t, "Git-failure runway refusal", runwayBefore, mustATCRunwaySnapshot(t, fixture))
		t.Logf("Refusal transcript: case=git-process-failure argv=%q exit=%s branch=DETACHED head=%s stderr=%q",
			argv, observed.ExitClass, runwayBefore.Head, strings.TrimSpace(observed.Stderr))
	})

	t.Run("hermetic child environment drops ambient Git steering", func(t *testing.T) {
		ambient := []string{
			"GIT_DIR=/developer/repository/.git",
			"GIT_WORK_TREE=/developer/repository",
			"GIT_INDEX_FILE=/developer/repository/.git/index",
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=core.excludesfile",
			"GIT_CONFIG_VALUE_0=/developer/global-ignore",
			"GIT_ASKPASS=/developer/credential-helper",
			"HTTPS_PROXY=http://developer-proxy.invalid",
			"HOME=/developer/home",
			"LC_ALL=developer-locale",
			"PWD=/developer/stale-working-directory",
		}
		for _, entry := range ambient {
			key, value, _ := strings.Cut(entry, "=")
			t.Setenv(key, value)
		}

		environment := newATCChildEnvironment(t)
		childValues, err := environment.verdi(atcVerdiEnvironmentOptions{})
		if err != nil {
			t.Fatal(err)
		}
		values := make(map[string]string, len(childValues))
		for _, entry := range childValues {
			key, value, _ := strings.Cut(entry, "=")
			values[key] = value
		}
		var keys []string
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		wantKeys := []string{
			"CI_DEFAULT_BRANCH",
			"GIT_CONFIG_GLOBAL",
			"GIT_CONFIG_NOSYSTEM",
			"GIT_TERMINAL_PROMPT",
			"HOME",
			"LANG",
			"LC_ALL",
			"PATH",
			"TMPDIR",
			"XDG_CONFIG_HOME",
		}
		if !reflect.DeepEqual(keys, wantKeys) {
			t.Fatalf("Verdi child environment keys = %v, want exact allowlist %v", keys, wantKeys)
		}
		for _, entry := range ambient {
			key, value, _ := strings.Cut(entry, "=")
			if values[key] == value {
				t.Fatalf("child environment retained ambient %s=%q", key, value)
			}
		}
		if values["PATH"] != environment.toolPath || values["CI_DEFAULT_BRANCH"] != "main" {
			t.Fatalf("declared child inputs = PATH=%q CI_DEFAULT_BRANCH=%q, want explicit values", values["PATH"], values["CI_DEFAULT_BRANCH"])
		}
		if values["HOME"] == "" || values["HOME"] == "/developer/home" || values["XDG_CONFIG_HOME"] == "" {
			t.Fatalf("isolated config boundary = HOME=%q XDG_CONFIG_HOME=%q", values["HOME"], values["XDG_CONFIG_HOME"])
		}
		if values["LC_ALL"] != "C" || values["LANG"] != "C" || values["GIT_CONFIG_NOSYSTEM"] != "1" || values["GIT_CONFIG_GLOBAL"] != os.DevNull {
			t.Fatalf("fixed locale/Git config boundary = LC_ALL=%q LANG=%q GIT_CONFIG_NOSYSTEM=%q GIT_CONFIG_GLOBAL=%q",
				values["LC_ALL"], values["LANG"], values["GIT_CONFIG_NOSYSTEM"], values["GIT_CONFIG_GLOBAL"])
		}
		if _, err := environment.verdi(atcVerdiEnvironmentOptions{gitExecutableDir: t.TempDir()}); err == nil {
			t.Fatal("Git fake directory without invocation sentinel succeeded, want rejected incomplete override")
		}
		if _, err := environment.verdi(atcVerdiEnvironmentOptions{gitSentinel: filepath.Join(t.TempDir(), "sentinel")}); err == nil {
			t.Fatal("Git fake sentinel without executable directory succeeded, want rejected incomplete override")
		}
	})

	t.Run("primary snapshot observes ignored candidates outside exact runway", func(t *testing.T) {
		fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 6})
		primaryBefore := mustATCPrimarySnapshot(t, fixture)

		fallbackMarker := filepath.Join(fixture.primary, ".vatc", "fallback-marker")
		if err := os.WriteFile(fallbackMarker, []byte("redirected mutation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		primaryAfterFallback := mustATCPrimarySnapshot(t, fixture)
		if diff := atcSnapshotDifference(primaryBefore, primaryAfterFallback); diff == "" || !strings.Contains(diff, ".vatc/fallback-marker") {
			t.Fatalf("ignored fallback mutation difference = %q, want exact candidate witness", diff)
		}

		if err := os.Remove(fallbackMarker); err != nil {
			t.Fatal(err)
		}
		siblingPath := filepath.Join(fixture.primary, ".vatc", "worktrees", "sibling-story", "a9", "mutation.txt")
		if err := os.MkdirAll(filepath.Dir(siblingPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(siblingPath, []byte("sibling mutation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		primaryAfterSibling := mustATCPrimarySnapshot(t, fixture)
		if diff := atcSnapshotDifference(primaryBefore, primaryAfterSibling); diff == "" || !strings.Contains(diff, ".vatc/worktrees/sibling-story/a9/mutation.txt") {
			t.Fatalf("ignored sibling mutation difference = %q, want exact candidate witness", diff)
		}

		runwayBefore := mustATCRunwaySnapshot(t, fixture)
		runwayOnly := filepath.Join(fixture.runway, "separately-owned.txt")
		if err := os.WriteFile(runwayOnly, []byte("runway mutation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		primaryAfterRunway := mustATCPrimarySnapshot(t, fixture)
		assertATCRepositoryUnchanged(t, "primary across exact separately-owned runway mutation", primaryAfterSibling, primaryAfterRunway)
		if diff := atcSnapshotDifference(runwayBefore, mustATCRunwaySnapshot(t, fixture)); diff == "" || !strings.Contains(diff, "separately-owned.txt") {
			t.Fatalf("separately-owned runway mutation difference = %q, want runway candidate witness", diff)
		}
	})

	t.Run("helper negative paths fail closed", func(t *testing.T) {
		for _, opts := range []atcFixtureOptions{
			{epoch: 1},
			{storySlug: atcStorySlug, epoch: 0},
			{storySlug: "../escape", epoch: 1},
		} {
			if err := validateATCFixtureOptions(opts); err == nil {
				t.Fatalf("validateATCFixtureOptions(%+v) succeeded, want error", opts)
			}
		}
		environment := newATCChildEnvironment(t)
		if _, err := snapshotATCRepository(t.TempDir(), environment, ""); err == nil {
			t.Fatal("snapshotATCRepository(non-repository) succeeded, want operational error")
		}
		if _, err := runATCVerdi(filepath.Join(t.TempDir(), "missing-verdi"), t.TempDir(), []string{"gate"}, environment, atcVerdiEnvironmentOptions{}); err == nil {
			t.Fatal("runATCVerdi(missing executable) succeeded, want process-start error")
		}
		for _, code := range []int{0, 1, 2} {
			if _, err := classifyATCExit(code); err != nil {
				t.Fatalf("classifyATCExit(%d): %v", code, err)
			}
		}
		if _, err := classifyATCExit(3); err == nil {
			t.Fatal("classifyATCExit(3) succeeded, want fail-closed unknown-class error")
		}

		fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 5})
		before := mustATCRunwaySnapshot(t, fixture)
		if err := os.WriteFile(filepath.Join(fixture.runway, "unexpected.txt"), []byte("mutation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		after := mustATCRunwaySnapshot(t, fixture)
		if diff := atcSnapshotDifference(before, after); diff == "" || !strings.Contains(diff, "candidate files") {
			t.Fatalf("snapshot difference = %q, want candidate-file mutation witness", diff)
		}
	})
}

func validateATCFixtureOptions(opts atcFixtureOptions) error {
	if opts.storySlug == "" {
		return errors.New("story slug is required")
	}
	if opts.epoch <= 0 {
		return fmt.Errorf("epoch must be positive: %d", opts.epoch)
	}
	if filepath.Base(opts.storySlug) != opts.storySlug || opts.storySlug == "." || strings.ContainsAny(opts.storySlug, `/\\`) {
		return fmt.Errorf("story slug must be one path component: %q", opts.storySlug)
	}
	return nil
}

func newATCRunwayFixture(t *testing.T, opts atcFixtureOptions) atcRunwayFixture {
	t.Helper()
	if err := validateATCFixtureOptions(opts); err != nil {
		t.Fatal(err)
	}
	childEnvironment := newATCChildEnvironment(t)

	layers := []fixturegit.Layer{{
		Files: map[string]string{
			".gitignore":                             atcGitIgnore,
			".verdi/verdi.yaml":                      atcManifestYAML,
			".verdi/specs/active/enum-spike/spec.md": atcSpikeStorySpecMD,
		},
		Message: "land accepted spike story",
	}}
	if opts.wrongFeatureBase {
		layers = append(layers, fixturegit.Layer{
			Files:   map[string]string{"README.md": "accepted story build base\n"},
			Message: "advance accepted story base",
		})
	}
	var repoDir, repoHead string
	var repoHeads []string
	withATCProcessEnvironment(t, childEnvironment.git(), func() {
		repo := fixturegit.Build(t, layers)
		repoDir = repo.Dir
		repoHead = repo.Head
		repoHeads = append([]string(nil), repo.Heads...)
	})

	var wrongBase string
	if opts.wrongFeatureBase {
		wrongBase = repoHeads[0]
		mustATCGit(t, repoDir, childEnvironment, "branch", "feature/"+atcFeatureName, wrongBase)
	}

	runwayRel := filepath.Join(".vatc", "worktrees", opts.storySlug, fmt.Sprintf("a%d", opts.epoch))
	runway := filepath.Join(repoDir, runwayRel)
	if err := os.MkdirAll(filepath.Dir(runway), 0o755); err != nil {
		t.Fatalf("creating ATC runway parent: %v", err)
	}
	mustATCGit(t, repoDir, childEnvironment, "worktree", "add", "--detach", runway, repoHead)

	return atcRunwayFixture{
		primary:          repoDir,
		runway:           runway,
		runwayRel:        filepath.ToSlash(runwayRel),
		expectedBase:     repoHead,
		wrongBase:        wrongBase,
		childEnvironment: childEnvironment,
	}
}

func newATCChildEnvironment(t *testing.T) atcChildEnvironment {
	t.Helper()
	toolPath := os.Getenv("PATH")
	if toolPath == "" {
		t.Fatal("constructing hermetic child environment: PATH is required for the real Git/toolchain binaries")
	}
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("constructing hermetic child environment: resolve git: %v", err)
	}
	boundary := t.TempDir()
	home := filepath.Join(boundary, "home")
	xdgConfig := filepath.Join(boundary, "xdg-config")
	tmp := filepath.Join(boundary, "tmp")
	for _, dir := range []string{home, xdgConfig, tmp} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("constructing hermetic child environment directory %s: %v", dir, err)
		}
	}
	return atcChildEnvironment{
		base: []string{
			"GIT_CONFIG_GLOBAL=" + os.DevNull,
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_TERMINAL_PROMPT=0",
			"HOME=" + home,
			"LANG=C",
			"LC_ALL=C",
			"TMPDIR=" + tmp,
			"XDG_CONFIG_HOME=" + xdgConfig,
		},
		toolPath:  toolPath,
		gitBinary: gitBinary,
	}
}

func (environment atcChildEnvironment) git() []string {
	values := append([]string(nil), environment.base...)
	return append(values, "PATH="+environment.toolPath)
}

func (environment atcChildEnvironment) verdi(options atcVerdiEnvironmentOptions) ([]string, error) {
	if (options.gitExecutableDir == "") != (options.gitSentinel == "") {
		return nil, errors.New("Git failure fake requires both executable directory and invocation sentinel")
	}
	toolPath := environment.toolPath
	values := append([]string(nil), environment.base...)
	if options.gitExecutableDir != "" {
		if !filepath.IsAbs(options.gitExecutableDir) || !filepath.IsAbs(options.gitSentinel) {
			return nil, errors.New("Git failure fake directory and invocation sentinel must be absolute")
		}
		toolPath = options.gitExecutableDir + string(os.PathListSeparator) + toolPath
		values = append(values, "VATC_GIT_SENTINEL="+options.gitSentinel)
	}
	values = append(values, "CI_DEFAULT_BRANCH=main", "PATH="+toolPath)
	return values, nil
}

func withATCProcessEnvironment(t *testing.T, environment []string, run func()) {
	t.Helper()
	original := os.Environ()
	defer func() {
		if err := replaceATCProcessEnvironment(original); err != nil {
			t.Errorf("restoring process environment: %v", err)
		}
	}()
	if err := replaceATCProcessEnvironment(environment); err != nil {
		t.Fatalf("installing hermetic fixture environment: %v", err)
	}
	run()
}

func replaceATCProcessEnvironment(environment []string) error {
	os.Clearenv()
	for _, entry := range environment {
		key, value, found := strings.Cut(entry, "=")
		if !found || key == "" {
			return fmt.Errorf("invalid child environment entry %q", entry)
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set child environment %s: %w", key, err)
		}
	}
	return nil
}

func mustATCGit(t *testing.T, dir string, environment atcChildEnvironment, args ...string) string {
	t.Helper()
	out, code, err := runATCGit(dir, environment, args...)
	if err != nil {
		t.Fatalf("starting git %v: %v", args, err)
	}
	if code != 0 {
		t.Fatalf("git %v exit=%d:\n%s", args, code, out)
	}
	return string(out)
}

func runATCGit(dir string, environment atcChildEnvironment, args ...string) ([]byte, int, error) {
	cmd := exec.Command(environment.gitBinary, args...)
	cmd.Dir = dir
	cmd.Env = environment.git()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode(), nil
	}
	return out, -1, err
}

func runATCVerdi(bin, dir string, argv []string, environment atcChildEnvironment, options atcVerdiEnvironmentOptions) (atcProcessObservation, error) {
	childEnvironment, err := environment.verdi(options)
	if err != nil {
		return atcProcessObservation{}, fmt.Errorf("constructing Verdi child environment: %w", err)
	}
	cmd := exec.Command(bin, argv...)
	cmd.Dir = dir
	cmd.Env = childEnvironment
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return atcProcessObservation{}, fmt.Errorf("starting verdi %v in %s: %w", argv, dir, err)
		}
		exitCode = exitErr.ExitCode()
	}
	exitClass, err := classifyATCExit(exitCode)
	if err != nil {
		return atcProcessObservation{}, fmt.Errorf("verdi %v in %s: %w; stdout=%s stderr=%s", argv, dir, err, stdout.String(), stderr.String())
	}
	return atcProcessObservation{ExitCode: exitCode, ExitClass: exitClass, Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func classifyATCExit(code int) (string, error) {
	switch code {
	case 0:
		return "clean", nil
	case 1:
		return "verdict", nil
	case 2:
		return "operational", nil
	default:
		return "", fmt.Errorf("unknown Verdi exit class %d", code)
	}
}

func mustATCPrimarySnapshot(t *testing.T, fixture atcRunwayFixture) atcRepositorySnapshot {
	t.Helper()
	return mustATCSnapshot(t, fixture.primary, fixture.childEnvironment, fixture.runwayRel)
}

func mustATCRunwaySnapshot(t *testing.T, fixture atcRunwayFixture) atcRepositorySnapshot {
	t.Helper()
	return mustATCSnapshot(t, fixture.runway, fixture.childEnvironment, "")
}

func mustATCSnapshot(t *testing.T, dir string, environment atcChildEnvironment, excludedSubtree string) atcRepositorySnapshot {
	t.Helper()
	snapshot, err := snapshotATCRepository(dir, environment, excludedSubtree)
	if err != nil {
		t.Fatalf("snapshotting %s: %v", dir, err)
	}
	return snapshot
}

func snapshotATCRepository(dir string, environment atcChildEnvironment, excludedSubtree string) (atcRepositorySnapshot, error) {
	var snapshot atcRepositorySnapshot
	excludedSubtree, err := normalizeATCExcludedSubtree(excludedSubtree)
	if err != nil {
		return snapshot, err
	}
	branch, code, err := runATCGit(dir, environment, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return snapshot, fmt.Errorf("starting git symbolic-ref: %w", err)
	}
	switch code {
	case 0:
		snapshot.Branch = strings.TrimSpace(string(branch))
	case 1:
		snapshot.Detached = true
	default:
		return snapshot, fmt.Errorf("git symbolic-ref exit %d: %s", code, branch)
	}

	var commandErr error
	if snapshot.Head, commandErr = atcGitText(dir, environment, "rev-parse", "HEAD"); commandErr != nil {
		return snapshot, commandErr
	}
	if snapshot.Tree, commandErr = atcGitText(dir, environment, "rev-parse", "HEAD^{tree}"); commandErr != nil {
		return snapshot, commandErr
	}
	indexPath, commandErr := atcGitText(dir, environment, "rev-parse", "--git-path", "index")
	if commandErr != nil {
		return snapshot, commandErr
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(dir, indexPath)
	}
	snapshot.Index, commandErr = os.ReadFile(indexPath)
	if commandErr != nil {
		return snapshot, fmt.Errorf("reading Git index %s: %w", indexPath, commandErr)
	}
	snapshot.IndexDigest = atcDigest(snapshot.Index)

	status, code, err := runATCGit(dir, environment, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return snapshot, fmt.Errorf("starting git status: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git status exit %d: %s", code, status)
	}

	tracked, code, err := runATCGit(dir, environment, "ls-files", "--cached", "-z")
	if err != nil {
		return snapshot, fmt.Errorf("starting git ls-files --cached: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git ls-files --cached exit %d: %s", code, tracked)
	}
	trackedPaths := atcCandidatePaths(tracked, excludedSubtree)
	trackedSet := make(map[string]struct{}, len(trackedPaths))
	for _, path := range trackedPaths {
		trackedSet[path] = struct{}{}
	}

	ordinaryUntracked, code, err := runATCGit(dir, environment, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return snapshot, fmt.Errorf("starting git ls-files --others: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git ls-files --others exit %d: %s", code, ordinaryUntracked)
	}
	ordinaryUntrackedSet := make(map[string]struct{})
	for _, path := range atcCandidatePaths(ordinaryUntracked, excludedSubtree) {
		ordinaryUntrackedSet[path] = struct{}{}
	}

	filesystemPaths, err := atcFilesystemCandidatePaths(dir, excludedSubtree)
	if err != nil {
		return snapshot, err
	}
	candidateSet := make(map[string]struct{}, len(trackedPaths)+len(filesystemPaths))
	for _, path := range trackedPaths {
		candidateSet[path] = struct{}{}
	}
	var ignoredPaths []string
	for _, path := range filesystemPaths {
		candidateSet[path] = struct{}{}
		if _, tracked := trackedSet[path]; tracked {
			continue
		}
		snapshot.UntrackedPaths = append(snapshot.UntrackedPaths, path)
		if _, ordinary := ordinaryUntrackedSet[path]; !ordinary {
			ignoredPaths = append(ignoredPaths, path)
		}
	}
	sort.Strings(snapshot.UntrackedPaths)
	snapshot.Status = atcStatusWithIgnored(strings.TrimSuffix(string(status), "\n"), ignoredPaths)
	snapshot.ChangedPaths = atcChangedPaths(snapshot.Status)

	candidatePaths := make([]string, 0, len(candidateSet))
	for path := range candidateSet {
		candidatePaths = append(candidatePaths, path)
	}
	sort.Strings(candidatePaths)
	for _, path := range candidatePaths {
		candidate, err := snapshotATCCandidate(dir, path)
		if err != nil {
			return snapshot, err
		}
		snapshot.CandidateFiles = append(snapshot.CandidateFiles, candidate)
	}
	return snapshot, nil
}

func atcGitText(dir string, environment atcChildEnvironment, args ...string) (string, error) {
	out, code, err := runATCGit(dir, environment, args...)
	if err != nil {
		return "", fmt.Errorf("starting git %v: %w", args, err)
	}
	if code != 0 {
		return "", fmt.Errorf("git %v exit %d: %s", args, code, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func normalizeATCExcludedSubtree(excludedSubtree string) (string, error) {
	if excludedSubtree == "" {
		return "", nil
	}
	if filepath.IsAbs(excludedSubtree) {
		return "", fmt.Errorf("snapshot exclusion must be repository-relative: %q", excludedSubtree)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(excludedSubtree)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("snapshot exclusion escapes repository: %q", excludedSubtree)
	}
	return clean, nil
}

func atcCandidatePaths(raw []byte, excludedSubtree string) []string {
	parts := bytes.Split(raw, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		path := filepath.ToSlash(string(part))
		if path == "" || atcCandidatePathExcluded(path, excludedSubtree) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func atcFilesystemCandidatePaths(root, excludedSubtree string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk candidate %s: %w", path, walkErr)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("candidate path relative to %s: %w", root, err)
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if atcCandidatePathExcluded(rel, excludedSubtree) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			paths = append(paths, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func atcCandidatePathExcluded(path, excludedSubtree string) bool {
	for _, excluded := range []string{".git", ".verdi/data", excludedSubtree} {
		if excluded != "" && (path == excluded || strings.HasPrefix(path, excluded+"/")) {
			return true
		}
	}
	return false
}

func atcStatusWithIgnored(status string, ignoredPaths []string) string {
	var lines []string
	if status != "" {
		lines = append(lines, strings.Split(status, "\n")...)
	}
	for _, path := range ignoredPaths {
		lines = append(lines, "!! "+path)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func snapshotATCCandidate(root, path string) (atcCandidateFile, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(fullPath)
	if errors.Is(err, fs.ErrNotExist) {
		return atcCandidateFile{Path: path, Mode: "missing", Digest: "missing"}, nil
	}
	if err != nil {
		return atcCandidateFile{}, fmt.Errorf("lstat candidate %s: %w", path, err)
	}
	var content []byte
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(fullPath)
		if err != nil {
			return atcCandidateFile{}, fmt.Errorf("readlink candidate %s: %w", path, err)
		}
		content = []byte(target)
	} else {
		content, err = os.ReadFile(fullPath)
		if err != nil {
			return atcCandidateFile{}, fmt.Errorf("read candidate %s: %w", path, err)
		}
	}
	return atcCandidateFile{Path: path, Mode: info.Mode().String(), Digest: atcDigest(content)}, nil
}

func atcChangedPaths(status string) []string {
	if status == "" {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		path := line[3:]
		if _, renamed, ok := strings.Cut(path, " -> "); ok {
			path = renamed
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func atcDigest(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func atcSnapshotDifference(want, got atcRepositorySnapshot) string {
	var fields []string
	if want.Branch != got.Branch || want.Detached != got.Detached {
		fields = append(fields, fmt.Sprintf("branch/detached %q/%v -> %q/%v", want.Branch, want.Detached, got.Branch, got.Detached))
	}
	if want.Head != got.Head {
		fields = append(fields, "HEAD "+want.Head+" -> "+got.Head)
	}
	if want.Tree != got.Tree {
		fields = append(fields, "tree "+want.Tree+" -> "+got.Tree)
	}
	if !bytes.Equal(want.Index, got.Index) {
		fields = append(fields, "index "+want.IndexDigest+" -> "+got.IndexDigest)
	}
	if want.Status != got.Status {
		fields = append(fields, fmt.Sprintf("status %q -> %q", want.Status, got.Status))
	}
	if !reflect.DeepEqual(want.ChangedPaths, got.ChangedPaths) {
		fields = append(fields, fmt.Sprintf("changed paths %v -> %v", want.ChangedPaths, got.ChangedPaths))
	}
	if !reflect.DeepEqual(want.UntrackedPaths, got.UntrackedPaths) {
		fields = append(fields, fmt.Sprintf("untracked paths %v -> %v", want.UntrackedPaths, got.UntrackedPaths))
	}
	if !reflect.DeepEqual(want.CandidateFiles, got.CandidateFiles) {
		fields = append(fields, fmt.Sprintf("candidate files %v -> %v", want.CandidateFiles, got.CandidateFiles))
	}
	return strings.Join(fields, "; ")
}

func assertATCRepositoryUnchanged(t *testing.T, boundary string, want, got atcRepositorySnapshot) {
	t.Helper()
	if diff := atcSnapshotDifference(want, got); diff != "" {
		t.Fatalf("%s mutated: %s", boundary, diff)
	}
}

func assertATCChangedPaths(t *testing.T, boundary string, snapshot atcRepositorySnapshot, want []string) {
	t.Helper()
	if !reflect.DeepEqual(snapshot.ChangedPaths, want) {
		t.Fatalf("%s changed paths = %v, want %v", boundary, snapshot.ChangedPaths, want)
	}
}

func atcCandidateDigest(snapshot atcRepositorySnapshot, path string) []byte {
	for _, candidate := range snapshot.CandidateFiles {
		if candidate.Path == path {
			return []byte(candidate.Digest)
		}
	}
	return nil
}

func writeATCGitFailureFake(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "git")
	script := "#!/bin/sh\n: > \"${VATC_GIT_SENTINEL:?}\"\nprintf '%s\\n' 'vatc-u3 injected git operational failure' >&2\nexit 73\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing Git process fake: %v", err)
	}
	return dir
}
