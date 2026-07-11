package workbench

import (
	"embed"
	"net/http"

	"github.com/OWNER/verdi/internal/dex"
)

//go:embed assets/board.js
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
		w.Write(data)
	}
}

// boardJSHandler serves the board page's one JS file (05 §Workbench:
// "keep board JS minimal and in ONE file").
func boardJSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := embeddedAssets.ReadFile("assets/board.js")
		if err != nil {
			http.Error(w, "board.js unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Write(data)
	}
}
