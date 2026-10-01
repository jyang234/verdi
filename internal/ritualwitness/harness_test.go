package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// harnessCase is one synthetic ritual, its declaration, and the exact
// verdicts it must get in each seeded state it runs in.
type harnessCase struct {
	name   string
	decl   ws.Declaration
	states []SeedState
	// setup seeds more than the SeedState gives, before the before
	// snapshot; nil for none.
	setup func(t *testing.T, ctx context.Context, fx *Fixture)
	// driver builds the run's Driver; it may close over fx and t.
	driver func(t *testing.T, fx *Fixture) Driver
	// want is the exact verdict multiset and outcome for one state.
	want func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome)
	// note, when set, heads any failure: a pinned disclosure names what
	// must change with it.
	note string
}

func both() []SeedState { return []SeedState{SeedFull, SeedClean} }

func inProcess(fn Ritual) func(*testing.T, *Fixture) Driver {
	return func(*testing.T, *Fixture) Driver { return InProcess{Fn: fn} }
}

// scopedDecl is the common scoped declaration: a ritual/* branch it
// switches to, commits of owned/*.
func scopedDecl() ws.Declaration {
	return ws.Declaration{
		Ritual:     "test_scoped",
		Verbs:      []ws.Verb{ws.CLI("test scoped")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true,
		StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
}

func noCommitDecl(mod func(*ws.Declaration)) ws.Declaration {
	d := ws.Declaration{Ritual: "test_no_commit", Verbs: []ws.Verb{ws.CLI("test no-commit")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	if mod != nil {
		mod(&d)
	}
	return d
}

func checkedOutDecl() ws.Declaration {
	return ws.Declaration{
		Ritual:     "test_checked_out",
		Verbs:      []ws.Verb{ws.CLI("test checked-out")},
		RefsMove:   []ws.RefPattern{ws.RefCheckedOut},
		StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
}

func withScoped(mod func(*ws.Declaration)) ws.Declaration {
	d := scopedDecl()
	mod(&d)
	return d
}

const fileWrite = ": a file write, not a git mutation"

// scopedWithin is ritualScoped's verdicts: everything within.
func scopedWithin(commit string) []Verdict {
	return []Verdict{
		v("refs_create", Within, "refs/heads/ritual/scoped-ok created"),
		v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/scoped-ok"),
		v("index", Within, "index entry owned/new.txt added"),
		v("working_tree", Within, "owned/new.txt created"+fileWrite),
		v("stage_paths", Within, "commit "+commit+" recorded owned/new.txt"),
		v("index_carry", Within, "declares scoped; observed scoped"),
	}
}

// TestHarness_SensorsAndVerdicts is obligation/ritual-effect-witness--
// ac-1--behavioral's producer. Over synthetic rituals, each in the seeded
// states ac-1 requires, it asserts the exact verdict multiset (field,
// status, and detail) and the run's Outcome: a ritual within its
// declaration passes; one that stages outside its paths, moves an
// undeclared ref, adds an undeclared worktree, pushes outside may-push,
// switches HEAD outside head-switch, commits under no_commit, or carries a
// foreign entry while declaring scoped fails naming the effect; one that
// refuses while declaring scoped fails; an effect the sensors cannot
// attribute is unattributable, never within; and every effect class
// SI-325 reads is classified, never silently passed. Every fixture is a
// fixturegit repository with a local bare remote (no network).
func TestHarness_SensorsAndVerdicts(t *testing.T) {
	ctx := context.Background()

	// Ambient core.quotePath must change no result: every path is read
	// with -z. These run first and sequentially, since they set the
	// process environment for git.
	for _, quote := range []string{"true", "false"} {
		t.Run("ambient core.quotePath="+quote, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_COUNT", "1")
			t.Setenv("GIT_CONFIG_KEY_0", "core.quotePath")
			t.Setenv("GIT_CONFIG_VALUE_0", quote)
			for _, c := range nonASCIICases() {
				t.Run(c.name, func(t *testing.T) { runCase(t, ctx, c, SeedClean) })
			}
		})
	}

	for _, c := range harnessCases() {
		for _, state := range c.states {
			t.Run(c.name+"/"+state.String(), func(t *testing.T) {
				t.Parallel()
				runCase(t, ctx, c, state)
			})
		}
	}
}

func runCase(t *testing.T, ctx context.Context, c harnessCase, state SeedState) {
	t.Helper()
	if err := c.decl.Validate(); err != nil {
		t.Fatalf("case declaration: %v", err)
	}
	fx := Build(t, ctx, state)
	if c.setup != nil {
		c.setup(t, ctx, fx)
	}
	res := RunOn(t, ctx, fx, c.driver(t, fx), c.decl)
	want, outcome := c.want(t, fx, res)
	if diff := verdictDiff(res.Verdicts, want); diff != "" {
		t.Fatalf("%s%s", c.note, diff)
	}
	if got := Outcome(res.Verdicts); got != outcome {
		t.Fatalf("%sOutcome = %s, want %s", c.note, got, outcome)
	}
}

func nonASCIICases() []harnessCase {
	return []harnessCase{
		{
			name:   "a non-ASCII path under a declared directory is within (B P9)",
			decl:   scopedDecl(),
			driver: inProcess(steps(newBranch("ritual/p9"), stage("owned/café.txt", "x\n"), commitPaths("owned/café.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p9 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p9"),
					v("index", Within, "index entry owned/café.txt added"),
					v("working_tree", Within, "owned/café.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/café.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Pass
			},
		},
		{
			name: "a carried non-ASCII foreign entry is named (A P13)",
			decl: scopedDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				writeFixtureFile(t, fx.Dir, "föreign.txt", "colleague\n")
				runGitFixture(t, ctx, fx.Dir, "add", "--", "föreign.txt")
			},
			driver: inProcess(ritualCarriesForeign),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/carries created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/carries"),
					v("index", Within, "index entry owned/c.txt added"),
					v("working_tree", Within, "owned/c.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/c.txt"),
					v("index_carry", Outside, "declares scoped; observed carried; carried foreign föreign.txt"),
				}, Fail
			},
		},
	}
}

func harnessCases() []harnessCase {
	var cases []harnessCase
	for _, group := range [][]harnessCase{
		obligationCases(),
		indexAndTreeCases(),
		refCases(),
		worktreeCases(),
		commitCases(),
		gitDirCases(),
		logCases(),
	} {
		cases = append(cases, group...)
	}
	return cases
}

// obligationCases are the obligation's own synthetic rituals.
func obligationCases() []harnessCase {
	return []harnessCase{
		{
			name: "within its declaration passes", decl: scopedDecl(), states: both(),
			driver: inProcess(ritualScoped),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return scopedWithin(onlyCommit(t, res)), Pass
			},
		},
		{
			name: "stages outside its declared paths fails naming the effect", decl: scopedDecl(), states: both(),
			driver: inProcess(steps(writeFile("owned/new2.txt", "new2\n"), newBranch("ritual/outside"),
				addPaths("owned/new2.txt", TrackedFile), commitPaths("owned/new2.txt", TrackedFile))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/outside created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/outside"),
					v("index", Within, "index entry owned/new2.txt added"),
					// The commit moved HEAD's tree at tracked.txt; the commit
					// itself is judged below.
					v("index", Within, "index entry tracked.txt changed"),
					v("working_tree", Within, "owned/new2.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/new2.txt"),
					v("stage_paths", Outside, "commit "+c+" recorded tracked.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "stages outside its paths without committing them fails naming the index entry (B P1)", decl: scopedDecl(), states: both(),
			driver: inProcess(steps(newBranch("ritual/p1"), stage("owned/p1.txt", "p1\n"), addPaths(TrackedFile), commitPaths("owned/p1.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p1 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p1"),
					v("index", Within, "index entry owned/p1.txt added"),
					v("index", Outside, "index entry tracked.txt changed"),
					v("working_tree", Within, "owned/p1.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/p1.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "moves an undeclared ref fails naming the effect",
			decl: ws.Declaration{Ritual: "test_moves_undeclared", Verbs: []ws.Verb{ws.CLI("test moves-undeclared")},
				RefsMove: []ws.RefPattern{"refs/heads/decoy"}, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped},
			states: both(),
			driver: inProcess(ritualMovesUndeclaredRef),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Outside, "refs/heads/main moved"),
					v("index", Within, "index entry owned/x.txt added"),
					v("working_tree", Within, "owned/x.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/x.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a commit on the checked-out branch moves @checked-out and is no HEAD switch (commit_to_design's shape; A P2, B P4)",
			decl: checkedOutDecl(), states: both(),
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
			name:   "@checked-out names only the branch checked out before the ritual",
			decl:   withScoped(func(d *ws.Declaration) { d.RefsCreate = nil; d.RefsMove = []ws.RefPattern{ws.RefCheckedOut} }),
			states: both(),
			driver: inProcess(steps(checkout(SideBranch), stage("owned/d.txt", "d\n"), commitPaths("owned/d.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Outside, "refs/heads/side moved"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/side"),
					v("index", Within, "index entry owned/d.txt added"),
					v("working_tree", Within, "owned/d.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/d.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "adds an undeclared worktree fails naming the effect",
			decl: noCommitDecl(func(d *ws.Declaration) {
				d.HeadSwitch = false
				d.Worktrees = []ws.WorktreePattern{".verdi/data/worktrees/*"}
			}),
			states: both(),
			driver: func(t *testing.T, _ *Fixture) Driver {
				outside := filepath.Join(t.TempDir(), "outside-wt")
				return InProcess{Fn: steps(func(ctx context.Context, dir string) error {
					return gitx.WorktreeAddDetached(ctx, dir, outside, "HEAD")
				})}
			},
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("worktrees", Outside, "worktree "+addedWorktree(t, res)+" added"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "carries a foreign entry while declaring scoped fails naming the effect; the clean state passes", decl: scopedDecl(), states: both(),
			driver: inProcess(ritualCarriesForeign),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{
					v("refs_create", Within, "refs/heads/ritual/carries created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/carries"),
					v("index", Within, "index entry owned/c.txt added"),
					v("working_tree", Within, "owned/c.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/c.txt"),
				}
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares scoped; observed carried; carried foreign "+ForeignFile)), Fail
				}
				return append(vs, v("index_carry", Within, "declares scoped; observed scoped")), Pass
			},
		},
		{
			name: "refuses while declaring scoped fails; the same ritual completes in the clean state", decl: scopedDecl(), states: both(),
			driver: inProcess(ritualScopedButRefuses),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				if fx.State == SeedFull {
					if res.Exit != 2 {
						t.Fatalf("full-state exit = %d, want 2", res.Exit)
					}
					return []Verdict{v("index_carry", Outside, "declares scoped; observed refused")}, Fail
				}
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/guard-then-scope created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/guard-then-scope"),
					v("index", Within, "index entry owned/f.txt added"),
					v("working_tree", Within, "owned/f.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/f.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Pass
			},
		},
		{
			name: "an effect the sensors cannot attribute is unattributable, never within", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.HeadSwitch = false; d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(ritualBypassesGitx),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Unattributable, "refs/heads/ritual/sneaky created"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Unproven
			},
		},
		{
			name: "pushes outside may-push fails naming the effect", decl: scopedDecl(), states: both(),
			driver: inProcess(steps(newBranch("ritual/push-bug"), stage("owned/p.txt", "p\n"), commitPaths("owned/p.txt"),
				func(ctx context.Context, dir string) error { return gitx.Push(ctx, dir) })),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return append(pushVerdicts(t, res, "ritual/push-bug", "owned/p.txt", Outside),
					v("index_carry", Within, "declares scoped; observed scoped")), Fail
			},
		},
		{
			name: "a may-push push with its upstream config and remote-tracking mirror is within (board_commit_push's push)",
			decl: withScoped(func(d *ws.Declaration) { d.MayPush = true }), states: both(),
			driver: inProcess(steps(newBranch("ritual/push-ok"), stage("owned/p.txt", "p\n"), commitPaths("owned/p.txt"),
				func(ctx context.Context, dir string) error { return gitx.Push(ctx, dir) })),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return append(pushVerdicts(t, res, "ritual/push-ok", "owned/p.txt", Within),
					v("index_carry", Within, "declares scoped; observed scoped")), Pass
			},
		},
		{
			name: "HEAD switched without head-switch fails naming the effect", states: both(),
			decl: noCommitDecl(func(d *ws.Declaration) {
				d.HeadSwitch = false
				d.Worktrees = []ws.WorktreePattern{ws.WorktreeRegistered}
			}),
			driver: inProcess(steps(checkout(SideBranch))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("head_switch", Outside, "HEAD switched from refs/heads/main to refs/heads/side"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a commit when no_commit is declared fails naming the effect", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(steps(newBranch("ritual/no-commit-bug"), stage("owned/w.txt", "w\n"), commitPaths("owned/w.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/no-commit-bug created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/no-commit-bug"),
					v("index", Within, "index entry owned/w.txt added"),
					v("working_tree", Within, "owned/w.txt created"+fileWrite),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded owned/w.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
	}
}

// pushVerdicts is a branch-commit-push run's verdicts but the carry, with
// the push's own four effects at status pushed.
func pushVerdicts(t *testing.T, res Result, branch, path string, pushed Status) []Verdict {
	t.Helper()
	ref := "refs/heads/" + branch
	return []Verdict{
		v("refs_create", Within, ref+" created"),
		v("head_switch", Within, "HEAD switched from refs/heads/main to "+ref),
		v("index", Within, "index entry "+path+" added"),
		v("working_tree", Within, path+" created"+fileWrite),
		v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded "+path),
		v("may_push", pushed, "refs/remotes/origin/"+branch+" created"),
		v("may_push", pushed, "the remote's "+ref+" created"),
		v("config", pushed, "config branch."+branch+".merge set"),
		v("config", pushed, "config branch."+branch+".remote set"),
	}
}

// addedWorktree returns the one worktree a run added.
func addedWorktree(t *testing.T, res Result) string {
	t.Helper()
	before := byPath(res.Before.Worktrees)
	var added []string
	for _, w := range res.After.Worktrees {
		if _, ok := before[w.Path]; !ok {
			added = append(added, w.Path)
		}
	}
	if len(added) != 1 {
		t.Fatalf("want exactly one added worktree, got %v", added)
	}
	return added[0]
}

// indexAndTreeCases cover SI-325 (1)-(3): the index, the working tree, and
// a refusal after a mutation.
func indexAndTreeCases() []harnessCase {
	return []harnessCase{
		{
			name: "staging the operator's work outside every stage path is outside (A P1)", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(steps(newBranch("ritual/p1"), func(ctx context.Context, dir string) error { return gitx.AddAll(ctx, dir) })),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p1 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p1"),
					v("index", Outside, "index entry tracked.txt changed"),
					v("index", Outside, "index entry untracked.txt added"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "discarding the operator's dirty and untracked work is outside (A P1c)", states: both(),
			decl:   noCommitDecl(func(d *ws.Declaration) { d.RefsCreate = []ws.RefPattern{"refs/heads/ritual/*"} }),
			driver: inProcess(steps(newBranch("ritual/p1c"), writeFile(TrackedFile, "original content\n"), removeFile(UntrackedFile))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p1c created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p1c"),
					v("working_tree", Outside, "tracked.txt changed: the operator's pre-existing work"),
					v("working_tree", Outside, "untracked.txt deleted: the operator's pre-existing work"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "a plain-git checkout discarding the operator's work is outside (B P2)", states: both(),
			decl:   noCommitDecl(nil),
			driver: inProcess(steps(plain("checkout", "--", TrackedFile), removeFile(UntrackedFile))),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("working_tree", Outside, "tracked.txt changed: the operator's pre-existing work"),
					v("working_tree", Outside, "untracked.txt deleted: the operator's pre-existing work"),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "content changes status cannot show are sensed: rewritten dirty and untracked files, a mode change", states: both(),
			decl: noCommitDecl(nil),
			driver: inProcess(steps(writeFile(TrackedFile, "rewritten, still dirty\n"), writeFile(UntrackedFile, "rewritten, still untracked\n"),
				func(_ context.Context, dir string) error {
					return os.Chmod(filepath.Join(dir, OwnedDir, "keep.txt"), 0o755)
				})),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("working_tree", Outside, "tracked.txt changed: the operator's pre-existing work"),
					v("working_tree", Outside, "untracked.txt changed: the operator's pre-existing work"),
					v("working_tree", Within, "owned/keep.txt changed"+fileWrite),
					v("index_carry", Within, "declares no_commit; observed no_commit"),
				}, Fail
			},
		},
		{
			name: "unstaging the foreign entry is outside; with nothing to unstage the ritual refuses cleanly (B P2b)", states: both(),
			decl:   noCommitDecl(nil),
			driver: inProcess(steps(plain("rm", "--cached", "-q", "--", ForeignFile))),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				if fx.State == SeedFull {
					return []Verdict{
						v("index", Outside, "index entry "+ForeignFile+" removed"),
						v("index_carry", Within, "declares no_commit; observed no_commit"),
					}, Fail
				}
				return []Verdict{v("index_carry", Within, "declares no_commit; observed refused")}, Pass
			},
		},
		{
			name: "exit 2 after a mutation is outside for a scoped declaration (B P3)", decl: scopedDecl(), states: both(),
			driver: inProcess(steps(newBranch("ritual/p3"), refuseIfStaged)),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p3 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p3"),
					v("index_carry", Within, "declares scoped; observed no_commit"),
				}
				if fx.State == SeedFull {
					return append(vs, v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: refs, HEAD")), Fail
				}
				return vs, Pass
			},
		},
		{
			name: "exit 2 after a mutation is outside for a refused declaration too", states: both(),
			decl:   withScoped(func(d *ws.Declaration) { d.IndexCarry = ws.CarryRefused }),
			driver: inProcess(steps(newBranch("ritual/p3"), refuseIfStaged)),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				vs := []Verdict{
					v("refs_create", Within, "refs/heads/ritual/p3 created"),
					v("head_switch", Within, "HEAD switched from refs/heads/main to refs/heads/ritual/p3"),
				}
				if fx.State == SeedFull {
					return append(vs, v("index_carry", Outside, "declares refused; observed no_commit"),
						v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: refs, HEAD")), Fail
				}
				return append(vs, v("index_carry", Within, "declares refused; observed no_commit")), Pass
			},
		},
		{
			name: "a refusal that rewrote the foreign entry first is outside (B P8)", states: []SeedState{SeedFull},
			decl: withScoped(func(d *ws.Declaration) { d.IndexCarry = ws.CarryRefused }),
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				if err := writeAndStage(ctx, dir, ForeignFile, "clobbered\n"); err != nil {
					return 2, err
				}
				return 2, os.ErrExist
			}),
			want: func(*testing.T, *Fixture, Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("index", Outside, "index entry "+ForeignFile+" changed"),
					v("working_tree", Outside, ForeignFile+" changed: the operator's pre-existing work"),
					v("index_carry", Outside, "declares refused; observed no_commit"),
					v("exit", Outside, "exit 2 (a refusal) with git mutations remaining: index"),
				}, Fail
			},
		},
		{
			name: "a branch cut and unwound before exit 2 leaves nothing remaining: a clean refusal (close's failure unwind)", states: both(),
			decl: ws.Declaration{Ritual: "test_unwind", Verbs: []ws.Verb{ws.CLI("test unwind")},
				RefsCreate: []ws.RefPattern{"refs/heads/close/*"}, RefsDelete: []ws.RefPattern{"refs/heads/close/*"}, HeadSwitch: true,
				StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryRefused},
			driver: inProcess(func(ctx context.Context, dir string) (int, error) {
				if code, err := steps(newBranch("close/x"), writeFile("owned/unstaged.txt", "a file write\n"), checkout("main"),
					func(ctx context.Context, dir string) error { return gitx.DeleteBranch(ctx, dir, "close/x") })(ctx, dir); err != nil {
					return code, err
				}
				return 2, os.ErrClosed
			}),
			want: func(_ *testing.T, fx *Fixture, _ Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("working_tree", Within, "owned/unstaged.txt created"+fileWrite),
					v("index_carry", Within, "declares refused; observed refused"),
				}, Pass
			},
		},
		{
			name: "a detached HEAD moved by a commit is a HEAD switch (SI-325 (4))", states: both(),
			decl: checkedOutDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--detach")
			},
			driver: inProcess(ritualMovesUndeclaredRef),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("head_switch", Outside, "HEAD switched from detached at "+short(fx.BaseCommit)+" to detached at "+c),
					v("index", Within, "index entry owned/x.txt added"),
					v("working_tree", Within, "owned/x.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/x.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a detached HEAD moved by a commit is within a declared HEAD switch", states: both(),
			decl: func() ws.Declaration { d := checkedOutDecl(); d.HeadSwitch = true; return d }(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--detach")
			},
			driver: inProcess(ritualMovesUndeclaredRef),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("head_switch", Within, "HEAD switched from detached at "+short(fx.BaseCommit)+" to detached at "+c),
					v("index", Within, "index entry owned/x.txt added"),
					v("working_tree", Within, "owned/x.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded owned/x.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Pass
			},
		},
		{
			name: "a store below the repository root keeps its declared paths store-relative (A10)", states: both(),
			decl: checkedOutDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				writeFixtureFile(t, fx.Dir, "store/owned/.keep", "")
				runGitFixture(t, ctx, fx.Dir, "add", "--", "store/owned/.keep")
				runGitFixture(t, ctx, fx.Dir, "commit", "--quiet", "-m", "a store below the root", "--", "store/owned/.keep")
				fx.Dir = filepath.Join(fx.Dir, "store")
			},
			driver: inProcess(steps(stage("owned/s.txt", "s\n"), stage("../owned/r.txt", "r\n"), commitPaths("owned/s.txt", "../owned/r.txt"))),
			want: func(t *testing.T, _ *Fixture, res Result) ([]Verdict, RunOutcome) {
				c := onlyCommit(t, res)
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry store/owned/s.txt added"),
					v("index", Within, "index entry owned/r.txt added"),
					v("working_tree", Within, "store/owned/s.txt created"+fileWrite),
					v("working_tree", Within, "owned/r.txt created"+fileWrite),
					v("stage_paths", Within, "commit "+c+" recorded store/owned/s.txt"),
					v("stage_paths", Outside, "commit "+c+" recorded owned/r.txt"),
					v("index_carry", Within, "declares scoped; observed scoped"),
				}, Fail
			},
		},
	}
}
