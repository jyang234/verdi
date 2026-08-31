package filelock

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAcquire_Happy covers a fresh acquisition (writes {pid,start}
// readable back off disk) and a clean release+reacquire cycle.
func TestAcquire_Happy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")

	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decoding lock file %q: %v", string(data), err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("lock pid = %d, want %d", info.PID, os.Getpid())
	}
	if info.Start <= 0 {
		t.Fatalf("lock start = %d, want a positive unix timestamp", info.Start)
	}

	if err := Release(f, path); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after Release: err=%v", err)
	}

	// Reacquire cleanly now that it's released.
	f2, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire after release: %v", err)
	}
	_ = Release(f2, path)
}

// TestRelease_Negative covers closing an already-closed file (a
// double-release) and removing a lock file whose parent directory has
// vanished out from under it.
func TestRelease_Negative(t *testing.T) {
	t.Run("already-closed file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := Acquire(path)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		_ = f.Close() // close it out from under Release
		if err := Release(f, path); err == nil {
			t.Fatal("Release(already-closed file): want error, got nil")
		}
	})

	t.Run("lock file already gone is not an error (idempotent release)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := Acquire(path)
		if err != nil {
			t.Fatalf("Acquire: %v", err)
		}
		_ = os.Remove(path) // simulate the file already having been cleaned up
		if err := Release(f, path); err != nil {
			t.Fatalf("Release(already-removed lock file): want nil (os.ErrNotExist tolerated), got %v", err)
		}
	})
}

// TestAcquire_HeldByLiveProcess proves a lock recording OUR OWN pid
// (definitely alive) with a start timestamp within tolerance of the real
// process start is reported held, not stale — the D3/I-12 "one writer"
// guarantee's negative case.
func TestAcquire_HeldByLiveProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	info := Info{PID: os.Getpid(), Start: time.Now().Unix()}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding lock file: %v", err)
	}

	_, err := Acquire(path)
	if err == nil {
		t.Fatal("Acquire(held by live pid): want error, got nil")
	}
	held, ok := err.(*ErrHeld)
	if !ok {
		t.Fatalf("Acquire error type = %T, want *ErrHeld (err=%v)", err, err)
	}
	if held.Info.PID != os.Getpid() {
		t.Fatalf("ErrHeld.Info.PID = %d, want %d", held.Info.PID, os.Getpid())
	}
}

// TestAcquire_TakeoverAfterDeadPID proves the S4-proven takeover path: a
// lock naming a pid that has exited (spawned and waited on here, so its
// pid is guaranteed reaped and not our own) is treated as stale, removed,
// and reacquired.
func TestAcquire_TakeoverAfterDeadPID(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	deadPID := cmd.Process.Pid

	path := filepath.Join(t.TempDir(), "writer.lock")
	info := Info{PID: deadPID, Start: time.Now().Add(-time.Hour).Unix()}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding stale lock file: %v", err)
	}

	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire(stale lock, dead pid %d): %v", deadPID, err)
	}
	defer func() { _ = Release(f, path) }()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading reacquired lock: %v", err)
	}
	var newInfo Info
	if err := json.Unmarshal(got, &newInfo); err != nil {
		t.Fatalf("decoding reacquired lock: %v", err)
	}
	if newInfo.PID != os.Getpid() {
		t.Fatalf("after takeover, lock pid = %d, want our own pid %d", newInfo.PID, os.Getpid())
	}
}

