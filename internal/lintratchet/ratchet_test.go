package lintratchet

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// baselineName is the committed baseline's repository-relative path.
const baselineName = ".golangci.strict-baseline.json"

// capturedReport returns the path of a report capture.sh captured from the
// pinned golangci-lint over the variant module of that name.
func capturedReport(name string) string {
	return filepath.Join("testdata", "reports", name+".json")
}

// capturedBaseline returns a baseline allowing exactly the findings of the
// named captured report.
func capturedBaseline(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(capturedReport(name))
	if err != nil {
		t.Fatalf("reading captured report %s: %v", name, err)
	}
	findings, err := ParseReport(data)
	if err != nil {
		t.Fatalf("parsing captured report %s: %v", name, err)
	}
	out, err := EncodeBaseline(CountFindings(findings))
	if err != nil {
		t.Fatalf("encoding the baseline of %s: %v", name, err)
	}
	return string(out)
}

// git runs git in dir and returns its trimmed stdout.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

// Baseline contents a commit can hold, beside a captured report's name.
const (
	noBaseline        = ""
	malformedBaseline = "malformed"
)

// buildRepo commits one layer per entry of baselines: a README naming the
// commit, and the committed baseline allowing the named captured report's
// findings (noBaseline: no baseline file; malformedBaseline: bytes that are
// not a baseline). Once a layer holds a baseline file, later layers keep it
// unless they replace it.
func buildRepo(t *testing.T, baselines ...string) *fixturegit.Repo {
	t.Helper()
	layers := make([]fixturegit.Layer, len(baselines))
	for i, b := range baselines {
		files := map[string]string{"README": "commit " + strconv.Itoa(i) + "\n"}
		switch b {
		case noBaseline:
		case malformedBaseline:
			files[baselineName] = "{\"findings\": [\n"
		default:
			files[baselineName] = capturedBaseline(t, b)
		}
		layers[i] = fixturegit.Layer{Files: files, Message: "commit " + strconv.Itoa(i)}
	}
	return fixturegit.Build(t, layers)
}

// setDefaultBranch points refs/remotes/origin/main, the default branch
// specstate resolves when nothing else names one, at commit.
func setDefaultBranch(t *testing.T, dir, commit string) {
	t.Helper()
	git(t, dir, "update-ref", "refs/remotes/origin/main", commit)
}

// check runs Check at the repository root dir against the named captured
// report with the given golangci-lint exit status, and returns its exit
// status and output.
func check(t *testing.T, dir, report string, lintExit int) (int, string) {
	t.Helper()
	return checkFrom(t, dir, baselineName, report, lintExit)
}

// checkFrom runs Check as cmd/lintratchet runs it: from the working
// directory dir, with the baseline at baselinePath relative to dir, and the
// earlier baseline read through NewGitEarlier.
func checkFrom(t *testing.T, dir, baselinePath, report string, lintExit int) (int, string) {
	t.Helper()
	earlier, err := NewGitEarlier(t.Context(), dir, baselinePath)
	if err != nil {
		t.Fatalf("NewGitEarlier(%s, %s): %v", dir, baselinePath, err)
	}
	var stdout, stderr bytes.Buffer
	code := Check(t.Context(), CheckInput{
		LintExit:     lintExit,
		ReportPath:   report,
		BaselinePath: filepath.Join(dir, filepath.FromSlash(baselinePath)),
		Earlier:      earlier,
	}, &stdout, &stderr)
	out := stdout.String() + stderr.String()
	t.Logf("exit %d:\n%s", code, out)
	return code, out
}

