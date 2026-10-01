package ritualwitness

// Reviewer R2-A probes, copied from the lane R2 review (half A,
// zz_probe_test.go, sha1 26d4fc6f) and adapted only to the harness's
// changed API: Evaluate takes no store root and returns an error; Result
// has no StoreRoot; a Snapshot's commits carry their own file lists. Every
// behavioral assertion is kept, except P9b's, which the controller ruled
// (2026-10-01) a pinned SI-325 (8) residual: it now asserts today's
// within result and names what closes it. Each logs its verdicts;
// t.Errorf marks a WRONG result (a probe "fires" when the harness is
// wrong).

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

func zzGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func zzAllWithin(vs []Verdict) bool {
	for _, v := range vs {
		if v.Status != Within {
			return false
		}
	}
	return len(vs) > 0
}

func zzHas(vs []Verdict, field string, st Status, sub string) bool {
	for _, v := range vs {
		if v.Field == field && v.Status == st && strings.Contains(v.Detail, sub) {
			return true
		}
	}
	return false
}

// manual run: build, optional pre-ritual setup, capture, drive, capture, evaluate.
func zzRun(t *testing.T, state SeedState, setup func(fx *Fixture), d Driver, decl ws.Declaration) (Result, *Fixture) {
	t.Helper()
	ctx := context.Background()
	fx := Build(t, ctx, state)
	if setup != nil {
		setup(fx)
	}
	before, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("before: %v", err)
	}
	exit, log, _ := d.Run(ctx, fx.Dir)
	after, err := Capture(ctx, fx.Dir, fx.Bare)
	if err != nil {
		t.Fatalf("after: %v", err)
	}
	verdicts, err := Evaluate(decl, exit, before, after, log)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	res := Result{Exit: exit, Log: log, Before: before, After: after, Verdicts: verdicts}
	t.Logf("exit=%d verdicts:\n%s", exit, formatVerdicts(res.Verdicts))
	return res, fx
}

func TestZZ_P1_StagingOnlyOutsideIsSilent(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p1", Verbs: []ws.Verb{ws.CLI("p one")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	if err := decl.Validate(); err != nil {
		t.Fatal(err)
	}
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p1"); err != nil {
			return 2, err
		}
		return 0, gitx.AddAll(ctx, dir) // stages the operator's dirty + untracked files
	}}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	t.Logf("index before=%d after=%d", len(res.Before.Index), len(res.After.Index))
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: ritual staged %s and %s (outside every declared stage path) and every verdict is within", TrackedFile, UntrackedFile)
	}
}

func TestZZ_P1c_WorkingTreeClobberIsSilent(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p1c", Verbs: []ws.Verb{ws.CLI("p onec")},
		RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/p1c"); err != nil {
			return 2, err
		}
		// discard the operator's dirty tracked work and delete their untracked file
		if err := os.WriteFile(filepath.Join(dir, TrackedFile), []byte("original content\n"), 0o644); err != nil {
			return 2, err
		}
		return 0, os.Remove(filepath.Join(dir, UntrackedFile))
	}}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: operator's dirty tracked work and untracked file destroyed; every verdict within")
	}
}

