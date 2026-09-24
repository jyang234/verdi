package gitx

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// buildBlameRepo is a two-commit history: the root commit writes f.txt as
// a/b/c, the second rewrites line 2 and appends line 4. The root commit is
// a history boundary for blame; the second commit is not.
func buildBlameRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"f.txt": "a\nb\nc\n", "other.txt": "x\n"}, Message: "base"},
		{Files: map[string]string{"f.txt": "a\nB\nc\nd\n"}, Message: "edit"},
	})
}

func TestBlame_Happy(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)
	base, edit := repo.Heads[0], repo.Heads[1]

	tests := []struct {
		name       string
		start, end int
		want       []BlameLine
	}{
		{
			name:  "whole file",
			start: 1, end: 4,
			want: []BlameLine{
				{Commit: base, OrigLine: 1, FinalLine: 1, Boundary: true, Filename: "f.txt"},
				{Commit: edit, OrigLine: 2, FinalLine: 2, Filename: "f.txt"},
				{Commit: base, OrigLine: 3, FinalLine: 3, Boundary: true, Filename: "f.txt"},
				{Commit: edit, OrigLine: 4, FinalLine: 4, Filename: "f.txt"},
			},
		},
		{
			name:  "interior range",
			start: 2, end: 3,
			want: []BlameLine{
				{Commit: edit, OrigLine: 2, FinalLine: 2, Filename: "f.txt"},
				{Commit: base, OrigLine: 3, FinalLine: 3, Boundary: true, Filename: "f.txt"},
			},
		},
		{
			name:  "single line",
			start: 4, end: 4,
			want: []BlameLine{{Commit: edit, OrigLine: 4, FinalLine: 4, Filename: "f.txt"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Blame(ctx, repo.Dir, repo.Head, "f.txt", tc.start, tc.end)
			if err != nil {
				t.Fatalf("Blame: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Blame(%d,%d) =\n%+v\nwant\n%+v", tc.start, tc.end, got, tc.want)
			}
		})
	}
}

// TestBlame_ShallowBoundary pins the boundary flag on a shallow clone: the
// grafted tip owns every line and blame marks it a boundary, so a caller
// can never mistake a shallow horizon for the line's true introduction.
func TestBlame_ShallowBoundary(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)
	shallow := fixturegit.ShallowClone(t, repo, 1)

	got, err := Blame(ctx, shallow, repo.Head, "f.txt", 1, 4)
	if err != nil {
		t.Fatalf("Blame(shallow): %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("Blame(shallow) returned %d lines, want 4", len(got))
	}
	for _, l := range got {
		if l.Commit != repo.Head || !l.Boundary {
			t.Fatalf("shallow line %+v: want commit %s marked boundary", l, repo.Head)
		}
	}
}

// TestBlame_RenameReportsOriginalFilename pins that blame follows a whole-
// file rename and reports the path the line had in the commit it is
// attributed to.
func TestBlame_RenameReportsOriginalFilename(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)
	runFor(t, repo.Dir, "mv", "f.txt", "g.txt")
	runFor(t, repo.Dir, "commit", "-q", "--no-verify", "-m", "rename")
	head := strings.TrimSpace(runForOutput(t, repo.Dir, "rev-parse", "HEAD"))

	got, err := Blame(ctx, repo.Dir, head, "g.txt", 2, 2)
	if err != nil {
		t.Fatalf("Blame(renamed): %v", err)
	}
	want := []BlameLine{{Commit: repo.Heads[1], OrigLine: 2, FinalLine: 2, Filename: "f.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Blame(renamed) = %+v, want %+v", got, want)
	}
}

