package scenario

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Repo is a built scenario store.
type Repo struct {
	// Dir is the repository's working directory, with the scenario's
	// Checkout branch checked out and refs/remotes/origin/<initial branch>
	// at that branch, so specstate.ResolveDefaultBranch resolves it with
	// no remote and no network.
	Dir string
	// Base holds the base steps' commits, in order. Base[0] is the store's
	// root commit, which every committed frozen stamp names.
	Base []string
	// Steps holds each step's commit, in step order.
	Steps []string
}

// Build builds the named scenario from the committed fixture (Dir) into a
// fresh repository and fails t on any error.
func Build(t testing.TB, name string) *Repo {
	t.Helper()
	repo, err := Materialize(context.Background(), Dir(), t.TempDir(), name)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

// Materialize builds scenario name from the fixture at fixtureDir into the
// empty directory repoDir, following the package doc's replay rules, and
// needs no testing.TB: it is the non-test materializer a harness calls.
// Every commit input comes from the manifest, and every git invocation
// carries its own config isolation (lane L3 review a M-2, re-review a m-3),
// so no ambient identity, date, or setting can move a SHA.
func Materialize(ctx context.Context, fixtureDir, repoDir, name string) (*Repo, error) {
	m, err := Load(fixtureDir)
	if err != nil {
		return nil, err
	}
	sc, ok := m.Scenarios[name]
	if !ok {
		return nil, fmt.Errorf("scenario: %q is not defined", name)
	}
	g := gitIn{ctx: ctx, dir: repoDir, id: m.Commit}
	if _, err := g.run(nil, "init", "--quiet", "--object-format=sha1", "--initial-branch="+m.Commit.InitialBranch); err != nil {
		return nil, err
	}
	// The repository's own config keeps later test commits deterministic
	// and free of signing and background maintenance, as fixturegit's do.
	for _, kv := range [][2]string{{"user.name", m.Commit.Name}, {"user.email", m.Commit.Email},
		{"commit.gpgsign", "false"}, {"merge.log", "false"}, {"gc.autoDetach", "false"}, {"maintenance.auto", "false"}} {
		if _, err := g.run(nil, "config", kv[0], kv[1]); err != nil {
			return nil, err
		}
	}
	repo := &Repo{Dir: repoDir}
	for _, st := range m.Base {
		c, err := g.step(m, fixtureDir, st)
		if err != nil {
			return nil, err
		}
		repo.Base = append(repo.Base, c)
	}
	for _, st := range sc.Steps {
		c, err := g.step(m, fixtureDir, st)
		if err != nil {
			return nil, err
		}
		repo.Steps = append(repo.Steps, c)
	}
	if _, err := g.run(nil, "update-ref", "refs/remotes/origin/"+m.Commit.InitialBranch, m.Commit.InitialBranch); err != nil {
		return nil, err
	}
	if _, err := g.run(nil, "checkout", "-q", sc.Checkout); err != nil {
		return nil, err
	}
	return repo, nil
}

// gitIn runs git in one scenario repository.
type gitIn struct {
	ctx context.Context
	dir string
	id  Identity
}

// step applies one step on its branch and returns the branch's commit
// after it: the step's commit, or for a fast-forward the commit reached,
// and for a rebase the last replayed commit.
func (g gitIn) step(m *Manifest, fixtureDir string, st Step) (string, error) {
	if head, _ := g.run(nil, "symbolic-ref", "--short", "HEAD"); head != st.Branch {
		args := []string{"checkout", "-q", st.Branch}
		if exists, _ := g.run(nil, "branch", "--list", st.Branch); exists == "" {
			args = []string{"checkout", "-q", "-b", st.Branch}
		}
		if _, err := g.run(nil, args...); err != nil {
			return "", err
		}
	}
	var err error
	switch {
	case st.Merge != "":
		_, err = g.run(&st, "merge", "-q", "--no-ff", "--no-verify", "-m", st.Message, st.Merge)
	case st.FastForward != "":
		_, err = g.run(nil, "merge", "-q", "--ff-only", st.FastForward)
	case st.Rebase != "":
		_, err = g.run(&st, "rebase", "-q", st.Rebase)
	case len(st.Moves) > 0:
		for _, mv := range st.Moves {
			if err = os.MkdirAll(filepath.Join(g.dir, filepath.Dir(filepath.FromSlash(mv.To))), 0o755); err != nil {
				return "", err
			}
			if _, err = g.run(nil, "mv", mv.From, mv.To); err != nil {
				return "", err
			}
		}
		_, err = g.run(&st, "commit", "-q", "--no-verify", "-m", st.Message)
	default:
		files, ferr := m.Files(fixtureDir, st.Layers...)
		if ferr != nil {
			return "", ferr
		}
		for _, p := range sortedKeys(files) {
			full := filepath.Join(g.dir, filepath.FromSlash(p))
			if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			if err = os.WriteFile(full, []byte(files[p]), 0o644); err != nil {
				return "", err
			}
		}
		if _, err = g.run(nil, "add", "-A"); err != nil {
			return "", err
		}
		_, err = g.run(&st, "commit", "-q", "--no-verify", "-m", st.Message)
	}
	if err != nil {
		return "", err
	}
	return g.run(nil, "rev-parse", "HEAD")
}

// isolation is the config every invocation carries, over no global or
// system config: settings a user's config could otherwise use to move a
// SHA (a merge message's shortlog, a commit encoding header, line-ending
// conversion, a message-rewriting hook, a signature).
var isolation = []string{"-c", "merge.log=false", "-c", "i18n.commitEncoding=UTF-8", "-c", "core.autocrlf=false",
	"-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgsign=false"}

// run runs git with isolation, every inherited GIT_* variable removed,
// global and system config off, TZ=UTC, and the manifest's identity; a
// commit step adds its dates. It returns trimmed stdout.
func (g gitIn) run(st *Step, args ...string) (string, error) {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") && !strings.HasPrefix(kv, "TZ=") {
			env = append(env, kv)
		}
	}
	env = append(env, "TZ=UTC", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME="+g.id.Name, "GIT_AUTHOR_EMAIL="+g.id.Email,
		"GIT_COMMITTER_NAME="+g.id.Name, "GIT_COMMITTER_EMAIL="+g.id.Email)
	if st != nil {
		author := st.AuthorDate
		if author == "" {
			author = st.Date
		}
		for key, date := range map[string]string{"GIT_AUTHOR_DATE": author, "GIT_COMMITTER_DATE": st.Date} {
			when, err := stepTime(date)
			if err != nil {
				return "", err
			}
			env = append(env, fmt.Sprintf("%s=%d +0000", key, when.Unix()))
		}
	}
	cmd := exec.CommandContext(g.ctx, "git", append(append([]string{}, isolation...), args...)...)
	cmd.Dir, cmd.Env = g.dir, env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("scenario: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
