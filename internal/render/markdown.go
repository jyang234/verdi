package render

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	gm "github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	gmtext "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// chromaStyle is the single, pinned chroma style the renderer tokenises
// with. The formatter emits CLASS-based HTML (see highlight), so this style
// no longer bakes any colour into the rendered bytes — its colours are
// instead emitted once, as a stylesheet, by chromacss.go (ChromaLightCSS).
// That strengthens the dex's byte-identical-rebuild property rather than
// merely preserving it: the rendered HTML is now a pure function of the
// markdown source AND carries zero palette bytes, so a theme choice can
// never perturb a single rendered byte (it lives entirely in CSS the page
// selects at view time). The same guarantee holds for the workbench, whose
// pages are likewise a pure function of the store at request time.
var chromaStyle = mustStyle("github")

func mustStyle(name string) *chroma.Style {
	if s := styles.Get(name); s != nil {
		return s
	}
	return styles.Fallback
}

// markdownRenderer is goldmark configured with GFM (tables, strikethrough,
// autolinks — all shipped inside goldmark's own module, no extra
// dependency) and auto-generated heading ids (built into goldmark core),
// plus a chroma-backed code renderer (05 §Verdi-dex mechanics: "markdown
// via goldmark and syntax highlighting via chroma at build time (pure
// Go)"). Fenced ```mermaid blocks are a deliberate exception: they render
// as a bare `<pre class="mermaid">`, left for the vendored client-side
// mermaid.js to turn into a diagram rather than being chroma-highlighted
// as code.
var markdownRenderer = gm.New(
	gm.WithExtensions(extension.GFM),
	gm.WithParserOptions(parser.WithAutoHeadingID()),
	gm.WithRendererOptions(
		gmhtml.WithUnsafe(), // trusted, render-time-only content: committed .verdi/ markdown, never user input
		renderer.WithNodeRenderers(util.Prioritized(&chromaCodeRenderer{style: chromaStyle}, 100)),
	),
)

// RenderMarkdown renders body (an artifact's markdown body, sans
// frontmatter) to a self-contained HTML fragment.
func RenderMarkdown(body string) (string, error) {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(body), &buf); err != nil {
		return "", fmt.Errorf("render: rendering markdown: %w", err)
	}
	return buf.String(), nil
}

// chromaCodeRenderer is a goldmark renderer.NodeRenderer that replaces
// goldmark's default (unhighlighted, HTML-escaped-only) code block
// rendering with chroma-tokenized, class-based HTML (the colours come from
// the generated stylesheet, chromacss.go — never inline).
type chromaCodeRenderer struct {
	style *chroma.Style
}

func (r *chromaCodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
}

func (r *chromaCodeRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	node := n.(*ast.FencedCodeBlock)
	lang := string(node.Language(source))
	code := linesText(node.Lines(), source)

	if lang == "mermaid" {
		if _, err := fmt.Fprintf(w, "<pre class=\"mermaid\">%s</pre>\n", html.EscapeString(code)); err != nil {
			return ast.WalkStop, err
		}
		return ast.WalkSkipChildren, nil
	}
	if err := r.highlight(w, code, lang); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkSkipChildren, nil
}

func (r *chromaCodeRenderer) renderCodeBlock(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	code := linesText(n.Lines(), source)
	if err := r.highlight(w, code, ""); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkSkipChildren, nil
}

// highlight tokenizes code with chroma's lexer for lang (falling back to
// plaintext for an unknown or empty language) and writes chroma's
// class-based HTML: token spans carry a `chroma-`-prefixed class, no inline
// colour. The colours are supplied by the served stylesheet's generated
// palettes (chromacss.go), so the same rendered markup is legible in both
// light and dark themes.
func (r *chromaCodeRenderer) highlight(w io.Writer, code, lang string) error {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return fmt.Errorf("render: chroma tokenise: %w", err)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(true), chromahtml.ClassPrefix(chromaClassPrefix))
	if err := formatter.Format(w, r.style, iterator); err != nil {
		return fmt.Errorf("render: chroma format: %w", err)
	}
	return nil
}

// linesText concatenates every text segment n.Lines() spans, over source.
func linesText(lines *gmtext.Segments, source []byte) string {
	var buf bytes.Buffer
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	return buf.String()
}

// HighlightCode renders code as a chroma-highlighted, class-based
// <pre><code>...</code></pre> block outside of any markdown document — used
// by pages that pretty-print a generated JSON blob rather than a
// markdown-authored code fence. Its colours, like the markdown path's, come
// from the served stylesheet's generated palettes (chromacss.go).
func HighlightCode(code, lang string) (template.HTML, error) {
	r := &chromaCodeRenderer{style: chromaStyle}
	var buf bytes.Buffer
	buf.WriteString("<pre><code>")
	if err := r.highlight(&buf, code, lang); err != nil {
		return "", err
	}
	buf.WriteString("</code></pre>")
	return template.HTML(buf.String()), nil
}
