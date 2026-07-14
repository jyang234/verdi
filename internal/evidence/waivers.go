package evidence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
)

// waiverActiveStatus is the only artifact.Status value that waives an AC
// (03 §The fold: "expired waivers do NOT waive"; 02 §Kind registry:
// waiver statuses are active -> expired).
const waiverActiveStatus artifact.Status = "active"

// WaiverActive reports whether an active waiver file exists for
// (storySlug, acID) under storeRoot's waivers/ directory
// (waivers/<storySlug>/<acID>.md, I-6's "<story>--<ac-id>" compound
// name). A waiver whose frontmatter status is "expired" is present but
// does not waive — 03 is explicit that expired waivers never waive, and
// this package reads that status directly off the committed frontmatter
// field rather than recomputing it from the `expiry` date against wall-
// clock time (CLAUDE.md: no wall-clock dependence in computed output);
// the expiry field is for humans and `verdi waivers` (out of v0 scope) to
// audit, not for the fold to reinterpret.
//
// A missing waiver file is not an error — most (story, AC) pairs never
// have one.
func WaiverActive(storeRoot, storySlug, acID string) (bool, error) {
	path := filepath.Join(storeRoot, ".verdi", "waivers", storySlug, acID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("evidence: reading waiver %s: %w", path, err)
	}

	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		return false, fmt.Errorf("evidence: waiver %s: %w", path, err)
	}
	w, err := artifact.DecodeWaiver(fm)
	if err != nil {
		return false, fmt.Errorf("evidence: waiver %s: %w", path, err)
	}
	return w.Status == waiverActiveStatus, nil
}
