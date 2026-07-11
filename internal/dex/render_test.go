package dex

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Happy(t *testing.T) {
	body := "# Title\n\nSome *text* with a [link](https://example.com).\n\n## Section one\n\nBody text.\n"
	out, err := renderMarkdown(body)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if !strings.Contains(out, `<h1 id="title">Title</h1>`) {
		t.Errorf("missing auto-id h1, got: %s", out)
	}
	if !strings.Contains(out, `<h2 id="section-one">Section one</h2>`) {
		t.Errorf("missing auto-id h2, got: %s", out)
	}
	if !strings.Contains(out, "<em>text</em>") {
		t.Errorf("expected emphasis rendering, got: %s", out)
	}
}

func TestRenderMarkdown_CodeBlockIsChromaHighlighted(t *testing.T) {
	body := "```go\nfunc main() {}\n```\n"
	out, err := renderMarkdown(body)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	// chroma's class-based output wraps tokens in <span class="chroma-...">,
	// distinct from goldmark's own unhighlighted <pre><code> passthrough —
	// and carries NO inline colour, so dark mode can restyle it via the
	// served stylesheet's dark palette.
	if !strings.Contains(out, `class="chroma-`) {
		t.Errorf("expected chroma class-based spans, got: %s", out)
	}
	if strings.Contains(out, `style="`) {
		t.Errorf("highlighted code must carry no inline style attributes, got: %s", out)
	}
	if !strings.Contains(out, "func") {
		t.Errorf("expected code content preserved, got: %s", out)
	}
}

func TestRenderMarkdown_MermaidBlockNotHighlighted(t *testing.T) {
	body := "```mermaid\ngraph TD\n  a --> b\n```\n"
	out, err := renderMarkdown(body)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if !strings.Contains(out, `<pre class="mermaid">`) {
		t.Errorf("expected a bare <pre class=\"mermaid\"> block, got: %s", out)
	}
	if strings.Contains(out, `style="`) {
		t.Errorf("mermaid block must not be chroma-highlighted, got: %s", out)
	}
	if !strings.Contains(out, "graph TD") {
		t.Errorf("expected mermaid source preserved verbatim, got: %s", out)
	}
}

func TestRenderMarkdown_Negative_Deterministic(t *testing.T) {
	body := "# Same input\n\nTwice.\n"
	out1, err := renderMarkdown(body)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	out2, err := renderMarkdown(body)
	if err != nil {
		t.Fatalf("renderMarkdown: %v", err)
	}
	if out1 != out2 {
		t.Fatalf("renderMarkdown not deterministic:\n%s\nvs\n%s", out1, out2)
	}
}

func TestExtractTOC_Happy(t *testing.T) {
	html := `<h1 id="title">Title</h1><p>x</p><h2 id="a">Section A</h2><h3 id="b">Sub B</h3>`
	toc := extractTOC(html)
	if len(toc) != 2 {
		t.Fatalf("extractTOC: got %d entries, want 2 (h1 excluded): %+v", len(toc), toc)
	}
	if toc[0].ID != "a" || toc[0].Text != "Section A" || toc[0].Level != 2 {
		t.Errorf("toc[0] = %+v, want {2 a Section A}", toc[0])
	}
	if toc[1].ID != "b" || toc[1].Text != "Sub B" || toc[1].Level != 3 {
		t.Errorf("toc[1] = %+v, want {3 b Sub B}", toc[1])
	}
}

func TestExtractTOC_Negative_NoHeadings(t *testing.T) {
	if toc := extractTOC("<p>no headings here</p>"); len(toc) != 0 {
		t.Fatalf("extractTOC: got %+v, want none", toc)
	}
}
