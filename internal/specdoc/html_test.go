package specdoc

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/render"
)

func TestRenderHTMLGolden(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("0", 39) + "1"
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: WithMatrix(FactsFromSpec(fm), matrixFixture(), commit), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, "golden-spec.html", html)
	for _, want := range []string{"<h1", "<h2", "<table", "not authority"} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q", want)
		}
	}
	if strings.Contains(html, "<html") || strings.Contains(html, "<body") {
		t.Errorf("RenderHTML must return a fragment, not a page")
	}
}

// TestRenderHTMLMatchesMarkdownEngine is the other half of F10 (fix
// round 1): RenderHTML must be exactly render.RenderMarkdown applied to
// RenderMarkdown's own output — no second, divergent rendering path.
func TestRenderHTMLMatchesMarkdownEngine(t *testing.T) {
	fm, body := loadFixture(t)
	commit := strings.Repeat("8", 40)
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: commit}, Facts: WithMatrix(FactsFromSpec(fm), matrixFixture(), commit), Kind: KindSpec})
	if err != nil {
		t.Fatal(err)
	}
	html, err := RenderHTML(doc)
	if err != nil {
		t.Fatal(err)
	}
	want, err := render.RenderMarkdown(RenderMarkdown(doc))
	if err != nil {
		t.Fatal(err)
	}
	if html != want {
		t.Fatalf("RenderHTML(doc) must equal render.RenderMarkdown(RenderMarkdown(doc))")
	}
}
