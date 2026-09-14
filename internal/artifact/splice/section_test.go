package splice

import (
	"strings"
	"testing"
)

// TestSetSectionText_ReplacesProseOnly proves SetSectionText replaces only
// the prose between a heading and the next heading, leaving the heading
// line itself and every other section byte-for-byte untouched — the
// creation-only body mirror internal/specimport's Compose needs to make a
// mapped Problem/Outcome statement's text appear in both frontmatter and
// its own body section (spec-import-contract.md).
func TestSetSectionText_ReplacesProseOnly(t *testing.T) {
	doc, err := Parse([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edit, err := doc.SetSectionText("problem", "Borrowers cannot resubmit a rejected document today.")
	if err != nil {
		t.Fatalf("SetSectionText: %v", err)
	}
	out, err := doc.Apply([]Edit{edit})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "## Problem\n\nBorrowers cannot resubmit a rejected document today.\n\n## Outcome") {
		t.Fatalf("body does not carry the replaced Problem section in canonical shape:\n%s", got)
	}
	if strings.Contains(got, "Prose.\n\n## Outcome") {
		t.Fatalf("old Problem prose (\"Prose.\") survived:\n%s", got)
	}
	// Every other section's own prose is untouched.
	if !strings.Contains(got, "## Outcome\n\nProse.\n\n## ac-1") {
		t.Fatalf("Outcome section was disturbed by a Problem-only edit:\n%s", got)
	}
}

// TestSetSectionText_UnknownAnchor_FailsClosed proves an anchor with no
// resolving heading is a hard error, never a silent no-op — a custom
// template whose body does not carry the section Compose expects to mirror
// into must surface as an explicit refusal, not vanish.
func TestSetSectionText_UnknownAnchor_FailsClosed(t *testing.T) {
	doc, err := Parse([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := doc.SetSectionText("no-such-section", "text"); err == nil {
		t.Fatal("SetSectionText on an unresolvable anchor: want error, got nil")
	}
}

// TestRemoveSection_DeletesHeadingAndProse proves RemoveSection removes the
// WHOLE section — heading line and its prose — the exact inverse of
// AppendObject's own body-section insertion, needed to delete a scaffold's
// now-orphaned placeholder section (e.g. the removed placeholder AC's own
// "## Ac 1").
func TestRemoveSection_DeletesHeadingAndProse(t *testing.T) {
	doc, err := Parse([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edit, err := doc.RemoveSection("ac-2")
	if err != nil {
		t.Fatalf("RemoveSection: %v", err)
	}
	out, err := doc.Apply([]Edit{edit})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "## ac-2") {
		t.Fatalf("removed section's own heading survived:\n%s", got)
	}
	// Neighboring sections survive untouched, directly adjacent.
	if !strings.Contains(got, "## ac-1\n\nProse.\n\n## ac-3") {
		t.Fatalf("neighboring sections were disturbed:\n%s", got)
	}
}

// TestRemoveSection_LastSection proves removing the FINAL section in the
// document (no following heading) removes cleanly through end of file.
func TestRemoveSection_LastSection(t *testing.T) {
	doc, err := Parse([]byte(sampleSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	edit, err := doc.RemoveSection("dc-3")
	if err != nil {
		t.Fatalf("RemoveSection: %v", err)
	}
	out, err := doc.Apply([]Edit{edit})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "## dc-3") {
		t.Fatalf("removed final section's heading survived:\n%s", got)
	}
	// The blank line separating dc-2's own prose from dc-3's now-removed
	// heading belongs to dc-2's trailing whitespace, not to dc-3's span, so
	// it survives — sampleSpec's own inter-section spacing convention.
	if !strings.HasSuffix(got, "## dc-2\n\nProse.\n\n") {
		t.Fatalf("document does not end cleanly right after the preceding dc-2 section:\n%q", got)
	}
}
