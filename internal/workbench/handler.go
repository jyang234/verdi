package workbench

import "net/http"

// NewHandler builds the workbench's full HTTP handler for the store
// rooted at root, with no injected collaborators (no forge wired: the
// board renders authoring/read-only purely from branch state).
func NewHandler(root string) http.Handler {
	return NewHandlerWith(root, Deps{})
}

// NewHandlerWith builds the handler with injected collaborators (Deps).
func NewHandlerWith(root string, deps Deps) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutesWith(mux, root, deps)
	return mux
}

// RegisterRoutes wires every workbench route onto mux with empty Deps —
// the pre-v1 signature, kept for existing callers and tests.
func RegisterRoutes(mux *http.ServeMux, root string) {
	RegisterRoutesWith(mux, root, Deps{})
}

// RegisterRoutesWith wires every workbench route onto mux. The single
// place a phase adds a page.
func RegisterRoutesWith(mux *http.ServeMux, root string, deps Deps) {
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

	// The disclosures page (spec/disclosures-panel): the checkout's
	// current disclosures, enumerated fresh per render through the shared
	// internal/disclosureview compute path (the dex ships the read-only
	// edition of the same view).
	mux.HandleFunc("/disclosures", disclosuresHandler(root, deps.Disclosures))

	// The verdict viewer: cross-commit per-AC diff of a story's derived
	// verdicts.json snapshots.
	mux.HandleFunc("/verdict/{story...}", verdictHandler(root))

	// The advisory preview matrix page (03 §Evidence records: "advisory
	// renders as preview").
	mux.HandleFunc("/matrix/{story...}", matrixHandler(root))

	// The v1 board: the spec-as-source projection (05 §Workbench, R4).
	// "/board/spec/{name}" is strictly more specific than the v0
	// "/board/{key}/{action}" patterns below, so ServeMux routes every
	// /board/spec/* request here without conflict.
	bs := &boardSpecServer{root: root, feed: deps.CommentFeed, reviewUnavailable: deps.ReviewUnavailable}
	mux.HandleFunc("/board/spec/{name}", bs.boardSpecPageHandler())
	mux.HandleFunc("/board/spec/{name}/fragment", bs.boardSpecFragmentHandler())
	mux.HandleFunc("/board/spec/{name}/api/{action}", bs.boardSpecAPIHandler())
	mux.HandleFunc("/board/spec/{name}/peek", bs.boardPeekHandler())

	// The v0 board — superseded by board-as-projection (R4-I-9) but kept
	// reachable for grandfathered v0 board.json state. Its two POST
	// routes share one {action} pattern so the v1 literal "spec" segment
	// above can coexist (two sibling 3-segment patterns with different
	// literal positions would be an unresolvable ServeMux conflict).
	mux.HandleFunc("/board/{key}", boardHandler(root))
	mux.HandleFunc("/board/{key}/{action}", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("action") {
		case "autosave":
			boardAutosaveHandler(root)(w, r)
		case "commit":
			boardCommitHandler(root)(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Static assets: the composed stylesheet and vendored mermaid.min.js
	// (both shared with internal/dex — one copy in the binary) and the
	// two board scripts (v0 board.js, v1 boardspec.js).
	mux.HandleFunc("/assets/style.css", styleCSSHandler())
	mux.HandleFunc("/assets/mermaid.min.js", mermaidHandler())
	mux.HandleFunc("/assets/board.js", boardJSHandler())
	mux.HandleFunc("/assets/boardspec.js", boardSpecJSHandler())
}
