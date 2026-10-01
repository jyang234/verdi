package ritualwitness

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/gitx"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// TestMutatingPrimitives_MatchTheClassification holds the attribution
// table to writescope's classification: exactly gitx's mutating functions,
// each once (SI-325 (8): attribution only to gitx's mutating primitives).
func TestMutatingPrimitives_MatchTheClassification(t *testing.T) {
	var want []string
	for _, f := range ws.MutatingFuncs(ws.Classification()) {
		if strings.HasPrefix(f, "internal/gitx.") {
			want = append(want, f)
		}
	}
	var got []string
	seen := map[string]bool{}
	for _, p := range mutatingPrimitives() {
		if seen[p.fn] {
			t.Errorf("%s is listed twice", p.fn)
		}
		seen[p.fn] = true
		got = append(got, p.fn)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutating primitives = %v\nwant writescope's gitx mutating functions %v", got, want)
	}
}

func TestSplitGlobal(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		args     []string
		wantDir  string
		wantArgs []string
		ok       bool
	}{
		{"no global options", "/r", []string{"commit", "-m", "x"}, "/r", []string{"commit", "-m", "x"}, true},
		{"a -c pair", "/r", []string{"-c", "status.showUntrackedFiles=all", "worktree", "remove", "p"}, "/r", []string{"worktree", "remove", "p"}, true},
		{"a flag", "/r", []string{"--literal-pathspecs", "rev-list", "x"}, "/r", []string{"rev-list", "x"}, true},
		{"a relative -C", "/r", []string{"-C", "sub", "add", "-A"}, "/r/sub", []string{"add", "-A"}, true},
		{"an absolute -C", "/r", []string{"-C", "/elsewhere", "add", "-A"}, "/elsewhere", []string{"add", "-A"}, true},
		{"a dangling -c", "/r", []string{"-c"}, "", nil, false},
		{"a dangling -C", "/r", []string{"-C"}, "", nil, false},
		{"no subcommand", "/r", []string{"--literal-pathspecs"}, "", nil, false},
		{"empty", "/r", nil, "", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, args, ok := splitGlobal(tt.dir, tt.args)
			if ok != tt.ok || dir != tt.wantDir || !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("splitGlobal = (%q, %q, %v), want (%q, %q, %v)", dir, args, ok, tt.wantDir, tt.wantArgs, tt.ok)
			}
		})
	}
}

