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
}
