package filelock

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
