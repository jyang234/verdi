package ritualwitness

// Closure-check probe (R2 half A), copied from the lane R2 closure check
// (zz_closure_a_probe_test.go) unchanged but for this header: its
// assertion is the reviewer's own.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// CL1: a commit made in the MAIN worktree (logged there) is owned by an
// added worktree via (5′)(i)'s hand-back exception once that worktree is
// pointed at it and @checked-out is fast-forwarded to it.
func TestZZCL_1_MainCommitOwnedViaHandback(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "cl_handback", Verbs: []ws.Verb{ws.CLI("cl handback")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	for _, pointW := range []bool{false, true} {
		for _, state := range both() {
			t.Run(map[bool]string{false: "control", true: "probe"}[pointW]+"/"+state.String(), func(t *testing.T) {
				fx := Build(t, ctx, state)
				runGitFixture(t, ctx, fx.Dir, "checkout", "--quiet", "--", TrackedFile)
				if err := os.Remove(filepath.Join(fx.Dir, UntrackedFile)); err != nil {
					t.Fatal(err)
				}
				if state == SeedFull {
					runGitFixture(t, ctx, fx.Dir, "rm", "--quiet", "--cached", "--", ForeignFile)
					if err := os.Remove(filepath.Join(fx.Dir, ForeignFile)); err != nil {
						t.Fatal(err)
					}
				}
				w := filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit")
				res := RunOn(t, ctx, fx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
					if err := gitx.WorktreeAddDetached(ctx, dir, w, "HEAD"); err != nil {
						return 2, err
					}
					base, err := gitx.RevParse(ctx, dir, "HEAD")
					if err != nil {
						return 2, err
					}
					if err := gitx.CheckoutExisting(ctx, dir, base); err != nil {
						return 2, err
					}
					if err := writeAndStage(ctx, dir, "outside.txt", "made in the main worktree\n"); err != nil {
						return 2, err
					}
					if _, err := gitx.CreateCommit(ctx, dir, "main-worktree commit"); err != nil {
						return 2, err
					}
					x, err := gitx.RevParse(ctx, dir, "HEAD")
					if err != nil {
						return 2, err
					}
					if err := gitx.CheckoutExisting(ctx, dir, "main"); err != nil {
						return 2, err
					}
					if pointW {
						if err := gitx.CheckoutExisting(ctx, w, x); err != nil {
							return 2, err
						}
					}
					if _, err := gitx.FastForwardOnly(ctx, dir, x); err != nil {
						return 2, err
					}
					return 0, nil
				}}, decl)
				t.Logf("exit=%d err=%v outcome=%s verdicts:\n%s", res.Exit, res.Err, Outcome(res.Verdicts), formatVerdicts(res.Verdicts))
				if pointW && Outcome(res.Verdicts) == Pass {
					t.Errorf("FIRES: a commit logged in the main worktree, recording outside.txt under no_commit, passes as owned by the added worktree")
				}
			})
		}
	}
}
