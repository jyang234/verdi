package ritualwitness

// Re-reviewer probes (R2 re-review half B), copied from the lane R2
// re-review (zz_rr_b_probe_test.go) and adapted only by renaming its log
// helper rrLog to rrbLog, which half A's copy also declares: every
// assertion is the re-reviewer's own. Each asserts the behaviour SI-325
// requires; a FAIL demonstrates a defect.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func rrNoCommitTempDecl() ws.Declaration {
	return ws.Declaration{
		Ritual: "rr_probe", Verbs: []ws.Verb{ws.CLI("rr probe")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, RefsDelete: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryNoCommit,
	}
}

func rrAddedWorktreeCommit(tmp string) []func(context.Context, string) error {
	return []func(context.Context, string) error{
		func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") },
		func(ctx context.Context, _ string) error { return writeAndStage(ctx, tmp, "eval.txt", "evaluated\n") },
		func(ctx context.Context, _ string) error { return commitIndex(ctx, tmp) },
	}
}

func rrbLog(t *testing.T, res Result) {
	t.Helper()
	t.Logf("exit=%d outcome=%s created=%v\n%s", res.Exit, Outcome(res.Verdicts), createdCommits(res), formatVerdicts(res.Verdicts))
}

// RR-P1: B P6's transient main-worktree commit under no_commit, preceded
// by a commit in a worktree the ritual added. SI-325 (5): the transient
// commit is not reachable from the added worktree's HEAD and the log shows
// it made in the main worktree, so no_commit fails on it.
func TestRRProbe_P1_TransientMainCommitWithAddedWorktree(t *testing.T) {
	ctx := context.Background()
	for _, st := range []SeedState{SeedFull, SeedClean} {
		tmp := filepath.Join(t.TempDir(), "evaluation")
		fns := append(rrAddedWorktreeCommit(tmp),
			newBranch("ritual/tmp"), stage("owned/t.txt", "t\n"), commitPaths("owned/t.txt"),
			checkout("main"), plain("branch", "-D", "ritual/tmp"))
		res := Run(t, ctx, InProcess{Fn: steps(fns...)}, rrNoCommitTempDecl(), st)
		rrbLog(t, res)
		if Outcome(res.Verdicts) != Fail {
			t.Errorf("%s: DEFECT (false pass): a transient commit made in the main worktree escaped no_commit", st)
		}
	}
}

// RR-P1b: same, but the unreferenced commit is `git stash create` in the
// main worktree (plain git), which records the operator's dirty tracked
// file. SI-325 (5): judged against the stage paths; no_commit fails.
func TestRRProbe_P1b_StashCreateWithAddedWorktree(t *testing.T) {
	ctx := context.Background()
	for _, st := range []SeedState{SeedFull, SeedClean} {
		tmp := filepath.Join(t.TempDir(), "evaluation")
		fns := append(rrAddedWorktreeCommit(tmp), plain("stash", "create"))
		res := Run(t, ctx, InProcess{Fn: steps(fns...)}, rrNoCommitTempDecl(), st)
		rrbLog(t, res)
		if Outcome(res.Verdicts) != Fail {
			t.Errorf("%s: DEFECT (false pass): stash commits recording the operator's dirty work escaped no_commit", st)
		}
	}
}

// RR-P1c: control without the added worktree: the same stash create.
func TestRRProbe_P1c_StashCreateAlone(t *testing.T) {
	ctx := context.Background()
	res := Run(t, ctx, InProcess{Fn: steps(plain("stash", "create"))}, rrNoCommitTempDecl(), SeedClean)
	rrbLog(t, res)
	if Outcome(res.Verdicts) != Fail {
		t.Errorf("control: stash commits not failed")
	}
}