// TestAcquire_Negative covers a malformed lock file (unreadable JSON) and
// a directory that cannot be created under (missing parent).
func TestAcquire_Negative(t *testing.T) {
	t.Run("malformed lock file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
			t.Fatalf("seeding malformed lock: %v", err)
		}
		if _, err := Acquire(path); err == nil {
			t.Fatal("Acquire(malformed lock file): want error, got nil")
		}
	})

	t.Run("parent directory does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent-subdir", "writer.lock")
		if _, err := Acquire(path); err == nil {
			t.Fatal("Acquire(no parent dir): want error, got nil")
		}
	})

	// spec/fail-loud ac-3/dc-2's strict-decode posture, preserved verbatim
	// by this extraction: Info is a file this package itself writes, so an
	// unrecognized field means a malformed/foreign lock file, not a
	// forward-compat member to tolerate — Acquire must refuse it BY NAME.
	t.Run("lock file has an unknown field (strict decode refuses it by name)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		seed := `{"pid":1,"start":2,"holder_reff":"bogus"}`
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			t.Fatalf("seeding lock file with unknown field: %v", err)
		}
		_, err := Acquire(path)
		if err == nil {
			t.Fatal("Acquire(lock file with unknown field): want error, got nil")
		}
		if !strings.Contains(err.Error(), "holder_reff") {
			t.Fatalf("Acquire error does not NAME the unknown field %q: %v", "holder_reff", err)
		}
	})
}

// TestAlive_PIDReuseCrossCheck exercises the I-12 PID-reuse close: our own
// pid (always live) with a wildly mismatched recorded start is reported
// NOT alive, because the real `ps -o lstart=` cross-check for our own
// process's true start time will not fall within tolerance of a bogus
// recorded start far in the past. Skipped if ps is unavailable or its
// output doesn't parse on this platform, since the fallback path (tested
// separately below) covers that case explicitly.
func TestAlive_PIDReuseCrossCheck(t *testing.T) {
	if _, err := psLstart(os.Getpid()); err != nil {
		t.Skipf("ps -o lstart= unavailable/unparseable on this platform: %v", err)
	}
	bogusStart := int64(0) // 1970-01-01: no real process here started then
	if alive(os.Getpid(), bogusStart) {
		t.Fatal("alive(self pid, bogus 1970 start) = true, want false (PID-reuse cross-check should catch this)")
	}
}

// TestAlive_FallsBackToKillProbeWhenPSUnparseable proves the documented
// fallback: when ps's output cannot be obtained or parsed, alive()
// reports true for a genuinely live pid rather than incorrectly claiming
// staleness.
func TestAlive_FallsBackToKillProbeWhenPSUnparseable(t *testing.T) {
	orig := psLstart
	defer func() { psLstart = orig }()
	psLstart = func(pid int) (time.Time, error) {
		return time.Time{}, os.ErrInvalid // simulate an unparseable/unavailable ps
	}
	if !alive(os.Getpid(), 0) {
		t.Fatal("alive(self pid, ps unavailable) = false, want true (documented kill-probe-only fallback)")
	}
}

// TestAlive_DeadPIDIsNeverAliveRegardlessOfPS proves a dead pid is never
// reported alive even under the ps-fallback path — the kill probe alone
// is authoritative for "definitely dead".
func TestAlive_DeadPIDIsNeverAliveRegardlessOfPS(t *testing.T) {
	orig := psLstart
	defer func() { psLstart = orig }()
	psLstart = func(pid int) (time.Time, error) {
		return time.Now(), nil // even if ps somehow "succeeds"
	}
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	if alive(cmd.Process.Pid, time.Now().Unix()) {
		t.Fatal("alive(dead pid) = true, want false")
	}
}

// TestAcquire_YoungEmptyBodyIsHeld proves the mid-flush-window semantic: a
// lock file that EXISTS but whose {pid,start} body has not landed yet (empty,
// i.e. the winner is between O_CREATE|O_EXCL and its flush) is reported HELD,
// never a hard malformed error — so the losing acquirer keeps polling instead
// of failing. This is the exact race the CI flake hit (empty "" body → EOF).
func TestAcquire_YoungEmptyBodyIsHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	// Fresh, empty (just-created) lock body: the create landed, the flush
	// has not. mtime is now, so it is inside lockMidFlushWindow.
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seeding young empty lock: %v", err)
	}

	_, err := Acquire(path)
	if err == nil {
		t.Fatal("Acquire(young empty body): want ErrHeld, got nil (would have cut a second worktree)")
	}
	if _, ok := err.(*ErrHeld); !ok {
		t.Fatalf("Acquire(young empty body) error type = %T, want *ErrHeld: %v", err, err)
	}
	// Must NOT have been taken over/removed: the holder is still mid-write.
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("young empty lock was removed, want it left for the mid-flush holder: %v", statErr)
	}
}

