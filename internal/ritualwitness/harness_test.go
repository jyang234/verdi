package ritualwitness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// writeAndStage writes content to dir/path and stages it (git add -- path),
// the shape every synthetic ritual below uses for the one file it itself
// owns.
func writeAndStage(ctx context.Context, dir, path, content string) error {
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return err
	}
	return gitx.AddPaths(ctx, dir, path)
}

// --- synthetic rituals ------------------------------------------------
//
// Each is an in-process Ritual (driver.go), run through InProcess so its
// gitx.Observer is always attached. None of them is a real ritual (that is
// spec/ritual-effect-witness's R3 lane); each exists only to exercise one
// of the obligation's required verdicts.

// ritualScoped stays within its declaration: a scoped branch-and-commit.
func ritualScoped(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/scoped-ok"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/new.txt", "new\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "add the owned file", "owned/new.txt"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualStagesOutside commits TrackedFile (the dirty tracked file) alongside
// its own owned path, staging outside its declared paths.
func ritualStagesOutside(ctx context.Context, dir string) (int, error) {
	full := filepath.Join(dir, "owned", "new2.txt")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 2, err
	}
	if err := os.WriteFile(full, []byte("new2\n"), 0o644); err != nil {
		return 2, err
	}
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/outside"); err != nil {
		return 2, err
	}
	if err := gitx.AddPaths(ctx, dir, "owned/new2.txt", TrackedFile); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "oops, tracked.txt too", "owned/new2.txt", TrackedFile); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualMovesUndeclaredRef never switches branches, but its commit still
