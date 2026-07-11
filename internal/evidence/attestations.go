package evidence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AttestationExists reports whether an attestation file exists for
// (storySlug, acID) under storeRoot's attestations/ directory
// (attestations/<storySlug>/<acID>.md). 03 §Evidence kinds is explicit
// that the attestation kind is "Satisfied by: attestation file exists for
// (story, AC)" and 02 §Kind registry says existence is the record (no
// status field at all) — so this checks existence only, deliberately not
// decoding or validating the file's frontmatter: a malformed attestation
// is still an attestation for the fold's purposes (VL-001 lint is where a
// malformed one gets caught, not the fold).
func AttestationExists(storeRoot, storySlug, acID string) (bool, error) {
	path := filepath.Join(storeRoot, ".verdi", "attestations", storySlug, acID+".md")
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("evidence: checking attestation %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("evidence: attestation path %s is a directory, not a file", path)
	}
	return true, nil
}
