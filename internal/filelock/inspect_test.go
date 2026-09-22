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

// reapedPID starts and waits a `true` subprocess and returns its pid,
// guaranteed reaped and never confusable with a live process. Named
// distinctly from filelock_test.go's own local variables named "deadPID"
// (2A-M9): a package-level function of that same name would be shadowed,
// not an error, but confusing to read.
func reapedPID(t *testing.T) int {
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
			writeLockInfo(t, p, Info{PID: reapedPID(t), Start: 1})
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
		// R-RR3-6 amended (2A-I3): a YOUNG empty or partial body is
		// LockHeld (mid-flush, Peek's own charity), never LockUndecidable
		// — the contested branch review 2A found untested.
		{"young empty body", func(t *testing.T) string {
			p := filepath.Join(dir, "young-empty.lock")
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}, nil, LockHeld, ""},
		{"young partial body", func(t *testing.T) string {
			p := filepath.Join(dir, "young-partial.lock")
			if err := os.WriteFile(p, []byte(`{"pid":1`), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}, nil, LockHeld, ""},
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
