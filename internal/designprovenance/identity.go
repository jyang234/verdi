package designprovenance

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// Identity names one sidecar's spec, lifecycle-zone path, and fixed context
// exclusion classification.
type Identity struct {
	Spec            string
	RelPath         string
	ExclusionReason string
}

// ResolveIdentity derives the committed sidecar identity for an active or
// archived unpinned whole-spec reference.
func ResolveIdentity(spec, zone string) (Identity, error) {
	ref, err := artifact.ParseRef(spec)
	if err != nil {
		return Identity{}, fmt.Errorf("designprovenance: spec identity: %w", err)
	}
	if ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return Identity{}, fmt.Errorf("designprovenance: %q must be an unpinned whole spec ref", spec)
	}
	if zone != store.ZoneActive && zone != store.ZoneArchive {
		return Identity{}, fmt.Errorf("designprovenance: unknown spec zone %q", zone)
	}
	return Identity{
		Spec:            spec,
		RelPath:         store.DesignProvenanceRelPath(zone, ref.Name),
		ExclusionReason: ExclusionReason,
	}, nil
}
