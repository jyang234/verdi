package workbench

// Tests for the placard body seam (R4 board polish): resolving a
// problem/outcome attribute's anchor to its body section, rendering it
// through the same path the corpus page uses (attributebody.go), and
// emitting it as a hidden, expand-ready element inside its placard
// (boardspecrender.go) — the seam a follow-on Fable pass wires to a
// click-to-read-full-prose dialog.

import (
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// placardBodyFixtureSpec carries BOTH a "## Problem" and a "## Outcome"
// body section under their declared anchors — the happy path — with a
// distinctive phrase in each, and real markdown (a list and emphasis) in
// the outcome section so "rendered as HTML, not raw text" has something
// to prove.
const placardBodyFixtureSpec = `---
id: spec/placard-body-fixture
kind: spec
class: feature
title: "Placard body fixture"
status: draft
owners: [platform-team]
problem: { text: "short problem headline", anchor: "#problem" }
outcome: { text: "short outcome headline", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "x", evidence: [attestation], anchor: "#ac-1" }
---
# Placard body fixture

## Problem

Applicants distrust the paperclip-and-yarn decline explainer, a distinctively worded confession of the real defect.

## Outcome

A rewritten flow where:

- decisions read in *plain English*
- appeals resolve in one business day

## ac-1

Prose.
`

// placardBodyFixtureOutcomeSection is the exact substring removed to
// produce the "missing ## Outcome section" fixture below — pulled out so
// the removal and the "is it gone" assertion can't silently drift apart.
const placardBodyFixtureOutcomeSection = "## Outcome\n\nA rewritten flow where:\n\n- decisions read in *plain English*\n- appeals resolve in one business day\n\n"

// mustSplitAndDecode is this file's own tiny fixture helper: unlike
// mustDecodeSpecForTest (projection_test.go), it keeps the body instead of
// discarding it. Every OTHER buildProjection call site in this package
// passes nil for body precisely because none of them need it.
func mustSplitAndDecode(t *testing.T, y string) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	fmBytes, body, err := artifact.SplitFrontmatter([]byte(y))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	fm, err := artifact.DecodeSpec(fmBytes)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	return fm, body
}

// TestBodySection is a table-driven happy/negative-path unit test of the
// low-level heading-section extractor: hash-prefixed and bare-slug
// anchors both resolve, a resolved section runs to the next heading of
// ANY level (or the document's end for the last section), and an empty
// or unresolvable anchor reports ok=false rather than erroring.
func TestBodySection(t *testing.T) {
	const body = "# Title\n\n## Problem\n\nFirst para.\n\nSecond para.\n\n## Outcome\n\nOutcome para.\n"
	cases := []struct {
		name   string
		anchor string
		want   string
		ok     bool
	}{
		{"resolves with hash prefix", "#problem", "First para.\n\nSecond para.", true},
		{"resolves bare slug", "problem", "First para.\n\nSecond para.", true},
		{"last section runs to end of document", "#outcome", "Outcome para.", true},
		{"empty anchor", "", "", false},
		{"unresolvable anchor", "#nope", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := bodySection([]byte(body), tc.anchor)
			if ok != tc.ok || got != tc.want {
				t.Errorf("bodySection(%q) = (%q, %v), want (%q, %v)", tc.anchor, got, ok, tc.want, tc.ok)
			}
		})
	}

	// A heading resolved by bodySection uses the exact same slug algorithm
	// (artifact.SlugifyHeading) 02 §Object model's anchor-resolution rule
	// uses elsewhere (artifact.HeadingAnchors/ResolveAnchor) — proven here
	// by cross-checking against HeadingAnchors directly, so this file's
	// own heading-line recognition can never silently diverge from the
	// canonical one.
	anchors := artifact.HeadingAnchors([]byte(body))
	for slug := range anchors {
		if _, ok := bodySection([]byte(body), slug); !ok {
			t.Errorf("bodySection did not resolve slug %q, but artifact.HeadingAnchors recognizes it as a heading", slug)
		}
	}
}

