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

// chromaStyle is the single, pinned chroma style every rendered code block
// uses. Pinning one style (rather than reading a runtime choice) keeps
// rendered HTML a pure function of the markdown source, per dex's
// byte-identical-rebuild requirement — a property this package's shared
// use by the workbench does not relax, since the workbench's own pages are
// likewise a pure function of the store at request time.
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
// rendering with chroma-tokenized, inline-styled HTML.
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
// inline-styled HTML.
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
	formatter := chromahtml.New(chromahtml.WithClasses(false))
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

// HighlightCode renders code as a chroma-highlighted, inline-styled
// <pre><code>...</code></pre> block outside of any markdown document — used
// by pages that pretty-print a generated JSON blob rather than a
// markdown-authored code fence.
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
