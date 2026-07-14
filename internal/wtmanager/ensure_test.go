package wtmanager

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// buildRepo returns a fresh fixturegit repo with a "design/x" local
// branch carrying content distinct from main's own — the shape ac-1's
// obligation asks for ("a local design branch carrying a committed
// spec.md distinct from the default branch's own content").
func buildRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{
			"spec.md":           "main content\n",
			".verdi/.gitignore": "data/\n",
		}, Message: "root"},
	})
	ctx := context.Background()
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("CheckoutNewBranch(design/x): %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.Dir, "spec.md"), []byte("draft content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo.Dir, "add", "-A")
	runGit(t, repo.Dir, "commit", "--quiet", "-m", "draft")
	if err := gitx.Checkout(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	return repo
}

// runGit runs git in dir with a fixed author/committer identity (for any
// invocation that creates a commit) and fails the test on a non-zero
// exit — there is nothing meaningful for a caller to do with a failure
// here beyond that, so it reports via t.Fatalf rather than returning an
// error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// worktreeListCount counts `git worktree list --porcelain` entries whose
// path matches want, at root. Paths are compared after resolving symlinks
// (git itself prints realpath'd worktree paths — e.g. macOS's /var/folders
// temp dirs are actually /private/var/folders symlinks — so a naive
// string comparison against a non-canonicalized t.TempDir() path would
// spuriously read as "no match" here).
func worktreeListCount(t *testing.T, root, want string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git worktree list: %v", err)
	}
	wantReal := realOrSelf(want)
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		if realOrSelf(strings.TrimPrefix(line, "worktree ")) == wantReal {
			count++
		}
	}
	return count
}