// TestAcquire_OldEmptyBodyIsStaleTakeover proves the crash-recovery half of
// the same semantic: an empty/partial body OLDER than lockMidFlushWindow is a
// writer that crashed between create and flush. No {pid} survives for a
// liveness probe, so age is the honest staleness signal — the lock is taken
// over (removed and reacquired under our own pid), not left held forever.
func TestAcquire_OldEmptyBodyIsStaleTakeover(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("seeding empty lock: %v", err)
	}
	// Backdate mtime well past the mid-flush window: a crashed writer.
	old := time.Now().Add(-lockMidFlushWindow - time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("backdating empty lock mtime: %v", err)
	}

	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire(old empty body): want stale takeover, got error: %v", err)
	}
	defer func() { _ = Release(f, path) }()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading reacquired lock: %v", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decoding reacquired lock %q: %v", string(data), err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("after takeover, lock pid = %d, want our own pid %d", info.PID, os.Getpid())
	}
}

// TestPeek_YoungAndOldEmptyBody proves Peek honours the same mid-flush window:
// a young empty body is held=true (a live holder mid-write), an old empty body
// is held=false (a crashed writer, treated like no lock) — and neither is
// reported as a malformed error, unlike a complete-but-garbled body.
func TestPeek_YoungAndOldEmptyBody(t *testing.T) {
	t.Run("young empty body reports held", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "worktree.lock")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("seeding young empty lock: %v", err)
		}
		_, held, err := Peek(path)
		if err != nil {
			t.Fatalf("Peek(young empty body): want no error, got %v", err)
		}
		if !held {
			t.Fatal("Peek(young empty body) held = false, want true (holder mid-flush)")
		}
	})

	t.Run("old empty body reports not held (stale)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "worktree.lock")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("seeding empty lock: %v", err)
		}
		old := time.Now().Add(-lockMidFlushWindow - time.Minute)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatalf("backdating empty lock mtime: %v", err)
		}
		_, held, err := Peek(path)
		if err != nil {
			t.Fatalf("Peek(old empty body): want no error, got %v", err)
		}
		if held {
			t.Fatal("Peek(old empty body) held = true, want false (crashed writer, stale)")
		}
	})
}

// TestAcquire_YoungGarbledBodyStaysMalformed proves the boundary the window
// does NOT cross: a complete-but-garbled body (valid bytes, not a truncation)
// is a hard malformed error even when freshly written — only empty/truncated
// bodies get the age-based charity, so real corruption is never masked as HELD.
func TestAcquire_YoungGarbledBodyStaysMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("seeding young garbled lock: %v", err)
	}
	_, err := Acquire(path)
	if err == nil {
		t.Fatal("Acquire(young garbled body): want malformed error, got nil")
	}
	if _, ok := err.(*ErrHeld); ok {
		t.Fatalf("Acquire(young garbled body) returned ErrHeld, want a hard malformed error: %v", err)
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("Acquire(young garbled body) error = %v, want a malformed error", err)
	}
}

// TestPeek_NoFile proves Peek reports (Info{}, false, nil) for a lock path
// that does not exist — not held, not an error.
func TestPeek_NoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.lock")
	info, held, err := Peek(path)
	if err != nil {
		t.Fatalf("Peek(no file): %v", err)
	}
	if held {
		t.Fatal("Peek(no file) reported held, want false")
	}
	if info != (Info{}) {
		t.Fatalf("Peek(no file) info = %+v, want zero value", info)
	}
}

