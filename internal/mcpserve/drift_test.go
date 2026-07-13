package mcpserve

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const driftTestBody = `# Design notes

The charge API needs a retry note, retried through the outbox pattern.

## AC 2

The refund path validates the charge API response before crediting.
`

// TestComputeDrift_Fresh proves a selector whose quote is still found
// under its pinned heading reports fresh.
func TestComputeDrift_Fresh(t *testing.T) {
	sel := artifact.Selector{Heading: "design-notes", Quote: "retried through the outbox pattern"}
	got := ComputeDrift(sel, driftTestBody)
	if got != DriftFresh {
		t.Fatalf("ComputeDrift = %q, want fresh", got)
	}
}

// TestComputeDrift_Moved proves a selector whose quote survives verbatim
// but has moved to a DIFFERENT heading section reports moved.
func TestComputeDrift_Moved(t *testing.T) {
	sel := artifact.Selector{Heading: "design-notes", Quote: "validates the charge API response"}
	got := ComputeDrift(sel, driftTestBody)
	if got != DriftMoved {
		t.Fatalf("ComputeDrift = %q, want moved (quote now lives under a different heading)", got)
	}
}

// TestComputeDrift_Gone proves a selector whose quote is nowhere in the
// current document reports gone, and that an entirely missing artifact
// (empty currentBody) also reports gone rather than erroring.
func TestComputeDrift_Gone(t *testing.T) {
	t.Run("quote no longer present anywhere", func(t *testing.T) {
		sel := artifact.Selector{Heading: "design-notes", Quote: "this text was deleted entirely"}
		got := ComputeDrift(sel, driftTestBody)
		if got != DriftGone {
			t.Fatalf("ComputeDrift = %q, want gone", got)
		}
	})

	t.Run("artifact no longer resolves (empty body)", func(t *testing.T) {
		sel := artifact.Selector{Heading: "design-notes", Quote: "retried through the outbox pattern"}
		got := ComputeDrift(sel, "")
		if got != DriftGone {
			t.Fatalf("ComputeDrift(empty body) = %q, want gone", got)
		}
	})
}

// TestComputeDrift_HeadingMovedButTextIntact proves the fresh/moved split
// is keyed on the HEADING match, not just quote presence: the same quote
// under a heading with a DIFFERENT pinned name is moved, never fresh, even
// though nothing about the quote itself changed.
func TestComputeDrift_HeadingMovedButTextIntact(t *testing.T) {
	sel := artifact.Selector{Heading: "some-other-heading", Quote: "retried through the outbox pattern"}
	got := ComputeDrift(sel, driftTestBody)
	if got != DriftMoved {
		t.Fatalf("ComputeDrift = %q, want moved (pinned heading %q does not match, but the quote is present verbatim elsewhere)", got, sel.Heading)
	}
}

// TestSplitSections_Happy checks a body with preamble text before the
// first heading, and multiple heading levels, are split correctly.
func TestSplitSections_Happy(t *testing.T) {
	body := "preamble\n# One\nfirst\n## Two\nsecond\n# Three\nthird\n"
	sections := splitSections(body)
	if len(sections) != 4 {
		t.Fatalf("got %d sections, want 4 (preamble + One + Two + Three): %+v", len(sections), sections)
	}
	if sections[0].Anchor != "" {
		t.Fatalf("preamble section anchor = %q, want empty", sections[0].Anchor)
	}
	wantAnchors := []string{"", "one", "two", "three"}
	for i, want := range wantAnchors {
		if sections[i].Anchor != want {
			t.Fatalf("sections[%d].Anchor = %q, want %q", i, sections[i].Anchor, want)
		}
	}
}

// TestMDSlugify_MatchesLintConvention pins mdSlugify's behavior against
// the same example inputs internal/lint/headings.go's private slugify is
// exercised with, so the two independently-maintained copies cannot drift
// apart silently (this file's package-doc comment on splitSections names
// the duplication and points here).
func TestMDSlugify_MatchesLintConvention(t *testing.T) {
	cases := map[string]string{
		"Design Notes":        "design-notes",
		"AC-2":                "ac-2",
		"  Leading/Trailing ": "leadingtrailing",
		// No collapsing of consecutive separators: this is the algorithm's
		// actual (documented) behavior, matching internal/lint/headings.go's
		// slugify exactly — the parity property under test, not an
		// idealized slugifier.
		"Multiple   Spaces": "multiple---spaces",
	}
	for in, want := range cases {
		if got := mdSlugify(in); got != want {
			t.Errorf("mdSlugify(%q) = %q, want %q", in, got, want)
		}
	}
}
