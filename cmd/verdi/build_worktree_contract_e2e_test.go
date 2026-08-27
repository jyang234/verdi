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
	primary      string
	runway       string
	expectedBase string
	wrongBase    string
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

	primaryBefore := mustATCSnapshot(t, fixture.primary)
	runwayBefore := mustATCSnapshot(t, fixture.runway)
	if !runwayBefore.Detached {
		t.Fatalf("runway starts on branch %q, want detached at accepted-story base %s", runwayBefore.Branch, fixture.expectedBase)
	}

	argv := []string{"build", "start", "spec/" + atcFeatureName}
	observed, err := runATCVerdi(bin, fixture.runway, argv, []string{"CI_DEFAULT_BRANCH=main"})
	if err != nil {
		t.Fatal(err)
	}
	if observed.ExitCode != 0 || observed.ExitClass != "clean" {
		t.Fatalf("verdi %v exit=%d class=%s, want 0/clean; stdout=%s stderr=%s", argv, observed.ExitCode, observed.ExitClass, observed.Stdout, observed.Stderr)
	}

	primaryAfter := mustATCSnapshot(t, fixture.primary)
	assertATCRepositoryUnchanged(t, "primary across build start", primaryBefore, primaryAfter)

	runwayAfter := mustATCSnapshot(t, fixture.runway)
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
	primaryBefore := mustATCSnapshot(t, fixture.primary)
	runwayBefore := mustATCSnapshot(t, fixture.runway)

	var transcript []atcTranscriptRow
	runBoundary := func(command string, argv []string, wantExit int, wantClass, wantWitness string) atcRepositorySnapshot {
		t.Helper()
		observed, err := runATCVerdi(bin, fixture.runway, argv, []string{"CI_DEFAULT_BRANCH=main"})
		if err != nil {
			t.Fatal(err)
		}
		if observed.ExitCode != wantExit || observed.ExitClass != wantClass {
			t.Fatalf("verdi %v exit=%d class=%s, want %d/%s; stdout=%s stderr=%s", argv, observed.ExitCode, observed.ExitClass, wantExit, wantClass, observed.Stdout, observed.Stderr)
		}
		if !strings.Contains(observed.Stdout+"\n"+observed.Stderr, wantWitness) {
			t.Fatalf("verdi %v output lacks deterministic witness %q; stdout=%s stderr=%s", argv, wantWitness, observed.Stdout, observed.Stderr)
		}

		primaryAfter := mustATCSnapshot(t, fixture.primary)
		assertATCRepositoryUnchanged(t, "primary after "+command, primaryBefore, primaryAfter)
		runwayAfter := mustATCSnapshot(t, fixture.runway)
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
		primaryBefore := mustATCSnapshot(t, fixture.primary)
		runwayBefore := mustATCSnapshot(t, fixture.runway)

		argv := []string{"build", "start", "spec/" + atcFeatureName}
		observed, err := runATCVerdi(bin, fixture.runway, argv, []string{"CI_DEFAULT_BRANCH=main"})
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
		assertATCRepositoryUnchanged(t, "wrong-base primary refusal", primaryBefore, mustATCSnapshot(t, fixture.primary))
		assertATCRepositoryUnchanged(t, "wrong-base runway refusal", runwayBefore, mustATCSnapshot(t, fixture.runway))
		t.Logf("Refusal transcript: case=wrong-base expected=%s existing=%s argv=%q exit=%s branch=DETACHED head=%s witness=%q",
			fixture.expectedBase, fixture.wrongBase, argv, observed.ExitClass, runwayBefore.Head, "feature/enum-spike already exists")
	})

	t.Run("injected Git process failure", func(t *testing.T) {
		fixture := newATCRunwayFixture(t, atcFixtureOptions{storySlug: atcStorySlug, epoch: 4})
		primaryBefore := mustATCSnapshot(t, fixture.primary)
		runwayBefore := mustATCSnapshot(t, fixture.runway)
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
		pathOverride := "PATH=" + fakeDir + string(os.PathListSeparator) + os.Getenv("PATH")
		observed, err := runATCVerdi(bin, fixture.runway, argv, []string{"CI_DEFAULT_BRANCH=main", pathOverride, "VATC_GIT_SENTINEL=" + sentinel})
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
		assertATCRepositoryUnchanged(t, "Git-failure primary refusal", primaryBefore, mustATCSnapshot(t, fixture.primary))
		assertATCRepositoryUnchanged(t, "Git-failure runway refusal", runwayBefore, mustATCSnapshot(t, fixture.runway))
		t.Logf("Refusal transcript: case=git-process-failure argv=%q exit=%s branch=DETACHED head=%s stderr=%q",
			argv, observed.ExitClass, runwayBefore.Head, strings.TrimSpace(observed.Stderr))
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
		if _, err := snapshotATCRepository(t.TempDir()); err == nil {
			t.Fatal("snapshotATCRepository(non-repository) succeeded, want operational error")
		}
		if _, err := runATCVerdi(filepath.Join(t.TempDir(), "missing-verdi"), t.TempDir(), []string{"gate"}, nil); err == nil {
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
		before := mustATCSnapshot(t, fixture.runway)
		if err := os.WriteFile(filepath.Join(fixture.runway, "unexpected.txt"), []byte("mutation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		after := mustATCSnapshot(t, fixture.runway)
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
	repo := fixturegit.Build(t, layers)

	var wrongBase string
	if opts.wrongFeatureBase {
		wrongBase = repo.Heads[0]
		mustATCGit(t, repo.Dir, "branch", "feature/"+atcFeatureName, wrongBase)
	}

	runway := filepath.Join(repo.Dir, ".vatc", "worktrees", opts.storySlug, fmt.Sprintf("a%d", opts.epoch))
	if err := os.MkdirAll(filepath.Dir(runway), 0o755); err != nil {
		t.Fatalf("creating ATC runway parent: %v", err)
	}
	mustATCGit(t, repo.Dir, "worktree", "add", "--detach", runway, repo.Head)

	return atcRunwayFixture{primary: repo.Dir, runway: runway, expectedBase: repo.Head, wrongBase: wrongBase}
}

func mustATCGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, code, err := runATCGit(dir, args...)
	if err != nil {
		t.Fatalf("starting git %v: %v", args, err)
	}
	if code != 0 {
		t.Fatalf("git %v exit=%d:\n%s", args, code, out)
	}
	return string(out)
}

func runATCGit(dir string, args ...string) ([]byte, int, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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

func runATCVerdi(bin, dir string, argv, overrides []string) (atcProcessObservation, error) {
	cmd := exec.Command(bin, argv...)
	cmd.Dir = dir
	cmd.Env = atcEnvironment(overrides)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
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

func atcEnvironment(overrides []string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		key, _, _ := strings.Cut(override, "=")
		keys[key] = struct{}{}
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if _, overridden := keys[key]; !overridden {
			env = append(env, value)
		}
	}
	return append(env, overrides...)
}

func mustATCSnapshot(t *testing.T, dir string) atcRepositorySnapshot {
	t.Helper()
	snapshot, err := snapshotATCRepository(dir)
	if err != nil {
		t.Fatalf("snapshotting %s: %v", dir, err)
	}
	return snapshot
}

func snapshotATCRepository(dir string) (atcRepositorySnapshot, error) {
	var snapshot atcRepositorySnapshot
	branch, code, err := runATCGit(dir, "symbolic-ref", "--short", "-q", "HEAD")
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
	if snapshot.Head, commandErr = atcGitText(dir, "rev-parse", "HEAD"); commandErr != nil {
		return snapshot, commandErr
	}
	if snapshot.Tree, commandErr = atcGitText(dir, "rev-parse", "HEAD^{tree}"); commandErr != nil {
		return snapshot, commandErr
	}
	indexPath, commandErr := atcGitText(dir, "rev-parse", "--git-path", "index")
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

	status, code, err := runATCGit(dir, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return snapshot, fmt.Errorf("starting git status: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git status exit %d: %s", code, status)
	}
	snapshot.Status = strings.TrimSuffix(string(status), "\n")
	snapshot.ChangedPaths = atcChangedPaths(snapshot.Status)

	untracked, code, err := runATCGit(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return snapshot, fmt.Errorf("starting git ls-files --others: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git ls-files --others exit %d: %s", code, untracked)
	}
	snapshot.UntrackedPaths = atcCandidatePaths(untracked)

	candidates, code, err := runATCGit(dir, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return snapshot, fmt.Errorf("starting git ls-files candidates: %w", err)
	}
	if code != 0 {
		return snapshot, fmt.Errorf("git ls-files candidates exit %d: %s", code, candidates)
	}
	for _, path := range atcCandidatePaths(candidates) {
		candidate, err := snapshotATCCandidate(dir, path)
		if err != nil {
			return snapshot, err
		}
		snapshot.CandidateFiles = append(snapshot.CandidateFiles, candidate)
	}
	return snapshot, nil
}

func atcGitText(dir string, args ...string) (string, error) {
	out, code, err := runATCGit(dir, args...)
	if err != nil {
		return "", fmt.Errorf("starting git %v: %w", args, err)
	}
	if code != 0 {
		return "", fmt.Errorf("git %v exit %d: %s", args, code, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func atcCandidatePaths(raw []byte) []string {
	parts := bytes.Split(raw, []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		path := filepath.ToSlash(string(part))
		if path == "" || path == ".verdi/data" || strings.HasPrefix(path, ".verdi/data/") {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
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