// TestPeek_LiveLock proves Peek reports held=true for a lock recording a
// live pid, WITHOUT removing or otherwise mutating the lock file (a
// second Peek immediately after must see the exact same content).
func TestPeek_LiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = Release(f, path) }()

	info, held, err := Peek(path)
	if err != nil {
		t.Fatalf("Peek(live lock): %v", err)
	}
	if !held {
		t.Fatal("Peek(live lock) reported not held, want true")
	}
	if info.PID != os.Getpid() {
		t.Fatalf("Peek(live lock).PID = %d, want %d", info.PID, os.Getpid())
	}

	// Peek must be read-only: the file must still be there, unchanged.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file vanished after Peek: %v", err)
	}
}

// TestPeek_StaleLock proves Peek reports held=false for a lock naming a
// dead pid, WITHOUT taking it over (the file is left exactly as found —
// gc performs its own explicit Acquire when it actually wants to remove
// something; Peek alone must never delete a stale lock out from under a
// concurrent Acquire-based takeover).
func TestPeek_StaleLock(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	deadPID := cmd.Process.Pid

	path := filepath.Join(t.TempDir(), "worktree.lock")
	info := Info{PID: deadPID, Start: time.Now().Add(-time.Hour).Unix()}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding stale lock file: %v", err)
	}

	gotInfo, held, err := Peek(path)
	if err != nil {
		t.Fatalf("Peek(stale lock): %v", err)
	}
	if held {
		t.Fatal("Peek(stale lock) reported held, want false")
	}
	if gotInfo.PID != deadPID {
		t.Fatalf("Peek(stale lock).PID = %d, want %d (Peek must not mutate)", gotInfo.PID, deadPID)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Peek removed the stale lock file, want it left untouched: %v", err)
	}
}

// TestPeek_Negative covers a malformed lock file: Peek must refuse it
// rather than silently reporting "not held".
func TestPeek_Negative(t *testing.T) {
	path := filepath.Join(t.TempDir(), "worktree.lock")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("seeding malformed lock: %v", err)
	}
	if _, _, err := Peek(path); err == nil {
		t.Fatal("Peek(malformed lock file): want error, got nil")
	}
}

// TestHeldByCurrentProcess_TrueOnlyForOurExactOpenHandle proves SI-177's
// registry-proven query: after our own successful Acquire, the query
// reports true for that exact path — and false again immediately after
// Release, since the registered handle is deregistered before the lock
// file is even removed.
func TestHeldByCurrentProcess_TrueOnlyForOurExactOpenHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	held, err := HeldByCurrentProcess(path)
	if err != nil || !held {
		t.Fatalf("HeldByCurrentProcess(after Acquire) = %t, %v, want true, nil", held, err)
	}
	if err := Release(f, path); err != nil {
		t.Fatalf("Release: %v", err)
	}
	held, err = HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(after Release) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_NoFileNeverAcquired proves a path this process
// never called Acquire on reports false, not an error — the ordinary
// "nothing to prove" case, distinct from the forged-lock case below.
func TestHeldByCurrentProcess_NoFileNeverAcquired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	held, err := HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(never acquired) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_ForgedLockWithOurOwnPIDIsNeverHeld is SI-177's
// "forged-lock rule": a lock file naming this process's own real pid/start
// bytes — but written directly to disk, never through this process's own
// successful Acquire — must NOT be reported held. Matching PID/start bytes
// alone is never ownership proof; only the registry is.
func TestHeldByCurrentProcess_ForgedLockWithOurOwnPIDIsNeverHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	info := Info{PID: os.Getpid(), Start: time.Now().Unix()}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding forged lock file: %v", err)
	}
	held, err := HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(forged, our own pid, no registry entry) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_ReplacedFileIsNotHeld proves that once the
// registered lock path is removed and a NEW file (even one with identical
// bytes) takes its place, the query reports false: the registry proves
// ownership of an exact open file identity, not of a path spelling.
func TestHeldByCurrentProcess_ReplacedFileIsNotHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = f.Close() }()

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("recreating lock with identical bytes: %v", err)
	}

	held, err := HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(removed+recreated, identical bytes) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_RemovedFileIsNotHeld proves a registered lock