func TestZZ_P2_CommitOnCheckedOutBranchReportsHeadSwitch(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p2", Verbs: []ws.Verb{ws.CLI("p two")},
		RefsMove: []ws.RefPattern{ws.RefCheckedOut}, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
	if err := decl.Validate(); err != nil {
		t.Fatal(err)
	}
	res := Run(t, ctx, InProcess{Fn: ritualMovesUndeclaredRef}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	if zzHas(res.Verdicts, "head_switch", Outside, "") {
		t.Errorf("FIRES: a commit on the checked-out branch (refs_move @checked-out, HEAD never switched) is reported head_switch outside")
	}
}

func TestZZ_P3_OrphanedCommitInvisible(t *testing.T) {
	ritual := func(ctx context.Context, dir string) (int, error) {
		base, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return 2, err
		}
		if err := gitx.CheckoutExisting(ctx, dir, base); err != nil { // detach
			return 2, err
		}
		if err := writeAndStage(ctx, dir, "owned/o.txt", "o\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommit(ctx, dir, "whole index on a detached HEAD"); err != nil {
			return 2, err
		}
		if err := gitx.CheckoutExisting(ctx, dir, "main"); err != nil {
			return 2, err
		}
		return 0, nil
	}
	t.Run("no_commit/clean", func(t *testing.T) {
		decl := ws.Declaration{Ritual: "p3", Verbs: []ws.Verb{ws.CLI("p three")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
		res, _ := zzRun(t, SeedClean, nil, InProcess{Fn: ritual}, decl)
		if zzAllWithin(res.Verdicts) {
			t.Errorf("FIRES: a commit object was created while no_commit is declared; every verdict within")
		}
	})
	t.Run("scoped/full carries foreign then orphans", func(t *testing.T) {
		decl := ws.Declaration{Ritual: "p3b", Verbs: []ws.Verb{ws.CLI("p threeb")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
			HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
		res, fx := zzRun(t, SeedFull, nil, InProcess{Fn: ritual}, decl)
		out, _ := zzGit(context.Background(), fx.Dir, "reflog", "-n", "4", "--format=%H %gs")
		t.Logf("HEAD reflog:\n%s", out)
		ls, _ := zzGit(context.Background(), fx.Dir, "status", "--porcelain")
		t.Logf("status after:\n%s", ls)
		if zzAllWithin(res.Verdicts) {
			t.Errorf("FIRES: scoped ritual committed the foreign entry (now reachable only from the reflog) and every verdict is within")
		}
	})
}

func TestZZ_P4_TempMatchesRemovalOfPreRegisteredOperatorWorktree(t *testing.T) {
	ctx := context.Background()
	var wt string
	setup := func(fx *Fixture) {
		base, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		wt = filepath.Join(base, "operators-own-worktree")
		if out, err := zzGit(ctx, fx.Dir, "worktree", "add", "--detach", wt, "HEAD"); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	decl := ws.Declaration{Ritual: "p4", Verbs: []ws.Verb{ws.CLI("p four")}, Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryNoCommit}
	res, _ := zzRun(t, SeedClean, setup, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		return 0, gitx.WorktreeRemove(ctx, dir, wt)
	}}, decl)
	if zzHas(res.Verdicts, "worktrees", Within, "removed") {
		t.Errorf("FIRES: removing a worktree registered BEFORE the ritual is within @temp (a fresh temporary directory)")
	}
}

func TestZZ_P5_InStoreWorktreeVsStoreRoot(t *testing.T) {
	ctx := context.Background()
	mk := func(rel string) Ritual {
		return func(ctx context.Context, dir string) (int, error) {
			head, err := gitx.RevParse(ctx, dir, "HEAD")
			if err != nil {
				return 2, err
			}
			root, err := filepath.EvalSymlinks(dir) // what git's --show-toplevel gives a real ritual
			if err != nil {
				return 2, err
			}
			return 0, gitx.WorktreeAddDetached(ctx, dir, filepath.Join(root, rel), head)
		}
	}
	ignore := func(fx *Fixture) { // a real store ignores .verdi/data/
		if err := os.WriteFile(filepath.Join(fx.Dir, ".git", "info", "exclude"), []byte(".verdi/data/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("@temp vs in-store worktree", func(t *testing.T) {
		decl := ws.Declaration{Ritual: "p5", Verbs: []ws.Verb{ws.CLI("p five")}, Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryNoCommit}
		res, fx := zzRun(t, SeedClean, ignore, InProcess{Fn: mk(".verdi/data/worktrees/w1")}, decl)
		t.Logf("storeRoot=%s worktrees after=%+v", fx.Dir, res.After.Worktrees)
		if zzHas(res.Verdicts, "worktrees", Within, "added") {
			t.Errorf("FIRES: a worktree INSIDE the repository matched @temp (outside the repository)")
		}
	})
	t.Run("dir pattern vs in-store worktree", func(t *testing.T) {
		decl := ws.Declaration{Ritual: "p6", Verbs: []ws.Verb{ws.CLI("p six")}, Worktrees: []ws.WorktreePattern{".verdi/data/worktrees/*"}, IndexCarry: ws.CarryNoCommit}
		res, _ := zzRun(t, SeedClean, ignore, InProcess{Fn: mk(".verdi/data/worktrees/w2")}, decl)
		if !zzHas(res.Verdicts, "worktrees", Within, "added") {
			t.Errorf("FIRES: a declared in-store worktree (.verdi/data/worktrees/*) is not within")
		}
	})
	t.Run("in-store worktree without an ignore rule", func(t *testing.T) {
		fx := Build(t, ctx, SeedClean)
		head, _ := zzGit(ctx, fx.Dir, "rev-parse", "HEAD")
		if out, err := zzGit(ctx, fx.Dir, "worktree", "add", "--detach", ".verdi/data/worktrees/w3", head); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		_, err := Capture(ctx, fx.Dir, fx.Bare)
		t.Logf("Capture with an in-store worktree and no ignore rule: err=%v", err)
		if err != nil {
			t.Errorf("FIRES (operational, not silent): the sensor cannot capture a fixture holding an in-store linked worktree: %v", err)
		}
	})
}

func TestZZ_P7_UnobservedGitStateClasses(t *testing.T) {
	decl := ws.Declaration{Ritual: "p7", Verbs: []ws.Verb{ws.CLI("p seven")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	res, fx := zzRun(t, SeedClean, nil, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		for _, a := range [][]string{
			{"tag", "v-light"},
			{"tag", "-a", "v-annot", "-m", "annotated"},
			{"config", "branch.main.remote", "origin"},
			{"config", "core.hooksPath", "/nonexistent/hooks"},
			{"update-ref", "refs/remotes/origin/sneaky", "HEAD"},
			{"update-ref", "refs/verdi/private", "HEAD"},
			{"push", "origin", "v-light"},
		} {
			if out, err := zzGit(ctx, dir, a...); err != nil {
				return 2, errWithOutput(err, []byte(out))
			}
		}
		if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "post-commit"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			return 2, err
		}
		return 0, os.WriteFile(filepath.Join(dir, ".git", "stray.txt"), []byte("x"), 0o644)
	}}, decl)
	_ = fx
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: tags (local + pushed), config, hooks, a local remote-tracking ref, a refs/verdi ref, and a .git file all changed; every verdict within")
	}
}

func TestZZ_P8_MergeCommitFileListEmpty(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p8", Verbs: []ws.Verb{ws.CLI("p eight")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
	setup := func(fx *Fixture) { // make side diverge so a true merge is possible
		tree, _ := zzGit(ctx, fx.Dir, "rev-parse", "HEAD^{tree}")
		c, err := zzGit(ctx, fx.Dir, "commit-tree", "-p", "side", "-m", "side diverges", tree)
		if err != nil {
			t.Fatal(c)
		}
		if out, err := zzGit(ctx, fx.Dir, "update-ref", "refs/heads/side", c); err != nil {
			t.Fatal(out)
		}
	}
	res, _ := zzRun(t, SeedClean, setup, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutNewBranch(ctx, dir, "ritual/merge"); err != nil {
			return 2, err
		}
		// a merge commit (git merge --no-ff shape) whose tree records outside.txt
		if out, err := zzGit(ctx, dir, "merge", "--no-ff", "--no-commit", "side"); err != nil {
			return 2, errWithOutput(err, []byte(out))
		}
		if err := writeAndStage(ctx, dir, "outside.txt", "outside\n"); err != nil {
			return 2, err
		}
		if _, err := gitx.CreateCommit(ctx, dir, "merge side"); err != nil {
			return 2, err
		}
		return 0, nil
	}}, decl)
	for sha, c := range res.After.Commits {
		if _, old := res.Before.Commits[sha]; !old {
			t.Logf("new commit %s files=%v", short(sha), c.Files)
		}
	}
	if !zzHas(res.Verdicts, "stage_paths", Outside, "outside.txt") {
		t.Errorf("FIRES: a merge commit recorded outside.txt and no stage_paths outside verdict names it")
	}
}

