package workbench

import "net/http"

// faviconHandler answers the browser's automatic /favicon.ico request with
// 204 No Content: the workbench ships no icon, and without this route the
// "/" catch-all would render a full disclosed not-found page — top bar
// facts, and so a round of git, included — on every page view
// (spec/chrome-and-tokens-v2 lane F1a review, F1A-B6). GET and HEAD only.
func faviconHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
