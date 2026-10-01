package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// si329Cases are the R2 re-review's probes and mutant killers as cases:
// commit ownership (SI-329 (5′)), attribution per effect ((8′)), the two
// refusal predicates ((3′)), and the sensing additions ((7′)).
func si329Cases() []harnessCase {
	tempDecl := func(mod func(*ws.Declaration)) ws.Declaration {
		d := ws.Declaration{Ritual: "test_temp", Verbs: []ws.Verb{ws.CLI("test temp")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, RefsDelete: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true, Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryNoCommit}
		if mod != nil {
			mod(&d)
		}
		return d
	}
	// withAddedWorktreeCommit adds a @temp worktree at the commit HEAD names
	// and commits in it, then runs then.
	withAddedWorktreeCommit := func(then ...func(context.Context, string) error) func(*testing.T, *Fixture) Driver {
		return func(t *testing.T, _ *Fixture) Driver {
			tmp := filepath.Join(t.TempDir(), "evaluation")
			return InProcess{Fn: steps(append([]func(context.Context, string) error{
				func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") },
				func(ctx context.Context, _ string) error { return writeAndStage(ctx, tmp, "eval.txt", "evaluated\n") },
				func(ctx context.Context, _ string) error { return commitIndex(ctx, tmp) },
			}, then...)...)}
		}
	}
	ownedEval := func(t *testing.T, res Result) []Verdict {
		w := addedWorktree(t, res)
		return []Verdict{
			v("worktrees", Within, "worktree "+w+" added"),
			v("worktrees", Within, "commit "+commitRecording(t, res, "eval.txt")+" made in added worktree "+w),
		}
	}
	orphanSteps := []func(context.Context, string) error{
		func(ctx context.Context, dir string) error {
			base, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return err
			}
			return gitx.CheckoutExisting(ctx, dir, base)
		},
		stage("owned/o.txt", "o\n"), commitIndex, checkout("main"),
	}
	return []harnessCase{
		{
			name: "a main-worktree commit is never credited to a worktree added after it (RR-B1 = A RR7, B P7)", states: both(),
			decl: func() ws.Declaration {
				d := checkedOutDecl()
				d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp}
				return d
			}(),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "after")
				return InProcess{Fn: steps(addPaths(TrackedFile), commitIndex,
					func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") })}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry tracked.txt changed"),
					v("worktrees", Within, "worktree "+addedWorktree(t, res)+" added"),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded tracked.txt"),
				}
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares scoped; observed carried; carried foreign "+ForeignFile)), Fail
				}
				return append(vs, v("index_carry", Within, "declares scoped; observed scoped")), Fail
			},
		},
		{
			name: "a transient main-worktree commit is judged though an added worktree holds a commit (RR-B2, B P1)", states: both(),
			decl: tempDecl(nil),
			driver: withAddedWorktreeCommit(newBranch("ritual/tmp"), stage("owned/t.txt", "t\n"), commitPaths("owned/t.txt"),
				checkout("main"), plain("branch", "-D", "ritual/tmp")),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return append(ownedEval(t, res),
					v("stage_paths", Outside, "commit "+commitRecording(t, res, "owned/t.txt")+" recorded owned/t.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "stash commits recording the operator's work are judged though an added worktree holds a commit (RR-B2, B P1b)", states: both(),
			decl:   tempDecl(nil),
			driver: withAddedWorktreeCommit(plain("stash", "create")),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				wip, index := createdWithParents(t, res, 2), createdWithParents(t, res, 1, commitRecording(t, res, "eval.txt"))
				vs := append(ownedEval(t, res), v("stage_paths", Outside, "commit "+wip+" recorded tracked.txt"))
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares no_commit; observed carried; carried foreign "+ForeignFile)), Fail
				}
				return append(vs,
					v("stage_paths", Unattributable, "commit "+index+" recorded no file"),
					v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "a gitx-only orphaned main-worktree commit is judged though an added worktree holds a commit (RR-B2, B P1d)", states: both(),
			decl:   tempDecl(nil),
			driver: withAddedWorktreeCommit(orphanSteps...),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := append(ownedEval(t, res), v("stage_paths", Outside, "commit "+commitRecording(t, res, "owned/o.txt")+" recorded owned/o.txt"))
				if fx.State == SeedFull {
					return append(vs, orphanedForeign()...), Fail
				}
				return append(vs, v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "a commit an added worktree holds is judged once a fixture ref reaches it (SI-329 (5′)(i))", states: both(),
			decl: tempDecl(nil),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "evaluation")
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") },
					func(ctx context.Context, _ string) error { return writeAndStage(ctx, tmp, "eval.txt", "evaluated\n") },
					func(ctx context.Context, _ string) error { return commitIndex(ctx, tmp) },
					func(ctx context.Context, dir string) error {
						tip, err := gitx.RevParse(ctx, tmp, "HEAD")
						if err != nil {
							return err
						}
						return gitx.UpdateRef(ctx, dir, "refs/heads/ritual/kept", tip)
					})}
			},
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/kept created"),
					v("worktrees", Within, "worktree "+addedWorktree(t, res)+" added"),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded eval.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a failed worktree add does not make its directory an added worktree (RR-A3, A RR5)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.Worktrees = []ws.WorktreePattern{".verdi/data/worktrees/*"} }),
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				x := filepath.Join(dir, ".verdi", "data", "worktrees", "x")
				if err := os.MkdirAll(x, 0o755); err != nil {
					return 2, err
				}
				if err := os.WriteFile(filepath.Join(x, "occupied"), []byte("x\n"), 0o644); err != nil {
					return 2, err
				}
				base, err := gitx.RevParse(ctx, dir, "HEAD")
				if err != nil {
					return 2, err
				}
				// Fails, x being occupied; it names a full commit, so only
				// the missing listing can keep x from counting as added.
				_ = gitx.WorktreeAddDetached(ctx, dir, x, base)
				return steps(func(ctx context.Context, _ string) error {
					base, err := gitx.RevParse(ctx, dir, "HEAD")
					if err != nil {
						return err
					}
					return gitx.CheckoutExisting(ctx, x, base)
				}, stage("owned/o.txt", "o\n"),
					func(ctx context.Context, _ string) error { return commitIndex(ctx, x) },
					func(ctx context.Context, _ string) error { return gitx.CheckoutExisting(ctx, x, "main") })(ctx, dir)
			}),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded owned/o.txt")}
				if fx.State == SeedFull {
					return append(vs, orphanedForeign()...), Fail
				}
				return append(vs, v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "a commit made before a worktree was added at it is never that worktree's (SI-329 (5′), made after the add)", states: both(),
			decl: tempDecl(nil),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "at-orphan")
				var orphan string
				return InProcess{Fn: steps(append(append([]func(context.Context, string) error{}, orphanSteps[:3]...),
					func(ctx context.Context, dir string) (err error) {
						orphan, err = gitx.RevParse(ctx, dir, "HEAD")
						return err
					},
					checkout("main"),
					func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, orphan) })...)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{
					v("worktrees", Within, "worktree "+addedWorktree(t, res)+" added"),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded owned/o.txt"),
				}
				if fx.State == SeedFull {
					return append(vs, orphanedForeign()...), Fail
				}
				return append(vs, v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "a branch move is judged by its net tree difference: a merge whose first parent is not the old tip (B P5)", states: []SeedState{SeedClean},
			decl: checkedOutDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				writeFixtureFile(t, fx.Dir, "side.txt", "side\n")
				runGitFixture(t, ctx, fx.Dir, "checkout", "-q", SideBranch)
				runGitFixture(t, ctx, fx.Dir, "add", "--", "side.txt")
				runGitFixture(t, ctx, fx.Dir, "commit", "-q", "-m", "side", "--", "side.txt")
				runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "main")
				runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "--", TrackedFile)
				runGitFixture(t, ctx, fx.Dir, "clean", "-q", "-f", "--", UntrackedFile)
			},
			driver: inProcess(steps(func(ctx context.Context, dir string) error {
				m, err := plainGit(ctx, dir, "commit-tree", SideBranch+"^{tree}", "-p", SideBranch, "-p", "main", "-m", "merge")
				if err != nil {
					return err
				}
				head, err := gitx.RevParse(ctx, dir, "HEAD")
				if err != nil {
					return err
				}
				if _, err := gitx.CommitTree(ctx, dir, head+"^{tree}", head, "benign"); err != nil {
					return err
				}
				_, err = gitx.FastForwardOnly(ctx, dir, m)
				return err
			})),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				merge, benign := createdWithParents(t, res, 2), createdWithParents(t, res, 1)
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry side.txt added"),
					v("working_tree", Within, "side.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+merge+" recorded no file"),
					v("stage_paths", Within, "commit "+benign+" recorded no file"),
					v("stage_paths", Outside, "refs/heads/main moved onto side.txt, which no created commit recorded"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a push is credited only with its own branch, never a deletion or a rewritten upstream (RR-A1, A RR1)", states: both(),
			decl: tempDecl(func(d *ws.Declaration) { d.Worktrees = nil; d.RefsDelete = nil; d.MayPush = true }),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				runGitFixture(t, ctx, fx.Dir, "push", "--quiet", "origin", SideBranch)
			},
			driver: inProcess(steps(newBranch("ritual/p"), func(ctx context.Context, dir string) error { return gitx.Push(ctx, dir) },
				plain("push", "--quiet", "origin", "--delete", SideBranch),
				plain("push", "--quiet", "origin", "main:refs/heads/elsewhere"),
				plain("config", "branch.ritual/p.merge", "refs/heads/not-what-push-wrote"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p"),
					v("may_push", Unattributable, "refs/remotes/origin/elsewhere created"),
					v("may_push", Within, "refs/remotes/origin/ritual/p created"),
					v("may_push", Unattributable, "refs/remotes/origin/side deleted"),
					v("may_push", Unattributable, "the remote's refs/heads/elsewhere created"),
					v("may_push", Within, "the remote's refs/heads/ritual/p created"),
					v("may_push", Unattributable, "the remote's refs/heads/side deleted"),
					v("config", Unattributable, "config branch.ritual/p.merge set"),
					v("config", Within, "config branch.ritual/p.remote set"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a HEAD switch is credited only to a checkout naming its target (A RR6)", states: both(),
			decl:   noCommitDecl(nil),
			driver: inProcess(steps(checkout(SideBranch), plain("checkout", "--quiet", "--detach", "main"))),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("head_switch", Unattributable, "HEAD switched from refs/heads/main to detached at "+short(fx.BaseCommit)),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "an index entry is credited only to an add whose pathspec matches it (A RR8)", states: both(),
			decl:   checkedOutDecl(),
			driver: inProcess(steps(stage("owned/a.txt", "a\n"), writeFile("owned/b.txt", "b\n"), plain("add", "owned/b.txt"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("index", Within, "index entry owned/a.txt added"),
					v("index", Unattributable, "index entry owned/b.txt added"),
					v("working_tree", Within, "owned/a.txt created"+fileWrite),
					v("working_tree", Within, "owned/b.txt created"+fileWrite),
					v("index_carry", Within, "declares scoped; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a checkout credits only its own tree difference, not an entry staged outside gitx (RR-B5, B P2)", states: both(),
			decl: scopedDecl(),
			driver: inProcess(steps(newBranch("ritual/x"), stage("owned/a.txt", "a\n"),
				writeFile("owned/b.txt", "b\n"), plain("add", "--", "owned/b.txt"), commitPaths("owned/a.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/x created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/x"),
					v("index", Within, "index entry owned/a.txt added"),
					v("index", Unattributable, "index entry owned/b.txt added"),
					v("working_tree", Within, "owned/a.txt created"+fileWrite),
					v("working_tree", Within, "owned/b.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/a.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Unproven
			},
		},
		{
			name: "a scoped ritual refusing after an unreferenced commit fails; it completes in the clean state (RR-B3, B P6)", states: both(),
			decl: scopedDecl(),
			driver: inProcess(steps(func(ctx context.Context, dir string) error {
				head, err := gitx.RevParse(ctx, dir, "HEAD")
				if err != nil {
					return err
				}
				_, err = gitx.CommitTree(ctx, dir, head+"^{tree}", head, "scratch")
				return err
			}, refuseIfStaged)),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded no file")}
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares scoped; observed refused, leaving an unreferenced commit")), Fail
				}
				return append(vs, v("index_carry", Within, "declares scoped; observed scoped")), Pass
			},
		},
		{
			name: "index flag bits are sensed (RR-A4, A RR2)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} }),
			driver: inProcess(steps(plain("update-index", "--assume-unchanged", TrackedFile),
				plain("update-index", "--skip-worktree", OwnedDir+"/keep.txt"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("index", Outside, "index entry owned/keep.txt changed"),
					v("index", Outside, "index entry tracked.txt changed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a linked worktree's own refs are sensed", states: both(),
			decl: noCommitDecl(nil),
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(func(ctx context.Context, _ string) error {
					_, err := plainGit(ctx, fx.Registered, "update-ref", "refs/bisect/bad", "HEAD")
					return err
				})}
			},
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Outside, "worktree "+fx.Registered+": ref refs/bisect/bad created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "the remote's HEAD is sensed", states: both(),
			decl: noCommitDecl(nil),
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(func(ctx context.Context, _ string) error {
					_, err := plainGit(ctx, fx.Bare, "symbolic-ref", "HEAD", "refs/heads/side")
					return err
				})}
			},
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("may_push", Outside, "the remote's HEAD changed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a plain-git branch move to a non-child after a gitx commit is unattributable (MY-M1)", states: both(),
			decl: checkedOutDecl(),
			driver: inProcess(steps(stage("owned/a.txt", "a\n"), commitPaths("owned/a.txt"), func(ctx context.Context, dir string) error {
				d, err := plainGit(ctx, dir, "commit-tree", "-p", "HEAD", "-m", "a grandchild", "HEAD^{tree}")
				if err != nil {
					return err
				}
				_, err = plainGit(ctx, dir, "update-ref", "refs/heads/main", d)
				return err
			})),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Unattributable, "refs/heads/main moved"),
					v("index", Within, "index entry owned/a.txt added"),
					v("working_tree", Within, "owned/a.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+commitRecording(t, res, "owned/a.txt")+" recorded owned/a.txt"),
					v("stage_paths", Within, "commit "+createdWithParentIn(t, res)+" recorded no file"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Unproven
			},
		},
		{
			name: "a gitx call in another repository attributes nothing in the fixture (MY-M2)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: func(t *testing.T, fx *Fixture) Driver {
				other := filepath.Join(t.TempDir(), "other")
				return InProcess{Fn: steps(
					func(ctx context.Context, _ string) error {
						_, err := plainGit(ctx, "", "clone", "--quiet", fx.Bare, other)
						return err
					},
					func(ctx context.Context, _ string) error {
						return gitx.UpdateRef(ctx, other, "refs/heads/ritual/x", fx.BaseCommit)
					},
					plain("update-ref", "refs/heads/ritual/x", fx.BaseCommit))}
			},
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Unattributable, "refs/heads/ritual/x created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "staging content the new tree lacks after a declared switch is outside (RR-M2, B P4)", states: both(),
			decl: noCommitDecl(nil),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				writeFixtureFile(t, fx.Dir, "side.txt", "side\n")
				runGitFixture(t, ctx, fx.Dir, "checkout", "-q", SideBranch)
				runGitFixture(t, ctx, fx.Dir, "add", "--", "side.txt")
				runGitFixture(t, ctx, fx.Dir, "commit", "-q", "-m", "side", "--", "side.txt")
				runGitFixture(t, ctx, fx.Dir, "checkout", "-q", "main")
			},
			driver: inProcess(steps(checkout(SideBranch), writeFile("side.txt", "other\n"), addPaths("side.txt"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/side"),
					v("index", Outside, "index entry side.txt added"),
					v("working_tree", Within, "side.txt created"+fileWrite),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "exit 2 with only a linked worktree remaining is outside (RR-M3, B P3)", states: both(),
			decl: ws.Declaration{Ritual: "test_refuse_worktree", Verbs: []ws.Verb{ws.CLI("test refuse-worktree")},
				Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryRefused},
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "left-behind")
				return InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
					if err := gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD"); err != nil {
						return 2, err
					}
					return 2, os.ErrExist
				}}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				carry := v("index_carry", Within, "declares refused; observed no_commit")
				if fx.State == SeedFull {
					carry.Status = Outside
				}
				return []Verdict{
					v("worktrees", Within, "worktree "+addedWorktree(t, res)+" added"),
					carry,
					v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: linked worktrees"),
				}, Fail
			},
		},
		{
			name: "exit 2 with only config remaining is outside (RR-M7)", states: both(),
			decl: noCommitDecl(nil),
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				if _, err := plainGit(ctx, dir, "config", "x.y", "z"); err != nil {
					return 2, err
				}
				return 2, os.ErrExist
			}),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("config", Outside, "config x.y set"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
					v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: config"),
				}, Fail
			},
		},
		{
			name: "a forced switch discarding the operator's work is outside though the switch is declared (RR-M1)", states: []SeedState{SeedClean},
			decl:   noCommitDecl(nil),
			driver: inProcess(steps(plain("checkout", "--quiet", "-f", SideBranch))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("head_switch", Unattributable, "HEAD switched from refs/heads/main to refs/heads/side"),
					v("working_tree", Outside, "tracked.txt changed: the operator's pre-existing work"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a gone worktree's commit a fixture ref later reaches is judged, not owned (SI-329 (5′)(ii))", states: both(),
			decl: tempDecl(nil),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "evaluation")
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						base, err := gitx.RevParse(ctx, dir, "HEAD")
						if err != nil {
							return err
						}
						return gitx.WorktreeAddDetached(ctx, dir, tmp, base)
					},
					func(ctx context.Context, _ string) error { return writeAndStage(ctx, tmp, "decoy.txt", "decoy\n") },
					func(ctx context.Context, _ string) error { return commitIndex(ctx, tmp) },
					func(ctx context.Context, dir string) error {
						head, err := gitx.RevParse(ctx, tmp, "HEAD")
						if err != nil {
							return err
						}
						if err := gitx.WorktreeRemove(ctx, dir, tmp); err != nil {
							return err
						}
						return gitx.UpdateRef(ctx, dir, "refs/heads/ritual/kept", head)
					},
				)}
			},
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/kept created"),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded decoy.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
	}
}

