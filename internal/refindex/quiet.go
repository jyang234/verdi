package refindex

import "time"

// QuietAfterDays is dc-2's one named constant: the threshold, in days,
// after which a drafts-in-progress entry reads quiet. Shared by IsQuiet
// below and the card's own quiet mark a later story renders — a single
// vocabulary value, never independently re-declared at the point of use.
const QuietAfterDays = 14

// dateLayout mirrors gitx.CommitDate's own canonical format exactly
// (internal/gitx/log.go), since Entry.Date is always produced by that
// function (through this package's GitRunner.CommitDate port method) —
// never a second, independently-chosen layout.
const dateLayout = "2006-01-02T15:04:05-07:00"

// IsQuiet reports whether e's last change is more than QuietAfterDays days
// before now (ac-2). now is always caller-injected, never read from a
// package variable or the wall clock directly (dc-3): this function is
// itself trivially deterministic, and callers needing a fake clock supply
// one directly as a time.Time value.
//
// Only an entry in StatusGroupDraftsInProgress can ever read quiet (dc-2:
// "no entry outside that group reads quiet"), regardless of its source. An
// entry whose date could not be read (DateDisclosed non-nil, or an empty
// Date) never reads quiet either — it stays disclosed instead of silently
// resolving to either extreme (ac-1's "never invent a fallback date"
// carried through to this decision too).
func IsQuiet(e Entry, now time.Time) bool {
	if e.StatusGroup != StatusGroupDraftsInProgress {
		return false
	}
	if e.DateDisclosed != nil || e.Date == "" {
		return false
	}
	t, err := time.Parse(dateLayout, e.Date)
	if err != nil {
		return false
	}
	return now.Sub(t) > time.Duration(QuietAfterDays)*24*time.Hour
}
