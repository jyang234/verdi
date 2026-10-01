package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/lintratchet"
)

// baselineName is the committed baseline's repository-relative path.
const baselineName = ".golangci.strict-baseline.json"

// capturedReport returns the absolute path of one of internal/lintratchet's
// captured golangci-lint reports.
func capturedReport(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "internal", "lintratchet", "testdata", "reports", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// baselineOf returns a baseline allowing exactly the captured report's
// findings.
func baselineOf(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(capturedReport(t, name))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := lintratchet.ParseReport(data)
	if err != nil {
		t.Fatal(err)
	}
	out, err := lintratchet.EncodeBaseline(lintratchet.CountFindings(findings))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// repoWithBaseline builds a two-commit repository whose default branch,
// origin/main, is the first commit; both commits' baselines allow the base
// capture's findings.
func repoWithBaseline(t *testing.T) string {
	t.Helper()
	base := baselineOf(t, "base")
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{baselineName: base}, Message: "the default branch"},
		{Files: map[string]string{"README": "change\n"}, Message: "a change"},
	})
	cmd := exec.CommandContext(t.Context(), "git", "update-ref", "refs/remotes/origin/main", repo.Heads[0])
	cmd.Dir = repo.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-ref: %v\n%s", err, out)
	}
	return repo.Dir
}

// TestRun covers the command's argument handling and that it hands each
// subcommand to internal/lintratchet with the exit status it decides.
func TestRun(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	cases := []struct {
		name    string
		args    func(t *testing.T, dir string) []string
		want    int
		wantOut string
	}{
		{name: "check: the findings equal the baseline", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "0", "-report", capturedReport(t, "base"), "-baseline", baselineName}
		}, want: 0, wantOut: "lint-strict OK"},
		{name: "check: a new finding", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "0", "-report", capturedReport(t, "newfinding"), "-baseline", baselineName}
		}, want: 1, wantOut: "new finding"},
		{name: "check: golangci-lint failed", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "3", "-report", capturedReport(t, "base"), "-baseline", baselineName}
		}, want: 2, wantOut: "golangci-lint exited 3"},
		{name: "check: a baseline outside the repository", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "0", "-report", capturedReport(t, "base"), "-baseline", "../elsewhere.json"}
		}, want: 2, wantOut: "inside the repository"},
		{name: "baseline: regenerates from the report", args: func(t *testing.T, dir string) []string {
			return []string{"baseline", "-lint-exit", "0", "-report", capturedReport(t, "newfinding"), "-baseline", filepath.Join(dir, "regenerated.json")}
		}, want: 0, wantOut: "3 finding(s) under 3 key(s)"},
		{name: "baseline: golangci-lint failed", args: func(t *testing.T, dir string) []string {
			return []string{"baseline", "-lint-exit", "1", "-report", capturedReport(t, "base"), "-baseline", filepath.Join(dir, "regenerated.json")}
		}, want: 2, wantOut: "golangci-lint exited 1"},
		{name: "no subcommand", args: func(*testing.T, string) []string { return nil }, want: 2, wantOut: "usage"},
		{name: "an unknown subcommand", args: func(*testing.T, string) []string { return []string{"verify"} }, want: 2, wantOut: `unknown subcommand "verify"`},
		{name: "no -lint-exit", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-report", capturedReport(t, "base"), "-baseline", baselineName}
		}, want: 2, wantOut: "-lint-exit"},
		{name: "a negative -lint-exit", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "-4", "-report", capturedReport(t, "base"), "-baseline", baselineName}
		}, want: 2, wantOut: "-lint-exit"},
		{name: "no -report", args: func(*testing.T, string) []string {
			return []string{"check", "-lint-exit", "0", "-baseline", baselineName}
		}, want: 2, wantOut: "-report"},
		{name: "no -baseline", args: func(t *testing.T, dir string) []string {
			return []string{"baseline", "-lint-exit", "0", "-report", capturedReport(t, "base")}
		}, want: 2, wantOut: "-baseline"},
		{name: "an unknown flag", args: func(*testing.T, string) []string { return []string{"check", "-fix"} }, want: 2, wantOut: "flag provided but not defined"},
		{name: "a stray argument", args: func(t *testing.T, dir string) []string {
			return []string{"check", "-lint-exit", "0", "-report", capturedReport(t, "base"), "-baseline", baselineName, "extra"}
		}, want: 2, wantOut: "unexpected argument"},
		{name: "help", args: func(*testing.T, string) []string { return []string{"check", "-h"} }, want: 0, wantOut: "-lint-exit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := repoWithBaseline(t)
			var stdout, stderr bytes.Buffer
			got := run(t.Context(), dir, tc.args(t, dir), &stdout, &stderr)
			out := stdout.String() + stderr.String()
			if got != tc.want || !strings.Contains(out, tc.wantOut) {
				t.Fatalf("run = %d, output %q; want %d containing %q", got, out, tc.want, tc.wantOut)
			}
		})
	}
}

// TestBinaryExitStatus drives the built command end to end and proves the
// process exit status is the check's own: 0, 1, and 2 reach the Makefile
// unchanged.
func TestBinaryExitStatus(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	bin := filepath.Join(t.TempDir(), "lintratchet")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	cases := []struct {
		name     string
		report   string
		lintExit string
		want     int
	}{
		{name: "clean", report: "base", lintExit: "0", want: 0},
		{name: "verdict", report: "newfinding", lintExit: "0", want: 1},
		{name: "operational", report: "base", lintExit: "2", want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), bin, "check", "-lint-exit", tc.lintExit, "-report", capturedReport(t, tc.report), "-baseline", baselineName)
			cmd.Dir = repoWithBaseline(t)
			out, err := cmd.CombinedOutput()
			got := 0
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				got = exitErr.ExitCode()
			} else if err != nil {
				t.Fatalf("running %s: %v", bin, err)
			}
			if got != tc.want {
				t.Fatalf("exit %d, want %d; output:\n%s", got, tc.want, out)
			}
		})
	}
}

// TestInDir covers resolving a path against the run's directory: a relative
// path joins it, an absolute one is kept.
func TestInDir(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "b.json")
	cases := []struct{ dir, p, want string }{
		{".", "b.json", "b.json"},
		{"repo", "sub/b.json", filepath.Join("repo", "sub", "b.json")},
		{"repo", abs, abs},
	}
	for _, tc := range cases {
		if got := inDir(tc.dir, tc.p); got != tc.want {
			t.Errorf("inDir(%q, %q) = %q, want %q", tc.dir, tc.p, got, tc.want)
		}
	}
}