// RR-P2: an index entry inside a declared stage path staged by plain git
// (outside gitx), alongside a logged gitx add of a different path. SI-325
// (8): attributed only to a logged mutating call, paths canonicalized; the
// logged add names owned/a.txt, not owned/b.txt.
func TestRRProbe_P2_IndexEntryStagedOutsideGitx(t *testing.T) {
	ctx := context.Background()
	res := Run(t, ctx, InProcess{Fn: steps(newBranch("ritual/x"), stage("owned/a.txt", "a\n"),
		writeFile("owned/b.txt", "b\n"), plain("add", "--", "owned/b.txt"), commitPaths("owned/a.txt"))}, scopedDecl(), SeedClean)
	rrbLog(t, res)
	for _, vd := range res.Verdicts {
		if vd.Field == "index" && vd.Detail == "index entry owned/b.txt added" && vd.Status == Within {
			t.Errorf("DEFECT? plain-git index entry credited to a gitx add of another path: %s", vd)
		}
	}
}

// RR-P1d: gitx only. A P3's orphaned commit (detach, stage, commit,
// switch back) under no_commit, after a commit in a worktree the ritual
// added and still holds. SI-325 (5): the orphan is judged; no_commit fails.
func TestRRProbe_P1d_GitxOnlyOrphanWithAddedWorktree(t *testing.T) {
	ctx := context.Background()
	orphan := []func(context.Context, string) error{
		func(ctx context.Context, dir string) error {
			base, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return err
			}
			return gitx.CheckoutExisting(ctx, dir, base)
		},
		stage("owned/o.txt", "o\n"), commitIndex, checkout("main"),
	}
	for _, st := range []SeedState{SeedFull, SeedClean} {
		tmp := filepath.Join(t.TempDir(), "evaluation")
		res := Run(t, ctx, InProcess{Fn: steps(append(rrAddedWorktreeCommit(tmp), orphan...)...)}, rrNoCommitTempDecl(), st)
		rrbLog(t, res)
		if Outcome(res.Verdicts) != Fail {
			t.Errorf("%s: DEFECT (false pass): a gitx-only orphaned main-worktree commit escaped no_commit", st)
		}
	}
}

// RR-P3: SI-325 (3) with only a linked worktree remaining: a refused
// declaration that may add a @temp worktree adds one, then exits 2.
func TestRRProbe_P3_RefusalWithWorktreeRemaining(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "rr_refuse_wt", Verbs: []ws.Verb{ws.CLI("rr refuse-wt")},
		Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryRefused}
	tmp := filepath.Join(t.TempDir(), "left-behind")
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if err := gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD"); err != nil {
			return 2, err
		}
		return 2, nil
	}}, decl, SeedFull)
	rrbLog(t, res)
	if Outcome(res.Verdicts) != Fail {
		t.Errorf("DEFECT: exit 2 with a linked worktree remaining passed")
	}
}

