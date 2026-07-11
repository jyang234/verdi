package dex

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"

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
// byte-identical-rebuild requirement.
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
// mermaid.js to turn into a diagram (the first of dex's three-item JS
// budget) rather than being chroma-highlighted as code.
var markdownRenderer = gm.New(
	gm.WithExtensions(extension.GFM),
	gm.WithParserOptions(parser.WithAutoHeadingID()),
	gm.WithRendererOptions(
		gmhtml.WithUnsafe(), // trusted, build-time-only content: committed .verdi/ markdown, never user input
		renderer.WithNodeRenderers(util.Prioritized(&chromaCodeRenderer{style: chromaStyle}, 100)),
	),
)

// renderMarkdown renders body (an artifact's markdown body, sans
// frontmatter) to a self-contained HTML fragment.
func renderMarkdown(body string) (string, error) {
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(body), &buf); err != nil {
		return "", fmt.Errorf("dex: rendering markdown: %w", err)
	}
	return buf.String(), nil
}

// chromaCodeRenderer is a goldmark renderer.NodeRenderer that replaces
// goldmark's default (unhighlighted, HTML-escaped-only) code block
// rendering with chroma-tokenized, inline-styled HTML — the "syntax
// highlighting via chroma at build time" mechanic. It is the only
// NodeRenderer this package registers; every other node kind still goes
// through goldmark's own default HTML renderer as usual, since dex adds
// this one alongside (Priority 100, ahead of goldmark's own priority-1000
// default) rather than replacing the renderer wholesale.
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
		fmt.Fprintf(w, "<pre class=\"mermaid\">%s</pre>\n", html.EscapeString(code))
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
// inline-styled HTML — no separate stylesheet dependency, so a page is
// fully self-contained the moment its own bytes are served.
func (r *chromaCodeRenderer) highlight(w util.BufWriter, code, lang string) error {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return fmt.Errorf("dex: chroma tokenise: %w", err)
	}
	formatter := chromahtml.New(chromahtml.WithClasses(false))
	if err := formatter.Format(w, r.style, iterator); err != nil {
		return fmt.Errorf("dex: chroma format: %w", err)
	}
	return nil
}

// linesText concatenates every text segment n.Lines() spans, over source —
// the raw code block content goldmark's parser already isolated.
func linesText(lines *gmtext.Segments, source []byte) string {
	var buf bytes.Buffer
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		buf.Write(seg.Value(source))
	}
	return buf.String()
}

// tocHeadingRe extracts every heading goldmark emitted with an
// auto-generated id — <h2 id="foo">Text</h2> — to build the on-this-page
// TOC (05 §Verdi-dex page anatomy) as a second pass over already-rendered
// HTML rather than a second AST walk; simpler, and just as deterministic
// since it's a pure function of the same rendered bytes.
var tocHeadingRe = regexp.MustCompile(`(?s)<h([2-4]) id="([^"]*)">(.*?)</h[2-4]>`)

// innerTagRe strips any nested tags (e.g. <code>, <em>) a heading's inline
// markdown produced, so the TOC shows plain text labels.
var innerTagRe = regexp.MustCompile(`<[^>]+>`)

// TOCEntry is one on-this-page table-of-contents entry.
type TOCEntry struct {
	Level int
	ID    string
	Text  string
}

// extractTOC walks renderedHTML's h2-h4 headings (goldmark's
// WithAutoHeadingID gave each one a stable id) in document order.
func extractTOC(renderedHTML string) []TOCEntry {
	matches := tocHeadingRe.FindAllStringSubmatch(renderedHTML, -1)
	entries := make([]TOCEntry, 0, len(matches))
	for _, m := range matches {
		level := 2
		fmt.Sscanf(m[1], "%d", &level)
		text := strings.TrimSpace(innerTagRe.ReplaceAllString(m[3], ""))
		entries = append(entries, TOCEntry{Level: level, ID: m[2], Text: html.UnescapeString(text)})
	}
	return entries
}
