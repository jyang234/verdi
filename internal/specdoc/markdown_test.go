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
	facts := WithMatrix(FactsFromSpec(fm), matrixFixture(), "matrix at "+commit[:8])
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

func matrixFixture() matrixprojection.Record {
	return matrixprojection.Record{Feature: &matrixprojection.FeatureBody{ACs: []matrixprojection.FeatureAC{
		{ID: "ac-1", Status: "eligible", Summary: "one implementing story, not yet closed", ImplementingStories: []string{"spec/key-holder"}},
		{ID: "ac-2", Status: "violated", Summary: "no implementing story"},
	}}}
}
