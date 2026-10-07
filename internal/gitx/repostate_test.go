package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// The whole-repository readers return git's exact stdout, so each happy
// case compares against git's own output for the same argv, run here
// directly, and then checks the reading means what its doc says.

func TestVersion(t *testing.T) {
	repo := buildRepo(t)
	cases := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{name: "in a repository", dir: repo.Dir},
		{name: "outside any repository, which git version does not need", dir: t.TempDir()},
		{name: "a directory that does not exist", dir: filepath.Join(t.TempDir(), "missing"), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Version(context.Background(), tc.dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Version(%s) = %q, want an error", tc.dir, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Version: %v", err)
			}
			if want := runForOutput(t, tc.dir, "version"); string(got) != want {
				t.Fatalf("Version = %q, want git's own %q", got, want)
			}
			if !strings.HasPrefix(string(got), "git version ") || !strings.HasSuffix(string(got), "\n") {
				t.Fatalf("Version = %q, want `git version <v>` and its newline", got)
			}
		})
	}
}

// refListRepo is a repository with every kind of ref RefList must carry:
// two branches, a tag, a remote-tracking branch, and a symbolic ref.
func refListRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := buildRepo(t)
	runFor(t, repo.Dir, "branch", "feature", repo.Heads[0])
	runFor(t, repo.Dir, "tag", "v1")
	runFor(t, repo.Dir, "update-ref", "refs/remotes/origin/main", repo.Head)
	runFor(t, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	return repo
}

func TestRefList(t *testing.T) {
	repo := refListRepo(t)
	got, err := RefList(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("RefList: %v", err)
	}
	want := runForOutput(t, repo.Dir, "for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
	if string(got) != want {
		t.Fatalf("RefList = %q, want git's own %q", got, want)
	}
	lines := strings.Split(strings.TrimSuffix(string(got), "\n"), "\n")
	wantLines := []string{
		"refs/heads/feature\x00" + repo.Heads[0] + "\x00",
		"refs/heads/main\x00" + repo.Head + "\x00",
		"refs/remotes/origin/HEAD\x00" + repo.Head + "\x00refs/remotes/origin/main",
		"refs/remotes/origin/main\x00" + repo.Head + "\x00",
		"refs/tags/v1\x00" + repo.Head + "\x00",
	}
	if strings.Join(lines, "\n") != strings.Join(wantLines, "\n") {
		t.Fatalf("RefList lines =\n%q\nwant\n%q", lines, wantLines)
	}
}

func TestRefList_Negative(t *testing.T) {
	cases := []struct {
		name string
		dir  string
	}{
		{name: "outside any repository", dir: t.TempDir()},
		{name: "a directory that does not exist", dir: filepath.Join(t.TempDir(), "missing")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := RefList(context.Background(), tc.dir); err == nil {
				t.Fatalf("RefList(%s) = %q, want an error", tc.dir, got)
			}
		})
	}
}

func TestConfigList(t *testing.T) {
	isolateGitConfig(t)
	repo := buildRepo(t)
	runFor(t, repo.Dir, "config", "--local", "verdi.probe", "two words")
	got, err := ConfigList(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("ConfigList: %v", err)
	}
	if want := runForOutput(t, repo.Dir, "config", "--list", "-z"); string(got) != want {
		t.Fatalf("ConfigList = %q, want git's own %q", got, want)
	}
	if !strings.Contains(string(got), "\x00verdi.probe\ntwo words\x00") && !strings.HasPrefix(string(got), "verdi.probe\ntwo words\x00") {
		t.Fatalf("ConfigList = %q, want the entry verdi.probe as key, newline, value, NUL", got)
	}
}

func TestConfigList_Negative(t *testing.T) {
	isolateGitConfig(t)
	broken := buildRepo(t)
	corruptConfig(t, broken.Dir)
	cases := []struct {
		name string
		dir  string
	}{
		{name: "an unparseable repository config", dir: broken.Dir},
		{name: "a directory that does not exist", dir: filepath.Join(t.TempDir(), "missing")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ConfigList(context.Background(), tc.dir); err == nil {
				t.Fatalf("ConfigList(%s) = %q, want an error", tc.dir, got)
			}
		})
	}
}