// TestAttributeBodyHTML is a table-driven happy/negative-path unit test
// of the attribute-to-rendered-HTML resolver: nil attribute, empty
// anchor, unresolvable anchor, nil body, and a blank (heading present but
// no prose under it) section all fail soft to "" — never an error, never
// a panic — while a resolving anchor over a real section renders through
// the markdown path (a list becomes actual HTML, not copied verbatim).
func TestAttributeBodyHTML(t *testing.T) {
	const body = "## Problem\n\n- one\n- two\n\n## Blank\n\n## Trailing\n\ntail prose\n"
	cases := []struct {
		name string
		body []byte
		attr *artifact.Attribute
		want string // substring the result must contain; "" means result must be exactly ""
	}{
		{"nil attribute", []byte(body), nil, ""},
		{"empty anchor", []byte(body), &artifact.Attribute{Text: "t", Anchor: ""}, ""},
		{"unresolved anchor", []byte(body), &artifact.Attribute{Text: "t", Anchor: "#nope"}, ""},
		{"nil body", nil, &artifact.Attribute{Text: "t", Anchor: "#problem"}, ""},
		{"blank section", []byte(body), &artifact.Attribute{Text: "t", Anchor: "#blank"}, ""},
		{"resolves and renders markdown", []byte(body), &artifact.Attribute{Text: "t", Anchor: "#problem"}, "<li>one</li>"},
		{"trailing section to end of doc", []byte(body), &artifact.Attribute{Text: "t", Anchor: "#trailing"}, "tail prose"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := attributeBodyHTML(tc.body, tc.attr)
			if tc.want == "" {
				if got != "" {
					t.Errorf("attributeBodyHTML = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(string(got), tc.want) {
				t.Errorf("attributeBodyHTML = %q, want containing %q", got, tc.want)
			}
		})
	}
}

// TestBuildProjection_AttributeBodyHTML proves item 1 of the seam end to
// end through buildProjection: both attributes' anchors resolve to their
// body section's RENDERED html (goldmark output — a markdown list and
// emphasis become real markup, not copied raw text), carried on the
// projection's new fields.
func TestBuildProjection_AttributeBodyHTML(t *testing.T) {
	fm, body := mustSplitAndDecode(t, placardBodyFixtureSpec)
	p, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}

	if !strings.Contains(string(p.ProblemBodyHTML), "paperclip-and-yarn decline explainer") {
		t.Errorf("ProblemBodyHTML missing its distinctive phrase: %s", p.ProblemBodyHTML)
	}
	if !strings.Contains(string(p.ProblemBodyHTML), "<p>") {
		t.Errorf("ProblemBodyHTML not rendered as HTML: %s", p.ProblemBodyHTML)
	}
	if !strings.Contains(string(p.OutcomeBodyHTML), "appeals resolve in one business day") {
		t.Errorf("OutcomeBodyHTML missing its distinctive phrase: %s", p.OutcomeBodyHTML)
	}
	// Markdown, not raw text: the list becomes <li>, the emphasis <em>.
	if !strings.Contains(string(p.OutcomeBodyHTML), "<li>") {
		t.Errorf("OutcomeBodyHTML list not rendered as HTML: %s", p.OutcomeBodyHTML)
	}
	if !strings.Contains(string(p.OutcomeBodyHTML), "<em>plain English</em>") {
		t.Errorf("OutcomeBodyHTML emphasis not rendered as HTML: %s", p.OutcomeBodyHTML)
	}
	if strings.Contains(string(p.OutcomeBodyHTML), "*plain English*") {
		t.Error("OutcomeBodyHTML still carries raw markdown emphasis syntax")
	}

	// Determinism: rebuilding the projection from the same inputs
	// reproduces byte-identical HTML.
	again, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection (again): %v", err)
	}
	if again.ProblemBodyHTML != p.ProblemBodyHTML || again.OutcomeBodyHTML != p.OutcomeBodyHTML {
		t.Error("buildProjection's attribute body HTML is not deterministic across identical inputs")
	}
}

// TestBuildProjection_AttributeBodyHTML_MissingSectionIsEmpty proves the
// negative path: a spec missing its "## Outcome" body section (the
// outcome attribute's anchor declared in frontmatter no longer resolves
// to anything) leaves OutcomeBodyHTML empty while ProblemBodyHTML still
// resolves normally — fail-soft, never an error, never a panic. Notably,
// artifact.DecodeSpec does not itself enforce anchor-body resolution
// (that is internal/lint's VL-014 concern, a separate opt-in check), so
// this fixture decodes successfully — precisely the "anchor doesn't
// resolve, but the board should never fall over" case this seam exists
// for.
func TestBuildProjection_AttributeBodyHTML_MissingSectionIsEmpty(t *testing.T) {
	noOutcomeSpec := strings.Replace(placardBodyFixtureSpec, placardBodyFixtureOutcomeSection, "", 1)
	if !strings.Contains(noOutcomeSpec, "## Problem") || strings.Contains(noOutcomeSpec, "## Outcome") {
		t.Fatal("test fixture setup broken: expected the Outcome section removed, Problem section intact")
	}
	fm, body := mustSplitAndDecode(t, noOutcomeSpec)
	p, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	if p.ProblemBodyHTML == "" {
		t.Error("ProblemBodyHTML wrongly empty when its own section is present")
	}
	if p.OutcomeBodyHTML != "" {
		t.Errorf("OutcomeBodyHTML should be empty with no matching body section, got %q", p.OutcomeBodyHTML)
	}
}

