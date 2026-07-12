package designscaffold

import (
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func TestHumanizeName(t *testing.T) {
	cases := map[string]string{
		"stale-decline":     "Stale Decline",
		"loan-mgmt":         "Loan Mgmt",
		"single":            "Single",
		"":                  "",
		"leading--doubled-": "Leading  Doubled ",
	}
	for in, want := range cases {
		if got := HumanizeName(in); got != want {
			t.Errorf("HumanizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFeature proves Feature's output self-validates as a draft feature
// spec carrying the 05 §CLI exit criterion's minimum surface (attributes,
// ACs, a stub) — the exact content cmd/verdi/design.go's `design start`
// relies on, now moved here (CLAUDE.md: two consumers, shared internal/
// home).
func TestFeature(t *testing.T) {
	for _, storyRef := range []string{"", "jira:LOAN-1482"} {
		content := Feature("spec/stale-decline", storyRef, "Stale decline handling")
		fm, _, err := artifact.SplitFrontmatter([]byte(content))
		if err != nil {
			t.Fatalf("SplitFrontmatter: %v", err)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			t.Fatalf("DecodeSpec: %v", err)
		}
		if spec.Class != artifact.ClassFeature {
			t.Fatalf("Class = %q, want feature", spec.Class)
		}
		if spec.Story != storyRef {
			t.Fatalf("Story = %q, want %q", spec.Story, storyRef)
		}
		if spec.Problem == nil || spec.Outcome == nil {
			t.Fatal("Feature scaffold has no problem/outcome")
		}
		if len(spec.AcceptanceCriteria) == 0 {
			t.Fatal("Feature scaffold has no acceptance criteria")
		}
		if len(spec.Stubs) == 0 {
			t.Fatal("Feature scaffold has no stubs")
		}
	}
}

// TestStory_Plain proves Story's non-spike path: a required story: ref, a
// placeholder AC, and the caller-supplied implements link(s).
func TestStory_Plain(t *testing.T) {
	links := []StoryLink{{Type: artifact.LinkImplements, Ref: "spec/loan-mgmt#ac-1"}}
	content := Story("spec/loan-mgmt-story", "jira:LOAN-1482", "Loan Mgmt Story", false, links)
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassStory {
		t.Fatalf("Class = %q, want story", spec.Class)
	}
	if spec.Spike {
		t.Fatal("Spike = true, want false")
	}
	if spec.Story != "jira:LOAN-1482" {
		t.Fatalf("Story = %q, want jira:LOAN-1482", spec.Story)
	}
	if spec.Problem == nil || spec.Outcome == nil {
		t.Fatal("Story scaffold has no problem/outcome")
	}
	var foundImplements bool
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements && l.Ref == "spec/loan-mgmt#ac-1" {
			foundImplements = true
		}
	}
	if !foundImplements {
		t.Fatalf("Story scaffold links = %+v, want the supplied implements edge", spec.Links)
	}
}

// TestStory_Spike proves Story's spike path: spike: true, no implements
// edges, no acceptance_criteria placeholder, and the caller-supplied
// resolves link(s) — validateStory's "spike carries NO implements edges"
// / ">=1 resolves edge" grammar (02 §Kind registry).
func TestStory_Spike(t *testing.T) {
	links := []StoryLink{
		{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-1"},
		{Type: artifact.LinkResolves, Ref: "spec/scoping-canvas#oq-2"},
	}
	content := Story("spec/retry-strategy-spike", "todo:REPLACE-ME", "Retry Strategy Spike", true, links)
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if !spec.Spike {
		t.Fatal("Spike = false, want true")
	}
	if len(spec.AcceptanceCriteria) != 0 {
		t.Fatalf("spike scaffold declares acceptance_criteria = %+v, want none", spec.AcceptanceCriteria)
	}
	var resolvesCount int
	for _, l := range spec.Links {
		if l.Type == artifact.LinkImplements {
			t.Fatal("spike scaffold carries an implements edge, want none")
		}
		if l.Type == artifact.LinkResolves {
			resolvesCount++
		}
	}
	if resolvesCount != 2 {
		t.Fatalf("resolves edge count = %d, want 2", resolvesCount)
	}
}

// TestStory_Negative_NoLinks proves the caller's contract is enforced:
// Story refuses to render with zero links, since a story with no
// implements/resolves edges would decode but fail validateStory anyway —
// better to fail fast, close to the mistake, than downstream at decode.
func TestStory_Negative_NoLinks(t *testing.T) {
	content := Story("spec/x", "jira:LOAN-1", "X", false, nil)
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(fm); err == nil {
		t.Fatal("Story with no links decoded successfully, want a validateStory failure (no implements edge)")
	}
}
