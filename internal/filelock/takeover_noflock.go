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
// path, the second deleting the lock the first has just created, and both
// then hold the lock. Release's path-identity check (filelock.go) applies
// here too.
func takeOverStale(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
