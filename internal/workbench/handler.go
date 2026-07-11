package workbench

import "net/http"

// NewHandler builds the workbench's full HTTP handler for the store
// rooted at root. Phase 10 (PLAN.md Stubs: "workbench pages beyond a
// health page land in phase 10") adds real corpus/board/verdict pages by
// registering more routes in RegisterRoutes — this function itself never
// needs to change shape.
func NewHandler(root string) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, root)
	return mux
}

// RegisterRoutes wires every workbench route onto mux. The single place a
// future phase adds a page.
func RegisterRoutes(mux *http.ServeMux, root string) {
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/", indexHandler(root))

	// Corpus artifact pages (05 §Workbench: server-rendered, goldmark +
	// client-side mermaid). Registered WITHOUT a method prefix (unlike
	// Go 1.22+ ServeMux's "GET /path" form) so every handler's own
	// r.Method check is what produces 405 — a method-qualified pattern
	// here would sometimes fall through to the "/" catch-all instead (its
	// own indexHandler 404s on any non-"/" path), which would silently
	// turn a wrong-method request on a real route into a confusing 404.
	mux.HandleFunc("/a/{kind}/{name}", corpusHandler(root))

	// The verdict viewer: cross-commit per-AC diff of a story's derived
	// verdicts.json snapshots.
	mux.HandleFunc("/verdict/{story...}", verdictHandler(root))

	// The advisory preview matrix page (03 §Evidence records: "advisory
	// renders as preview").
	mux.HandleFunc("/matrix/{story...}", matrixHandler(root))

	// The board: cards, stickies, yarn, autosave, commit-to-design.
	mux.HandleFunc("/board/{key}", boardHandler(root))
	mux.HandleFunc("/board/{key}/autosave", boardAutosaveHandler(root))
	mux.HandleFunc("/board/{key}/commit", boardCommitHandler(root))

	// Static assets: the composed stylesheet and vendored mermaid.min.js
	// (both shared with internal/dex — one copy in the binary) and the
	// board's one JS file.
	mux.HandleFunc("/assets/style.css", styleCSSHandler())
	mux.HandleFunc("/assets/mermaid.min.js", mermaidHandler())
	mux.HandleFunc("/assets/board.js", boardJSHandler())
}