// TestRenderBoardRegion_PlacardFullHiddenElements proves item 2 of the
// seam: each placard whose attribute resolves a body section carries a
// HIDDEN, expand-ready element inside it, with the stable contract a
// follow-on Fable pass reads verbatim — exact class, exact data-testid,
// and the `hidden` attribute — alongside (never instead of) the placard's
// own visible headline text.
func TestRenderBoardRegion_PlacardFullHiddenElements(t *testing.T) {
	fm, body := mustSplitAndDecode(t, placardBodyFixtureSpec)
	p, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	rendered := renderBoardRegion(p, &boardGitState{})

	for _, want := range []string{
		`data-testid="placard-full-problem"`,
		`data-testid="placard-full-outcome"`,
		`class="placard-full"`,
		"paperclip-and-yarn decline explainer",
		"appeals resolve in one business day",
		"<li>", "<em>plain English</em>",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered board missing %q\nfull output:\n%s", want, rendered)
		}
	}
	if !strings.Contains(rendered, `<div class="placard-full" data-testid="placard-full-problem" hidden>`) {
		t.Error(`problem placard-full missing the exact "hidden" element open tag`)
	}
	if !strings.Contains(rendered, `<div class="placard-full" data-testid="placard-full-outcome" hidden>`) {
		t.Error(`outcome placard-full missing the exact "hidden" element open tag`)
	}
	// The placard's own concise headline (the attribute TEXT) still
	// renders — the hidden element is additive, never a replacement.
	if !strings.Contains(rendered, `data-testid="placard-problem"><span class="placard-tag">problem</span><p class="placard-text">short problem headline</p>`) {
		t.Error("problem placard's own headline text got displaced by the full-body element")
	}

	// Determinism: rebuilding the projection and re-rendering from the
	// same four inputs reproduces byte-identical output.
	again, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection (again): %v", err)
	}
	if got := renderBoardRegion(again, &boardGitState{}); got != rendered {
		t.Error("renderBoardRegion is not deterministic across identical inputs")
	}
}

// TestRenderBoardRegion_PlacardFullOmittedWhenSectionMissing proves the
// negative rendering path: a spec missing its "## Outcome" body section
// renders the problem placard's hidden element but omits the outcome
// one entirely (never an empty placeholder) — the Fable pass's
// documented fallback is the attribute's own headline text, unaffected.
func TestRenderBoardRegion_PlacardFullOmittedWhenSectionMissing(t *testing.T) {
	noOutcomeSpec := strings.Replace(placardBodyFixtureSpec, placardBodyFixtureOutcomeSection, "", 1)
	fm, body := mustSplitAndDecode(t, noOutcomeSpec)
	p, err := buildProjection("placard-body-fixture", fm, body, nil, nil, nil, modeReadOnly)
	if err != nil {
		t.Fatalf("buildProjection: %v", err)
	}
	rendered := renderBoardRegion(p, &boardGitState{})
	if !strings.Contains(rendered, `data-testid="placard-full-problem"`) {
		t.Error("problem placard-full missing even though its section is present")
	}
	if strings.Contains(rendered, `data-testid="placard-full-outcome"`) {
		t.Error("outcome placard-full rendered despite no matching body section")
	}
	// The outcome placard itself still renders from its attribute
	// headline — only the hidden extra is gone.
	if !strings.Contains(rendered, `data-testid="placard-outcome"`) {
		t.Error("outcome placard itself should still render from its attribute headline")
	}
}

// TestRenderBoardRegion_NoPlacardFullWithoutBodyHTML proves a projection
// built with no attribute body HTML at all (the common case for every
// OTHER test in this package, and for a bare in-memory BoardProjection
// literal) renders neither hidden element — the additive seam changes
// nothing about a projection that never populated it.
func TestRenderBoardRegion_NoPlacardFullWithoutBodyHTML(t *testing.T) {
	proj := &BoardProjection{Spec: "s", Mode: modeReadOnly, Problem: "p", Outcome: "o"}
	rendered := renderBoardRegion(proj, &boardGitState{})
	for _, absent := range []string{"placard-full", `data-testid="placard-full-problem"`, `data-testid="placard-full-outcome"`} {
		if strings.Contains(rendered, absent) {
			t.Errorf("rendered board carries %q with no attribute body HTML set", absent)
		}
	}
}
