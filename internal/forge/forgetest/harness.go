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
}
