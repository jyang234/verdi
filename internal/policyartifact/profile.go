package policyartifact

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// StoredProfile is the constitution store's record of one
// governance-profile artifact (OD-1/DC-20: profiles live in this store
// as typed policy artifacts; the kernel owns their schema and rule
// semantics, this feature owns their storage, identity, digest, and
// recording). The stored file's frontmatter IS the kernel's profile
// document — decoded exclusively by governanceprincipal.DecodeProfile,
// never by a second decoder here — and the body carries the profile's
// human rationale.
type StoredProfile struct {
	// ID is the profile's own kernel id — the storage identity this
	// feature records; the file's name stem must match it (Load).
	ID string
	// ProfileDigest is the kernel-sealed canonical content digest —
	// the digest recorded into manifests, approvals, and receipts
	// (DC-19).
	ProfileDigest string
	// Profile is the kernel's sealed, validated profile value.
	Profile governanceprincipal.Profile
	// Rationale is the stored artifact's body prose.
	Rationale string
}

// DecodeStoredProfile decodes one stored governance-profile artifact:
// frontmatter through the kernel's DecodeProfile against the store's
// registered governance catalog, body as required rationale.
func DecodeStoredProfile(data []byte, catalog governanceprincipal.Catalog) (*StoredProfile, error) {
	fm, body, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("policyartifact: %w", err)
	}
	profile, err := governanceprincipal.DecodeProfile(fm, catalog)
	if err != nil {
		return nil, err
	}
	digest, err := profile.Digest()
	if err != nil {
		return nil, err
	}
	rationale, err := requireRationale("governance-profile", body)
	if err != nil {
		return nil, err
	}
	return &StoredProfile{
		ID:            profile.ID,
		ProfileDigest: digest,
		Profile:       profile,
		Rationale:     rationale,
	}, nil
}
