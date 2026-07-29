package disclosure

import (
	"strings"
	"testing"
)

// TestAdvisoryPreview pins the disclosure VALUE every advisory-fold surface
// constructs — its source, its (absent) scope, its derived id and its fixed
// severity — mirroring this package's review-feed constructor tests. The
// banner is about the FOLD, not about one artifact, so the scope is empty by
// construction and the id is the bare source. (Hoisted with the constructor
// from cmd/verdi's TestAdvisoryPreviewDisclosure when the workbench matrix
// page became the family's second producing package.)
func TestAdvisoryPreview(t *testing.T) {
	d := AdvisoryPreview()
	if d.Source != SourceAdvisoryPreview {
		t.Errorf("Source = %q, want %q", d.Source, SourceAdvisoryPreview)
	}
	if d.Scope != "" {
		t.Errorf("Scope = %q, want empty (the banner is about the whole fold, not one artifact)", d.Scope)
	}
	if d.ID != SourceAdvisoryPreview {
		t.Errorf("ID = %q, want the bare source %q", d.ID, SourceAdvisoryPreview)
	}
	if d.Severity != SeverityDisclosedUnproven {
		t.Errorf("Severity = %q, want %q", d.Severity, SeverityDisclosedUnproven)
	}
	if d.Text != advisoryPreviewText {
		t.Errorf("Text = %q, want %q", d.Text, advisoryPreviewText)
	}
	// Negative path — the defects this replaced were banners that spoke their
	// own severity words ("PREVIEW:" on the CLI, "PREVIEW — ADVISORY" in the
	// workbench) instead of the shared one. The text must name the observed
	// fact and its consequence and never restate the severity: Render supplies
	// that one word (spec/disclosure-legibility#ac-1).
	if strings.Contains(d.Text, SeverityDisclosedUnproven) {
		t.Errorf("Text = %q states the severity itself; Render already supplies it", d.Text)
	}
	if strings.Contains(d.Text, "PREVIEW") || strings.Contains(d.Text, "ADVISORY") {
		t.Errorf("Text = %q still carries a hand-authored severity marker", d.Text)
	}
	if !IsRendered(Render(d)) {
		t.Errorf("Render(%+v) is not recognized as a disclosure line", d)
	}
}