func TestZZ_P9_ReadOnlyMentionAttributesOutOfGitxMutation(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p9", Verbs: []ws.Verb{ws.CLI("p nine")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"}, IndexCarry: ws.CarryNoCommit}
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if code, err := ritualBypassesGitx(ctx, dir); err != nil {
			return code, err
		}
		_, err := gitx.RevParse(ctx, dir, "ritual/sneaky") // read-only read-back through gitx
		return 0, err
	}}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	if zzHas(res.Verdicts, "refs_create", Within, "ritual/sneaky") {
		t.Errorf("FIRES: a ref created outside gitx is counted within because a read-only gitx call named it")
	}
}

func TestZZ_P9b_FailedCommitAttributesOutOfGitxMove(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p9b", Verbs: []ws.Verb{ws.CLI("p nineb")}, RefsMove: []ws.RefPattern{ws.RefCheckedOut},
		StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		_, _ = gitx.CreateCommitPaths(ctx, dir, "nothing to commit: fails", "owned/keep.txt") // logged before exec, fails
		// the real move happens outside gitx
		if err := os.WriteFile(filepath.Join(dir, "owned", "q.txt"), []byte("q\n"), 0o644); err != nil {
			return 2, err
		}
		for _, a := range [][]string{{"add", "owned/q.txt"}} {
			if out, err := zzGit(ctx, dir, a...); err != nil {
				return 2, errWithOutput(err, []byte(out))
			}
		}
		tree, err := zzGit(ctx, dir, "write-tree")
		if err != nil {
			return 2, err
		}
		c, err := zzGit(ctx, dir, "commit-tree", tree, "-p", "HEAD", "-m", "outside gitx")
		if err != nil {
			return 2, err
		}
		if out, err := zzGit(ctx, dir, "update-ref", "refs/heads/main", c); err != nil {
			return 2, errWithOutput(err, []byte(out))
		}
		return 0, nil
	}}, decl, SeedClean)
	t.Logf("log=%v", res.Log.Calls)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	// Pinned disclosed residual (controller ruling, 2026-10-01): SI-325 (8)
	// credits this failed, logged `git commit` with the move made outside
	// gitx. The probe now asserts that within result, so closing the
	// residual turns it red.
	if !zzHas(res.Verdicts, "refs_move", Within, "refs/heads/main") {
		t.Errorf("PINNED DISCLOSURE changed: ledger SI-325 (8)'s residual (a failed logged `git commit` credited with a branch move made outside gitx) no longer reads within; if spec/gitx-recorder-seam ac-1 closed it, update doc.go's residual paragraph and this probe")
	}
}

