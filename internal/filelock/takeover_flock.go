//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package filelock

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"syscall"
	"time"
)

// takeOverStale is SI-302's takeover of a lock acquire has judged stale — a
// dead or reused-pid holder's body, or an empty/partial body older than
// lockMidFlushWindow (01 §D3 makes such a lock "stale and eligible for
// takeover" without saying how). Removing the path by name after judging a
// body read earlier let two detectors that judged the same stale body both
// succeed: the second's remove deleted the lock the first had just created
// (BL-108). Instead, this detector:
//
//  1. opens the lock, and judges the body it reads through that handle —
//     the exact file it will act on — by acquire's own rules (bodyStale);
//  2. takes a non-blocking exclusive flock(2) on that file. EWOULDBLOCK
//     means another detector is taking this very file over, so the lock is
//     treated as held (SI-302): the result is *ErrHeld with a zero Info, as
//     for a lock whose winner has not flushed its body yet. Retrying
//     instead would spin through acquire's small retry budget while the
//     other detector finishes, and end in an operational error rather than
//     the held answer every caller already polls or proxies on;
//  3. under the flock, re-checks that the path still names that file (a
//     detector that finished first has unlinked it and created its own
//     lock, which this detector must not touch), that the body re-read
//     through the handle is byte-identical to the one judged, and that the
//     judged body is still stale (a young mtime means a writer is at work);
//  4. only then unlinks the path. Closing the handle drops the flock.
//
// Only a flock holder unlinks a judged file, and only one process can hold
// the flock on it, so only one detector removes it; a detector that takes
// the flock after that finds the path naming another file and re-evaluates
// from scratch, reading the new holder's live lock.
//
// Holders do not take the flock themselves. The race BL-108 names is
// between detectors, and detectors serialize on the inode they judged. A
// holder's lock is judged stale only when its holder is dead (its pid gone,
// or reused by another process), and a dead holder's flocks are dropped by
// the kernel anyway; a holder flock would only guard a live holder whose
// lock a detector has misjudged stale (BL-109's clock step, or SI-300's
// disclosed fallback), which already means two writers, and would add a
// second liveness authority beside 01 §D3's pid-and-start rule. Nor does
// Release take it: its path-identity check (Release, filelock.go) can race
// only such a misjudging detector, or an older binary's by-name takeover.
//
// It returns nil when the caller must re-evaluate from scratch — the path
// is gone, names another file, holds another body, is no longer stale, or
// this detector has unlinked it — and the caller's exclusive-create retry
// spends one unit of acquire's retry budget. It returns *ErrHeld when
// another detector holds the takeover flock, and any other error when the
// lock could not be judged, flocked, or unlinked; nothing is removed then.
//
// Residuals (SI-302): an older binary still removes the path by name, so in
// a mixed-version store it can delete a lock this protocol created, and it
// can do so between this detector's re-check and its unlink. And the flock
// does not change what an aged empty body means: a writer stalled for more
// than lockMidFlushWindow between its exclusive create and its flush is
// still judged crashed, exactly as before, since it takes no flock either.
func takeOverStale(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening it: %w", err)
	}
	defer func() { _ = f.Close() }()

	judged, stale, err := judgeOpenLock(f)
	if err != nil {
		return err
	}
	if !stale {
		return nil
	}
	takeoverHook(takeoverJudged)
	if err := lockFlock(f); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return &ErrHeld{}
		}
		return fmt.Errorf("taking its takeover flock: %w", err)
	}
	still, err := stillStaleUnderFlock(f, path, judged)
	if err != nil {
		return err
	}
	if !still {
		return nil
	}
	takeoverHook(takeoverRechecked)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unlinking it: %w", err)
	}
	return nil
}

// judgeOpenLock reads the lock body through f and judges it by bodyStale
// against f's own modification time, returning the body it judged.
func judgeOpenLock(f *os.File) (body []byte, stale bool, err error) {
	body, err = readOpenLock(f)
	if err != nil {
		return nil, false, fmt.Errorf("reading it: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("stating it: %w", err)
	}
	return body, bodyStale(body, st.ModTime()), nil
}

// stillStaleUnderFlock is takeOverStale's re-check under the flock: path
// still names f (os.Lstat, so a symlink at path never counts as f), the body
// re-read through f is byte-identical to judged, and judged is still stale
// against f's current modification time. Any mismatch is (false, nil): the
// caller re-evaluates from scratch.
func stillStaleUnderFlock(f *os.File, path string, judged []byte) (bool, error) {
	held, err := f.Stat()
	if err != nil {
		return false, fmt.Errorf("stating it under its takeover flock: %w", err)
	}
	disk, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stating its path under its takeover flock: %w", err)
	}
	if !os.SameFile(held, disk) {
		return false, nil
	}
	now, err := readOpenLock(f)
	if err != nil {
		return false, fmt.Errorf("re-reading it under its takeover flock: %w", err)
	}
	if !bytes.Equal(now, judged) {
		return false, nil
	}
	return bodyStale(judged, held.ModTime()), nil
}

// bodyStale is acquire's staleness rule for one lock body: a complete body
// is stale when its holder is not alive (alive keeps its kill-probe-only
// fallback, so an undecided probe is never stale); an empty or truncated
// body is stale when modTime lies outside lockMidFlushWindow (a writer that
// crashed between its exclusive create and its flush); a complete but
// garbled body is never stale here — re-evaluation reports it malformed.
func bodyStale(body []byte, modTime time.Time) bool {
	var info Info
	err := strictUnmarshal(body, &info)
	switch {
	case err == nil:
		return !alive(info.PID, info.Start)
	case lockBodyIncomplete(err):
		return !modifiedWithinMidFlushWindow(modTime)
	default:
		return false
	}
}

// readOpenLock reads f's whole body from offset 0, whatever f's own offset.
func readOpenLock(f *os.File) ([]byte, error) {
	return io.ReadAll(io.NewSectionReader(f, 0, math.MaxInt64))
}

// lockFlock takes a non-blocking exclusive flock(2) on f. A var only so a
// test can make it fail with an error other than EWOULDBLOCK.
var lockFlock = func(f *os.File) error {
	rc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var ferr error
	if cerr := rc.Control(func(fd uintptr) {
		ferr = syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB)
	}); cerr != nil {
		return cerr
	}
	return ferr
}

// takeoverStep names the two points of takeOverStale a test can stop a
// detector at through takeoverHook.
type takeoverStep int

const (
	// takeoverJudged: the file was judged stale through its own handle;
	// the flock is not taken yet.
	takeoverJudged takeoverStep = iota
	// takeoverRechecked: the re-check under the flock passed; the path is
	// not unlinked yet.
	takeoverRechecked
)

// takeoverHook is called as a detector passes each takeoverStep. A no-op in
// production; a var only so a test can interleave two detectors or change
// the lock between a detector's steps.
var takeoverHook = func(takeoverStep) {}
