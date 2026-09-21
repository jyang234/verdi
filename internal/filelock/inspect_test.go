package filelock

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeLockInfo marshals info as JSON to path, mirroring the on-disk shape
// Acquire itself writes.
func writeLockInfo(t *testing.T, path string, info Info) {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshaling lock info: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing lock file: %v", err)
	}
}

// deadPID starts and waits a `true` subprocess and returns its pid,
// guaranteed reaped and never confusable with a live process.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatalf("running short-lived child: %v", err)
	}
	return cmd.Process.Pid
}

func TestInspect_Table(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		setup  func(t *testing.T) string
		ps     func(pid int) (time.Time, error)
		want   LockStatus
		reason string
	}{
		{"absent", func(t *testing.T) string { return filepath.Join(dir, "absent.lock") }, nil, LockAbsent, ""},
		{"held by this process", func(t *testing.T) string {
			p := filepath.Join(dir, "held.lock")
			writeLockInfo(t, p, Info{PID: os.Getpid(), Start: time.Now().Unix()})
			return p
		}, nil, LockHeld, ""},
		{"stale dead pid", func(t *testing.T) string {
			p := filepath.Join(dir, "stale.lock")
			writeLockInfo(t, p, Info{PID: deadPID(t), Start: 1})
			return p
		}, nil, LockStale, "pid"},
		{"undecidable ps", func(t *testing.T) string {
			p := filepath.Join(dir, "undecidable.lock")
			writeLockInfo(t, p, Info{PID: os.Getpid(), Start: 1})
			return p
		}, func(int) (time.Time, error) { return time.Time{}, errors.New("ps unavailable") }, LockUndecidable, "ps unavailable"},
		{"old empty body", func(t *testing.T) string {
			p := filepath.Join(dir, "empty.lock")
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-time.Minute)
			if err := os.Chtimes(p, old, old); err != nil {
				t.Fatal(err)
			}
			return p
		}, nil, LockStale, "empty lock body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ps != nil {
				orig := psLstart
				psLstart = tc.ps
				t.Cleanup(func() { psLstart = orig })
			}
			got, err := Inspect(tc.setup(t))
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != tc.want || !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("got %+v, want %s/%q", got, tc.want, tc.reason)
			}
		})
	}
}

// TestInspect_MalformedBody mirrors Peek's own negative case: a
// complete-but-garbled body is a hard error, never a guessed status.
func TestInspect_MalformedBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malformed.lock")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("seeding malformed lock: %v", err)
	}
	if _, err := Inspect(path); err == nil {
		t.Fatal("Inspect(malformed lock) = nil error, want an error")
	}
}
