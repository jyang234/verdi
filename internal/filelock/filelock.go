// Package filelock implements I-12's per-checkout writer-lock algorithm as
// a shared primitive (CLAUDE.md: "anything used by two or more packages
// lives in a shared internal/ package"). It was born as
// internal/mcpserve/lock.go, guarding exactly one `verdi serve`/`verdi mcp`
// process per checkout's data/writer.lock; spec/worktree-manager dc-2
// widens its packaging — not its algorithm — so a second caller
// (internal/wtmanager, guarding one managed-worktree lockfile per design
// branch) can use the EXACT SAME O_CREATE|O_EXCL {pid,start} JSON body,
// kill(pid,0)-plus-`ps -o lstart=` liveness cross-check, and stale-lock
// takeover without copy-pasting a second implementation. Base algorithm is
// the wave-4 S4 spike's proven design (read-only reference, reimplemented
// here); the PID-reuse gap closure (ps -o lstart= cross-check, with a
// documented kill-probe-only fallback) is this package's own addition, per
// PLAN.md Phase 9 exit criteria.
package filelock

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// Info is the lock body: O_CREATE|O_EXCL JSON {pid, start}.
type Info struct {
	PID   int   `json:"pid"`
	Start int64 `json:"start"` // unix seconds the lock was created
}

// ErrHeld means the lock is held by a live process: the caller should
// proxy (dial the socket) or reuse the winner's result rather than
// proceeding as if it owned the resource.
type ErrHeld struct {
	Info Info
}

func (e *ErrHeld) Error() string {
	return fmt.Sprintf("filelock: lock held by live pid %d (started %s)", e.Info.PID, time.Unix(e.Info.Start, 0).Format(time.RFC3339))
}

// strictUnmarshal decodes raw into dst with DisallowUnknownFields and
// trailing-data rejection, delegating to internal/artifact's own
// DecodeStrictJSON rather than reimplementing the same json.NewDecoder
// posture a second time. A lock file is one this package itself writes
// (Acquire, below), so an unexpected extra field is never a forward-compat
// signal to tolerate — it means a malformed or foreign lock file, and the
// read refuses it BY NAME rather than silently dropping the field
// (mirrors spec/fail-loud's strict-decode posture for verdi-owned files).
func strictUnmarshal(raw []byte, dst any) error {
	return artifact.DecodeStrictJSON(raw, dst)
}

// lockStartTolerance bounds how far a live pid's actual process start time
// (per `ps -o lstart=`) may drift from the lock's recorded start before
// it is treated as a DIFFERENT process that happens to have reused the
// pid, rather than the lock's genuine holder. Generous on purpose: the
// real holder's own startup work between process start and lock-write can
// itself take some seconds; a true pid-reuse collision is expected to
// differ by much more than this in practice (a different, unrelated
// process started at an unrelated time).
const lockStartTolerance = 5 * time.Minute