// advances "main" — a move its declaration names a different ref for.
func ritualMovesUndeclaredRef(ctx context.Context, dir string) (int, error) {
	if err := writeAndStage(ctx, dir, "owned/x.txt", "x\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "commit on the current branch", "owned/x.txt"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualCarriesForeign declares scoped but commits with no pathspec, so the
// pre-staged foreign entry rides into its commit.
func ritualCarriesForeign(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/carries"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/c.txt", "c\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommit(ctx, dir, "whole index, oops"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualScopedButRefuses guards on a pre-staged entry (refusing before any
// mutation) and otherwise makes a genuinely scoped commit.
func ritualScopedButRefuses(ctx context.Context, dir string) (int, error) {
	staged, err := gitx.StagedPaths(ctx, dir)
	if err != nil {
		return 2, err
	}
	if len(staged) > 0 {
		return 2, errors.New("refusing: a foreign entry is already staged")
	}
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/guard-then-scope"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/f.txt", "f\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "scoped after the guard", "owned/f.txt"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualPushesUnduly commits and pushes, though its declaration never says
// it may push.
func ritualPushesUnduly(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/push-bug"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/p.txt", "p\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "about to push without leave", "owned/p.txt"); err != nil {
		return 2, err
	}
	if err := gitx.Push(ctx, dir); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualSwitchesHeadUnduly switches to an existing branch and does nothing
// else, though its declaration never says HEAD may switch.
func ritualSwitchesHeadUnduly(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutExisting(ctx, dir, SideBranch); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualCommitsUnduly creates a branch and a commit, though its declaration
// says it never commits at all.
func ritualCommitsUnduly(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/no-commit-bug"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/w.txt", "w\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "should never have committed", "owned/w.txt"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualUntrackedEnters guards like ritualScopedButRefuses, and when it
// proceeds also stages UntrackedFile alongside its own owned path.
func ritualUntrackedEnters(ctx context.Context, dir string) (int, error) {
	staged, err := gitx.StagedPaths(ctx, dir)
	if err != nil {
		return 2, err
	}
	if len(staged) > 0 {
		return 2, errors.New("refusing: a foreign entry is already staged")
	}
	if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/untracked-ok"); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/z.txt", "z\n"); err != nil {
		return 2, err
	}
	if err := gitx.AddPaths(ctx, dir, UntrackedFile); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "owned plus the operator's untracked file", "owned/z.txt", UntrackedFile); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualChecksOutThenCommitsElsewhere switches to SideBranch (not the
// branch checked out before the ritual ran) and commits there, moving
// SideBranch rather than the pre-ritual checked-out branch RefCheckedOut
// names.
func ritualChecksOutThenCommitsElsewhere(ctx context.Context, dir string) (int, error) {
	if err := gitx.CheckoutExisting(ctx, dir, SideBranch); err != nil {
		return 2, err
	}
	if err := writeAndStage(ctx, dir, "owned/d.txt", "d\n"); err != nil {
		return 2, err
	}
	if _, err := gitx.CreateCommitPaths(ctx, dir, "commit on side, not main", "owned/d.txt"); err != nil {
		return 2, err
	}
	return 0, nil
}

// ritualBypassesGitx creates a ref with a bare os/exec call, never through
// gitx, so no gitx.Observer ever sees it — a mutation made outside gitx
// (parent dc-2), which the command log cannot explain regardless of
// whether the resulting ref happens to match the declaration.
func ritualBypassesGitx(ctx context.Context, dir string) (int, error) {
	head, err := gitx.RevParse(ctx, dir, "HEAD")
	if err != nil {
		return 2, err
	}
	cmd := exec.CommandContext(ctx, "git", "update-ref", "refs/heads/ritual/sneaky", head)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return 2, errWithOutput(err, out)
	}
	return 0, nil
}

func errWithOutput(err error, out []byte) error {
	return errors.New(err.Error() + ": " + string(out))
}

// makeRitualAddsUndeclaredWorktree returns a Ritual that adds a detached
// worktree entirely outside the fixture's own directory tree — needs t for
// a second TempDir, so it is built per-test rather than a package-level
// function.
func makeRitualAddsUndeclaredWorktree(t *testing.T) Ritual {
	return func(ctx context.Context, dir string) (int, error) {
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return 2, err
		}
		// Resolve t.TempDir() itself (it exists) before joining the new
		// worktree's own path onto it: on macOS t.TempDir() sits under a
		// /var/folders symlink, and `git worktree add` records the
		// resolved /private/var/folders form in `git worktree list
		// --porcelain` (gitx.WorktreeEntry's own doc comment). Passing the
		// unresolved form here would make the logged argv's path differ,
		// byte for byte, from what the harness's own sensor observes —
		// a real attribution mismatch, not the one this case means to
		// demonstrate.
		base, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			return 2, err
		}
		outside := filepath.Join(base, "outside-wt")
		if err := gitx.WorktreeAddDetached(ctx, dir, outside, head); err != nil {
			return 2, err
		}
		return 0, nil
	}
}

// --- assertion helpers --------------------------------------------------

func formatVerdicts(vs []Verdict) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString("  " + v.String() + "\n")
	}
	if b.Len() == 0 {
		return "  (no verdicts)\n"
	}
	return b.String()
}

func requireVerdict(t *testing.T, vs []Verdict, field string, status Status, substr string) {
	t.Helper()
	for _, v := range vs {
		if v.Field == field && v.Status == status && (substr == "" || strings.Contains(v.Detail, substr)) {
			return
		}
	}
	t.Fatalf("no %s verdict with status %s (detail containing %q) among:\n%s", field, status, substr, formatVerdicts(vs))
}

func requireAllWithin(t *testing.T, vs []Verdict) {
	t.Helper()
	if len(vs) == 0 {
		t.Fatal("expected at least one verdict, got none")
	}
	for _, v := range vs {
		if v.Status != Within {
			t.Fatalf("expected every verdict within, got:\n%s", formatVerdicts(vs))
		}
	}
}

// --- TestHarness_SensorsAndVerdicts --------------------------------------

// TestHarness_SensorsAndVerdicts is obligation/ritual-effect-witness--ac-1--
// behavioral's producer. Over the synthetic rituals above: a ritual that
// stays within its declaration passes; one that stages outside its
// declared paths, moves an undeclared ref, adds an undeclared worktree,
// pushes outside may-push, switches HEAD outside head-switch, commits when
// no_commit is declared, or carries a foreign entry while declaring scoped
// fails naming the effect; one that refuses while declaring scoped fails;
// and an effect the sensors cannot attribute is reported unattributable,
// never within scope. Every case runs through InProcess against a fresh
// fixturegit repository with a local bare remote (no network).
func TestHarness_SensorsAndVerdicts(t *testing.T) {
	ctx := context.Background()

	// mutantUnattributableCase and mutantCheckedOutCase/mutantCheckedOutDecl
	// carry real Snapshot/CommandLog pairs captured by two subtests below
	// into the mutants subtest at the end, so those two mutants are
	// demonstrated against real git state rather than hand-built data —
	// local to this test function, never package state (ground-rules: no
	// package-level mutable state).
	var (
		mutantUnattributableCase Result
		mutantUnattributableDecl ws.Declaration
		mutantCheckedOutCase     Result
		mutantCheckedOutDecl     ws.Declaration
	)

	t.Run("within declaration passes, in both seeded states", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_scoped",
			Verbs:      []ws.Verb{ws.CLI("test scoped")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		for _, state := range []SeedState{SeedFull, SeedClean} {
			t.Run(state.String(), func(t *testing.T) {
				res := Run(t, ctx, InProcess{Fn: ritualScoped}, decl, state)
				if res.Exit != 0 {
					t.Fatalf("exit = %d, want 0", res.Exit)
				}
				requireAllWithin(t, res.Verdicts)
				requireVerdict(t, res.Verdicts, "index_carry", Within, "observed scoped")
			})
		}
	})

	t.Run("stages outside its declared paths fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_outside",
			Verbs:      []ws.Verb{ws.CLI("test outside")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		res := Run(t, ctx, InProcess{Fn: ritualStagesOutside}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "stage_paths", Outside, TrackedFile)
	})

	t.Run("moves an undeclared ref fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_moves_undeclared",
			Verbs:      []ws.Verb{ws.CLI("test moves-undeclared")},
			RefsMove:   []ws.RefPattern{"refs/heads/decoy"},
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		res := Run(t, ctx, InProcess{Fn: ritualMovesUndeclaredRef}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "refs_move", Outside, "refs/heads/main")
	})

	t.Run("RefCheckedOut names only the branch checked out before the ritual ran", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_checked_out",
			Verbs:      []ws.Verb{ws.CLI("test checked-out")},
			RefsMove:   []ws.RefPattern{ws.RefCheckedOut},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		res := Run(t, ctx, InProcess{Fn: ritualChecksOutThenCommitsElsewhere}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "refs_move", Outside, "refs/heads/side")
		requireVerdict(t, res.Verdicts, "head_switch", Within, "")
		mutantCheckedOutCase = res // captured for the RefCheckedOut mutant, below
		mutantCheckedOutDecl = decl
	})

	t.Run("adds an undeclared worktree fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_undeclared_worktree",
			Verbs:      []ws.Verb{ws.CLI("test undeclared-worktree")},
			Worktrees:  []ws.WorktreePattern{".verdi/data/worktrees/*"},
			IndexCarry: ws.CarryNoCommit,
		}
		res := Run(t, ctx, InProcess{Fn: makeRitualAddsUndeclaredWorktree(t)}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "worktrees", Outside, "worktree added")
	})

	t.Run("carries a foreign entry while declaring scoped fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_carries_foreign",
			Verbs:      []ws.Verb{ws.CLI("test carries-foreign")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		res := Run(t, ctx, InProcess{Fn: ritualCarriesForeign}, decl, SeedFull)
		requireVerdict(t, res.Verdicts, "index_carry", Outside, ForeignFile)
	})

	t.Run("refuses while declaring scoped fails; the same ritual passes in the clean state", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_guard_then_scope",
			Verbs:      []ws.Verb{ws.CLI("test guard-then-scope")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		full := Run(t, ctx, InProcess{Fn: ritualScopedButRefuses}, decl, SeedFull)
		if full.Exit != 2 {
			t.Fatalf("full-state exit = %d, want 2 (the guard must refuse)", full.Exit)
		}
		requireVerdict(t, full.Verdicts, "index_carry", Outside, "declares scoped")

		clean := Run(t, ctx, InProcess{Fn: ritualScopedButRefuses}, decl, SeedClean)
		if clean.Exit != 0 {
			t.Fatalf("clean-state exit = %d, want 0 (nothing to refuse)", clean.Exit)
		}
		requireAllWithin(t, clean.Verdicts)
	})

	t.Run("an effect the sensors cannot attribute is reported unattributable, never within", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_bypasses_gitx",
			Verbs:      []ws.Verb{ws.CLI("test bypasses-gitx")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, // matches the sneaky ref's namespace on purpose
			IndexCarry: ws.CarryNoCommit,
		}
		res := Run(t, ctx, InProcess{Fn: ritualBypassesGitx}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "refs_create", Unattributable, "ritual/sneaky")
		for _, v := range res.Verdicts {
			if v.Field == "refs_create" && strings.Contains(v.Detail, "ritual/sneaky") && v.Status == Within {
				t.Fatalf("the unattributable ref was counted within scope: %s", v)
			}
		}
		mutantUnattributableCase = res // captured for the "treat unattributable as within" mutant, below
		mutantUnattributableDecl = decl
	})

	t.Run("pushes outside may-push fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_push_bug",
			Verbs:      []ws.Verb{ws.CLI("test push-bug")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			StagePaths: []ws.PathPattern{"owned/*"},
			IndexCarry: ws.CarryScoped,
		}
		res := Run(t, ctx, InProcess{Fn: ritualPushesUnduly}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "may_push", Outside, "")
	})

	t.Run("HEAD switched without head-switch fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_head_switch_bug",
			Verbs:      []ws.Verb{ws.CLI("test head-switch-bug")},
			Worktrees:  []ws.WorktreePattern{ws.WorktreeRegistered}, // declares something, so Validate-shaped
			IndexCarry: ws.CarryNoCommit,
		}
		res := Run(t, ctx, InProcess{Fn: ritualSwitchesHeadUnduly}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "head_switch", Outside, "main")
	})

	t.Run("a commit when no_commit is declared fails naming the effect", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:     "test_no_commit_bug",
			Verbs:      []ws.Verb{ws.CLI("test no-commit-bug")},
			RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true,
			IndexCarry: ws.CarryNoCommit,
		}
		res := Run(t, ctx, InProcess{Fn: ritualCommitsUnduly}, decl, SeedClean)
		requireVerdict(t, res.Verdicts, "index_carry", Outside, "declares no_commit")
	})

	t.Run("an untracked file may enter under untracked_may_enter, in both seeded states", func(t *testing.T) {
		decl := ws.Declaration{
			Ritual:            "test_untracked_enters",
			Verbs:             []ws.Verb{ws.CLI("test untracked-enters")},
			RefsCreate:        []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch:        true,
			StagePaths:        []ws.PathPattern{"owned/*"},
			IndexCarry:        ws.CarryRefused,
			UntrackedMayEnter: true,
		}
		clean := Run(t, ctx, InProcess{Fn: ritualUntrackedEnters}, decl, SeedClean)
		if clean.Exit != 0 {
			t.Fatalf("clean-state exit = %d, want 0", clean.Exit)
		}
		requireVerdict(t, clean.Verdicts, "untracked_may_enter", Within, UntrackedFile)
		requireAllWithin(t, clean.Verdicts)

		full := Run(t, ctx, InProcess{Fn: ritualUntrackedEnters}, decl, SeedFull)
		if full.Exit != 2 {
			t.Fatalf("full-state exit = %d, want 2 (the guard must refuse)", full.Exit)
		}
		requireAllWithin(t, full.Verdicts)
	})

	t.Run("mutants: each is killed by this test's own assertions", func(t *testing.T) {
		testHarnessMutants(t, mutantUnattributableCase, mutantUnattributableDecl, mutantCheckedOutCase, mutantCheckedOutDecl)
	})
}
