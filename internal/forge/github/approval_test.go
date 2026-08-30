package github

import (
	"testing"

	"github.com/jyang234/verdi/internal/forge"
)

func TestNormalizeReviewState(t *testing.T) {
	tests := []struct {
		name      string
		provider  string
		want      forge.ApprovalState
		wantKeep  bool
		wantError bool
	}{
		{"approved", "APPROVED", forge.ApprovalActive, true, false},
		{"dismissed", "DISMISSED", forge.ApprovalDismissed, true, false},
		{"commented", "COMMENTED", "", false, false},
		{"changes requested", "CHANGES_REQUESTED", "", false, false},
		{"pending", "PENDING", "", false, false},
		{"unknown", "MYSTERY", "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, keep, err := normalizeReviewState(tt.provider)
			if (err != nil) != tt.wantError {
				t.Fatalf("normalizeReviewState error = %v, wantError %v", err, tt.wantError)
			}
			if got != tt.want || keep != tt.wantKeep {
				t.Fatalf("normalizeReviewState = (%q, %v), want (%q, %v)", got, keep, tt.want, tt.wantKeep)
			}
		})
	}
}
