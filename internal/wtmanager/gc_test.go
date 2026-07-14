package wtmanager

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
)

// gcFixture builds a store root on "main" with a design/<name> branch for
// each entry in names, each one carrying a distinct committed file, and
// returns the root directory.
func gcFixture(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "--quiet", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Verdi Fixture")
	runGit(t, root, "config", "user.email", "fixture@verdi.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".verdi/data/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "root")

	ctx := context.Background()
	for _, name := range names {
		branch := "design/" + name
		if err := gitx.CheckoutNewBranch(ctx, root, branch); err != nil {
			t.Fatalf("CheckoutNewBranch(%s): %v", branch, err)
		}
		if err := os.WriteFile(filepath.Join(root, name+".txt"), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "-A")
		runGit(t, root, "commit", "--quiet", "-m", "add "+name)
		if err := gitx.Checkout(ctx, root, "main"); err != nil {
			t.Fatalf("Checkout(main) after %s: %v", branch, err)
		}
	}
	return root
}

// mergeBranch merges branch into main (fast-forward or a real merge
// commit — either way branch's tip becomes an ancestor of main's tip
// afterward), then returns to main.
func mergeBranch(t *testing.T, root, branch string) {
	t.Helper()
	ctx := context.Background()
	if err := gitx.Checkout(ctx, root, "main"); err != nil {
		t.Fatalf("Checkout(main): %v", err)
	}
	runGit(t, root, "merge", "--quiet", "--no-ff", "-m", "merge "+branch, branch)
}

// cutManagedWorktree cuts a managed worktree for branch under root's data
// zone via the real EnsureWorktree path (never a hand-rolled `git
// worktree add`), matching production shape exactly.
func cutManagedWorktree(t *testing.T, root, branch string) string {
	t.Helper()
	path, err := EnsureWorktree(context.Background(), root, branch)
	if err != nil {
		t.Fatalf("EnsureWorktree(%s) seeding gc fixture: %v", branch, err)
	}
	return path
}

// seedLiveLock writes a lockfile at path naming our OWN pid (guaranteed
// alive) as its holder — the same trick internal/filelock's own tests use
// to simulate a live owner deterministically without a real second
// process.
func seedLiveLock(t *testing.T, lockPath string) {
	t.Helper()
	info := filelock.Info{PID: os.Getpid(), Start: time.Now().Unix()}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0o644); err != nil {
		t.Fatalf("seeding live lock at %s: %v", lockPath, err)
	}
}

func resultFor(t *testing.T, results []Result, name string) Result {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("GC results contain no entry for %q (results: %+v)", name, results)
	return Result{}
}

