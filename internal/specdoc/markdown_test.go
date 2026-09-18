package specdoc

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files under testdata/")

func loadFixture(t *testing.T) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "lockbox.md"))
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

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update): %v", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs from golden; run `go test ./internal/specdoc/ -update` and review the diff\n--- got ---\n%s", name, got)
	}
}

func TestRenderMarkdownGoldens(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	facts := WithMatrix(FactsFromSpec(fm), matrixFixture(), commit)
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: facts, Kind: k})
		if err != nil {
			t.Fatal(err)
		}
		checkGolden(t, "golden-"+string(k)+".md", RenderMarkdown(doc))
	}
	proposed, err := Build(Input{Spec: fm, Body: body, Status: "proposed", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit, Proposed: true}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec-proposed.md", RenderMarkdown(proposed))
	nofacts, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec-nofacts.md", RenderMarkdown(nofacts))
}

func TestRenderMarkdownIsDeterministic(t *testing.T) {
	fm, body := loadFixture(t)
	for _, k := range []Kind{KindSpec, KindPlan, KindTasks} {
		in := Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("d", 40)}, Facts: FactsFromSpec(fm), Kind: k}
		a, err := Build(in)
		if err != nil {
			t.Fatalf("%s: Build: %v", k, err)
		}
		b, err := Build(in)
		if err != nil {
			t.Fatalf("%s: Build: %v", k, err)
		}
		if RenderMarkdown(a) != RenderMarkdown(b) {
			t.Fatalf("%s: two renders of the same input differ", k)
		}
	}
}

// TestRenderMarkdownEndsWithSingleNewline is half of F10 (fix round 1);
// the other half, RenderHTML(doc) == render.RenderMarkdown(RenderMarkdown(doc)),
// lives in html_test.go alongside the RenderHTML it exercises.
func TestRenderMarkdownEndsWithSingleNewline(t *testing.T) {
	fm, body := loadFixture(t)
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("9", 40)}, Facts: FactsFromSpec(fm), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(doc)
	if !strings.HasSuffix(md, "\n") {
		t.Fatalf("RenderMarkdown must end with a newline")
	}
	if strings.HasSuffix(md, "\n\n") {
		t.Fatalf("RenderMarkdown must end with exactly one newline, not more")
	}
}

func TestRenderMarkdownHonestyLines(t *testing.T) {
	fm, body := loadFixture(t)
	doc, _ := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("e", 40)}, Kind: KindSpec})
	md := RenderMarkdown(doc)
	for _, want := range []string{
		"Coverage: not computed for this render.",
		"Claims: not computed for this render.",
		"Evidence was not supplied for this render.",
		"Status | not resolved for this render",
		"Derived from the spec's objects; not authority.",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing honesty line %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Proposed, not") {
		t.Errorf("an accepted render must not carry the proposed header")
	}
}

// TestPluralIf is fix round 2's F1(a): a table-driven proof that
// pluralIf's plural branch is reachable and correct, for n = 0, 1, 2,
// under both the default and a renamed (multi-word) vocabulary. Before
// this test, no committed golden ever built a Document whose Coverage,
// Claims, or Evidence.Stories held more than one entry, so an
// always-singular mutant of pluralIf survived the whole suite; see
// TestRenderMarkdownPluralBranches for the same proof at the rendered-
// output level, and the fix-round report for the mutant kill transcript.
func TestPluralIf(t *testing.T) {
	tests := []struct {
		name             string
		singular, plural string
		n                int
		want             string
	}{
		{"n=0, default", "story", "stories", 0, "stories"},
		{"n=1, default", "story", "stories", 1, "story"},
		{"n=2, default", "story", "stories", 2, "stories"},
		{"n=0, renamed", "planned story", "planned stories", 0, "planned stories"},
		{"n=1, renamed", "planned story", "planned stories", 1, "planned story"},
		{"n=2, renamed", "planned story", "planned stories", 2, "planned stories"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pluralIf(tt.singular, tt.plural, tt.n); got != tt.want {
				t.Errorf("pluralIf(%q, %q, %d) = %q, want %q", tt.singular, tt.plural, tt.n, got, tt.want)
			}
		})
	}
}

// TestRenderMarkdownPluralBranches is fix round 2's F1(b): a rendered
// proof, not just a unit test on pluralIf in isolation. Neither
// committed fixture ever gives a criterion more than one covering stub
// or more than one implementing story, so this builds the lockbox
// fixture with Facts overridden to give ac-1 two of each and asserts the
// exact plural lines render. No golden changes — a focused assertion
// test, per the finding's own instruction.
func TestRenderMarkdownPluralBranches(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("4", 40)
	facts := FactsFromSpec(fm)
	facts.Coverage["ac-1"] = []string{"key-holder", "other"}
	facts = WithMatrix(facts, matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "eligible", Summary: "two implementing stories", ImplementingStories: []string{"spec/a", "spec/b"}},
		{ID: "ac-2", Status: "violated", Summary: "no implementing story"},
	}}}, commit)
	doc, err := Build(Input{Spec: fm, Body: body, Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: facts, Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	md := RenderMarkdown(doc)
	if !strings.Contains(md, "covered by stories `key-holder`, `other`") {
		t.Errorf("missing plural coverage line, got:\n%s", md)
	}
	if !strings.Contains(md, "implementing stories: `spec/a`, `spec/b`") {
		t.Errorf("missing plural evidence-detail line, got:\n%s", md)
	}
}

// TestCriterionCellAndIdsWithTextMissingID is fix round 2's A3 minor
// proof: an id absent from the id->text map (a dangling stub coverage/
// resolves entry, or an evidence row naming an id the spec never
// declared) must render an explicit, honest gap — never a bare trailing
// "id — " separator or empty "id ()" parentheses.
func TestCriterionCellAndIdsWithTextMissingID(t *testing.T) {
	textByID := map[string]string{"ac-1": "A key opens one box."}
	if got, want := criterionCell("ac-9", textByID), "ac-9 — (not declared on this spec)"; got != want {
		t.Errorf("criterionCell(missing id) = %q, want %q", got, want)
	}
	if got, want := idsWithText([]string{"ac-9"}, textByID, "no declared criterion."), "ac-9 (not declared on this spec)"; got != want {
		t.Errorf("idsWithText(missing id) = %q, want %q", got, want)
	}
}

func matrixFixture() matrixprojection.Record {
	return matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "eligible", Summary: "one implementing story, not yet closed", ImplementingStories: []string{"spec/key-holder"}},
		{ID: "ac-2", Status: "violated", Summary: "no implementing story"},
	}}}
}
