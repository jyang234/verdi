package execworkspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/store"
)

// Shape distinguishes execution-workspace's two materialization request
// shapes (spec §Exact workspace materialization): an exact commit SHA
// materialized as a detached worktree, or a base commit plus a canonical
// patch applied on top of it.
type Shape int

const (
	// ExactSHA is the exact-commit-SHA request shape: {RunID, CommitSHA}.
	ExactSHA Shape = iota
	// BasePlusPatch is the base-commit-plus-patch request shape:
	// {RunID, CommitSHA, PatchBytes}.
	BasePlusPatch
)

// String renders Shape for diagnostics and error messages.
func (s Shape) String() string {
	switch s {
	case ExactSHA:
		return "exact-sha"
	case BasePlusPatch:
		return "base-plus-patch"
	default:
		return fmt.Sprintf("execworkspace.Shape(%d)", int(s))
	}
}

// Identity is a materialization request's full, byte-comparable identity
// (spec §Workspace naming: "IDENTITY IS ALWAYS THE FULL DIGESTS"). RunID is
// the consumer-supplied run identity, unslugged; CommitSHA is always the
// full canonical lowercase 40-hex commit SHA (exact or base); PatchSHA256
// is the full 64-hex lowercase sha256 of the patch bytes for the
// BasePlusPatch shape, and is always empty for ExactSHA.
//
// Construct an Identity only via NewExactIdentity or NewPatchIdentity —
// both validate CommitSHA and, for the patch shape, compute PatchSHA256
// from the supplied bytes — or via DecodeSidecar, which applies the same
// validation to a persisted sidecar's fields.
type Identity struct {
	Shape       Shape
	RunID       string
	CommitSHA   string
	PatchSHA256 string
}

// NewExactIdentity builds the exact-SHA shape's Identity. CommitSHA must be
// the canonical lowercase full 40-hex commit SHA (spec §Workspace naming,
// AD-3); anything else is a fail-closed error. No wall clock, no
// randomness: the same (runID, commitSHA) always yields the same Identity.
func NewExactIdentity(runID, commitSHA string) (Identity, error) {
	if err := validateCommitSHA(commitSHA); err != nil {
		return Identity{}, err
	}
	return Identity{Shape: ExactSHA, RunID: runID, CommitSHA: commitSHA}, nil
}

// NewPatchIdentity builds the base-plus-patch shape's Identity. CommitSHA
// must be the canonical lowercase full 40-hex base commit SHA; patchBytes
// must be non-empty (spec §Workspace naming: "Patch shape requires
// non-empty patch bytes") and is hashed as given — patchBytes is trusted to
// already be the canonical patch representation the caller is responsible
// for producing; this package only hashes it deterministically via sha256,
// no wall clock, no randomness.
func NewPatchIdentity(runID, commitSHA string, patchBytes []byte) (Identity, error) {
	if err := validateCommitSHA(commitSHA); err != nil {
		return Identity{}, err
	}
	if len(patchBytes) == 0 {
		return Identity{}, fmt.Errorf("execworkspace: base-plus-patch request requires non-empty patch bytes")
	}
	sum := sha256.Sum256(patchBytes)
	return Identity{
		Shape:       BasePlusPatch,
		RunID:       runID,
		CommitSHA:   commitSHA,
		PatchSHA256: hex.EncodeToString(sum[:]),
	}, nil
}

// WorkspaceID computes the deterministic <workspace-id> path segment (spec
// §Workspace naming): RefSlug(RunID) + "--" + the first 12 hex characters
// of CommitSHA, plus "-p" + the first 12 hex characters of PatchSHA256 for
// the BasePlusPatch shape. RefSlug is internal/store's single normative
// slugging implementation — never re-derived here. The truncated hexes
// appear only in this path; full-identity comparison must use Equal, never
// WorkspaceID, since truncation alone cannot distinguish two distinct
// requests that happen to collide after truncation.
func (id Identity) WorkspaceID() string {
	base := store.RefSlug(id.RunID) + "--" + id.CommitSHA[:12]
	if id.Shape == BasePlusPatch {
		base += "-p" + id.PatchSHA256[:12]
	}
	return base
}

// Equal reports whether id and other are the SAME request identity by a
// full byte-compare of every identity field, including Shape: two
// identities that agree on RunID and CommitSHA but differ in Shape (one
// exact, one base-plus-patch) are never equal, even though an exact-shape
// Identity always carries an empty PatchSHA256. This is the full-identity
// extension of the RefSlug collision rule (spec §Workspace naming).
func (id Identity) Equal(other Identity) bool {
	return id.Shape == other.Shape &&
		id.RunID == other.RunID &&
		id.CommitSHA == other.CommitSHA &&
		id.PatchSHA256 == other.PatchSHA256
}

// String renders id for diagnostics and error messages (used by
// ErrIdentityMismatch to name both requests it reports on).
func (id Identity) String() string {
	if id.Shape == BasePlusPatch {
		return fmt.Sprintf("{shape:%s run_id:%q commit_sha:%s patch_sha256:%s}", id.Shape, id.RunID, id.CommitSHA, id.PatchSHA256)
	}
	return fmt.Sprintf("{shape:%s run_id:%q commit_sha:%s}", id.Shape, id.RunID, id.CommitSHA)
}

// validateCommitSHA enforces AD-3: CommitSHA is accepted ONLY as the
// canonical lowercase full 40-hex form. Anything else — wrong length,
// uppercase, or a non-hex byte — is a fail-closed error.
func validateCommitSHA(sha string) error {
	if len(sha) != 40 {
		return fmt.Errorf("execworkspace: commit sha %q: want canonical lowercase 40-hex, got %d characters", sha, len(sha))
	}
	for i := 0; i < len(sha); i++ {
		c := sha[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return fmt.Errorf("execworkspace: commit sha %q: not canonical lowercase 40-hex (byte %d = %q)", sha, i, c)
	}
	return nil
}

// validatePatchSHA256 enforces the sidecar's patch digest form: the
// canonical lowercase full 64-hex sha256, mirroring validateCommitSHA's
// discipline for the 40-hex commit form.
func validatePatchSHA256(sum string) error {
	if len(sum) != 64 {
		return fmt.Errorf("execworkspace: patch sha256 %q: want canonical lowercase 64-hex, got %d characters", sum, len(sum))
	}
	for i := 0; i < len(sum); i++ {
		c := sum[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return fmt.Errorf("execworkspace: patch sha256 %q: not canonical lowercase 64-hex (byte %d = %q)", sum, i, c)
	}
	return nil
}
