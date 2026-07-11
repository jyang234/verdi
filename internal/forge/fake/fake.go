// Package fake is a hermetic, in-memory Forge double (04 §Testing's
// pattern applied to the I-22 forge port): no HTTP, no network, used by
// `verdi sync`'s own tests and anywhere else a Forge is needed without a
// real GitLab/GitHub server.
package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/OWNER/verdi/internal/forge"
)

// Forge is a configurable, in-memory forge.Forge.
type Forge struct {
	mu sync.Mutex

	bundles   map[string]forge.EvidenceBundle
	attribute string
	ci        forge.CIInfo
}

// New returns an empty Forge: no bundles seeded, GeneratedAttribute
// returns "fake-generated", CIContext returns a zero CIInfo.
func New() *Forge {
	return &Forge{
		bundles:   make(map[string]forge.EvidenceBundle),
		attribute: "fake-generated",
	}
}

func bundleKey(ref, commit string) string { return ref + "@" + commit }

// SeedBundle makes FetchEvidenceBundle(ref, commit) succeed with bundle.
func (f *Forge) SeedBundle(ref, commit string, bundle forge.EvidenceBundle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[bundleKey(ref, commit)] = bundle
}

// SetGeneratedAttribute overrides GeneratedAttribute's return value.
func (f *Forge) SetGeneratedAttribute(attr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attribute = attr
}

// SetCIContext overrides CIContext's return value.
func (f *Forge) SetCIContext(info forge.CIInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ci = info
}

// FetchEvidenceBundle implements forge.Forge.
func (f *Forge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forge.EvidenceBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	b, ok := f.bundles[bundleKey(ref, commit)]
	if !ok {
		return nil, fmt.Errorf("fake: no bundle seeded for ref %q commit %q: %w", ref, commit, forge.ErrNoBundle)
	}
	return &b, nil
}

// GeneratedAttribute implements forge.Forge.
func (f *Forge) GeneratedAttribute() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attribute
}

// CIContext implements forge.Forge.
func (f *Forge) CIContext(ctx context.Context) (forge.CIInfo, error) {
	if err := ctx.Err(); err != nil {
		return forge.CIInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ci, nil
}

var _ forge.Forge = (*Forge)(nil)
