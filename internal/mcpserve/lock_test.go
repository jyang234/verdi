package mcpserve

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestAcquireLock_Happy covers a fresh acquisition (writes {pid,start}
// readable back off disk) and a clean release+reacquire cycle.
func TestAcquireLock_Happy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")

	f, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decoding lock file %q: %v", string(data), err)
	}
	if info.PID != os.Getpid() {
		t.Fatalf("lock pid = %d, want %d", info.PID, os.Getpid())
	}
	if info.Start <= 0 {
		t.Fatalf("lock start = %d, want a positive unix timestamp", info.Start)
	}

	if err := ReleaseLock(f, path); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after ReleaseLock: err=%v", err)
	}

	// Reacquire cleanly now that it's released.
	f2, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("AcquireLock after release: %v", err)
	}
	_ = ReleaseLock(f2, path)
}

// TestReleaseLock_Negative covers closing an already-closed file (a
// double-release) and removing a lock file whose parent directory has
// vanished out from under it.
func TestReleaseLock_Negative(t *testing.T) {
	t.Run("already-closed file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := AcquireLock(path)
		if err != nil {
			t.Fatalf("AcquireLock: %v", err)
		}
		_ = f.Close() // close it out from under ReleaseLock
		if err := ReleaseLock(f, path); err == nil {
			t.Fatal("ReleaseLock(already-closed file): want error, got nil")
		}
	})

	t.Run("lock file already gone is not an error (idempotent release)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		f, err := AcquireLock(path)
		if err != nil {
			t.Fatalf("AcquireLock: %v", err)
		}
		_ = os.Remove(path) // simulate the file already having been cleaned up
		if err := ReleaseLock(f, path); err != nil {
			t.Fatalf("ReleaseLock(already-removed lock file): want nil (os.ErrNotExist tolerated), got %v", err)
		}
	})
}

// TestAcquireLock_HeldByLiveProcess proves a lock recording OUR OWN pid
// (definitely alive) with a start timestamp within tolerance of the real
// process start is reported held, not stale — the D3/I-12 "one writer"
// guarantee's negative case.
func TestAcquireLock_HeldByLiveProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "writer.lock")
	info := LockInfo{PID: os.Getpid(), Start: time.Now().Unix()}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding lock file: %v", err)
	}

	_, err := AcquireLock(path)
	if err == nil {
		t.Fatal("AcquireLock(held by live pid): want error, got nil")
	}
	held, ok := err.(*ErrLockHeld)
	if !ok {
		t.Fatalf("AcquireLock error type = %T, want *ErrLockHeld (err=%v)", err, err)
	}
	if held.Info.PID != os.Getpid() {
		t.Fatalf("ErrLockHeld.Info.PID = %d, want %d", held.Info.PID, os.Getpid())
	}
}

// TestAcquireLock_TakeoverAfterDeadPID proves the S4-proven takeover path:
// a lock naming a pid that has exited (spawned and waited on here, so its
// pid is guaranteed reaped and not our own) is treated as stale, removed,
// and reacquired.
func TestAcquireLock_TakeoverAfterDeadPID(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	deadPID := cmd.Process.Pid

	path := filepath.Join(t.TempDir(), "writer.lock")
	info := LockInfo{PID: deadPID, Start: time.Now().Add(-time.Hour).Unix()}
	data, _ := json.Marshal(info)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seeding stale lock file: %v", err)
	}

	f, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("AcquireLock(stale lock, dead pid %d): %v", deadPID, err)
	}
	defer func() { _ = ReleaseLock(f, path) }()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading reacquired lock: %v", err)
	}
	var newInfo LockInfo
	if err := json.Unmarshal(got, &newInfo); err != nil {
		t.Fatalf("decoding reacquired lock: %v", err)
	}
	if newInfo.PID != os.Getpid() {
		t.Fatalf("after takeover, lock pid = %d, want our own pid %d", newInfo.PID, os.Getpid())
	}
}

// TestAcquireLock_Negative covers a malformed lock file (unreadable JSON)
// and a directory that cannot be created under (permission denied).
func TestAcquireLock_Negative(t *testing.T) {
	t.Run("malformed lock file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "writer.lock")
		if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
			t.Fatalf("seeding malformed lock: %v", err)
		}
		if _, err := AcquireLock(path); err == nil {
			t.Fatal("AcquireLock(malformed lock file): want error, got nil")
		}
	})

	t.Run("parent directory does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent-subdir", "writer.lock")
		if _, err := AcquireLock(path); err == nil {
			t.Fatal("AcquireLock(no parent dir): want error, got nil")
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
