// Package nobuild never builds: its test file references an undefined
// identifier, so `go test -json` reports build-output and build-fail events
// and a package fail carrying FailedBuild, and no test runs.
package nobuild

import "testing"

func TestNeverBuilds(t *testing.T) {
	undefinedOnPurpose(t)
}