// layoutArgv is RepositoryLayout's argv, run directly for the comparison.
func layoutArgv() []string {
	return []string{
		"rev-parse", "--path-format=absolute",
		"--git-dir", "--git-common-dir",
		"--git-path", "objects", "--git-path", "shallow", "--git-path", "info/grafts",
		"--is-shallow-repository", "HEAD", "--symbolic-full-name", "HEAD",
	}
}

func TestRepositoryLayout(t *testing.T) {
	repo := buildRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, "store", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked")
	runFor(t, repo.Dir, "worktree", "add", "--detach", linked, repo.Heads[0])
	common := realPath(t, filepath.Join(repo.Dir, ".git"))
	cases := []struct {
		name       string
		dir        string
		wantGitDir string
	}{
		{name: "the top level", dir: repo.Dir, wantGitDir: common},
		{name: "a directory below the top level", dir: filepath.Join(repo.Dir, "store", "deep"), wantGitDir: common},
		{name: "a linked worktree, whose git directory is its own", dir: linked, wantGitDir: filepath.Join(common, "worktrees", "linked")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepositoryLayout(context.Background(), tc.dir)
			if err != nil {
				t.Fatalf("RepositoryLayout: %v", err)
			}
			if want := runForOutput(t, tc.dir, layoutArgv()...); string(got.Output) != want {
				t.Fatalf("Output = %q, want git's own %q", got.Output, want)
			}
			lines := strings.Split(strings.TrimSuffix(string(got.Output), "\n"), "\n")
			fields := []string{got.GitDir, got.CommonDir, got.ObjectsDir, got.ShallowFile, got.GraftsFile}
			if strings.Join(fields, "\n") != strings.Join(lines[:5], "\n") {
				t.Fatalf("fields %q are not the first five lines of %q", fields, got.Output)
			}
			checks := []struct{ what, got, want string }{
				{"GitDir", realPath(t, got.GitDir), tc.wantGitDir},
				{"CommonDir", realPath(t, got.CommonDir), common},
				{"ObjectsDir", realPath(t, got.ObjectsDir), filepath.Join(common, "objects")},
				{"ShallowFile", filepath.Join(realPath(t, filepath.Dir(got.ShallowFile)), filepath.Base(got.ShallowFile)), filepath.Join(common, "shallow")},
				{"GraftsFile", filepath.Join(realPath(t, filepath.Dir(got.GraftsFile)), filepath.Base(got.GraftsFile)), filepath.Join(common, "info", "grafts")},
			}
			for _, c := range checks {
				if c.got != c.want {
					t.Errorf("%s = %q, want %q", c.what, c.got, c.want)
				}
			}
		})
	}
}

func TestRepositoryLayout_Negative(t *testing.T) {
	cases := []struct {
		name    string
		dir     func(t *testing.T) string
		wantErr string
	}{
		{name: "outside any repository", dir: func(t *testing.T) string { return t.TempDir() }, wantErr: "RepositoryLayout"},
		{name: "a directory that does not exist", dir: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }, wantErr: "RepositoryLayout"},
		{name: "an unborn HEAD, which has no commit to name", dir: initRepo, wantErr: "RepositoryLayout"},
		{name: "a git directory holding a newline, which the line-oriented answer cannot carry", dir: func(t *testing.T) string {
			odd := filepath.Join(t.TempDir(), "two\nlines")
			if err := os.Mkdir(odd, 0o755); err != nil {
				t.Skipf("this filesystem refuses a newline in a name: %v", err)
			}
			runFor(t, odd, "init", "--quiet")
			runFor(t, odd, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "--allow-empty", "--quiet", "-m", "seed")
			return odd
		}, wantErr: "want 8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.dir(t)
			got, err := RepositoryLayout(context.Background(), dir)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("RepositoryLayout(%q) = %+v, %v; want an error mentioning %q", dir, got, err, tc.wantErr)
			}
		})
	}
}

// realPath resolves p's symbolic links (macOS's /var -> /private/var).
func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return r
}
