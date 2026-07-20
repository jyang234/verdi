package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

// TestHasRemoteTrackingBranch_Happy proves D6-6's hermetic building block
// finds a fabricated refs/remotes/origin/<branch> ref (no clone/fetch, no
// network — the ref is seeded directly via UpdateRef, same idiom
// TestHasLocalBranch_Negative's remote-tracking subtest uses).
func TestHasRemoteTrackingBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := UpdateRef(ctx, repo.Dir, "refs/remotes/origin/main", repo.Head); err != nil {
		t.Fatalf("seeding refs/remotes/origin/main: %v", err)
	}

	has, err := HasRemoteTrackingBranch(ctx, repo.Dir, "origin", "main")
	if err != nil {
		t.Fatalf("HasRemoteTrackingBranch(origin, main): %v", err)
	}
	if !has {
		t.Fatal("HasRemoteTrackingBranch(origin, main) = false, want true")
	}
}

func TestHasRemoteTrackingBranch_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("no such remote-tracking ref", func(t *testing.T) {
		has, err := HasRemoteTrackingBranch(ctx, repo.Dir, "origin", "main")
		if err != nil {
			t.Fatalf("HasRemoteTrackingBranch(origin, main): unexpected error: %v", err)
		}
		if has {
			t.Fatal("HasRemoteTrackingBranch(origin, main) = true, want false (no origin remote configured at all)")
		}
	})

	t.Run("a local branch of the same name does not count", func(t *testing.T) {
		// fixturegit's own "main" is a refs/heads/ branch, never a
		// refs/remotes/origin/ one — the two namespaces must not be
		// conflated (mirrors HasLocalBranch's inverse check).
		has, err := HasRemoteTrackingBranch(ctx, repo.Dir, "origin", "main")
		if err != nil {
			t.Fatalf("HasRemoteTrackingBranch(origin, main): unexpected error: %v", err)
		}
		if has {
			t.Fatal("HasRemoteTrackingBranch(origin, main) = true, want false (refs/heads/main must not satisfy a refs/remotes/origin/main query)")
		}
	})

	t.Run("not a repository at all", func(t *testing.T) {
		notARepo := t.TempDir()
		if _, err := HasRemoteTrackingBranch(ctx, notARepo, "origin", "main"); err == nil {
			t.Fatal("HasRemoteTrackingBranch outside a repo: want error, got nil")
		}
	})
}

func TestCheckoutExisting_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "close/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	got, err := CurrentBranch(ctx, repo.Dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "main" {
		t.Fatalf("after CheckoutExisting(main), CurrentBranch = %q, want main", got)
	}
}

// TestCheckoutExisting_DirtyTreeNotRefused is the load-bearing contrast with
// Checkout (the guarded board switch): CheckoutExisting switches even with an
// uncommitted/untracked working tree — the unwind's exact situation, a
// just-cut branch at the same commit carrying the artifacts an aborted freeze
// left behind — where Checkout would refuse, and it carries the untracked
// residue across untouched (nothing lost).
func TestCheckoutExisting_DirtyTreeNotRefused(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "close/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.txt"), []byte("residue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err := StatusDirty(ctx, repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatal("precondition: expected a dirty working tree from the untracked file")
	}
	if err := Checkout(ctx, repo.Dir, "main"); err == nil {
		t.Fatal("precondition: the guarded Checkout should refuse a dirty tree (contrast being tested)")
	}
	if err := CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main) with a dirty tree: %v — it must not refuse (nothing is lost switching back to the same commit)", err)
	}
	if got, _ := CurrentBranch(ctx, repo.Dir); got != "main" {
		t.Fatalf("CurrentBranch = %q, want main", got)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, "untracked.txt")); err != nil {
		t.Fatalf("the untracked residue was lost across CheckoutExisting: %v", err)
	}
}

func TestCheckoutExisting_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("nonexistent ref", func(t *testing.T) {
		if err := CheckoutExisting(ctx, repo.Dir, "no/such/ref"); err == nil {
			t.Fatal("CheckoutExisting(no/such/ref): want error, got nil")
		}
	})
	t.Run("not a repository at all", func(t *testing.T) {
		if err := CheckoutExisting(ctx, t.TempDir(), "main"); err == nil {
			t.Fatal("CheckoutExisting outside a repo: want error, got nil")
		}
	})
}

func TestDeleteBranch_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	// A branch cut at HEAD (trivially merged), switched away from, then deleted.
	if err := CheckoutNewBranch(ctx, repo.Dir, "close/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if err := DeleteBranch(ctx, repo.Dir, "close/x"); err != nil {
		t.Fatalf("DeleteBranch(close/x): %v", err)
	}
	has, err := HasLocalBranch(ctx, repo.Dir, "close/x")
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("close/x still exists after DeleteBranch")
	}
}

// TestDeleteBranch_RefusesUnmergedCommits proves the SAFE-delete posture the
// unwind relies on: a branch carrying a commit not merged into HEAD is refused
// (`git branch -d`), never force-removed, so committed work can never be
// silently discarded.
func TestDeleteBranch_RefusesUnmergedCommits(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := CheckoutNewBranch(ctx, repo.Dir, "close/x"); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "onbranch.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddAll(ctx, repo.Dir); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCommit(ctx, repo.Dir, "work only on close/x"); err != nil {
		t.Fatal(err)
	}
	if err := CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	if err := DeleteBranch(ctx, repo.Dir, "close/x"); err == nil {
		t.Fatal("DeleteBranch(close/x carrying an unmerged commit): want a safe-delete refusal, got nil")
	}
	has, err := HasLocalBranch(ctx, repo.Dir, "close/x")
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("close/x was removed despite an unmerged commit — the safe-delete refusal did not protect it")
	}
}

func TestDeleteBranch_Negative(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("nonexistent branch", func(t *testing.T) {
		if err := DeleteBranch(ctx, repo.Dir, "close/nope"); err == nil {
			t.Fatal("DeleteBranch(nonexistent): want error, got nil")
		}
	})
	t.Run("the current branch cannot be deleted", func(t *testing.T) {
		if err := DeleteBranch(ctx, repo.Dir, "main"); err == nil {
			t.Fatal("DeleteBranch(current branch): want error, got nil")
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