// whose path has simply been removed (never recreated) also reports
// false, not an error.
func TestHeldByCurrentProcess_RemovedFileIsNotHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	held, err := HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(removed, not recreated) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_SymlinkToTheStillOpenFileIsNotHeld pins the
// documented symlink guarantee against mutation: it is not enough for
// path's ultimate TARGET to be our still-open registered file — path
// itself must directly name that exact open file identity. Here the
// registered file is renamed aside (the open *os.File handle stays valid,
// still referring to the exact same inode) and a symlink is placed at the
// ORIGINAL path pointing at that renamed-aside file. A query using
// os.Stat (which follows symlinks) would incorrectly resolve the symlink
// to the same inode and report true; only os.Lstat's refusal to follow
// the symlink — comparing the SYMLINK's own identity, not its target's —
// correctly reports false. This is exactly the mutation
// (os.Lstat -> os.Stat at HeldByCurrentProcess's disk-side stat) that
// must fail this test.
func TestHeldByCurrentProcess_SymlinkToTheStillOpenFileIsNotHeld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = f.Close() }()

	aside := filepath.Join(dir, "writer.lock.real")
	if err := os.Rename(path, aside); err != nil {
		t.Fatalf("renaming the registered lock aside: %v", err)
	}
	if err := os.Symlink(aside, path); err != nil {
		t.Fatalf("symlinking path back at the still-open registered file: %v", err)
	}

	held, err := HeldByCurrentProcess(path)
	if err != nil || held {
		t.Fatalf("HeldByCurrentProcess(symlink at path pointing at the still-open registered file) = %t, %v, want false, nil", held, err)
	}
}

// TestHeldByCurrentProcess_ClosedRegisteredHandleIsAnError drives a real
// stat failure into HeldByCurrentProcess's held.Stat() error branch
// (filelock.go's "stating held lock handle" path): the registry still
// names a handle, but that handle was closed out from under it (bypassing
// Release, which would have deregistered it first) — held.Stat() on an
// already-closed *os.File fails. The query must return the honest error,
// never a guessed boolean either way.
func TestHeldByCurrentProcess_ClosedRegisteredHandleIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing the registered handle directly: %v", err)
	}
	t.Cleanup(func() {
		deregisterAcquired(path, f)
		_ = os.Remove(path)
	})

	held, err := HeldByCurrentProcess(path)
	if err == nil {
		t.Fatalf("HeldByCurrentProcess(closed registered handle) = %t, nil, want a non-nil error (never a guessed boolean)", held)
	}
	if held {
		t.Fatalf("HeldByCurrentProcess(closed registered handle) = true, %v, want false alongside the error", err)
	}
}

// TestHeldByCurrentProcess_UnreadableLockPathIsAnError drives a real stat
// failure into HeldByCurrentProcess's os.Lstat(path) error branch (the
// "stating lock path" path, distinct from held.Stat()'s branch above): the
// registered handle itself is perfectly healthy, but the lock path's
// parent directory has been made unreadable, so os.Lstat(path) fails with
// something other than os.ErrNotExist. The query must return the honest
// error rather than reporting either boolean.
func TestHeldByCurrentProcess_UnreadableLockPathIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permission bits do not restrict access")
	}
	parent := t.TempDir()
	sub := filepath.Join(parent, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer func() { _ = f.Close() }()

	if err := os.Chmod(sub, 0o000); err != nil {
		t.Fatalf("removing directory permissions: %v", err)
	}
	defer func() { _ = os.Chmod(sub, 0o755) }()

	held, err := HeldByCurrentProcess(path)
	if err == nil {
		t.Skipf("HeldByCurrentProcess(unreadable lock directory) = %t, nil — this environment does not enforce directory permission bits (want a non-nil error where it does)", held)
	}
	if held {
		t.Fatalf("HeldByCurrentProcess(unreadable lock directory) = true, %v, want false alongside the error", err)
	}
}

