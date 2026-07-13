package render

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

func TestRenderMarkdown_Happy(t *testing.T) {
	out, err := RenderMarkdown("# Hello\n\nSome *text*.\n\n```mermaid\ngraph TD; A-->B;\n```\n")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(out, `<h1 id="hello">Hello</h1>`) {
		t.Fatalf("missing auto-id heading: %s", out)
	}
	if !strings.Contains(out, `<pre class="mermaid">`) {
		t.Fatalf("mermaid fence not left bare for client-side rendering: %s", out)
	}
}

func TestRenderMarkdown_CodeBlockIsClassBased(t *testing.T) {
	out, err := RenderMarkdown("```go\nfunc main() {}\n```\n")
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	// The whole point of the fix: highlighted code carries CSS classes, not
	// inline colour. A `style="` attribute anywhere in the block would bake
	// one theme's ink into the HTML and break dark mode.
	if strings.Contains(out, `style="`) {
		t.Fatalf("highlighted code must not carry inline style attributes, got: %s", out)
	}
	if !strings.Contains(out, `class="chroma-`) {
		t.Fatalf("expected chroma-prefixed classes on highlighted tokens, got: %s", out)
	}
	if !strings.Contains(out, "func") {
		t.Fatalf("expected code content preserved, got: %s", out)
	}
}

func TestRenderBody_DiagramKindIsMermaidNotMarkdown(t *testing.T) {
	// The user-reported defect: a diagram-kind body ran through the markdown
	// renderer, collapsing the diagram DSL into a <p>graph TD ...</p>. It must
	// instead become the bare <pre class="mermaid"> the client-side engine
	// turns into an SVG.
	body := "graph TD\n  loansvc --> notification-svc\n  loansvc --> charge-svc\n"
	out, err := RenderBody(string(artifact.KindDiagram), body)
	if err != nil {
		t.Fatalf("RenderBody: %v", err)
	}
	if !strings.Contains(out, `<pre class="mermaid">`) {
		t.Fatalf("diagram body not wrapped for client-side rendering: %s", out)
	}
	// The arrow must survive as diagram syntax (HTML-escaped, textContent),
	// never as goldmark prose.
	if !strings.Contains(out, "loansvc --&gt; notification-svc") {
		t.Fatalf("diagram source not HTML-escaped verbatim: %s", out)
	}
	if strings.Contains(out, "<p>") {
		t.Fatalf("diagram body must not be markdown-rendered into a <p>: %s", out)
	}
}

func TestRenderBody_NonDiagramKindIsMarkdown(t *testing.T) {
	out, err := RenderBody(string(artifact.KindSpec), "# Title\n\nBody.\n")
	if err != nil {
		t.Fatalf("RenderBody: %v", err)
	}
	if !strings.Contains(out, `<h1 id="title">Title</h1>`) {
		t.Fatalf("spec body not markdown-rendered: %s", out)
	}
	if strings.Contains(out, `class="mermaid"`) {
		t.Fatalf("a non-diagram body must never become a mermaid block: %s", out)
	}
}

func TestChromaPaletteCSS(t *testing.T) {
	light := ChromaLightCSS()
	dark := ChromaDarkCSS()
	if light == "" || dark == "" {
		t.Fatal("chroma palette CSS must be non-empty for both themes")
	}
	// Both palettes must colour the same prefixed classes the renderer
	// emits, or the highlighted HTML would reference selectors nothing
	// defines.
	for _, css := range []string{light, dark} {
		if !strings.Contains(css, ".chroma-chroma") {
			t.Fatalf("palette CSS missing the .chroma-chroma wrapper rule: %s", css)
		}
	}
	// github-dark sets a light foreground on the wrapper; github (light)
	// does not — a cheap proof the two palettes are genuinely different
	// styles, not the same one emitted twice.
	if !strings.Contains(dark, "#e6edf3") {
		t.Fatalf("dark palette missing github-dark's light foreground, got: %s", dark)
	}
	if light == dark {
		t.Fatal("light and dark palettes are byte-identical — one style was pinned twice")
	}
	// Purity: regenerating yields the identical bytes (byte-identical
	// rebuild depends on this).
	if ChromaLightCSS() != light || ChromaDarkCSS() != dark {
		t.Fatal("palette CSS is not stable across calls")
	}
}

func TestRenderMarkdown_Negative(t *testing.T) {
	// goldmark's Convert only errors on a writer failure, which never
	// happens against a bytes.Buffer — so RenderMarkdown has no reachable
	// error path from a well-formed string input. This test instead pins
	// the negative-path contract that arbitrary (even malformed-looking)
	// markdown never panics and always returns non-error output, since
	// goldmark is deliberately permissive about its input.
	out, err := RenderMarkdown("```unterminated fence with no closing")
	if err != nil {
		t.Fatalf("RenderMarkdown returned an error for permissive markdown input: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty rendered output")
	}
}

func TestDispositionsTable_Happy(t *testing.T) {
	ds := []artifact.Disposition{
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: artifact.DispositionIncorporated, Where: "#design-notes"},
		{Sticky: "a-01J8Z0K4BBBBBBBBBBBBBBBBBB", Disposition: artifact.DispositionContradicted, Note: "out of scope"},
		{Sticky: "a-01J8Z0K5CCCCCCCCCCCCCCCCCC", Disposition: artifact.DispositionOpenQuestion},
	}
	got := string(DispositionsTable(ds))
	if !strings.Contains(got, "<table class=\"dispositions-table\">") {
		t.Fatalf("missing table wrapper: %s", got)
	}
	if !strings.Contains(got, `href="#design-notes"`) {
		t.Fatalf("incorporated disposition missing where link: %s", got)
	}
	if !strings.Contains(got, "out of scope") {
		t.Fatalf("contradicted disposition missing note: %s", got)
	}
	if strings.Count(got, "<tr>") != 4 { // header + 3 rows
		t.Fatalf("expected 4 <tr> (1 header + 3 rows), got %d: %s", strings.Count(got, "<tr>"), got)
	}
}

func TestDispositionsTable_Negative_Empty(t *testing.T) {
	if got := DispositionsTable(nil); got != "" {
		t.Fatalf("expected empty string for no dispositions, got %q", got)
	}
}