func TestMutatingCall(t *testing.T) {
	prims := mutatingPrimitives()
	tests := []struct {
		name     string
		args     []string
		want     bool
		kind     primKind
		created  string
		deleted  string
		switched string
		target   string
	}{
		{"add -A", []string{"add", "-A"}, true, primStage, "", "", "", ""},
		{"add -- paths", []string{"add", "--", "a", "b"}, true, primStage, "", "", "", ""},
		{"checkout an existing branch", []string{"checkout", "side"}, true, primCheckout, "", "", "refs/heads/side", ""},
		{"checkout a commit", []string{"checkout", oidA}, true, primCheckout, "", "", "", ""},
		{"checkout -b", []string{"checkout", "-b", "design/x"}, true, primCheckoutNew, "refs/heads/design/x", "", "refs/heads/design/x", ""},
		{"checkout -b from a base", []string{"checkout", "-b", "feature/x", "origin/main"}, true, primCheckoutNew, "refs/heads/feature/x", "", "refs/heads/feature/x", ""},
		{"commit -m", []string{"commit", "-m", "msg"}, true, primCommit, "", "", "", ""},
		{"commit -m -- paths", []string{"commit", "-m", "msg", "--", "a"}, true, primCommit, "", "", "", ""},
		{"commit-tree", []string{"commit-tree", oidA, "-p", oidB, "-m", "msg"}, true, primCommitTree, "", "", "", ""},
		{"branch -d", []string{"branch", "-d", "close/x"}, true, primBranchDelete, "", "refs/heads/close/x", "", ""},
		{"merge --ff-only", []string{"merge", "--ff-only", oidC}, true, primFastForward, "", "", "", oidC},
		{"push", []string{"push", "--set-upstream", "origin", "HEAD"}, true, primPush, "", "", "", ""},
		{"update-ref with a full refname", []string{"update-ref", "refs/heads/design/x", oidA, strings.Repeat("0", 40)}, true, primUpdateRef, "refs/heads/design/x", "", "", ""},
		{"update-ref with a short name names no ref", []string{"update-ref", "design/x", oidA, strings.Repeat("0", 40)}, true, primUpdateRef, "", "", "", ""},
		{"worktree add", []string{"worktree", "add", "/w", "side"}, true, primWorktreeAdd, "", "", "", ""},
		{"worktree add --detach", []string{"worktree", "add", "--detach", "/w", oidA}, true, primWorktreeAdd, "", "", "", ""},
		{"worktree remove behind -c", []string{"-c", "status.showUntrackedFiles=all", "worktree", "remove", "/w"}, true, primWorktreeRemove, "", "", "", ""},
		{"apply", []string{"apply"}, true, primApply, "", "", "", ""},
		{"hash-object -w --stdin", []string{"hash-object", "-w", "--stdin"}, true, primWriteBlob, "", "", "", ""},
		{"read-tree into a scratch index", []string{"read-tree", oidA}, true, primScratchTree, "", "", "", ""},
		{"update-index into a scratch index", []string{"update-index", "--add", "--cacheinfo", "100644," + oidA + ",p"}, true, primScratchTree, "", "", "", ""},
		{"write-tree from a scratch index", []string{"write-tree"}, true, primScratchTree, "", "", "", ""},
		// Read-only gitx calls, which attribute nothing.
		{"rev-parse naming a ref", []string{"rev-parse", "--verify", "ritual/sneaky"}, false, 0, "", "", "", ""},
		{"symbolic-ref", []string{"symbolic-ref", "--short", "-q", "HEAD"}, false, 0, "", "", "", ""},
		{"worktree list", []string{"worktree", "list", "--porcelain"}, false, 0, "", "", "", ""},
		{"hash-object without -w", []string{"hash-object", "/p"}, false, 0, "", "", "", ""},
		{"merge-base", []string{"merge-base", "--is-ancestor", oidA, oidB}, false, 0, "", "", "", ""},
		{"status", []string{"status", "--porcelain"}, false, 0, "", "", "", ""},
		{"config", []string{"config", "--local", "--get-all", "user.name"}, false, 0, "", "", "", ""},
		{"show-ref", []string{"show-ref", "--verify", "--quiet", "refs/heads/x"}, false, 0, "", "", "", ""},
		{"a checkout of paths is not gitx's argv", []string{"checkout", "--", "a"}, false, 0, "", "", "", ""},
		{"a branch listing", []string{"branch"}, false, 0, "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc, ok := mutatingCall(Call{Dir: "/r", Args: tt.args}, prims)
			if ok != tt.want {
				t.Fatalf("mutatingCall(%q) ok = %v, want %v", tt.args, ok, tt.want)
			}
			if !ok {
				return
			}
			if lc.Kind != tt.kind {
				t.Errorf("kind = %v, want %v", lc.Kind, tt.kind)
			}
			if got := lc.createdRef(); got != tt.created {
				t.Errorf("createdRef = %q, want %q", got, tt.created)
			}
			if got := lc.deletedRef(); got != tt.deleted {
				t.Errorf("deletedRef = %q, want %q", got, tt.deleted)
			}
			if got := lc.checkedOutRef(); got != tt.switched {
				t.Errorf("checkedOutRef = %q, want %q", got, tt.switched)
			}
			if got := lc.fastForwardTarget(); got != tt.target {
				t.Errorf("fastForwardTarget = %q, want %q", got, tt.target)
			}
		})
	}
}

