package ritualwitness

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// SeedState is one of the two seeded states spec/ritual-effect-witness
// ac-1 and parent ac-2 require, so a ritual that refuses a foreign index
// entry is also observed running to completion.
type SeedState int

const (
	// SeedFull has an untracked file, a pre-staged unrelated index entry
	// (the foreign entry the index-carry assertions are about), and a
	// dirty tracked file.
	SeedFull SeedState = iota
	// SeedClean has the same untracked and dirty tracked work as SeedFull,
	// and a clean index.
	SeedClean
)

// String renders the state's name for test output.
func (s SeedState) String() string {
	if s == SeedClean {
		return "clean"
	}
	return "full"
}

// The fixture's fixed names, exported so a ritual function can name
// exactly what it owns and what belongs to the operator.
const (
	// TrackedFile is seeded tracked, then locally modified without being
	// staged: the dirty tracked file every seeded state carries.
	TrackedFile = "tracked.txt"
	// UntrackedFile is a new, never-added file every seeded state carries.
	UntrackedFile = "untracked.txt"
	// ForeignFile is the pre-staged unrelated index entry SeedFull carries
	// and SeedClean does not.
	ForeignFile = "foreign-staged.txt"
	// OwnedDir is a directory tracked at the fixture's base commit,
	// standing in for a ritual's own declared stage path.
	OwnedDir = "owned"
	// SideBranch is a second local branch at the base commit, never
	// checked out, so a ritual can switch to it without disturbing any
	// tracked file.
	SideBranch = "side"
	// IgnoredDir is ignored by the fixture's committed .gitignore, as a
	// real store ignores .verdi/data/: linked worktrees inside it never
	// appear as untracked work.
	IgnoredDir = ".verdi/data/"
	// RegisteredWorktree is the store-relative path of the linked
	// worktree every fixture registers before a ritual runs, detached at
	// the base commit, so @registered and changes inside a pre-existing
	// worktree are exercised.
	RegisteredWorktree = ".verdi/data/worktrees/registered"
)

// Fixture is a deterministic fixturegit repository seeded for one ritual
// run.
type Fixture struct {
	// Dir is the working checkout (the store root) a ritual runs against.
	Dir string
	// Bare is the local bare remote, configured as origin with main
	// pushed and refs/remotes/origin/HEAD resolvable (no network).
	Bare string
	// BaseCommit is the fixture's one seed commit.
	BaseCommit string
	// Registered is the canonical path of the linked worktree registered
	// before the ritual runs.
	Registered string
	// State is the seeded state Build was asked for.
	State SeedState
	// Acting is the checkout the ritual acts on when it is not Dir: a
	// linked worktree registered before the run, such as a /b/ route's
	// managed worktree, whose branch @checked-out then names (the
	// write-scope grammar's "the checkout the ritual acts on"; ledger
	// SI-348 (4)). "" is Dir. A test sets it after Build, before RunOn.
	Acting string
}

// Build returns a fresh Fixture seeded to state: a fixturegit repository
// with one commit (TrackedFile, OwnedDir/keep.txt, and a .gitignore
// ignoring IgnoredDir), SideBranch at the same commit, a linked worktree
// registered at RegisteredWorktree, a local bare remote as origin with a
// resolvable default branch, and state's own working-tree and index layer.
// Every git invocation here is local-path-only, and runs with ambient git
// configuration isolated (isolateGitConfig).
func Build(t testing.TB, ctx context.Context, state SeedState) *Fixture {
	t.Helper()
	return BuildWith(t, ctx, state, nil)
}

// BuildWith is Build with base's files added to the one seed commit: a
// real ritual runs against a verdi store, whose files must be tracked at
// the commit the default branch resolves to before state's working-tree
// and index layer is laid over them. base maps repository-relative slash
// paths to content; a path that collides with the fixture's own
// (baseCollision), or that the seed commit does not track (a base
// .gitignore can ignore one), fails the test.
func BuildWith(t testing.TB, ctx context.Context, state SeedState, base map[string]string) *Fixture {
	t.Helper()
	if err := baseCollision(base); err != nil {
		t.Fatalf("ritualwitness: BuildWith: %v", err)
	}
	isolateGitConfig(t)

	files := map[string]string{
		".gitignore":           IgnoredDir + "\n",
		TrackedFile:            "original content\n",
		OwnedDir + "/keep.txt": "kept\n",
	}
	for path, content := range base {
		files[path] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "seed"}})
	tracked := make(map[string]bool)
	for _, path := range strings.Split(runGitFixture(t, ctx, repo.Dir, "ls-tree", "-r", "-z", "--name-only", repo.Head), "\x00") {
		tracked[path] = true
	}
	var untracked []string
	for path := range base {
		if !tracked[path] {
			untracked = append(untracked, path)
		}
	}
	if len(untracked) > 0 {
		sort.Strings(untracked)
		t.Fatalf("ritualwitness: BuildWith: the seed commit does not track base file(s) %s", strings.Join(untracked, ", "))
	}

	runGitFixture(t, ctx, repo.Dir, "branch", SideBranch)
	runGitFixture(t, ctx, repo.Dir, "worktree", "add", "--quiet", "--detach", RegisteredWorktree, repo.Head)

	bare := t.TempDir()
	runGitFixture(t, ctx, "", "init", "--bare", "--quiet", "--initial-branch=main", bare)
	runGitFixture(t, ctx, repo.Dir, "remote", "add", "origin", bare)
	runGitFixture(t, ctx, repo.Dir, "push", "--quiet", "origin", "main")
	runGitFixture(t, ctx, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	seedWorkingState(t, ctx, repo.Dir, state)

	return &Fixture{
		Dir:        repo.Dir,
		Bare:       bare,
		BaseCommit: repo.Head,
		Registered: canonicalPath(repo.Dir, RegisteredWorktree),
		State:      state,
	}
}