func TestZZ_P10_FullRefnameArgvIsUnattributable(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p10", Verbs: []ws.Verb{ws.CLI("p ten")}, RefsCreate: []ws.RefPattern{"refs/heads/design/*"}, IndexCarry: ws.CarryNoCommit}
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return 2, err
		}
		return 0, gitx.UpdateRef(ctx, dir, "refs/heads/design/x", head) // the real stub-instantiate primitive today
	}}, decl, SeedClean)
	t.Logf("log=%v", res.Log.Calls)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
	if zzHas(res.Verdicts, "refs_create", Unattributable, "design/x") {
		t.Logf("NOTE (fails toward unattributable): gitx.UpdateRef's full refname argv does not attribute its own ref create")
	}
}

func TestZZ_P10b_RelativeAndSymlinkWorktreeArgv(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p10b", Verbs: []ws.Verb{ws.CLI("p tenb")}, Worktrees: []ws.WorktreePattern{ws.WorktreeTemp}, IndexCarry: ws.CarryNoCommit}
	unresolved := t.TempDir() // /var/folders on macOS
	res := Run(t, ctx, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		head, err := gitx.RevParse(ctx, dir, "HEAD")
		if err != nil {
			return 2, err
		}
		if err := gitx.WorktreeAddDetached(ctx, dir, filepath.Join(unresolved, "wt-sym"), head); err != nil {
			return 2, err
		}
		return 0, gitx.WorktreeAddDetached(ctx, dir, "../wt-rel-"+filepath.Base(unresolved), head)
	}}, decl, SeedClean)
	t.Logf("verdicts:\n%s", formatVerdicts(res.Verdicts))
}

func TestZZ_P11_ShortHeadAmbiguity(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p11", Verbs: []ws.Verb{ws.CLI("p eleven")}, RefsMove: []ws.RefPattern{ws.RefCheckedOut},
		HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
	res, _ := zzRun(t, SeedClean, func(fx *Fixture) {
		if out, err := zzGit(ctx, fx.Dir, "tag", "main"); err != nil {
			t.Fatal(out)
		}
	}, InProcess{Fn: ritualMovesUndeclaredRef}, decl)
	t.Logf("before head=%+v", res.Before.Head)
	if !zzHas(res.Verdicts, "refs_move", Within, "refs/heads/main") {
		t.Errorf("FIRES (toward unproven/outside): a tag named like the branch breaks Head.Branch and so @checked-out")
	}
}

type zzNoLog struct{ fn Ritual }

func (d zzNoLog) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	exit, err := d.fn(ctx, dir)
	return exit, CommandLog{}, err
}

func TestZZ_P12_UnavailableLogLeavesNoVerdict(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p12", Verbs: []ws.Verb{ws.CLI("p twelve")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryRefused}
	res := Run(t, ctx, zzNoLog{fn: ritualScopedButRefuses}, decl, SeedFull)
	t.Logf("exit=%d logOK=%v verdicts:\n%s", res.Exit, res.Log.OK, formatVerdicts(res.Verdicts))
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: no verdict discloses that the command log was unavailable; the run reads all-within")
	}
	res2 := Run(t, ctx, zzNoLog{fn: ritualScoped}, decl, SeedClean)
	t.Logf("clean exit=%d verdicts:\n%s", res2.Exit, formatVerdicts(res2.Verdicts))
}

func TestZZ_P13_QuotedPathForeignCarryMislabelled(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p13", Verbs: []ws.Verb{ws.CLI("p thirteen")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped}
	res, _ := zzRun(t, SeedClean, func(fx *Fixture) {
		writeFixtureFile(t, fx.Dir, "föreign.txt", "colleague\n")
		if out, err := zzGit(ctx, fx.Dir, "add", "--", "föreign.txt"); err != nil {
			t.Fatal(out)
		}
	}, InProcess{Fn: ritualCarriesForeign}, decl)
	if !zzHas(res.Verdicts, "index_carry", Outside, "") {
		t.Errorf("FIRES: a foreign entry with a non-ASCII name was carried and index_carry is within (observed scoped)")
	}
}

func TestZZ_P14_PreexistingLinkedWorktreeEffectsSilent(t *testing.T) {
	ctx := context.Background()
	var wt string
	setup := func(fx *Fixture) {
		base, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		wt = filepath.Join(base, "operator-wt")
		if out, err := zzGit(ctx, fx.Dir, "worktree", "add", "--detach", wt, "HEAD"); err != nil {
			t.Fatalf("%v %s", err, out)
		}
		writeFixtureFile(t, wt, "op-staged.txt", "operator staged in their worktree\n")
		if out, err := zzGit(ctx, wt, "add", "op-staged.txt"); err != nil {
			t.Fatal(out)
		}
	}
	decl := ws.Declaration{Ritual: "p14", Verbs: []ws.Verb{ws.CLI("p fourteen")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	res, _ := zzRun(t, SeedClean, setup, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if err := gitx.CheckoutExisting(ctx, wt, SideBranch); err != nil { // switch the operator's linked worktree
			return 2, err
		}
		if out, err := zzGit(ctx, wt, "rm", "--cached", "-q", "op-staged.txt"); err != nil { // unstage their work
			return 2, errWithOutput(err, []byte(out))
		}
		return 0, nil
	}}, decl)
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: an existing linked worktree's HEAD switched and its index changed; every verdict within")
	}
}

func TestZZ_P15_RemoteOnlyCommitGetsFileList(t *testing.T) {
	decl := ws.Declaration{Ritual: "p15", Verbs: []ws.Verb{ws.CLI("p fifteen")}, RefsCreate: []ws.RefPattern{"refs/heads/ritual/*"},
		HeadSwitch: true, StagePaths: []ws.PathPattern{"owned/*"}, IndexCarry: ws.CarryScoped, MayPush: true}
	res, _ := zzRun(t, SeedClean, nil, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		head, _ := gitx.RevParse(ctx, dir, "HEAD")
		if err := os.WriteFile(filepath.Join(dir, "outside-remote.txt"), []byte("r\n"), 0o644); err != nil {
			return 2, err
		}
		for _, a := range [][]string{{"add", "outside-remote.txt"}} {
			if out, err := zzGit(ctx, dir, a...); err != nil {
				return 2, errWithOutput(err, []byte(out))
			}
		}
		tree, _ := zzGit(ctx, dir, "write-tree")
		c, _ := zzGit(ctx, dir, "commit-tree", tree, "-p", head, "-m", "remote only")
		for _, a := range [][]string{{"reset", "-q"}, {"push", "-q", "origin", c + ":refs/heads/ritual/remote-only"}, {"update-ref", "-d", "refs/remotes/origin/ritual/remote-only"}} {
			if out, err := zzGit(ctx, dir, a...); err != nil {
				return 2, errWithOutput(err, []byte(out))
			}
		}
		return 0, nil
	}}, decl)
	if !zzHas(res.Verdicts, "stage_paths", Outside, "outside-remote.txt") {
		t.Errorf("FIRES: a commit reachable only from a new remote ref did not get its file list classified")
	}
}

func TestZZ_P16_RemoteRefDeletionSilent(t *testing.T) {
	ctx := context.Background()
	decl := ws.Declaration{Ritual: "p16", Verbs: []ws.Verb{ws.CLI("p sixteen")}, HeadSwitch: true, IndexCarry: ws.CarryNoCommit}
	res, _ := zzRun(t, SeedClean, func(fx *Fixture) {
		if out, err := zzGit(ctx, fx.Dir, "push", "-q", "origin", SideBranch); err != nil {
			t.Fatal(out)
		}
	}, InProcess{Fn: func(ctx context.Context, dir string) (int, error) {
		if out, err := zzGit(ctx, dir, "push", "-q", "origin", "--delete", SideBranch); err != nil {
			return 2, errWithOutput(err, []byte(out))
		}
		return 0, nil
	}}, decl)
	t.Logf("remote before=%v after=%v", res.Before.RemoteRefs, res.After.RemoteRefs)
	if zzAllWithin(res.Verdicts) {
		t.Errorf("FIRES: the remote's refs/heads/side was deleted (may_push=false) and every verdict is within")
	}
}
