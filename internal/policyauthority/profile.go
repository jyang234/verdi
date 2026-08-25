package policyauthority

import (
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// SelectedProfile returns a deep-cloned, kernel-sealed copy of the selected
// accepted governance profile. It re-proves the loaded store and stored
// profile before egress; callers never receive slices aliased to Store.
func (s *Store) SelectedProfile() (governanceprincipal.Profile, error) {
	if s == nil || !s.sealed {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: selected profile requires a Load-produced store")
	}
	if err := crossValidate(s); err != nil {
		return governanceprincipal.Profile{}, err
	}
	stored := s.Profiles[s.Constitution.SelectedProfile]
	digest, err := stored.Profile.Digest()
	if err != nil {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: selected profile %q: %w", stored.ID, err)
	}
	if digest != stored.ProfileDigest {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: selected profile %q digest %q does not match stored digest %q", stored.ID, digest, stored.ProfileDigest)
	}
	catalog, err := s.Constitution.GovernanceCatalog()
	if err != nil {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: constitution governance catalog: %w", err)
	}
	canonical, err := canonjson.Marshal(stored.Profile)
	if err != nil {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: cloning selected profile %q: %w", stored.ID, err)
	}
	clone, err := governanceprincipal.DecodeProfile(canonical, catalog)
	if err != nil {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: cloning selected profile %q: %w", stored.ID, err)
	}
	cloneDigest, err := clone.Digest()
	if err != nil {
		return governanceprincipal.Profile{}, err
	}
	if cloneDigest != digest {
		return governanceprincipal.Profile{}, fmt.Errorf("policyauthority: selected profile %q clone digest changed from %q to %q", stored.ID, digest, cloneDigest)
	}
	return clone, nil
}
