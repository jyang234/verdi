package ritualwitness

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

// closureCases are the R2 closure checks' findings as cases: the
// hand-back exception never applies to a gone worktree or to a target the
// present worktree does not reach (CC-B1), nor when a commit was logged in
// the fixture's own checkout (RR-A7); and a carried foreign entry is
// outside under no_commit too (CC-B2).
func closureCases() []harnessCase {
	decl := ws.Declaration{Ritual: "test_closure", Verbs: []ws.Verb{ws.CLI("test closure")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
	unit := func(fx *Fixture) string { return filepath.Join(fx.Dir, ".verdi", "data", "worktrees", "unit") }
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
	fastForwardTo := func(rev func(context.Context, string) (string, error)) func(context.Context, string) error {
		return func(ctx context.Context, dir string) error {
			x, err := rev(ctx, dir)
			if err != nil {
				return err
			}
			_, err = gitx.FastForwardOnly(ctx, dir, x)
			return err
		}
	}
	return []harnessCase{
		{
			name: "a gone worktree never owns a commit the fixture reaches, not even by a logged fast-forward (CC-B1, CC P10)", states: []SeedState{SeedClean},
			decl: decl, setup: cleanRunway,
			driver: func(t *testing.T, fx *Fixture) Driver {
				scratch := filepath.Join(t.TempDir(), "index")
				var x string
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						base, err := gitx.RevParse(ctx, dir, "HEAD")
						if err != nil {
							return err
						}
						return gitx.WorktreeAddDetached(ctx, dir, unit(fx), base)
					},
					func(ctx context.Context, _ string) error { return writeAndStage(ctx, unit(fx), "unit.txt", "agent\n") },
					func(ctx context.Context, _ string) error { return commitIndex(ctx, unit(fx)) },
					func(ctx context.Context, dir string) error { return gitx.WorktreeRemove(ctx, dir, unit(fx)) },
					func(ctx context.Context, dir string) (err error) {
						x, err = scratchCommit(ctx, dir, scratch, "outside.txt")
						return err
					},
					fastForwardTo(func(context.Context, string) (string, error) { return x, nil }),
				)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				// The gone worktree's own commit, which nothing in the fixture
				// reaches, stays its; the main checkout's commit the runway
				// was fast-forwarded to is judged.
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry outside.txt added"),
					v("working_tree", Within, "outside.txt created"+fileWrite),
					v("worktrees", Within, "commit "+commitRecording(t, res, "unit.txt")+" made in added worktree "+canonicalPath("", unit(fx))),
					v("stage_paths", Outside, "commit "+commitRecording(t, res, "outside.txt")+" recorded outside.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "the hand-back exception needs the present worktree to reach the fast-forward's target (CC-M1)", states: both(),
			decl: decl, setup: cleanRunway,
			driver: func(_ *testing.T, fx *Fixture) Driver {
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						return gitx.WorktreeAddDetached(ctx, dir, unit(fx), "HEAD")
					},
					func(ctx context.Context, _ string) error {
						return writeAndStage(ctx, unit(fx), "eval.txt", "evaluated\n")
					},
					func(ctx context.Context, _ string) error { return commitIndex(ctx, unit(fx)) },
					// A child of the worktree's commit, made with plain git in
					// the main checkout: the worktree does not reach it.
					fastForwardTo(func(ctx context.Context, dir string) (string, error) {
						w, err := gitx.RevParse(ctx, unit(fx), "HEAD")
						if err != nil {
							return "", err
						}
						return plainGit(ctx, dir, "commit-tree", "-p", w, "-m", "beyond the worktree", w+"^{tree}")
					}),
				)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry eval.txt added"),
					v("working_tree", Within, "eval.txt created"+fileWrite),
					v("worktrees", Within, "worktree "+canonicalPath("", unit(fx))+" added"),
					v("stage_paths", Outside, "commit "+commitRecording(t, res, "eval.txt")+" recorded eval.txt"),
					v("stage_paths", Unattributable, "commit "+createdWithParentIn(t, res)+" recorded no file"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "a commit logged in the main checkout is never owned through the hand-back exception (RR-A7, A CL1)", states: both(),
			decl: decl, setup: cleanRunway,
			driver: func(_ *testing.T, fx *Fixture) Driver {
				var x string
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						return gitx.WorktreeAddDetached(ctx, dir, unit(fx), "HEAD")
					},
					func(ctx context.Context, dir string) error {
						base, err := gitx.RevParse(ctx, dir, "HEAD")
						if err != nil {
							return err
						}
						return gitx.CheckoutExisting(ctx, dir, base)
					},
					stage("outside.txt", "made in the main worktree\n"), commitIndex,
					func(ctx context.Context, dir string) (err error) {
						x, err = gitx.RevParse(ctx, dir, "HEAD")
						return err
					},
					checkout("main"),
					func(ctx context.Context, _ string) error { return gitx.CheckoutExisting(ctx, unit(fx), x) },
					fastForwardTo(func(context.Context, string) (string, error) { return x, nil }),
				)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("index", Within, "index entry outside.txt added"),
					v("working_tree", Within, "outside.txt created"+fileWrite),
					v("worktrees", Within, "worktree "+canonicalPath("", unit(fx))+" added"),
					v("stage_paths", Outside, "commit "+onlyCommit(t, res)+" recorded outside.txt"),
					v("index_carry", Outside, "declares no_commit; observed scoped"),
				}, Fail
			},
		},
		{
			name: "an owned commit carrying the foreign entry fails no_commit (CC-B2, CC P9)", states: []SeedState{SeedFull},
			decl: decl,
			driver: func(t *testing.T, fx *Fixture) Driver {
				aside := t.TempDir()
				move := func(from, to string) func(context.Context, string) error {
					return func(context.Context, string) error { return os.Rename(from, to) }
				}
				return InProcess{Fn: steps(
					func(ctx context.Context, dir string) error {
						return gitx.WorktreeAddDetached(ctx, dir, unit(fx), "HEAD")
					},
					func(_ context.Context, dir string) error {
						b, err := os.ReadFile(filepath.Join(dir, ForeignFile))
						if err != nil {
							return err
						}
						return os.WriteFile(filepath.Join(unit(fx), ForeignFile), b, 0o644)
					},
					func(ctx context.Context, _ string) error {
						_, err := plainGit(ctx, unit(fx), "add", ForeignFile)
						return err
					},
					func(ctx context.Context, _ string) error {
						_, err := plainGit(ctx, unit(fx), "commit", "--quiet", "-m", "agent work")
						return err
					},
					plain("rm", "--cached", "-q", "--", ForeignFile),
					plain("update-index", "--assume-unchanged", TrackedFile),
					move(filepath.Join(fx.Dir, ForeignFile), filepath.Join(aside, "f")),
					move(filepath.Join(fx.Dir, UntrackedFile), filepath.Join(aside, "u")),
					fastForwardTo(func(ctx context.Context, _ string) (string, error) { return gitx.RevParse(ctx, unit(fx), "HEAD") }),
					move(filepath.Join(aside, "u"), filepath.Join(fx.Dir, UntrackedFile)),
					plain("update-index", "--no-assume-unchanged", TrackedFile),
				)}
			},
			want: func(t *testing.T, fx *Fixture, res Result) ([]Verdict, RunOutcome) {
				w := canonicalPath("", unit(fx))
				return []Verdict{
					v("refs_move", Within, "refs/heads/main moved"),
					v("worktrees", Within, "worktree "+w+" added"),
					v("worktrees", Within, "commit "+onlyCommit(t, res)+" made in added worktree "+w),
					v("index_carry", Outside, "declares no_commit; observed carried; carried foreign "+ForeignFile),
				}, Fail
			},
		},
	}
}

// scratchCommit makes, with plain git and a scratch index (so nothing is
// logged and the real index is untouched), a child of HEAD that adds path,
// and returns it.
func scratchCommit(ctx context.Context, dir, scratch, path string) (string, error) {
	git := func(args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+scratch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", errWithOutput(err, out)
		}
		return strings.TrimSpace(string(out)), nil
	}
	if _, err := git("read-tree", "HEAD"); err != nil {
		return "", err
	}
	blob, err := git("hash-object", "-w", filepath.Join(dir, "owned", "keep.txt"))
	if err != nil {
		return "", err
	}
	if _, err := git("update-index", "--add", "--cacheinfo", "100644,"+blob+","+path); err != nil {
		return "", err
	}
	tree, err := git("write-tree")
	if err != nil {
		return "", err
	}
	return git("commit-tree", tree, "-p", "HEAD", "-m", "made in main")
}
