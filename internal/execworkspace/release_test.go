package execworkspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/filelock"
)

// --- fresh release ---

func TestRelease_Fresh_CreatesZeroByteMarkerAndReleasesLock(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	rel := NewReleaser(storeRoot)
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("Release: %v", err)
	}

	markerPath := ReleasedPath(storeRoot, workspaceID)
	fi, err := os.Lstat(markerPath)
	if err != nil {
		t.Fatalf("lstat marker: %v", err)
	}
	if !fi.Mode().IsRegular() {
		t.Fatalf("marker is not a regular file: mode=%v", fi.Mode())
	}
	if fi.Size() != 0 {
		t.Fatalf("marker is not zero-byte: size=%d", fi.Size())
	}

	// The lock file must be gone (released, not left held) after a
	// successful call.
	lockAbsent(t, storeRoot, workspaceID)
}

// TestRelease_ConcurrentAcquireDuringRelease proves the lock was genuinely
// HELD for the marker-creation operation: a concurrent Acquire attempted
// while Release is holding it (simulated by acquiring first ourselves and
// confirming Release then fails, rather than a true goroutine race, which
// would be nondeterministic) sees it as held.
func TestRelease_ConcurrentAcquireDuringRelease(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := LockPath(storeRoot, workspaceID)
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("filelock.Acquire (test holds lock first): %v", err)
	}

	rel := NewReleaser(storeRoot)
	err = rel.Release(workspaceID)
	if err == nil {
		t.Fatal("Release while lock externally held: want operational error, got nil")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error is not *OperationalError: %v (%T)", err, err)
	}

	// No marker was created while the lock was held by someone else.
	if _, err := os.Lstat(ReleasedPath(storeRoot, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker created despite failed lock acquisition: lstat err=%v", err)
	}

	if err := filelock.Release(lockFile, lockPath); err != nil {
		t.Fatalf("releasing test-held lock: %v", err)
	}

	// Now that the lock is free, Release must succeed and create the
	// marker.
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("Release after lock freed: %v", err)
	}
	if _, err := os.Lstat(ReleasedPath(storeRoot, workspaceID)); err != nil {
		t.Fatalf("marker not created after lock freed: %v", err)
	}
}

// --- idempotent re-release ---

func TestRelease_IdempotentReRelease_RegularFileSuccess(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	rel := NewReleaser(storeRoot)
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("first Release: %v", err)
	}
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("second Release (idempotent): %v", err)
	}
	lockAbsent(t, storeRoot, workspaceID)
}

func TestRelease_NonemptyMarker_ContentIgnored(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	markerPath := ReleasedPath(storeRoot, workspaceID)
	if err := os.WriteFile(markerPath, []byte("some unrelated bytes, never decoded"), 0o644); err != nil {
		t.Fatalf("seed nonempty marker: %v", err)
	}

	rel := NewReleaser(storeRoot)
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("Release against a pre-existing nonempty regular marker: want success (content ignored), got %v", err)
	}

	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("reading marker after Release: %v", err)
	}
	if string(data) != "some unrelated bytes, never decoded" {
		t.Fatalf("marker content was mutated: %q", string(data))
	}
}

// --- wedged marker ---

func TestRelease_WedgedMarker_DirectoryIsOperationalError(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	markerPath := ReleasedPath(storeRoot, workspaceID)
	if err := os.Mkdir(markerPath, 0o755); err != nil {
		t.Fatalf("seed directory at marker path: %v", err)
	}

	rel := NewReleaser(storeRoot)
	err := rel.Release(workspaceID)
	if err == nil {
		t.Fatal("Release against a directory at the marker path: want operational error, got nil (never a successful release)")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error is not *OperationalError: %v (%T)", err, err)
	}
	// Untouched: still a directory.
	fi, serr := os.Lstat(markerPath)
	if serr != nil || !fi.IsDir() {
		t.Fatalf("marker path no longer a directory after failed Release: fi=%v err=%v", fi, serr)
	}
	lockAbsent(t, storeRoot, workspaceID)
}