// checkIn runs check at repo's root, or, when fromSub, from a directory
// below the root with the baseline named relative to it, as `lintratchet
// check -baseline ../.golangci.strict-baseline.json` run there names it.
func checkIn(t *testing.T, repo *fixturegit.Repo, fromSub bool, report string) (int, string) {
	t.Helper()
	if !fromSub {
		return check(t, repo.Dir, report, 0)
	}
	sub := filepath.Join(repo.Dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	return checkFrom(t, sub, "../"+baselineName, report, 0)
}

// TestRatchet_Verdicts proves the baseline check's verdicts (spec/strict-lint-gate
// ac-2) over captured golangci-lint reports and fixturegit repositories with a
// baseline at the merge base and at the first parent: equal findings and
// baseline exit 0; a new finding, a stale allowance, or a baseline grown
// against the merge base, or on the default branch against HEAD's first
// parent, exits 1; a line shift or a move within a package exits 0; a move
// across packages, or an edit of the flagged line, exits 1; a fix followed by
// a later reintroduction fails the later change; and a truncated report, a
// malformed baseline, a failing golangci-lint, or an unresolvable merge base
// or parent exits 2. An earlier commit that resolves but holds no baseline
// makes the growth comparison not applicable, disclosed, while the other two
// comparisons still apply (ledger SI-311 (1)). Each comparison also holds
// when the check runs from a directory below the root (review finding
// S1-A1).
func TestRatchet_Verdicts(t *testing.T) {
	// The default branch is origin/main in every repository below, resolved
	// by specstate's fallback; an ambient CI_DEFAULT_BRANCH must not override
	// it.
	t.Setenv("CI_DEFAULT_BRANCH", "")

	// Feature-branch cases: the default branch is the first commit, so the
	// merge base is that commit; HEAD is the second, whose committed baseline
	// is checked against the named captured report.
	feature := []struct {
		name               string
		mergeBase, head    string // baselines committed at the merge base and at HEAD
		report             string
		fromSub            bool // the check runs from a directory below the root
		want               int
		wantOut            string
		wantOutAtMergeBase bool // wantOut is followed by the merge base's commit
	}{
		{name: "equal findings and baseline", mergeBase: "base", head: "base", report: "base", want: 0, wantOut: "lint-strict OK"},
		{name: "a new finding", mergeBase: "base", head: "base", report: "newfinding", want: 1, wantOut: "new finding: gochecknoglobals in alpha"},
		{name: "a fixed finding's allowance left behind", mergeBase: "base", head: "base", report: "fixed", want: 1, wantOut: "stale allowance: gochecknoglobals in alpha"},
		{name: "a fix that removes its allowance", mergeBase: "base", head: "fixed", report: "fixed", want: 0, wantOut: "lint-strict OK"},
		{name: "a line shift", mergeBase: "base", head: "base", report: "lineshift", want: 0, wantOut: "lint-strict OK"},
		{name: "a move within a package", mergeBase: "base", head: "base", report: "movefile", want: 0, wantOut: "lint-strict OK"},
		{name: "a move across packages", mergeBase: "base", head: "base", report: "movepkg", want: 1, wantOut: "new finding: gochecknoglobals in beta"},
		{name: "an edited flagged line", mergeBase: "base", head: "base", report: "editline", want: 1, wantOut: `new finding: gochecknoglobals in alpha: Counter is a global variable (flagged line "var Counter int64")`},
		{name: "an identical finding repeated", mergeBase: "base", head: "base", report: "duplicate", want: 1, wantOut: "reported 2 time(s), the baseline allows 1"},
		{name: "a repeated finding's count left behind", mergeBase: "duplicate", head: "duplicate", report: "base", want: 1, wantOut: "the baseline allows 2, the run reports 1"},
		{name: "a second finding on an already-flagged line", mergeBase: "base", head: "base", report: "sameline", want: 1, wantOut: "new finding: contextcheck in alpha"},
		{name: "a baseline grown against the merge base", mergeBase: "base", head: "newfinding", report: "newfinding", want: 1, wantOut: "baseline grew: gochecknoglobals in alpha"},
		{name: "a baseline count grown against the merge base", mergeBase: "base", head: "duplicate", report: "duplicate", want: 1, wantOut: "the baseline allows 2, the baseline at the merge base of HEAD with origin/main"},
		{name: "a baseline shrunk against the merge base", mergeBase: "duplicate", head: "base", report: "base", want: 0, wantOut: "lint-strict OK"},
		{name: "bootstrap: no baseline at the merge base", mergeBase: noBaseline, head: "base", report: "base", want: 0, wantOut: "disclosed-unproven [lint-strict:growth-comparison] ", wantOutAtMergeBase: true},
		{name: "bootstrap: a new finding still fails", mergeBase: noBaseline, head: "base", report: "newfinding", want: 1, wantOut: "new finding: gochecknoglobals in alpha"},
		{name: "bootstrap: a stale allowance still fails", mergeBase: noBaseline, head: "base", report: "fixed", want: 1, wantOut: "stale allowance: gochecknoglobals in alpha"},
		{name: "a malformed baseline at the merge base", mergeBase: malformedBaseline, head: "base", report: "base", want: 2, wantOut: "malformed"},
		{name: "from a subdirectory: equal findings and baseline", mergeBase: "base", head: "base", report: "base", fromSub: true, want: 0, wantOut: "it allows nothing more than the baseline at the merge base of HEAD with origin/main "},
		{name: "from a subdirectory: a baseline grown against the merge base", mergeBase: "base", head: "newfinding", report: "newfinding", fromSub: true, want: 1, wantOut: "baseline grew: gochecknoglobals in alpha"},
	}
	for _, tc := range feature {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildRepo(t, tc.mergeBase, tc.head)
			setDefaultBranch(t, repo.Dir, repo.Heads[0])
			code, out := checkIn(t, repo, tc.fromSub, capturedReport(tc.report))
			if code != tc.want {
				t.Fatalf("exit %d, want %d; output:\n%s", code, tc.want, out)
			}
			want := tc.wantOut
			if tc.wantOutAtMergeBase {
				want += repo.Heads[0]
			}
			if !strings.Contains(out, want) {
				t.Fatalf("output does not contain %q:\n%s", want, out)
			}
		})
	}

	// Default-branch cases: HEAD is the default branch's tip, so the merge
	// base is HEAD itself and the growth comparison reads HEAD's first parent.
	onDefault := []struct {
		name          string
		parent, head  string
		report        string
		fromSub       bool // the check runs from a directory below the root
		want          int
		wantOut       string
		wantOutParent bool
	}{
		{name: "on the default branch: equal", parent: "base", head: "base", report: "base", want: 0, wantOut: "lint-strict OK"},
		{name: "on the default branch: shrunk", parent: "base", head: "fixed", report: "fixed", want: 0, wantOut: "lint-strict OK"},
		{name: "on the default branch: grown against the first parent", parent: "base", head: "newfinding", report: "newfinding", want: 1, wantOut: "baseline grew: gochecknoglobals in alpha"},
		{name: "on the default branch: bootstrap, no baseline at the first parent", parent: noBaseline, head: "base", report: "base", want: 0, wantOut: "disclosed-unproven [lint-strict:growth-comparison] ", wantOutParent: true},
		{name: "on the default branch, from a subdirectory: grown against the first parent", parent: "base", head: "newfinding", report: "newfinding", fromSub: true, want: 1, wantOut: "baseline grew: gochecknoglobals in alpha"},
	}
	for _, tc := range onDefault {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildRepo(t, tc.parent, tc.head)
			setDefaultBranch(t, repo.Dir, repo.Head)
			code, out := checkIn(t, repo, tc.fromSub, capturedReport(tc.report))
			if code != tc.want {
				t.Fatalf("exit %d, want %d; output:\n%s", code, tc.want, out)
			}
			want := tc.wantOut
			if tc.wantOutParent {
				want += repo.Heads[0]
			}
			if !strings.Contains(out, want) {
				t.Fatalf("output does not contain %q:\n%s", want, out)
			}
			if tc.want == 1 && !strings.Contains(out, "HEAD's first parent") {
				t.Fatalf("output does not name HEAD's first parent as the earlier baseline:\n%s", out)
			}
		})
	}

	t.Run("a fix in one change and its reintroduction in a later change", func(t *testing.T) {
		// One repository: the default branch at commit 0 allows the finding;
		// commit 1 fixes it and removes its allowance; commit 2 reintroduces
		// it without the allowance; commit 3 reintroduces it with the
		// allowance put back.
		repo := buildRepo(t, "base", "fixed", "fixed", "base")
		// The fix, checked against the default branch before it: passes.
		git(t, repo.Dir, "checkout", "--quiet", "--detach", repo.Heads[1])
		setDefaultBranch(t, repo.Dir, repo.Heads[0])
		if code, out := check(t, repo.Dir, capturedReport("fixed"), 0); code != 0 {
			t.Fatalf("the fix: exit %d, want 0; output:\n%s", code, out)
		}
		// The fix lands on the default branch.
		setDefaultBranch(t, repo.Dir, repo.Heads[1])
		// The later change reintroduces the finding, baseline untouched.
		git(t, repo.Dir, "checkout", "--quiet", "--detach", repo.Heads[2])
		if code, out := check(t, repo.Dir, capturedReport("base"), 0); code != 1 || !strings.Contains(out, "new finding: gochecknoglobals in alpha") {
			t.Fatalf("the reintroduction without its allowance: exit %d, want 1 naming the new finding; output:\n%s", code, out)
		}
		// The later change reintroduces it and puts its allowance back.
		git(t, repo.Dir, "checkout", "--quiet", "--detach", repo.Heads[3])
		if code, out := check(t, repo.Dir, capturedReport("base"), 0); code != 1 || !strings.Contains(out, "baseline grew: gochecknoglobals in alpha") {
			t.Fatalf("the reintroduction with its allowance: exit %d, want 1 naming the grown baseline; output:\n%s", code, out)
		}
	})

	// Operational failures: each exits 2, never 0 or 1.
	operational := []struct {
		name    string
		setup   func(t *testing.T) (dir, report string, lintExit int)
		wantOut string
	}{
		{name: "golangci-lint exited 1", setup: func(t *testing.T) (string, string, int) {
			return featureRepo(t), capturedReport("base"), 1
		}, wantOut: "golangci-lint exited 1"},
		{name: "golangci-lint exited 3 (another run held its lock)", setup: func(t *testing.T) (string, string, int) {
			return featureRepo(t), capturedReport("base"), 3
		}, wantOut: "golangci-lint exited 3"},
		{name: "golangci-lint could not load a package", setup: func(t *testing.T) (string, string, int) {
			return featureRepo(t), capturedReport("broken"), 0
		}, wantOut: "could not load the code"},
		{name: "a truncated report", setup: func(t *testing.T) (string, string, int) {
			data, err := os.ReadFile(capturedReport("base"))
			if err != nil {
				t.Fatal(err)
			}
			truncated := filepath.Join(t.TempDir(), "truncated.json")
			if err := os.WriteFile(truncated, data[:len(data)/2], 0o644); err != nil {
				t.Fatal(err)
			}
			return featureRepo(t), truncated, 0
		}, wantOut: "golangci-lint report"},
		{name: "a missing report", setup: func(t *testing.T) (string, string, int) {
			return featureRepo(t), filepath.Join(t.TempDir(), "absent.json"), 0
		}, wantOut: "reading the golangci-lint report"},
		{name: "a malformed baseline", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, "base", malformedBaseline)
			setDefaultBranch(t, repo.Dir, repo.Heads[0])
			return repo.Dir, capturedReport("base"), 0
		}, wantOut: "baseline"},
		{name: "a missing baseline", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, noBaseline, noBaseline)
			setDefaultBranch(t, repo.Dir, repo.Heads[0])
			return repo.Dir, capturedReport("base"), 0
		}, wantOut: "reading the baseline"},
		{name: "no default branch resolves", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, "base", "base")
			return repo.Dir, capturedReport("base"), 0
		}, wantOut: "cannot determine the default branch"},
		{name: "no merge base with the default branch", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, "base", "base")
			orphan := git(t, repo.Dir, "commit-tree", "-m", "unrelated history", git(t, repo.Dir, "mktree"))
			setDefaultBranch(t, repo.Dir, orphan)
			return repo.Dir, capturedReport("base"), 0
		}, wantOut: "no merge base"},
		{name: "HEAD's first parent beyond a shallow clone's horizon", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, "base", "base")
			clone := fixturegit.ShallowClone(t, repo, 1)
			return clone, capturedReport("base"), 0
		}, wantOut: "HEAD's first parent"},
		{name: "a root commit on the default branch has no first parent", setup: func(t *testing.T) (string, string, int) {
			repo := buildRepo(t, "base")
			setDefaultBranch(t, repo.Dir, repo.Head)
			return repo.Dir, capturedReport("base"), 0
		}, wantOut: "HEAD's first parent"},
	}
	for _, tc := range operational {
		t.Run(tc.name, func(t *testing.T) {
			dir, report, lintExit := tc.setup(t)
			code, out := check(t, dir, report, lintExit)
			if code != 2 {
				t.Fatalf("exit %d, want 2; output:\n%s", code, out)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("output does not contain %q:\n%s", tc.wantOut, out)
			}
		})
	}
}

// featureRepo builds a two-commit repository whose default branch is the
// first commit and whose baselines both allow the base capture's findings.
func featureRepo(t *testing.T) string {
	t.Helper()
	repo := buildRepo(t, "base", "base")
	setDefaultBranch(t, repo.Dir, repo.Heads[0])
	return repo.Dir
}
