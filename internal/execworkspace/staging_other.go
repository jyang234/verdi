//go:build !unix

package execworkspace

// The non-unix half of materialization step 6's staging-witness open (see
// staging_unix.go for the split's rationale, whole-wave finding F3).
//
// FAIL CLOSED, never a weakened open: the unix implementation's O_NOFOLLOW is
// a REQUIRED guard, not an optimization — it is what makes the spec's "never
// followed, never written through" hold across the lstat->open window rather
// than only at the instant of the lstat. A platform where this package has no
// equivalent primitive wired up therefore gets an OPERATIONAL ERROR naming
// the gap, so materialization refuses on that platform instead of shipping a
// symlink-followable open that would look like it worked.

import (
	"fmt"
	"os"
	"runtime"
)

// openStagingWitness refuses on every non-unix platform, naming the missing
// primitive. It never returns a usable file: an open without the
// no-follow guarantee is not a degraded version of this operation, it is a
// different (unsafe) one.
func openStagingWitness(stagingPath string) (*os.File, error) {
	return nil, operationalError(
		"materialize: open staging witness",
		fmt.Errorf(
			"no no-follow open primitive is wired up for GOOS=%s, so the staging path %q cannot be opened with the required symlink refusal (O_NOFOLLOW on unix); refusing rather than opening a followable path",
			runtime.GOOS, stagingPath,
		),
	)
}
