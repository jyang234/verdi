package workbench

import (
	"embed"
	"net/http"

	"github.com/jyang234/verdi/internal/dex"
)

//go:embed assets/board.js assets/boardspec.js assets/boarddiagram.js
var embeddedAssets embed.FS

// mermaidHandler serves dex's vendored mermaid.min.js (05 §Workbench:
// "mermaid client-side reusing the dex's vendored asset") — the same
// bytes, not a second copy.
func mermaidHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := dex.MermaidJS()
		if err != nil {
			http.Error(w, "mermaid.min.js unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}

// styleCSSHandler serves internal/dex's composed stylesheet — the same
// bytes dex writes to its static site, chroma light/dark palettes and all
// (dex.StyleCSS) — so the workbench's shared class-based code rendering is
// coloured and equally dark-mode-correct without owning a second stylesheet
// (the same one-copy-two-surfaces pattern as the vendored mermaid.min.js).
func styleCSSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := dex.StyleCSS()
		if err != nil {
			http.Error(w, "style.css unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}

// boardJSHandler serves the v0 board page's one JS file (05 §Workbench:
// "keep board JS minimal and in ONE file").
func boardJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/board.js")
}

// boardSpecJSHandler serves the v1 board's one JS file — same minimal,
// dependency-free posture as the v0 board's.
func boardSpecJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/boardspec.js")
}

// boardDiagramJSHandler serves the diagram editor's one JS file
// (spec/board-editor) — same minimal, dependency-free posture.
func boardDiagramJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/boarddiagram.js")
}

func embeddedJSHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := embeddedAssets.ReadFile(name)
		if err != nil {
			http.Error(w, name+" unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}
