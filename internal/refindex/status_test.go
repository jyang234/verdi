package refindex

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func TestMapStatusGroup_Happy(t *testing.T) {
	cases := []struct {
		status artifact.Status
		want   StatusGroup
	}{
		{"draft", StatusGroupDraftsInProgress},
		{"accepted-pending-build", StatusGroupAcceptedPendingBuild},
		{"active", StatusGroupActiveComponents},
		{"closed", StatusGroupTerminal},
		{"superseded", StatusGroupTerminal},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			got, err := mapStatusGroup(tc.status)
			if err != nil {
				t.Fatalf("mapStatusGroup(%q): unexpected error: %v", tc.status, err)
			}
			if got != tc.want {
				t.Fatalf("mapStatusGroup(%q) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

// TestMapStatusGroup_Negative_FailsClosed proves an unrecognized status
// value errors rather than silently landing in a default bucket
// (CLAUDE.md: "unknown enum values fail closed"; ac-3's static obligation).
func TestMapStatusGroup_Negative_FailsClosed(t *testing.T) {
	cases := []artifact.Status{"", "bogus", "proposed", "expired"}
	for _, status := range cases {
		t.Run(string(status), func(t *testing.T) {
			if _, err := mapStatusGroup(status); err == nil {
				t.Fatalf("mapStatusGroup(%q): want error, got nil", status)
			}
		})
	}
}
