package refindex

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/specstate"
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

// TestEffectiveStatusGroup_Happy is a pure-function unit test (no ctx, no
// git, no fake needed) of Task 6a's own class: feature/story mapping:
// AcceptedPendingBuild lands in the accepted bucket; Superseded and Closed
// both land in Terminal; neither carries a Disclosure.
func TestEffectiveStatusGroup_Happy(t *testing.T) {
	cases := []struct {
		state specstate.State
		want  StatusGroup
	}{
		{specstate.AcceptedPendingBuild, StatusGroupAcceptedPendingBuild},
		{specstate.Superseded, StatusGroupTerminal},
		{specstate.Closed, StatusGroupTerminal},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			got, disclosed := effectiveStatusGroup("spec/x", specstate.Result{State: tc.state})
			if got != tc.want {
				t.Fatalf("effectiveStatusGroup(%q) = %q, want %q", tc.state, got, tc.want)
			}
			if disclosed != nil {
				t.Fatalf("effectiveStatusGroup(%q) Disclosed = %+v, want nil", tc.state, disclosed)
			}
		})
	}
}

// TestEffectiveStatusGroup_Negative_UnprovenNeverAccepted proves an
// Unproven Result (and, defensively, a Proposed one — never actually
// reachable in production, since a default-branch candidate's content is
// always read FROM that same branch, but the switch must not fail open on
// it either) lands in DraftsInProgress — never the accepted group — and
// carries a Disclosure whose text embeds specstate's own witness message
// when one is present, or a fallback when Disclosures is empty.
func TestEffectiveStatusGroup_Negative_UnprovenNeverAccepted(t *testing.T) {
	t.Run("unproven with witnesses", func(t *testing.T) {
		got, disclosed := effectiveStatusGroup("spec/x", specstate.Result{
			State:       specstate.Unproven,
			Disclosures: []string{"first witness", "second witness"},
		})
		if got == StatusGroupAcceptedPendingBuild {
			t.Fatalf("effectiveStatusGroup(Unproven) = %q, want anything but the accepted group", got)
		}
		if got != StatusGroupDraftsInProgress {
			t.Errorf("effectiveStatusGroup(Unproven) = %q, want %q", got, StatusGroupDraftsInProgress)
		}
		if disclosed == nil {
			t.Fatal("Disclosed = nil, want a populated disclosure")
		}
		if disclosed.Scope != "spec/x" {
			t.Errorf("Disclosed.Scope = %q, want %q", disclosed.Scope, "spec/x")
		}
		if disclosed.Text != "first witness; second witness" {
			t.Errorf("Disclosed.Text = %q, want both witnesses joined", disclosed.Text)
		}
	})

	t.Run("unproven with no witnesses still discloses", func(t *testing.T) {
		_, disclosed := effectiveStatusGroup("spec/x", specstate.Result{State: specstate.Unproven})
		if disclosed == nil {
			t.Fatal("Disclosed = nil, want a populated fallback disclosure even with no specstate witnesses")
		}
	})

	t.Run("proposed defensively never accepted either", func(t *testing.T) {
		got, disclosed := effectiveStatusGroup("spec/x", specstate.Result{State: specstate.Proposed})
		if got == StatusGroupAcceptedPendingBuild {
			t.Fatalf("effectiveStatusGroup(Proposed) = %q, want anything but the accepted group", got)
		}
		if disclosed == nil {
			t.Fatal("Disclosed = nil, want a populated disclosure")
		}
	})
}
