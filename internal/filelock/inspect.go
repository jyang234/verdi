package filelock

import (
	"errors"
	"fmt"
	"os"
)

// LockStatus is Inspect's closed, read-only answer about a lock path.
type LockStatus string

const (
	LockAbsent      LockStatus = "absent"
	LockHeld        LockStatus = "held"
	LockStale       LockStatus = "stale"
	LockUndecidable LockStatus = "undecidable"
)

// Inspection is Inspect's read-only answer. Reason is a sentence for
// LockStale (the liveness evidence) and LockUndecidable (why the probe
// could not decide), "" otherwise.
type Inspection struct {
	Status LockStatus
	Info   Info
	Reason string
}

// Inspect reads path without acquiring or taking over anything. It
// distinguishes the states Peek deliberately collapses (R-RR3-6): a
// complete body whose pid is not alive is LockStale; a young empty or
// partial body is LockHeld (mid-flush, exactly Peek's charity); an old
// empty body is LockStale with reason "empty lock body older than 2s"; a
// live pid whose start time cannot be cross-checked (ps unparseable) is
// LockUndecidable with the ps error as reason — never reported stale.
// Peek's own behavior and tests are unchanged: Peek still calls alive,
// which keeps its documented kill-probe-only fallback.
func Inspect(path string) (Inspection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Inspection{Status: LockAbsent}, nil
		}
		return Inspection{}, fmt.Errorf("filelock: inspecting lock %s: %w", path, err)
	}

	info, jerr := decodeLockInfo(path, data)
	if jerr != nil {
		// Same mid-flush window Acquire/Peek honour: a complete-but-garbled
		// body is a hard malformed error, but an empty/partial body is judged
		// by age.
		if !lockBodyIncomplete(jerr) {
			return Inspection{}, fmt.Errorf("filelock: lock %s exists but is malformed (%q): %w", path, string(data), jerr)
		}
		young, serr := lockFileYoung(path)
		if serr != nil {
			if errors.Is(serr, os.ErrNotExist) {
				return Inspection{Status: LockAbsent}, nil // removed under us: no lock at all
			}
			return Inspection{}, fmt.Errorf("filelock: lock %s exists but its empty/partial body could not be aged: %w", path, serr)
		}
		if young {
			return Inspection{Status: LockHeld}, nil
		}
		return Inspection{Status: LockStale, Reason: fmt.Sprintf("empty lock body older than %s", lockMidFlushWindow)}, nil
	}

	isAlive, decided, reason := probe(info.PID, info.Start)
	switch {
	case !decided:
		return Inspection{Status: LockUndecidable, Info: info, Reason: reason}, nil
	case isAlive:
		return Inspection{Status: LockHeld, Info: info}, nil
	default:
		return Inspection{Status: LockStale, Info: info, Reason: reason}, nil
	}
}