func TestGC_FourWorktrees_EachRatifiedOutcome(t *testing.T) {
	root := gcFixture(t, "clean", "dirty", "locked", "unmerged")
	ctx := context.Background()

	mergeBranch(t, root, "design/clean")
	mergeBranch(t, root, "design/dirty")
	mergeBranch(t, root, "design/locked")
	// design/unmerged is deliberately left unmerged.

	cleanPath := cutManagedWorktree(t, root, "design/clean")
	dirtyPath := cutManagedWorktree(t, root, "design/dirty")
	lockedPath := cutManagedWorktree(t, root, "design/locked")
	cutManagedWorktree(t, root, "design/unmerged")

	if err := os.WriteFile(filepath.Join(dirtyPath, "uncommitted.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedLiveLock(t, lockedPath+".lock")

	results, err := GC(ctx, root, "main")
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(results) != 4 {
		t.Fatalf("GC returned %d results, want 4: %+v", len(results), results)
	}

	clean := resultFor(t, results, "clean")
	if clean.Decision != Reclaim {
		t.Fatalf("clean worktree decision = %v, want Reclaim", clean.Decision)
	}
	if _, err := os.Stat(cleanPath); !os.IsNotExist(err) {
		t.Fatalf("merged-and-clean worktree still on disk after GC: err=%v", err)
	}

	dirty := resultFor(t, results, "dirty")
	if dirty.Decision != KeepDirty {
		t.Fatalf("dirty worktree decision = %v, want KeepDirty", dirty.Decision)
	}
	if _, err := os.Stat(dirtyPath); err != nil {
		t.Fatalf("dirty worktree removed despite uncommitted changes: %v", err)
	}

	locked := resultFor(t, results, "locked")
	if locked.Decision != KeepLocked {
		t.Fatalf("locked worktree decision = %v, want KeepLocked", locked.Decision)
	}
	if _, err := os.Stat(lockedPath); err != nil {
		t.Fatalf("locked worktree removed despite a live lock: %v", err)
	}

	unmerged := resultFor(t, results, "unmerged")
	if unmerged.Decision != KeepNotEligible {
		t.Fatalf("unmerged worktree decision = %v, want KeepNotEligible", unmerged.Decision)
	}

	// Every keep-reason gets a DISTINCT message — never one undifferentiated
	// "kept" line for all three.
	lines := map[string]string{
		"dirty":    dirty.Line(),
		"locked":   locked.Line(),
		"unmerged": unmerged.Line(),
	}
	seen := map[string]bool{}
	for who, line := range lines {
		if seen[line] {
			t.Fatalf("keep-reason lines are not distinct: %q duplicated (from %s)", line, who)
		}
		seen[line] = true
	}
	if got := clean.Line(); !containsAll(got, "reclaimed", "clean") {
		t.Fatalf("clean.Line() = %q, want it to read as a reclaim of the clean worktree", got)
	}
	if got := dirty.Line(); !containsAll(got, "kept", "uncommitted") {
		t.Fatalf("dirty.Line() = %q, want it to name uncommitted changes", got)
	}
	if got := locked.Line(); !containsAll(got, "kept", "in use") {
		t.Fatalf("locked.Line() = %q, want it to name in-use/locked", got)
	}
	if got := unmerged.Line(); !containsAll(got, "kept") {
		t.Fatalf("unmerged.Line() = %q, want a kept disclosure", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

func TestGC_DeletedBranchSignal_LocalOnly(t *testing.T) {
	root := gcFixture(t, "gone")
	ctx := context.Background()

	path := cutManagedWorktree(t, root, "design/gone")

	// Delete the LOCAL branch only — never merged, but dc-3's deleted
	// signal fires purely on local absence. git itself refuses to delete
	// a branch that is checked out in any of its worktrees ("git branch
	// -D" errors "Cannot delete branch ... checked out at ..."), so this
	// models the orphaning as it can actually happen: the worktree's own
	// HEAD is detached first (no worktree symbolically references the
	// branch anymore, exactly as if some out-of-band cleanup had already
	// moved it off the branch), and only then is the now-unreferenced
	// branch ref itself removed at the low level (`update-ref -d`, which
	// — unlike `branch -D` — has no worktree-awareness of its own to
	// refuse the deletion). GC identifies which branch a managed worktree
	// belongs to purely from its directory name (dc-1's naming contract),
	// never from the worktree's own live HEAD, so this is a faithful
	// proof of dc-3's local-only deleted signal.
	runGit(t, path, "checkout", "--detach", "--quiet")
	runGit(t, root, "update-ref", "-d", "refs/heads/design/gone")

	results, err := GC(ctx, root, "main")
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	r := resultFor(t, results, "gone")
	if r.Decision != Reclaim {
		t.Fatalf("locally-deleted branch's worktree decision = %v, want Reclaim", r.Decision)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("worktree for a locally-deleted branch still on disk: err=%v", err)
	}
}

// TestGC_ConcurrentLiveLockSurvivesGC is ac-3's second behavioral proof:
// a worktree whose lock is held by a genuinely live, concurrently-running
// goroutine is never removed by a GC run that overlaps with it — GC
// observes it as held (filelock.Peek/Acquire's own liveness probe) and
// discloses it kept, not removed, in the SAME run the holder is active.
// Once the holder releases, an immediately following GC run reclaims the
// exact same worktree — proving the earlier skip was the narrow,
// transient race window dc-2/dc-4 describe, not a permanent exemption.
func TestGC_ConcurrentLiveLockSurvivesGC(t *testing.T) {
	root := gcFixture(t, "held")
	mergeBranch(t, root, "design/held")
	path := cutManagedWorktree(t, root, "design/held")
	lp := path + ".lock"

	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		f, err := filelock.Acquire(lp)
		if err != nil {
			t.Errorf("holder: filelock.Acquire: %v", err)
			close(held)
			return
		}
		close(held)
		<-release
		_ = filelock.Release(f, lp)
	}()

	<-held
	results, err := GC(context.Background(), root, "main")
	if err != nil {
		close(release)
		<-done
		t.Fatalf("GC while lock held: %v", err)
	}
	r := resultFor(t, results, "held")
	if r.Decision != KeepLocked {
		close(release)
		<-done
		t.Fatalf("GC decision while lock held by a live goroutine = %v, want KeepLocked", r.Decision)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		close(release)
		<-done
		t.Fatalf("worktree removed while its lock was live: %v", statErr)
	}

	close(release)
	<-done

	results, err = GC(context.Background(), root, "main")
	if err != nil {
		t.Fatalf("GC after lock released: %v", err)
	}
	r = resultFor(t, results, "held")
	if r.Decision != Reclaim {
		t.Fatalf("GC decision after lock released = %v, want Reclaim (the earlier skip must be a narrow, transient window, not permanent)", r.Decision)
	}
}

func TestGC_NoWorktreesDirectory_NotAnError(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init", "--quiet", "--initial-branch=main")
	runGit(t, root, "config", "user.name", "Verdi Fixture")
	runGit(t, root, "config", "user.email", "fixture@verdi.invalid")
	runGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "--quiet", "-m", "root")

	results, err := GC(context.Background(), root, "main")
	if err != nil {
		t.Fatalf("GC(no worktrees dir): unexpected error: %v", err)
	}
	if results != nil {
		t.Fatalf("GC(no worktrees dir) = %+v, want nil", results)
	}
}

func TestGC_NeverPassesForceToRemove(t *testing.T) {
	// Static-shaped assertion, kept as a real test: WorktreeRemove itself
	// (internal/gitx) already proves git's own dirty-tree refusal fires
	// without --force (TestWorktreeRemove_Negative_DirtyRefusedWithoutForce);
	// this proves GC's dirty path never even reaches removal in the first
	// place, so that guarantee is never bypassed from this package's side.
	root := gcFixture(t, "dirty2")
	mergeBranch(t, root, "design/dirty2")
	path := cutManagedWorktree(t, root, "design/dirty2")
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var removeCalled bool
	origRemove := worktreeRemove
	worktreeRemove = func(ctx context.Context, dir, p string) error {
		removeCalled = true
		return origRemove(ctx, dir, p)
	}
	defer func() { worktreeRemove = origRemove }()

	if _, err := GC(context.Background(), root, "main"); err != nil {
		t.Fatalf("GC: %v", err)
	}
	if removeCalled {
		t.Fatal("GC called git worktree remove on a dirty worktree; it must be skipped before reaching removal at all")
	}
}

func TestGC_Negative_MalformedLockDoesNotCrashRun(t *testing.T) {
	root := gcFixture(t, "badlock")
	mergeBranch(t, root, "design/badlock")
	path := cutManagedWorktree(t, root, "design/badlock")
	if err := os.WriteFile(path+".lock", []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := GC(context.Background(), root, "main"); err == nil {
		t.Fatal("GC with a malformed lockfile: want an error (fail loud), got nil")
	}
}
