package execworkspace

// Release implements SI-16's release operation (spec §GC slice, §Safe
// cleanup): the durable, mechanically checkable signal a consuming
// feature's own lifecycle uses to authorize a later `verdi gc` reclaim.
//
// DESIGN CHOICE (documented per the contract): Release lives on a dedicated
// Releaser type, not on *Materializer. A Materializer requires a
// Reconciler and a git repoRoot (NewMaterializer refuses a nil
// Reconciler) — neither of which Release ever needs, since release never
// touches git's worktree registry at all. Forcing every release call
// through a Materializer would force every caller to also supply a
// Reconciler and repoRoot it would never use. Releaser needs only
// storeRoot, exactly like the sidecar/grammar helpers in this package.
//
// Release is an OPERATIONAL FACT, never a proof, a verdict, or a
// ratification: creating data/execution/<workspace-id>.released requires
// the consuming feature's own lifecycle to have already produced whatever
// durable record its authority demands (for CSE, the durably recorded
// human decision) — a record this component never inspects and never
// interprets.
import (
	"errors"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/filelock"
)

// Releaser performs the release operation for one store root's
// data/execution/ tree. Construct with NewReleaser.
type Releaser struct {
	storeRoot string
}

// NewReleaser builds a Releaser over storeRoot (the directory containing
// .verdi/, under which data/execution/ lives — see ExecutionRoot).
func NewReleaser(storeRoot string) *Releaser {
	return &Releaser{storeRoot: storeRoot}
}

// Release durably records that workspaceID's consuming feature has
// finished with it, by creating data/execution/<workspaceID>.released — a
// ZERO-BYTE REGULAR FILE created O_CREATE|O_EXCL, this store's own
// write-once idiom (store-layout D3's data/writer.lock precedent).
//
// LOCKING: Release acquires the unit's .lock (filelock.Acquire,
// NON-BLOCKING) before creating the marker and releases it (filelock.Release
// — the lock FILE is removed) immediately after — the same per-operation
// discipline materialization and the gc reclaim follow (spec: "held only
// for that operation"). This is NOT gc's fused reclaim deletion of the same
// lock (§Safe cleanup's fixed reclaim order, where the final `.lock`
// deletion IS the reclaim's own release of the lock); Release always
// performs an ordinary, independent filelock.Release. Because
// materialization holds the SAME lock continuously across its own steps
// 1-6, a release can never land inside a materialization: ANY acquisition
// failure here — a live holder, an undecodable lock body, or any other
// filelock.Acquire error — is an operational error, disclosed and
// retryable, and NEVER a wait; the marker is never created outside the
// lock.
//
// IDEMPOTENCE: Release succeeds when O_CREATE|O_EXCL creates the marker,
// and on EEXIST when the existing object at the marker path IS a regular
// file (its content is IGNORED — existence is the entire record, so even a
// nonempty regular file there still witnesses release, and nothing is ever
// decoded). EEXIST against a directory, a symlink, or any other
// non-regular object is an OPERATIONAL ERROR: a consumer is never told a
// wedged marker path was a successful release.
//
// Release may be invoked regardless of materialization completeness —
// including for an id with NOTHING AT ALL on disk yet (an abandoned run):
// the consuming feature owns WHEN release happens, never this component,
// and this call still creates the marker under the lock.
//
// ID VALIDATION FIRST. Release takes a RAW workspaceID from its caller, so
// it gates that id on ValidWorkspaceID (grammar.go — the package's ONE
// <workspace-id> shape test) BEFORE assembling any path and before creating
// the execution root. The path assemblers are pure grammar helpers that
// join whatever they are handed, so without this gate an id carrying a path
// segment — "../writer" — would resolve LockPath and ReleasedPath OUT of
// data/execution/ and operate on data/writer.lock, the store-layout writer
// lock, leaving a data/writer.released marker beside it. A refused id is an
// operational error with NO filesystem effect whatsoever.
func (r *Releaser) Release(workspaceID string) error {
	if !ValidWorkspaceID(workspaceID) {
		return operationalError("release: workspace id", fmt.Errorf(
			"%q is not a valid <workspace-id>: the grammar is `<run-slug>--<sha12>` or `<run-slug>--<sha12>-p<patch12>` over the store's normative slug alphabet, with a non-empty slug and 12 lowercase hex digits per group (spec §Workspace naming; ValidWorkspaceID) — no path is assembled from an unvalidated id",
			workspaceID))
	}

	if err := os.MkdirAll(ExecutionRoot(r.storeRoot), 0o755); err != nil {
		return operationalError("release: prepare execution root", err)
	}

	lockPath := LockPath(r.storeRoot, workspaceID)
	lockFile, acqErr := filelock.Acquire(lockPath)
	if acqErr != nil {
		return operationalError("release: acquire lock", acqErr)
	}

	releaseErr := r.releaseMarkerLocked(workspaceID)

	if relErr := filelock.Release(lockFile, lockPath); relErr != nil {
		if releaseErr == nil {
			return operationalError("release: release lock", relErr)
		}
		// The flow's own error wins (same priority Materialize's deferred
		// release uses); Release has no Result to append a second
		// disclosure line to, so the lock-release failure is dropped in
		// favor of the more actionable marker-creation failure already
		// being returned.
	}
	return releaseErr
}

// releaseMarkerLocked creates workspaceID's .released marker with the unit
// lock already held, implementing the O_CREATE|O_EXCL-then-EEXIST-lstat
// idiom described on Release above.
func (r *Releaser) releaseMarkerLocked(workspaceID string) error {
	markerPath := ReleasedPath(r.storeRoot, workspaceID)

	f, err := os.OpenFile(markerPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		if cerr := f.Close(); cerr != nil {
			// vocab:identity — "close" names os.File.Close on the marker's file handle, not the close CLI verb.
			return operationalError("release: close marker", cerr)
		}
		return nil
	}
	if !errors.Is(err, os.ErrExist) {
		return operationalError("release: create marker", err)
	}

	// EEXIST: lstat-type the existing object — never a following stat, so a
	// symlink at the marker path is a non-regular object, never followed.
	kind, lerr := LstatType(markerPath)
	if lerr != nil {
		return operationalError("release: lstat existing marker", lerr)
	}
	if kind != PathRegular {
		return operationalError("release: existing marker", fmt.Errorf("marker %q is %s, not a regular file: never treated as a successful release", markerPath, kind))
	}
	// A regular file, whatever its content, is idempotent success — content
	// is IGNORED, existence is the entire record.
	return nil
}
