package ritualwitness

// Closure-check probes (R2 half B), copied from the lane R2 closure check
// (zz_cc_b_probe_test.go) unchanged but for this header and gofmt's import
// order: every assertion is the reviewer's own. Each asserts the behaviour
// SI-329 requires; a FAIL demonstrates a defect.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func ccLog(t *testing.T, res Result) {
	t.Helper()
	t.Logf("exit=%d outcome=%s created=%v\n%s", res.Exit, Outcome(res.Verdicts), createdCommits(res), formatVerdicts(res.Verdicts))
}

// CC-P8: the hand-back exception carrying the operator's pre-staged entry.
// The agent commits, in its added worktree, a file equal to the foreign
// entry; a logged gitx fast-forward fails (dirty runway) and a plain
// `merge --ff-only` succeeds (the disclosed (8) residual credits it).
func TestCCProbe_P8_HandbackCarriesForeignViaResidual(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "cc_handback", Verbs: []ws.Verb{ws.CLI("cc handback")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	fx := Build(t, ctx, SeedFull)
	unit := filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit")
	res := RunOn(t, ctx, fx, InProcess{Fn: steps(
		func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, unit, "HEAD") },
		func(_ context.Context, dir string) error {
			b, err := os.ReadFile(filepath.Join(dir, ForeignFile))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(unit, ForeignFile), b, 0o644)
		},
		func(ctx context.Context, _ string) error {
			_, err := plainGit(ctx, unit, "add", ForeignFile)
			return err
		},
		func(ctx context.Context, _ string) error {
			_, err := plainGit(ctx, unit, "commit", "--quiet", "-m", "agent work")
			return err
		},
		func(ctx context.Context, dir string) error {
			o, err := gitx.RevParse(ctx, unit, "HEAD")
			if err != nil {
				return err
			}
			_, _ = gitx.FastForwardOnly(ctx, dir, o) // logged, refuses the dirty runway
			_, err = plainGit(ctx, dir, "merge", "--ff-only", "--quiet", o)
			return err
		},
	)}, decl)
	ccLog(t, res)
	if Outcome(res.Verdicts) == Pass {
		t.Errorf("RESIDUAL: the operator's pre-staged entry reached @checked-out and the run passed")
	}
}

// CC-P9: the same carry with a logged, succeeding gitx fast-forward: the
// ritual hides the operator's work with net-zero plain-git steps so the
// runway reads clean, fast-forwards, and restores it.
func TestCCProbe_P9_HandbackCarriesForeignNetZero(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "cc_handback", Verbs: []ws.Verb{ws.CLI("cc handback")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	fx := Build(t, ctx, SeedFull)
	unit := filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit")
	aside := t.TempDir()
	mv := func(from, to string) error { return os.Rename(from, to) }
	res := RunOn(t, ctx, fx, InProcess{Fn: steps(
		func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, unit, "HEAD") },
		func(_ context.Context, dir string) error {
			b, err := os.ReadFile(filepath.Join(dir, ForeignFile))
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(unit, ForeignFile), b, 0o644)
		},
		func(ctx context.Context, _ string) error {
			_, err := plainGit(ctx, unit, "add", ForeignFile)
			return err
		},
		func(ctx context.Context, _ string) error {
			_, err := plainGit(ctx, unit, "commit", "--quiet", "-m", "agent work")
			return err
		},
		plain("rm", "--cached", "-q", "--", ForeignFile),
		plain("update-index", "--assume-unchanged", TrackedFile),
		func(_ context.Context, dir string) error {
			return mv(filepath.Join(dir, ForeignFile), filepath.Join(aside, "f"))
		},
		func(_ context.Context, dir string) error {
			return mv(filepath.Join(dir, UntrackedFile), filepath.Join(aside, "u"))
		},
		func(ctx context.Context, dir string) error {
			o, err := gitx.RevParse(ctx, unit, "HEAD")
			if err != nil {
				return err
			}
			_, err = gitx.FastForwardOnly(ctx, dir, o)
			return err
		},
		func(_ context.Context, dir string) error {
			return mv(filepath.Join(aside, "u"), filepath.Join(dir, UntrackedFile))
		},
		plain("update-index", "--no-assume-unchanged", TrackedFile),
	)}, decl)
	ccLog(t, res)
	if Outcome(res.Verdicts) == Pass {
		t.Errorf("RESIDUAL: the operator's pre-staged entry reached @checked-out and the run passed")
	}
}

// CC-P10: SI-329 (5')(ii)'s exception for a gone worktree. The ritual adds
// a worktree at the base commit, commits there through gitx, removes it;
// then builds a commit in the main checkout with plain git (a scratch
// index, so nothing logged in the fixture) recording outside.txt, and
// fast-forwards @checked-out to it through gitx. The gone worktree never
// reached that commit, so (ii)'s exception ("a commit that worktree
// reaches") cannot be shown: it should be judged (no_commit fails).
func TestCCProbe_P10_GoneWorktreeExceptionOwnsMainCommit(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "cc_gone", Verbs: []ws.Verb{ws.CLI("cc gone")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	fx := Build(t, ctx, SeedClean)
	runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--", TrackedFile)
	if err := os.Remove(filepath.Join(fx.Dir, UntrackedFile)); err != nil {
		t.Fatal(err)
	}
	unit := filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit")
	scratch := filepath.Join(t.TempDir(), "index")
	git := func(ctx context.Context, dir string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+scratch)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	res := RunOn(t, ctx, fx, InProcess{Fn: steps(
		func(ctx context.Context, dir string) error {
			base, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return err
			}
			return gitx.WorktreeAddDetached(ctx, dir, unit, base)
		},
		func(ctx context.Context, _ string) error { return writeAndStage(ctx, unit, "unit.txt", "agent\n") },
		func(ctx context.Context, _ string) error { return commitIndex(ctx, unit) },
		func(ctx context.Context, dir string) error { return gitx.WorktreeRemove(ctx, dir, unit) },
		func(ctx context.Context, dir string) error {
			if _, err := git(ctx, dir, "read-tree", "HEAD"); err != nil {
				return err
			}
			blob, err := plainGit(ctx, dir, "hash-object", "-w", "--stdin")
			_ = blob
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(t.TempDir(), "x"), nil, 0o644); err != nil {
				return err
			}
			b, err := git(ctx, dir, "hash-object", "-w", filepath.Join(dir, "owned", "keep.txt"))
			if err != nil {
				return err
			}
			if _, err := git(ctx, dir, "update-index", "--add", "--cacheinfo", "100644,"+b+",outside.txt"); err != nil {
				return err
			}
			tree, err := git(ctx, dir, "write-tree")
			if err != nil {
				return err
			}
			x, err := git(ctx, dir, "commit-tree", tree, "-p", "HEAD", "-m", "made in main")
			if err != nil {
				return err
			}
			_, err = gitx.FastForwardOnly(ctx, dir, x)
			return err
		},
	)}, decl)
	ccLog(t, res)
	if Outcome(res.Verdicts) == Pass {
		t.Errorf("DEFECT: a commit made in the main checkout was owned by a gone worktree and passed")
	}
}
