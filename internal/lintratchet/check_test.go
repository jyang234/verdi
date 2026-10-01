package lintratchet

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestNewGitEarlier covers resolving the baseline's repository-relative path
// from a path relative to the working directory, and that every git read
// runs at the repository's top level, wherever the check runs from: git
// ls-tree resolves its path against its working directory (review finding
// S1-A1).
func TestNewGitEarlier(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"sub/README": "x\n"}, Message: "init"}})
	sub := filepath.Join(repo.Dir, "sub")
	notRepo := t.TempDir()
	// git prints the top level with symlinks resolved (a temporary directory
	// under /var is /private/var on macOS).
	top, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, dir, path string
		want            string
		wantErr         string
	}{
		{name: "at the root", dir: repo.Dir, path: ".golangci.strict-baseline.json", want: ".golangci.strict-baseline.json"},
		{name: "below the root", dir: sub, path: "baseline.json", want: "sub/baseline.json"},
		{name: "from below the root up to it", dir: sub, path: "../.golangci.strict-baseline.json", want: ".golangci.strict-baseline.json"},
		{name: "empty", dir: repo.Dir, path: "", wantErr: "must be relative"},
		{name: "absolute", dir: repo.Dir, path: filepath.Join(repo.Dir, "b.json"), wantErr: "must be relative"},
		{name: "outside the repository", dir: repo.Dir, path: "../b.json", wantErr: "inside the repository"},
		{name: "the repository itself", dir: sub, path: "..", wantErr: "inside the repository"},
		{name: "not in a repository", dir: notRepo, path: "b.json", wantErr: "locating"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewGitEarlier(t.Context(), tc.dir, tc.path)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("NewGitEarlier = %+v, %v; want an error containing %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewGitEarlier: %v", err)
			}
			if got.Path != tc.want || got.Dir != top {
				t.Fatalf("NewGitEarlier = %+v, want Dir %s (the top level) Path %s", got, top, tc.want)
			}
		})
	}
}

// fixedEarlier is an EarlierBaselineReader with a canned answer.
type fixedEarlier struct {
	earlier EarlierBaseline
	err     error
}

func (f fixedEarlier) ReadEarlierBaseline(context.Context) (EarlierBaseline, error) {
	return f.earlier, f.err
}

// TestCheckReaderEdges covers Check's reader-facing edges that no git
// repository produces: no reader at all, and a reader's own error.
func TestCheckReaderEdges(t *testing.T) {
	baseline := filepath.Join(t.TempDir(), baselineName)
	if err := os.WriteFile(baseline, []byte(capturedBaseline(t, "base")), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		earlier EarlierBaselineReader
		want    int
		wantOut string
	}{
		{name: "an earlier baseline equal to the committed one", earlier: fixedEarlier{earlier: EarlierBaseline{Path: baselineName, Commit: "c0", Where: "w", Present: true, Counts: mustCounts(t, "base")}}, want: 0, wantOut: "allows nothing more than the baseline at w c0"},
		{name: "no reader", earlier: nil, want: 2, wantOut: "no earlier-baseline reader"},
		{name: "the reader fails", earlier: fixedEarlier{err: os.ErrPermission}, want: 2, wantOut: "permission denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Check(t.Context(), CheckInput{ReportPath: capturedReport("base"), BaselinePath: baseline, Earlier: tc.earlier}, &stdout, &stderr)
			out := stdout.String() + stderr.String()
			if code != tc.want || !strings.Contains(out, tc.wantOut) {
				t.Fatalf("Check = %d, output %q; want %d containing %q", code, out, tc.want, tc.wantOut)
			}
		})
	}
}

