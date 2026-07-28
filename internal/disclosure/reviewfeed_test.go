package disclosure

import (
	"errors"
	"strings"
	"testing"
)

func TestReviewUnavailableNoCredentials(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		wantText string
	}{
		{"named forge", "gitlab", `forge "gitlab" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`},
		// Negative path: an empty kind is still disclosed, never silenced —
		// the disclosure exists because the forge could not be consulted, and
		// a missing kind name does not make that fact go away.
		{"empty kind", "", `forge "" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ReviewUnavailableNoCredentials(tc.kind)
			if d.Source != SourceReviewFeed || d.Scope != "" || d.ID != SourceReviewFeed {
				t.Errorf("ReviewUnavailableNoCredentials(%q) = %+v, want source/ID %q with no scope", tc.kind, d, SourceReviewFeed)
			}
			if d.Text != tc.wantText {
				t.Errorf("Text = %q, want %q", d.Text, tc.wantText)
			}
			if d.Severity != SeverityDisclosedUnproven {
				t.Errorf("Severity = %q, want %q", d.Severity, SeverityDisclosedUnproven)
			}
		})
	}
}

func TestReviewUnavailableTransport(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantText string
	}{
		{"transport error", errors.New("dial tcp: connection refused"), "the configured forge could not be consulted (dial tcp: connection refused); review state cannot be shown"},
		// Negative path: a nil error must never render as a silent or
		// half-formed line — the caller only reaches this constructor because
		// a consult FAILED, so the disclosure is emitted regardless (fail
		// closed, constitution 2), naming the missing detail explicitly.
		{"nil error", nil, "the configured forge could not be consulted (unknown error); review state cannot be shown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := ReviewUnavailableTransport(tc.err)
			if d.Source != SourceReviewFeed || d.Scope != "" || d.ID != SourceReviewFeed {
				t.Errorf("ReviewUnavailableTransport(%v) = %+v, want source/ID %q with no scope", tc.err, d, SourceReviewFeed)
			}
			if d.Text != tc.wantText {
				t.Errorf("Text = %q, want %q", d.Text, tc.wantText)
			}
			if !IsRendered(Render(d)) {
				t.Errorf("Render(%+v) is not recognized as a disclosure line", d)
			}
		})
	}
}

// TestReviewUnavailable_OneFamilyOneVocabulary pins the property
// spec/disclosure-legibility#ac-1 actually asks for: both review-feed
// states are the SAME source and close the SAME way, so a reader who has
// learned to recognize one recognizes the other.
func TestReviewUnavailable_OneFamilyOneVocabulary(t *testing.T) {
	creds := Render(ReviewUnavailableNoCredentials("gitlab"))
	transport := Render(ReviewUnavailableTransport(errors.New("boom")))
	prefix := SeverityDisclosedUnproven + " [" + SourceReviewFeed + "]: "
	for _, line := range []string{creds, transport} {
		if !strings.HasPrefix(line, prefix) {
			t.Errorf("%q does not open with the shared review-feed vocabulary %q", line, prefix)
		}
		if !strings.HasSuffix(line, "; review state cannot be shown") {
			t.Errorf("%q does not close with the shared review-feed consequence clause", line)
		}
	}
}
