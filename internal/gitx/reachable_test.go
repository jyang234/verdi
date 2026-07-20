package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// TestReachableFromHEAD_Happy proves the ordinary cases: a real ancestor is
// reachable, and a commit is reachable from itself (self-ancestor,
// mirroring IsAncestor's own documented semantics).
func TestReachableFromHEAD_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("real ancestor is reachable", func(t *testing.T) {
		ok, err := ReachableFromHEAD(ctx, repo.Dir, repo.Heads[0], repo.Heads[1])
		if err != nil {
			t.Fatalf("ReachableFromHEAD(layer1, layer2): %v", err)
		}
		if !ok {
			t.Fatal("ReachableFromHEAD(layer1, layer2) = false, want true (layer1 is layer2's parent)")
		}
	})

	t.Run("a commit is reachable from itself", func(t *testing.T) {
		ok, err := ReachableFromHEAD(ctx, repo.Dir, repo.Head, repo.Head)
		if err != nil {
			t.Fatalf("ReachableFromHEAD(HEAD, HEAD): %v", err)
		}
		if !ok {
			t.Fatal("ReachableFromHEAD(HEAD, HEAD) = false, want true")
		}
	})
}

// TestReachableFromHEAD_NonExistentCommit proves the X-15 shape: a
// well-formed but never-existing sha (e.g. one whose source branch was
// deleted and the object itself was never fetched, or has since been
// garbage-collected) is reported false with NO error — unlike IsAncestor,
// which reports this same shape as an error (git's own "fatal: Not a
// valid commit name").
func TestReachableFromHEAD_NonExistentCommit(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	ok, err := ReachableFromHEAD(ctx, repo.Dir, "0000000000000000000000000000000000000000", repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(nonexistent sha): unexpected error: %v (want false, nil — X-15's exact hard-fail shape must not reach the caller as an operational error)", err)
	}
	if ok {
		t.Fatal("ReachableFromHEAD(nonexistent sha) = true, want false")
	}
}

// TestReachableFromHEAD_DanglingObject proves the X-11b shape: a commit
// that IS a real, locally-present object (git can resolve it) but that no
// branch or ref anywhere reaches is reported false with no error — the
// exact distinction CommitExists alone (VL-009's old check) could not
// make, since CommitExists is satisfied by mere object presence.
func TestReachableFromHEAD_DanglingObject(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()
	dangling := fixturegit.Dangle(t, repo, map[string]string{"orphan.txt": "orphan\n"}, "orphaned commit")

	// Sanity: the object really is present (mirrors CommitExists's own
	// "false green" — this is exactly what the old, insufficient check
	// would have accepted).
	exists, err := CommitExists(ctx, repo.Dir, dangling)
	if err != nil {
		t.Fatalf("CommitExists(dangling): %v", err)
	}
	if !exists {
		t.Fatalf("test bug: dangling commit %s is not even present as a loose object", dangling)
	}

	ok, err := ReachableFromHEAD(ctx, repo.Dir, dangling, repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(dangling object): unexpected error: %v", err)
	}
	if ok {
		t.Fatal("ReachableFromHEAD(dangling object) = true, want false (no ref reaches it — X-11b)")
	}
}

// TestReachableFromHEAD_DivergedSibling proves the ordinary, mundane
// non-ancestor case (a real commit on a diverged, unmerged branch) is
// unaffected: still false, still no error — IsAncestor's existing
// behavior for this shape, unchanged.
func TestReachableFromHEAD_DivergedSibling(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if _, err := run(ctx, repo.Dir, "checkout", "--quiet", "-b", "sibling", repo.Heads[0]); err != nil {
		t.Fatalf("checkout sibling: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "sibling.txt"), []byte("sibling\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := run(ctx, repo.Dir, "add", "-A"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := run(ctx, repo.Dir, "commit", "--quiet", "-m", "sibling commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}
	out, err := run(ctx, repo.Dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	sibling := strings.TrimSpace(string(out))
	if _, err := run(ctx, repo.Dir, "checkout", "--quiet", "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	ok, err := ReachableFromHEAD(ctx, repo.Dir, sibling, repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(diverged sibling): unexpected error: %v", err)
	}
	if ok {
		t.Fatal("ReachableFromHEAD(diverged sibling) = true, want false (real commit, but not an ancestor)")
	}
}

// TestReachableFromHEAD_NotARepo proves a genuine operational failure (dir
// is not a git repository at all) is still a real, surfaced error — only a
// resolvable-but-unreachable commit is folded into the false case.
func TestReachableFromHEAD_NotARepo(t *testing.T) {
	notARepo := t.TempDir()
	if _, err := ReachableFromHEAD(context.Background(), notARepo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "HEAD"); err == nil {
		t.Fatal("ReachableFromHEAD outside a repo: want error, got nil")
	}
}
