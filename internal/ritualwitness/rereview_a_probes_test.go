package ritualwitness

// Re-reviewer R2-RR-A probes, copied from the lane R2 re-review (half A,
// zz_rr_a_probe_test.go) unchanged but for this header: every assertion
// is the re-reviewer's own. Each logs its verdicts and outcome; t.Errorf
// marks a WRONG result (a probe "fires" when the harness is wrong).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

func rrLog(t *testing.T, res Result) {
	t.Helper()
	t.Logf("exit=%d outcome=%s verdicts:\n%s", res.Exit, Outcome(res.Verdicts), formatVerdicts(res.Verdicts))
}

// RR1: a logged gitx.Push (which pushes HEAD only) is credited with a
// remote branch created and another deleted by plain git outside gitx.
func TestZZRR_1_PushCreditedWithUnrelatedRemoteRefs(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "rr1", Verbs: []ws.Verb{ws.CLI("rr one")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit, MayPush: true}
	for _, state := range both() {
		t.Run(state.String(), func(t *testing.T) {
			fx := Build(t, ctx, state)
			runGitFixture(t, ctx, fx.Dir, "push", "--quiet", "origin", SideBranch)
			res := RunOn(t, ctx, fx, InProcess{Fn: steps(
				newBranch("ritual/p"),
				func(ctx context.Context, dir string) error { return gitx.Push(ctx, dir) },
				// outside gitx: no observer sees these
				plain("push", "--quiet", "origin", "--delete", SideBranch),
				plain("push", "--quiet", "origin", "main:refs/heads/elsewhere"),
				plain("config", "branch.ritual/p.merge", "refs/heads/not-what-push-wrote"),
			)}, decl)
			rrLog(t, res)
			for _, d := range []string{"the remote's refs/heads/side deleted", "refs/remotes/origin/side deleted",
				"the remote's refs/heads/elsewhere created", "refs/remotes/origin/elsewhere created"} {
				if zzHas(res.Verdicts, "may_push", Within, d) {
					t.Errorf("FIRES: %q made by plain git outside gitx is within, credited to gitx.Push (push --set-upstream origin HEAD), which pushes refs/heads/ritual/p only", d)
				}
			}
			if zzHas(res.Verdicts, "config", Within, "branch.ritual/p.merge") {
				t.Logf("NOTE: branch.ritual/p.merge rewritten by plain git to a value no push writes reads within (value never checked)")
			}
		})
	}
}

// RR2: index flag bits (assume-unchanged, skip-worktree) are index
// mutations; ls-files -s does not show them.
func TestZZRR_2_IndexFlagBitsUnsensed(t *testing.T) {
	ctx := context.Background()
	decl := noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} })
	for _, state := range both() {
		t.Run(state.String(), func(t *testing.T) {
			res := Run(t, ctx, InProcess{Fn: steps(
				plain("update-index", "--assume-unchanged", TrackedFile),
				plain("update-index", "--skip-worktree", OwnedDir+"/keep.txt"),
			)}, decl, state)
			rrLog(t, res)
			out, _ := plainGit(ctx, res.After.Root, "ls-files", "-v")
			t.Logf("ls-files -v after:\n%s", out)
			st, _ := plainGit(ctx, res.After.Root, "status", "--porcelain")
			t.Logf("status after (the operator's dirty tracked.txt is now hidden):\n%s", st)
			if Outcome(res.Verdicts) == Pass {
				t.Errorf("FIRES: the operator's dirty %s was hidden from status by an index flag change made outside gitx, and the run passes: neither sensed nor disclosed (SI-325 (9) does not list it)", TrackedFile)
			}
		})
	}
}