// leaseCount reports the outstanding lease count registered for path — a
// white-box read used only to prove that failed/erroring lease attempts
// leak nothing.
func leaseCount(t *testing.T, path string) int {
	t.Helper()
	registryMu.Lock()
	defer registryMu.Unlock()
	entry, ok := registry[filepath.Clean(path)]
	if !ok {
		return -1
	}
	return entry.leases
}

// TestLeaseIfHeldByCurrentProcess_MirrorsTheOwnershipQuery pins that the
// lease form answers exactly what HeldByCurrentProcess answers on every
// not-held shape — never acquired, forged with our own pid, released, or
// replaced at the path — and that each such refusal hands back no release
// func and leaves no lease behind.
func TestLeaseIfHeldByCurrentProcess_MirrorsTheOwnershipQuery(t *testing.T) {
	t.Run("never acquired", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		release, held, err := LeaseIfHeldByCurrentProcess(path)
		if release != nil || held || err != nil {
			t.Fatalf("LeaseIfHeldByCurrentProcess(never acquired) = release!=nil %t, held %t, err %v, want false, false, nil", release != nil, held, err)
		}
	})

	t.Run("forged lock with our own pid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		data, err := json.Marshal(Info{PID: os.Getpid(), Start: time.Now().Unix()})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		release, held, err := LeaseIfHeldByCurrentProcess(path)
		if release != nil || held || err != nil {
			t.Fatalf("LeaseIfHeldByCurrentProcess(forged) = release!=nil %t, held %t, err %v, want false, false, nil", release != nil, held, err)
		}
	})

	t.Run("after release", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := Acquire(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := Release(f, path); err != nil {
			t.Fatal(err)
		}
		release, held, err := LeaseIfHeldByCurrentProcess(path)
		if release != nil || held || err != nil {
			t.Fatalf("LeaseIfHeldByCurrentProcess(after Release) = release!=nil %t, held %t, err %v, want false, false, nil", release != nil, held, err)
		}
	})

	t.Run("replaced file leaves no lease behind", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := Acquire(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = Release(f, path) }()
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, original, 0o644); err != nil {
			t.Fatal(err)
		}
		release, held, err := LeaseIfHeldByCurrentProcess(path)
		if release != nil || held || err != nil {
			t.Fatalf("LeaseIfHeldByCurrentProcess(replaced file) = release!=nil %t, held %t, err %v, want false, false, nil", release != nil, held, err)
		}
		if n := leaseCount(t, path); n != 0 {
			t.Fatalf("outstanding leases after a refused lease = %d, want 0 (a refused proof must drop its provisional lease)", n)
		}
	})

	t.Run("closed registered handle is an error with no lease left", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := Acquire(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			deregisterAcquired(path, f)
			_ = os.Remove(path)
		})
		release, held, err := LeaseIfHeldByCurrentProcess(path)
		if err == nil {
			t.Fatalf("LeaseIfHeldByCurrentProcess(closed handle) = release!=nil %t, held %t, err nil, want a non-nil error", release != nil, held)
		}
		if release != nil || held {
			t.Fatalf("LeaseIfHeldByCurrentProcess(closed handle) = release!=nil %t, held %t, err %v, want false, false alongside the error", release != nil, held, err)
		}
		if n := leaseCount(t, path); n != 0 {
			t.Fatalf("outstanding leases after an errored lease = %d, want 0", n)
		}
	})
}

