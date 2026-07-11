// Package forgetest is the forge port's shared contract-test suite (04
// §Testing's pattern applied to I-22): both the gitlab and github adapters
// run it against an httptest double of their own forge's API, proving they
// satisfy the same behavioral contract the fake also satisfies.
package forgetest

import (
	"testing"

	"github.com/OWNER/verdi/internal/forge"
)

// Harness lets Run drive an adapter under test without knowing which forge
// it targets. Implementations should return fresh, isolated state from
// each call the NewHarness constructor passed to Run makes.
type Harness interface {
	// Forge returns the forge.Forge under test.
	Forge() forge.Forge
	// SeedBundle arranges for FetchEvidenceBundle(ref, commit) to
	// succeed with bundle, via whatever means the underlying forge API
	// double requires (e.g. registering an httptest handler response).
	SeedBundle(t *testing.T, ref, commit string, bundle forge.EvidenceBundle)
	// WantGeneratedAttribute is the token Forge().GeneratedAttribute()
	// must return for this forge kind.
	WantGeneratedAttribute() string
	// SeedOpenMR arranges for ListOpenMRs(ctx, targetBranch) to include an
	// open MR/PR with the given source branch and title (V1-P3's open-MR
	// port extension, openmr.go).
	SeedOpenMR(t *testing.T, targetBranch, sourceBranch, title string)
	// SeedFile arranges for FetchFileAtRef(ref, path) to succeed with
	// content.
	SeedFile(t *testing.T, ref, path string, content []byte)
	// SeedComment arranges for ListComments(mrID) to include c already
	// present (V1-P7's comment-round-trip extension, comments.go). If
	// c.ThreadID is non-empty, an unresolved ThreadResolution for it must
	// also become visible via GetThreadResolution unless
	// SeedThreadResolution overrides it — mirroring both real forges,
	// where a diff-anchored comment always belongs to a thread that
	// exists (unresolved) from the moment it is created.
	SeedComment(t *testing.T, mrID string, c forge.Comment)
	// SeedThreadResolution arranges for GetThreadResolution(mrID) to
	// report tr for tr.ThreadID, overriding any auto-created entry from
	// SeedComment.
	SeedThreadResolution(t *testing.T, mrID string, tr forge.ThreadResolution)
}
