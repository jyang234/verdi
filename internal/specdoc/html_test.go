package specdoc

import (
	"strings"
	"testing"
)

func TestRenderHTMLGolden(t *testing.T) {
	fm, body := loadFixture(t)
	doc, err := Build(Input{Spec: fm, Body: body, Status: "accepted-pending-build", Stamp: Stamp{Ref: "spec/lockbox", Commit: strings.Repeat("0", 39) + "1"}, Facts: WithMatrix(FactsFromSpec(fm), matrixFixture(), "matrix at 00000000"), Kind: KindSpec})
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
