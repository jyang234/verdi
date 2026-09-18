package specdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// renamedModel constructs a real *model.Model — model.Canonical() (a
// fresh decode every call, never a shared pointer, per its own doc
// comment, so mutating the result here cannot corrupt any other test or
// caller) with story and spike renamed via the same Vocabulary.Classes
// field a real model.yaml's `vocabulary: { classes: ... }` block decodes
// into (internal/model/model.go's Vocabulary type). Real, not mocked:
// Words.Story/Words.StoryPlural/Words.Spike/Words.SpikePlural all flow
// through the genuine DisplayClass/DisplayClassPlural chain.
func renamedModel(t *testing.T) *model.Model {
	t.Helper()
	m := model.Canonical()
	m.Vocabulary.Classes = map[string]string{"story": "planned story", "spike": "research spike"}
	return m
}

// loadRenamedStoryFixture mirrors loadFixture (markdown_test.go) for the
// second, renamed-vocabulary fixture: a class: feature spec (fix round 1
// disclosed substitution for the brief's "class: story" suggestion — see
// the fix-round report's "class: story vs class: feature" note) with no
// problem/outcome, 11 acceptance criteria, one constraint and one open
// question each carrying body prose, no stubs, and no supersedes.
func loadRenamedStoryFixture(t *testing.T) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "renamed-story.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	fm, err := artifact.DecodeSpec(raw)
	if err != nil {
		t.Fatal(err)
	}
	return fm, body
}

// TestRenderMarkdownGoldens_RenamedStory is fix round 1's proof for F1
// (irregular plurals through a renamed word), F2 (the criteria
// empty-state line — exercised indirectly here since this fixture's
// Plan is empty, not its Criteria; see TestRenderMarkdownEmptyCriteria
// for F2's direct proof), and F3 (continuation indent past item 9) all
// at once: 11 acceptance criteria (item 10 crosses into double digits),
// a renamed story/spike vocabulary, and an empty Plan (no stubs) so the
// renamed StoryPlural/SpikePlural empty-state line renders.
func TestRenderMarkdownGoldens_RenamedStory(t *testing.T) {
	fm, body := loadRenamedStoryFixture(t)
	mdl := renamedModel(t)
	commit := strings.Repeat("7", 40)
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/renamed-vocabulary", Commit: commit}, Facts: FactsFromSpec(fm), Model: mdl, Kind: k})
		if err != nil {
			t.Fatal(err)
		}
		checkGolden(t, "golden-renamed-story-"+string(k)+".md", RenderMarkdown(doc))
	}
}

// TestRenderHTMLGolden_RenamedStory is fix round 1's F3 HTML-level proof:
// before the fix, item 10's under-indented continuation split the
// acceptance-criteria <ol> in two, and the second half restarted
// numbering as <ol start="11">. See the fix-round report for the RED
// transcript captured against the pre-fix renderer.
func TestRenderHTMLGolden_RenamedStory(t *testing.T) {
	fm, body := loadRenamedStoryFixture(t)
	mdl := renamedModel(t)
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/renamed-vocabulary", Commit: strings.Repeat("7", 40)}, Facts: FactsFromSpec(fm), Model: mdl, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-renamed-story-spec.html", html)
	if strings.Contains(html, "<ol start=") {
		t.Errorf("item 10's continuation must not split the acceptance-criteria list into two <ol> elements (fix round 1, F3)")
	}
}

// TestRenamedModelPluralsAreIrregular directly asserts the fix round's
// closing golden-review question ("'planned stories'? check what
// DisplayClassPlural yields for the renamed word and assert it"): the
// renamed word is a two-word phrase, and DisplayClassPlural's irregular
// "y" -> "ies" rule applies to its LAST word exactly as it would to the
// bare word, so "story" -> "planned story" -> "planned stories" (never
// "planned storys" — F1's bug — and never "planned story" pluralized as
// a whole unit some other way).
func TestRenamedModelPluralsAreIrregular(t *testing.T) {
	mdl := renamedModel(t)
	if got := mdl.DisplayClassPlural("story"); got != "planned stories" {
		t.Errorf("DisplayClassPlural(%q) = %q, want %q", "story", got, "planned stories")
	}
	if got := mdl.DisplayClassPlural("spike"); got != "research spikes" {
		t.Errorf("DisplayClassPlural(%q) = %q, want %q", "spike", got, "research spikes")
	}
}

// TestRenderMarkdownEscapesPipesInTableCells is F2's sibling fix round 1
// finding, F8: a declared value containing "|" must not fracture a
// Markdown table row. Exercised directly (neither fixture's data
// contains a "|") against both an Identity value and an Evidence row,
// the two cell families F8 names.
func TestRenderMarkdownEscapesPipesInTableCells(t *testing.T) {
	doc := Document{
		Kind:          KindSpec,
		Stamp:         Stamp{Ref: "spec/x", Commit: strings.Repeat("5", 40)},
		Sections:      []SectionID{SectionIdentity, SectionEvidence},
		Words:         Words{Story: "story", StoryPlural: "stories", Spike: "spike", SpikePlural: "spikes"},
		Identity:      []KV{{Label: "Ref", Value: "spec/x | evil"}},
		Criteria:      []Criterion{{ID: "ac-1", Text: "A | B"}},
		Evidence:      []EvidenceRow{{ID: "ac-1", Status: "a | b", Summary: "c | d"}},
		EvidenceKnown: true,
	}
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "| spec/x \\| evil |") {
		t.Errorf("Identity value's \"|\" must render escaped, got:\n%s", md)
	}
	if !strings.Contains(md, "a \\| b") || !strings.Contains(md, "c \\| d") {
		t.Errorf("Evidence status/summary \"|\" must render escaped, got:\n%s", md)
	}
	if !strings.Contains(md, "A \\| B") {
		t.Errorf("Evidence table's Criterion cell (id + declared text) must also escape \"|\", got:\n%s", md)
	}
}

// TestRenderMarkdownEmptyCriteria is fix round 1's direct F2 proof: a
// Document with zero Criteria must still say so, the same way the
// Decisions/Constraints/Open questions sections already do. Neither
// fixture's Criteria list is ever empty (lockbox has 2, the renamed-story
// fixture has 11), so this is exercised synthetically rather than through
// a golden.
func TestRenderMarkdownEmptyCriteria(t *testing.T) {
	doc := Document{
		Kind:     KindSpec,
		Stamp:    Stamp{Ref: "spec/x", Commit: strings.Repeat("6", 40)},
		Sections: []SectionID{SectionCriteria},
		Words:    Words{Story: "story", StoryPlural: "stories", Spike: "spike", SpikePlural: "spikes"},
	}
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "No acceptance criteria are declared.") {
		t.Errorf("empty Criteria must render its own empty-state line, got:\n%s", md)
	}
}
