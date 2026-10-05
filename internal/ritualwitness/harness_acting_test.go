package ritualwitness

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// actingBranch is the branch a pre-existing linked worktree holds in the
// acting-checkout cases, the shape of a /b/ route's managed worktree.
const actingBranch = "acting"

// actingWorktree is that worktree's path, inside the fixture's ignored
// data zone as a managed worktree is.
func actingWorktree(fx *Fixture) string {
	return filepath.Join(fx.Dir, ".verdi", "data", "worktrees", actingBranch)
}

// addActingWorktree registers the acting worktree on a new branch at the
// base commit before the run.
func addActingWorktree(t *testing.T, ctx context.Context, fx *Fixture) {
	t.Helper()
	runGitFixture(t, ctx, fx.Dir, "worktree", "add", "--quiet", "-b", actingBranch, actingWorktree(fx), fx.BaseCommit)
}

// commitIn writes and stages owned/acting.txt in dir and commits it by
// path, through gitx, so the log attributes the move.
func commitIn(dir func(*Fixture) string) func(*testing.T, *Fixture) Driver {
	return func(_ *testing.T, fx *Fixture) Driver {
		return InProcess{Fn: func(ctx context.Context, _ string) (int, error) {
			if err := writeAndStage(ctx, dir(fx), "owned/acting.txt", "made in the acting checkout\n"); err != nil {
				return 2, err
			}
			if _, err := gitx.CreateCommitPaths(ctx, dir(fx), "commit in the acting checkout", "owned/acting.txt"); err != nil {
				return 2, err
			}
			return 0, nil
		}}
	}
}

// actingDecl moves @checked-out, may change worktrees under the data
// zone, and commits owned/*: the board's Commit and push beneath /b/,
// scoped for the test.
func actingDecl() ws.Declaration {
	return ws.Declaration{
		Ritual:     "test_acting",
		Verbs:      []ws.Verb{ws.CLI("test acting")},
		RefsMove:   []ws.RefPattern{ws.RefCheckedOut},
		Worktrees:  []ws.WorktreePattern{".verdi/data/worktrees/*"},
		StagePaths: []ws.PathPattern{"owned/*"},
		IndexCarry: ws.CarryScoped,
	}
}

// TestHarness_CheckedOutBindsToTheActingCheckout is ledger SI-348 (4):
// @checked-out is the branch checked out in the checkout the ritual acts
// on (the write-scope grammar), so a ritual acting in a pre-existing
// linked worktree — a /b/ route's managed worktree — moves that
// worktree's branch within refs_move, and a move of the fixture root's
// branch is outside it. With no acting checkout named, the binding is the
// fixture root's, as before; an acting checkout that is no worktree
// registered before the run binds no branch, so every @checked-out move is
// outside (fail closed).
func TestHarness_CheckedOutBindsToTheActingCheckout(t *testing.T) {
	ctx := context.Background()
	inWorktree := func(fx *Fixture) string { return actingWorktree(fx) }
	atRoot := func(fx *Fixture) string { return fx.Dir }
	cases := []harnessCase{
		{
			name: "a commit moving the acting worktree's own branch is within", states: both(), decl: actingDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				addActingWorktree(t, ctx, fx)
				fx.Acting = actingWorktree(fx)
			},
			driver: commitIn(inWorktree),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				wt := canonicalPath("", actingWorktree(fx))
				return []Verdict{
					v("refs_move", Within, "refs/heads/"+actingBranch+" moved"),
					v("worktrees", Within, "worktree "+wt+": index entry owned/acting.txt added"),
					v("index_carry", Within, "declares scoped; observed scoped"),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/acting.txt"),
				}, Pass
			},
		},
		{
			name: "with no acting checkout named, @checked-out is the fixture root's branch", states: both(), decl: actingDecl(),
			setup:  addActingWorktree,
			driver: commitIn(inWorktree),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				wt := canonicalPath("", actingWorktree(fx))
				return []Verdict{
					v("refs_move", Outside, "refs/heads/"+actingBranch+" moved"),
					v("worktrees", Within, "worktree "+wt+": index entry owned/acting.txt added"),
					v("index_carry", Within, "declares scoped; observed scoped"),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/acting.txt"),
				}, Fail
			},
		},
		{
			name: "acting in a linked worktree, a move of the fixture root's branch is outside", states: both(), decl: actingDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				addActingWorktree(t, ctx, fx)
				fx.Acting = actingWorktree(fx)
			},
			driver: commitIn(atRoot),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Outside, "refs/heads/main moved"),
					v("index", Within, "index entry owned/acting.txt added"),
					v("working_tree", Within, "owned/acting.txt created"+fileWrite),
					v("index_carry", Within, "declares scoped; observed scoped"),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/acting.txt"),
				}, Fail
			},
		},
		{
			name: "an acting checkout that is no registered worktree binds no branch", states: both(), decl: actingDecl(),
			setup: func(t *testing.T, ctx context.Context, fx *Fixture) {
				addActingWorktree(t, ctx, fx)
				fx.Acting = filepath.Join(fx.Dir, OwnedDir)
			},
			driver: commitIn(inWorktree),
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				wt := canonicalPath("", actingWorktree(fx))
				return []Verdict{
					v("refs_move", Outside, "refs/heads/"+actingBranch+" moved"),
					v("worktrees", Within, "worktree "+wt+": index entry owned/acting.txt added"),
					v("index_carry", Within, "declares scoped; observed scoped"),
					v("stage_paths", Within, "commit "+onlyCommit(t, res)+" recorded owned/acting.txt"),
				}, Fail
			},
		},
	}
	for _, c := range cases {
		for _, state := range c.states {
			t.Run(c.name+"/"+state.String(), func(t *testing.T) {
				t.Parallel()
				runCase(t, ctx, c, state)
			})
		}
	}
}
