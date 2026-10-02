package ritualwitness

// Reviewer probes (R2 half B), copied from the lane R2 review
// (zz_review_probe_test.go, sha1 0d745bc8) and adapted only to the
// harness's changed API: Evaluate takes no store root and returns an
// error, and the package-level declaration variable became the function
// reviewScopedDecl (ground rules: no package-level mutable state; its old
// name is the harness test's own). Every behavioral assertion is kept.
// Each asserts the CORRECT behaviour the binding authority requires; a
// FAIL demonstrates the defect.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func rawGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return errWithOutput(err, out)
	}
	return nil
}

func requireNotAllWithin(t *testing.T, vs []Verdict) {
	t.Helper()
	for _, v := range vs {
		if v.Status != Within {
			return
		}
	}
	t.Errorf("DEFECT: every verdict is within (silent pass):\n%s", formatVerdicts(vs))
}

func reviewScopedDecl() ws.Declaration {
	return ws.Declaration{
		Ritual:     "probe_scoped",
		Verbs:      []ws.Verb{ws.CLI("probe scoped")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true,
		StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
}

// P1: stages the operator's dirty tracked file (outside stage_paths) but
// commits only its own path; the stray staged entry stays in the index.
func TestProbe_P1_StagesOutsideWithoutCommittingIt(t *testing.T) {
	ctx := context.Background()
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p1"); err != nil {
			return 2, err
		}
		if err := writeAndStage(ctx, dir, "owned/p1.txt", "p1\n"); err != nil {
			return 2, err
		}
		if err := gitx.AddPaths(ctx, dir, TrackedFile); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommitPaths(ctx, dir, "p1", "owned/p1.txt"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	for _, st := range []SeedState{SeedFull, SeedClean} {
		res := Run(t, ctx, InProcess{Fn: fn}, reviewScopedDecl(), st)
		t.Logf("%s:\n%s", st, formatVerdicts(res.Verdicts))
		requireNotAllWithin(t, res.Verdicts)
	}
}

// P2: discards the operator's dirty tracked edit and deletes the untracked
// file; declared no_commit with a head switch.
func TestProbe_P2_DiscardsOperatorWork(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "probe_discard", Verbs: []ws.Verb{ws.CLI("probe discard")},
		HeadSwitch: true, IndexCarry: ws.CarryNoCommit,
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := rawGit(ctx, dir, "checkout", "--", TrackedFile); err != nil {
			return 2, err
		}
		if err := os.Remove(filepath.Join(dir, UntrackedFile)); err != nil {
			return 2, err
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	requireNotAllWithin(t, res.Verdicts)
}

// P2b: unstages the operator's pre-staged foreign entry (index effect),
// declared no_commit.
func TestProbe_P2b_UnstagesForeignEntry(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "probe_unstage", Verbs: []ws.Verb{ws.CLI("probe unstage")},
		HeadSwitch: true, IndexCarry: ws.CarryNoCommit,
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := rawGit(ctx, dir, "rm", "--cached", "-q", "--", ForeignFile); err != nil {
			return 2, err
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, decl, SeedFull)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	requireNotAllWithin(t, res.Verdicts)
}

// P3: declared scoped, mutates (creates+switches a branch) and then refuses
// with exit 2 over the foreign entry.
func TestProbe_P3_ScopedRefusesAfterMutation(t *testing.T) {
	ctx := context.Background()
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p3"); err != nil {
			return 2, err
		}
		staged, err := gitx.StagedPaths(ctx, dir)
		if err != nil {
			return 2, err
		}
		if len(staged) > 0 {
			return 2, os.ErrExist
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, reviewScopedDecl(), SeedFull)
	t.Logf("exit=%d verdicts:\n%s", res.Exit, formatVerdicts(res.Verdicts))
	requireNotAllWithin(t, res.Verdicts)
}

// P4: commit_to_design's shape: RefsMove @checked-out, no head switch; a
// plain scoped commit on the checked-out branch.
func TestProbe_P4_CommitOnCheckedOutIsNotAHeadSwitch(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "probe_ctd", Verbs: []ws.Verb{ws.CLI("probe ctd")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
	if err := decl.Validate(); err != nil {
		t.Fatal(err)
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := writeAndStage(ctx, dir, "owned/p4.txt", "p4\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommitPaths(ctx, dir, "p4", "owned/p4.txt"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	for _, v := range res.Verdicts {
		if v.Status != Within {
			t.Errorf("DEFECT (false outside): %s", v)
		}
	}
}

// P5: deletes a pre-existing remote branch with no may_push declared.
func TestProbe_P5_RemoteRefDeletionIsSilent(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "probe_rdel", Verbs: []ws.Verb{ws.CLI("probe rdel")},
		HeadSwitch: true, IndexCarry: ws.CarryNoCommit,
	}
	fx := Build(t, ctx, SeedClean)
	runGitFixture(t, ctx, fx.Dir, "push", "--quiet", "origin", SideBranch)
	before, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatal(err)
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := rawGit(ctx, dir, "push", "--quiet", "origin", "--delete", SideBranch); err != nil {
			return 2, err
		}
		return 0, nil
	}
	exit, log, derr := InProcess{Fn: fn}.Run(ctx, fx.Dir)
	if derr != nil {
		t.Fatal(derr)
	}
	after, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := Evaluate(decl, exit, before, after, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before remote=%v after remote=%v verdicts:\n%s", before.RemoteRefs, after.RemoteRefs, formatVerdicts(vs))
	requireNotAllWithin(t, vs)
}

// P6: declared no_commit; makes a commit on a transient branch, switches
// back, deletes the branch (net: no ref, HEAD, or reachable-commit delta).
func TestProbe_P6_TransientCommitUnderNoCommit(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "probe_transient", Verbs: []ws.Verb{ws.CLI("probe transient")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, RefsDelete: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, IndexCarry: ws.CarryNoCommit,
	}
	if err := decl.Validate(); err != nil {
		t.Fatal(err)
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/tmp"); err != nil {
			return 2, err
		}
		if err := writeAndStage(ctx, dir, "owned/t.txt", "t\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommitPaths(ctx, dir, "transient", "owned/t.txt"); err != nil {
			return 2, err
		}
		if err := gitx.CheckoutExisting(ctx, dir, "main"); err != nil {
			return 2, err
		}
		if err := rawGit(ctx, dir, "branch", "-D", "ritual/tmp"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, decl, SeedClean)
	t.Logf("log=%v\nverdicts:\n%s", res.Log.Calls, formatVerdicts(res.Verdicts))
	requireNotAllWithin(t, res.Verdicts)
}

// P7: the operator pre-staged a file INSIDE the declared stage path; a
// scoped commit of the declared directory records it.
func TestProbe_P7_PreStagedInsideDeclaredPath(t *testing.T) {
	ctx := context.Background()
	decl := reviewScopedDecl()
	decl.StagePaths = []ws.PathPattern{"owned/"}
	fx := Build(t, ctx, SeedClean)
	writeFixtureFile(t, fx.Dir, "owned/pre.txt", "operator\n")
	runGitFixture(t, ctx, fx.Dir, "add", "--", "owned/pre.txt")
	before, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatal(err)
	}
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p7"); err != nil {
			return 2, err
		}
		if err := writeAndStage(ctx, dir, "owned/mine.txt", "mine\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommitPaths(ctx, dir, "p7", "owned/"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	exit, log, derr := InProcess{Fn: fn}.Run(ctx, fx.Dir)
	if derr != nil {
		t.Fatal(derr)
	}
	after, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := Evaluate(decl, exit, before, after, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("verdicts:\n%s", formatVerdicts(vs))
	for _, v := range vs {
		if v.Status != Within {
			t.Errorf("DEFECT (false outside): %s", v)
		}
	}
}

// P8: declared refused (close's shape); in the full state the ritual
// re-stages new content over the operator's foreign entry, then exits 2.
func TestProbe_P8_RefusedAfterRewritingForeignStagedEntry(t *testing.T) {
	ctx := context.Background()
	decl := reviewScopedDecl()
	decl.IndexCarry = ws.CarryRefused
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := writeAndStage(ctx, dir, ForeignFile, "clobbered\n"); err != nil {
			return 2, err
		}
		return 2, os.ErrExist
	}
	res := Run(t, ctx, InProcess{Fn: fn}, decl, SeedFull)
	t.Logf("exit=%d verdicts:\n%s", res.Exit, formatVerdicts(res.Verdicts))
	requireNotAllWithin(t, res.Verdicts)
}

// P9: a commit under a declared directory with a non-ASCII file name.
func TestProbe_P9_NonASCIIPathUnderDeclaredDir(t *testing.T) {
	ctx := context.Background()
	fn := func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p9"); err != nil {
			return 2, err
		}
		if err := writeAndStage(ctx, dir, "owned/caf\u00e9.txt", "x\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommitPaths(ctx, dir, "p9", "owned/caf\u00e9.txt"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	res := Run(t, ctx, InProcess{Fn: fn}, reviewScopedDecl(), SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	for _, v := range res.Verdicts {
		if v.Status != Within {
			t.Errorf("DEFECT (false outside): %s", v)
		}
	}
}

type noLogDriver struct{ fn Ritual }

func (d noLogDriver) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	exit, err := d.fn(ctx, dir)
	return exit, CommandLog{}, err
}

// P10: the scoped ritual through a driver with no command log.
func TestProbe_P10_UnavailableLog(t *testing.T) {
	ctx := context.Background()
	res := Run(t, ctx, noLogDriver{fn: ritualScoped}, reviewScopedDecl(), SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	requireVerdict(t, res.Verdicts, "refs_create", Unattributable, "ritual/scoped-ok")
}

// P11: the lane's own "moves an undeclared ref" case, printing every verdict.
func TestProbe_P11_LaneMovesUndeclaredRefVerdicts(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{
		Ritual: "test_moves_undeclared", Verbs: []ws.Verb{ws.CLI("test moves-undeclared")},
		RefsMove: []ws.RefPattern{"refs/heads/decoy"}, StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
	res := Run(t, ctx, InProcess{Fn: ritualMovesUndeclaredRef}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
}