// mustCounts returns the named captured report's counts.
func mustCounts(t *testing.T, name string) Counts {
	t.Helper()
	c, err := ParseBaseline([]byte(capturedBaseline(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestRenderViolation covers each kind's rendering, including a kind outside
// the closed set, which still renders its counts rather than nothing.
func TestRenderViolation(t *testing.T) {
	k := Key{Linter: "noctx", Package: "p", Message: "m", Source: "\ts"}
	earlier := EarlierBaseline{Commit: "abc", Where: "the merge base of HEAD with origin/main"}
	cases := []struct {
		v    Violation
		want string
	}{
		{Violation{Kind: KindNew, Key: k, Have: 2, Allowed: 1}, `lint-strict: new finding: noctx in p: m (flagged line "\ts"): reported 2 time(s), the baseline allows 1; at p/a.go:3`},
		{Violation{Kind: KindStale, Key: k, Have: 0, Allowed: 1}, `lint-strict: stale allowance: noctx in p: m (flagged line "\ts"): the baseline allows 1, the run reports 0; remove the fixed finding's allowance`},
		{Violation{Kind: KindGrowth, Key: k, Have: 3, Allowed: 1}, `lint-strict: baseline grew: noctx in p: m (flagged line "\ts"): the baseline allows 3, the baseline at the merge base of HEAD with origin/main abc allows 1`},
		{Violation{Kind: Kind(7), Key: k, Have: 3, Allowed: 1}, `lint-strict: unknown violation kind 7: noctx in p: m (flagged line "\ts"): have 3, allowed 1`},
	}
	for _, tc := range cases {
		if got := renderViolation(tc.v, []string{"p/a.go:3"}, earlier); got != tc.want {
			t.Errorf("renderViolation(%v) =\n%s\nwant\n%s", tc.v.Kind, got, tc.want)
		}
	}
}

// TestWriteBaseline covers regenerating the baseline from a report: exactly
// the report's findings in canonical form, and every operational refusal,
// which leaves the existing baseline untouched.
func TestWriteBaseline(t *testing.T) {
	cases := []struct {
		name     string
		lintExit int
		report   func(t *testing.T) string
		target   func(t *testing.T, dir string) string
		want     int
		wantFile string // the baseline's bytes afterwards; "" for unchanged
		wantOut  string
	}{
		{name: "the base capture", report: func(*testing.T) string { return capturedReport("base") }, want: 0, wantOut: "2 finding(s) under 2 key(s)"},
		{name: "a repeated finding", report: func(*testing.T) string { return capturedReport("duplicate") }, want: 0, wantOut: "3 finding(s) under 2 key(s)"},
		{name: "golangci-lint failed", lintExit: 1, report: func(*testing.T) string { return capturedReport("base") }, want: 2, wantOut: "golangci-lint exited 1"},
		{name: "a missing report", report: func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.json") }, want: 2, wantOut: "reading the golangci-lint report"},
		{name: "a typecheck failure", report: func(*testing.T) string { return capturedReport("broken") }, want: 2, wantOut: "could not load the code"},
		{name: "an unwritable target", report: func(*testing.T) string { return capturedReport("base") }, target: func(t *testing.T, dir string) string {
			blocker := filepath.Join(dir, "file")
			if err := os.WriteFile(blocker, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(blocker, baselineName)
		}, want: 2, wantOut: "writing the baseline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, baselineName)
			if tc.target != nil {
				target = tc.target(t, dir)
			} else if err := os.WriteFile(target, []byte("previous\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := WriteBaseline(BaselineInput{LintExit: tc.lintExit, ReportPath: tc.report(t), BaselinePath: target}, &stdout, &stderr)
			out := stdout.String() + stderr.String()
			if code != tc.want || !strings.Contains(out, tc.wantOut) {
				t.Fatalf("WriteBaseline = %d, output %q; want %d containing %q", code, out, tc.want, tc.wantOut)
			}
			if tc.target != nil {
				return
			}
			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if code != 0 {
				if string(got) != "previous\n" {
					t.Fatalf("a refused regeneration changed the baseline to %q", got)
				}
				return
			}
			findings, err := ParseReport(mustRead(t, tc.report(t)))
			if err != nil {
				t.Fatal(err)
			}
			want, err := EncodeBaseline(CountFindings(findings))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("baseline =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// mustRead returns the file's bytes.
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