// RR-P4: SI-325 (1)'s "the index now holds exactly the new tree's entry":
// after a declared switch to a branch whose tree differs at side.txt, the
// ritual stages different content there (outside every stage path).
func TestRRProbe_P4_IndexDiffersFromNewTreeAfterSwitch(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	writeFixtureFile(t, fx.Dir, "side.txt", "side\n")
	runGitFixture(t, ctx, fx.Dir, "checkout", "-q", SideBranch)
	runGitFixture(t, ctx, fx.Dir, "add", "--", "side.txt")
	runGitFixture(t, ctx, fx.Dir, "commit", "-q", "-m", "side", "--", "side.txt")
	runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "main")
	decl := ws.Declaration{Ritual: "rr_switch", Verbs: []ws.Verb{ws.CLI("rr switch")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	res := RunOn(t, ctx, fx, InProcess{Fn: steps(checkout(SideBranch), writeFile("side.txt", "other\n"), addPaths("side.txt"))}, decl)
	rrbLog(t, res)
	if Outcome(res.Verdicts) != Fail {
		t.Errorf("DEFECT: staging content the new tree lacks, outside every stage path, passed")
	}
}

// RR-P5: reading "tree differences come from the ritual's own commits,
// each judged under (5)". A created merge whose first parent is not the
// old tip (side first, main second; side's tree), fast-forwarded onto
// @checked-out: side.txt enters main, yet no judged first-parent file list
// records it. Logs a benign gitx commit-tree so commits are attributed.
func TestRRProbe_P5_MergeFirstParentNotOldTip(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	writeFixtureFile(t, fx.Dir, "side.txt", "side\n")
	runGitFixture(t, ctx, fx.Dir, "checkout", "-q", SideBranch)
	runGitFixture(t, ctx, fx.Dir, "add", "--", "side.txt")
	runGitFixture(t, ctx, fx.Dir, "commit", "-q", "-m", "side", "--", "side.txt")
	runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "main")
	runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "--", TrackedFile)
	runGitFixture(t, ctx, fx.Dir, "clean", "-q", "-f", "--", UntrackedFile)
	res := RunOn(t, ctx, fx, InProcess{Fn: steps(func(ctx context.Context, dir string) error {
		m, err := plainGit(ctx, dir, "commit-tree", SideBranch+"^{tree}", "-p", SideBranch, "-p", "main", "-m", "merge")
		if err != nil {
			return err
		}
		tree, err := gitx.RevParse(ctx, dir, "HEAD^{tree}")
		if err != nil {
			return err
		}
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return err
		}
		if _, err := gitx.CommitTree(ctx, dir, tree, head, "benign"); err != nil {
			return err
		}
		_, err = gitx.FastForwardOnly(ctx, dir, m)
		return err
	})}, checkedOutDecl())
	rrbLog(t, res)
	if Outcome(res.Verdicts) == Pass {
		t.Errorf("OBSERVATION: side.txt entered @checked-out with no judged file list recording it (Outcome pass)")
	}
}

// RR-P6: reading "a refusal also requires that no commit object was
// created" against parent dc-6 (an unreferenced object is no mutation): a
// scoped declaration whose ritual writes an unreferenced commit object
// (gitx.CommitTree, as a scratch-index ritual does) and then refuses the
// foreign entry with exit 2, nothing remaining.
func TestRRProbe_P6_ScopedRefusesAfterUnreferencedCommit(t *testing.T) {
	ctx := context.Background()
	res := Run(t, ctx, InProcess{Fn: steps(func(ctx context.Context, dir string) error {
		tree, err := gitx.RevParse(ctx, dir, "HEAD^{tree}")
		if err != nil {
			return err
		}
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return err
		}
		_, err = gitx.CommitTree(ctx, dir, tree, head, "scratch")
		return err
	}, refuseIfStaged)}, scopedDecl(), SeedFull)
	rrbLog(t, res)
	if Outcome(res.Verdicts) != Fail {
		t.Errorf("DEFECT? a scoped declaration refusing the foreign entry (exit 2, nothing remaining) passed")
	}
}

// RR-P7: gitx only. commit_to_design's shape plus a declared @temp
// worktree: the ritual commits the operator's dirty tracked file (and in
// the full state the whole index, foreign entry included) on the
// checked-out branch, then adds a worktree at HEAD. The obligation: a
// ritual that stages outside its declared paths, or carries a foreign
// entry while declaring scoped, fails naming the effect.
func TestRRProbe_P7_SweepThenAddWorktreeAtHead(t *testing.T) {
	ctx := context.Background()
	decl := checkedOutDecl()
	decl.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp}
	for _, st := range []SeedState{SeedFull, SeedClean} {
		tmp := filepath.Join(t.TempDir(), "after")
		res := Run(t, ctx, InProcess{Fn: steps(addPaths(TrackedFile), commitIndex,
			func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") })}, decl, st)
		rrbLog(t, res)
		if Outcome(res.Verdicts) != Fail {
			t.Errorf("%s: DEFECT (false pass): the operator's work swept into a commit on @checked-out passed", st)
		}
	}
}
