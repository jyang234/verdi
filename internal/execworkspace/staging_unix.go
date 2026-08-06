//go:build unix

package execworkspace

// The unix half of materialization step 6's staging-witness open (see
// writeCompletionWitness in materialize.go). Split out of materialize.go for
// whole-wave finding F3: syscall.O_NOFOLLOW does not exist on every GOOS Go
// can build for, and referencing it unconditionally broke
// `GOOS=windows go build ./...` for the WHOLE MODULE, not just this package.
// The split keeps the unix behavior byte-for-byte identical and moves the
// platform question into the build tags, where a missing platform fails
// CLOSED (staging_other.go) instead of silently weakening the open.

import (
	"os"
	"syscall"
)

// openStagingWitness opens the staging path for step 6's write. O_NOFOLLOW
// is the SECOND guard, independent of writeCompletionWitness's lstat
// pre-check: between that lstat and this open lies a window in which a
// fresh symlink can be planted at the staging path, and without O_NOFOLLOW
// the O_TRUNC would follow it and empty whatever it names. With the flag,
// the kernel refuses (ELOOP) and the caller reports it through the same
// non-regular-object operational error path the lstat pre-check feeds —
// "never followed, never written through" holds across the whole window,
// not just at the instant of the check.
func openStagingWitness(stagingPath string) (*os.File, error) {
	return os.OpenFile(stagingPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
}
