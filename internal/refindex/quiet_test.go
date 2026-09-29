package refindex

import (
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/disclosure"
)

// fixedNow anchors every quiet boundary case below: a design-branch entry's
// Date thirteen/fourteen/fifteen days before it is expressed relative to
// this single instant, so the test's own arithmetic — not a wall-clock
// read — decides each boundary (dc-3).
var fixedNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

func daysBefore(now time.Time, days int) string {
	return now.Add(-time.Duration(days) * 24 * time.Hour).Format(dateLayout)
}

// TestQuiet_FourteenDays is ac-2's static obligation: the boundary sits at
// exactly fourteen days ("more than fourteen days before now"), quiet
// applies only within the drafts-in-progress group regardless of source,
// and an unreadable date is never quiet.
func TestQuiet_FourteenDays(t *testing.T) {
	unreadable := disclosure.New("refindex:date-unreadable", "spec/x", "test")

	tests := []struct {
		name string
		e    Entry
		want bool
	}{
		{
			name: "13 days before now: not yet quiet",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Date: daysBefore(fixedNow, 13)},
			want: false,
		},
		{
			name: "exactly 14 days before now: not yet quiet (boundary is exclusive)",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Date: daysBefore(fixedNow, 14)},
			want: false,
		},
		{
			name: "15 days before now: quiet",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Date: daysBefore(fixedNow, 15)},
			want: true,
		},
		{
			name: "default-branch source in the drafts group: still eligible for quiet",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Source: SourceDefault, Date: daysBefore(fixedNow, 20)},
			want: true,
		},
		{
			name: "design-branch source in the drafts group: eligible for quiet",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Source: SourceLocal, Date: daysBefore(fixedNow, 20)},
			want: true,
		},
		{
			name: "unreadable date: never quiet, however old the entry might be",
			e:    Entry{StatusGroup: StatusGroupDraftsInProgress, Date: "", DateDisclosed: &unreadable},
			want: false,
		},
		{
			name: "entry in another group (accepted-pending-build): never quiet, however old",
			e:    Entry{StatusGroup: StatusGroupAcceptedPendingBuild, Date: daysBefore(fixedNow, 365)},
			want: false,
		},
		{
			name: "entry in another group (active components): never quiet",
			e:    Entry{StatusGroup: StatusGroupActiveComponents, Date: daysBefore(fixedNow, 365)},
			want: false,
		},
		{
			name: "entry in another group (terminal): never quiet",
			e:    Entry{StatusGroup: StatusGroupTerminal, Date: daysBefore(fixedNow, 365)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsQuiet(tt.e, fixedNow); got != tt.want {
				t.Fatalf("IsQuiet(%+v, %v) = %v, want %v", tt.e, fixedNow, got, tt.want)
			}
		})
	}
}

// TestQuiet_UnparsableDate_NegativePath proves a Date that is present but
// not in the expected layout fails closed (never quiet) rather than
// panicking or silently treating it as arbitrarily old.
func TestQuiet_UnparsableDate_NegativePath(t *testing.T) {
	e := Entry{StatusGroup: StatusGroupDraftsInProgress, Date: "not-a-date"}
	if IsQuiet(e, fixedNow) {
		t.Fatal("IsQuiet with an unparsable Date = true, want false (fail closed, never quiet)")
	}
}
