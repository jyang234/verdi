package ritualwitness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
}

// Build returns a fresh Fixture seeded to state: a fixturegit repository
// with one commit (TrackedFile, OwnedDir/keep.txt, and a .gitignore
// ignoring IgnoredDir), SideBranch at the same commit, a linked worktree
// registered at RegisteredWorktree, a local bare remote as origin with a
// resolvable default branch, and state's own working-tree and index layer.
// Every git invocation here is local-path-only.
func Build(t testing.TB, ctx context.Context, state SeedState) *Fixture {
	t.Helper()

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".gitignore":           IgnoredDir + "\n",
			TrackedFile:            "original content\n",
			OwnedDir + "/keep.txt": "kept\n",
		},
		Message: "seed",
	}})

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
