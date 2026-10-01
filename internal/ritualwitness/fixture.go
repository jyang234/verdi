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

// SeedState is one of the two seeded states spec/ritual-effect-witness ac-1
// and parent ac-2 require, so a ritual that refuses a foreign index entry
// is also observed running to completion (parent ac-2's own reasoning: the
// full state alone is vacuous for a refusing ritual, since it exits before
// any later out-of-scope staging could ever be observed).
type SeedState int

const (
	// SeedFull has an untracked file, a pre-staged unrelated index entry
	// (the "foreign entry" the index-carry assertions are about), and a
	// dirty tracked file.
	SeedFull SeedState = iota
	// SeedClean has the same untracked and dirty tracked work as SeedFull,
	// but a clean index: no foreign entry to carry, refuse, or scope out.
	SeedClean
)

// String renders the state's name for test output.
func (s SeedState) String() string {
	if s == SeedClean {
		return "clean"
	}
	return "full"
}

// The fixture's fixed file names, exported so a ritual function (this
// package's own synthetic ones, or a future real-ritual driver) can name
// exactly what it owns and what belongs to the operator.
const (
	// TrackedFile is seeded tracked, then locally modified without being
	// staged: the dirty tracked file every seeded state carries.
	TrackedFile = "tracked.txt"
	// UntrackedFile is a new, never-added file: the untracked file every
	// seeded state carries.
	UntrackedFile = "untracked.txt"
	// ForeignFile is the pre-staged unrelated index entry SeedFull carries
	// and SeedClean does not — a colleague's unrelated staged work no
	// ritual may claim as its own.
	ForeignFile = "foreign-staged.txt"
	// OwnedDir is a directory already tracked at the fixture's base commit,
	// standing in for a ritual's own declared stage path.
	OwnedDir = "owned"
	// SideBranch is a second local branch, seeded pointing at the same
	// commit as main and never checked out by Build, so a synthetic ritual
	// can switch to it without disturbing any tracked file (both branches
	// name the identical tree).
	SideBranch = "side"
)

// Fixture is a deterministic fixturegit repository seeded for one ritual
// run.
type Fixture struct {
	// Dir is the working checkout a ritual runs against.
	Dir string
	// Bare is the local bare remote's directory, configured as origin with
	// main pushed and refs/remotes/origin/HEAD resolvable — hermetic (co-3):
	// a local path, no network.
	Bare string
	// BaseCommit is the fixture's one seed commit, before any SeedState
	// work is written on top.
	BaseCommit string
	// State is the seeded state Build was asked for.
	State SeedState
}

// Build returns a fresh Fixture seeded to state: a fixturegit repository
// with one commit (TrackedFile, OwnedDir/keep.txt), a second local branch
// (SideBranch) at the same commit, a local bare remote as origin with a
// resolvable default branch, and state's own working-tree/index layer
// written on top. Every git invocation here is local-path-only; nothing
// touches the network.
func Build(t testing.TB, ctx context.Context, state SeedState) *Fixture {
	t.Helper()

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			TrackedFile:            "original content\n",
			OwnedDir + "/keep.txt": "kept\n",
		},
		Message: "seed",
	}})

	runGitFixture(t, ctx, repo.Dir, "branch", SideBranch)

	bare := t.TempDir()
	runGitFixture(t, ctx, "", "init", "--bare", "--quiet", "--initial-branch=main", bare)
	runGitFixture(t, ctx, repo.Dir, "remote", "add", "origin", bare)
	runGitFixture(t, ctx, repo.Dir, "push", "--quiet", "origin", "main")
	runGitFixture(t, ctx, repo.Dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	seedWorkingState(t, ctx, repo.Dir, state)

	return &Fixture{Dir: repo.Dir, Bare: bare, BaseCommit: repo.Head, State: state}
}

// seedWorkingState writes state's operator work on top of the fixture's
// clean base commit: a locally modified (unstaged) TrackedFile and a new
// UntrackedFile in every state, plus a pre-staged ForeignFile in SeedFull
// only.
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
// creating parent directories as needed, failing the test on error.
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

// runGitFixture runs `git <args...>` for fixture setup only — deliberately
// never through gitx's own run(), so a fixture never depends on the very
// seam the harness measures (mirrors internal/gitx's own test helper,
// gitConfigure). dir == "" runs with no working directory override, for a
// command (like `git init --bare <path>`) that names its target as an
// argument rather than needing cmd.Dir set.
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
