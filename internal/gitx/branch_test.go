package gitx

import (
	"context"
	"os/exec"
	"testing"
)

func TestCurrentBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "main" {
		t.Fatalf("CurrentBranch = %q, want %q (fixturegit.Build uses --initial-branch=main)", got, "main")
	}
}

func TestCurrentBranch_DetachedHEAD(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := exec.CommandContext(ctx, "git", "-C", repo.Dir, "checkout", "--quiet", repo.Heads[0]).Run(); err != nil {
		t.Fatalf("detaching HEAD: %v", err)
	}

	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch on detached HEAD: unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("CurrentBranch on detached HEAD = %q, want empty", got)
	}
}

func TestCurrentBranch_Negative(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := CurrentBranch(ctx, notARepo); err == nil {
		t.Fatal("CurrentBranch outside a repo: want error, got nil")
	}
}

func TestDefaultBranch_NoRemote(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// fixturegit repos never configure an "origin" remote, so this exercises
	// the common local-fixture / bare-clone case: unknown, not an error
	// (I-14: local-otherwise-warns).
	got, err := DefaultBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("DefaultBranch with no origin remote: unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("DefaultBranch with no origin remote = %q, want empty", got)
	}
}

func TestDefaultBranch_RemoteHEADConfigured(t *testing.T) {
	upstream := buildRepo(t)
	ctx := context.Background()

	clone := t.TempDir()
	runFor(t, clone, "clone", "--quiet", upstream.Dir, ".")
	runFor(t, clone, "remote", "set-head", "origin", "-a")

	got, err := DefaultBranch(ctx, clone)
	if err != nil {
		t.Fatalf("DefaultBranch: %v", err)
	}
	if got != "main" {
		t.Fatalf("DefaultBranch = %q, want %q", got, "main")
	}
}

func TestDefaultBranch_Negative(t *testing.T) {
	ctx := context.Background()
	notARepo := t.TempDir()
	if _, err := DefaultBranch(ctx, notARepo); err == nil {
		t.Fatal("DefaultBranch outside a repo: want error, got nil")
	}
}

func TestHasLocalBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}

	has, err := HasLocalBranch(ctx, repo.Dir, "design/x")
	if err != nil {
		t.Fatalf("HasLocalBranch(design/x): %v", err)
	}
	if !has {
		t.Fatal("HasLocalBranch(design/x) = false, want true")
	}

	has, err = HasLocalBranch(ctx, repo.Dir, "main")
	if err != nil {
		t.Fatalf("HasLocalBranch(main): %v", err)
	}
	if !has {
		t.Fatal("HasLocalBranch(main) = false, want true")
	}
}

func TestHasLocalBranch_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("branch resolves nowhere at all", func(t *testing.T) {
		has, err := HasLocalBranch(ctx, repo.Dir, "design/nope")
		if err != nil {
			t.Fatalf("HasLocalBranch(design/nope): unexpected error: %v", err)
		}
		if has {
			t.Fatal("HasLocalBranch(design/nope) = true, want false")
		}
	})

	t.Run("remote-tracking-only branch is not a local branch", func(t *testing.T) {
		// Fabricate a remote-tracking ref directly (no clone/fetch, co-2:
		// no network in any test) — HasLocalBranch must say false for
		// this even though *some* ref named design/remote-only exists.
		if err := UpdateRef(ctx, repo.Dir, "refs/remotes/origin/design/remote-only", repo.Head); err != nil {
			t.Fatalf("seeding remote-tracking ref: %v", err)
		}
		has, err := HasLocalBranch(ctx, repo.Dir, "design/remote-only")
		if err != nil {
			t.Fatalf("HasLocalBranch(design/remote-only): unexpected error: %v", err)
		}
		if has {
			t.Fatal("HasLocalBranch(design/remote-only) = true, want false (remote-tracking ref must not count as local)")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := HasLocalBranch(ctx, notARepo, "main"); err == nil {
			t.Fatal("HasLocalBranch outside a repo: want error, got nil")
		}
	})
}

func TestMergeBase_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := MergeBase(ctx, repo.Dir, repo.Heads[0], repo.Head)
	if err != nil {
		t.Fatalf("MergeBase: %v", err)
	}
	if got != repo.Heads[0] {
		t.Fatalf("MergeBase(layer1, HEAD) = %q, want %q (layer1 is an ancestor of HEAD)", got, repo.Heads[0])
	}
}

func TestMergeBase_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	if _, err := MergeBase(ctx, repo.Dir, "not-a-real-rev", repo.Head); err == nil {
		t.Fatal("MergeBase(bogus rev): want error, got nil")
	}
}

// runFor runs git in dir with the process's inherited environment, failing
// the test on a non-zero exit — used by tests that need a real "origin"
// remote (clone/remote set-head) or a real rename (git mv), neither of
// which buildRepo's fixturegit-based setup provides.
func runFor(t *testing.T, dir string, args ...string) {
	t.Helper()
	runForOutput(t, dir, args...)
}

// runForOutput is runFor plus the combined stdout+stderr, for callers that
// need the output (e.g. `rev-parse HEAD`).
func runForOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
