// I-12's writer lock: exactly one `verdi serve` process per checkout may
// hold `data/writer.lock`. Base algorithm (O_CREATE|O_EXCL JSON
// {pid,start}; kill(pid,0) liveness probe; takeover on a dead holder) is
// the wave-4 S4 spike's proven design (read-only reference, reimplemented
// here). This file additionally closes S4's documented PID-reuse gap: a
// live-but-not-the-same-process pid cross-checked against the OS's own
// notion of that process's start time via `ps -o lstart=`, with a
// documented fallback to kill-probe-only when ps's output is unparseable
// (PLAN.md Phase 9 exit criteria).
package mcpserve

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// LockInfo is I-12(a)'s lock body: O_CREATE|O_EXCL JSON {pid, start}.
type LockInfo struct {
	PID   int   `json:"pid"`
	Start int64 `json:"start"` // unix seconds the lock was created
}

// ErrLockHeld means the lock is held by a live process: the caller should
// proxy (dial the socket) rather than serve standalone.
type ErrLockHeld struct {
	Info LockInfo
}

func (e *ErrLockHeld) Error() string {
	return fmt.Sprintf("mcpserve: writer lock held by live pid %d (started %s)", e.Info.PID, time.Unix(e.Info.Start, 0).Format(time.RFC3339))
}

// lockStartTolerance bounds how far a live pid's actual process start time
// (per `ps -o lstart=`) may drift from the lock's recorded start before
// it is treated as a DIFFERENT process that happens to have reused the
// pid, rather than the lock's genuine holder. Generous on purpose: the
// real holder's own startup work (index build, socket bind) between
// process start and lock-write can itself take some seconds; a true
// pid-reuse collision is expected to differ by much more than this in
// practice (a different, unrelated process started at an unrelated time).
const lockStartTolerance = 5 * time.Minute

// psLstart execs `ps -o lstart= -p <pid>` and parses its stdout as the
// named process's actual start time — the cross-check I-12 asks for.
// Overridable in tests (both to avoid a real ps dependency in some paths
// and to exercise the "ps output unparseable" fallback deterministically).
var psLstart = func(pid int) (time.Time, error) {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("mcpserve: ps -o lstart= -p %d: %w", pid, err)
	}
	return parseLstart(strings.TrimSpace(string(out)))
}

// lstartLayouts are the reference-time layouts `ps -o lstart=` is known to
// emit (BSD/macOS and GNU/Linux both print "Www Mmm [ ]d HH:MM:SS YYYY" in
// the C/POSIX locale; "_2" absorbs the space-padded single-digit day both
// platforms use).
var lstartLayouts = []string{
	"Mon Jan _2 15:04:05 2006",
	"Mon Jan 2 15:04:05 2006",
}

func parseLstart(s string) (time.Time, error) {
	for _, layout := range lstartLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("mcpserve: unparseable ps -o lstart= output %q", s)
}

// alive reports whether pid names a live process that is plausibly the
// SAME process that wrote recordedStart, closing S4's documented
// PID-reuse gap. First a classic kill(pid,0) liveness probe (no signal
// delivered, existence/permission only); a dead pid short-circuits to
// false. For a live pid, cross-check its actual start time against
// recordedStart within lockStartTolerance — a live pid whose actual start
// time drifts far from the lock's recorded start is a DIFFERENT process
// that reused the pid, so it is reported not-alive (the lock is stale,
// eligible for takeover). When ps's output cannot be obtained or parsed,
// the documented fallback is kill-probe-only: report alive (the narrow,
// disclosed limitation S4 and PLAN.md's ledger both name).
func alive(pid int, recordedStart int64) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid) // always succeeds on Unix; not the real check
	if err != nil {
		return false
	}
	sigErr := proc.Signal(syscall.Signal(0))
	switch {
	case sigErr == nil:
		// Exists; fall through to the start-time cross-check.
	case errors.Is(sigErr, os.ErrProcessDone):
		return false
	case errors.Is(sigErr, syscall.ESRCH):
		return false
	case errors.Is(sigErr, syscall.EPERM):
		// Exists, we just can't signal it — still alive. ps may also be
		// permission-restricted for this pid; the fallback below covers it.
	default:
		return false
	}

	actual, perr := psLstart(pid)
	if perr != nil {
		return true // documented fallback: kill-probe-only
	}
	diff := actual.Unix() - recordedStart
	if diff < 0 {
		diff = -diff
	}
	return time.Duration(diff)*time.Second <= lockStartTolerance
}

// AcquireLock implements I-12(a) end to end: create path with
// O_CREATE|O_EXCL and write {pid,start} JSON on success. If the path
// already exists, inspect the holder recorded inside — alive (per alive,
// above) yields ErrLockHeld (the caller should proxy instead of serving);
// dead/stale removes the lock and retries acquisition, up to a small
// bound (guards a takeover race between two simultaneous stale
// detectors: both remove+recreate, only one O_EXCL create wins, the loser
// retries and finds the winner's fresh live lock).
func AcquireLock(path string) (*os.File, error) {
	return acquireLock(path, 5)
}

func acquireLock(path string, retriesLeft int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		info := LockInfo{PID: os.Getpid(), Start: time.Now().Unix()}
		if encErr := json.NewEncoder(f).Encode(info); encErr != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("mcpserve: writing lock %s: %w", path, encErr)
		}
		return f, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("mcpserve: acquiring lock %s: %w", path, err)
	}

	data, rerr := os.ReadFile(path)
	if rerr != nil {
		// Lost the race with the remover between our OpenFile and this Read.
		if errors.Is(rerr, os.ErrNotExist) && retriesLeft > 0 {
			return acquireLock(path, retriesLeft-1)
		}
		return nil, fmt.Errorf("mcpserve: lock %s exists but is unreadable: %w", path, rerr)
	}
	// Strict decode (spec/fail-loud ac-3/dc-2): LockInfo is a file verdi
	// itself writes (AcquireLock, above), so an unexpected extra field is
	// never a forward-compat signal to tolerate — it means a malformed or
	// foreign lock file, and the read should refuse it by name rather than
	// silently drop the field.
	var info LockInfo
	if jerr := strictUnmarshal(data, &info); jerr != nil {
		return nil, fmt.Errorf("mcpserve: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
	}
	if alive(info.PID, info.Start) {
		return nil, &ErrLockHeld{Info: info}
	}
	if retriesLeft <= 0 {
		return nil, fmt.Errorf("mcpserve: stale lock %s (pid %d dead) but exceeded takeover retries", path, info.PID)
	}
	if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return nil, fmt.Errorf("mcpserve: stale lock %s (pid %d dead) but could not remove: %w", path, info.PID, rmErr)
	}
	return acquireLock(path, retriesLeft-1)
}

// ReleaseLock closes f and removes path — the holder's own clean-shutdown
// path (SIGTERM/SIGINT handling in cmd/verdi/serve.go). A crash leaves
// the lock behind on disk exactly as I-12 intends: the next acquirer's
// alive() probe discovers the dead pid and takes over.
func ReleaseLock(f *os.File, path string) error {
	if cerr := f.Close(); cerr != nil {
		return fmt.Errorf("mcpserve: closing lock %s: %w", path, cerr)
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("mcpserve: removing lock %s: %w", path, rerr)
	}
	return nil
}
