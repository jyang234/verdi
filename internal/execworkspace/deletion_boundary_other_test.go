//go:build !darwin

package execworkspace

import "testing"

// See deletion_boundary_darwin_test.go's own file-level MECHANISM comment:
// isolating ONE flat sibling step's unlink (.request.staging, .request,
// .released, or .lock) — independent of lock acquisition and the other
// fixed-order steps sharing the same parent directory — requires
// per-target immutability (darwin's `chflags UF_IMMUTABLE`, available
// without root). POSIX permission bits on the shared parent cannot do it:
// they would also block lock acquisition (a new file in that same
// parent) and the (separately, already-tested) directory step's own
// final unlink. This platform has no equivalent primitive available
// without root (Linux's nearest analogue, `chattr +i`, is normally
// root/CAP_LINUX_IMMUTABLE-gated), so these four sub-tests are
// documented-skipped here — named, not faked with a coupled mechanism
// that would not actually isolate the step under test — mirroring this
// package's own "skip-if-root pattern used by existing tests" precedent
// (gc_test.go's TestDecideUnit_Rank5_PartialAtDirectoryStep_
// ThenReEntrantFinish) for a platform-primitive gap rather than a
// privilege gap.
func TestDecideUnit_Rank5_DeletionFailureBoundary_StagingStep(t *testing.T) {
	t.Skip("named reason: chflags UF_IMMUTABLE (darwin-only, root not required) is the only available primitive that isolates one flat sibling's unlink from lock acquisition and the other fixed-order steps; see deletion_boundary_darwin_test.go's MECHANISM comment")
}

func TestDecideUnit_Rank5_DeletionFailureBoundary_RequestStep(t *testing.T) {
	t.Skip("named reason: same as the staging-step sub-test above")
}

func TestDecideUnit_Rank5_DeletionFailureBoundary_ReleasedStep(t *testing.T) {
	t.Skip("named reason: same as the staging-step sub-test above")
}

func TestDecideUnit_Rank5_DeletionFailureBoundary_LockStep(t *testing.T) {
	t.Skip("named reason: same as the staging-step sub-test above")
}