// TestBlame_AmbientConfigCannotRedirect pins the two repository settings
// that would otherwise change attribution: blame.ignoreRevsFile (which
// re-attributes an ignored commit's lines to an older commit) and
// blame.showRoot (which hides boundaries). Blame neutralizes both.
func TestBlame_AmbientConfigCannotRedirect(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)
	edit := repo.Heads[1]
	ignore := filepath.Join(repo.Dir, ".git", "ignore-revs")
	if err := os.WriteFile(ignore, []byte(edit+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runFor(t, repo.Dir, "config", "blame.ignoreRevsFile", ignore)
	runFor(t, repo.Dir, "config", "blame.showRoot", "true")

	got, err := Blame(ctx, repo.Dir, repo.Head, "f.txt", 1, 2)
	if err != nil {
		t.Fatalf("Blame: %v", err)
	}
	want := []BlameLine{
		{Commit: repo.Heads[0], OrigLine: 1, FinalLine: 1, Boundary: true, Filename: "f.txt"},
		{Commit: edit, OrigLine: 2, FinalLine: 2, Filename: "f.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Blame under ambient config = %+v, want %+v", got, want)
	}
}

// TestBlame_ObservedArgv pins the exact argv, including the flags that
// neutralize ambient configuration.
func TestBlame_ObservedArgv(t *testing.T) {
	isolateGitConfig(t)
	repo := buildBlameRepo(t)
	obs := &recordingObserver{}
	ctx := WithObserver(context.Background(), obs)
	if _, err := Blame(ctx, repo.Dir, repo.Head, "f.txt", 1, 2); err != nil {
		t.Fatalf("Blame: %v", err)
	}
	want := [][]string{{repo.Dir,
		"-c", "blame.showRoot=false", "-c", "core.quotePath=false",
		"blame", "--line-porcelain", "--no-textconv", "--ignore-revs-file=",
		"-L", "1,2", repo.Head, "--", "f.txt"}}
	if !reflect.DeepEqual(obs.calls, want) {
		t.Fatalf("observed %v, want %v", obs.calls, want)
	}
}

func TestBlame_Negative(t *testing.T) {
	isolateGitConfig(t)
	ctx := context.Background()
	repo := buildBlameRepo(t)

	tests := []struct {
		name       string
		dir        string
		rev, path  string
		start, end int
		wantSubstr string
	}{
		{name: "range beyond the file", dir: repo.Dir, rev: repo.Head, path: "f.txt", start: 5, end: 5, wantSubstr: "Blame"},
		{name: "range end beyond the file", dir: repo.Dir, rev: repo.Head, path: "f.txt", start: 3, end: 9, wantSubstr: "Blame"},
		{name: "path missing at rev", dir: repo.Dir, rev: repo.Heads[0], path: "missing.txt", start: 1, end: 1, wantSubstr: "Blame"},
		{name: "zero start", dir: repo.Dir, rev: repo.Head, path: "f.txt", start: 0, end: 1, wantSubstr: "line range"},
		{name: "end before start", dir: repo.Dir, rev: repo.Head, path: "f.txt", start: 3, end: 2, wantSubstr: "line range"},
		{name: "option-shaped rev", dir: repo.Dir, rev: "--root", path: "f.txt", start: 1, end: 1, wantSubstr: "rev"},
		{name: "empty rev", dir: repo.Dir, rev: "", path: "f.txt", start: 1, end: 1, wantSubstr: "rev"},
		{name: "empty path", dir: repo.Dir, rev: repo.Head, path: "", start: 1, end: 1, wantSubstr: "path"},
		{name: "not a repository", dir: t.TempDir(), rev: repo.Head, path: "f.txt", start: 1, end: 1, wantSubstr: "Blame"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Blame(ctx, tc.dir, tc.rev, tc.path, tc.start, tc.end)
			if err == nil {
				t.Fatalf("Blame: want error, got %+v", got)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("Blame error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}

func TestParseBlamePorcelain(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	const sha256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	entry := func(commit, orig, final string, extra ...string) string {
		lines := []string{commit + " " + orig + " " + final, "author x", "summary s"}
		lines = append(lines, extra...)
		return strings.Join(lines, "\n") + "\n\tcontent\n"
	}

	good := []struct {
		name       string
		out        string
		start, end int
		want       []BlameLine
	}{
		{
			name: "boundary and filename", start: 7, end: 7,
			out:  entry(sha, "3", "7 1", "boundary", "filename a/b.md"),
			want: []BlameLine{{Commit: sha, OrigLine: 3, FinalLine: 7, Boundary: true, Filename: "a/b.md"}},
		},
		{
			name: "sha256 object id and previous line", start: 1, end: 1,
			out:  entry(sha256, "1", "1", "previous "+sha+" old.md", "filename new.md"),
			want: []BlameLine{{Commit: sha256, OrigLine: 1, FinalLine: 1, Filename: "new.md"}},
		},
		{
			name: "quoted filename", start: 2, end: 2,
			out:  entry(sha, "2", "2", `filename "a\tb.md"`),
			want: []BlameLine{{Commit: sha, OrigLine: 2, FinalLine: 2, Filename: "a\tb.md"}},
		},
		{
			name: "content line that looks like a key", start: 1, end: 2,
			out: entry(sha, "1", "1 2", "filename f") +
				sha + " 2 2\nfilename f\n\tboundary\n",
			want: []BlameLine{
				{Commit: sha, OrigLine: 1, FinalLine: 1, Filename: "f"},
				{Commit: sha, OrigLine: 2, FinalLine: 2, Filename: "f"},
			},
		},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBlamePorcelain([]byte(tc.out), tc.start, tc.end)
			if err != nil {
				t.Fatalf("parseBlamePorcelain: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseBlamePorcelain = %+v, want %+v", got, tc.want)
			}
		})
	}

	bad := []struct {
		name       string
		out        string
		start, end int
	}{
		{name: "empty output", out: "", start: 1, end: 1},
		{name: "missing filename", out: entry(sha, "1", "1"), start: 1, end: 1},
		{name: "short object id", out: entry("abc123", "1", "1", "filename f"), start: 1, end: 1},
		{name: "uppercase object id", out: entry(strings.ToUpper(sha), "1", "1", "filename f"), start: 1, end: 1},
		{name: "non-numeric line", out: entry(sha, "x", "1", "filename f"), start: 1, end: 1},
		{name: "zero orig line", out: entry(sha, "0", "1", "filename f"), start: 1, end: 1},
		{name: "header with too many fields", out: entry(sha, "1", "1 1 9", "filename f"), start: 1, end: 1},
		{name: "final line out of order", out: entry(sha, "1", "2", "filename f"), start: 1, end: 1},
		{name: "fewer lines than requested", out: entry(sha, "1", "1", "filename f"), start: 1, end: 2},
		{name: "more lines than requested", out: entry(sha, "1", "1", "filename f") + entry(sha, "2", "2", "filename f"), start: 1, end: 1},
		{name: "truncated entry", out: sha + " 1 1\nfilename f\n", start: 1, end: 1},
		{name: "duplicate filename", out: entry(sha, "1", "1", "filename f", "filename g"), start: 1, end: 1},
		{name: "empty filename", out: entry(sha, "1", "1", "filename "), start: 1, end: 1},
		{name: "malformed quoted filename", out: entry(sha, "1", "1", `filename "unterminated`), start: 1, end: 1},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parseBlamePorcelain([]byte(tc.out), tc.start, tc.end); err == nil {
				t.Fatalf("parseBlamePorcelain: want error, got %+v", got)
			}
		})
	}
}