// TestLease_BlocksReleaseUntilDrained is the kernel half of Codex's
// shutdown finding: while a lease is outstanding, the owner's Release must
// not complete, must leave the lock file on disk (so no other process can
// acquire the checkout), and must complete promptly once the lease drops.
func TestLease_BlocksReleaseUntilDrained(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	release, held, err := LeaseIfHeldByCurrentProcess(path)
	if err != nil || !held {
		t.Fatalf("LeaseIfHeldByCurrentProcess(our own lock) = %t, %v, want true, nil", held, err)
	}

	released := make(chan error, 1)
	go func() { released <- Release(f, path) }()

	select {
	case relErr := <-released:
		t.Fatalf("Release completed while a lease was outstanding (err=%v)", relErr)
	case <-time.After(250 * time.Millisecond):
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("lock file removed while a lease was outstanding: %v", statErr)
	}
	// A blocked Release must not deregister early either: ownership is
	// continuously provable for as long as the lease lasts.
	if ok, herr := HeldByCurrentProcess(path); herr != nil || !ok {
		t.Fatalf("HeldByCurrentProcess during a blocked Release = %t, %v, want true, nil", ok, herr)
	}

	release()
	select {
	case relErr := <-released:
		if relErr != nil {
			t.Fatalf("Release after the lease drained: %v", relErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Release never completed after its last lease was released")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("lock file still present after the drained Release: err=%v", statErr)
	}
}

// TestLease_DoubleReleaseIsInert pins the documented lease-release
// discipline: calling the returned func more than once is idempotent, so a
// defensive extra call can never decrement another caller's lease. A
// second, independent lease taken alongside it must still hold Release.
func TestLease_DoubleReleaseIsInert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	first, held, err := LeaseIfHeldByCurrentProcess(path)
	if err != nil || !held {
		t.Fatalf("first lease = %t, %v, want true, nil", held, err)
	}
	second, held, err := LeaseIfHeldByCurrentProcess(path)
	if err != nil || !held {
		t.Fatalf("second lease = %t, %v, want true, nil", held, err)
	}
	if n := leaseCount(t, path); n != 2 {
		t.Fatalf("outstanding leases = %d, want 2", n)
	}

	first()
	first()
	first()
	if n := leaseCount(t, path); n != 1 {
		t.Fatalf("outstanding leases after three calls of one lease's release = %d, want 1 (double release must be inert)", n)
	}

	released := make(chan error, 1)
	go func() { released <- Release(f, path) }()
	select {
	case relErr := <-released:
		t.Fatalf("Release completed while the second lease was still outstanding (err=%v)", relErr)
	case <-time.After(250 * time.Millisecond):
	}

	second()
	select {
	case relErr := <-released:
		if relErr != nil {
			t.Fatalf("Release after both leases drained: %v", relErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Release never completed after both leases were released")
	}
}

// TestLease_ReleaseWithoutLeasesNeverWaits is the negative/no-regression
// half: the overwhelmingly common Release — no lease was ever taken — must
// not wait at all, and a lease taken and dropped before Release leaves no
// residue.
func TestLease_ReleaseWithoutLeasesNeverWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	release, held, err := LeaseIfHeldByCurrentProcess(path)
	if err != nil || !held {
		t.Fatalf("lease = %t, %v, want true, nil", held, err)
	}
	release()

	done := make(chan error, 1)
	go func() { done <- Release(f, path) }()
	select {
	case relErr := <-done:
		if relErr != nil {
			t.Fatalf("Release with no outstanding leases: %v", relErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Release with no outstanding leases blocked")
	}
}

// TestLease_ConcurrentLeasesAndReleaseAreRaceClean hammers the lease
// accounting under -race: many concurrent take/drop cycles against one
// registered handle, with the owner's Release racing them. Every lease
// must drain and Release must return exactly once, without a data race and
// without hanging.
func TestLease_ConcurrentLeasesAndReleaseAreRaceClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	f, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 32; j++ {
				release, held, lerr := LeaseIfHeldByCurrentProcess(path)
				if lerr != nil {
					t.Errorf("lease: %v", lerr)
					return
				}
				if !held {
					return // the owner's Release won; nothing left to lease
				}
				release()
			}
		}()
	}

	released := make(chan error, 1)
	go func() { released <- Release(f, path) }()
	wg.Wait()
	select {
	case relErr := <-released:
		if relErr != nil {
			t.Fatalf("Release racing concurrent leases: %v", relErr)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("Release racing concurrent leases never completed")
	}
}
