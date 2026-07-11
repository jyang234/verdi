package dex

import (
	"embed"
	"fmt"
)

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

// writeStaticAssets writes every entry of staticAssets to
// outDir/assets/<Name>.
func writeStaticAssets(outDir string) error {
	for _, a := range staticAssets {
		data, err := embeddedAssets.ReadFile(a.EmbedPath)
		if err != nil {
			return fmt.Errorf("dex: reading embedded asset %s: %w", a.EmbedPath, err)
		}
		if err := writeFile(outDir, "assets/"+a.Name, data); err != nil {
			return err
		}
	}
	return nil
}
