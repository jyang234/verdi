package workbench

import (
	"net/http"

	"github.com/jyang234/verdi/internal/store"
)

// The v1 board's route suffixes — the one route table's row names, shared
// by the unprefixed mount and the /b/{branch} prefix mount (and by the
// per-branch dispatcher, which switches on them for its sealed render).
const (
	routeBoardPage      = "/board/spec/{name}"
	routeBoardFragment  = "/board/spec/{name}/fragment"
	routeBoardAPI       = "/board/spec/{name}/api/{action}"
	routeBoardPeek      = "/board/spec/{name}/peek"
	routeBoardPinSearch = "/board/spec/{name}/pinsearch"
)

// boardSpecRoute is one row of the v1 board's route table: a route
// suffix, the boardSpecServer method serving it, and whether its client
// reads failures as JSON (the api route) rather than HTML.
type boardSpecRoute struct {
	suffix  string
	handler func(*boardSpecServer) http.HandlerFunc
	json    bool
}

// boardSpecRoutes is THE v1 board route table, declared once
// (spec/draft-boards dc-1: "no second route table") and mounted at the
// root and beneath /b/{branch} alike by RegisterRoutesWith.
func boardSpecRoutes() []boardSpecRoute {
	return []boardSpecRoute{
		{suffix: routeBoardPage, handler: (*boardSpecServer).boardSpecPageHandler},
		{suffix: routeBoardFragment, handler: (*boardSpecServer).boardSpecFragmentHandler},
		{suffix: routeBoardAPI, handler: (*boardSpecServer).boardSpecAPIHandler, json: true},
		{suffix: routeBoardPeek, handler: (*boardSpecServer).boardPeekHandler},
		{suffix: routeBoardPinSearch, handler: (*boardSpecServer).boardPinSearchHandler},
	}
}

// NewHandler builds the workbench's full HTTP handler for the store
// rooted at root, with no injected collaborators (no forge wired: the
// board renders authoring/read-only purely from branch state).
func NewHandler(root string) http.Handler {
	return NewHandlerWith(root, Deps{})
}

// NewHandlerWith builds the handler with injected collaborators (Deps)
// and the home page's production wiring (HomeDeps' zero value).
func NewHandlerWith(root string, deps Deps) http.Handler {
	return NewHandlerWithHome(root, deps, HomeDeps{})
}

// NewHandlerWithHome builds the handler with injected collaborators for
// both the board (Deps) and the directory home page (HomeDeps —
// spec/directory-home; a separate struct so the board's dependency
// surface is untouched).
func NewHandlerWithHome(root string, deps Deps, home HomeDeps) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutesWithHome(mux, root, deps, home)
	return mux
}

// RegisterRoutes wires every workbench route onto mux with empty Deps —
// the pre-v1 signature, kept for existing callers and tests.
func RegisterRoutes(mux *http.ServeMux, root string) {
	RegisterRoutesWith(mux, root, Deps{})
}

// RegisterRoutesWith wires every workbench route onto mux with the home
// page's production wiring.
func RegisterRoutesWith(mux *http.ServeMux, root string, deps Deps) {
	RegisterRoutesWithHome(mux, root, deps, HomeDeps{})
}