// baseCollision reports a base file BuildWith cannot seed: one of the
// fixture's own seed files (.gitignore, TrackedFile, OwnedDir/keep.txt),
// one of the files the seeded states lay over the base (UntrackedFile,
// ForeignFile), or a path under IgnoredDir, which the seed's .gitignore
// keeps out of every commit.
func baseCollision(base map[string]string) error {
	own := map[string]bool{".gitignore": true, TrackedFile: true, OwnedDir + "/keep.txt": true, UntrackedFile: true, ForeignFile: true}
	for path := range base {
		switch {
		case own[path]:
			return fmt.Errorf("base file %s is one the fixture seeds itself", path)
		case strings.HasPrefix(path, IgnoredDir):
			return fmt.Errorf("base file %s lies under %s, which the fixture ignores", path, IgnoredDir)
		}
	}
	return nil
}

// isolateGitConfig makes every git process the test starts — the
// fixture's, the ritual's, and the sensors' — ignore ambient global and
// system configuration (SI-329 (7′)), so an operator's core.excludesFile
// or hooks cannot change a fixture or a classification. A process already
// isolated by IsolateGitConfig (this package's TestMain) keeps its
// parallel tests; otherwise the test is isolated with t.Setenv, and so
// cannot run in parallel.
func isolateGitConfig(t testing.TB) {
	t.Helper()
	if gitConfigIsolated() {
		return
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

// gitConfigIsolated reports whether the process already ignores ambient
// global and system git configuration: GIT_CONFIG_GLOBAL is the empty
// file, system configuration is off, and XDG_CONFIG_HOME holds no git
// directory.
func gitConfigIsolated() bool {
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if os.Getenv("GIT_CONFIG_GLOBAL") != os.DevNull || os.Getenv("GIT_CONFIG_NOSYSTEM") != "1" || xdg == "" {
		return false
	}
	_, err := os.Lstat(filepath.Join(xdg, "git"))
	return errors.Is(err, fs.ErrNotExist)
}

// IsolateGitConfig isolates the whole test process from ambient global
// and system git configuration, for a TestMain whose tests use Build in
// parallel. It returns a function restoring the previous environment.
func IsolateGitConfig() (restore func(), err error) {
	xdg, err := os.MkdirTemp("", "ritualwitness-xdg-")
	if err != nil {
		return nil, fmt.Errorf("ritualwitness: IsolateGitConfig: %w", err)
	}
	keys := []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM", "XDG_CONFIG_HOME"}
	values := []string{os.DevNull, "1", xdg}
	previous := make([]*string, len(keys))
	for i, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			previous[i] = &v
		}
		if err := os.Setenv(k, values[i]); err != nil {
			return nil, fmt.Errorf("ritualwitness: IsolateGitConfig: %w", err)
		}
	}
	return func() {
		for i, k := range keys {
			if previous[i] == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *previous[i])
			}
		}
		_ = os.RemoveAll(xdg)
	}, nil
}

// seedWorkingState writes state's operator work on top of the clean base
// commit: a locally modified TrackedFile and a new UntrackedFile in every
// state, plus a pre-staged ForeignFile in SeedFull only.
func seedWorkingState(t testing.TB, ctx context.Context, dir string, state SeedState) {
	t.Helper()
	writeFixtureFile(t, dir, TrackedFile, "original content\nedited by the operator\n")
	writeFixtureFile(t, dir, UntrackedFile, "the operator's new, never-added file\n")
	if state == SeedFull {
		writeFixtureFile(t, dir, ForeignFile, "a colleague's unrelated staged work\n")
		runGitFixture(t, ctx, dir, "add", "--", ForeignFile)
	}
}

// writeFixtureFile writes content to dir/path (forward-slash relative),
// creating parent directories, failing the test on error.
func writeFixtureFile(t testing.TB, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("ritualwitness: mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("ritualwitness: write %s: %v", path, err)
	}
}

// runGitFixture runs `git <args...>` for fixture setup only — never
// through gitx, so a fixture never depends on the seam the harness
// measures. dir == "" runs with no working-directory override.
func runGitFixture(t testing.TB, ctx context.Context, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ritualwitness: git %s (dir %q): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return string(out)
}