// psLstart execs `ps -o lstart= -p <pid>` and parses its stdout as the
// named process's actual start time — the cross-check I-12 asks for.
// Overridable in tests (both to avoid a real ps dependency in some paths
// and to exercise the "ps output unparseable" fallback deterministically).
var psLstart = func(pid int) (time.Time, error) {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("filelock: ps -o lstart= -p %d: %w", pid, err)
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
	return time.Time{}, fmt.Errorf("filelock: unparseable ps -o lstart= output %q", s)
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

// Acquire implements I-12(a) end to end: create path with O_CREATE|O_EXCL
// and write {pid,start} JSON on success. If the path already exists,
// inspect the holder recorded inside — alive (per alive, above) yields
// ErrHeld (the caller should proxy/reuse rather than proceed);
// dead/stale removes the lock and retries acquisition, up to a small
// bound (guards a takeover race between two simultaneous stale
// detectors: both remove+recreate, only one O_EXCL create wins, the loser
// retries and finds the winner's fresh live lock).
func Acquire(path string) (*os.File, error) {
	return acquire(path, 5)
}

func acquire(path string, retriesLeft int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		info := Info{PID: os.Getpid(), Start: time.Now().Unix()}
		if encErr := json.NewEncoder(f).Encode(info); encErr != nil {
			_ = f.Close()
			_ = os.Remove(path)
			return nil, fmt.Errorf("filelock: writing lock %s: %w", path, encErr)
		}
		return f, nil
	}
	if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("filelock: acquiring lock %s: %w", path, err)
	}

	data, rerr := os.ReadFile(path)
	if rerr != nil {
		// Lost the race with the remover between our OpenFile and this Read.
		if errors.Is(rerr, os.ErrNotExist) && retriesLeft > 0 {
			return acquire(path, retriesLeft-1)
		}
		return nil, fmt.Errorf("filelock: lock %s exists but is unreadable: %w", path, rerr)
	}
	info, jerr := decodeLockInfo(path, data)
	if jerr != nil {
		// A decode failure is a HARD malformed error ONLY for a
		// complete-but-garbled body. An empty or truncated body is the
		// signature of a mid-flush partial write: Acquire's own
		// O_CREATE|O_EXCL makes the path exist an instant before the winner
		// flushes its {pid,start} JSON, so a racing acquirer can read the
		// bytes-not-landed-yet file (the ""/EOF this closes). Resolve that by
		// the file's age rather than failing hard.
		if !lockBodyIncomplete(jerr) {
			return nil, fmt.Errorf("filelock: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
		}
		young, serr := lockFileYoung(path)
		if serr != nil {
			// Lost the race with a concurrent remover between our read and
			// this stat — retry acquisition rather than fail hard.
			if errors.Is(serr, os.ErrNotExist) && retriesLeft > 0 {
				return acquire(path, retriesLeft-1)
			}
			return nil, fmt.Errorf("filelock: lock %s exists but its empty/partial body could not be aged: %w", path, serr)
		}
		if young {
			// Freshly created, not-yet-flushed: HELD, never a hard error —
			// the winner is mid-write, so the caller must keep polling. No
			// {pid} body has landed yet, hence the zero-value Info.
			return nil, &ErrHeld{Info: Info{}}
		}
		// An empty/partial body older than the mid-flush window is a writer
		// that crashed between create and flush. No {pid} survives for a
		// liveness probe, so age is the honest staleness signal: take it over
		// exactly like a dead-pid lock.
		if retriesLeft <= 0 {
			return nil, fmt.Errorf("filelock: stale lock %s (empty/partial body older than %s) but exceeded takeover retries", path, lockMidFlushWindow)
		}
		if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
			return nil, fmt.Errorf("filelock: stale lock %s (empty/partial body) but could not remove: %w", path, rmErr)
		}
		return acquire(path, retriesLeft-1)
	}
	if alive(info.PID, info.Start) {
		return nil, &ErrHeld{Info: info}
	}
	if retriesLeft <= 0 {
		return nil, fmt.Errorf("filelock: stale lock %s (pid %d dead) but exceeded takeover retries", path, info.PID)
	}
	if rmErr := os.Remove(path); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return nil, fmt.Errorf("filelock: stale lock %s (pid %d dead) but could not remove: %w", path, info.PID, rmErr)
	}
	return acquire(path, retriesLeft-1)
}

// Release closes f and removes path — the holder's own clean path. A
// crash leaves the lock behind on disk exactly as I-12 intends: the next
// acquirer's alive() probe discovers the dead pid and takes over.
func Release(f *os.File, path string) error {
	if cerr := f.Close(); cerr != nil {
		return fmt.Errorf("filelock: closing lock %s: %w", path, cerr)
	}
	if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("filelock: removing lock %s: %w", path, rerr)
	}
	return nil
}