// RR3: a globally ignored path (ambient core.excludesFile) is not sensed,
// so deleting the operator's untracked file passes.
func TestZZRR_3_AmbientExcludesFileHidesUntrackedWork(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	excludes := filepath.Join(dir, "ignore")
	if err := os.WriteFile(excludes, []byte(UntrackedFile+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(dir, "gitconfig")
	if err := os.WriteFile(global, []byte("[core]\n\texcludesFile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	decl := noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} })
	ritual := InProcess{Fn: steps(removeFile(UntrackedFile))}
	hermetic := Run(t, ctx, ritual, decl, SeedClean)
	t.Logf("hermetic:")
	rrLog(t, hermetic)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	ambient := Run(t, ctx, ritual, decl, SeedClean)
	t.Logf("with a global excludesFile naming %s:", UntrackedFile)
	rrLog(t, ambient)
	if Outcome(hermetic.Verdicts) != Outcome(ambient.Verdicts) {
		t.Errorf("FIRES: the same ritual is %s hermetically and %s under an ambient global excludesFile", Outcome(hermetic.Verdicts), Outcome(ambient.Verdicts))
	}
}

// RR4: ambient merge.autoStash makes gitx.FastForwardOnly create stash
// commit objects over the seeded dirty work.
func TestZZRR_4_AmbientAutoStashChangesFastForwardVerdict(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "rr4", Verbs: []ws.Verb{ws.CLI("rr four")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, IndexCarry: ws.CarryNoCommit}
	run := func(t *testing.T, state SeedState) Result {
		fx := Build(t, ctx, state)
		c := trimNL(runGitFixture(t, ctx, fx.Dir, "commit-tree", "-p", "HEAD", "-m", "runway target", "HEAD^{tree}"))
		return RunOn(t, ctx, fx, InProcess{Fn: steps(func(ctx context.Context, dir string) error {
			_, err := gitx.FastForwardOnly(ctx, dir, c)
			return err
		})}, decl)
	}
	for _, state := range both() {
		t.Run(state.String(), func(t *testing.T) {
			hermetic := run(t, state)
			t.Logf("hermetic:")
			rrLog(t, hermetic)
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "merge.autoStash")
			t.Setenv("GIT_CONFIG_VALUE_0", "true")
			ambient := run(t, state)
			t.Logf("with ambient merge.autoStash=true:")
			rrLog(t, ambient)
			if Outcome(hermetic.Verdicts) != Outcome(ambient.Verdicts) {
				t.Errorf("FIRES: gitx.FastForwardOnly is %s hermetically and %s under ambient merge.autoStash", Outcome(hermetic.Verdicts), Outcome(ambient.Verdicts))
			}
		})
	}
}

// RR5: a FAILED logged `worktree add` makes its path an "added worktree",
// so an orphaned commit made through gitx with that (in-checkout) path as
// its directory is credited to it and escapes no_commit.
func TestZZRR_5_FailedWorktreeAddOwnsOrphanedCommit(t *testing.T) {
	ctx := context.Background()
	decl := noCommitDecl(func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{".verdi/data/worktrees/*"} })
	mk := func(withFailedAdd bool) Ritual {
		return func(ctx context.Context, dir string) (int, error) {
			x := filepath.Join(dir, ".verdi", "data", "worktrees", "x")
			if err := os.MkdirAll(x, 0o755); err != nil {
				return 2, err
			}
			if err := os.WriteFile(filepath.Join(x, "occupied"), []byte("x\n"), 0o644); err != nil {
				return 2, err
			}
			if withFailedAdd {
				_ = gitx.WorktreeAddDetached(ctx, dir, x, "HEAD") // fails: x exists and is not empty
			}
			base, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return 2, err
			}
			if err := gitx.CheckoutExisting(ctx, x, base); err != nil { // detaches the MAIN worktree
				return 2, err
			}
			if err := writeAndStage(ctx, dir, "owned/o.txt", "o\n"); err != nil {
				return 2, err
			}
			if _, err := gitx.CreateCommit(ctx, x, "commit on the main worktree's detached HEAD"); err != nil {
				return 2, err
			}
			if err := gitx.CheckoutExisting(ctx, x, "main"); err != nil {
				return 2, err
			}
			return 0, nil
		}
	}
	control := Run(t, ctx, InProcess{Fn: mk(false)}, decl, SeedClean)
	t.Logf("control (no failed add):")
	rrLog(t, control)
	probe := Run(t, ctx, InProcess{Fn: mk(true)}, decl, SeedClean)
	t.Logf("probe (a failed worktree add logged first):")
	rrLog(t, probe)
	t.Logf("log: %v", probe.Log.Calls)
	if Outcome(control.Verdicts) == Fail && Outcome(probe.Verdicts) == Pass {
		t.Errorf("FIRES: a commit object under no_commit passes because a failed, logged `worktree add` named the directory it was made from")
	}
}

// RR6: a logged checkout naming one branch is credited with a HEAD switch
// plain git made to somewhere else.
func TestZZRR_6_CheckoutCreditedWithAnyHeadSwitch(t *testing.T) {
	ctx := context.Background()
	decl := noCommitDecl(func(d *ws.Declaration) {})
	res := Run(t, ctx, InProcess{Fn: steps(checkout(SideBranch), plain("checkout", "--quiet", "--detach", "main"))}, decl, SeedClean)
	rrLog(t, res)
	if zzHas(res.Verdicts, "head_switch", Within, "detached") {
		t.Logf("NOTE: HEAD ended detached (plain git); the only logged checkout named refs/heads/side, yet the switch is within")
	}
}

// RR7 (cross-half, SI-325 (5)): a scoped commit made in the MAIN worktree
// that carries the foreign entry escapes judgement once the ritual adds a
// declared worktree whose HEAD reaches it.
func TestZZRR_7_MainWorktreeCommitCreditedToLaterAddedWorktree(t *testing.T) {
	ctx := context.Background()
	decl := withScoped(func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} })
	mk := func(addAfter bool) func(t *testing.T) Ritual {
		return func(t *testing.T) Ritual {
			tmp := filepath.Join(t.TempDir(), "wt")
			return func(ctx context.Context, dir string) (int, error) {
				code, err := ritualCarriesForeign(ctx, dir)
				if err != nil || !addAfter {
					return code, err
				}
				return 0, gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD")
			}
		}
	}
	control := Run(t, ctx, InProcess{Fn: mk(false)(t)}, decl, SeedFull)
	t.Logf("control (no worktree added):")
	rrLog(t, control)
	probe := Run(t, ctx, InProcess{Fn: mk(true)(t)}, decl, SeedFull)
	t.Logf("probe (a declared @temp worktree added at HEAD afterwards):")
	rrLog(t, probe)
	if Outcome(control.Verdicts) == Fail && Outcome(probe.Verdicts) == Pass {
		t.Errorf("FIRES: a scoped commit made in the main worktree carrying %s passes once an added worktree's HEAD reaches it", ForeignFile)
	}
}

// RR8: a logged `add -- owned/a.txt` is credited with an index entry
// plain git staged at another path.
func TestZZRR_8_AddCreditedWithAnyIndexEntry(t *testing.T) {
	ctx := context.Background()
	res := Run(t, ctx, InProcess{Fn: steps(stage("owned/a.txt", "a\n"), writeFile("owned/b.txt", "b\n"), plain("add", "owned/b.txt"))},
		checkedOutDecl(), SeedClean)
	rrLog(t, res)
	if zzHas(res.Verdicts, "index", Within, "owned/b.txt") {
		t.Logf("NOTE: index entry owned/b.txt staged by plain git reads within, credited to `add -- owned/a.txt`")
	}
}