func TestRelease_WedgedMarker_SymlinkIsOperationalError(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, []byte("danger"), 0o644); err != nil {
		t.Fatalf("seed symlink target: %v", err)
	}
	markerPath := ReleasedPath(storeRoot, workspaceID)
	if err := os.Symlink(target, markerPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	rel := NewReleaser(storeRoot)
	err := rel.Release(workspaceID)
	if err == nil {
		t.Fatal("Release against a symlink at the marker path: want operational error, got nil (a consumer must never be told a wedged marker path was a successful release)")
	}
	var opErr *OperationalError
	if !errors.As(err, &opErr) {
		t.Fatalf("error is not *OperationalError: %v (%T)", err, err)
	}
	// The symlink itself must be untouched, and its target never followed
	// or written through.
	fi, serr := os.Lstat(markerPath)
	if serr != nil {
		t.Fatalf("lstat marker after failed Release: %v", serr)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("marker path is no longer a symlink after failed Release: mode=%v", fi.Mode())
	}
	data, rerr := os.ReadFile(target)
	if rerr != nil {
		t.Fatalf("reading symlink target: %v", rerr)
	}
	if string(data) != "danger" {
		t.Fatalf("symlink target was written through: %q", string(data))
	}
}

// --- lock held by live holder ---

func TestRelease_LockHeldByLiveHolder_OperationalErrorNoMarker(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := LockPath(storeRoot, workspaceID)
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("filelock.Acquire: %v", err)
	}
	t.Cleanup(func() { _ = filelock.Release(lockFile, lockPath) })

	rel := NewReleaser(storeRoot)
	err = rel.Release(workspaceID)
	if err == nil {
		t.Fatal("Release with lock held by a live holder: want operational error, got nil")
	}
	var heldErr *filelock.ErrHeld
	if !errors.As(err, &heldErr) {
		t.Fatalf("error does not unwrap to *filelock.ErrHeld: %v", err)
	}
	if _, err := os.Lstat(ReleasedPath(storeRoot, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker created despite live-held lock: lstat err=%v", err)
	}
}

// --- release of an id with nothing on disk (abandoned run) ---

func TestRelease_AbandonedRun_NothingOnDisk_MarkerStillCreated(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "never-materialized--0123456789ab"

	// Nothing at all under data/execution/ yet — not even the root.
	if _, err := os.Stat(ExecutionRoot(storeRoot)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("setup sanity: execution root already exists: %v", err)
	}

	rel := NewReleaser(storeRoot)
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("Release(abandoned run, nothing on disk): %v", err)
	}

	if _, err := os.Lstat(UnitPath(storeRoot, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpectedly created a unit directory: err=%v", err)
	}
	fi, err := os.Lstat(ReleasedPath(storeRoot, workspaceID))
	if err != nil {
		t.Fatalf("marker not created for abandoned run: %v", err)
	}
	if !fi.Mode().IsRegular() || fi.Size() != 0 {
		t.Fatalf("marker is not a zero-byte regular file: mode=%v size=%d", fi.Mode(), fi.Size())
	}
}

// --- release cannot land inside a materialization ---

func TestRelease_CannotLandInsideMaterialization(t *testing.T) {
	storeRoot := t.TempDir()
	const workspaceID = "run--0123456789ab"

	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	lockPath := LockPath(storeRoot, workspaceID)

	// Simulate "inside a materialization": the test itself acquires the
	// unit lock, exactly as Materializer.Materialize holds it continuously
	// across its own steps 1-6.
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("simulate materialization lock hold: %v", err)
	}

	rel := NewReleaser(storeRoot)
	if err := rel.Release(workspaceID); err == nil {
		t.Fatal("Release while a simulated materialization holds the lock: want operational error, got nil")
	}
	if _, err := os.Lstat(ReleasedPath(storeRoot, workspaceID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker created while materialization's lock was held: lstat err=%v", err)
	}

	// "Materialization" finishes and releases its lock.
	if err := filelock.Release(lockFile, lockPath); err != nil {
		t.Fatalf("releasing simulated materialization lock: %v", err)
	}

	// Now Release succeeds.
	if err := rel.Release(workspaceID); err != nil {
		t.Fatalf("Release after materialization's lock freed: %v", err)
	}
}
