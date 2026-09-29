package refindex

import "time"

// QuietAfterDays is dc-2's one named constant: the threshold, in days,
// after which a drafts-in-progress entry reads quiet. Shared by IsQuiet
// below and the card's own quiet mark a later story renders — a single
// vocabulary value, never independently re-declared at the point of use.
const QuietAfterDays = 14

// dateLayout mirrors gitx.CommitDates' (and gitx.CommitDate's) canonical
// format exactly (internal/gitx), since Entry.Date is always produced by
// this package's GitRunner.CommitDates port method — never a second,
// independently-chosen layout.
const dateLayout = "2006-01-02T15:04:05-07:00"

// parseLastChange parses a committer date in dateLayout — the one parse
// behind LastChange here and ComputeIndex's own readable-date check
// (refindex.go's dateFor), so what ComputeIndex accepts as a date is
// exactly what every consumer can read back.
func parseLastChange(date string) (time.Time, error) {
	return time.Parse(dateLayout, date)
}

// LastChange returns e's last-change instant when it is readable: a Date
// in the canonical committer-date layout and no DateDisclosed. ok is false
// otherwise — the date is disclosed or absent, and the zero instant
// returned then is never a stand-in for it (ac-1). It is the one readable-
// date rule IsQuiet and a renderer's date carrier share.
func LastChange(e Entry) (time.Time, bool) {
	if e.DateDisclosed != nil || e.Date == "" {
		return time.Time{}, false
	}
	t, err := parseLastChange(e.Date)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// IsQuiet reports whether e's last change is more than QuietAfterDays days
// before now (ac-2). now is always caller-injected, never read from a
// package variable or the wall clock directly (dc-3): this function is
// itself trivially deterministic, and callers needing a fake clock supply
// one directly as a time.Time value.
//
// Only an entry in StatusGroupDraftsInProgress can ever read quiet (dc-2:
// "no entry outside that group reads quiet"), regardless of its source. An
// entry whose date is not readable (LastChange's ok false: disclosed,
// empty, or not a committer date) never reads quiet either — it stays
// disclosed instead of silently resolving to either extreme (ac-1's "never
// invent a fallback date" carried through to this decision too).
func IsQuiet(e Entry, now time.Time) bool {
	if e.StatusGroup != StatusGroupDraftsInProgress {
		return false
	}
	last, ok := LastChange(e)
	if !ok {
		return false
	}
	return now.Sub(last) > time.Duration(QuietAfterDays)*24*time.Hour
}
