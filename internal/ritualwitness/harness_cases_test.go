package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// refCases cover SI-325 (7) and (8) for refs: deletions, every namespace,
// remote refs and their mirrors, and attribution by canonical refname.
func refCases() []harnessCase {
	deleteSide := func(ctx context.Context, dir string) error { return gitx.DeleteBranch(ctx, dir, SideBranch) }
	return []harnessCase{
		{
			name: "deleting a declared ref is within", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsDelete = []ws.RefPattern{"refs/heads/side"} }),
			driver: inProcess(steps(deleteSide)),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_delete", Within, "refs/heads/side deleted"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "deleting an undeclared ref is outside", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsDelete = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(steps(deleteSide)),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_delete", Outside, "refs/heads/side deleted"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a declared ref deleted outside gitx is unattributable", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsDelete = []ws.RefPattern{"refs/heads/side"} }),
			driver: inProcess(steps(plain("branch", "-D", SideBranch))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_delete", Unattributable, "refs/heads/side deleted"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a declared ref moved outside gitx is unattributable", states: both(),
			decl: ws.Declaration{Ritual: "test_move_side", Verbs: []ws.Verb{ws.CLI("test move-side")},
				RefsMove: []ws.RefPattern{"refs/heads/side"}, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped},
			driver: inProcess(steps(func(ctx context.Context, dir string) error {
				c, err := plainGit(ctx, dir, "commit-tree", "-p", SideBranch, "-m", "outside gitx", "HEAD^{tree}")
				if err != nil {
					return err
				}
				_, err = plainGit(ctx, dir, "update-ref", "refs/heads/side", c)
				return err
			})),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Unattributable, "refs/heads/side moved"),
					v("stage_paths", Unattributable, "commit "+onlyCommit(t, res)+" recorded no file"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Unproven
			},
		},
		{
			name: "a read-only gitx call naming a ref made outside gitx attributes nothing (A P9)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				if code, err := ritualBypassesGitx(ctx, dir); err != nil {
					return code, err
				}
				_, err := gitx.RevParse(ctx, dir, "ritual/sneaky")
				return 0, err
			}),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Unattributable, "refs/heads/ritual/sneaky created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a full refname in gitx's argv attributes its own create (A P10)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsCreate = []ws.RefPattern{"refs/heads/design/*"} }),
			driver: inProcess(steps(func(ctx context.Context, dir string) error {
				head, err := gitx.RevParse(ctx, dir, "HEAD")
				if err != nil {
					return err
				}
				return gitx.UpdateRef(ctx, dir, "refs/heads/design/x", head)
			})),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/design/x created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "a tag named like the checked-out branch does not confuse @checked-out (A P11)", decl: checkedOutDecl(), states: both(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				runGitFixture(t, ctx, fx.Dir, "tag", "main")
			},
			driver: inProcess(ritualMovesUndeclaredRef),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry owned/x.txt added"),
					v("working_tree", Within, "owned/x.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/x.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Pass
			},
		},
		{
			name: "a tag is outside even where refs/heads/* is declared", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsCreate = []ws.RefPattern{"refs/heads/*"} }),
			driver: inProcess(steps(plain("tag", "v1"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Outside, "refs/tags/v1 created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a remote branch deleted without may-push is outside, with its mirror (A P16, B P5)", states: both(),
			decl: noCommitDecl(nil),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				runGitFixture(t, ctx, fx.Dir, "push", "--quiet", "origin", SideBranch)
			},
			driver: inProcess(steps(plain("push", "--quiet", "origin", "--delete", SideBranch))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("may_push", Outside, "refs/remotes/origin/side deleted"),
					v("may_push", Outside, "the remote's refs/heads/side deleted"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a may-push push outside gitx is unattributable, its mirror too", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.MayPush = true }),
			driver: inProcess(steps(plain("push", "--quiet", "origin", "HEAD:refs/heads/ritual/raw"))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("may_push", Unattributable, "refs/remotes/origin/ritual/raw created"),
					v("may_push", Unattributable, "the remote's refs/heads/ritual/raw created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a remote-tracking ref that does not mirror the remote is outside even under may-push", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.MayPush = true }),
			// gitx creates only branches, so the ref is seeded through
			// fixturegit (ledger SI-359 (5a)). A remote-tracking ref that
			// mirrors nothing is outside whether or not a logged call names
			// it, so the verdict does not depend on which seeds it.
			driver: func(t *testing.T, _ *Fixture) Driver {
				return InProcess{Fn: steps(func(ctx context.Context, dir string) error {
					head, err := gitx.RevParse(ctx, dir, "HEAD")
					if err != nil {
						return err
					}
					fixturegit.CreateRef(t, dir, "refs/remotes/origin/sneaky", head)
					return nil
				})}
			},
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("may_push", Outside, "refs/remotes/origin/sneaky created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
	}
}

// worktreeCases cover linked worktrees: adds and removes against each
// pattern kind, canonical paths, and changes inside a pre-existing one.
func worktreeCases() []harnessCase {
	addAt := func(rel string) func(context.Context, string) error {
		return func(ctx context.Context, dir string) error {
			return gitx.WorktreeAddDetached(ctx, dir, filepath.Join(dir, filepath.FromSlash(rel)), "HEAD")
		}
	}
	worktreeDecl := func(p ...ws.WorktreePattern) ws.Declaration {
		return noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = p })
	}
	return []harnessCase{
		{
			name: "an in-store worktree under a declared directory is within (A P5b)", states: both(),
			decl:   worktreeDecl(".verdi/data/worktrees/*"),
			driver: inProcess(steps(addAt(".verdi/data/worktrees/w2"))),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Within, "worktree "+canonicalPath(fx.Dir, ".verdi/data/worktrees/w2")+" added"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "@temp rejects a worktree inside the repository (A P5a)", states: both(),
			decl:   worktreeDecl(ws.WorktreeTemp),
			driver: inProcess(steps(addAt(".verdi/data/worktrees/w1"))),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Outside, "worktree "+canonicalPath(fx.Dir, ".verdi/data/worktrees/w1")+" added"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "@temp admits fresh worktrees named by a symlinked or a relative path (A P10b)", states: both(),
			decl: worktreeDecl(ws.WorktreeTemp),
			driver: func(t *testing.T, _ *Fixture) Driver {
				unresolved := t.TempDir()
				return InProcess{Fn: steps(func(ctx context.Context, dir string) error {
					if err := gitx.WorktreeAddDetached(ctx, dir, filepath.Join(unresolved, "wt-sym"), "HEAD"); err != nil {
						return err
					}
					return gitx.WorktreeAddDetached(ctx, dir, "../wt-rel-"+filepath.Base(unresolved), "HEAD")
				})}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				before := byPath(res.Before.Worktrees)
				var vs []Verdict
				for _, w := range res.After.Worktrees {
					if _, ok := before[w.Path]; !ok {
						vs = append(vs, v("worktrees", Within, "worktree "+w.Path+" added"))
					}
				}
				if len(vs) != 2 {
					t.Fatalf("want two added worktrees, got %d", len(vs))
				}
				return append(vs, v("index_carry", Within, "declares no_commit; observed no_commit")), Pass
			},
		},
		{
			name: "@temp never matches a worktree registered before the run (A P4)", states: both(),
			decl: worktreeDecl(ws.WorktreeTemp),
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(func(ctx context.Context, dir string) error { return gitx.WorktreeRemove(ctx, dir, fx.Registered) })}
			},
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Outside, "worktree "+fx.Registered+" removed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "@registered admits removing a worktree registered before the run", states: both(),
			decl: worktreeDecl(ws.WorktreeRegistered),
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(func(ctx context.Context, dir string) error { return gitx.WorktreeRemove(ctx, dir, fx.Registered) })}
			},
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Within, "worktree "+fx.Registered+" removed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "a worktree added outside gitx is unattributable", states: both(),
			decl:   worktreeDecl(".verdi/data/worktrees/*"),
			driver: inProcess(steps(plain("worktree", "add", "--quiet", "--detach", ".verdi/data/worktrees/raw", "HEAD"))),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Unattributable, "worktree "+canonicalPath(fx.Dir, ".verdi/data/worktrees/raw")+" added"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "changes inside a pre-existing worktree are outside unless a pattern covers it (A P14)", states: both(),
			decl:   noCommitDecl(nil),
			setup:  stageInRegistered,
			driver: changeRegistered,
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Outside, "worktree "+fx.Registered+": HEAD changed"),
					v("worktrees", Outside, "worktree "+fx.Registered+": index entry op-staged.txt removed"),
					v("worktrees", Outside, "worktree "+fx.Registered+": administrative entry changed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a covered pre-existing worktree: a gitx switch is within; plain-git unstaging and the lock are unattributable", states: both(),
			decl:   worktreeDecl(ws.WorktreeRegistered),
			setup:  stageInRegistered,
			driver: changeRegistered,
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Within, "worktree "+fx.Registered+": HEAD changed"),
					v("worktrees", Unattributable, "worktree "+fx.Registered+": index entry op-staged.txt removed"),
					v("worktrees", Unattributable, "worktree "+fx.Registered+": administrative entry changed"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
	}
}

// stageInRegistered stages an operator's file in the fixture's
// pre-registered worktree.
func stageInRegistered(t *testing.T, ctx context.Context, fx *Fixture) {
	writeFixtureFile(t, fx.Registered, "op-staged.txt", "operator staged in their worktree\n")
	runGitFixture(t, ctx, fx.Registered, "add", "--", "op-staged.txt")
}

// changeRegistered switches the pre-registered worktree's HEAD through
// gitx, unstages its operator's file and locks it with plain git.
func changeRegistered(_ *testing.T, fx *Fixture) Driver {
	return InProcess{Fn: steps(
		func(ctx context.Context, _ string) error {
			return gitx.CheckoutExisting(ctx, fx.Registered, SideBranch)
		},
		func(ctx context.Context, _ string) error {
			_, err := plainGit(ctx, fx.Registered, "rm", "--cached", "-q", "--", "op-staged.txt")
			return err
		},
		plain("worktree", "lock", "--reason", "held", fx.Registered),
	)}
}

// commitCases cover SI-325 (5)-(6): every created commit, reachable or
// not, local or remote-only, merge commits, foreign entries, and commits
// made in a worktree the ritual added.
func commitCases() []harnessCase {
	orphan := steps(
		func(ctx context.Context, dir string) error {
			base, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return err
			}
			return gitx.CheckoutExisting(ctx, dir, base)
		},
		stage("owned/o.txt", "o\n"), commitIndex, checkout("main"))
	unit := func(fx *Fixture) string { return filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit") }
	handbackDecl := ws.Declaration{Ritual: "test_handback", Verbs: []ws.Verb{ws.CLI("test handback")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	// cleanRunway: a runway is clean, since the fast-forward refuses a
	// dirty tree.
	cleanRunway := func(t *testing.T, ctx context.Context, fx *Fixture) {
		runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--", TrackedFile)
		if err := os.Remove(filepath.Join(fx.Dir, UntrackedFile)); err != nil {
			t.Fatal(err)
		}
		if fx.State == SeedFull {
			runGitFixture(t, ctx, fx.Dir, "rm", "--quiet", "--cached", "--", ForeignFile)
			if err := os.Remove(filepath.Join(fx.Dir, ForeignFile)); err != nil {
				t.Fatal(err)
			}
		}
	}
	// handback is context execution's shape: a unit worktree, an agent
	// commit made there with plain git, and a fast-forward of the runway.
	handback := func(fx *Fixture) Ritual {
		return steps(
			func(ctx context.Context, dir string) error {
				return gitx.WorktreeAddDetached(ctx, dir, unit(fx), "HEAD")
			},
			writeFile(".verdi/data/worktrees/unit/unit.txt", "agent work\n"),
			func(ctx context.Context, _ string) error {
				_, err := plainGit(ctx, unit(fx), "add", "unit.txt")
				return err
			},
			func(ctx context.Context, _ string) error {
				_, err := plainGit(ctx, unit(fx), "commit", "--quiet", "-m", "agent work")
				return err
			},
			func(ctx context.Context, dir string) error {
				out, err := gitx.RevParse(ctx, unit(fx), "HEAD")
				if err != nil {
					return err
				}
				_, err = gitx.FastForwardOnly(ctx, dir, out)
				return err
			},
		)
	}
	return []harnessCase{
		{
			name: "a transient commit no ref reaches still counts under no_commit (B P6)", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) {
				d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"}
				d.RefsDelete = []ws.RefPattern{"refs/heads/ritual/*"}
			}),
			driver: inProcess(steps(newBranch("ritual/tmp"), stage("owned/t.txt", "t\n"), commitPaths("owned/t.txt"),
				checkout("main"), plain("branch", "-D", "ritual/tmp"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded owned/t.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "an orphaned commit counts under no_commit (A P3)", states: both(),
			decl:   noCommitDecl(nil),
			driver: inProcess(orphan),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				vs := []Verdict{v("stage_paths", Outside, "commit "+c+" recorded owned/o.txt")}
				if fx.State == SeedFull {
					return append(vs, orphanedForeign()...), Fail
				}
				return append(vs, v("index_carry", Outside, "declares no_commit; observed scoped")), Fail
			},
		},
		{
			name: "an orphaned commit carrying the foreign entry fails scoped; without one it passes (A P3b)", decl: scopedDecl(), states: both(),
			driver: inProcess(orphan),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				vs := []Verdict{v("stage_paths", Within, "commit "+c+" recorded owned/o.txt")}
				if fx.State == SeedFull {
					return append(vs,
						v("index", Outside, "index entry "+ForeignFile+" removed"),
						v("working_tree", Outside, ForeignFile+" deleted: the operator's pre-existing work"),
						v("index_carry", Outside, "declares scoped; observed carried; carried foreign "+ForeignFile)), Fail
				}
				return append(vs, v("index_carry", Within, "declares scoped; observed scoped")), Pass
			},
		},
		{
			name: "a merge commit's file list is against its first parent (A P8)", decl: scopedDecl(), states: []SeedState{SeedClean},
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				c := runGitFixture(t, ctx, fx.Dir, "commit-tree", "-p", SideBranch, "-m", "side diverges", "HEAD^{tree}")
				runGitFixture(t, ctx, fx.Dir, "update-ref", "refs/heads/side", trimNL(c))
				// A clean tracked tree: git 2.50 and later make stash-like
				// commit objects ("WIP on", "index on") when merging over
				// dirty tracked work, which this case is not about.
				runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--", TrackedFile)
			},
			driver: inProcess(steps(newBranch("ritual/merge"), plain("merge", "--quiet", "--no-ff", "--no-commit", SideBranch),
				stage("outside.txt", "outside\n"), commitIndex)),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/merge created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/merge"),
					v("index", Within, "index entry outside.txt added"),
					v("working_tree", Within, "outside.txt created"+fileWrite),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded outside.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a commit only the remote holds gets its file list judged", states: both(),
			decl: noCommitDecl(nil),
			driver: func(t *testing.T, fx *Fixture) Driver {
				elsewhere := filepath.Join(t.TempDir(), "elsewhere")
				return InProcess{Fn: steps(func(ctx context.Context, _ string) error {
					for _, args := range [][]string{
						{"clone", "--quiet", fx.Bare, elsewhere},
						{"-C", elsewhere, "-c", "user.name=E", "-c", "user.email=e@verdi.invalid", "commit", "--quiet", "--allow-empty", "-m", "empty"},
					} {
						if _, err := plainGit(ctx, "", args...); err != nil {
							return err
						}
					}
					if err := os.WriteFile(filepath.Join(elsewhere, "elsewhere.txt"), []byte("e\n"), 0o644); err != nil {
						return err
					}
					for _, args := range [][]string{
						{"add", "elsewhere.txt"},
						{"-c", "user.name=E", "-c", "user.email=e@verdi.invalid", "commit", "--quiet", "-m", "elsewhere"},
						{"push", "--quiet", "origin", "HEAD:refs/heads/ritual/elsewhere"},
					} {
						if _, err := plainGit(ctx, elsewhere, args...); err != nil {
							return err
						}
					}
					return nil
				})}
			},
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				ids := createdCommits(res)
				if len(ids) != 2 {
					t.Fatalf("want two created commits, got %v", ids)
				}
				empty, file := ids[0], ids[1]
				if len(res.After.Commits[full(t, res, empty)].Files) != 0 {
					empty, file = file, empty
				}
				return []Verdict{
					v("may_push", Outside, "the remote's refs/heads/ritual/elsewhere created"),
					v("stage_paths", Unattributable, "commit "+empty+" recorded no file"),
					v("stage_paths", Outside, "commit "+file+" recorded elsewhere.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "untracked_may_enter admits an untracked file, never a tracked one", states: []SeedState{SeedClean},
			decl: withScoped(func(d *ws.Declaration) { d.IndexCarry = ws.CarryRefused; d.UntrackedMayEnter = true }),
			driver: inProcess(steps(refuseIfStaged, newBranch("ritual/untracked"), stage("owned/z.txt", "z\n"),
				addPaths(UntrackedFile, TrackedFile), commitPaths("owned/z.txt", UntrackedFile, TrackedFile))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/untracked created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/untracked"),
					v("index", Within, "index entry owned/z.txt added"),
					v("index", Within, "index entry tracked.txt changed"),
					v("index", Within, "index entry untracked.txt added"),
					v("working_tree", Within, "owned/z.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/z.txt"),
					v("stage_paths", Outside, "commit "+c+" recorded tracked.txt"),
					v("untracked_may_enter", Within, "commit "+c+" recorded previously-untracked untracked.txt"),
					v("index_carry", Within, "declares refused; observed scoped"),
				}, Fail
			},
		},
		{
			name: "an untracked file may enter in the clean state; the full state refuses before any mutation", states: both(),
			decl: withScoped(func(d *ws.Declaration) { d.IndexCarry = ws.CarryRefused; d.UntrackedMayEnter = true }),
			driver: inProcess(steps(refuseIfStaged, newBranch("ritual/untracked"), stage("owned/z.txt", "z\n"),
				addPaths(UntrackedFile), commitPaths("owned/z.txt", UntrackedFile))),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				if fx.State == SeedFull {
					return []Verdict{v("index_carry", Within, "declares refused; observed refused")}, Pass
				}
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/untracked created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/untracked"),
					v("index", Within, "index entry owned/z.txt added"),
					v("index", Within, "index entry untracked.txt added"),
					v("working_tree", Within, "owned/z.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/z.txt"),
					v("untracked_may_enter", Within, "commit "+c+" recorded previously-untracked untracked.txt"),
					v("index_carry", Within, "declares refused; observed scoped"),
				}, Pass
			},
		},
		{
			name: "a nested file under a single-segment wildcard is outside", decl: checkedOutDecl(), states: both(),
			driver: inProcess(steps(stage("owned/sub/n.txt", "n\n"), commitPaths("owned/sub/n.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry owned/sub/n.txt added"),
					v("working_tree", Within, "owned/sub/n.txt created"+fileWrite),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded owned/sub/n.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a pre-staged entry inside a declared path is not foreign (B P7)", states: both(),
			decl: withScoped(func(d *ws.Declaration) { d.StagePaths = []ws.PathPattern{"owned/"} }),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				writeFixtureFile(t, fx.Dir, "owned/pre.txt", "operator\n")
				runGitFixture(t, ctx, fx.Dir, "add", "--", "owned/pre.txt")
			},
			driver: inProcess(steps(newBranch("ritual/p7"), stage("owned/mine.txt", "mine\n"), commitPaths("owned/"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p7 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p7"),
					v("index", Within, "index entry owned/mine.txt added"),
					v("working_tree", Within, "owned/mine.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/mine.txt"),
					v("stage_paths", Within, "commit "+c+" recorded owned/pre.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Pass
			},
		},
		{
			name: "a commit made in a worktree the ritual added belongs to it (SI-325 (5))", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) {
				d.HeadSwitch = false
				d.Worktrees = []ws.WorktreePattern{".verdi/data/worktrees/*"}
			}),
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						return gitx.WorktreeAddDetached(ctx, dir, unit(fx), "HEAD")
					},
					func(ctx context.Context, _ string) error {
						return writeAndStage(ctx, unit(fx), "unit.txt", "agent work\n")
					},
					func(ctx context.Context, _ string) error { return commitIndex(ctx, unit(fx)) },
				)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				w := canonicalPath("", unit(fx))
				return []Verdict{
					v("worktrees", Within, "worktree "+w+" added"),
					v("worktrees", Within, "commit "+onlyCommit(t, res)+" made in added worktree "+w),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "a commit made in an added worktree since removed belongs to it by the log", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp} }),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "evaluation")
				return InProcess{Fn: steps(
					// constitution_evaluation's shape: the add names the
					// commit it starts at.
					func(ctx context.Context, dir string) error {
						base, err := gitx.RevParse(ctx, dir, "HEAD")
						if err != nil {
							return err
						}
						return gitx.WorktreeAddDetached(ctx, dir, tmp, base)
					},
					func(ctx context.Context, _ string) error { return writeAndStage(ctx, tmp, "eval.txt", "evaluated\n") },
					func(ctx context.Context, _ string) error { return commitIndex(ctx, tmp) },
					func(ctx context.Context, dir string) error { return gitx.WorktreeRemove(ctx, dir, tmp) },
				)}
			},
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				var tmp string
				for _, c := range res.Log.Calls {
					if len(c.Args) == 5 && c.Args[1] == "add" {
						tmp = canonicalPath(c.Dir, c.Args[3])
					}
				}
				return []Verdict{
					v("worktrees", Within, "commit "+onlyCommit(t, res)+" made in added worktree "+tmp),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "a fast-forward of @checked-out to an added worktree's commit is a declared move (context execution's hand-back)", states: both(),
			decl:   handbackDecl,
			setup:  cleanRunway,
			driver: func(_ *testing.T, fx *Fixture) Driver { return InProcess{Fn: handback(fx)} },
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				w := canonicalPath("", unit(fx))
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry unit.txt added"),
					v("working_tree", Within, "unit.txt created"+fileWrite),
					v("worktrees", Within, "worktree "+w+" added"),
					v("worktrees", Within, "commit "+onlyCommit(t, res)+" made in added worktree "+w),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Pass
			},
		},
		{
			name: "without a command log the hand-back is unproven, never outside: its agent commit needs the logged fast-forward (SI-329 (5′))", states: both(),
			decl:   handbackDecl,
			setup:  cleanRunway,
			driver: func(_ *testing.T, fx *Fixture) Driver { return noLog{fn: handback(fx)} },
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				w := canonicalPath("", unit(fx))
				return []Verdict{
					v("command_log", Unattributable, "the driver supplied no git command log"),
					v("refs_move", Unattributable, "refs/heads/main moved"),
					v("index", Unattributable, "index entry unit.txt added"),
					v("working_tree", Within, "unit.txt created"+fileWrite),
					v("worktrees", Unattributable, "worktree "+w+" added"),
					v("worktrees", Unattributable, "commit "+onlyCommit(t, res)+" made in added worktree "+w),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "a commit reachable from a fixture ref is never credited to an added worktree", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) {
				d.HeadSwitch = false
				d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"}
				d.Worktrees = []ws.WorktreePattern{ws.WorktreeTemp}
			}),
			driver: func(t *testing.T, _ *Fixture) Driver {
				tmp := filepath.Join(t.TempDir(), "decoy")
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error { return gitx.WorktreeAddDetached(ctx, dir, tmp, "HEAD") },
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

// orphanedForeign is A P3's full-state tail: switching back from the
// orphaned commit deletes the foreign entry it carried.
func orphanedForeign() []Verdict {
	return []Verdict{
		v("index", Outside, "index entry "+ForeignFile+" removed"),
		v("working_tree", Outside, ForeignFile+" deleted: the operator's pre-existing work"),
		v("index_carry", Outside, "declares no_commit; observed carried; carried foreign "+ForeignFile),
	}
}

// gitDirCases cover SI-325 (7)'s classes no field admits: tags and other
// namespaces, config, hooks, and info.
func gitDirCases() []harnessCase {
	return []harnessCase{
		{
			name: "tags, private refs, config, hooks, and a stray git-dir file (A P7)", states: both(),
			decl: noCommitDecl(nil),
			driver: inProcess(steps(
				plain("tag", "v-light"),
				plain("tag", "-a", "v-annot", "-m", "annotated"),
				plain("config", "branch.main.remote", "origin"),
				plain("config", "core.hooksPath", "/nonexistent/hooks"),
				plain("update-ref", "refs/remotes/origin/sneaky", "HEAD"),
				plain("update-ref", "refs/verdi/private", "HEAD"),
				plain("push", "--quiet", "origin", "v-light"),
				writeFile(".git/hooks/post-commit", "#!/bin/sh\nexit 0\n"),
				writeFile(".git/info/attributes", "* -text\n"),
				writeFile(".git/stray.txt", "x"),
			)),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Outside, "refs/tags/v-annot created"),
					v("refs_create", Outside, "refs/tags/v-light created"),
					v("refs_create", Outside, "refs/verdi/private created"),
					v("may_push", Outside, "refs/remotes/origin/sneaky created"),
					v("may_push", Outside, "the remote's refs/tags/v-light created"),
					v("config", Outside, "config branch.main.remote set"),
					v("config", Outside, "config core.hookspath set"),
					v("git_dir", Outside, "hooks/post-commit created"),
					v("git_dir", Outside, "info/attributes created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a mode-only change to an already-dirty file is sensed (beyond SI-325 (9))", states: both(),
			decl: noCommitDecl(nil),
			driver: inProcess(steps(func(_ context.Context, dir string) error {
				return os.Chmod(filepath.Join(dir, TrackedFile), 0o755)
			})),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("working_tree", Outside, "tracked.txt changed: the operator's pre-existing work"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
	}
}

// logCases cover the command log: a driver without one, and SI-325 (8)'s
// disclosed residual.
func logCases() []harnessCase {
	return []harnessCase{
		{
			name: "a driver without a command log is disclosed and its effects unattributable (A P12, B P10)", decl: scopedDecl(), states: both(),
			driver: func(*testing.T, *Fixture) Driver { return noLog{fn: ritualScoped} },
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("command_log", Unattributable, "the driver supplied no git command log"),
					v("refs_create", Unattributable, "refs/heads/ritual/scoped-ok created"),
					v("head_switch", Unattributable, "HEAD switched from refs/heads/main to refs/heads/ritual/scoped-ok"),
					v("index", Unattributable, "index entry owned/new.txt added"),
					v("working_tree", Within, "owned/new.txt created"+fileWrite),
					v("stage_paths", Unattributable, "commit "+onlyCommit(t, res)+" recorded owned/new.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Unproven
			},
		},
		{
			name: "a driver without a command log still fails what no field admits", decl: scopedDecl(), states: []SeedState{SeedFull},
			driver: func(*testing.T, *Fixture) Driver { return noLog{fn: ritualCarriesForeign} },
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("command_log", Unattributable, "the driver supplied no git command log"),
					v("refs_create", Unattributable, "refs/heads/ritual/carries created"),
					v("head_switch", Unattributable, "HEAD switched from refs/heads/main to refs/heads/ritual/carries"),
					v("index", Unattributable, "index entry owned/c.txt added"),
					v("working_tree", Within, "owned/c.txt created"+fileWrite),
					v("stage_paths", Unattributable, "commit "+onlyCommit(t, res)+" recorded owned/c.txt"),
					v("index_carry", Outside, "declares scoped; observed carried; carried foreign "+ForeignFile),
				}, Fail
			},
		},
		{
			name: "disclosed residual (SI-325 (8)): a failed commit is credited with a move made outside gitx (A P9b)", states: both(),
			decl: checkedOutDecl(),
			note: "PINNED DISCLOSURE: this case pins ledger SI-325 (8)'s residual (gitx's observer fires before exec, so a failed " +
				"`git commit` is credited with a branch move made outside gitx). If spec/gitx-recorder-seam ac-1 (no git execution " +
				"outside gitx) closed it, update the residual paragraph in doc.go, then this case.\n",
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				_, _ = gitx.CreateCommitPaths(ctx, dir, "nothing to commit: fails", "owned/keep.txt") // logged before exec; fails
				return steps(writeFile("owned/q.txt", "q\n"), plain("add", "owned/q.txt"), func(ctx context.Context, dir string) error {
					tree, err := plainGit(ctx, dir, "write-tree")
					if err != nil {
						return err
					}
					c, err := plainGit(ctx, dir, "commit-tree", tree, "-p", "HEAD", "-m", "outside gitx")
					if err != nil {
						return err
					}
					_, err = plainGit(ctx, dir, "update-ref", "refs/heads/main", c)
					return err
				})(ctx, dir)
			}),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				// The residual is the refs_move verdict: within, though the
				// move was made outside gitx. The index entry and the
				// commit's path are attributed per path (SI-329 (8′)): the
				// failed commit named owned/keep.txt, so neither is. In the
				// full state the plain write-tree also recorded the foreign
				// entry, which the state diff catches whoever made it.
				vs := []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Unattributable, "index entry owned/q.txt added"),
					v("working_tree", Within, "owned/q.txt created"+fileWrite),
					v("stage_paths", Unattributable, "commit "+onlyCommit(t, res)+" recorded owned/q.txt"),
				}
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares scoped; observed carried; carried foreign "+ForeignFile)), Fail
				}
				return append(vs, v("index_carry", Within, "declares scoped; observed scoped")), Unproven
			},
		},
	}
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// full returns the full id of a created commit from its short id.
func full(t *testing.T, res Result, shortID string) string {
	t.Helper()
	for id := range res.After.Commits {
		if short(id) == shortID {
			return id
		}
	}
	t.Fatalf("no commit %s", shortID)
	return ""
}
