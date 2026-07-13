package dex

import (
	"embed"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/render"
)

// The two placeholder markers in assets/style.css that StyleCSS replaces
// with internal/render's generated syntax-highlighting palettes. Keeping
// the palettes OUT of the committed asset (chroma is their one source of
// truth) and composing them in at build/serve time is what keeps the served
// stylesheet self-contained AND a pure function of the pinned styles.
const (
	chromaLightMarker = "/* CHROMA-LIGHT-PALETTE */"
	chromaDarkMarker  = "/* CHROMA-DARK-PALETTE */"
)

// StyleCSS returns the served stylesheet: the committed assets/style.css
// with the light palette (github) composed in at its default marker and the
// dark palette (github-dark) composed in inside the existing
// prefers-color-scheme:dark block. The result is deterministic (a pure
// function of the embedded bytes and the two pinned chroma styles), so
// writing it into a dex build stays byte-identical across rebuilds.
//
// internal/workbench serves this exact same composed stylesheet at its own
// /assets/style.css route (as it already reuses this package's vendored
// mermaid.min.js) rather than owning a second copy — one stylesheet in the
// binary, two surfaces, so both surfaces' shared class-based code rendering
// is coloured identically and is equally dark-mode-correct.
func StyleCSS() ([]byte, error) {
	raw, err := embeddedAssets.ReadFile("assets/style.css")
	if err != nil {
		return nil, fmt.Errorf("dex: reading embedded style.css: %w", err)
	}
	css := string(raw)
	for marker, palette := range map[string]string{
		chromaLightMarker: render.ChromaLightCSS(),
		chromaDarkMarker:  render.ChromaDarkCSS(),
	} {
		if !strings.Contains(css, marker) {
			return nil, fmt.Errorf("dex: style.css is missing the %q marker — the chroma palette cannot be composed in", marker)
		}
		css = strings.Replace(css, marker, palette, 1)
	}
	return []byte(css), nil
}

// embeddedAssets embeds dex's entire static client surface into the
// verdi binary, so a build never depends on locating its own source tree
// at runtime — the "vendor mermaid.min.js so the site is self-contained"
// instruction extended to every asset dex ships. See
// internal/dex/assets/mermaid/README.md for mermaid's vendoring
// provenance (version, source URL, sha256, license).
//
//go:embed assets/style.css assets/search.js assets/openapi-renderer.js assets/mermaid/mermaid.min.js
var embeddedAssets embed.FS

// assetFile is one static file dex writes to outDir/assets/<Name>, sourced
// from embeddedAssets at EmbedPath.
type assetFile struct {
	EmbedPath string
	Name      string
}

// staticAssets is dex's fixed, closed asset list — exactly three
// JavaScript files (mermaid.min.js, search.js, openapi-renderer.js: 05
// §Verdi-dex mechanics' "client-side JavaScript budget is exactly three
// items") plus the one stylesheet.
var staticAssets = []assetFile{
	{EmbedPath: "assets/style.css", Name: "style.css"},
	{EmbedPath: "assets/search.js", Name: "search.js"},
	{EmbedPath: "assets/openapi-renderer.js", Name: "openapi-renderer.js"},
	{EmbedPath: "assets/mermaid/mermaid.min.js", Name: "mermaid.min.js"},
}

// MermaidJS returns the vendored mermaid.min.js bytes this package embeds
// (internal/dex/assets/mermaid/README.md has the vendoring provenance).
// internal/workbench serves this exact same asset at its own
// /assets/mermaid.min.js route rather than vendoring a second copy (05
// §Workbench: "mermaid client-side reusing the dex's vendored asset") —
// one vendored file in the binary, two surfaces reading it.
func MermaidJS() ([]byte, error) {
	return embeddedAssets.ReadFile("assets/mermaid/mermaid.min.js")
}

// writeStaticAssets writes every entry of staticAssets to
// outDir/assets/<Name>.
func writeStaticAssets(outDir string) error {
	for _, a := range staticAssets {
		var data []byte
		var err error
		if a.Name == "style.css" {
			// The stylesheet is the one asset that is composed, not copied
			// verbatim: its two chroma palettes are generated (StyleCSS).
			data, err = StyleCSS()
		} else {
			data, err = embeddedAssets.ReadFile(a.EmbedPath)
		}
		if err != nil {
			return fmt.Errorf("dex: reading asset %s: %w", a.Name, err)
		}
		if err := writeFile(outDir, "assets/"+a.Name, data); err != nil {
			return err
		}
	}
	return nil
}
