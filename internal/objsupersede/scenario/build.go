package scenario

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// Repo is a built scenario store.
type Repo struct {
	// Dir is the repository's working directory, with the scenario's
	// Checkout branch checked out and refs/remotes/origin/main at main, so
	// specstate.ResolveDefaultBranch resolves main with no remote and no
	// network.
	Dir string
	// Base holds the base layers' commits on main, in order. Base[0] is
	// the store's root commit, which every committed frozen stamp names.
	Base []string
	// Steps holds each step's commit, in step order.
	Steps []string
}

// Build builds the named scenario from the committed fixture (Dir) into a
// fresh repository and fails t on any error. Identical inputs yield
// identical commits: fixturegit's fixed identity, each step's own date,
// and TZ=UTC.
func Build(t testing.TB, name string) *Repo {
	t.Helper()
	dir := Dir()
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := m.Scenarios[name]
	if !ok {
		t.Fatalf("scenario: %q is not defined", name)
	}
	layers := make([]fixturegit.Layer, 0, len(m.Base))
	for _, b := range m.Base {
		files, err := m.Files(dir, b)
		if err != nil {
			t.Fatal(err)
		}
		layers = append(layers, fixturegit.Layer{Files: files, Message: "Base layer " + b})
	}
	fg := fixturegit.Build(t, layers)
	repo := &Repo{Dir: fg.Dir, Base: fg.Heads}
	for _, st := range sc.Steps {
		repo.Steps = append(repo.Steps, applyStep(t, m, dir, repo.Dir, st))
	}
	git(t, repo.Dir, nil, "update-ref", "refs/remotes/origin/main", "main")
	git(t, repo.Dir, nil, "checkout", "-q", sc.Checkout)
	return repo
}

// applyStep commits one step and returns its commit.
func applyStep(t testing.TB, m *Manifest, dir, repoDir string, st Step) string {
	t.Helper()
	if git(t, repoDir, nil, "branch", "--list", st.Branch) == "" {
		git(t, repoDir, nil, "checkout", "-q", "-b", st.Branch)
	} else {
		git(t, repoDir, nil, "checkout", "-q", st.Branch)
	}
	when, err := stepTime(st.Date)
	if err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprintf("%d +0000", when.Unix())
	env := []string{"TZ=UTC", "GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp}
	if st.Merge != "" {
		git(t, repoDir, env, "merge", "-q", "--no-ff", "--no-verify", "-m", st.Message, st.Merge)
	} else {
		files, err := m.Files(dir, st.Layers...)
		if err != nil {
			t.Fatal(err)
		}
		paths := make([]string, 0, len(files))
		for p := range files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			full := filepath.Join(repoDir, filepath.FromSlash(p))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		git(t, repoDir, nil, "add", "-A")
		git(t, repoDir, env, "commit", "-q", "--no-verify", "-m", st.Message)
	}
	return git(t, repoDir, nil, "rev-parse", "HEAD")
}

// git runs git in dir with env added, failing t on a non-zero exit, and
// returns its trimmed output.
func git(t testing.TB, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("scenario: git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
