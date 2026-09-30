//go:build !(darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd)

package filelock

import (
	"errors"
	"os"
)

// takeOverStale on a platform without flock(2) (Windows, AIX, Solaris, and
// the other GOOS values outside this file's build constraint) keeps the
// takeover as it was before SI-302: it removes the stale lock by name, and
// the caller's exclusive-create retry follows.
//
// Disclosure (SI-302, BL-108): on these platforms the takeover race stays
// open. Two detectors that judged the same stale lock can both remove the
// path by name after the first has already created its own lock there;
// what the second's remove then does depends on the platform:
//
//   - On Windows, Go opens files without FILE_SHARE_DELETE, and the first
//     detector keeps its new lock open for as long as it holds it, so the
//     second's remove fails with a sharing violation: its acquisition ends
//     in an operational error (the stale lock could not be taken over)
//     rather than leaving two holders.
//   - On AIX, Solaris, and the other platforms here, the second's remove
//     deletes the lock the first has just created, and both then hold the
//     lock.
//
// Release's path-identity check (filelock.go) applies here too.
func takeOverStale(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