func TestLoggedCallWorktreePath(t *testing.T) {
	base := canonicalPath("", t.TempDir())
	tests := []struct {
		name string
		call loggedCall
		want string
	}{
		{"an absolute add", loggedCall{Dir: base, Kind: primWorktreeAdd, Args: []string{"worktree", "add", filepath.Join(base, "w"), "side"}}, filepath.Join(base, "w")},
		{"a relative detached add", loggedCall{Dir: filepath.Join(base, "repo"), Kind: primWorktreeAdd, Args: []string{"worktree", "add", "--detach", "../w", oidA}}, filepath.Join(base, "w")},
		{"a remove", loggedCall{Dir: base, Kind: primWorktreeRemove, Args: []string{"worktree", "remove", "w"}}, filepath.Join(base, "w")},
		{"not a worktree call", loggedCall{Dir: base, Kind: primCommit, Args: []string{"commit", "-m", "x"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.call.worktreePath(); got != tt.want {
				t.Errorf("worktreePath = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMutatingPrimitives_RecognizeGitxArgv runs every mutating gitx
// function against a fixture with the recorder attached and requires each
// call it logs to be recognized as that function's primitive kind — so a
// change to gitx's argv cannot silently stop attributing — and runs
// read-only gitx functions and requires none of their calls to be.
func TestMutatingPrimitives_RecognizeGitxArgv(t *testing.T) {
	ctx := context.Background()
	fx := Build(t, ctx, SeedClean)
	dir := fx.Dir
	runGitFixture(t, ctx, dir, "checkout", "--quiet", "--", TrackedFile)
	if err := os.Remove(filepath.Join(dir, UntrackedFile)); err != nil {
		t.Fatal(err)
	}
	head := fx.BaseCommit
	wt := filepath.Join(t.TempDir(), "wt")
	wt2 := filepath.Join(t.TempDir(), "wt2")
	var blob, tree, commit string
	mutating := []struct {
		fn   string
		kind primKind
		run  func(ctx context.Context) error
	}{
		{"internal/gitx.WriteBlob", primWriteBlob, func(ctx context.Context) (err error) { blob, err = gitx.WriteBlob(ctx, dir, []byte("b\n")); return }},
		{"internal/gitx.BuildTreeWithFile", primScratchTree, func(ctx context.Context) (err error) {
			tree, err = gitx.BuildTreeWithFile(ctx, dir, head+"^{tree}", "owned/b.txt", blob)
			return
		}},
		{"internal/gitx.CommitTree", primCommitTree, func(ctx context.Context) (err error) {
			commit, err = gitx.CommitTree(ctx, dir, tree, head, "tree")
			return
		}},
		{"internal/gitx.UpdateRef", primUpdateRef, func(ctx context.Context) error { return gitx.UpdateRef(ctx, dir, "refs/heads/design/x", commit) }},
		{"internal/gitx.CheckoutNewBranch", primCheckoutNew, func(ctx context.Context) error { return gitx.CheckoutNewBranch(ctx, dir, "ritual/a") }},
		{"internal/gitx.AddAll", primStage, func(ctx context.Context) error {
			if err := os.WriteFile(filepath.Join(dir, "owned", "a.txt"), []byte("a\n"), 0o644); err != nil {
				return err
			}
			return gitx.AddAll(ctx, dir)
		}},
		{"internal/gitx.CreateCommit", primCommit, func(ctx context.Context) error { _, err := gitx.CreateCommit(ctx, dir, "all"); return err }},
		{"internal/gitx.AddPaths", primStage, func(ctx context.Context) error {
			if err := os.WriteFile(filepath.Join(dir, "owned", "p.txt"), []byte("p\n"), 0o644); err != nil {
				return err
			}
			return gitx.AddPaths(ctx, dir, "owned/p.txt")
		}},
		{"internal/gitx.CreateCommitPaths", primCommit, func(ctx context.Context) error {
			_, err := gitx.CreateCommitPaths(ctx, dir, "paths", "owned/p.txt")
			return err
		}},
		{"internal/gitx.ApplyPatch", primApply, func(ctx context.Context) error {
			return gitx.ApplyPatch(ctx, dir, []byte("--- a/owned/keep.txt\n+++ b/owned/keep.txt\n@@ -1 +1 @@\n-kept\n+patched\n"))
		}},
		{"internal/gitx.CheckoutExisting", primCheckout, func(ctx context.Context) error {
			if _, err := plainGit(ctx, dir, "checkout", "--", "owned/keep.txt"); err != nil {
				return err
			}
			return gitx.CheckoutExisting(ctx, dir, "main")
		}},
		{"internal/gitx.Checkout", primCheckout, func(ctx context.Context) error { return gitx.Checkout(ctx, dir, "ritual/a") }},
		{"internal/gitx.CheckoutNewBranchFrom", primCheckoutNew, func(ctx context.Context) error {
			return gitx.CheckoutNewBranchFrom(ctx, dir, "ritual/b", "main")
		}},
		{"internal/gitx.FastForwardOnly", primFastForward, func(ctx context.Context) error {
			_, err := gitx.FastForwardOnly(ctx, dir, commit)
			return err
		}},
		{"internal/gitx.Push", primPush, func(ctx context.Context) error { return gitx.Push(ctx, dir) }},
		{"internal/gitx.DeleteBranch", primBranchDelete, func(ctx context.Context) error { return gitx.DeleteBranch(ctx, dir, "design/x") }},
		{"internal/gitx.DeleteMergedBranch", primBranchDelete, func(ctx context.Context) error {
			_, err := gitx.DeleteMergedBranch(ctx, dir, SideBranch)
			return err
		}},
		{"internal/gitx.WorktreeAdd", primWorktreeAdd, func(ctx context.Context) error { return gitx.WorktreeAdd(ctx, dir, wt, "ritual/a") }},
		{"internal/gitx.WorktreeAddDetached", primWorktreeAdd, func(ctx context.Context) error { return gitx.WorktreeAddDetached(ctx, dir, wt2, head) }},
		{"internal/gitx.WorktreeRemove", primWorktreeRemove, func(ctx context.Context) error { return gitx.WorktreeRemove(ctx, dir, wt) }},
	}
	prims := mutatingPrimitives()
	covered := map[string]bool{}
	for _, m := range mutating {
		rec := &recorder{}
		if err := m.run(gitx.WithObserver(ctx, rec)); err != nil {
			t.Fatalf("%s: %v", m.fn, err)
		}
		recognized := false
		for _, c := range rec.snapshot() {
			if lc, ok := mutatingCall(c, prims); ok {
				if lc.Kind != m.kind {
					t.Errorf("%s logged %q, recognized as kind %v, want %v", m.fn, c.Args, lc.Kind, m.kind)
				}
				recognized = true
			}
		}
		if !recognized {
			t.Errorf("%s logged no call recognized as a mutating primitive: %v", m.fn, rec.snapshot())
		}
		covered[m.fn] = true
	}
	for _, p := range prims {
		if !covered[p.fn] {
			t.Errorf("%s is never run here", p.fn)
		}
	}

	readOnly := []func(ctx context.Context) error{
		func(ctx context.Context) error { _, err := gitx.RevParse(ctx, dir, "HEAD"); return err },
		func(ctx context.Context) error { _, err := gitx.CurrentBranch(ctx, dir); return err },
		func(ctx context.Context) error { _, err := gitx.WorktreeList(ctx, dir); return err },
		func(ctx context.Context) error { _, err := gitx.HashObject(ctx, dir, "owned/keep.txt"); return err },
		func(ctx context.Context) error { _, err := gitx.StagedPaths(ctx, dir); return err },
		func(ctx context.Context) error { _, err := gitx.LocalBranches(ctx, dir); return err },
		func(ctx context.Context) error { _, err := gitx.StatusDirty(ctx, dir); return err },
		func(ctx context.Context) error { _, err := gitx.ConfigValue(ctx, dir, "user.name"); return err },
		func(ctx context.Context) error { _, err := gitx.HasLocalBranch(ctx, dir, "main"); return err },
		func(ctx context.Context) error { _, err := gitx.IsAncestor(ctx, dir, head, "HEAD"); return err },
		func(ctx context.Context) error { _, err := gitx.DefaultBranch(ctx, dir); return err },
	}
	for i, fn := range readOnly {
		rec := &recorder{}
		if err := fn(gitx.WithObserver(ctx, rec)); err != nil {
			t.Fatalf("read-only call %d: %v", i, err)
		}
		for _, c := range rec.snapshot() {
			if _, ok := mutatingCall(c, prims); ok {
				t.Errorf("read-only gitx call %q was recognized as a mutating primitive", c.Args)
			}
		}
	}
}
