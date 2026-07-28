//go:build !darwin && !linux

package main

import (
	"fmt"
	"runtime"
)

// renameExclusive on platforms without a known atomic rename-exclusive
// primitive returns an operational error naming the platform rather than
// silently falling back to a bare os.Rename that, on some filesystems,
// would replace an existing destination directory. verdi init's supported
// targets are darwin (renamex_np/RENAME_EXCL) and linux
// (renameat2/RENAME_NOREPLACE); any other GOOS refuses to promote rather
// than risk a non-atomic store claim. The error is deliberately NOT
// os.ErrExist-classified: it is operational trouble (exit 2 via the
// caller's non-existence branch), not a "the store already exists" refusal.
func renameExclusive(oldpath, newpath string) error {
	return fmt.Errorf("atomic exclusive rename is unsupported on %s (verdi init supports darwin and linux only); refusing to promote rather than risk a non-atomic store claim", runtime.GOOS)
}
