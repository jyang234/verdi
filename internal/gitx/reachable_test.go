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
// Reachable, and a commit is Reachable from itself (self-ancestor,
// mirroring IsAncestor's own documented semantics). A positive answer is a
// real proof of reachability and is shallow-independent.
func TestReachableFromHEAD_Happy(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	t.Run("real ancestor is reachable", func(t *testing.T) {
		got, err := ReachableFromHEAD(ctx, repo.Dir, repo.Heads[0], repo.Heads[1])
		if err != nil {
			t.Fatalf("ReachableFromHEAD(layer1, layer2): %v", err)
		}
		if got != Reachable {
			t.Fatalf("ReachableFromHEAD(layer1, layer2) = %v, want Reachable (layer1 is layer2's parent)", got)
		}
	})

	t.Run("a commit is reachable from itself", func(t *testing.T) {
		got, err := ReachableFromHEAD(ctx, repo.Dir, repo.Head, repo.Head)
		if err != nil {
			t.Fatalf("ReachableFromHEAD(HEAD, HEAD): %v", err)
		}
		if got != Reachable {
			t.Fatalf("ReachableFromHEAD(HEAD, HEAD) = %v, want Reachable", got)
		}
	})
}

// TestReachableFromHEAD_NonExistentCommit proves the X-15 shape in a FULL
// (non-shallow) clone: a well-formed but never-existing sha (e.g. one whose
// source branch was deleted and the object itself was never fetched, or has
// since been garbage-collected) is a PROVEN Unreachable with NO error —
// unlike IsAncestor, which reports this same shape as an error (git's own
// "fatal: Not a valid commit name"). In a full clone, absence IS proof.
func TestReachableFromHEAD_NonExistentCommit(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	got, err := ReachableFromHEAD(ctx, repo.Dir, "0000000000000000000000000000000000000000", repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(nonexistent sha): unexpected error: %v (want Unreachable, nil — X-15's exact hard-fail shape must not reach the caller as an operational error)", err)
	}
	if got != Unreachable {
		t.Fatalf("ReachableFromHEAD(nonexistent sha) = %v, want Unreachable (full clone: absence is proof)", got)
	}
}

// TestReachableFromHEAD_DanglingObject proves the X-11b shape in a FULL
// clone: a commit that IS a real, locally-present object (git can resolve
// it) but that no branch or ref anywhere reaches is a PROVEN Unreachable
// with no error — the exact distinction CommitExists alone (VL-009's old
// check) could not make. The X-11b pin holds: a full clone still reds.
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

	got, err := ReachableFromHEAD(ctx, repo.Dir, dangling, repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(dangling object): unexpected error: %v", err)
	}
	if got != Unreachable {
		t.Fatalf("ReachableFromHEAD(dangling object) = %v, want Unreachable (no ref reaches it, full clone — X-11b)", got)
	}
}

// TestReachableFromHEAD_DivergedSibling proves the ordinary, mundane
// non-ancestor case (a real commit on a diverged, unmerged branch) in a
// FULL clone: a PROVEN Unreachable, no error — IsAncestor's existing
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

	got, err := ReachableFromHEAD(ctx, repo.Dir, sibling, repo.Head)
	if err != nil {
		t.Fatalf("ReachableFromHEAD(diverged sibling): unexpected error: %v", err)
	}
	if got != Unreachable {
		t.Fatalf("ReachableFromHEAD(diverged sibling) = %v, want Unreachable (real commit, not an ancestor, full clone)", got)
	}
}

// TestReachableFromHEAD_NotARepo proves a genuine operational failure (dir
// is not a git repository at all) is still a real, surfaced error — only a
// resolvable-but-unreachable commit is folded into the Unreachable case.
func TestReachableFromHEAD_NotARepo(t *testing.T) {
	notARepo := t.TempDir()
	if _, err := ReachableFromHEAD(context.Background(), notARepo, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "HEAD"); err == nil {
		t.Fatal("ReachableFromHEAD outside a repo: want error, got nil")
	}
}

