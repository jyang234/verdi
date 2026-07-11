package render

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
)

// chromaClassPrefix namespaces every CSS class chroma's class-based
// formatter emits (`chroma-chroma` on the wrapper <pre>, `chroma-kd`,
// `chroma-nf`, ... on token spans), so neither the highlighted HTML nor the
// two generated palettes below can collide with the dex/workbench
// stylesheet's own selectors. The renderer formats with this exact prefix
// (markdown.go) and the palettes are generated with it — one shared
// constant precisely so the emitted classes and the CSS that colours them
// can never drift apart.
const chromaClassPrefix = "chroma-"

// The two pinned syntax-highlighting palettes. Because the renderer emits
// class-based HTML (markdown.go), the rendered bytes carry no colour at
// all — every colour lives here instead, split across a light default and a
// dark override a page selects at view time via prefers-color-scheme. That
// is the whole fix: one theme's ink is no longer baked into the HTML, so a
// dark-mode browser gets legible light-on-dark code rather than the pinned
// github light palette showing through on a dark page.
//
// `github` and `github-dark` both ship inside chroma's own module, so
// pinning them needs no new dependency and no network. Generating the CSS
// from the pinned styles keeps chroma the single source of truth for token
// colours: a chroma upgrade re-derives both palettes, and the emitted
// classes stay in lockstep with the CSS because both use chromaClassPrefix.
var (
	// chromaLightCSS is generated from chromaStyle (github) — the same
	// style the renderer formats with (markdown.go) — so the default
	// palette and the highlighting machinery can never name different
	// styles.
	chromaLightCSS = mustChromaCSS(chromaStyle)

	chromaDarkStyle = mustStyle("github-dark")
	chromaDarkCSS   = mustChromaCSS(chromaDarkStyle)
)

// ChromaLightCSS returns the light (github) syntax-highlighting palette as
// CSS class rules (no surrounding HTML, no <style> tag). It is a pure,
// deterministic value: chroma's WriteCSS emits its rules in a fixed
// (token-type-sorted) order, so embedding this into a stylesheet keeps that
// stylesheet a pure function of the pinned style — preserving, not
// relaxing, the dex's byte-identical-rebuild property.
func ChromaLightCSS() string { return chromaLightCSS }

// ChromaDarkCSS is ChromaLightCSS's dark (github-dark) counterpart, meant
// to live inside a `@media (prefers-color-scheme: dark)` block.
func ChromaDarkCSS() string { return chromaDarkCSS }

// mustChromaCSS renders style's palette to class-based CSS once, at init.
func mustChromaCSS(style *chroma.Style) string {
	var b strings.Builder
	f := chromahtml.New(chromahtml.WithClasses(true), chromahtml.ClassPrefix(chromaClassPrefix))
	if err := f.WriteCSS(&b, style); err != nil {
		// WriteCSS only fails on a writer error, and a strings.Builder never
		// fails its Write — so reaching here means chroma's API changed
		// under us. Fail loud at init rather than silently serve a
		// colourless site.
		panic("render: generating chroma palette CSS: " + err.Error())
	}
	return b.String()
}