// ambientConfigCase sets the process environment, so it runs before the
// parallel cases.
func ambientConfigCase() harnessCase {
	return harnessCase{
		name: "an ambient global excludesFile cannot hide the operator's untracked work from the sensors (RR-A5, A RR3)",
		decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} }),
		setup: func(t *testing.T, _ context.Context, _ *Fixture) {
			dir := t.TempDir()
			excludes := filepath.Join(dir, "ignore")
			if err := os.WriteFile(excludes, []byte(UntrackedFile+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			global := filepath.Join(dir, "gitconfig")
			if err := os.WriteFile(global, []byte("[core]\n\texcludesFile = "+excludes+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			xdg := filepath.Join(dir, "xdg")
			if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(xdg, "git", "ignore"), []byte(UntrackedFile+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GIT_CONFIG_GLOBAL", global)
			t.Setenv("XDG_CONFIG_HOME", xdg)
		},
		driver: inProcess(steps(removeFile(UntrackedFile))),
		want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
			return []Verdict{
				v("working_tree", Outside, "untracked.txt deleted: the operator's pre-existing work"),
				v("index_carry", Within, "declares no_commit; observed no_commit"),
			}, Fail
		},
	}
}

// commitRecording returns the short id of the one created commit whose
// file list holds path.
func commitRecording(t *testing.T, res Result, path string) string {
	t.Helper()
	var found []string
	for id, c := range res.After.Commits {
		if _, old := res.Before.Commits[id]; old {
			continue
		}
		for _, f := range c.Files {
			if f == path {
				found = append(found, short(id))
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one created commit recording %s, got %v", path, found)
	}
	return found[0]
}

// createdWithParents returns the short id of the one created commit with
// n parents, other than the commits except names by short id.
func createdWithParents(t *testing.T, res Result, n int, except ...string) string {
	t.Helper()
	skip := map[string]bool{}
	for _, e := range except {
		skip[e] = true
	}
	var found []string
	for id, c := range res.After.Commits {
		if _, old := res.Before.Commits[id]; !old && len(c.Parents) == n && !skip[short(id)] {
			found = append(found, short(id))
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one created commit with %d parents, got %v", n, found)
	}
	return found[0]
}

// createdWithParentIn returns the short id of the one created commit whose
// parent was also created in the run.
func createdWithParentIn(t *testing.T, res Result) string {
	t.Helper()
	var found []string
	for id, c := range res.After.Commits {
		if _, old := res.Before.Commits[id]; old || len(c.Parents) == 0 {
			continue
		}
		if _, parentOld := res.Before.Commits[c.Parents[0]]; !parentOld {
			found = append(found, short(id))
		}
	}
	if len(found) != 1 {
		t.Fatalf("want one created commit with a created parent, got %v", found)
	}
	return found[0]
}
