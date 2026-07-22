// The waiver half of `verdi audit`'s counterweight machinery
// (spec/verb-surfaces ac-3, guide 8.4: "verdi audit counts active waivers
// with the same budget machinery as deviations"). Kept in its own file
// beside audit.go's spec-stale scan, mirroring backlinks.go/autofile.go's
// own split for the exemption half — one file, one topic.
package decisionsweep

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// DefaultWaiversStaleThreshold mirrors evidence.DefaultDeviationsStaleThreshold's
// own documented default (3) — the smallest reversible starting point,
// applied whenever verdi.yaml's audit.waivers_stale_threshold is absent or
// non-positive, exactly as the deviations threshold's own consumer already
// does (internal/evidence.SpecStale).
const DefaultWaiversStaleThreshold = 3

// waiverActiveStatus mirrors internal/evidence's own unexported constant of
// the same name (waivers.go) — kept local rather than exported cross-
// package, since both packages independently decode artifact.WaiverFrontmatter
// and both need only the one literal 02 §Kind registry fixes.
const waiverActiveStatus artifact.Status = "active"

// WaiverAuditRow is one waiver file's audit-time disclosure: which AC it
// covers, where it lives, its committed status and expiry, whether that
// expiry has lapsed by wall-clock as of the scan's `now`, and whether it
// counts toward the story's active total (status active AND not lapsed —
// guide 8.4: "past expiry the waiver lapses... reverts to pending").
type WaiverAuditRow struct {
	ACID         string
	Path         string // store-relative display form (store.WaiverPath("", ...))
	Status       artifact.Status
	Expiry       string // "" when none was given
	Lapsed       bool
	CountsActive bool
}

// WaiverStaleEntry is one story spec's waiver-audit computation: every
// waiver row found (for full disclosure — a lapsed or expired-status
// waiver is named, never silently dropped from the listing) plus the
// active count, the threshold it was measured against, and whether that
// count crossed it.
type WaiverStaleEntry struct {
	StoryRef    string
	Waivers     []WaiverAuditRow
	ActiveCount int
	Threshold   int
	Flagged     bool
}

// ScanWaiverStale computes the waiver-audit entry for every story-class
// spec snap found that has at least one waiver file on disk under
// .verdi/waivers/<story-slug>/ — a story with none at all is skipped
// entirely (never flagged, never listed), mirroring ScanSpecStale's own
// "no report yet, skip" posture. threshold <= 0 uses
// DefaultWaiversStaleThreshold. now is the caller's single wall-clock read
// (cmd/verdi/audit.go's own boundary, mirroring attest.go's stamp
// convention) — never read again inside this scan, so the whole computation
// is deterministic given (root, now).
func ScanWaiverStale(root string, snap *lint.Snapshot, threshold int, now time.Time) ([]WaiverStaleEntry, error) {
	if threshold <= 0 {
		threshold = DefaultWaiversStaleThreshold
	}

	var out []WaiverStaleEntry
	for _, doc := range snap.Docs {
		if doc.DecodeErr != nil || doc.Spec == nil || doc.Spec.Class != artifact.ClassStory {
			continue
		}
		if doc.Spec.Story == "" {
			// VL-005 requires a story spec to carry exactly one story: link
			// on the default branch; a design-branch draft that has not
			// reached that gate yet has no slug to look waivers up under.
			continue
		}
		storySlug := store.RefSlug(doc.Spec.Story)

		rows, err := scanWaiverRows(root, storySlug, now)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			continue
		}

		active := 0
		for _, r := range rows {
			if r.CountsActive {
				active++
			}
		}
		out = append(out, WaiverStaleEntry{
			StoryRef:    doc.Spec.ID,
			Waivers:     rows,
			ActiveCount: active,
			Threshold:   threshold,
			Flagged:     active > threshold,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StoryRef < out[j].StoryRef })
	return out, nil
}

// scanWaiverRows reads and decodes every waiver file under
// storeRoot's waivers/<storySlug>/ directory (store.WaiverDir), returning
// one WaiverAuditRow per file, sorted by AC id. A missing directory is not
// an error — most stories have no waivers at all.
func scanWaiverRows(root, storySlug string, now time.Time) ([]WaiverAuditRow, error) {
	dir := store.WaiverDir(root, storySlug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("decisionsweep: reading %s: %w", dir, err)
	}

	var rows []WaiverAuditRow
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		acID := strings.TrimSuffix(e.Name(), ".md")
		path := filepath.Join(dir, e.Name())

		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, fmt.Errorf("decisionsweep: reading %s: %w", path, rerr)
		}
		fm, _, serr := artifact.SplitFrontmatter(data)
		if serr != nil {
			return nil, fmt.Errorf("decisionsweep: %s: %w", path, serr)
		}
		w, derr := artifact.DecodeWaiver(fm)
		if derr != nil {
			return nil, fmt.Errorf("decisionsweep: %s: %w", path, derr)
		}

		lapsed := waiverLapsed(w.Expiry, now)
		rows = append(rows, WaiverAuditRow{
			ACID:         acID,
			Path:         store.WaiverPath("", storySlug, acID),
			Status:       w.Status,
			Expiry:       w.Expiry,
			Lapsed:       lapsed,
			CountsActive: w.Status == waiverActiveStatus && !lapsed,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ACID < rows[j].ACID })
	return rows, nil
}

// waiverLapsed reports whether expiry (a YYYY-MM-DD date, or "" for none)
// has passed as of now, at day granularity: a waiver remains active
// THROUGH the end of its expiry day and lapses starting the day after
// (spec/verb-surfaces ac-3's disclosed reading — the guide's own prose
// names no finer grain than a date). now is truncated to a UTC calendar
// day before comparing, so the caller's local time-of-day never changes
// the verdict. A malformed expiry is not this function's concern to fail
// on — WaiverFrontmatter.Validate() (VL-001, at lint/decode time) is where
// that is caught; here it degrades to "never lapsed" rather than erroring
// a scan over unrelated stories.
func waiverLapsed(expiry string, now time.Time) bool {
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
