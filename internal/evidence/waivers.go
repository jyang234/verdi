package evidence

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// waiverActiveStatus is the only artifact.Status value that waives an AC
// (03 §The fold: "expired waivers do NOT waive"; 02 §Kind registry:
// waiver statuses are active -> expired).
const waiverActiveStatus artifact.Status = "active"

// WaiverActive reports whether an active waiver file exists for
// (storySlug, acID) under storeRoot's waivers/ directory
// (store.WaiverPath: waivers/<storySlug>/<acID>.md, 03 §Attestations and
// waivers). A waiver whose frontmatter status is "expired" is present but
// does not waive — 03 is explicit that expired waivers never waive, and
// this package reads that status directly off the committed frontmatter
// field rather than recomputing it from the `expiry` date against wall-
// clock time (CLAUDE.md: no wall-clock dependence in computed output);
// the fold never reinterprets it. spec/verb-surfaces ac-3's `verdi audit`
// waiver section is where wall-clock expiry gets consulted instead — an
// ephemeral, per-invocation disclosure, never baked back into this
// deterministic fold computation. A reaffirmed waiver's status is reset to
// "active" by `verdi waive --reaffirm` (spec/verb-surfaces ac-2), so this
// function needs no reaffirmation-awareness of its own: it always reads
// whatever the committed file's status field currently says.
//
// A missing waiver file is not an error — most (story, AC) pairs never
// have one.
func WaiverActive(storeRoot, storySlug, acID string) (bool, error) {
	path := store.WaiverPath(storeRoot, storySlug, acID)
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

// WaiverLapsed reports whether expiry (a YYYY-MM-DD date, or "" for none)
// has passed as of now, at day granularity: a waiver remains active
// THROUGH the end of its expiry day and lapses starting the day after
// (spec/verb-surfaces ac-2/ac-3's disclosed reading — the guide's own
// prose names no finer grain than a date). now is truncated to a UTC
// calendar day before comparing, so the caller's local time-of-day never
// changes the verdict. A malformed expiry is not this function's concern
// to fail on — WaiverFrontmatter.Validate() (VL-001, at lint/decode time)
// is where that is caught; here it degrades to "never lapsed" rather than
// erroring a caller whose real job is unrelated to that one file (`verdi
// audit`'s corpus-wide scan, `verdi waive --reaffirm`'s own disclosure).
//
// Deliberately never consulted by WaiverActive/the fold above (see its own
// doc comment): this is the ephemeral, per-invocation surfacing path
// (spec/verb-surfaces ac-3), not a change to the deterministic fold
// computation.
func WaiverLapsed(expiry string, now time.Time) bool {
	if expiry == "" {
		return false
	}
	d, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return false
	}
	nowDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return nowDay.After(d)
}