// realOrSelf resolves symlinks in path, falling back to path unchanged if
// it cannot be resolved (e.g. it does not exist).
func realOrSelf(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

func TestEnsureWorktree_FirstCallCutsRealWorktree(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	rootHeadBefore, _ := gitx.RevParse(ctx, repo.Dir, "HEAD")
	rootBranchBefore, _ := gitx.CurrentBranch(ctx, repo.Dir)
	rootDirtyBefore, _ := gitx.StatusDirty(ctx, repo.Dir)

	path, err := EnsureWorktree(ctx, repo.Dir, "design/x")
	if err != nil {
		t.Fatalf("EnsureWorktree: %v", err)
	}

	wantPath := filepath.Join(repo.Dir, ".verdi", "data", "worktrees", "x")
	if path != wantPath {
		t.Fatalf("EnsureWorktree path = %q, want %q", path, wantPath)
	}
	content, err := os.ReadFile(filepath.Join(path, "spec.md"))
	if err != nil {
		t.Fatalf("reading cut worktree's spec.md: %v", err)
	}
	if string(content) != "draft content\n" {
		t.Fatalf("cut worktree spec.md = %q, want the design branch's own content", string(content))
	}

	// The serving checkout itself is provably undisturbed.
	rootHeadAfter, _ := gitx.RevParse(ctx, repo.Dir, "HEAD")
	rootBranchAfter, _ := gitx.CurrentBranch(ctx, repo.Dir)
	rootDirtyAfter, _ := gitx.StatusDirty(ctx, repo.Dir)
	if rootHeadAfter != rootHeadBefore || rootBranchAfter != rootBranchBefore || rootDirtyAfter != rootDirtyBefore {
		t.Fatalf("serving checkout changed: head %q->%q branch %q->%q dirty %v->%v",
			rootHeadBefore, rootHeadAfter, rootBranchBefore, rootBranchAfter, rootDirtyBefore, rootDirtyAfter)
	}

	if got := worktreeListCount(t, repo.Dir, path); got != 1 {
		t.Fatalf("git worktree list entries for %s = %d, want exactly 1", path, got)
	}
}

func TestEnsureWorktree_SecondCallReusesNoRecut(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	var calls int32
	orig := worktreeAdd
	worktreeAdd = func(ctx context.Context, dir, path, branch string) error {
		atomic.AddInt32(&calls, 1)
		return orig(ctx, dir, path, branch)
	}
	defer func() { worktreeAdd = orig }()

	first, err := EnsureWorktree(ctx, repo.Dir, "design/x")
	if err != nil {
		t.Fatalf("first EnsureWorktree: %v", err)
	}
	second, err := EnsureWorktree(ctx, repo.Dir, "design/x")
	if err != nil {
		t.Fatalf("second EnsureWorktree: %v", err)
	}
	if first != second {
		t.Fatalf("EnsureWorktree paths differ across calls: %q vs %q", first, second)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("git worktree add ran %d times, want exactly 1", calls)
	}
	if got := worktreeListCount(t, repo.Dir, first); got != 1 {
		t.Fatalf("git worktree list entries for %s = %d, want exactly 1 (no duplicate entry)", first, got)
	}
}

func TestEnsureWorktree_Negative_RemoteTrackingOnly(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	head, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := gitx.UpdateRef(ctx, repo.Dir, "refs/remotes/origin/design/y", head); err != nil {
		t.Fatalf("seeding remote-tracking ref: %v", err)
	}

	_, err = EnsureWorktree(ctx, repo.Dir, "design/y")
	if err == nil {
		t.Fatal("EnsureWorktree(remote-tracking-only branch): want error, got nil")
	}
	if !errors.Is(err, ErrNotLocalBranch) {
		t.Fatalf("EnsureWorktree(remote-tracking-only branch) error = %v, want ErrNotLocalBranch", err)
	}
	assertNoWorktreeCreated(t, repo.Dir, "y")
}

func TestEnsureWorktree_Negative_NonexistentBranch(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	_, err := EnsureWorktree(ctx, repo.Dir, "design/nope")
	if err == nil {
		t.Fatal("EnsureWorktree(nonexistent branch): want error, got nil")
	}
	if !errors.Is(err, ErrNotLocalBranch) {
		t.Fatalf("EnsureWorktree(nonexistent branch) error = %v, want ErrNotLocalBranch", err)
	}
	assertNoWorktreeCreated(t, repo.Dir, "nope")
}

func TestEnsureWorktree_Negative_AlreadyCheckedOutHere(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	if err := gitx.Checkout(ctx, repo.Dir, "design/x"); err != nil {
		t.Fatalf("Checkout(design/x): %v", err)
	}

	_, err := EnsureWorktree(ctx, repo.Dir, "design/x")
	if err == nil {
		t.Fatal("EnsureWorktree(branch checked out at root): want error, got nil")
	}
	if !errors.Is(err, ErrCheckedOutHere) {
		t.Fatalf("EnsureWorktree(branch checked out at root) error = %v, want ErrCheckedOutHere", err)
	}
	assertNoWorktreeCreated(t, repo.Dir, "x")
}

func TestEnsureWorktree_Concurrent_ExactlyOneCut(t *testing.T) {
	repo := buildRepo(t)
	ctx := context.Background()

	var calls int32
	orig := worktreeAdd
	worktreeAdd = func(ctx context.Context, dir, path, branch string) error {
		atomic.AddInt32(&calls, 1)
		return orig(ctx, dir, path, branch)
	}
	defer func() { worktreeAdd = orig }()

	const n = 8
	paths := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			paths[i], errs[i] = EnsureWorktree(ctx, repo.Dir, "design/x")
		}(i)
	}
	wg.Wait()

	want := filepath.Join(repo.Dir, ".verdi", "data", "worktrees", "x")
	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: EnsureWorktree error: %v", i, errs[i])
		}
		if paths[i] != want {
			t.Fatalf("goroutine %d: path = %q, want %q", i, paths[i], want)
		}
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("git worktree add ran %d times across %d concurrent callers, want exactly 1", calls, n)
	}
	if got := worktreeListCount(t, repo.Dir, want); got != 1 {
		t.Fatalf("git worktree list entries for %s = %d, want exactly 1", want, got)
	}
}

func assertNoWorktreeCreated(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "data", "worktrees", name)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree directory %s exists after a refused EnsureWorktree call: err=%v", path, err)
	}
}