// Peek reports whether path currently names a LIVE lock, without
// creating, removing, or otherwise mutating anything on disk — a
// read-only liveness check for a caller (spec/worktree-manager's `gc`,
// dc-2/dc-4) that needs to know "is this held right now" without racing
// Acquire's own create/takeover side effects. It returns (Info{}, false,
// nil) both when no lock file exists at all and when one exists but its
// recorded holder is not alive (stale) — gc treats a stale lock exactly
// like no lock at all (Acquire's own stale-takeover semantics), without
// actually taking it over here: gc performs its own explicit Acquire
// immediately before its own mutating git call, never relying on a Peek
// result alone to decide it is safe to remove anything.
func Peek(path string) (Info, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Info{}, false, nil
		}
		return Info{}, false, fmt.Errorf("filelock: peeking lock %s: %w", path, err)
	}
	info, jerr := decodeLockInfo(path, data)
	if jerr != nil {
		// Same mid-flush window Acquire honours (above): a complete-but-garbled
		// body is a hard malformed error, but an empty/partial body is judged
		// by age — young = a live holder still mid-write (held); old = a
		// crashed writer (stale, reported not-held exactly like no lock, per
		// gc's Peek contract).
		if !lockBodyIncomplete(jerr) {
			return Info{}, false, fmt.Errorf("filelock: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
		}
		young, serr := lockFileYoung(path)
		if serr != nil {
			if errors.Is(serr, os.ErrNotExist) {
				return Info{}, false, nil // removed under us: no lock at all
			}
			return Info{}, false, fmt.Errorf("filelock: lock %s exists but its empty/partial body could not be aged: %w", path, serr)
		}
		return Info{}, young, nil
	}
	return info, alive(info.PID, info.Start), nil
}

// lockMidFlushWindow bounds how long after a lock file's last modification an
// empty or truncated (mid-flush) body is still charitably read as "the winner
// is mid-write, HELD" rather than "a writer crashed between O_CREATE|O_EXCL
// and its flush, stale". Conservative on purpose: the real mid-flush gap is
// sub-millisecond (one Encode call), so 2s is enormously generous for a live
// holder yet still lets a genuinely crashed writer's empty lock be taken over
// promptly. An empty body carries no {pid} for a liveness probe, so age is the
// only honest staleness signal available for it.
const lockMidFlushWindow = 2 * time.Second

// lockBodyIncomplete reports whether a decode error is the signature of a
// mid-flush partial write — an empty file (io.EOF) or a truncated JSON prefix
// (io.ErrUnexpectedEOF) — as opposed to a complete-but-garbled body (a syntax
// or unknown-field error), which is a genuine malformed lock and stays a hard
// error. artifact.DecodeStrictJSON wraps the underlying error with %w, so
// errors.Is sees through it.
func lockBodyIncomplete(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// lockFileYoung reports whether path was last modified within
// lockMidFlushWindow of now — recent enough that an empty/partial body is a
// live holder still mid-write rather than a crashed one. The stat error is
// returned unwrapped so the caller can distinguish os.ErrNotExist (the file
// was removed out from under us — lost a race) from a real stat failure.
func lockFileYoung(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return time.Since(st.ModTime()) <= lockMidFlushWindow, nil
}

// lockDecodeRetries/lockDecodeRetryDelay bound decodeLockInfo's tolerance
// for a benign, extremely short race: Acquire's own O_CREATE|O_EXCL
// succeeds (the path exists) a moment before its JSON body is fully
// flushed, so a concurrent reader (another Acquire call racing for the
// same lock, or a Peek) can observe a transiently empty or truncated
// file. 25 attempts at 2ms apart bounds the wait at ~50ms — generous
// under -race/heavy goroutine contention, still tiny next to any real
// git-worktree-mutating call this lock actually guards (dc-2).
const (
	lockDecodeRetries    = 25
	lockDecodeRetryDelay = 2 * time.Millisecond
)

// decodeLockInfo strict-decodes data (already read from path) as Info. If
// that fails, it re-reads path a bounded number of times before giving
// up — closing the transient partial-write race described above — and
// returns the LAST attempt's decode error if every retry still fails
// (a genuinely malformed or foreign lock file decodes the same way every
// time, so this adds bounded latency to that case, never a wrong
// answer).
func decodeLockInfo(path string, data []byte) (Info, error) {
	var info Info
	err := strictUnmarshal(data, &info)
	if err == nil {
		return info, nil
	}
	for i := 0; i < lockDecodeRetries; i++ {
		time.Sleep(lockDecodeRetryDelay)
		data2, rerr := os.ReadFile(path)
		if rerr != nil {
			continue // e.g. removed by a concurrent takeover; keep retrying within budget
		}
		err = strictUnmarshal(data2, &info)
		if err == nil {
			return info, nil
		}
	}
	return Info{}, err
}