// RegisterRoutesWithHome wires every workbench route onto mux. The single
// place a phase adds a page.
func RegisterRoutesWithHome(mux *http.ServeMux, root string, deps Deps, home HomeDeps) {
	// Resolve the store's operating model ONCE at registration
	// (spec/vocabulary-surfaces: surfaces receive the resolved model from
	// their entrypoint — store.Open here, never re-opened per render) and
	// hand it to both the board instances and the home page. A store whose
	// config cannot be opened serves bare ids (nil model), the exact
	// posture a model with no renames has.
	if deps.Model == nil {
		if cfg, err := store.Open(root); err == nil {
			deps.Model = cfg.Model
		}
	}
	if home.Model == nil {
		home.Model = deps.Model
	}
	mux.HandleFunc("/healthz", healthHandler())
	mux.HandleFunc("/", indexHandler(root, home))

	// Corpus artifact pages (05 §Workbench: server-rendered, goldmark +
	// client-side mermaid). Registered WITHOUT a method prefix (unlike
	// Go 1.22+ ServeMux's "GET /path" form) so every handler's own
	// r.Method check is what produces 405 — a method-qualified pattern
	// here would sometimes fall through to the "/" catch-all instead (its
	// own indexHandler 404s on any non-"/" path), which would silently
	// turn a wrong-method request on a real route into a confusing 404.
	mux.HandleFunc("/a/{kind}/{name}", corpusHandler(root, deps.Model))

	// The disclosures page (spec/disclosures-panel): the checkout's
	// current disclosures, enumerated fresh per render through the shared
	// internal/disclosureview compute path (the dex ships the read-only
	// edition of the same view).
	mux.HandleFunc("/disclosures", disclosuresHandler(root, deps.Disclosures))

	// The verdict viewer: cross-commit per-AC diff of a story's derived
	// verdicts.json snapshots.
	mux.HandleFunc("/verdict/{story...}", verdictHandler(root, deps.Model))

	// The advisory preview matrix page (03 §Evidence records: "advisory
	// renders as preview").
	mux.HandleFunc("/matrix/{story...}", matrixHandler(root))

	// The v1 board: the spec-as-source projection (05 §Workbench, R4).
	// "/board/spec/{name}" is strictly more specific than the v0
	// "/board/{key}/{action}" patterns below, so ServeMux routes every
	// /board/spec/* request here without conflict. The route table itself
	// is declared ONCE (boardSpecRoutes, below) and mounted twice: here at
	// the root for the serving checkout's own tree (spec/draft-boards
	// dc-3: unprefixed addresses keep serving the serving checkout,
	// semantics unchanged), and under the /b/{branch}/ prefix for every
	// design branch's managed worktree (spec/draft-boards ac-1/dc-1: the
	// existing board server rooted at the branch's tree, never a second
	// board implementation).
	bs := &boardSpecServer{root: root, feed: deps.CommentFeed, reviewUnavailable: deps.ReviewUnavailable, supersession: deps.SupersessionCandidates, model: deps.Model}
	for _, rt := range boardSpecRoutes() {
		mux.HandleFunc(rt.suffix, rt.handler(bs))
	}

	// The per-branch draft boards (spec/draft-boards dc-1): the SAME route
	// table beneath /b/{branch}/, the branch riding one path segment with
	// its slashes percent-encoded (design/foo -> design%2Ffoo; Go 1.22+
	// ServeMux segment semantics decode it back). Each request resolves
	// its branch to a managed worktree through the worktree-manager seam
	// and dispatches into that branch's own boardSpecServer instance.
	bb := newBranchBoards(root, deps, bs)
	for _, rt := range boardSpecRoutes() {
		mux.HandleFunc("/b/{branch}"+rt.suffix, bb.dispatch(rt))
	}

	// The diagram-proposal editor (spec/board-editor dc-1): its own board
	// surface at /board/diagram/{name} — page, fragment, and POST api —
	// the same routing grammar as the spec board's trio above. The literal
	// "diagram" segment coexists with "spec" the same way "spec" coexists
	// with the v0 "{key}" patterns below.
	bd := &boardDiagramServer{root: root, verifier: deps.DiagramVerifier}
	mux.HandleFunc("/board/diagram/{name}", bd.boardDiagramPageHandler())
	mux.HandleFunc("/board/diagram/{name}/fragment", bd.boardDiagramFragmentHandler())
	mux.HandleFunc("/board/diagram/{name}/api/{action}", bd.boardDiagramAPIHandler())

	// The v0 board — superseded by board-as-projection (R4-I-9) but kept
	// reachable for grandfathered v0 board.json state. Its two POST
	// routes share one {action} pattern so the v1 literal "spec" segment
	// above can coexist (two sibling 3-segment patterns with different
	// literal positions would be an unresolvable ServeMux conflict).
	mux.HandleFunc("/board/{key}", boardHandler(root, deps.Model))
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
	mux.HandleFunc("/assets/boarddiagram.js", boardDiagramJSHandler())
}