// buildThreeLayerRepo builds L1 -> L2 -> L3(tip), so a --depth 2 shallow
// clone leaves L1 beyond the horizon (absent) while L2 and L3 stay present.
func buildThreeLayerRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "one\n"}, Message: "layer 1"},
		{Files: map[string]string{"a.txt": "two\n"}, Message: "layer 2"},
		{Files: map[string]string{"a.txt": "three\n"}, Message: "layer 3"},
	})
}

// TestReachableFromHEAD_ShallowBeyondHorizon is the P2-10b red-first pin: in
// a SHALLOW clone, a commit that is genuinely reachable in the full history
// but sits BEYOND the shallow horizon (its object was never fetched) must
// read UnprovableShallow — NOT the definitive Unreachable a full clone would
// (and used to) return. Shallow history can prove YES, never NO: absence
// below the horizon is not proof of unreachability.
func TestReachableFromHEAD_ShallowBeyondHorizon(t *testing.T) {
	ctx := context.Background()
	src := buildThreeLayerRepo(t)
	beyond := src.Heads[0] // L1 — a real ancestor of the tip in full history
	clone := fixturegit.ShallowClone(t, src, 2)

	// Sanity: the shallow clone genuinely lacks L1's object (the exact
	// horizon-dependent absence that used to read as a false Unreachable).
	exists, err := CommitExists(ctx, clone, beyond)
	if err != nil {
		t.Fatalf("CommitExists(beyond-horizon commit): %v", err)
	}
	if exists {
		t.Skipf("test environment did not produce a shallow horizon that excludes L1 (git clone --depth semantics); beyond=%s present", beyond)
	}

	got, err := ReachableFromHEAD(ctx, clone, beyond, "HEAD")
	if err != nil {
		t.Fatalf("ReachableFromHEAD(beyond-horizon, shallow): unexpected error: %v", err)
	}
	if got != UnprovableShallow {
		t.Fatalf("ReachableFromHEAD(beyond-horizon, shallow) = %v, want UnprovableShallow (shallow history cannot prove unreachability — P2-10b)", got)
	}
}

// TestReachableFromHEAD_ShallowWithinHorizon proves positive proof is
// unchanged in a shallow clone: the tip (self) and a within-horizon real
// ancestor both read Reachable — ancestry that is fully visible within the
// horizon is real proof, shallow or not.
func TestReachableFromHEAD_ShallowWithinHorizon(t *testing.T) {
	ctx := context.Background()
	src := buildThreeLayerRepo(t)
	within := src.Heads[1] // L2 — present within a depth-2 horizon, ancestor of tip
	clone := fixturegit.ShallowClone(t, src, 2)

	// Sanity: L2 really is present within the horizon.
	exists, err := CommitExists(ctx, clone, within)
	if err != nil {
		t.Fatalf("CommitExists(within-horizon commit): %v", err)
	}
	if !exists {
		t.Fatalf("test bug: within-horizon commit %s absent from a depth-2 clone", within)
	}

	t.Run("within-horizon ancestor is proven reachable", func(t *testing.T) {
		got, err := ReachableFromHEAD(ctx, clone, within, "HEAD")
		if err != nil {
			t.Fatalf("ReachableFromHEAD(within-horizon ancestor, shallow): %v", err)
		}
		if got != Reachable {
			t.Fatalf("ReachableFromHEAD(within-horizon ancestor, shallow) = %v, want Reachable (positive proof unchanged)", got)
		}
	})

	t.Run("tip is proven reachable from itself", func(t *testing.T) {
		head, err := RevParse(ctx, clone, "HEAD")
		if err != nil {
			t.Fatalf("RevParse(HEAD) in clone: %v", err)
		}
		got, err := ReachableFromHEAD(ctx, clone, head, "HEAD")
		if err != nil {
			t.Fatalf("ReachableFromHEAD(tip, shallow): %v", err)
		}
		if got != Reachable {
			t.Fatalf("ReachableFromHEAD(tip, shallow) = %v, want Reachable", got)
		}
	})
}

// TestReachability_String pins the legible names used in disclosure/test
// output for each three-valued outcome.
func TestReachability_String(t *testing.T) {
	cases := map[Reachability]string{
		Unreachable:       "unreachable",
		Reachable:         "reachable",
		UnprovableShallow: "unprovable-shallow",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("Reachability(%d).String() = %q, want %q", int(r), got, want)
		}
	}
}
